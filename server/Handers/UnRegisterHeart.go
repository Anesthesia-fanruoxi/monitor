package Handers

import (
	"log"
	"monitor-server/Metrics"
	"sync"
	"time"
)

var agentHeartbeatTimes = sync.Map{}

// 反解析 label 字符串并获取 hostname 和 project
func parseHeartbeatLabel(metricLabel string) (string, string) {
	parts := SplitLabels(metricLabel)
	if len(parts) < 2 {
		log.Printf("标签 %s 无法解析，格式不正确", metricLabel)
		return "", ""
	}
	return parts[0], parts[1]
}

// 定期检查超时的心跳数据
func CheckHeartbeats() {
	currentTime := time.Now()

	// 遍历 agentHeartbeatTimes 以检查每个 agent 的心跳状态
	agentHeartbeatTimes.Range(func(key, value interface{}) bool {
		metricLabel, ok := key.(string)
		if !ok {
			log.Printf("标签格式不正确，跳过")
			return true
		}

		timestamp, ok := value.(time.Time)
		if !ok {
			log.Printf("时间戳格式不正确，跳过")
			return true
		}

		// 反解析 metricLabel 获取 hostname 和 project
		hostname, project := parseHeartbeatLabel(metricLabel)

		// 如果超过 10 秒没有接收到心跳
		if currentTime.Sub(timestamp) > 20*time.Second {
			// 设置 IsActive 为 0，表示该 agent 已经不活跃
			Metrics.IsActiveMetric.WithLabelValues(hostname, project).Set(0)
		}
		return true // 确保继续遍历所有的键值对
	})
}
