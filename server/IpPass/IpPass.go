package IpPass

import (
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
)

// 定义允许的域名（使用读写锁保护）
var allowedDomains []string
var allowedDomainsMu sync.RWMutex

var domainIPCache = struct {
	mapping map[string][]string
	mutex   sync.RWMutex
}{
	mapping: make(map[string][]string),
}

// 设置允许的域名（线程安全）
func SetAllowedDomains(domains []string) {
	allowedDomainsMu.Lock()
	defer allowedDomainsMu.Unlock()
	allowedDomains = domains
	log.Printf("已设置允许的域名: %v", allowedDomains)
}

// 获取允许的域名（线程安全）
func getAllowedDomains() []string {
	allowedDomainsMu.RLock()
	defer allowedDomainsMu.RUnlock()
	// 返回副本避免外部修改
	result := make([]string, len(allowedDomains))
	copy(result, allowedDomains)
	return result
}

// 刷新域名解析缓存（单次执行，线程安全）
func RefreshDomainIPCache() {
	domains := getAllowedDomains()
	for _, domain := range domains {
		ips, err := net.LookupHost(domain)
		if err != nil {
			log.Printf("域名解析失败: %s, 错误: %v", domain, err)
			continue
		}

		// 更新缓存
		domainIPCache.mutex.Lock()
		domainIPCache.mapping[domain] = ips
		domainIPCache.mutex.Unlock()
	}
}

// 检查请求 IP 是否在缓存的域名解析 IP 列表中
func isAllowedIP(ip string) bool {
	domainIPCache.mutex.RLock()
	defer domainIPCache.mutex.RUnlock()

	for _, ips := range domainIPCache.mapping {
		for _, cachedIP := range ips {
			if cachedIP == ip {
				return true
			}
		}
	}
	return false
}

// getClientIP 获取客户端真实 IP
// 优先从 Nginx 代理头获取，适用于 Nginx 反向代理场景
func getClientIP(r *http.Request) (string, error) {
	// 优先使用 X-Real-IP（Nginx 通常设置此头）
	if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		return strings.TrimSpace(realIP), nil
	}

	// 其次使用 X-Forwarded-For（取第一个 IP，即原始客户端）
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		ip := strings.TrimSpace(forwarded)
		if idx := strings.Index(ip, ","); idx != -1 {
			ip = strings.TrimSpace(ip[:idx])
		}
		return ip, nil
	}

	// 最后使用直连 IP
	remoteIP, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return "", err
	}
	return remoteIP, nil
}

func IpRestrictionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, err := getClientIP(r)
		if err != nil {
			log.Printf("无法解析客户端 IP 地址，RemoteAddr: %s, 错误: %v", r.RemoteAddr, err)
			http.Error(w, "无法解析客户端 IP 地址", http.StatusForbidden)
			return
		}

		// 打印请求 IP 信息（用于调试）
		// remoteIP, _, _ := net.SplitHostPort(r.RemoteAddr)
		// log.Printf("[Metrics] 请求 IP 信息 - 路径: %s, 最终IP: %s, RemoteAddr: %s, X-Forwarded-For: %s, X-Real-IP: %s",
		// 	r.URL.Path, ip, remoteIP, r.Header.Get("X-Forwarded-For"), r.Header.Get("X-Real-IP"))

		// 检查 IP 是否允许访问
		if !isAllowedIP(ip) {
			log.Printf("IP 访问被拒绝: %s", ip)
			http.Error(w, "404", http.StatusNotFound)
			return
		}
		next.ServeHTTP(w, r)
	})
}
