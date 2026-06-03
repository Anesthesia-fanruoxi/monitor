package Handers

import (
	"log"
	"monitor-server/Metrics"
	"sync"
	"time"
)

// 用来存储时间戳和指标名称的 sync.Map
var ContainerTimestamp sync.Map

// 反解析 label 字符串并更新数据
func parseContainerLabel(metricLabel string) (string, string, string, string, string) {
	parts := SplitLabels(metricLabel)
	if len(parts) < 5 {
		log.Printf("标签 %s 无法解析，格式不正确", metricLabel)
		return "", "", "", "", ""
	}
	return parts[0], parts[1], parts[2], parts[3], parts[4]
}

// 定期检查超时的心跳数据
func CheckContainerHeartbeats() {

	currentTime := time.Now()

	// 使用 Range 遍历所有存储的时间戳，检查是否有超过 10 秒未更新的数据
	ContainerTimestamp.Range(func(key, value interface{}) bool {
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

		// 如果超过 10 秒没有更新
		if currentTime.Sub(timestamp) > 20*time.Second {
			// 反解析 metricLabel 获取各个标签的值
			namespace, podName, container, controllerName, project := parseContainerLabel(metricLabel)

			// 如果标签解析成功，且字段不为空，则删除相应的指标
			if namespace != "" && podName != "" && container != "" && controllerName != "" && project != "" {
				// 删除对应的指标
				Metrics.ContainerCpuUsageMetric.DeleteLabelValues(namespace, podName, container, controllerName, project)
				Metrics.ContainerMemoryUsageMetric.DeleteLabelValues(namespace, podName, container, controllerName, project)
				Metrics.ContainerCpuLimitMetric.DeleteLabelValues(namespace, podName, container, controllerName, project)
				Metrics.ContainerMemoryLimitMetric.DeleteLabelValues(namespace, podName, container, controllerName, project)
				Metrics.ContainerRestartCountMetric.DeleteLabelValues(namespace, podName, container, controllerName, project)
				Metrics.ContainerLastTerminationTimeMetric.DeleteLabelValues(namespace, podName, container, controllerName, project)
			} else {
				log.Printf("标签 %s 格式不正确，跳过注销", metricLabel)
			}

			// 删除时间戳
			ContainerTimestamp.Delete(metricLabel)
		}
		return true
	})
}

// 更新指标并记录时间戳
func UpdateContainerMetricWithTimestamp(metricLabel string) {
	// 获取当前时间
	currentTime := time.Now()
	// 存储时间戳
	ContainerTimestamp.Store(metricLabel, currentTime)
}
