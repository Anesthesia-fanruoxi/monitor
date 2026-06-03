package Handers

import (
	"log"
	"monitor-server/Metrics"
	"sync"
	"time"
)

// TrafficSwitchingTimestamp 用来存储各实例的时间戳，key 格式: service|:|project
var TrafficSwitchingTimestamp = sync.Map{}

// parseTrafficSwitchingLabel 反解析 label 字符串，返回 service 和 project
func parseTrafficSwitchingLabel(metricLabel string) (string, string) {
	parts := SplitLabels(metricLabel)
	if len(parts) < 2 {
		log.Printf("[TrafficSwitching] 标签 %s 无法解析，格式不正确", metricLabel)
		return "", ""
	}
	return parts[0], parts[1]
}

// CheckTrafficSwitchingHeartbeats 定期检查超时的 TrafficSwitching 数据
func CheckTrafficSwitchingHeartbeats() {
	currentTime := time.Now()

	TrafficSwitchingTimestamp.Range(func(key, value interface{}) bool {
		metricLabel, ok := key.(string)
		if !ok {
			log.Printf("[TrafficSwitching] 标签格式不正确，跳过")
			return true
		}

		timestamp, ok := value.(time.Time)
		if !ok {
			log.Printf("[TrafficSwitching] 时间戳格式不正确，跳过")
			return true
		}

		// 如果超过 10 秒没有更新
		if currentTime.Sub(timestamp) > 20*time.Second {
			service, project := parseTrafficSwitchingLabel(metricLabel)

			if service != "" && project != "" {
				log.Printf("[TrafficSwitching] 实例失活，清理指标 -> service=%s project=%s", service, project)

				// 累计统计
				Metrics.TrafficSwitchingTotalRequests.DeleteLabelValues(service, project)
				Metrics.TrafficSwitchingTotalSuccess.DeleteLabelValues(service, project)
				Metrics.TrafficSwitchingTotalErrors.DeleteLabelValues(service, project)
				Metrics.TrafficSwitchingTotalSuccessRate.DeleteLabelValues(service, project)

				// 今日统计
				Metrics.TrafficSwitchingTodayRequests.DeleteLabelValues(service, project)
				Metrics.TrafficSwitchingTodaySuccess.DeleteLabelValues(service, project)
				Metrics.TrafficSwitchingTodayErrors.DeleteLabelValues(service, project)
				Metrics.TrafficSwitchingTodayCanceled.DeleteLabelValues(service, project)
				Metrics.TrafficSwitchingTodayStatus2xx.DeleteLabelValues(service, project)
				Metrics.TrafficSwitchingTodayStatus3xx.DeleteLabelValues(service, project)
				Metrics.TrafficSwitchingTodayStatus4xx.DeleteLabelValues(service, project)
				Metrics.TrafficSwitchingTodayStatus5xx.DeleteLabelValues(service, project)

				// 实时统计
				Metrics.TrafficSwitchingRealtimeQPS.DeleteLabelValues(service, project)
				Metrics.TrafficSwitchingRealtimeSuccessQPS.DeleteLabelValues(service, project)
				Metrics.TrafficSwitchingRealtimeErrorQPS.DeleteLabelValues(service, project)
				Metrics.TrafficSwitchingRealtimeActiveConnections.DeleteLabelValues(service, project)
				Metrics.TrafficSwitchingRealtimeAvgLatencyMs.DeleteLabelValues(service, project)
				Metrics.TrafficSwitchingRealtimeMaxLatencyMs.DeleteLabelValues(service, project)

				// 错误类型
				Metrics.TrafficSwitchingErrorBackendError.DeleteLabelValues(service, project)
				Metrics.TrafficSwitchingErrorBrokenPipe.DeleteLabelValues(service, project)
				Metrics.TrafficSwitchingErrorConnectionRefused.DeleteLabelValues(service, project)
				Metrics.TrafficSwitchingErrorConnectionReset.DeleteLabelValues(service, project)
				Metrics.TrafficSwitchingErrorDNSError.DeleteLabelValues(service, project)
				Metrics.TrafficSwitchingErrorEOF.DeleteLabelValues(service, project)
				Metrics.TrafficSwitchingErrorTimeout.DeleteLabelValues(service, project)

				// 缓存
				Metrics.TrafficSwitchingProxyCacheSize.DeleteLabelValues(service, project)
				Metrics.TrafficSwitchingProxyMaxCacheSize.DeleteLabelValues(service, project)

				// Runtime
				Metrics.TrafficSwitchingRuntimeGoroutines.DeleteLabelValues(service, project)
				Metrics.TrafficSwitchingRuntimeMemoryMB.DeleteLabelValues(service, project)
				Metrics.TrafficSwitchingRuntimeCPUCores.DeleteLabelValues(service, project)
				Metrics.TrafficSwitchingRuntimeGomaxprocs.DeleteLabelValues(service, project)
				Metrics.TrafficSwitchingRuntimeGcCycles.DeleteLabelValues(service, project)

				// Transport
				Metrics.TrafficSwitchingTransportMaxConnsPerHost.DeleteLabelValues(service, project)
				Metrics.TrafficSwitchingTransportMaxIdleConns.DeleteLabelValues(service, project)
				Metrics.TrafficSwitchingTransportMaxIdleConnsPerHost.DeleteLabelValues(service, project)

				// 时间戳
				Metrics.TrafficSwitchingTimestamp.DeleteLabelValues(service, project)
			} else {
				log.Printf("[TrafficSwitching] 标签 %s 格式不正确，跳过注销", metricLabel)
			}

			// 删除时间戳
			TrafficSwitchingTimestamp.Delete(metricLabel)
		}
		return true
	})
}

// UpdateTrafficSwitchingTimestamp 更新指标并记录时间戳
func UpdateTrafficSwitchingTimestamp(metricLabel string) {
	TrafficSwitchingTimestamp.Store(metricLabel, time.Now())
}
