package store

import (
	"fmt"
	"sync/atomic"
)

// 全局序列闸门
//
// 单指标的基数上限（MaxSeriesPerMetric）只防住了"一个指标内部打爆"，
// 但插件可以声明几十个指标，每个各打几万条序列，总量照样能把内存吃光。
// 这两个常量从全局维度兜住"所有动态指标的序列总数"和"单次上报的采样总数"。
const (
	// MaxSeriesTotal 所有动态指标的活跃时间序列总数上限
	MaxSeriesTotal = 200000
	// MaxSamplesTotalPerReport 单次声明式上报允许的采样值总数上限
	MaxSamplesTotalPerReport = 200000
)

// totalSeries 当前所有动态指标的活跃时间序列总数
//
// 用 atomic 而非 dynMu 保护：Observe/sweep 在 DynMetric.mu 下增删序列，
// 若再获取 dynMu 会违反 dynMu→DynMetric.mu 的加锁顺序，因此用无锁原子计数。
var totalSeries atomic.Int64

// CheckGlobalGate 在 EnsureMetric 之前做粗粒度准入检查
//
// specs 保留给将来按"本次上报可能新增的序列数"做更精细的预估；
// 当前实现只看已存在的全局序列数与本次采样总数。
func CheckGlobalGate(specs []DynamicMetricSpec, totalSamples int) error {
	current := totalSeries.Load()
	if current >= MaxSeriesTotal {
		return fmt.Errorf("全局时间序列数 %d 已达上限 %d，拒绝本次上报", current, MaxSeriesTotal)
	}
	if totalSamples > MaxSamplesTotalPerReport {
		return fmt.Errorf("单次上报采样条数 %d 超过上限 %d", totalSamples, MaxSamplesTotalPerReport)
	}
	return nil
}
