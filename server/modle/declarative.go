package modle

// ===== 声明式上报协议（插件化 Agent）=====
//
// 背景：Agent 拆成"壳 + 采集器插件"后，新增采集器不应该要求改服务端代码。
// 因此插件在上报时自带指标的完整定义（名称、说明、标签、注销阈值），
// 服务端据此动态注册指标；而不是像旧 data 路径那样，每个 source 在服务端
// 硬编码一个 Handle*Data 函数和一批 GaugeVec。
//
// 与旧协议的关系：两条路径并存，靠顶层有没有 metrics 字段区分。
//   - 有 metrics  → 声明式路径（本文件定义）
//   - 只有 data   → 旧路径，按 source 分发给硬编码 handler
// 这样新老 Agent 可以同时在线，迁移可以逐个采集器做。

// SchemaV2 声明式上报的协议版本标识
const SchemaV2 = "v2"

// MetricTypeGauge 目前仅支持 gauge
//
// 为什么不支持 counter：本系统所有时间序列都靠"超时注销"清理，同一个
// label 组合在主机下线后会被 DeleteLabelValues 摘掉；如果语义是 counter，
// 它再次出现时会从 0 重新计数，Prometheus 侧会算出一个错误的 rate 尖峰。
const MetricTypeGauge = "gauge"

// ProjectLabel 由服务端统一注入的标签名
//
// 插件不得自行声明这个标签：project 的取值来自配置，并且服务端要把它映射成
// 中文项目名，这两件事都必须由服务端统一做，否则每个插件都要重复实现一遍，
// 还会出现同一项目在指标里写成不同值的情况。
const ProjectLabel = "project"

// DeclarativePayload 声明式上报的顶层结构
type DeclarativePayload struct {
	Project   string           `json:"project"`
	Source    string           `json:"source"`
	Timestamp int64            `json:"timestamp"` // 毫秒
	Schema    string           `json:"schema"`    // 期望 "v2"
	Metrics   []DeclaredMetric `json:"metrics"`
}

// DeclaredMetric 一个指标的自我描述 + 本轮采样值
type DeclaredMetric struct {
	Name string `json:"name"` // 指标名，全局唯一
	Help string `json:"help"` // 指标说明，写入 /metrics 的 HELP 行
	Type string `json:"type"` // 目前仅支持 "gauge"

	// Labels 声明该指标的业务标签名（不含 project，project 由服务端注入）
	//
	// 标签值通过 map 传递而不是数组：数组要求发送方和接收方的顺序严格一致，
	// 一旦错位就是静默的数据错乱；map 由服务端按 Labels 的顺序取值，天然免疫。
	Labels []string `json:"labels"`

	// Common 是所有 sample 共享的标签值，用于消除重复
	//
	// 例如 hard 插件每个采样值都带 cpu_model/os_version/kernel_version，
	// 这些在一轮里是常量，放进 Common 后单个 sample 只需带变化的部分，
	// 能显著压小载荷（K8s 那类数据的标签重复尤其严重）。
	Common map[string]string `json:"common,omitempty"`

	// Timeout 该类数据的注销阈值（Go duration 格式，如 "45s"）
	//
	// 必须大于插件的采集周期，否则时间序列会在两次上报之间被清掉，
	// Prometheus 抓到的就是断点。建议取 3 倍采集周期。
	Timeout string `json:"timeout"`

	Samples []MetricSample `json:"samples"`
}

// MetricSample 单个时间序列的取值
type MetricSample struct {
	// Labels 是本条样本的标签值（可以不包含 Common 里已有的键）
	Labels map[string]string `json:"labels,omitempty"`
	Value  float64           `json:"value"`
}

// 协议常量：服务端在响应里回传的拒绝原因长度上限，避免把大段文本写回给上报端
const MaxRejectReasonLen = 200
