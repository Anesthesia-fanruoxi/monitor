package Metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	// 定义容器 CPU 使用情况指标
	ControllerReplicasMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "controller_replicas", // 容器 CPU 使用情况
			Help: "副本数量",
		},
		[]string{"namespace", "container", "controllerType", "project"},
	)

	// 定义容器内存使用情况指标
	ControllerReplicasAvailableMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "controller_replicas_available", // 容器内存使用情况
			Help: "已就绪副本数量",
		},
		[]string{"namespace", "container", "controllerType", "project"},
	)

	// 定义容器 CPU 限制情况指标
	ControllerReplicasUnavailableMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "controller_replicas_unavailable", // 容器 CPU 限制
			Help: "未就绪副本数量",
		},
		[]string{"namespace", "container", "controllerType", "project"},
	)
)
