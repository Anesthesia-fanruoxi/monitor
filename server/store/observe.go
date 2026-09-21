package store

import (
	"log"
	"strings"
	"time"
)

// Observe 写入一批采样值，返回写入条数与因基数超限被丢弃的条数
func (m *DynMetric) Observe(project string, samples []Sample) (written, dropped int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for _, s := range samples {
		values := make([]string, 0, len(m.labels)+1)
		values = append(values, project)
		for _, name := range m.labels {
			values = append(values, s.Labels[name])
		}
		key := strings.Join(values, seriesSep)

		entry, exists := m.series[key]
		if !exists {
			// 基数上限：新序列必须受控，否则插件只要不断变化标签值就能吃光内存
			if len(m.series) >= MaxSeriesPerMetric {
				dropped++
				if !m.warnedFull {
					m.warnedFull = true
					log.Printf("指标 %s 的时间序列已达上限 %d，本次新增序列被丢弃（后续同类丢弃不再重复告警）",
						m.Name, MaxSeriesPerMetric)
				}
				continue
			}
			entry = &seriesEntry{values: values}
			m.series[key] = entry
			// 02-S5: 全局序列闸门计数
			totalSeries.Add(1)
		} else {
			// 标签值可能与首次出现时不同（Common 缺省导致），以首次的为准并保持 values 一致
			values = entry.values
		}

		m.gauge.WithLabelValues(values...).Set(s.Value)
		entry.lastSeen = now
		written++
	}
	return written, dropped
}

// sweep 摘掉超过 timeout 未更新过的时间序列
func (m *DynMetric) sweep(now time.Time) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	var expired []string
	for key, entry := range m.series {
		if now.Sub(entry.lastSeen) > m.timeout {
			expired = append(expired, key)
		}
	}
	for _, key := range expired {
		entry := m.series[key]
		if len(entry.values) > 0 {
			m.gauge.DeleteLabelValues(entry.values...)
		} else {
			m.gauge.Reset()
		}
		delete(m.series, key)
	}
	if len(expired) > 0 {
		// 02-S5: 全局序列闸门扣减
		totalSeries.Add(int64(-len(expired)))
	}
	// 序列清空后允许再次告警基数超限
	if len(m.series) < MaxSeriesPerMetric {
		m.warnedFull = false
	}
	return len(expired)
}
