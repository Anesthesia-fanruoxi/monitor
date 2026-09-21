package main

import (
	"agent-plugs/sdk"
)

func init() {
	sdk.SetName("nginx")
	sdk.SetVersion("0.1.0")

	// 13 个指标，指标名逐字来自任务 06-S1（docs/插件设计.md §5.2）。
	// 标签集合：前 11 个仅 hostName；upstream 两个带 upstream/status 业务标签。
	hostLabels := []string{"hostName"}
	upstreamLabels := []string{"hostName", "upstream"}
	upstreamStatusLabels := []string{"hostName", "upstream", "status"}

	sdk.RegisterMetric("nginx_connections_active", hostLabels, helpConnectionsActive, nil)
	sdk.RegisterMetric("nginx_connections_reading", hostLabels, helpConnectionsReading, nil)
	sdk.RegisterMetric("nginx_connections_writing", hostLabels, helpConnectionsWriting, nil)
	sdk.RegisterMetric("nginx_connections_waiting", hostLabels, helpConnectionsWaiting, nil)
	sdk.RegisterMetric("nginx_connections_accepted", hostLabels, helpConnectionsAccepted, nil)
	sdk.RegisterMetric("nginx_connections_handled", hostLabels, helpConnectionsHandled, nil)
	sdk.RegisterMetric("nginx_requests_total", hostLabels, helpRequestsTotal, nil)
	sdk.RegisterMetric("nginx_request_time_avg", hostLabels, helpRequestTimeAvg, nil)
	sdk.RegisterMetric("nginx_request_time_max", hostLabels, helpRequestTimeMax, nil)
	sdk.RegisterMetric("nginx_ssl_handshakes", hostLabels, helpSSLHandshakes, nil)
	sdk.RegisterMetric("nginx_ssl_handshakes_failed", hostLabels, helpSSLHandshakesFailed, nil)
	sdk.RegisterMetric("nginx_upstream_response_time", upstreamLabels, helpUpstreamResponseTime, nil)
	sdk.RegisterMetric("nginx_upstream_requests", upstreamStatusLabels, helpUpstreamRequests, nil)
}

func main() {
	sdk.SetCollect(Collect)
	sdk.Run()
}
