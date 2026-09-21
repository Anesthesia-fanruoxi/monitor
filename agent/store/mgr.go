package store

import (
	"encoding/json"
	"path/filepath"
	"sync"

	"agent/config"
)

// Dirs 壳运行时需要的几个目录。
type Dirs struct {
	ExeDir  string // agent 二进制所在目录
	Plugins string // 插件二进制根目录（通常 ExeDir/plugins）
}

// PluginMgr 维护所有插件对象。
type PluginMgr struct {
	mu      sync.Mutex
	plugins map[string]*Plugin
	dirs    Dirs
}

// NewPluginMgr 创建插件监管。
func NewPluginMgr(dirs Dirs) *PluginMgr {
	if dirs.Plugins == "" {
		dirs.Plugins = filepath.Join(dirs.ExeDir, "plugins")
	}
	return &PluginMgr{
		plugins: make(map[string]*Plugin),
		dirs:    dirs,
	}
}

// Register 登记一个插件（不启动进程）。
func (m *PluginMgr) Register(name string) *Plugin {
	m.mu.Lock()
	defer m.mu.Unlock()
	p := NewPlugin(name, m.dirs.Plugins)
	m.plugins[name] = p
	return p
}

// Get 按名字拿插件。
func (m *PluginMgr) Get(name string) *Plugin {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.plugins[name]
}

// All 返回所有已登记插件。
func (m *PluginMgr) All() []*Plugin {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*Plugin, 0, len(m.plugins))
	for _, p := range m.plugins {
		out = append(out, p)
	}
	return out
}

// ShutdownAll 逐个 shutdown 所有插件（忽略错误）。
func (m *PluginMgr) ShutdownAll() {
	for _, p := range m.All() {
		p.Shutdown()
	}
}

// ExitCodes 读取所有插件最近一次退出码。
func (m *PluginMgr) ExitCodes() map[string]int {
	out := make(map[string]int)
	for _, p := range m.All() {
		p.mu.Lock()
		var code int
		if p.cmd != nil && p.cmd.Process != nil {
			code = p.cmd.ProcessState.ExitCode()
		} else if p.state == StateDead {
			code = -1
		}
		p.mu.Unlock()
		out[p.Name] = code
	}
	return out
}

// EnvForPlugin 组装发给插件的 MONITOR_CONFIG 环境变量 JSON。
func EnvForPlugin(name string, cfg *config.ConfigCenter) string {
	p, ok := cfg.PluginConf(name)
	if !ok {
		return "{}"
	}
	raw := map[string]any{
		"enable":   p.Enable,
		"interval": p.Interval,
		"timeout":  p.Timeout,
	}
	for k, v := range p.Config {
		raw[k] = v
	}
	b, _ := json.Marshal(raw)
	return string(b)
}
