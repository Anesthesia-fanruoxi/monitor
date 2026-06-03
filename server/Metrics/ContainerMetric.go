package Metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	// 定义容器 CPU 使用情况指标
	ContainerCpuUsageMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "container_cpu_usage", // 容器 CPU 使用情况
			Help: "容器 CPU 使用情况",
		},
		[]string{"namespace", "podName", "container", "controllerName", "project"},
	)

	// 定义容器内存使用情况指标
	ContainerMemoryUsageMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "container_memory_usage", // 容器内存使用情况
			Help: "容器内存使用情况",
		},
		[]string{"namespace", "podName", "container", "controllerName", "project"},
	)

	// 定义容器 CPU 限制情况指标
	ContainerCpuLimitMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "container_cpu_limit", // 容器 CPU 限制
			Help: "容器 CPU 限制",
		},
		[]string{"namespace", "podName", "container", "controllerName", "project"},
	)

	// 定义容器内存限制情况指标
	ContainerMemoryLimitMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "container_memory_limit", // 容器内存限制
			Help: "容器内存限制",
		},
		[]string{"namespace", "podName", "container", "controllerName", "project"},
	)

	// 定义容器重启次数指标
	ContainerRestartCountMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "container_restart_count", // 容器重启次数
			Help: "容器重启次数",
		},
		[]string{"namespace", "podName", "container", "controllerName", "project"},
	)
	// 定义容器重启次数指标
	ContainerLastTerminationTimeMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "container_last_termination_time", // 容器重启次数
			Help: "容器重启时间",
		},
		[]string{"namespace", "podName", "container", "controllerName", "project"},
	)
)
