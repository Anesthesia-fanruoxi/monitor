package Metrics

import "github.com/prometheus/client_golang/prometheus"

var CustomRegistry = prometheus.NewRegistry()

// 预先注册静态指标
// 预先注册静态指标
func init() {
	// ====================== 系统 & 主机指标 ======================
	CustomRegistry.MustRegister(CpuPercentMetric)
	CustomRegistry.MustRegister(DiskTotalMetric)
	CustomRegistry.MustRegister(DiskUsedMetric)
	CustomRegistry.MustRegister(DiskFreeMetric)
	CustomRegistry.MustRegister(DiskUsedPercentMetric)
	CustomRegistry.MustRegister(MemoryTotalMetric)
	CustomRegistry.MustRegister(MemoryUsedMetric)
	CustomRegistry.MustRegister(MemoryFreeMetric)
	CustomRegistry.MustRegister(MemoryUsedPercentMetric)
	CustomRegistry.MustRegister(CpuLoad1Metric)
	CustomRegistry.MustRegister(CpuLoad5Metric)
	CustomRegistry.MustRegister(CpuLoad15Metric)
	CustomRegistry.MustRegister(CpuTotalMetric)

	// ====================== Nginx 指标 ======================
	CustomRegistry.MustRegister(NginxIsRunMetric)
	CustomRegistry.MustRegister(NginxReTotalMetric)
	CustomRegistry.MustRegister(NginxLoginUserCountMetric)
	CustomRegistry.MustRegister(NginxRawTotalMetric)
	CustomRegistry.MustRegister(NginxUdptotalMetric)
	CustomRegistry.MustRegister(NginxTcpTotalMetric)
	CustomRegistry.MustRegister(NginxTotalTcpMetric)
	CustomRegistry.MustRegister(NginxInetTotalMetric)
	CustomRegistry.MustRegister(NginxFragTotalMetric)
	CustomRegistry.MustRegister(NginxTcpEstabMetric)
	CustomRegistry.MustRegister(NginxTcpClosedMetric)
	CustomRegistry.MustRegister(NginxTcpOrphanedMetric)
	CustomRegistry.MustRegister(NginxTcpTimewaitMetric)

	// ====================== SSL & 容器 & Agent & Controller ======================
	CustomRegistry.MustRegister(SslDaysLeftMetric)
	CustomRegistry.MustRegister(ContainerCpuUsageMetric)
	CustomRegistry.MustRegister(ContainerMemoryUsageMetric)
	CustomRegistry.MustRegister(ContainerCpuLimitMetric)
	CustomRegistry.MustRegister(ContainerMemoryLimitMetric)
	CustomRegistry.MustRegister(ContainerRestartCountMetric)
	CustomRegistry.MustRegister(ContainerLastTerminationTimeMetric)
	CustomRegistry.MustRegister(IsActiveMetric)
	CustomRegistry.MustRegister(AgentVerisonMetric)
	CustomRegistry.MustRegister(ControllerReplicasMetric)
	CustomRegistry.MustRegister(ControllerReplicasAvailableMetric)
	CustomRegistry.MustRegister(ControllerReplicasUnavailableMetric)

	// ====================== TrafficSwitching 业务指标 ======================
	// 累计统计
	CustomRegistry.MustRegister(TrafficSwitchingTotalRequests)
	CustomRegistry.MustRegister(TrafficSwitchingTotalSuccess)
	CustomRegistry.MustRegister(TrafficSwitchingTotalErrors)
	CustomRegistry.MustRegister(TrafficSwitchingTotalSuccessRate)
	// 今日统计
	CustomRegistry.MustRegister(TrafficSwitchingTodayRequests)
	CustomRegistry.MustRegister(TrafficSwitchingTodaySuccess)
	CustomRegistry.MustRegister(TrafficSwitchingTodayErrors)
	CustomRegistry.MustRegister(TrafficSwitchingTodayCanceled)
	CustomRegistry.MustRegister(TrafficSwitchingTodayStatus2xx)
	CustomRegistry.MustRegister(TrafficSwitchingTodayStatus3xx)
	CustomRegistry.MustRegister(TrafficSwitchingTodayStatus4xx)
	CustomRegistry.MustRegister(TrafficSwitchingTodayStatus5xx)
	// 实时统计
	CustomRegistry.MustRegister(TrafficSwitchingRealtimeQPS)
	CustomRegistry.MustRegister(TrafficSwitchingRealtimeSuccessQPS)
	CustomRegistry.MustRegister(TrafficSwitchingRealtimeErrorQPS)
	CustomRegistry.MustRegister(TrafficSwitchingRealtimeActiveConnections)
	CustomRegistry.MustRegister(TrafficSwitchingRealtimeAvgLatencyMs)
	CustomRegistry.MustRegister(TrafficSwitchingRealtimeMaxLatencyMs)
	// 错误类型
	CustomRegistry.MustRegister(TrafficSwitchingErrorBackendError)
	CustomRegistry.MustRegister(TrafficSwitchingErrorBrokenPipe)
	CustomRegistry.MustRegister(TrafficSwitchingErrorConnectionRefused)
	CustomRegistry.MustRegister(TrafficSwitchingErrorConnectionReset)
	CustomRegistry.MustRegister(TrafficSwitchingErrorDNSError)
	CustomRegistry.MustRegister(TrafficSwitchingErrorEOF)
	CustomRegistry.MustRegister(TrafficSwitchingErrorTimeout)
	// 缓存
	CustomRegistry.MustRegister(TrafficSwitchingProxyCacheSize)
	CustomRegistry.MustRegister(TrafficSwitchingProxyMaxCacheSize)
	// Runtime
	CustomRegistry.MustRegister(TrafficSwitchingRuntimeGoroutines)
	CustomRegistry.MustRegister(TrafficSwitchingRuntimeMemoryMB)
	CustomRegistry.MustRegister(TrafficSwitchingRuntimeCPUCores)
	CustomRegistry.MustRegister(TrafficSwitchingRuntimeGomaxprocs)
	CustomRegistry.MustRegister(TrafficSwitchingRuntimeGcCycles)
	// Transport
	CustomRegistry.MustRegister(TrafficSwitchingTransportMaxConnsPerHost)
	CustomRegistry.MustRegister(TrafficSwitchingTransportMaxIdleConns)
	CustomRegistry.MustRegister(TrafficSwitchingTransportMaxIdleConnsPerHost)
	// 时间戳
	CustomRegistry.MustRegister(TrafficSwitchingTimestamp)
}
