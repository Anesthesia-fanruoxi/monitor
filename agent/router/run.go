package router

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"agent/api"
	"agent/config"
	"agent/store"
)

// Run 是壳的主入口。main() 里调 Run 即可。
//
// 流程：
//  1. 构造 ConfigCenter + Checker → 启动自检
//  2. 热加载 goroutine
//  3. 为每个启用的插件 Start（并发）
//  4. 启动 Scheduler / Transport / Heartbeat
//  5. 等 ctx 取消 → 优雅退出（10s 宽限 → kill）
func Run(ctx context.Context) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("executable: %w", err)
	}
	exeDir := filepath.Dir(exe)

	// 1. 配置 + 自检
	cc, err := config.NewConfigCenter(exeDir)
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	cc.LogSummary()
	cfg := cc.Get()

	// 插件目录：exe 侧不存在时回落到工作目录（go run 时 exe 在临时目录，
	// 回落保证插件与 .plugins.json 状态持久，与 config 的回落策略一致）
	pluginsDir := filepath.Join(exeDir, "plugins")
	if _, err := os.Stat(pluginsDir); err != nil {
		if wd, werr := os.Getwd(); werr == nil {
			pluginsDir = filepath.Join(wd, "plugins")
		}
	}

	checker := api.NewChecker(cc, pluginsDir, cfg.MetricsURL, false)
	if err := checker.Run(); err != nil {
		return fmt.Errorf("selfcheck 失败: %w", err)
	}
	log.Println("[shell] 启动自检通过")

	// 2. 热加载
	go cc.HotReload(ctx)

	// 3. 插件监管
	dirs := store.Dirs{ExeDir: exeDir, Plugins: pluginsDir}
	mgr := store.NewPluginMgr(dirs)

	for name := range cfg.Plugins {
		mgr.Register(name)
	}

	// 并发 Start 所有启用插件
	var startWg sync.WaitGroup
	for _, p := range mgr.All() {
		if !cc.IsEnabled(p.Name) {
			log.Printf("[shell] %s 未启用，跳过 start", p.Name)
			continue
		}
		startWg.Add(1)
		go func(pl *store.Plugin) {
			defer startWg.Done()
			if !pl.EnsureBinary(store.ServiceBase(cfg.MetricsURL), []byte(cfg.EncryptionKey)) {
				log.Printf("[shell] %s 二进制缺失，跳过启动", pl.Name)
				return
			}
			if err := pl.Start(ctx, store.EnvForPlugin(pl.Name, cc)); err != nil {
				log.Printf("[shell] %s 启动失败: %v", pl.Name, err)
			} else {
				log.Printf("[shell] %s 已启动", pl.Name)
			}
		}(p)
	}
	startWg.Wait()

	// 4. 传输 / 调度 / 心跳
	xfer := api.NewTransport(cc)

	var runWg sync.WaitGroup

	runWg.Add(1)
	go func() {
		defer runWg.Done()
		xfer.Run(ctx)
	}()

	runWg.Add(1)
	go func() {
		defer runWg.Done()
		sched := NewScheduler(cc, mgr, xfer)
		sched.Run(ctx)
	}()

	runWg.Add(1)
	go func() {
		defer runWg.Done()
		hb := api.NewHeartbeat(cc, xfer)
		hb.Run(ctx)
	}()

	// 4.5 插件版本定时检查更新（auto_update）
	if cfg.AutoUpdate {
		runWg.Add(1)
		go func() {
			defer runWg.Done()
			// 启动即查一次，之后按周期轮询
			checkUpdates(ctx, cc, mgr, dirs.Plugins)
			t := time.NewTicker(updateCheckInterval)
			defer t.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-t.C:
					checkUpdates(ctx, cc, mgr, dirs.Plugins)
				}
			}
		}()
	}

	log.Println("[shell] 壳已就绪，进入主循环")

	// 等 ctx 取消
	<-ctx.Done()
	log.Println("[shell] 收到退出信号，开始优雅关闭...")

	// 5. 优雅退出：给子 goroutine 宽限时间
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 关闭插件
	mgr.ShutdownAll()

	// 再等一轮 xfer / scheduler / heartbeat 退出（受 ctx 取消驱动）
	// 这里复用 runWg：Run 里的 goroutine 已经在 ctx.Done() 处感知退出信号
	// 我们最多等 10 秒
	done := make(chan struct{})
	go func() { runWg.Wait(); close(done) }()

	select {
	case <-done:
		log.Println("[shell] 所有组件已优雅退出")
	case <-shutdownCtx.Done():
		log.Println("[shell] 优雅退出超时，强制返回")
	}

	log.Printf("[shell] 插件退出码: %v", mgr.ExitCodes())
	return nil
}

// updateCheckInterval 插件版本检查周期
const updateCheckInterval = time.Minute

// checkUpdates 对每个启用插件做 HEAD 轻量比对（远端 sha256 vs .plugins.json 记录），
// 有变化则：停插件 → 重新下载替换 → 重启。失败仅日志，不影响运行中插件。
func checkUpdates(ctx context.Context, cc *config.ConfigCenter, mgr *store.PluginMgr, pluginsDir string) {
	cfg := cc.Get()
	base := store.ServiceBase(cfg.MetricsURL)
	key := []byte(cfg.EncryptionKey)
	if base == "" || len(key) == 0 {
		return
	}
	state := store.LoadPluginState(pluginsDir)
	for _, p := range mgr.All() {
		if !cc.IsEnabled(p.Name) {
			continue
		}
		changed, err := store.UpdateIfNew(p.Name, base, key, pluginsDir, state[p.Name].SHA256)
		if err != nil {
			log.Printf("[shell] %s 更新检查失败: %v", p.Name, err)
			continue
		}
		if !changed {
			continue
		}
		p.Shutdown()
		if err := p.Start(ctx, store.EnvForPlugin(p.Name, cc)); err != nil {
			log.Printf("[shell] %s 更新后重启失败: %v", p.Name, err)
			continue
		}
		log.Printf("[shell] %s 已更新并重启", p.Name)
	}
}
