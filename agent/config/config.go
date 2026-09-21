package config

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// PluginConf 插件级配置。enable/interval/timeout 是壳保留键，其余留给插件。
type PluginConf struct {
	Enable   bool           `yaml:"enable" json:"enable"`
	Interval string         `yaml:"interval" json:"interval"`
	Timeout  string         `yaml:"timeout" json:"timeout"`
	Config   map[string]any `yaml:"config,omitempty" json:"config,omitempty"`
}

// Config 是壳的完整运行时配置。顶层字段 + 各插件配置。
type Config struct {
	Project       string                `yaml:"project" json:"project"`
	MetricsURL    string                `yaml:"metrics_url" json:"metrics_url"`
	EncryptionKey string                `yaml:"encryption_key" json:"encryption_key"`
	AutoUpdate    bool                  `yaml:"auto_update" json:"auto_update"`
	Plugins       map[string]PluginConf `yaml:"plugins" json:"plugins"`
}

// ---- 钳制常量 ----
const (
	minInterval     = 1 * time.Second
	maxInterval     = 30 * time.Minute
	defaultInterval = 30 * time.Second

	// timeout 约束：[2×interval, 30m]，越界回落 2.5×interval
	maxTimeout = 30 * time.Minute
)

type clamped struct {
	interval time.Duration
	timeout  time.Duration
}

func clampInterval(s string) time.Duration {
	if s == "" {
		return defaultInterval
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		log.Printf("[shell] interval 非法 %q，回落默认 %v", s, defaultInterval)
		return defaultInterval
	}
	if d < minInterval || d > maxInterval {
		log.Printf("[shell] interval %v 越界 [%v,%v]，回落默认 %v", d, minInterval, maxInterval, defaultInterval)
		return defaultInterval
	}
	return d
}

func clampTimeout(s string, interval time.Duration) time.Duration {
	upper := interval * 2
	lower := interval * 2
	fallback := time.Duration(float64(interval) * 2.5)

	d, err := time.ParseDuration(s)
	if err != nil {
		log.Printf("[shell] timeout 非法 %q，回落 2.5×interval=%v", s, fallback)
		return fallback
	}
	_ = lower // 下限就是 2×interval
	if d < upper || d > maxTimeout {
		log.Printf("[shell] timeout %v 越界 [2×interval=%v,%v]，回落 2.5×interval=%v", d, upper, maxTimeout, fallback)
		return fallback
	}
	return d
}

// ConfigCenter 配置中心：原子读 + 5 秒热加载（轮询 mtime，无 fsnotify 依赖）。
type ConfigCenter struct {
	mu   sync.RWMutex
	cfg  *Config
	plug map[string]clamped

	path    string // config.yaml 路径
	envJSON string // MONITOR_CONFIG（优先）
	offline bool
}

// NewConfigCenter 从 config.yaml 或 MONITOR_CONFIG 环境变量加载。
// allowOffline + service 不可达时标记 offline。
func NewConfigCenter(exeDir string) (*ConfigCenter, error) {
	cc := &ConfigCenter{
		path:    filepath.Join(exeDir, "config.yaml"),
		envJSON: os.Getenv("MONITOR_CONFIG"),
	}
	// go run / 异地运行时 exe 在临时目录，exe 侧找不到则回落到工作目录
	if cc.envJSON == "" {
		if _, err := os.Stat(cc.path); err != nil {
			if wd, werr := os.Getwd(); werr == nil {
				if _, err2 := os.Stat(filepath.Join(wd, "config.yaml")); err2 == nil {
					cc.path = filepath.Join(wd, "config.yaml")
				}
			}
		}
	}
	if err := cc.load(); err != nil {
		return nil, err
	}
	return cc, nil
}

// LogSummary 启动时打印加载到的配置概要，便于排查部署问题。
func (cc *ConfigCenter) LogSummary() {
	cfg := cc.Get()
	key := "未配置"
	if len(cfg.EncryptionKey) > 0 {
		key = fmt.Sprintf("%d字节", len(cfg.EncryptionKey))
	}
	log.Printf("[shell] 配置文件: %s", cc.path)
	log.Printf("[shell] 配置内容: project=%s metrics_url=%s encryption_key=%s auto_update=%v",
		cfg.Project, cfg.MetricsURL, key, cfg.AutoUpdate)
	for name, p := range cfg.Plugins {
		log.Printf("[shell] 插件 %s: enable=%v interval=%s timeout=%s config=%v",
			name, p.Enable, p.Interval, p.Timeout, p.Config)
	}
}

