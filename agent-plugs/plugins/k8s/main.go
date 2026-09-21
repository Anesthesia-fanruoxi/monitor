package main

import (
	"agent-plugs/sdk"
)

func init() {
	sdk.SetName("k8s")
	sdk.SetVersion("0.2.0")

	// 25 个指标：11 条基础 + 14 条异常/容量/工作负载/存储扩展。
	// k8s 为集群级指标，不带 hostName 标签（与现状一致）。
	sdk.RegisterMetric("k8s_node_count", []string{"role"}, helpNodeCount, nil)
	sdk.RegisterMetric("k8s_node_status", []string{"node", "status"}, helpNodeStatus, nil)
	sdk.RegisterMetric("k8s_pod_count", []string{"namespace", "phase"}, helpPodCount, nil)
	sdk.RegisterMetric("k8s_pod_status", []string{"namespace", "pod", "status"}, helpPodStatus, nil)
	sdk.RegisterMetric("k8s_deployment_replicas", []string{"namespace", "deployment"}, helpDeploymentReplicas, nil)
	sdk.RegisterMetric("k8s_deployment_available_replicas", []string{"namespace", "deployment"}, helpDeploymentAvailable, nil)
	sdk.RegisterMetric("k8s_container_cpu_usage", []string{"namespace", "pod", "container"}, helpContainerCPUUsage, nil)
	sdk.RegisterMetric("k8s_container_memory_usage", []string{"namespace", "pod", "container"}, helpContainerMemoryUsage, nil)
	sdk.RegisterMetric("k8s_pod_cpu_request", []string{"namespace", "pod"}, helpPodCPURequest, nil)
	sdk.RegisterMetric("k8s_pod_memory_request", []string{"namespace", "pod"}, helpPodMemoryRequest, nil)
	sdk.RegisterMetric("k8s_pod_limit", []string{"namespace", "pod", "resource"}, helpPodLimit, nil)

	// 节点容量 / 压力 / pod 数
	sdk.RegisterMetric("k8s_node_cpu_capacity", []string{"node"}, helpNodeCPUCapacity, nil)
	sdk.RegisterMetric("k8s_node_memory_capacity", []string{"node"}, helpNodeMemCapacity, nil)
	sdk.RegisterMetric("k8s_node_cpu_allocatable", []string{"node"}, helpNodeCPUAllocatable, nil)
	sdk.RegisterMetric("k8s_node_memory_allocatable", []string{"node"}, helpNodeMemAllocatable, nil)
	sdk.RegisterMetric("k8s_node_condition", []string{"node", "condition"}, helpNodeCondition, nil)
	sdk.RegisterMetric("k8s_node_pod_count", []string{"node"}, helpNodePodCount, nil)

	// 容器异常信号
	sdk.RegisterMetric("k8s_container_restart_count", []string{"namespace", "pod", "container"}, helpContainerRestart, nil)
	sdk.RegisterMetric("k8s_container_ready", []string{"namespace", "pod", "container"}, helpContainerReady, nil)
	sdk.RegisterMetric("k8s_container_waiting_reason", []string{"namespace", "pod", "container", "reason"}, helpContainerWaiting, nil)

	// 工作负载 / 存储
	sdk.RegisterMetric("k8s_statefulset_replicas", []string{"namespace", "statefulset"}, helpStsReplicas, nil)
	sdk.RegisterMetric("k8s_statefulset_ready_replicas", []string{"namespace", "statefulset"}, helpStsReadyReplicas, nil)
	sdk.RegisterMetric("k8s_daemonset_desired", []string{"namespace", "daemonset"}, helpDsDesired, nil)
	sdk.RegisterMetric("k8s_daemonset_ready", []string{"namespace", "daemonset"}, helpDsReady, nil)
	sdk.RegisterMetric("k8s_pvc_status", []string{"namespace", "pvc", "phase"}, helpPVCStatus, nil)
}

func main() {
	sdk.SetCollect(Collect)
	sdk.Run()
}
