package sdk

import (
	"encoding/json"
	"os"
)

// shellReservedKeys 是壳保留的三个键，插件读配置时必须过滤掉。
// 壳契约 §9.1：plugins.<name> 下壳只认 enable/interval/timeout，
// 其余全部留给插件自己解释。
var shellReservedKeys = map[string]struct{}{
	"enable":   {},
	"interval": {},
	"timeout":  {},
}

// LoadConfig 读插件自身配置。
//
// 来源优先级：
//  1. 环境变量 MONITOR_CONFIG（壳注入的合并后 JSON）
//  2. 空 map（插件里自己处理缺省）
//
// 返回时过滤掉 enable / interval / timeout 三个壳保留键。
// 任何读取或解析错误都只返回空 map，不 panic。
func LoadConfig() map[string]any {
	raw := os.Getenv("MONITOR_CONFIG")
	if raw == "" {
		return map[string]any{}
	}
	var full map[string]any
	if err := json.Unmarshal([]byte(raw), &full); err != nil {
		Fprintln("[sdk] MONITOR_CONFIG parse failed: ", err)
		return map[string]any{}
	}
	out := make(map[string]any, len(full))
	for k, v := range full {
		if _, reserved := shellReservedKeys[k]; reserved {
			continue
		}
		out[k] = v
	}
	return out
}
