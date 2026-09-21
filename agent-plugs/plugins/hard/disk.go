package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"agent-plugs/sdk"
)

// ===== 磁盘空间 =====
// 用 df -P -k /，最后一行第 2/3/4 列 × 1024 转字节
func getDiskInfo() (total, used, free uint64, usedPct float64, err error) {
	output, err := exec.Command("df", "-P", "-k", "/").Output()
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("df failed: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) < 2 {
		return 0, 0, 0, 0, fmt.Errorf("df output unexpected: %q", string(output))
	}

	fields := strings.Fields(lines[len(lines)-1])
	if len(fields) < 4 {
		return 0, 0, 0, 0, fmt.Errorf("df fields too few: %q", lines[len(lines)-1])
	}

	total, _ = strconv.ParseUint(fields[1], 10, 64)
	used, _ = strconv.ParseUint(fields[2], 10, 64)
	free, _ = strconv.ParseUint(fields[3], 10, 64)
	total *= 1024
	used *= 1024
	free *= 1024

	if total == 0 {
		return 0, 0, 0, 0, fmt.Errorf("root total is 0")
	}
	usedPct = float64(used) / float64(total) * 100
	return total, used, free, usedPct, nil
}

// ===== inode =====
// df -P -i /，同样取最后一行
func getInodeInfo() (total, used uint64, usedPct float64, err error) {
	output, err := exec.Command("df", "-P", "-i", "/").Output()
	if err != nil {
		return 0, 0, 0, fmt.Errorf("df -i failed: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) < 2 {
		return 0, 0, 0, fmt.Errorf("df -i output unexpected: %q", string(output))
	}

	fields := strings.Fields(lines[len(lines)-1])
	if len(fields) < 4 {
		return 0, 0, 0, fmt.Errorf("df -i fields too few: %q", lines[len(lines)-1])
	}

	total, _ = strconv.ParseUint(fields[1], 10, 64)
	used, _ = strconv.ParseUint(fields[2], 10, 64)
	if total == 0 {
		return 0, 0, 0, fmt.Errorf("inode total is 0")
	}
	usedPct = float64(used) / float64(total) * 100
	return total, used, usedPct, nil
}

// ===== 磁盘 IO 差分状态（scheduler 单在途保证串行，无锁）=====
type ioSnap struct {
	read, write uint64 // 累计扇区数
	time        time.Time
}

var lastDiskIO = map[string]ioSnap{}

// appendDiskIO 从 /proc/diskstats 差分出读写速率（kB/s）。
// 扇区 × 512B / 1024 = kB，即差分扇区数 / 2。
// 过滤 loop / ram 伪设备；首轮无基线不输出。
// 多设备合并为同一指标的多个 sample，条数恒定不随设备数增长。
func appendDiskIO(out []sdk.DeclaredMetric, host string) []sdk.DeclaredMetric {
	data, err := os.ReadFile(procPath("/diskstats"))
	if err != nil {
		sdk.Fprintln("[hard] diskstats:", err)
		return out
	}

	now := time.Now()
	prev := lastDiskIO
	lastDiskIO = make(map[string]ioSnap, len(prev)+2)

	readSamples := make([]sdk.Sample, 0, 4)
	writeSamples := make([]sdk.Sample, 0, 4)
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 10 {
			continue
		}
		dev := fields[2]
		if strings.HasPrefix(dev, "loop") || strings.HasPrefix(dev, "ram") {
			continue
		}
		read, _ := strconv.ParseUint(fields[5], 10, 64)  // 读扇区累计
		write, _ := strconv.ParseUint(fields[9], 10, 64) // 写扇区累计
		lastDiskIO[dev] = ioSnap{read: read, write: write, time: now}
		if p, ok := prev[dev]; ok && now.Sub(p.time) <= time.Minute && read >= p.read && write >= p.write {
			dt := now.Sub(p.time).Seconds()
			readSamples = append(readSamples, sdk.Sample{Labels: map[string]string{"hostName": host, "device": dev}, Value: float64(read-p.read) / 2 / dt})
			writeSamples = append(writeSamples, sdk.Sample{Labels: map[string]string{"hostName": host, "device": dev}, Value: float64(write-p.write) / 2 / dt})
		}
	}
	if len(readSamples) > 0 {
		out = append(out,
			buildMetricMulti("disk_io_read_kbps", "磁盘读速率 (kB/s)", ioLabels, readSamples),
			buildMetricMulti("disk_io_write_kbps", "磁盘写速率 (kB/s)", ioLabels, writeSamples),
		)
	}
	return out
}

// appendDisk：disk_*（4 条）+ inode（2 条）+ IO（2 条）
func appendDisk(out []sdk.DeclaredMetric, host string, labels []string) []sdk.DeclaredMetric {
	if total, used, free, pct, err := getDiskInfo(); err == nil {
		out = append(out,
			BuildMetric("disk_total", "磁盘总容量 (bytes)", labels, host, float64(total)),
			BuildMetric("disk_used", "磁盘已用 (bytes)", labels, host, float64(used)),
			BuildMetric("disk_free", "磁盘剩余 (bytes)", labels, host, float64(free)),
			BuildMetric("disk_used_percent", "磁盘已用百分比", labels, host, pct),
		)
	} else {
		sdk.Fprintln("[hard] disk:", err)
	}

	if total, _, pct, err := getInodeInfo(); err == nil {
		out = append(out,
			BuildMetric("disk_inode_total", "根分区 inode 总数", labels, host, float64(total)),
			BuildMetric("disk_inode_used_percent", "根分区 inode 使用百分比", labels, host, pct),
		)
	} else {
		sdk.Fprintln("[hard] inode:", err)
	}

	return appendDiskIO(out, host)
}
