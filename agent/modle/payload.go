// Package modle agent 数据模型。
// DeclarativePayload 是 Agent→Server 的声明式上报体（schema v2），
// 与 server/modle/declarative.go 镜像，字段与 json tag 冻结。
// agent 是转发壳：只组装信封（Project/Source/Timestamp/Schema），
// 不理解也不产出指标语义。
package modle

import "encoding/json"

// DeclarativePayload Agent 壳发给 Server 的声明式上报。
// Metrics 是插件输出的原样 JSON 字节（DeclaredMetric 数组）：
// agent 不解析、不合并、零改动，仅组装信封后压缩加密转发。
type DeclarativePayload struct {
	Project   string          `json:"project"`
	Source    string          `json:"source"`
	Timestamp int64           `json:"timestamp"` // 毫秒
	Schema    string          `json:"schema"`    // "v2"
	Metrics   json.RawMessage `json:"metrics"`
}

// DeclaredMetric 一个声明式指标（镜像插件侧 pkg/plugproto 定义，仅壳心跳构造使用）。
type DeclaredMetric struct {
	Name    string            `json:"name"`
	Help    string            `json:"help"`
	Type    string            `json:"type"` // "gauge"
	Labels  []string          `json:"labels"`
	Common  map[string]string `json:"common,omitempty"`
	Timeout string            `json:"timeout,omitempty"` // "45s"
	Samples []Sample          `json:"samples"`
}

// Sample 一条采样值。
type Sample struct {
	Labels map[string]string `json:"labels"`
	Value  float64           `json:"value"`
}

const MetricTypeGauge = "gauge"
