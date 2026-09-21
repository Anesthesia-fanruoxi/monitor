// Package proto 是 agent 壳侧唯一允许读插件输出的文件。
// 壳对插件输出的全部认知被限制在这里：4 个 json tag，不再增加。
// Metrics 是 json.RawMessage，壳永不 Unmarshal 它，只测长度。
//
// 详见 docs/壳契约-v1.md §2.1 / §3
package proto

import (
	"encoding/json"
	"fmt"
	"strconv"
)

const (
	maxStdinLineBytes   = 8 * 1024 * 1024  // 单行 stdout 上限 8 MiB
	maxTotalOutputBytes = 32 * 1024 * 1024 // 单轮输出总量 32 MiB

	protoV1 = 1
)

// Envelope 壳眼里插件响应的全部。恰好 4 个 json tag。
type Envelope struct {
	ID      uint64          `json:"id"`
	OK      bool            `json:"ok"`
	Proto   int             `json:"proto"`
	Metrics json.RawMessage `json:"metrics"`
}

// Decode 宽容解析单行 JSON 为 Envelope。
// 只在"根本不是 JSON"或"metrics 超上限"时返回 error。
// 其余全部走 warn-once 降级：缺省值回落、字段类型宽容转换。
//
// 返回: env (永远可用), warnings (降级条目), err (致命错误)
func Decode(line []byte) (Env, []Warn, error) {
	if len(line) > maxStdinLineBytes {
		return Env{}, nil, fmt.Errorf("stdout 单行超限 %d > %d", len(line), maxStdinLineBytes)
	}

	var raw struct {
		ID      any             `json:"id"`
		OK      any             `json:"ok"`
		Proto   any             `json:"proto"`
		Metrics json.RawMessage `json:"metrics"`
	}
	if err := json.Unmarshal(line, &raw); err != nil {
		return Env{}, nil, fmt.Errorf("不是合法 JSON: %w", err)
	}

	var warns []Warn
	env := Envelope{Proto: protoV1}

	// id: 支持数字 / 字符串数字 / 缺失(默认 0)
	switch v := raw.ID.(type) {
	case float64:
		env.ID = uint64(v)
	case string:
		if n, err := strconv.ParseUint(v, 10, 64); err == nil {
			env.ID = n
		} else {
			warns = append(warns, Warn{Field: "id", Msg: fmt.Sprintf("无法解析字符串 %q", v)})
		}
	case nil:
		// 缺失，默认 0（壳一次只允许一个在途请求，按在途匹配）
	default:
		warns = append(warns, Warn{Field: "id", Msg: fmt.Sprintf("未知类型 %T", v)})
	}

	// ok: true/false/缺失(默认 false，保守)
	switch v := raw.OK.(type) {
	case bool:
		env.OK = v
	case nil:
		env.OK = false // 保守降级：宁可计一次失败
	default:
		env.OK = false
		warns = append(warns, Warn{Field: "ok", Msg: fmt.Sprintf("未知类型 %T，保守视为 false", v)})
	}

	// proto: 数字/缺失(默认 1)/>1 拒绝
	switch v := raw.Proto.(type) {
	case float64:
		env.Proto = int(v)
	case nil:
		env.Proto = protoV1 // 缺失视为 1
	default:
		env.Proto = protoV1
		warns = append(warns, Warn{Field: "proto", Msg: fmt.Sprintf("未知类型 %T，视为 %d", v, protoV1)})
	}
	if env.Proto > protoV1 {
		return Env{}, nil, fmt.Errorf("proto %d > %d，需升级 agent", env.Proto, protoV1)
	}

	// metrics: 缺失/空=无数据合法；超上限=error
	if len(raw.Metrics) == 0 {
		// 合法，ok:true 且无 metrics 是正常"这轮没采到"
	} else if len(raw.Metrics) > maxTotalOutputBytes {
		return Env{}, nil, fmt.Errorf("metrics 超限 %d > %d", len(raw.Metrics), maxTotalOutputBytes)
	}
	env.Metrics = raw.Metrics

	return Env{Envelope: env}, warns, nil
}

// Env 壳拿到解析结果后的只读视图。
// 壳只用到 OK 和 Metrics，不认识其它字段。
type Env struct {
	Envelope
}

// Warn 降级告警条目，每个插件每个字段只警告一次（warn-once 由上游做）。
type Warn struct {
	Field string
	Msg   string
}

// ===== 请求侧编码 =====
// 壳发给插件的请求只有三种：hello / collect / shutdown

func EncodeHello(id uint64) []byte {
	return jmarshal(map[string]any{"id": id, "cmd": "hello", "proto": protoV1})
}

func EncodeCollect(id uint64) []byte {
	return jmarshal(map[string]any{"id": id, "cmd": "collect"})
}

func EncodeShutdown(id uint64) []byte {
	return jmarshal(map[string]any{"id": id, "cmd": "shutdown"})
}

func jmarshal(v any) []byte {
	b, _ := json.Marshal(v)
	b = append(b, '\n')
	return b
}
