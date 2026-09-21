package store

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"agent/common/proto"
	"agent/config"
)

const (
	circuitFailThreshold = 5 // 连续失败 N 次熔断暂停
)

// PluginState 插件进程生命周期状态。
type PluginState int

const (
	StateInit PluginState = iota
	StateRunning
	StateCircuitOpen // 熔断暂停上报
	StateDead
)

// Plugin 是一个已启动/待启动的插件监管对象。
type Plugin struct {
	Name string
	Dir  string // 插件二进制所在目录（agent 同级 plugins/<name>/）

	mu    sync.Mutex
	state PluginState
	cmd   *exec.Cmd
	stdin io.WriteCloser
	// stdout 管道在启动时一次性读掉，不能复用。
	stdout io.ReadCloser

	// 插件写回的请求-响应对，由 scheduler 发 collect，pump 回 Env。
	collectCh chan proto.Env

	// 运行期错误计数，用于熔断
	failCount int
}

// NewPlugin 创建一个插件对象（不启动进程）。
func NewPlugin(name, pluginsDir string) *Plugin {
	return &Plugin{
		Name:      name,
		Dir:       filepath.Join(pluginsDir, name),
		collectCh: make(chan proto.Env, 1),
	}
}

// BinaryPath 返回插件预期二进制绝对路径（Linux 无后缀，与 build.sh 制品名一致）。
func (p *Plugin) BinaryPath() string {
	return filepath.Join(p.Dir, p.Name)
}

// EnsureBinary 确保插件二进制就绪：本地存在直接放行；
// 缺失时从 service 签名下载安装。失败只告警返回 false，不阻断壳启动。
func (p *Plugin) EnsureBinary(base string, key []byte) bool {
	if st, err := os.Stat(p.BinaryPath()); err == nil && st.Size() > 0 {
		return true
	}
	if base == "" {
		log.Printf("[shell] %s: 无制品服务，跳过懒下载", p.Name)
		return false
	}
	if len(key) == 0 {
		log.Printf("[shell] %s: encryption_key 未配置，无法签名下载", p.Name)
		return false
	}
	if _, err := InstallPlugin(p.Name, base, key, filepath.Dir(p.Dir)); err != nil {
		log.Printf("[shell] %s: 下载安装失败: %v", p.Name, err)
		return false
	}
	log.Printf("[shell] %s: 下载安装成功", p.Name)
	return true
}

// Start 启动插件子进程，建立 stdin/stdout pipe，发送 hello，启动 pump。
// envJSON 为壳注入的插件配置（MONITOR_CONFIG，由 EnvForPlugin 组装）。
func (p *Plugin) Start(ctx context.Context, envJSON string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 已在运行：幂等返回，避免并发重启（更新 vs watchdog）漏掉旧进程
	if p.state == StateRunning && p.cmd != nil {
		return nil
	}

	bin := p.BinaryPath()
	if _, err := os.Stat(bin); err != nil {
		return fmt.Errorf("插件二进制不存在: %w", err)
	}

	cmd := exec.CommandContext(ctx, bin)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), "MONITOR_CONFIG="+envJSON)
	cmd.Dir = p.Dir

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start %s: %w", p.Name, err)
	}

	p.cmd = cmd
	p.stdin = stdin
	p.stdout = stdout
	p.state = StateRunning
	p.failCount = 0 // 重启后熔断计数清零

	go p.pump(ctx)
	_, _ = stdin.Write(proto.EncodeHello(1))
	return nil
}

// Shutdown 给插件发 shutdown 请求，等进程退出；超时后 kill。
func (p *Plugin) Shutdown() {
	p.mu.Lock()
	cmd := p.cmd
	stdin := p.stdin
	p.mu.Unlock()

	if cmd == nil {
		return
	}

	if stdin != nil {
		_, _ = stdin.Write(proto.EncodeShutdown(2))
		_ = stdin.Close()
	}

	done := make(chan error, 1)
	go func() {
		_, err := cmd.Process.Wait()
		done <- err
	}()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		_ = cmd.Process.Kill()
	}

	p.mu.Lock()
	p.state = StateDead
	p.cmd = nil
	p.mu.Unlock()
}

