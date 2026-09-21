package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"agent-plugs/sdk"
)

// ===== 指标 help 常量（§3.4 第 4 条：help 必须是常量，不拼时间戳/主机名）=====
const (
	helpConnectionsActive    = "nginx 活动连接数"
	helpConnectionsReading   = "nginx 读连接数"
	helpConnectionsWriting   = "nginx 写连接数"
	helpConnectionsWaiting   = "nginx 等待连接数"
	helpConnectionsAccepted  = "nginx 已接受连接数"
	helpConnectionsHandled   = "nginx 已处理连接数"
	helpRequestsTotal        = "nginx 已处理请求总数"
	helpRequestTimeAvg       = "nginx 请求平均耗时"
	helpRequestTimeMax       = "nginx 请求最大耗时"
	helpSSLHandshakes        = "nginx SSL 握手数"
	helpSSLHandshakesFailed  = "nginx SSL 握手失败数"
	helpUpstreamResponseTime = "nginx upstream 响应耗时"
	helpUpstreamRequests     = "nginx upstream 请求数"
)

// defaultStatusURL 与 default.yaml 一致；shell 未注入配置时兜底。
const defaultStatusURL = "http://127.0.0.1/nginx_status"

// warn-once：stub_status 不可达只告警一次，相关指标上报 0，不让插件失败。
var stubWarnOnce sync.Once

func warnStub(msg string) {
	stubWarnOnce.Do(func() {
		sdk.Fprintln("[nginx] stub_status 不可达，相关指标本轮上报 0：", msg)
	})
}

// statusURL 读 nginx_status_url 配置项，缺省回 defaultStatusURL。
func statusURL() string {
	cfg := sdk.LoadConfig()
	if v, ok := cfg["nginx_status_url"].(string); ok && strings.TrimSpace(v) != "" {
		return v
	}
	return defaultStatusURL
}

// fetchStubStatus HTTP GET stub_status 文本，5 秒超时。
func fetchStubStatus(url string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("stub_status HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// stub_status 样例：
//
//	Active connections: 5
//	server accepts handled requests
//	 1234 1234 5678
//	Reading: 0 Writing: 1 Waiting: 4
var (
	reActive = regexp.MustCompile(`Active connections:\s*(\d+)`)
	reRW     = regexp.MustCompile(`Reading:\s*(\d+)\s+Writing:\s*(\d+)\s+Waiting:\s*(\d+)`)
)

// parseStubStatus 解析 7 个 stub_status 可得指标；缺项保持 0。
func parseStubStatus(text string) map[string]float64 {
	out := make(map[string]float64, 7)
	if m := reActive.FindStringSubmatch(text); len(m) > 1 {
		if v, err := strconv.ParseFloat(m[1], 64); err == nil {
			out["active"] = v
		}
	}
	if m := reRW.FindStringSubmatch(text); len(m) > 3 {
		if v, err := strconv.ParseFloat(m[1], 64); err == nil {
			out["reading"] = v
		}
		if v, err := strconv.ParseFloat(m[2], 64); err == nil {
			out["writing"] = v
		}
		if v, err := strconv.ParseFloat(m[3], 64); err == nil {
			out["waiting"] = v
		}
	}
	// accepts/handled/requests 行：紧跟 "server accepts handled requests" 之后
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if strings.Contains(line, "server accepts handled requests") && i+1 < len(lines) {
			fields := strings.Fields(lines[i+1])
			if len(fields) >= 3 {
				if v, err := strconv.ParseFloat(fields[0], 64); err == nil {
					out["accepted"] = v
				}
				if v, err := strconv.ParseFloat(fields[1], 64); err == nil {
					out["handled"] = v
				}
				if v, err := strconv.ParseFloat(fields[2], 64); err == nil {
					out["requests"] = v
				}
			}
			break
		}
	}
	return out
}

// buildHostMetric 构造一条仅 hostName 标签的单样本 DeclaredMetric。
func buildHostMetric(name, help, host string, value float64) sdk.DeclaredMetric {
	return sdk.DeclaredMetric{
		Name:   name,
		Help:   help,
		Type:   sdk.MetricTypeGauge,
		Labels: []string{"hostName"},
		Samples: []sdk.Sample{
			{Labels: map[string]string{"hostName": host}, Value: value},
		},
	}
}

// Collect 主入口：返回 11 条 hostName 指标。
//   - stub_status 可达：7 个连接/请求指标取真实值
//   - stub_status 不可达：11 个 hostName 指标全部上报 0 + warn-once
//   - 4 个 request_time/ssl 指标：stub_status 不提供，固定上报 0
//   - 2 个 upstream 指标：需 tail access log 聚合，本期不实现，本轮无样本
//     （指标声明已在 main.go 注册，不发样本即"本轮无数据"，见 §3.4）
func Collect() []sdk.DeclaredMetric {
	host := sdk.Hostname()
	out := make([]sdk.DeclaredMetric, 0, 11)

	values := make(map[string]float64)
	if text, err := fetchStubStatus(statusURL()); err == nil {
		values = parseStubStatus(text)
	} else {
		warnStub(err.Error())
	}

	out = append(out,
		buildHostMetric("nginx_connections_active", helpConnectionsActive, host, values["active"]),
		buildHostMetric("nginx_connections_reading", helpConnectionsReading, host, values["reading"]),
		buildHostMetric("nginx_connections_writing", helpConnectionsWriting, host, values["writing"]),
		buildHostMetric("nginx_connections_waiting", helpConnectionsWaiting, host, values["waiting"]),
		buildHostMetric("nginx_connections_accepted", helpConnectionsAccepted, host, values["accepted"]),
		buildHostMetric("nginx_connections_handled", helpConnectionsHandled, host, values["handled"]),
		buildHostMetric("nginx_requests_total", helpRequestsTotal, host, values["requests"]),
		// stub_status 不提供，固定 0
		buildHostMetric("nginx_request_time_avg", helpRequestTimeAvg, host, 0),
		buildHostMetric("nginx_request_time_max", helpRequestTimeMax, host, 0),
		buildHostMetric("nginx_ssl_handshakes", helpSSLHandshakes, host, 0),
		buildHostMetric("nginx_ssl_handshakes_failed", helpSSLHandshakesFailed, host, 0),
	)
	// nginx_upstream_response_time / nginx_upstream_requests：
	// 本期不 tail access log，这两个指标本轮不发样本。
	return out
}
