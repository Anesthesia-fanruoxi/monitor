package router

import (
	"context"
	"log"
	"math/rand"
	"sync"
	"time"

	"agent/api"
	"agent/config"
	"agent/store"
)

// Scheduler 按插件 interval 定时调度 collect。
// 特性：
//   - 启动时随机相位打散（每个插件启动延迟 0~interval 随机值）
//   - 单在途请求：上一轮 collect 没结束则跳过本轮
type Scheduler struct {
	cfg  *config.ConfigCenter
	mgr  *store.PluginMgr
	xfer *api.Transport
}

// NewScheduler 创建调度器。
func NewScheduler(cfg *config.ConfigCenter, mgr *store.PluginMgr, xfer *api.Transport) *Scheduler {
	return &Scheduler{cfg: cfg, mgr: mgr, xfer: xfer}
}

// Run 为每个启用的插件启动一个 goroutine，阻塞直到 ctx 取消。
func (s *Scheduler) Run(ctx context.Context) {
	var wg sync.WaitGroup

	for _, p := range s.mgr.All() {
		name := p.Name
		if !s.cfg.IsEnabled(name) {
			log.Printf("[shell] scheduler: %s 未启用，跳过", name)
			continue
		}

		interval := s.cfg.PluginInterval(name)
		// 随机相位打散
		jitter := time.Duration(rand.Int63n(int64(interval)))
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.loop(ctx, name, p, jitter)
		}()
	}

	wg.Wait()
}

// maxRestartWait 自愈重启的退避上限（轮）：连续失败时按 1/3/7/8… 轮间隔重试，
// 避免插件系统性问题（如配置错）导致每轮崩溃重启刷日志。
const maxRestartWait = 8

// loop 一个插件的主循环。
func (s *Scheduler) loop(ctx context.Context, name string, p *store.Plugin, initialDelay time.Duration) {
	log.Printf("[shell] scheduler: %s 启动，初始延迟 %v，间隔 %v", name, initialDelay, s.cfg.PluginInterval(name))

	// 初始相位
	select {
	case <-ctx.Done():
		return
	case <-time.After(initialDelay):
	}

	var inFlight bool      // 单在途标记
	var restartWait int    // 自愈重启剩余等待轮数（退避）
	for {
		interval := s.cfg.PluginInterval(name)
		timer := time.NewTimer(interval)

		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}

		// 存活检查：插件崩溃 / 熔断 → 退避自愈重启（含二进制缺失补下载）
		if !p.IsAlive() {
			if restartWait > 0 {
				restartWait--
				continue
			}
			cfg := s.cfg.Get()
			log.Printf("[shell] scheduler: %s 不在运行态，尝试自愈重启", name)
			p.Shutdown()
			p.EnsureBinary(store.ServiceBase(cfg.MetricsURL), []byte(cfg.EncryptionKey))
			if err := p.Start(ctx, store.EnvForPlugin(name, s.cfg)); err != nil {
				restartWait = min(restartWait*2+1, maxRestartWait)
				log.Printf("[shell] scheduler: %s 自愈重启失败（%v），%d 轮后重试", name, err, restartWait+1)
			} else {
				log.Printf("[shell] scheduler: %s 已自愈重启", name)
			}
			continue
		}
		restartWait = 0

		if inFlight {
			log.Printf("[shell] scheduler: %s 上一轮未结束，跳过本轮", name)
			continue
		}

		inFlight = true
		go func() {
			defer func() { inFlight = false }()
			if env, ok := p.Collect(ctx, s.cfg); ok {
				s.xfer.OnCollect(name, env.Metrics)
			}
		}()
	}
}
