// Package store 维护服务端对外暴露的 Prometheus 注册表与动态指标生命周期。
//
// 旧静态指标已全部移除，注册表只包含动态插件化 Agent 上报的指标。
// 防护清单见包注释：所有限制都是为了"不让上报方决定内存占用"。
package store

import (
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"monitor-server/modle"

	"github.com/prometheus/client_golang/prometheus"
)

// CustomRegistry 服务端对外暴露的唯一 Prometheus 注册表
var CustomRegistry = prometheus.NewRegistry()

// 防护清单：
//  1. 指标名/标签名做正则校验，且拒绝 Prometheus 保留前缀 __
//  2. project 标签由服务端注入，插件声明同名标签直接拒绝
//  3. 单指标时间序列基数上限（MaxSeriesPerMetric）
//  4. 指标名全局唯一，且跨 source 不允许重名
//  5. 动态指标总量上限
//  6. 注销阈值有下限
//  7. 全局序列总数上限（02-S5）
//  8. 单次上报采样总数上限（02-S5）
const (
	MaxMetricNameLen    = 200
	MaxMetricHelpLen    = 1024
	MaxLabelsPerMetric  = 32
	MaxSamplesPerMetric = 20000
	MaxMetricsPerReport = 64
	MaxSeriesPerMetric  = 50000
	MaxDynamicMetrics   = 2000

	// MinTimeout 注销阈值下限：低于这个值时间序列会在两次采集之间被摘掉
	MinTimeout = 20 * time.Second
	MaxTimeout = 24 * time.Hour
	// DefaultTimeout 插件未声明 timeout 时的兜底值
	DefaultTimeout = 45 * time.Second
)

// seriesSep 用于把标签值拼成时间序列的唯一 key
// 用不可见字符分隔，避免与标签值本身的内容冲突
const seriesSep = "\x1f"

// DynamicMetricSpec 一个动态指标的"形状"描述
type DynamicMetricSpec struct {
	Name    string
	Help    string
	Type    string
	Labels  []string // 业务标签，不含 project
	Timeout time.Duration
}

// Sample 一条采样值
type Sample struct {
	Labels map[string]string
	Value  float64
}

type seriesEntry struct {
	values   []string
	lastSeen time.Time
}

// DynMetric 一个已注册的动态指标
type DynMetric struct {
	Name     string
	Source   string
	Help     string
	Labels   []string // 只读快照，实际以持锁后的 m.labels 为准
	Timeout  time.Duration
	Registry *prometheus.Registry

	gauge *prometheus.GaugeVec

	// mu 保护 labels / timeout / series / warnedFull
	// 锁顺序约定：dynMu → DynMetric.mu，不可反向获取
	mu         sync.RWMutex
	labels     []string
	timeout    time.Duration
	series     map[string]*seriesEntry
	warnedFull bool
}

var (
	dynMu          sync.RWMutex
	dynamicMetrics = map[string]*DynMetric{} // 指标名 → 定义
)

