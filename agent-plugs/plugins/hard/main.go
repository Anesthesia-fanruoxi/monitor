package main

import (
	"agent-plugs/sdk"
)

func init() {
	sdk.SetName("hard")
	sdk.SetVersion("0.3.0")

	// 标签集合与 docs/插件设计.md §5.1 一致；iface/device 是网络/磁盘 IO 指标的额外标签
	labels := []string{"hostName", "cpu_model", "os_version", "kernel_version"}
	netLabels := []string{"hostName", "iface", "cpu_model", "os_version", "kernel_version"}
	ioLabels := []string{"hostName", "device", "cpu_model", "os_version", "kernel_version"}

	// cpu / 负载
	sdk.RegisterMetric("cpu_percent", labels, "cpu 使用率", nil)
	sdk.RegisterMetric("cpu_load_1", labels, "cpu 负载 1min", nil)
	sdk.RegisterMetric("cpu_load_5", labels, "cpu 负载 5min", nil)
	sdk.RegisterMetric("cpu_load_15", labels, "cpu 负载 15min", nil)
	sdk.RegisterMetric("cpu_total", labels, "cpu 核心数", nil)

	// 磁盘空间 / inode / IO
	sdk.RegisterMetric("disk_total", labels, "磁盘总容量 (bytes)", nil)
	sdk.RegisterMetric("disk_used", labels, "磁盘已用 (bytes)", nil)
	sdk.RegisterMetric("disk_free", labels, "磁盘剩余 (bytes)", nil)
	sdk.RegisterMetric("disk_used_percent", labels, "磁盘已用百分比", nil)
	sdk.RegisterMetric("disk_inode_total", labels, "根分区 inode 总数", nil)
	sdk.RegisterMetric("disk_inode_used_percent", labels, "根分区 inode 使用百分比", nil)
	sdk.RegisterMetric("disk_io_read_kbps", ioLabels, "磁盘读速率 (kB/s)", nil)
	sdk.RegisterMetric("disk_io_write_kbps", ioLabels, "磁盘写速率 (kB/s)", nil)

	// 内存 / swap
	sdk.RegisterMetric("memory_total", labels, "内存总量 (kB)", nil)
	sdk.RegisterMetric("memory_free", labels, "内存空闲 (kB)", nil)
	sdk.RegisterMetric("memory_buffered", labels, "内存 buffered (kB)", nil)
	sdk.RegisterMetric("memory_cached", labels, "内存 cached (kB)", nil)
	sdk.RegisterMetric("memory_shared", labels, "内存 shared (kB)", nil)
	sdk.RegisterMetric("memory_available", labels, "内存 available (kB)", nil)
	sdk.RegisterMetric("memory_used", labels, "内存已用 (kB)", nil)
	sdk.RegisterMetric("memory_used_percent", labels, "内存已用百分比", nil)
	sdk.RegisterMetric("swap_total", labels, "swap 总量 (kB)", nil)
	sdk.RegisterMetric("swap_free", labels, "swap 剩余 (kB)", nil)
	sdk.RegisterMetric("swap_used_percent", labels, "swap 使用百分比", nil)

	// 网络 / 连接
	sdk.RegisterMetric("net_recv_kbps", netLabels, "网卡接收速率 (kB/s)", nil)
	sdk.RegisterMetric("net_sent_kbps", netLabels, "网卡发送速率 (kB/s)", nil)
	sdk.RegisterMetric("tcp_inuse", labels, "TCP 连接数", nil)
	sdk.RegisterMetric("udp_inuse", labels, "UDP 连接数", nil)
	sdk.RegisterMetric("sockets_inuse", labels, "socket 总数", nil)

	// 进程 / 句柄 / 系统
	sdk.RegisterMetric("procs_running", labels, "运行中进程数", nil)
	sdk.RegisterMetric("procs_blocked", labels, "不可中断阻塞进程数", nil)
	sdk.RegisterMetric("procs_total", labels, "进程总数", nil)
	sdk.RegisterMetric("fd_used", labels, "已分配文件句柄数", nil)
	sdk.RegisterMetric("fd_max", labels, "文件句柄上限", nil)
	sdk.RegisterMetric("uptime_seconds", labels, "开机时长 (秒)", nil)
}

func main() {
	sdk.SetCollect(Collect)
	sdk.Run()
}
