package sdk

import (
	"os"
	"strings"
	"sync"
	"time"
)

// Hostname 缓存策略与壳侧 agent/Metrics.GetHostName 保持一致：
// 优先 os.Hostname()，失败或为空则退回 /etc/hostname。
// 5 分钟 TTL，双重检查锁。
const hostnameCacheTTL = 5 * time.Minute

var (
	cachedHostname     string
	hostnameMu         sync.RWMutex
	hostnameLastLoaded time.Time
)

// Hostname 返回当前主机名。
//
// 优先级：
//  1. os.Hostname()
//  2. /etc/hostname
//
// 任何错误都返回空串（采集逻辑应容忍空 hostname）。
func Hostname() string {
	hostnameMu.RLock()
	if cachedHostname != "" && time.Since(hostnameLastLoaded) < hostnameCacheTTL {
		h := cachedHostname
		hostnameMu.RUnlock()
		return h
	}
	hostnameMu.RUnlock()

	hostnameMu.Lock()
	defer hostnameMu.Unlock()

	if cachedHostname != "" && time.Since(hostnameLastLoaded) < hostnameCacheTTL {
		return cachedHostname
	}

	name, err := os.Hostname()
	if err != nil || strings.TrimSpace(name) == "" {
		if data, readErr := os.ReadFile("/etc/hostname"); readErr == nil {
			name = strings.TrimSpace(string(data))
		}
	}

	if name != "" {
		cachedHostname = name
		hostnameLastLoaded = time.Now()
	}
	return name
}
