package main

import (
	"bufio"
	"crypto/tls"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"agent-plugs/sdk"
)

// helpCertExpiresDays 是常量（§3.4 第 4 条）。
const helpCertExpiresDays = "ssl 证书剩余有效天数"

// defaultDomainsFile 与 default.yaml 一致；shell 未注入配置时兜底。
const defaultDomainsFile = "/etc/monitor/ssl_domains.txt"

// 单域名 TLS 握手超时；并发上限（与现状一致）。
const (
	dialTimeout    = 5 * time.Second
	maxConcurrency = 20
)

// domainsFile 读 domains_file 配置项，缺省回 defaultDomainsFile。
func domainsFile() string {
	cfg := sdk.LoadConfig()
	if v, ok := cfg["domains_file"].(string); ok && strings.TrimSpace(v) != "" {
		return v
	}
	return defaultDomainsFile
}

// readDomains 读域名列表，每行一个；忽略空行与 # 注释，去重。
func readDomains(path string) []string {
	f, err := os.Open(path)
	if err != nil {
		sdk.Fprintln("[ssl] domains_file 读取失败：", err)
		return nil
	}
	defer f.Close()

	var out []string
	seen := make(map[string]struct{})
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if _, dup := seen[line]; dup {
			continue
		}
		seen[line] = struct{}{}
		out = append(out, line)
	}
	return out
}

// certDaysLeft TLS 握手取证书 NotAfter，计算剩余天数。
// 不足一天但还有效记 1；已过期返回负数天数（与现状一致）。
// 探测失败返回 0,false —— 调用方 skip，不填 -1（避免误告警）。
func certDaysLeft(domain string) (float64, bool) {
	dialer := &net.Dialer{Timeout: dialTimeout}
	conn, err := tls.DialWithDialer(dialer, "tcp", net.JoinHostPort(domain, "443"), &tls.Config{
		ServerName:         domain,
		InsecureSkipVerify: true, // 容错：证书过期/自签也要能取到证书信息
	})
	if err != nil {
		return 0, false
	}
	defer conn.Close()

	certs := conn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return 0, false
	}
	return float64(daysUntilExpiration(certs[0].NotAfter)), true
}

// daysUntilExpiration 与旧 agent/Metrics/Ssl.go 行为一致。
func daysUntilExpiration(expiration time.Time) int {
	remaining := time.Until(expiration)
	if remaining <= 0 {
		// 已过期：向上取整负天数
		return -int(-remaining.Hours()/24) - 1
	}
	if remaining < 24*time.Hour {
		return 1 // 不足一天但还有效，至少算 1 天
	}
	return int(remaining.Hours() / 24)
}

// Collect 主入口：对 domains_file 中每个域名并发 TLS 探测。
// 探测失败的域名本轮不上报（skip，不填 -1）；全部失败 → metrics 为空（§3.4 合法）。
func Collect() []sdk.DeclaredMetric {
	domains := readDomains(domainsFile())
	if len(domains) == 0 {
		return nil
	}

	type result struct {
		domain string
		days   float64
		ok     bool
	}
	results := make([]result, len(domains))

	sem := make(chan struct{}, maxConcurrency)
	var wg sync.WaitGroup
	for i, d := range domains {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, d string) {
			defer wg.Done()
			defer func() { <-sem }()
			defer func() {
				if r := recover(); r != nil {
					sdk.Fprintln("[ssl] 探测 panic 恢复：", d, r)
				}
			}()
			days, ok := certDaysLeft(d)
			results[i] = result{domain: d, days: days, ok: ok}
		}(i, d)
	}
	wg.Wait()

	samples := make([]sdk.Sample, 0, len(results))
	for _, r := range results {
		if !r.ok {
			sdk.Fprintf("[ssl] 域名 %s 探测失败，本轮不上报", r.domain)
			continue
		}
		samples = append(samples, sdk.Sample{
			Labels: map[string]string{"domain": r.domain},
			Value:  r.days,
		})
	}
	if len(samples) == 0 {
		return nil
	}
	return []sdk.DeclaredMetric{{
		Name:    "ssl_cert_expires_days",
		Help:    helpCertExpiresDays,
		Type:    sdk.MetricTypeGauge,
		Labels:  []string{"domain"},
		Samples: samples,
	}}
}
