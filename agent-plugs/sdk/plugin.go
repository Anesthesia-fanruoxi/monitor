// Package sdk 是 monitor 插件协议的 Go SDK。
//
// 插件通过 RegisterMetric 注册指标声明，然后调用 Run()
// 启动 stdio JSON-RPC 主循环。SDK 负责协议编解码、命令分发
// 和 panic recover。
package sdk

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// ProtocolVersion 是本 SDK 支持的协议版本，与壳契约 v1 对齐。
const ProtocolVersion = PluginProtoV1

// MetricDecl 是一条指标声明。
type MetricDecl struct {
	Name    string
	Labels  []string
	Help    string
	Samples []Sample
}

// MetricManifest 是插件级自描述，用于 Describe 命令（预留）。
type MetricManifest struct {
	Name    string
	Version string
	Metrics []MetricDecl
}

var (
	pluginName    = "plugin"
	pluginVersion = "0.1.0"
	decls         []MetricDecl
)

// SetName 设置插件名（应在 init 或 main 早期调用）。
func SetName(name string) { pluginName = name }

// SetVersion 设置插件版本。
func SetVersion(v string) { pluginVersion = v }

// RegisterMetric 注册一条指标声明。
//
// name / labels / help 完全按 docs/插件设计.md §5 逐字填写；
// samples 是初始值占位，真实采集在 Collect 回调里填。
func RegisterMetric(name string, labels []string, help string, samples []Sample) {
	decls = append(decls, MetricDecl{
		Name:    name,
		Labels:  labels,
		Help:    help,
		Samples: samples,
	})
}

// Describe 返回插件名、版本和已注册的指标声明列表。
func Describe() MetricManifest {
	out := make([]MetricDecl, len(decls))
	copy(out, decls)
	return MetricManifest{
		Name:    pluginName,
		Version: pluginVersion,
		Metrics: out,
	}
}

// CollectFunc 是插件的采集回调，由插件在 Run 之前通过 SetCollect 注入。
// 返回的 []DeclaredMetric 会被序列化到 Envelope.Metrics。
// 回调里 panic 会被 SDK recover 并转为 ok:false 响应。
type CollectFunc func() []DeclaredMetric

var collectFn CollectFunc

// SetCollect 注入采集回调。插件必须在 Run 之前调用。
func SetCollect(fn CollectFunc) { collectFn = fn }

// Run 启动 stdio JSON-RPC 主循环。
//
// 协议约定：
//   - stdin 逐行读取 JSON 请求（json.Decoder）
//   - stdout 写 JSON 响应，每行一条（fmt.Fprintln）
//   - Collect 回调 panic 被 recover，自动回 ok:false
func Run() {
	dec := json.NewDecoder(os.Stdin)
	for {
		var req PluginRequest
		if err := dec.Decode(&req); err != nil {
			if err == io.EOF {
				return
			}
			// 无法解码：不知道 id，写一条空 id 错误响应
			writeError(0, ProtocolVersion, fmt.Sprintf("decode request: %v", err))
			continue
		}
		handle(&req)
	}
}

func handle(req *PluginRequest) {
	switch req.Cmd {
	case "hello":
		writeOK(req.ID, req.Proto, nil)
	case "collect":
		safeCollect(req.ID, req.Proto)
	case "shutdown":
		writeOK(req.ID, req.Proto, nil)
		os.Exit(0)
	default:
		writeError(req.ID, req.Proto, "unknown cmd: "+req.Cmd)
	}
}

func safeCollect(id uint64, proto int) {
	var (
		metrics  []DeclaredMetric
		panicked bool
		reason   string
	)
	defer func() {
		if r := recover(); r != nil {
			panicked = true
			reason = fmt.Sprintf("collect panic: %v", r)
		}
		if panicked {
			writeError(id, proto, reason)
			return
		}
		raw, err := json.Marshal(metrics)
		if err != nil {
			writeError(id, proto, fmt.Sprintf("marshal metrics: %v", err))
			return
		}
		writeRaw(id, proto, true, raw)
	}()

	if collectFn != nil {
		metrics = collectFn()
	}
}

// writeOK 写 ok:true 的响应；metrics 为 nil 时 Envelope.Metrics 是 null。
func writeOK(id uint64, proto int, metrics []DeclaredMetric) {
	var raw json.RawMessage
	if metrics != nil {
		b, err := json.Marshal(metrics)
		if err != nil {
			writeError(id, proto, err.Error())
			return
		}
		raw = b
	}
	writeRaw(id, proto, true, raw)
}

func writeError(id uint64, proto int, reason string) {
	// ok:false 时 Metrics 必须是空数组（壳契约要求）
	raw, _ := json.Marshal([]DeclaredMetric{})
	writeRaw(id, proto, false, raw)
	// 同时给 stderr 一条原因，方便排查
	Fprintln("[sdk] ", reason)
}

func writeRaw(id uint64, proto int, ok bool, raw json.RawMessage) {
	env := PluginEnvelope{
		ID:      id,
		OK:      ok,
		Proto:   proto,
		Metrics: raw,
	}
	b, err := json.Marshal(env)
	if err != nil {
		// 最后兜底：协议层自己 marshal 失败，只能打 stderr
		Fprintln("[sdk] marshal envelope failed: ", err)
		return
	}
	fmt.Fprintln(os.Stdout, string(b))
}
