package main

import (
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"agent-plugs/sdk"
)

// ===== procfs 根路径 =====
// 由插件配置 proc_root 指定（容器内挂载宿主 /proc 时配 /host/proc），缺省 /proc。
var procRoot = "/proc"

// procPath 拼接 procfs 路径（procPath("/meminfo") → /proc/meminfo）
func procPath(name string) string { return procRoot + name }

// initProcRoot 从插件配置读取 proc_root（幂等）
func initProcRoot() {
	if v, ok := sdk.LoadConfig()["proc_root"].(string); ok {
		if v = strings.TrimRight(strings.TrimSpace(v), "/"); v != "" {
			procRoot = v
		}
	}
}

// hostname 节点名：优先 procfs（容器内挂载宿主 /proc 后即节点名），回落系统调用。
func hostname() string {
	if b, err := os.ReadFile(procPath("/sys/kernel/hostname")); err == nil {
		if h := strings.TrimSpace(string(b)); h != "" {
			return h
		}
	}
	return sdk.Hostname()
}

// ===== 静态信息（一轮内不变，30min 缓存）=====
type staticInfo struct {
	CPUCount      int
	CPUModel      string
	OSVersion     string
	KernelVersion string
}

var (
	staticCache     staticInfo
	staticMu        sync.RWMutex
	staticCacheTime time.Time
	staticCacheTTL  = 30 * time.Minute
)

// getStatic 带缓存的静态信息：核心数 / cpu 型号 / os 版本 / 内核版本
func getStatic() staticInfo {
	staticMu.RLock()
	if staticCacheTime.After(time.Now().Add(-staticCacheTTL)) && staticCache.CPUCount > 0 {
		info := staticCache
		staticMu.RUnlock()
		return info
	}
	staticMu.RUnlock()

	staticMu.Lock()
	defer staticMu.Unlock()

	if staticCacheTime.After(time.Now().Add(-staticCacheTTL)) && staticCache.CPUCount > 0 {
		return staticCache
	}

	staticCache = staticInfo{}

	// CPU 核心数
	if data, err := os.ReadFile(procPath("/cpuinfo")); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "processor") {
				staticCache.CPUCount++
			}
		}
	}
	if staticCache.CPUCount == 0 {
		staticCache.CPUCount = runtime.NumCPU()
	}

	// CPU 型号
	if data, err := os.ReadFile(procPath("/cpuinfo")); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			k, v, ok := strings.Cut(line, ":")
			if !ok {
				continue
			}
			switch strings.TrimSpace(k) {
			case "model name", "Model", "Hardware":
				if m := strings.TrimSpace(v); m != "" {
					staticCache.CPUModel = m
				}
			}
			if staticCache.CPUModel != "" {
				break
			}
		}
	}

	// OS 版本
	if data, err := os.ReadFile("/etc/redhat-release"); err == nil {
		staticCache.OSVersion = strings.TrimSpace(string(data))
	}
	if staticCache.OSVersion == "" {
		if data, err := os.ReadFile("/etc/os-release"); err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				k, v, ok := strings.Cut(line, "=")
				if !ok || strings.TrimSpace(k) != "PRETTY_NAME" {
					continue
				}
				staticCache.OSVersion = strings.Trim(strings.TrimSpace(v), `"`)
				break
			}
		}
	}

	// 内核版本
	if data, err := os.ReadFile(procPath("/sys/kernel/osrelease")); err == nil {
		staticCache.KernelVersion = strings.TrimSpace(string(data))
	}

	staticCacheTime = time.Now()
	return staticCache
}

// ===== 公共标签 =====
func commonLabels() map[string]string {
	st := getStatic()
	return map[string]string{
		"cpu_model":      st.CPUModel,
		"os_version":     st.OSVersion,
		"kernel_version": st.KernelVersion,
	}
}

// ===== 指标标签集合 =====
// 样本携带的标签必须全部在声明 labels 中（服务端逐样本校验），
// iface / device 作为额外标签随对应指标声明上行。
var (
	baseLabels = []string{"hostName", "cpu_model", "os_version", "kernel_version"}
	netLabels  = []string{"hostName", "iface", "cpu_model", "os_version", "kernel_version"}
	ioLabels   = []string{"hostName", "device", "cpu_model", "os_version", "kernel_version"}
)

