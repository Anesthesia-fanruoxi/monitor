package Handers

import (
	"log"
	"monitor-server/Metrics"
	"sync"
	"time"
)

var sslTimestamp = sync.Map{}

// 反解析 label 字符串并更新 SSL 数据
func parseSSLLabel(metricLabel string) (string, string, string, string, string) {
	parts := SplitLabels(metricLabel)
	if len(parts) < 5 {
		log.Printf("标签 %s 无法解析，格式不正确", metricLabel)
		return "", "", "", "", ""
	}
	return parts[0], parts[1], parts[2], parts[3], parts[4]
}

// 定期检查超时的 SSL 数据
func CheckSSLHeartbeats() {

	currentTime := time.Now()
	// 使用 Range 遍历所有存储的时间戳，检查是否有超过 10 秒未更新的数据
	sslTimestamp.Range(func(key, value interface{}) bool {
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
			domain, comment, status, resolve, projectName := parseSSLLabel(metricLabel)

			if domain != "" && comment != "" && status != "" && resolve != "" {

				// 删除对应的 SSL 指标
				Metrics.SslDaysLeftMetric.DeleteLabelValues(domain, comment, status, resolve, projectName)

				// 删除时间戳
				sslTimestamp.Delete(metricLabel) // 删除时间戳
			} else {
				log.Printf("标签 %s 格式不正确，跳过注销", metricLabel)
			}
		}
		return true
	})
}
