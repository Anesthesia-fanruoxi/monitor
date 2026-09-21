package main

import (
	"context"
	"strings"

	"agent-plugs/sdk"
)

const (
	helpPodCount         = "k8s pod 数量"
	helpPodStatus        = "k8s pod 状态"
	helpPodCPURequest    = "k8s pod cpu request"
	helpPodMemoryRequest = "k8s pod 内存 request (kB)"
	helpPodLimit         = "k8s pod 资源 limit"
	helpNodePodCount     = "k8s 节点上 pod 数"
	helpContainerRestart = "k8s 容器重启次数"
	helpContainerReady   = "k8s 容器就绪 (1=就绪)"
	helpContainerWaiting = "k8s 容器等待原因 (1=该容器处于此等待状态)"
)

// appendPods pod 域：数量 / 状态 / request / limit + 容器重启 / 就绪 / 等待原因 + 节点 pod 数。
// 容器级异常信号只对未终止（非 Succeeded/Failed）的 pod 输出，避免 Job 历史噪声。
func appendPods(out []sdk.DeclaredMetric, ctx context.Context, allow []string, limitOnly bool) []sdk.DeclaredMetric {
	var list k8sPodList
	if err := kc.getJSON(ctx, "/api/v1/pods", &list); err != nil {
		sdk.Fprintln("[k8s] list pods:", err)
		return out
	}

	podCount := map[string]float64{}
	podCountByNode := map[string]float64{}
	var podStatus, podCPUReq, podMemReq, podLimit []sdk.Sample
	var restarts, ready, waiting []sdk.Sample
	for _, p := range list.Items {
		ns, name := p.Metadata.Namespace, p.Metadata.Name
		if !nsAllowed(ns, allow) {
			continue
		}
		phase := p.Status.Phase
		if phase == "" {
			phase = "Unknown"
		}
		podCount[ns+"\x00"+phase]++
		if p.Spec.NodeName != "" {
			podCountByNode[p.Spec.NodeName]++
		}
		podStatus = append(podStatus, sdk.Sample{
			Labels: map[string]string{"namespace": ns, "pod": name, "status": phase}, Value: 1,
		})

		var cpuReq, memReq, cpuLimit, memLimit float64
		hasLimit := false
		for _, c := range p.Spec.Containers {
			if v := c.Resources.Requests["cpu"]; v != "" {
				cpuReq += parseQuantity(v)
			}
			if v := c.Resources.Requests["memory"]; v != "" {
				memReq += parseQuantity(v) / 1024
			}
			if v := c.Resources.Limits["cpu"]; v != "" {
				cpuLimit += parseQuantity(v)
				hasLimit = true
			}
			if v := c.Resources.Limits["memory"]; v != "" {
				memLimit += parseQuantity(v) / 1024
				hasLimit = true
			}
		}
		podCPUReq = append(podCPUReq, sdk.Sample{
			Labels: map[string]string{"namespace": ns, "pod": name}, Value: cpuReq})
		podMemReq = append(podMemReq, sdk.Sample{
			Labels: map[string]string{"namespace": ns, "pod": name}, Value: memReq})
		// only_with_limits=false → 全部 pod 报 limit（无 limit 则 0）；
		// true → 仅给有 limit 的 pod 报。
		if !limitOnly || hasLimit {
			podLimit = append(podLimit,
				sdk.Sample{Labels: map[string]string{"namespace": ns, "pod": name, "resource": "cpu"}, Value: cpuLimit},
				sdk.Sample{Labels: map[string]string{"namespace": ns, "pod": name, "resource": "memory"}, Value: memLimit},
			)
		}

		if phase == "Succeeded" || phase == "Failed" {
			continue
		}
		for _, cs := range p.Status.ContainerStatuses {
			lbl := map[string]string{"namespace": ns, "pod": name, "container": cs.Name}
			restarts = append(restarts, sdk.Sample{Labels: lbl, Value: float64(cs.RestartCount)})
			ready = append(ready, sdk.Sample{Labels: lbl, Value: boolF(cs.Ready)})
			if w := cs.State.Waiting; w != nil && w.Reason != "" {
				waiting = append(waiting, sdk.Sample{
					Labels: map[string]string{
						"namespace": ns, "pod": name,
						"container": cs.Name, "reason": w.Reason,
					}, Value: 1,
				})
			}
		}
	}

	var pcSamples []sdk.Sample
	for k, v := range podCount {
		ns, ph, _ := strings.Cut(k, "\x00")
		pcSamples = append(pcSamples, sdk.Sample{
			Labels: map[string]string{"namespace": ns, "phase": ph}, Value: v,
		})
	}
	var npSamples []sdk.Sample
	for node, v := range podCountByNode {
		npSamples = append(npSamples, sdk.Sample{Labels: map[string]string{"node": node}, Value: v})
	}

	out = append(out,
		metric("k8s_pod_count", helpPodCount, []string{"namespace", "phase"}, pcSamples),
		metric("k8s_pod_status", helpPodStatus, []string{"namespace", "pod", "status"}, podStatus),
		metric("k8s_pod_cpu_request", helpPodCPURequest, []string{"namespace", "pod"}, podCPUReq),
		metric("k8s_pod_memory_request", helpPodMemoryRequest, []string{"namespace", "pod"}, podMemReq),
		metric("k8s_pod_limit", helpPodLimit, []string{"namespace", "pod", "resource"}, podLimit),
		metric("k8s_node_pod_count", helpNodePodCount, []string{"node"}, npSamples),
		metric("k8s_container_restart_count", helpContainerRestart, []string{"namespace", "pod", "container"}, restarts),
		metric("k8s_container_ready", helpContainerReady, []string{"namespace", "pod", "container"}, ready),
		metric("k8s_container_waiting_reason", helpContainerWaiting, []string{"namespace", "pod", "container", "reason"}, waiting),
	)
	return out
}
