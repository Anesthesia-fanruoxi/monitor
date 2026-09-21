package ippass

import (
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
)

// trustProxy 是否允许识别反向代理传入的 X-Real-IP / X-Forwarded-For
// 默认开启以兼容 nginx 反代部署，但只有"直连方本身可信"（在白名单内或是本机回环）
// 时才会采信代理头，因此直连服务端的外部客户端无法靠伪造请求头绕过白名单
var trustProxy atomic.Bool

func init() {
	trustProxy.Store(true)
}

// SetTrustProxy 设置是否识别代理请求头
func SetTrustProxy(enabled bool) {
	trustProxy.Store(enabled)
	log.Printf("代理请求头识别: %v", enabled)
}

// Default 默认白名单实例，提供 /metrics 的访问控制
var Default = NewIPWhitelist("metrics")

// IPWhitelist 一个独立的 IP 白名单
// 条目支持三种形式：域名（定期重新解析）、字面 IP、CIDR 网段
type IPWhitelist struct {
	name string

	mu       sync.RWMutex
	entries  []string
	flatSet  map[string]struct{} // 精确 IP 集合，O(1) 查找
	networks []*net.IPNet        // CIDR 网段
}

// NewIPWhitelist 创建一个白名单实例
func NewIPWhitelist(name string) *IPWhitelist {
	return &IPWhitelist{
		name:    name,
		flatSet: make(map[string]struct{}),
	}
}

// Name 返回白名单名称（用于日志区分）
func (w *IPWhitelist) Name() string {
	return w.name
}

// SetEntries 更新白名单条目并立即解析一次
func (w *IPWhitelist) SetEntries(entries []string) {
	copied := make([]string, 0, len(entries))
	for _, e := range entries {
		if trimmed := strings.TrimSpace(e); trimmed != "" {
			copied = append(copied, trimmed)
		}
	}

	w.mu.Lock()
	w.entries = copied
	w.mu.Unlock()

	log.Printf("[%s] IP 白名单已更新: %v", w.name, copied)
	w.Refresh()
}

// Entries 返回当前配置的条目副本
func (w *IPWhitelist) Entries() []string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	out := make([]string, len(w.entries))
	copy(out, w.entries)
	return out
}

// Refresh 重新解析域名并整体替换缓存
// 解析失败的域名会被跳过（保留其余条目），下一次刷新会重试
func (w *IPWhitelist) Refresh() {
	entries := w.Entries()

	flatSet := make(map[string]struct{}, len(entries))
	var networks []*net.IPNet

	for _, entry := range entries {
		// CIDR 网段
		if _, ipnet, err := net.ParseCIDR(entry); err == nil {
			networks = append(networks, ipnet)
			continue
		}
		// 字面 IP
		if ip := net.ParseIP(entry); ip != nil {
			flatSet[ip.String()] = struct{}{}
			continue
		}
		// 域名
		ips, err := net.LookupHost(entry)
		if err != nil {
			log.Printf("[%s] 域名解析失败: %s, 错误: %v", w.name, entry, err)
			continue
		}
		for _, ip := range ips {
			flatSet[ip] = struct{}{}
		}
	}

	w.mu.Lock()
	w.flatSet = flatSet
	w.networks = networks
	w.mu.Unlock()
}

// Allowed 判断 IP 是否在白名单内
func (w *IPWhitelist) Allowed(ip string) bool {
	parsed := net.ParseIP(strings.TrimSpace(ip))
	if parsed == nil {
		return false
	}

	w.mu.RLock()
	defer w.mu.RUnlock()

	if _, ok := w.flatSet[parsed.String()]; ok {
		return true
	}
	for _, network := range w.networks {
		if network.Contains(parsed) {
			return true
		}
	}
	return false
}

// IsLoopback 判断是否为回环地址
func isLoopback(ip string) bool {
	parsed := net.ParseIP(strings.TrimSpace(ip))
	return parsed != nil && parsed.IsLoopback()
}

// proxyHeaderIP 从代理请求头中取出原始客户端 IP
func proxyHeaderIP(r *http.Request) string {
	if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
		return realIP
	}
	if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); forwarded != "" {
		// X-Forwarded-For 形如 "client, proxy1, proxy2"，取最左侧的原始客户端
		if idx := strings.Index(forwarded, ","); idx != -1 {
			return strings.TrimSpace(forwarded[:idx])
		}
		return forwarded
	}
	return ""
}

// clientIP 解析请求来源 IP
// 只有在直连方可信时才采信代理头：能直连端口的客户端无法伪造 X-Real-IP 混进白名单
func (w *IPWhitelist) clientIP(r *http.Request) (string, error) {
	remoteIP, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return "", err
	}

	if !trustProxy.Load() {
		return remoteIP, nil
	}
	if !w.Allowed(remoteIP) && !isLoopback(remoteIP) {
		return remoteIP, nil
	}
	if ip := proxyHeaderIP(r); ip != "" {
		return ip, nil
	}
	return remoteIP, nil
}

// Middleware 返回带白名单校验的 handler
// 不在白名单内的请求返回 404（不暴露端点是否存在）
func (w *IPWhitelist) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		ip, err := w.clientIP(r)
		if err != nil {
			log.Printf("[%s] 无法解析客户端 IP 地址，RemoteAddr: %s, 错误: %v", w.name, r.RemoteAddr, err)
			http.Error(rw, "无法解析客户端 IP 地址", http.StatusForbidden)
			return
		}

		if !w.Allowed(ip) {
			log.Printf("[%s] IP 访问被拒绝: %s 路径: %s", w.name, ip, r.URL.Path)
			http.Error(rw, "404", http.StatusNotFound)
			return
		}
		next.ServeHTTP(rw, r)
	})
}

// ===================== 兼容旧调用方的包级函数 =====================

// SetAllowedDomains 设置默认白名单（/metrics 使用）
func SetAllowedDomains(domains []string) {
	Default.SetEntries(domains)
}

// RefreshDomainIPCache 刷新默认白名单的域名解析缓存
func RefreshDomainIPCache() {
	Default.Refresh()
}

// IpRestrictionMiddleware 使用默认白名单限制访问
func IpRestrictionMiddleware(next http.Handler) http.Handler {
	return Default.Middleware(next)
}
