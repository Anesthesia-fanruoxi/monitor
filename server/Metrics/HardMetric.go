package Metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	// 定义 CPU 使用率百分比指标
	CpuPercentMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cpu_percent", // CPU 使用率百分比
			Help: "CPU 使用率百分比",
		},
		[]string{"hostName", "project", "cpu_model", "os_version", "kernel_version"},
	)

	// 定义磁盘总空间指标
	DiskTotalMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "disk_total", // 磁盘总空间
			Help: "磁盘总空间",
		},
		[]string{"hostName", "project", "cpu_model", "os_version", "kernel_version"},
	)

	// 定义已使用的磁盘空间指标
	DiskUsedMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "disk_used", // 已使用的磁盘空间
			Help: "已使用的磁盘空间",
		},
		[]string{"hostName", "project", "cpu_model", "os_version", "kernel_version"},
	)

	// 定义可用磁盘空间指标
	DiskFreeMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "disk_free", // 可用磁盘空间
			Help: "可用磁盘空间",
		},
		[]string{"hostName", "project", "cpu_model", "os_version", "kernel_version"},
	)

	// 定义磁盘使用百分比指标
	DiskUsedPercentMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "disk_used_percent", // 磁盘使用百分比
			Help: "磁盘使用百分比",
		},
		[]string{"hostName", "project", "cpu_model", "os_version", "kernel_version"},
	)

	// 定义内存总量指标
	MemoryTotalMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "memory_total", // 内存总量
			Help: "内存总量",
		},
		[]string{"hostName", "project", "cpu_model", "os_version", "kernel_version"},
	)

	// 定义已使用的内存指标
	MemoryUsedMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "memory_used", // 已使用的内存
			Help: "已使用的内存",
		},
		[]string{"hostName", "project", "cpu_model", "os_version", "kernel_version"},
	)

	// 定义空闲内存指标
	MemoryFreeMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "memory_free", // 空闲内存
			Help: "空闲内存",
		},
		[]string{"hostName", "project", "cpu_model", "os_version", "kernel_version"},
	)

	// 定义内存使用百分比指标
	MemoryUsedPercentMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "memory_used_percent", // 内存使用百分比
			Help: "内存使用百分比",
		},
		[]string{"hostName", "project", "cpu_model", "os_version", "kernel_version"},
	)

	// 定义 1 分钟 CPU 负载平均值指标
	CpuLoad1Metric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cpu_load_1", // 1 分钟 CPU 负载平均值
			Help: "1 分钟 CPU 负载平均值",
		},
		[]string{"hostName", "project", "cpu_model", "os_version", "kernel_version"},
	)

	// 定义 5 分钟 CPU 负载平均值指标
	CpuLoad5Metric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cpu_load_5", // 5 分钟 CPU 负载平均值
			Help: "5 分钟 CPU 负载平均值",
		},
		[]string{"hostName", "project", "cpu_model", "os_version", "kernel_version"},
	)

	// 定义 15 分钟 CPU 负载平均值指标
	CpuLoad15Metric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cpu_load_15", // 15 分钟 CPU 负载平均值
			Help: "15 分钟 CPU 负载平均值",
		},
		[]string{"hostName", "project", "cpu_model", "os_version", "kernel_version"},
	)

	// 定义 CPU 核心数指标
	CpuTotalMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cpu_total", // CPU 核心数
			Help: "cpu 核心数",
		},
		[]string{"hostName", "project", "cpu_model", "os_version", "kernel_version"},
	)
)
