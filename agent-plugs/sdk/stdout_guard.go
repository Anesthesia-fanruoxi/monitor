package sdk

import (
	"fmt"
	"os"
)

// Fprintln 替代 fmt.Println / fmt.Fprintln(os.Stdout, ...) 的安全日志出口。
// 所有调试 / 错误信息都走 stderr，避免污染协议通道。
// 壳侧约定 stdout 只准出 JSON 行，任何混入都会被判定为协议违规。
func Fprintln(a ...any) {
	fmt.Fprintln(os.Stderr, a...)
}

// Fprintf 是 Fprintln 的格式化版本，同样走 stderr。
func Fprintf(format string, a ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", a...)
}
