package Modles

import "time"

type HardSource struct {
	HostName          string  `json:"hostName" mapstructure:"hostName"`
	CPUPercent        float64 `json:"cpu_percent" mapstructure:"cpu_percent"`
	DiskTotal         float64 `json:"disk_total" mapstructure:"disk_total"`
	DiskUsed          float64 `json:"disk_used" mapstructure:"disk_used"`
	DiskFree          float64 `json:"disk_free" mapstructure:"disk_free"`
	DiskUsedPercent   float64 `json:"disk_used_percent" mapstructure:"disk_used_percent"`
	MemoryTotal       float64 `json:"memory_total" mapstructure:"memory_total"`
	MemoryUsed        float64 `json:"memory_used" mapstructure:"memory_used"`
	MemoryFree        float64 `json:"memory_free" mapstructure:"memory_free"`
	MemoryBuffered    float64 `json:"memory_buffered" mapstructure:"memory_buffered"`
	MemoryCached      float64 `json:"memory_cached" mapstructure:"memory_cached"`
	MemoryShared      float64 `json:"memory_shared" mapstructure:"memory_shared"`
	MemoryAvailable   float64 `json:"memory_available" mapstructure:"memory_available"`
	MemoryUsedPercent float64 `json:"memory_used_percent" mapstructure:"memory_used_percent"`
	CPULoad1          float64 `json:"cpu_load_1" mapstructure:"cpu_load_1"`
	CPULoad5          float64 `json:"cpu_load_5" mapstructure:"cpu_load_5"`
	CPULoad15         float64 `json:"cpu_load_15" mapstructure:"cpu_load_15"`
	CPUCount          float64 `json:"cpu_count" mapstructure:"cpu_count"`
	CPUModel          string  `json:"cpu_model" mapstructure:"cpu_model"`
	OSVersion         string  `json:"os_version" mapstructure:"os_version"`
	KernelVersion     string  `json:"kernel_version" mapstructure:"kernel_version"`
}
type NginxSource struct {
	HostName       string `json:"hostName" mapstructure:"hostName"`
	IsRun          int    `json:"isRun" mapstructure:"isRun"`
	ReTotal        int    `json:"reTotal" mapstructure:"reTotal"`
	LoginUserCount int    `json:"loginUserCount" mapstructure:"loginUserCount"`
	RawTotal       int    `json:"rawTotal" mapstructure:"rawTotal"`
	Udptotal       int    `json:"udptotal" mapstructure:"udptotal"`
	TcpTotal       int    `json:"tcpTotal" mapstructure:"tcpTotal"`
	TotalTcp       int    `json:"totaltcp" mapstructure:"totaltcp"`
	InetTotal      int    `json:"inetTotal" mapstructure:"inetTotal"`
	FragTotal      int    `json:"fragTotal" mapstructure:"fragTotal"`
	TcpEstab       int    `json:"tcpEstab" mapstructure:"tcpEstab"`
	TcpClosed      int    `json:"tcpClosed" mapstructure:"tcpClosed"`
	TcpOrphaned    int    `json:"tcpOrphaned" mapstructure:"tcpOrphaned"`
	TcpTimewait    int    `json:"tcpTimewait" mapstructure:"tcpTimewait"`
}

type SslSource struct {
	Domain     string `json:"domain" mapstructure:"domain"`
	Comment    string `json:"comment" mapstructure:"comment"`
	Expiration string `json:"expiration" mapstructure:"expiration"`
	DaysLeft   int    `json:"days_left" mapstructure:"days_left"`
	Status     string `json:"status" mapstructure:"status"`
	Resolve    bool   `json:"resolve" mapstructure:"resolve"`
}

type ContainerResource struct {
	Namespace           string  `json:"namespace" mapstructure:"namespace"`
	PodName             string  `json:"podName" mapstructure:"podName"`
	ControllerName      string  `json:"controllerName" mapstructure:"controllerName"`
	Container           string  `json:"container" mapstructure:"container"`
	LimitCpu            float64 `json:"limitCpu" mapstructure:"limitCpu"`           // CPU 核心数
	LimitMemory         int64   `json:"limitMemory" mapstructure:"limitMemory"`     // 内存字节数
	RequestCpu          float64 `json:"requestCpu" mapstructure:"requestCpu"`       // CPU 核心数
	RequestMemory       int64   `json:"requestMemory" mapstructure:"requestMemory"` // 内存字节数
	UseCpu              float64 `json:"useCpu" mapstructure:"useCpu"`               // 使用的 CPU 核心数
	UseMemory           int64   `json:"useMemory" mapstructure:"useMemory"`         // 使用的内存字节数
	RestartCount        int     `json:"restartCount" mapstructure:"restartCount"`   // 重启次数
	LastTerminationTime int64   `json:"lastTerminationTime" mapstructure:"lastTerminationTime"`
}
type ControllerResource struct {
	Namespace           string `json:"namespace" mapstructure:"namespace"`
	Container           string `json:"container" mapstructure:"container"`
	ControllerType      string `json:"controllerType" mapstructure:"controllerType"`
	Replicas            int32  `json:"replicas" mapstructure:"replicas"`
	ReplicasAvailable   int32  `json:"replicas_available" mapstructure:"replicas_available"`
	ReplicasUnavailable int32  `json:"replicas_unavailable" mapstructure:"replicas_unavailable"`
}

