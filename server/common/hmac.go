package common

import (
	"crypto/hmac"
	"crypto/sha256"
	"sync"
	"time"
)

// ===== HMAC key 注入 =====
//
// 由 config 包在加载配置时调 SetHMACKey。
// 复用 AES 加密密钥（docs/插件化架构设计.md §0 决策 #3），
// 通过 setter 注入而不是直接依赖加密实现，保持各层解耦。

var (
	hmacKeyMu sync.RWMutex
	hmacKey   []byte
)

// SetHMACKey 设置 HMAC 签名密钥（返回副本防止外部篡改）。
// 接受 nil / 空表示禁用下载端点（拒绝所有未签名请求）。
func SetHMACKey(key []byte) {
	hmacKeyMu.Lock()
	defer hmacKeyMu.Unlock()
	if key == nil {
		hmacKey = nil
		return
	}
	hmacKey = append([]byte(nil), key...)
}

// GetHMACKey 获取当前 HMAC 签名密钥（返回副本）
func GetHMACKey() []byte {
	hmacKeyMu.RLock()
	defer hmacKeyMu.RUnlock()
	if hmacKey == nil {
		return nil
	}
	return append([]byte(nil), hmacKey...)
}

// ===== nonce 防重放缓存 =====
//
// 用环形缓冲保存最近见过的 nonce，TTL = MaxSkew。
// 超过容量时丢掉最老的（保守做法：宁可错杀也不放过重放）。

const (
	// MaxSkew 允许的时间戳偏移，与上报路径的 MaxClockSkew 保持一致
	MaxSkew = 5 * time.Minute
	// NonceCacheCap nonce 缓存上限（1000 足够覆盖一个轮询周期内的高并发）
	NonceCacheCap = 1000
)

type nonceCache struct {
	mu    sync.Mutex
	seen  map[string]time.Time
	order []string // FIFO 队列，淘汰最老
}

var nonces = &nonceCache{seen: make(map[string]time.Time, NonceCacheCap)}

// checkAndStore 检查 nonce 是否重复；不重复则记下当前时间并按 FIFO 淘汰过期项。
// 返回 true 表示 nonce 是新的（未重放），false 表示已被见过。
func (c *nonceCache) checkAndStore(nonce string, now time.Time) bool {
	if nonce == "" {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	if t, ok := c.seen[nonce]; ok {
		// 仍在 TTL 内 → 重放
		if now.Sub(t) < MaxSkew {
			return false
		}
		// 已过期 → 视为新 nonce，覆盖
	}
	// 容量满 → 淘汰最老（且过期）的
	if len(c.seen) >= NonceCacheCap {
		c.evictLocked(now)
	}
	c.seen[nonce] = now
	c.order = append(c.order, nonce)
	return true
}

// evictLocked 调用方需持有 c.mu
func (c *nonceCache) evictLocked(now time.Time) {
	// 整轮扫一遍清掉所有过期的；如果还不够，丢掉最老的一半
	for n, t := range c.seen {
		if now.Sub(t) >= MaxSkew {
			delete(c.seen, n)
		}
	}
	// 重建 order（去掉已删的）
	if len(c.seen) < NonceCacheCap {
		newOrder := make([]string, 0, len(c.seen))
		for n := range c.seen {
			newOrder = append(newOrder, n)
		}
		c.order = newOrder
		return
	}
	// 仍然满 → 砍掉最老的 10%，腾位置
	cut := NonceCacheCap / 10
	if cut < 1 {
		cut = 1
	}
	for i := 0; i < cut && i < len(c.order); i++ {
		delete(c.seen, c.order[i])
	}
	c.order = c.order[cut:]
}

// CheckNonce 检查 nonce 是否重放；未见过返回 true 并记录
func CheckNonce(nonce string, now time.Time) bool {
	return nonces.checkAndStore(nonce, now)
}

// ComputeHMAC 返回 HMAC-SHA256(key, material) 的字节
func ComputeHMAC(key []byte, material string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(material))
	return h.Sum(nil)
}
