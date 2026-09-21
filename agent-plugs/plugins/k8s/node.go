package main

import (
	"context"

	"agent-plugs/sdk"
)

const (
	helpNodeCount          = "k8s 节点数"
	helpNodeStatus         = "k8s 节点状态"
	helpNodeCPUCapacity    = "k8s 节点 cpu 容量 (核)"
	helpNodeMemCapacity    = "k8s 节点内存容量 (kB)"
	helpNodeCPUAllocatable = "k8s 节点可分配 cpu (核)"
	helpNodeMemAllocatable = "k8s 节点可分配内存 (kB)"
	helpNodeCondition      = "k8s 节点压力条件 (1=异常)"
)

// pressureConditions 需要关注的节点异常条件（Ready 已由 node_status 覆盖）
var pressureConditions = []string{
	"MemoryPressure", "DiskPressure", "PIDPressure", "NetworkUnavailable",
}

// appendNodes 节点域：数量 / 状态 / 容量 / 可分配 / 压力条件。
// 容量单位：cpu 转核、内存转 kB（parseQuantity 统一解析后换算）。
func appendNodes(out []sdk.DeclaredMetric, ctx context.Context) []sdk.DeclaredMetric {
	var list k8sNodeList
	if err := kc.getJSON(ctx, "/api/v1/nodes", &list); err != nil {
		sdk.Fprintln("[k8s] list nodes:", err)
		return out
	}

	var masters, workers float64
	nsamples := make([]sdk.Sample, 0, len(list.Items)*2)
	cpuCap := make([]sdk.Sample, 0, len(list.Items))
	memCap := make([]sdk.Sample, 0, len(list.Items))
	cpuAlloc := make([]sdk.Sample, 0, len(list.Items))
	memAlloc := make([]sdk.Sample, 0, len(list.Items))
	conds := make([]sdk.Sample, 0, len(list.Items)*len(pressureConditions))
	for _, n := range list.Items {
		node := n.Metadata.Name
		ready := nodeReady(n)
		nsamples = append(nsamples,
			sdk.Sample{Labels: map[string]string{"node": node, "status": "Ready"}, Value: boolF(ready)},
			sdk.Sample{Labels: map[string]string{"node": node, "status": "NotReady"}, Value: boolF(!ready)},
		)
		if isMaster(n) {
			masters++
		} else {
			workers++
		}

		if v := n.Status.Capacity["cpu"]; v != "" {
			cpuCap = append(cpuCap, sdk.Sample{Labels: map[string]string{"node": node}, Value: parseQuantity(v)})
		}
		if v := n.Status.Capacity["memory"]; v != "" {
			memCap = append(memCap, sdk.Sample{Labels: map[string]string{"node": node}, Value: parseQuantity(v) / 1024})
		}
		if v := n.Status.Allocatable["cpu"]; v != "" {
			cpuAlloc = append(cpuAlloc, sdk.Sample{Labels: map[string]string{"node": node}, Value: parseQuantity(v)})
		}
		if v := n.Status.Allocatable["memory"]; v != "" {
			memAlloc = append(memAlloc, sdk.Sample{Labels: map[string]string{"node": node}, Value: parseQuantity(v) / 1024})
		}

		for _, c := range n.Status.Conditions {
			for _, p := range pressureConditions {
				if c.Type == p {
					conds = append(conds, sdk.Sample{
						Labels: map[string]string{"node": node, "condition": c.Type},
						Value:  boolF(c.Status == "True"),
					})
				}
			}
		}
	}

	out = append(out,
		metric("k8s_node_count", helpNodeCount, []string{"role"}, []sdk.Sample{
			{Labels: map[string]string{"role": "master"}, Value: masters},
			{Labels: map[string]string{"role": "worker"}, Value: workers},
		}),
		metric("k8s_node_status", helpNodeStatus, []string{"node", "status"}, nsamples),
		metric("k8s_node_cpu_capacity", helpNodeCPUCapacity, []string{"node"}, cpuCap),
		metric("k8s_node_memory_capacity", helpNodeMemCapacity, []string{"node"}, memCap),
		metric("k8s_node_cpu_allocatable", helpNodeCPUAllocatable, []string{"node"}, cpuAlloc),
		metric("k8s_node_memory_allocatable", helpNodeMemAllocatable, []string{"node"}, memAlloc),
		metric("k8s_node_condition", helpNodeCondition, []string{"node", "condition"}, conds),
	)
	return out
}
