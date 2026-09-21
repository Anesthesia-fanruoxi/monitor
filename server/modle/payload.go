package modle

import "encoding/json"

// SendPayload Agent 上报报文的顶层结构
// Data 用 json.RawMessage 保留原始 JSON 字节，各 source 对应的 Handle 函数自行做 typed decode
//
// Metrics 是声明式上报（插件化 Agent）的字段，与 Data 二选一：
// 用 RawMessage 而不是 []DeclaredMetric，是为了让 metrics 字段的格式错误
// 只影响声明式路径，不会把旧格式的请求一起判为 400。
type SendPayload struct {
	Project   string          `json:"project"`
	Source    string          `json:"source"`
	Timestamp int64           `json:"timestamp"` // 毫秒
	Data      json.RawMessage `json:"data"`      // 原始 JSON 数组（旧路径，已废弃）
	Schema    string          `json:"schema"`    // 声明式协议的版本标识（新路径）
	Metrics   json.RawMessage `json:"metrics"`   // 指标自我描述 + 采样值（新路径）
}
