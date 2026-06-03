package Metrics

import "github.com/prometheus/client_golang/prometheus"

// trafficSwitchingLabels 通用标签：service, project
var trafficSwitchingLabels = []string{"service", "project"}

var (
	// 累计请求统计
	TrafficSwitchingTotalRequests = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "trafficswitching_total_requests",
			Help: "累计请求总数",
		},
		trafficSwitchingLabels,
	)

	TrafficSwitchingTotalSuccess = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "trafficswitching_total_success",
			Help: "累计成功请求数",
		},
		trafficSwitchingLabels,
	)

	TrafficSwitchingTotalErrors = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "trafficswitching_total_errors",
			Help: "累计失败请求数",
		},
		trafficSwitchingLabels,
	)

	TrafficSwitchingTotalSuccessRate = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "trafficswitching_total_success_rate",
			Help: "累计成功率（0-1）",
		},
		trafficSwitchingLabels,
	)

	// 今日统计
	TrafficSwitchingTodayRequests = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "trafficswitching_today_requests",
			Help: "今日请求数",
		},
		trafficSwitchingLabels,
	)

	TrafficSwitchingTodaySuccess = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "trafficswitching_today_success",
			Help: "今日成功数",
		},
		trafficSwitchingLabels,
	)

	TrafficSwitchingTodayErrors = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "trafficswitching_today_errors",
			Help: "今日错误数",
		},
		trafficSwitchingLabels,
	)

	TrafficSwitchingTodayCanceled = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "trafficswitching_today_canceled",
			Help: "今日取消数",
		},
		trafficSwitchingLabels,
	)

	TrafficSwitchingTodayStatus2xx = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "trafficswitching_today_status_2xx",
			Help: "今日 2xx 状态码数",
		},
		trafficSwitchingLabels,
	)

	TrafficSwitchingTodayStatus3xx = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "trafficswitching_today_status_3xx",
			Help: "今日 3xx 状态码数",
		},
		trafficSwitchingLabels,
	)

	TrafficSwitchingTodayStatus4xx = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "trafficswitching_today_status_4xx",
			Help: "今日 4xx 状态码数",
		},
		trafficSwitchingLabels,
	)

	TrafficSwitchingTodayStatus5xx = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "trafficswitching_today_status_5xx",
			Help: "今日 5xx 状态码数",
		},
		trafficSwitchingLabels,
	)

	// 实时统计
	TrafficSwitchingRealtimeQPS = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "trafficswitching_realtime_qps",
			Help: "实时 QPS",
		},
		trafficSwitchingLabels,
	)

	TrafficSwitchingRealtimeSuccessQPS = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "trafficswitching_realtime_success_qps",
			Help: "实时成功 QPS",
		},
		trafficSwitchingLabels,
	)

	TrafficSwitchingRealtimeErrorQPS = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "trafficswitching_realtime_error_qps",
			Help: "实时错误 QPS",
		},
		trafficSwitchingLabels,
	)

	TrafficSwitchingRealtimeActiveConnections = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "trafficswitching_realtime_active_connections",
			Help: "实时活跃连接数",
		},
		trafficSwitchingLabels,
	)

	TrafficSwitchingRealtimeAvgLatencyMs = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "trafficswitching_realtime_avg_latency_ms",
			Help: "实时平均延迟（毫秒）",
		},
		trafficSwitchingLabels,
	)

	TrafficSwitchingRealtimeMaxLatencyMs = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "trafficswitching_realtime_max_latency_ms",
			Help: "实时最大延迟（毫秒）",
		},
		trafficSwitchingLabels,
	)

	// 错误类型统计
	TrafficSwitchingErrorBackendError = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "trafficswitching_error_backend_error",
			Help: "后端错误数",
		},
		trafficSwitchingLabels,
	)

	TrafficSwitchingErrorBrokenPipe = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "trafficswitching_error_broken_pipe",
			Help: "管道断开错误数",
		},
		trafficSwitchingLabels,
	)

	TrafficSwitchingErrorConnectionRefused = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "trafficswitching_error_connection_refused",
			Help: "连接拒绝错误数",
		},
		trafficSwitchingLabels,
	)

	TrafficSwitchingErrorConnectionReset = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "trafficswitching_error_connection_reset",
			Help: "连接重置错误数",
		},
		trafficSwitchingLabels,
	)

	TrafficSwitchingErrorDNSError = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "trafficswitching_error_dns_error",
			Help: "DNS 解析错误数",
		},
		trafficSwitchingLabels,
	)

	TrafficSwitchingErrorEOF = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "trafficswitching_error_eof",
			Help: "EOF 错误数",
		},
		trafficSwitchingLabels,
	)

	TrafficSwitchingErrorTimeout = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "trafficswitching_error_timeout",
			Help: "超时错误数",
		},
		trafficSwitchingLabels,
	)

	// 代理缓存
	TrafficSwitchingProxyCacheSize = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "trafficswitching_proxy_cache_size",
			Help: "当前代理缓存大小",
		},
		trafficSwitchingLabels,
	)

	TrafficSwitchingProxyMaxCacheSize = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "trafficswitching_proxy_max_cache_size",
			Help: "代理缓存最大限制",
		},
		trafficSwitchingLabels,
	)

	// Runtime 指标
	TrafficSwitchingRuntimeGoroutines = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "trafficswitching_runtime_goroutines",
			Help: "当前 Goroutine 数量",
		},
		trafficSwitchingLabels,
	)

	TrafficSwitchingRuntimeMemoryMB = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "trafficswitching_runtime_memory_mb",
			Help: "Go 程序当前使用的内存（MB）",
		},
		trafficSwitchingLabels,
	)

	TrafficSwitchingRuntimeCPUCores = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "trafficswitching_runtime_cpu_cores",
			Help: "机器CPU核心数",
		},
		trafficSwitchingLabels,
	)

	TrafficSwitchingRuntimeGomaxprocs = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "trafficswitching_runtime_gomaxprocs",
			Help: "GOMAXPROCS 值",
		},
		trafficSwitchingLabels,
	)

	TrafficSwitchingRuntimeGcCycles = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "trafficswitching_runtime_gc_cycles",
			Help: "GC 运行次数",
		},
		trafficSwitchingLabels,
	)

	// Transport 配置
	TrafficSwitchingTransportMaxConnsPerHost = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "trafficswitching_transport_max_conns_per_host",
			Help: "每个主机最大连接数",
		},
		trafficSwitchingLabels,
	)

	TrafficSwitchingTransportMaxIdleConns = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "trafficswitching_transport_max_idle_conns",
			Help: "全局最大空闲连接数",
		},
		trafficSwitchingLabels,
	)

	TrafficSwitchingTransportMaxIdleConnsPerHost = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "trafficswitching_transport_max_idle_conns_per_host",
			Help: "每个主机最大空闲连接数",
		},
		trafficSwitchingLabels,
	)

	// 上报时间戳
	TrafficSwitchingTimestamp = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "trafficswitching_timestamp",
			Help: "本次指标上报的时间戳",
		},
		trafficSwitchingLabels,
	)
)