// collectRequestID 与 proto.EncodeCollect 的 id 对应，用于在回包里识别本轮响应。
const collectRequestID = 3

// Collect 发一轮 collect，阻塞等待 Env 或 ctx 取消。
// 单在途由 scheduler 保证。
func (p *Plugin) Collect(ctx context.Context, cfg *config.ConfigCenter) (proto.Env, bool) {
	p.mu.Lock()
	if p.state == StateCircuitOpen {
		p.mu.Unlock()
		return proto.Env{}, false
	}
	stdin := p.stdin
	p.mu.Unlock()

	if stdin == nil {
		return proto.Env{}, false
	}

	// 先清掉陈旧回包（hello 的响应、上一轮超时后迟到的响应）：
	// collectCh 缓冲只有 1，不清的话本轮真正的响应会被挤掉，
	// 而 Collect 反而读到旧的空回包，表现为首轮采集静默丢失。
drain:
	for {
		select {
		case stale := <-p.collectCh:
			log.Printf("[shell] %s 丢弃陈旧回包 id=%d", p.Name, stale.ID)
		default:
			break drain
		}
	}

	if _, err := stdin.Write(proto.EncodeCollect(collectRequestID)); err != nil {
		log.Printf("[shell] %s 写 collect 失败: %v", p.Name, err)
		p.recordFail()
		return proto.Env{}, false
	}

	timeout := cfg.PluginTimeout(p.Name)
	tctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	for {
		select {
		case env := <-p.collectCh:
			if env.ID != collectRequestID {
				log.Printf("[shell] %s 丢弃陈旧回包 id=%d", p.Name, env.ID)
				continue
			}
			if !env.OK {
				p.recordFail()
			} else {
				p.resetFail()
			}
			return env, env.OK
		case <-tctx.Done():
			log.Printf("[shell] %s collect 超时 %v", p.Name, timeout)
			p.recordFail()
			return proto.Env{}, false
		}
	}
}

// pump 从 stdout 逐行读，Decode 后送 collectCh。panic recover。
func (p *Plugin) pump(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[shell] %s pump panic: %v", p.Name, r)
		}
		p.mu.Lock()
		if p.state == StateRunning {
			p.state = StateDead
		}
		p.mu.Unlock()
	}()

	p.mu.Lock()
	reader := p.stdout
	p.mu.Unlock()
	if reader == nil {
		return
	}

	scanner := bufio.NewScanner(reader)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 16*1024*1024)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}

		env, warns, err := proto.Decode(scanner.Bytes())
		if err != nil {
			log.Printf("[shell] %s decode 错误: %v", p.Name, err)
			continue
		}
		for _, w := range warns {
			log.Printf("[shell] %s 字段降级 %s: %s", p.Name, w.Field, w.Msg)
		}

		select {
		case p.collectCh <- env:
		default:
			log.Printf("[shell] %s collectCh 满，丢弃一条响应", p.Name)
		}
	}

	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
		log.Printf("[shell] %s stdout 读停止: %v", p.Name, err)
	}
}

func (p *Plugin) recordFail() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.failCount++
	if p.failCount >= circuitFailThreshold && p.state == StateRunning {
		p.state = StateCircuitOpen
		log.Printf("[shell] %s 连续失败 %d 次，熔断暂停上报", p.Name, p.failCount)
	}
}

func (p *Plugin) resetFail() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.failCount = 0
	if p.state == StateCircuitOpen {
		p.state = StateRunning
		log.Printf("[shell] %s 恢复正常，关闭熔断", p.Name)
	}
}

// State 对外只读状态。
func (p *Plugin) State() PluginState {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.state
}

// IsAlive 插件是否处于运行态（崩溃/熔断返回 false，供 watchdog 自愈判定）。
func (p *Plugin) IsAlive() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.state == StateRunning
}
