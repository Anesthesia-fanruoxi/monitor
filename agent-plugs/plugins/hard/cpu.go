package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"agent-plugs/sdk"
)

// ===== CPU 采样状态 =====
var (
	lastCPUTotal uint64
	lastCPUIdle  uint64
	cpuMu        sync.Mutex
	cpuLastTime  time.Time
)

// getCPUPercent 采样 CPU 使用率：首次 / 计数器回绕 /
// 距上轮超 1 分钟 → 只重建基线并返回 0（该轮无数据）
func getCPUPercent() (float64, error) {
	cpuMu.Lock()
	defer cpuMu.Unlock()

	data, err := os.ReadFile(procPath("/stat"))
	if err != nil {
		return 0, err
	}

	lines := strings.Split(string(data), "\n")
	if len(lines) == 0 {
		return 0, fmt.Errorf("cannot read cpu info")
	}
	fields := strings.Fields(lines[0])
	if len(fields) < 5 {
		return 0, fmt.Errorf("cannot parse cpu info")
	}

	values := make([]uint64, 0, 8)
	for i := 1; i < 9 && i < len(fields); i++ {
		v, _ := strconv.ParseUint(fields[i], 10, 64)
		values = append(values, v)
	}

	var total uint64
	for _, v := range values {
		total += v
	}
	idle := values[3]
	if len(values) > 4 {
		idle += values[4]
	}
	if total == 0 {
		return 0, fmt.Errorf("cannot read cpu info")
	}

	if lastCPUTotal == 0 || total < lastCPUTotal || time.Since(cpuLastTime) > time.Minute {
		lastCPUTotal = total
		lastCPUIdle = idle
		cpuLastTime = time.Now()
		return 0, nil
	}

	totalDelta := total - lastCPUTotal
	idleDelta := idle - lastCPUIdle

	lastCPUTotal = total
	lastCPUIdle = idle
	cpuLastTime = time.Now()

	if totalDelta == 0 || idleDelta > totalDelta {
		return 0, nil
	}
	return 100 * float64(totalDelta-idleDelta) / float64(totalDelta), nil
}

// getLoad 读 1/5/15 分钟负载
func getLoad() (l1, l5, l15 float64, err error) {
	data, err := os.ReadFile(procPath("/loadavg"))
	if err != nil {
		return 0, 0, 0, err
	}
	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		return 0, 0, 0, fmt.Errorf("cannot read load")
	}
	l1, _ = strconv.ParseFloat(fields[0], 64)
	l5, _ = strconv.ParseFloat(fields[1], 64)
	l15, _ = strconv.ParseFloat(fields[2], 64)
	return
}

// appendCPU：cpu_percent / cpu_load_* / cpu_total
func appendCPU(out []sdk.DeclaredMetric, host string, labels []string) []sdk.DeclaredMetric {
	if v, err := getCPUPercent(); err == nil {
		out = append(out, BuildMetric("cpu_percent", "cpu 使用率", labels, host, v))
	} else {
		sdk.Fprintln("[hard] cpu_percent:", err)
	}

	if l1, l5, l15, err := getLoad(); err == nil {
		out = append(out,
			BuildMetric("cpu_load_1", "cpu 负载 1min", labels, host, l1),
			BuildMetric("cpu_load_5", "cpu 负载 5min", labels, host, l5),
			BuildMetric("cpu_load_15", "cpu 负载 15min", labels, host, l15),
		)
	} else {
		sdk.Fprintln("[hard] load:", err)
	}

	out = append(out, BuildMetric("cpu_total", "cpu 核心数", labels, host, float64(getStatic().CPUCount)))
	return out
}