// EnsureMetric 确保指标已按 spec 注册，返回可用的动态指标
func EnsureMetric(reg *prometheus.Registry, source string, spec DynamicMetricSpec) (*DynMetric, error) {
	if reg == nil {
		return nil, errors.New("registry 为空")
	}
	if err := validateSpec(spec); err != nil {
		return nil, err
	}
	if spec.Timeout <= 0 {
		spec.Timeout = DefaultTimeout
	}

	// 快路径：已存在且形状一致，只更新阈值
	dynMu.RLock()
	existing, ok := dynamicMetrics[spec.Name]
	if ok && existing.Source == source && labelsEqual(existing.snapshotLabels(), spec.Labels) {
		existing.setTimeout(spec.Timeout)
		dynMu.RUnlock()
		return existing, nil
	}
	dynMu.RUnlock()

	dynMu.Lock()
	defer dynMu.Unlock()

	if existing, ok := dynamicMetrics[spec.Name]; ok {
		if existing.Source != source {
			return nil, fmt.Errorf("指标 %s 已由 source %s 注册，source %s 不能重复注册",
				spec.Name, existing.Source, source)
		}
		if !labelsEqual(existing.snapshotLabels(), spec.Labels) {
			log.Printf("指标 %s 的标签集合发生变化: %v → %v，重新注册该指标（旧时间序列将被清空）",
				spec.Name, existing.snapshotLabels(), spec.Labels)
			if err := existing.reRegisterLocked(spec); err != nil {
				return nil, err
			}
		}
		existing.setTimeoutLocked(spec.Timeout)
		return existing, nil
	}

	if len(dynamicMetrics) >= MaxDynamicMetrics {
		return nil, fmt.Errorf("动态指标数量已达上限 %d，拒绝注册 %s", MaxDynamicMetrics, spec.Name)
	}

	gauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{Name: spec.Name, Help: spec.Help},
		fullLabelNames(spec.Labels),
	)

	if err := reg.Register(gauge); err != nil {
		if _, already := err.(prometheus.AlreadyRegisteredError); already {
			return nil, fmt.Errorf("指标 %s 与服务端已注册的指标重名，请更换指标名", spec.Name)
		}
		return nil, fmt.Errorf("注册指标 %s 失败: %v", spec.Name, err)
	}

	dm := &DynMetric{
		Name:     spec.Name,
		Source:   source,
		Help:     spec.Help,
		Labels:   spec.Labels,
		Timeout:  spec.Timeout,
		Registry: reg,
		gauge:    gauge,
		labels:   spec.Labels,
		timeout:  spec.Timeout,
		series:   make(map[string]*seriesEntry, 16),
	}
	dynamicMetrics[spec.Name] = dm
	log.Printf("已注册动态指标 %s（source=%s 标签=%v 注销阈值=%s）",
		spec.Name, source, fullLabelNames(spec.Labels), spec.Timeout)

	return dm, nil
}

// fullLabelNames 返回完整标签名列表，project 固定在第 0 位
func fullLabelNames(business []string) []string {
	out := make([]string, 0, len(business)+1)
	out = append(out, modle.ProjectLabel)
	out = append(out, business...)
	return out
}

// reRegisterLocked 在标签集合变化时重建 GaugeVec
// 调用方需持有 dynMu 写锁
func (m *DynMetric) reRegisterLocked(spec DynamicMetricSpec) error {
	// 02-S6: 记录重建次数
	IngestIncReregisterSuppress()

	newGauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{Name: spec.Name, Help: spec.Help},
		fullLabelNames(spec.Labels),
	)

	if !m.Registry.Unregister(m.gauge) {
		return fmt.Errorf("注销旧指标 %s 失败：该指标不在注册表中", spec.Name)
	}
	if err := m.Registry.Register(newGauge); err != nil {
		if rollbackErr := m.Registry.Register(m.gauge); rollbackErr != nil {
			log.Printf("指标 %s 重新注册失败且回滚也失败: %v", spec.Name, rollbackErr)
		}
		return fmt.Errorf("重新注册指标 %s 失败: %v", spec.Name, err)
	}

	m.mu.Lock()
	// 02-S5: 标签集合变了，旧序列整体作废，全局序列计数相应扣减
	totalSeries.Add(int64(-len(m.series)))
	m.gauge = newGauge
	m.labels = spec.Labels
	m.Labels = spec.Labels
	m.Help = spec.Help
	m.series = make(map[string]*seriesEntry, 16)
	m.warnedFull = false
	m.mu.Unlock()
	return nil
}

func (m *DynMetric) snapshotLabels() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]string, len(m.labels))
	copy(out, m.labels)
	return out
}

func (m *DynMetric) setTimeout(d time.Duration) {
	m.mu.Lock()
	m.timeout = d
	m.mu.Unlock()
}

// setTimeoutLocked 调用方需持有 dynMu 写锁
func (m *DynMetric) setTimeoutLocked(d time.Duration) {
	m.mu.Lock()
	m.timeout = d
	m.Timeout = d
	m.mu.Unlock()
}

func labelsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
