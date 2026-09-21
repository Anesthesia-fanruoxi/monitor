package sdk

import "encoding/json"

// ===== 壳 ↔ 插件 的 Envelope（stdio JSON-RPC，冻结）=====

// PluginEnvelope 是壳对插件响应的全部认知。
// 严格 4 个 json tag，不再增加。Metrics 是 json.RawMessage，
// 壳永不 Unmarshal 它，只测长度和可选算 sha256。
// 详见 docs/壳契约-v1.md §2.1
type PluginEnvelope struct {
	ID      uint64          `json:"id"`
	OK      bool            `json:"ok"`
	Proto   int             `json:"proto"`
	Metrics json.RawMessage `json:"metrics"`
}

// PluginRequest 壳发给插件的请求。
type PluginRequest struct {
	ID     uint64 `json:"id"`
	Cmd    string `json:"cmd"` // "hello" | "collect" | "shutdown"
	Proto  int    `json:"proto"`
	Config string `json:"config,omitempty"` // 可选，合并后的插件配置 JSON
}

// ===== Agent ↔ Server 的声明式上报 Payload =====

// DeclarativePayload Agent 壳发给 Server 的声明式上报。
// 这是 schema v2，与旧 JSON (schema="v1") 完全并存于同一条路径。
type DeclarativePayload struct {
	Project   string           `json:"project"`
	Source    string           `json:"source"`
	Timestamp int64            `json:"timestamp"` // 毫秒
	Schema    string           `json:"schema"`    // "v2"
	Metrics   []DeclaredMetric `json:"metrics"`
}

// DeclaredMetric 一个声明式指标。
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

// 常量
const (
	MetricTypeGauge = "gauge"
	PluginProtoV1   = 1
)