// load 读一次配置。env 优先；文件不存在但 env 存在则 OK；两者都缺失则报错。
func (cc *ConfigCenter) load() error {
	var raw []byte
	if cc.envJSON != "" {
		raw = []byte(cc.envJSON)
	} else {
		b, err := os.ReadFile(cc.path)
		if err != nil {
			return err
		}
		raw = b
	}

	cfg := &Config{}
	if len(raw) > 0 && raw[0] == '{' {
		if err := json.Unmarshal(raw, cfg); err != nil {
			return err
		}
	} else {
		if err := yaml.Unmarshal(raw, cfg); err != nil {
			return err
		}
	}
	cc.recompute(cfg)
	return nil
}

func (cc *ConfigCenter) recompute(cfg *Config) {
	plug := make(map[string]clamped, len(cfg.Plugins))
	for name, p := range cfg.Plugins {
		iv := clampInterval(p.Interval)
		to := clampTimeout(p.Timeout, iv)
		plug[name] = clamped{interval: iv, timeout: to}
	}
	cc.mu.Lock()
	cc.cfg = cfg
	cc.plug = plug
	cc.mu.Unlock()
}

// Get 返回顶层配置快照。
func (cc *ConfigCenter) Get() *Config {
	cc.mu.RLock()
	defer cc.mu.RUnlock()
	return cc.cfg
}

// PluginInterval 返回插件当前 interval；缺省回落默认值。
func (cc *ConfigCenter) PluginInterval(name string) time.Duration {
	cc.mu.RLock()
	defer cc.mu.RUnlock()
	if c, ok := cc.plug[name]; ok {
		return c.interval
	}
	return defaultInterval
}

// PluginTimeout 返回插件当前 timeout。
func (cc *ConfigCenter) PluginTimeout(name string) time.Duration {
	cc.mu.RLock()
	defer cc.mu.RUnlock()
	if c, ok := cc.plug[name]; ok {
		return c.timeout
	}
	return clampTimeout("", cc.PluginInterval(name))
}

// PluginConf 返回插件配置（含 enable 和自定义 Config）。
func (cc *ConfigCenter) PluginConf(name string) (PluginConf, bool) {
	cc.mu.RLock()
	defer cc.mu.RUnlock()
	p, ok := cc.cfg.Plugins[name]
	return p, ok
}

// IsEnabled 插件是否启用。
func (cc *ConfigCenter) IsEnabled(name string) bool {
	p, ok := cc.PluginConf(name)
	return ok && p.Enable
}

// IsOffline 壳当前是否处于离线模式。
func (cc *ConfigCenter) IsOffline() bool {
	cc.mu.RLock()
	defer cc.mu.RUnlock()
	return cc.offline
}

// SetOffline 设置离线标志（启动自检时由 selfcheck 调用）。
func (cc *ConfigCenter) SetOffline(v bool) {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	cc.offline = v
}

// HotReload 启动 5 秒轮询 mtime 的热加载 goroutine。
func (cc *ConfigCenter) HotReload(ctx Done) {
	// 没有文件路径（纯 env 注入）时不做热加载
	if cc.envJSON != "" {
		return
	}

	var lastMtime time.Time
	if st, err := os.Stat(cc.path); err == nil {
		lastMtime = st.ModTime()
	}

	t := time.NewTicker(5 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			st, err := os.Stat(cc.path)
			if err != nil {
				log.Printf("[shell] config.yaml stat 失败: %v", err)
				continue
			}
			if !st.ModTime().After(lastMtime) {
				continue
			}
			lastMtime = st.ModTime()
			if err := cc.load(); err != nil {
				log.Printf("[shell] 配置热加载失败: %v，保留旧值", err)
				continue
			}
			log.Println("[shell] 配置热加载成功")
		}
	}
}

// Done 是 context 的最小子集，方便替换测试。
type Done interface {
	Done() <-chan struct{}
}
