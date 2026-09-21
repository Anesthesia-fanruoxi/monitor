package store

import (
	"fmt"
	"log"
	"time"
)

// SweepDynamicMetrics 遍历所有动态指标执行超时注销，返回摘除的序列总数
func SweepDynamicMetrics() int {
	dynMu.RLock()
	list := make([]*DynMetric, 0, len(dynamicMetrics))
	for _, m := range dynamicMetrics {
		list = append(list, m)
	}
	dynMu.RUnlock()

	now := time.Now()
	removed := 0
	for _, m := range list {
		removed += m.sweep(now)
	}
	return removed
}

// StartDynamicSweeper 启动动态指标的超时注销循环
// interval 取各指标阈值的最小值量级即可：这里的所有注销阈值都有 20s 下限，
// 所以 5 秒扫一次的精度足够，开销也只是遍历指标定义（不遍历时间序列以外的数据）
func StartDynamicSweeper(interval time.Duration, stop <-chan struct{}) {
	if interval <= 0 {
		interval = 5 * time.Second
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				if n := SweepDynamicMetrics(); n > 0 {
					log.Printf("动态指标超时注销: 摘除 %d 条时间序列", n)
				}
			}
		}
	}()
}

// DynamicMetricsSnapshot 返回当前动态指标的概要，供诊断/日志使用
func DynamicMetricsSnapshot() []string {
	dynMu.RLock()
	defer dynMu.RUnlock()

	out := make([]string, 0, len(dynamicMetrics))
	for name, m := range dynamicMetrics {
		m.mu.RLock()
		out = append(out, fmt.Sprintf("%s(source=%s 标签=%v 序列=%d 阈值=%s)",
			name, m.Source, m.labels, len(m.series), m.timeout))
		m.mu.RUnlock()
	}
	return out
}
