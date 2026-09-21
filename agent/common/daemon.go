package common

import (
	"os"
	"path/filepath"
	"sync"
)

// RestartFlagFile 重启标记文件：守护模式下的工作进程通过它通知父进程立即重启
const RestartFlagFile = "restart.flag"

// EnvDaemonChild 由守护进程注入，用于区分当前进程是否由守护进程拉起
const EnvDaemonChild = "MONITOR_DAEMON_CHILD"

var (
	exeDirOnce sync.Once
	exeDirValue string
)

// exeDir 返回可执行文件所在目录，解析一次后缓存
func exeDir() string {
	exeDirOnce.Do(func() {
		exePath, err := os.Executable()
		if err != nil {
			exeDirValue = "."
			return
		}
		if resolved, err := filepath.EvalSymlinks(exePath); err == nil {
			exePath = resolved
		}
		if abs, err := filepath.Abs(exePath); err == nil {
			exePath = abs
		}
		exeDirValue = filepath.Dir(exePath)
	})
	return exeDirValue
}

// IsDaemonChild 当前进程是否由守护进程拉起
func IsDaemonChild() bool {
	return os.Getenv(EnvDaemonChild) == "1"
}

// WorkPIDFile 返回 PID 文件绝对路径
func WorkPIDFile() string { return filepath.Join(exeDir(), "work.pid") }

// RestartFlagPath 返回重启标记文件绝对路径
func RestartFlagPath() string { return filepath.Join(exeDir(), RestartFlagFile) }
