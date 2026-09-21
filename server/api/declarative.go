package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"monitor-server/modle"
	"monitor-server/common"
	"monitor-server/store"

	"github.com/prometheus/client_golang/prometheus"
)

// ===== 声明式上报的处理链路 =====
//
// 与旧 data 路径最大的区别是"校验时机"：
//   旧路径：先返回 200，再异步分发。上报端拿不到任何错误，字段写错就是数据静默消失。
//   声明式路径：校验与指标注册在响应之前完成，失败返回 400 并带上具体原因。
//
// 之所以能这么做，是因为校验本身很轻（正则 + map 查找），真正的重活（写值、维护
// 时间序列）仍然异步交给 worker pool。插件由运维自己发布，让作者第一时间看到
// "指标名非法""标签未声明"这类错误，比事后翻日志排查划算得多。

// dynamicSourceRe 声明式路径的 source 格式校验
var dynamicSourceRe = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]{0,31}$`)

// PrepareDeclarative 校验声明式上报并完成指标注册，返回实际写值的闭包
//
// 返回闭包而不是直接写入，是为了让调用方"同步拿错误、异步做落库"。
func PrepareDeclarative(dp modle.DeclarativePayload, reg *prometheus.Registry) (func(), error) {
	if dp.Schema != modle.SchemaV2 {
		return nil, fmt.Errorf("不支持的协议版本 %q，期望 %q", dp.Schema, modle.SchemaV2)
	}
	if !dynamicSourceRe.MatchString(dp.Source) {
		return nil, fmt.Errorf("source %q 格式非法（只允许字母开头的字母/数字/下划线，长度 1-32）", dp.Source)
	}
	if len(dp.Metrics) == 0 {
		return nil, fmt.Errorf("metrics 不能为空")
	}
	if len(dp.Metrics) > store.MaxMetricsPerReport {
		return nil, fmt.Errorf("单次上报的指标数量超过 %d", store.MaxMetricsPerReport)
	}

	type job struct {
		metric  *store.DynMetric
		samples []store.Sample
	}

	// 先收集所有 spec 与采样，再做全局闸门检查，最后批量注册
	specs := make([]store.DynamicMetricSpec, 0, len(dp.Metrics))
	sampleBatches := make([][]store.Sample, 0, len(dp.Metrics))
	totalSamples := 0

	for i := range dp.Metrics {
		spec, samples, err := convertDeclaredMetric(dp.Metrics[i])
		if err != nil {
			return nil, fmt.Errorf("第 %d 个指标: %v", i+1, err)
		}
		specs = append(specs, spec)
		sampleBatches = append(sampleBatches, samples)
		totalSamples += len(samples)
	}

	// 全局序列闸门
	if err := store.CheckGlobalGate(specs, totalSamples); err != nil {
		return nil, err
	}

	jobs := make([]job, 0, len(specs))
	for i := range specs {
		dm, err := store.EnsureMetric(reg, dp.Source, specs[i])
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job{metric: dm, samples: sampleBatches[i]})
	}

	// project 标签存上报方原始值（英文缩写，如 jxh），中文展示由展示层负责
	projectLabel := dp.Project
	source := dp.Source

	return func() {
		written, dropped := 0, 0
		for _, j := range jobs {
			w, d := j.metric.Observe(projectLabel, j.samples)
			written += w
			dropped += d
		}
		if dropped > 0 {
			log.Printf("声明式上报部分采样被丢弃: source=%s project=%s 写入=%d 丢弃=%d",
				source, dp.Project, written, dropped)
		}
	}, nil
}

// convertDeclaredMetric 把上报的指标描述转成注册用的 spec 与采样值
func convertDeclaredMetric(m modle.DeclaredMetric) (store.DynamicMetricSpec, []store.Sample, error) {
	timeout := store.DefaultTimeout
	if t := strings.TrimSpace(m.Timeout); t != "" {
		d, err := parseDuration(t)
		if err != nil {
			return store.DynamicMetricSpec{}, nil, fmt.Errorf("timeout %q 不是合法的时长（如 45s / 3m）", m.Timeout)
		}
		timeout = d
	}

	if len(m.Samples) > store.MaxSamplesPerMetric {
		return store.DynamicMetricSpec{}, nil, fmt.Errorf("采样条数 %d 超过上限 %d", len(m.Samples), store.MaxSamplesPerMetric)
	}

	// 标签必须先声明再使用：写错了要立刻报错，而不是静默忽略
	declared := make(map[string]struct{}, len(m.Labels))
	for _, name := range m.Labels {
		declared[name] = struct{}{}
	}

	for k, v := range m.Common {
		if k == modle.ProjectLabel {
			return store.DynamicMetricSpec{}, nil, fmt.Errorf("标签 %s 由服务端注入，不得出现在 common 中", modle.ProjectLabel)
		}
		if _, ok := declared[k]; !ok {
			return store.DynamicMetricSpec{}, nil, fmt.Errorf("common 中的标签 %q 未在 labels 中声明", k)
		}
		if len(v) > MaxLabelValueLen {
			return store.DynamicMetricSpec{}, nil, fmt.Errorf("标签 %q 的值长度超过 %d 字节", k, MaxLabelValueLen)
		}
	}

	samples := make([]store.Sample, 0, len(m.Samples))
	for _, s := range m.Samples {
		merged := make(map[string]string, len(m.Common)+len(s.Labels))
		for k, v := range m.Common {
			merged[k] = v
		}
		for k, v := range s.Labels {
			if k == modle.ProjectLabel {
				return store.DynamicMetricSpec{}, nil, fmt.Errorf("标签 %s 由服务端注入，不得出现在样本中", modle.ProjectLabel)
			}
			if _, ok := declared[k]; !ok {
				return store.DynamicMetricSpec{}, nil, fmt.Errorf("样本中的标签 %q 未在 labels 中声明", k)
			}
			if len(v) > MaxLabelValueLen {
				return store.DynamicMetricSpec{}, nil, fmt.Errorf("标签 %q 的值长度超过 %d 字节", k, MaxLabelValueLen)
			}
			merged[k] = v
		}
		samples = append(samples, store.Sample{Labels: merged, Value: s.Value})
	}

	return store.DynamicMetricSpec{
		Name:    m.Name,
		Help:    m.Help,
		Type:    m.Type,
		Labels:  m.Labels,
		Timeout: timeout,
	}, samples, nil
}

// HandleDeclarativePayload 声明式上报的完整处理入口
//
// 校验与注册在响应之前完成，失败返回 400 并带上具体原因；
// 写值（重活）异步交给 worker pool。
func HandleDeclarativePayload(w http.ResponseWriter, payload modle.SendPayload, reg *prometheus.Registry) {
	if payload.Source == "" {
		writeJSONError(w, http.StatusBadRequest, "缺少 source 字段")
		return
	}
	if err := validateTimestamp(payload.Timestamp); err != nil {
		log.Printf("拒绝声明式上报 source=%s project=%s: %v", payload.Source, payload.Project, err)
		writeJSONError(w, http.StatusBadRequest, "上报时间戳无效")
		return
	}

	dp := modle.DeclarativePayload{
		Project:   payload.Project,
		Source:    payload.Source,
		Timestamp: payload.Timestamp,
		Schema:    payload.Schema,
	}
	if err := json.Unmarshal(payload.Metrics, &dp.Metrics); err != nil {
		writeJSONError(w, http.StatusBadRequest, "metrics 字段格式错误")
		return
	}

	// 同步完成校验与注册：插件作者必须能立刻看到失败原因
	task, err := PrepareDeclarative(dp, reg)
	if err != nil {
		log.Printf("声明式上报被拒绝 source=%s project=%s: %v", payload.Source, payload.Project, err)
		writeJSONError(w, http.StatusBadRequest, "上报校验失败: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]interface{}{"code": 200, "msg": "ok"}); err != nil {
		log.Printf("响应失败: %v", err)
	}

	// 队列满时丢弃 + 计数（由 common.Submit 内部回调处理），不 fallback 到 inline 执行
	if !common.Submit(task) {
		log.Printf("worker pool 队列已满，丢弃上报 source=%s project=%s", payload.Source, payload.Project)
	}

	// 记录成功接收
	store.IngestIncReceived(payload.Source)
}

// parseDuration 解析时长字符串
// 单独包一层是为了将来需要支持 "1d" 这类 time.ParseDuration 不认的单位
func parseDuration(s string) (time.Duration, error) {
	return time.ParseDuration(s)
}