// MetricWithTimestamp 用于存储指标值和最后更新的时间戳
type MetricWithTimestamp struct {
	Value      float64
	MetricName string
	Timestamp  time.Time
}
type HeartSource struct {
	IsActive int     `json:"isActive" mapstructure:"isActive"` // 是否活跃（1：活跃，0：不活跃）
	Project  string  `json:"project" mapstructure:"project"`   // 项目名称
	Hostname string  `json:"hostname" mapstructure:"hostname"` // 主机名
	Version  float64 `json:"version" mapstructure:"version"`   //当前版本号
}
type EsIpSource struct {
	IpCount  int    `json:"client_ip_count"`
	ClientIp string `json:"client_ip"`
	Project  string `json:"project"`
}
type EsCountrySource struct {
	CountryCount int    `json:"country_name_count"`
	CountryName  string `json:"country_name"`
	Project      string `json:"project"`
}
type EsUrlSource struct {
	UrlCount   int    `json:"request_url_count"`
	RequestUrl string `json:"request_url"`
	Project    string `json:"project"`
}

// TrafficSwitchingSource 流量切换平台上报数据
type TrafficSwitchingSource struct {
	Service string `json:"service" mapstructure:"service"` // 服务名称

	// 累计统计（持续增长）
	TotalRequests    float64     `json:"total_requests" mapstructure:"total_requests"`
	TotalSuccess     float64     `json:"total_success" mapstructure:"total_success"`
	TotalErrors      float64     `json:"total_errors" mapstructure:"total_errors"`
	TotalSuccessRate interface{} `json:"total_success_rate" mapstructure:"total_success_rate"`

	// 今日统计（每天凌晨重置）
	TodayRequests  float64 `json:"today_requests" mapstructure:"today_requests"`
	TodaySuccess   float64 `json:"today_success" mapstructure:"today_success"`
	TodayErrors    float64 `json:"today_errors" mapstructure:"today_errors"`
	TodayCanceled  float64 `json:"today_canceled" mapstructure:"today_canceled"`
	TodayStatus2xx float64 `json:"today_status_2xx" mapstructure:"today_status_2xx"`
	TodayStatus3xx float64 `json:"today_status_3xx" mapstructure:"today_status_3xx"`
	TodayStatus4xx float64 `json:"today_status_4xx" mapstructure:"today_status_4xx"`
	TodayStatus5xx float64 `json:"today_status_5xx" mapstructure:"today_status_5xx"`

	// 实时统计
	RealtimeQPS               float64 `json:"realtime_qps" mapstructure:"realtime_qps"`
	RealtimeSuccessQPS        float64 `json:"realtime_success_qps" mapstructure:"realtime_success_qps"`
	RealtimeErrorQPS          float64 `json:"realtime_error_qps" mapstructure:"realtime_error_qps"`
	RealtimeActiveConnections float64 `json:"realtime_active_connections" mapstructure:"realtime_active_connections"`
	RealtimeAvgLatencyMs      float64 `json:"realtime_avg_latency_ms" mapstructure:"realtime_avg_latency_ms"`
	RealtimeMaxLatencyMs      float64 `json:"realtime_max_latency_ms" mapstructure:"realtime_max_latency_ms"`

	// 错误类型统计
	ErrorBackendError      float64 `json:"error_backend_error" mapstructure:"error_backend_error"`
	ErrorBrokenPipe        float64 `json:"error_broken_pipe" mapstructure:"error_broken_pipe"`
	ErrorConnectionRefused float64 `json:"error_connection_refused" mapstructure:"error_connection_refused"`
	ErrorConnectionReset   float64 `json:"error_connection_reset" mapstructure:"error_connection_reset"`
	ErrorDNSError          float64 `json:"error_dns_error" mapstructure:"error_dns_error"`
	ErrorEOF               float64 `json:"error_eof" mapstructure:"error_eof"`
	ErrorTimeout           float64 `json:"error_timeout" mapstructure:"error_timeout"`

	// 代理缓存
	ProxyCacheSize    float64 `json:"proxy_cache_size" mapstructure:"proxy_cache_size"`
	ProxyMaxCacheSize float64 `json:"proxy_max_cache_size" mapstructure:"proxy_max_cache_size"`

	// Runtime
	RuntimeGoroutines float64 `json:"runtime_goroutines" mapstructure:"runtime_goroutines"`
	RuntimeMemoryMB   float64 `json:"runtime_memory_mb" mapstructure:"runtime_memory_mb"`
	RuntimeCPUCores   float64 `json:"runtime_cpu_cores" mapstructure:"runtime_cpu_cores"`
	RuntimeGomaxprocs float64 `json:"runtime_gomaxprocs" mapstructure:"runtime_gomaxprocs"`
	RuntimeGcCycles   float64 `json:"runtime_gc_cycles" mapstructure:"runtime_gc_cycles"`

	// Transport 配置
	TransportMaxConnsPerHost     float64 `json:"transport_max_conns_per_host" mapstructure:"transport_max_conns_per_host"`
	TransportMaxIdleConns        float64 `json:"transport_max_idle_conns" mapstructure:"transport_max_idle_conns"`
	TransportMaxIdleConnsPerHost float64 `json:"transport_max_idle_conns_per_host" mapstructure:"transport_max_idle_conns_per_host"`

	// 上报时间戳
	Timestamp float64 `json:"timestamp" mapstructure:"timestamp"`
}
