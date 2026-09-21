package main

import (
	"os"
	"strconv"
	"strings"

	"agent-plugs/sdk"
)

// ===== 内存 =====
// 从 /proc/meminfo 读；单位 kB，原样上报（历史遗留，不乘 1024）
type memInfo struct {
	total, free, buffered, cached, shared, available, used uint64
	usedPct                                                float64
}

func getMemInfo() (memInfo, error) {
	var mem memInfo
	data, err := os.ReadFile(procPath("/meminfo"))
	if err != nil {
		return mem, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		v, _ := strconv.ParseUint(fields[1], 10, 64)
		switch fields[0] {
		case "MemTotal:":
			mem.total = v
		case "MemFree:":
			mem.free = v
		case "Buffers:":
			mem.buffered = v
		case "Cached:":
			mem.cached = v
		case "Shmem:":
			mem.shared = v
		case "MemAvailable:":
			mem.available = v
		}
	}
	mem.used = mem.total - mem.free - mem.buffered - mem.cached
	if mem.total > 0 {
		mem.usedPct = float64(mem.used) / float64(mem.total) * 100
	}
	return mem, nil
}

// ===== swap =====
type swapInfo struct {
	total, free, used uint64
	usedPct           float64
}

func getSwap() (swapInfo, error) {
	var sw swapInfo
	data, err := os.ReadFile(procPath("/meminfo"))
	if err != nil {
		return sw, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		v, _ := strconv.ParseUint(fields[1], 10, 64)
		switch fields[0] {
		case "SwapTotal:":
			sw.total = v
		case "SwapFree:":
			sw.free = v
		}
	}
	sw.used = sw.total - sw.free
	if sw.total > 0 {
		sw.usedPct = float64(sw.used) / float64(sw.total) * 100
	}
	return sw, nil
}

// appendMem：memory_*（8 条）+ swap_*（3 条）
func appendMem(out []sdk.DeclaredMetric, host string, labels []string) []sdk.DeclaredMetric {
	if mem, err := getMemInfo(); err == nil {
		out = append(out,
			BuildMetric("memory_total", "内存总量 (kB)", labels, host, float64(mem.total)),
			BuildMetric("memory_free", "内存空闲 (kB)", labels, host, float64(mem.free)),
			BuildMetric("memory_buffered", "内存 buffered (kB)", labels, host, float64(mem.buffered)),
			BuildMetric("memory_cached", "内存 cached (kB)", labels, host, float64(mem.cached)),
			BuildMetric("memory_shared", "内存 shared (kB)", labels, host, float64(mem.shared)),
			BuildMetric("memory_available", "内存 available (kB)", labels, host, float64(mem.available)),
			BuildMetric("memory_used", "内存已用 (kB)", labels, host, float64(mem.used)),
			BuildMetric("memory_used_percent", "内存已用百分比", labels, host, mem.usedPct),
		)
	} else {
		sdk.Fprintln("[hard] memory:", err)
	}

	if sw, err := getSwap(); err == nil {
		out = append(out,
			BuildMetric("swap_total", "swap 总量 (kB)", labels, host, float64(sw.total)),
			BuildMetric("swap_free", "swap 剩余 (kB)", labels, host, float64(sw.free)),
			BuildMetric("swap_used_percent", "swap 使用百分比", labels, host, sw.usedPct),
		)
	} else {
		sdk.Fprintln("[hard] swap:", err)
	}

	return out
}
