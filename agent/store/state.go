package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// 插件本地状态：隐藏文件 plugins/.plugins.json，
// 记录每个已安装插件的版本与制品校验信息，作为版本比对与排障的本地事实来源。
// agent 不关心插件数据语义，但必须知道"本地装的是哪个版本"。

// PluginInfo 单个插件的安装信息（无版本号概念，以文件 sha256 为准）。
type PluginInfo struct {
	SHA256      string `json:"sha256"`
	InstalledAt int64  `json:"installed_at"` // 毫秒
}

var stateMu sync.Mutex

func statePath(pluginsDir string) string {
	return filepath.Join(pluginsDir, ".plugins.json")
}

// LoadPluginState 读取插件状态；文件不存在或损坏返回空 map（不报错）。
func LoadPluginState(pluginsDir string) map[string]PluginInfo {
	stateMu.Lock()
	defer stateMu.Unlock()
	return loadStateLocked(pluginsDir)
}

func loadStateLocked(pluginsDir string) map[string]PluginInfo {
	out := map[string]PluginInfo{}
	b, err := os.ReadFile(statePath(pluginsDir))
	if err != nil {
		return out
	}
	_ = json.Unmarshal(b, &out)
	return out
}

// SavePluginInfo 记录/更新一个插件的安装信息（tmp + rename 原子写）。
func SavePluginInfo(pluginsDir, name string, info PluginInfo) error {
	stateMu.Lock()
	defer stateMu.Unlock()

	st := loadStateLocked(pluginsDir)
	info.InstalledAt = time.Now().UnixMilli()
	st[name] = info

	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	tmp := statePath(pluginsDir) + ".tmp"
	if err := os.WriteFile(tmp, b, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, statePath(pluginsDir))
}