// ===== BuildMetric 构造单值指标（sample 只带 hostName）=====
func BuildMetric(name, help string, labels []string, host string, value float64) sdk.DeclaredMetric {
	return buildMetric(name, help, labels, host, nil, value)
}

// buildMetric 构造带额外 sample 标签的指标（如 iface / device）
func buildMetric(name, help string, labels []string, host string, extra map[string]string, value float64) sdk.DeclaredMetric {
	sampleLabels := make(map[string]string, 1+len(extra))
	sampleLabels["hostName"] = host
	for k, v := range extra {
		sampleLabels[k] = v
	}
	return sdk.DeclaredMetric{
		Name:    name,
		Help:    help,
		Type:    sdk.MetricTypeGauge,
		Labels:  labels,
		Common:  commonLabels(),
		Samples: []sdk.Sample{{Labels: sampleLabels, Value: value}},
	}
}

// buildMetricMulti 构造一个指标多条样本（多网卡 / 多磁盘设备共用一个指标名），
// 条数恒定，不随接口/设备数增长。
func buildMetricMulti(name, help string, labels []string, samples []sdk.Sample) sdk.DeclaredMetric {
	return sdk.DeclaredMetric{
		Name:    name,
		Help:    help,
		Type:    sdk.MetricTypeGauge,
		Labels:  labels,
		Common:  commonLabels(),
		Samples: samples,
	}
}

// ===== 系统域：文件句柄 / 进程 / uptime =====
func appendSystem(out []sdk.DeclaredMetric, host string, labels []string) []sdk.DeclaredMetric {
	// file-nr 三列 = 已分配 / 已分配未使用 / 上限
	if data, err := os.ReadFile(procPath("/sys/fs/file-nr")); err == nil {
		fields := strings.Fields(string(data))
		if len(fields) >= 3 {
			used, _ := strconv.ParseUint(fields[0], 10, 64)
			max, _ := strconv.ParseUint(fields[2], 10, 64)
			out = append(out,
				BuildMetric("fd_used", "已分配文件句柄数", labels, host, float64(used)),
				BuildMetric("fd_max", "文件句柄上限", labels, host, float64(max)),
			)
		}
	} else {
		sdk.Fprintln("[hard] fd:", err)
	}

	// loadavg 第 4 字段 "running/total"
	if data, err := os.ReadFile(procPath("/loadavg")); err == nil {
		fields := strings.Fields(string(data))
		if len(fields) >= 4 {
			if kv := strings.Split(fields[3], "/"); len(kv) == 2 {
				running, _ := strconv.ParseUint(kv[0], 10, 64)
				total, _ := strconv.ParseUint(kv[1], 10, 64)
				out = append(out,
					BuildMetric("procs_running", "运行中进程数", labels, host, float64(running)),
					BuildMetric("procs_total", "进程总数", labels, host, float64(total)),
				)
			}
		}
	} else {
		sdk.Fprintln("[hard] loadavg:", err)
	}

	// procs_blocked 在 /proc/stat
	if data, err := os.ReadFile(procPath("/stat")); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "procs_blocked ") {
				v, _ := strconv.ParseUint(strings.TrimSpace(strings.TrimPrefix(line, "procs_blocked ")), 10, 64)
				out = append(out, BuildMetric("procs_blocked", "不可中断阻塞进程数", labels, host, float64(v)))
				break
			}
		}
	}

	// uptime
	if data, err := os.ReadFile(procPath("/uptime")); err == nil {
		fields := strings.Fields(string(data))
		if len(fields) >= 1 {
			up, _ := strconv.ParseFloat(fields[0], 64)
			out = append(out, BuildMetric("uptime_seconds", "开机时长 (秒)", labels, host, up))
		}
	} else {
		sdk.Fprintln("[hard] uptime:", err)
	}

	return out
}

// ===== Collect 主入口 =====
// 按域组装 35 条指标；单项失败只打日志、丢该条，不 panic。
// 各域实现在 cpu.go / mem.go / disk.go / net.go。
func Collect() []sdk.DeclaredMetric {
	initProcRoot()
	host := hostname()

	out := make([]sdk.DeclaredMetric, 0, 35)
	out = appendCPU(out, host, baseLabels)
	out = appendMem(out, host, baseLabels)
	out = appendDisk(out, host, baseLabels)
	out = appendNet(out, host, baseLabels)
	out = appendSystem(out, host, baseLabels)
	return out
}
