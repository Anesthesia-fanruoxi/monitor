package store

import (
	"monitor-server/common"

	"github.com/prometheus/client_golang/prometheus"
)

// 服务端自监控指标
//
// 这些指标注册在 CustomRegistry 上，与业务指标一起被 /metrics 暴露，
// 运维侧可以直接在 Grafana 看到接收链路的健康度。
var (
	ingestReceivedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "monitor_ingest_received_total",
			Help: "声明式上报成功处理的次数",
		},
		[]string{"source"},
	)
	ingestQueueLevel = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "monitor_ingest_queue_level",
			Help: "当前 worker pool 任务队列长度，每秒采样一次",
		},
	)
	ingestDroppedTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "monitor_ingest_dropped_total",
			Help: "worker pool 队列满时丢弃的任务数",
		},
	)
	reregisterSuppressTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "monitor_ingest_reregister_suppress_total",
			Help: "reRegisterLocked 被调用的次数（标签集合变化导致重建 GaugeVec）",
		},
	)
)

func init() {
	CustomRegistry.MustRegister(ingestReceivedTotal)
	CustomRegistry.MustRegister(ingestQueueLevel)
	CustomRegistry.MustRegister(ingestDroppedTotal)
	CustomRegistry.MustRegister(reregisterSuppressTotal)

	// worker pool 的观测回调：队列丢弃/长度采样写入自监控指标。
	// common 不反向依赖 store，由 store 在 init 时注入，保持依赖方向单一。
	common.SetQueueHooks(IngestIncDropped, IngestSetQueueLevel)
}

// IngestIncReceived 每次 HandleDeclarativePayload 成功后调用
func IngestIncReceived(source string) {
	ingestReceivedTotal.WithLabelValues(source).Inc()
}

// IngestSetQueueLevel 由 worker pool 的采样 goroutine 每秒调用
func IngestSetQueueLevel(n int) {
	ingestQueueLevel.Set(float64(n))
}

// IngestIncDropped 队列满丢弃任务时调用
func IngestIncDropped() {
	ingestDroppedTotal.Inc()
}

// IngestIncReregisterSuppress reRegisterLocked 被调用时计数
func IngestIncReregisterSuppress() {
	reregisterSuppressTotal.Inc()
}
