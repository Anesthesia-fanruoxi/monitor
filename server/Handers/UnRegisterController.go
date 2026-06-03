package Handers

import (
	"log"
	"monitor-server/Metrics"
	"sync"
	"time"
)

// 用来存储时间戳和指标名称的 map
var ControllerTimestamp = sync.Map{}

// 反解析 label 字符串并更新数据
func parseControllerLabel(metricLabel string) (string, string, string, string) {
	parts := SplitLabels(metricLabel)
	if len(parts) < 4 {
		log.Printf("标签 %s 无法解析，格式不正确", metricLabel)
		return "", "", "", ""
	}
	return parts[0], parts[1], parts[2], parts[3]
}

// 定期检查超时的心跳数据
func CheckControllerHeartbeats() {

	currentTime := time.Now()

	// 使用 Range 遍历所有存储的时间戳，检查是否有超过 10 秒未更新的数据
	ControllerTimestamp.Range(func(key, value interface{}) bool {
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
		if currentTime.Sub(timestamp) > 20*time.Second {
			// 反解析 metricLabel 获取各个标签的值
			namespace, continer, ControllerType, project := parseControllerLabel(metricLabel)

			// 如果标签解析成功，且字段不为空，则删除相应的指标
			if namespace != "" && continer != "" && ControllerType != "" && project != "" {
				// 删除对应的指标
				Metrics.ControllerReplicasMetric.DeleteLabelValues(namespace, continer, ControllerType, project)
				Metrics.ControllerReplicasAvailableMetric.DeleteLabelValues(namespace, continer, ControllerType, project)
				Metrics.ControllerReplicasUnavailableMetric.DeleteLabelValues(namespace, continer, ControllerType, project)
			} else {
				log.Printf("标签 %s 格式不正确，跳过注销", metricLabel)
			}

			// 删除时间戳
			ControllerTimestamp.Delete(metricLabel)
		}
		return true
	})
}

// 更新指标并记录时间戳
func UpdateControllerMetricWithTimestamp(metricLabel string) {
	// 获取当前时间
	currentTime := time.Now()
	// 存储时间戳
	ControllerTimestamp.Store(metricLabel, currentTime)
}
