package main

import (
	"context"

	"agent-plugs/sdk"
)

const (
	helpContainerCPUUsage    = "k8s 容器 cpu 使用"
	helpContainerMemoryUsage = "k8s 容器内存使用 (kB)"
)

// appendUsage 容器实时用量（数据来自 metrics-server）：cpu (核) / 内存 (kB)。
func appendUsage(out []sdk.DeclaredMetric, ctx context.Context, allow []string) []sdk.DeclaredMetric {
	var list k8sPodMetricsList
	if err := kc.getJSON(ctx, "/apis/metrics.k8s.io/v1beta1/pods", &list); err != nil {
		sdk.Fprintln("[k8s] list podmetrics:", err)
		return out
	}

	var cpuUse, memUse []sdk.Sample
	for _, pm := range list.Items {
		ns, name := pm.Metadata.Namespace, pm.Metadata.Name
		if !nsAllowed(ns, allow) {
			continue
		}
		for _, c := range pm.Containers {
			cpu := parseQuantity(c.Usage["cpu"])
			mem := parseQuantity(c.Usage["memory"]) / 1024 // bytes → kB
			lbl := map[string]string{"namespace": ns, "pod": name, "container": c.Name}
			cpuUse = append(cpuUse, sdk.Sample{Labels: lbl, Value: cpu})
			memUse = append(memUse, sdk.Sample{Labels: lbl, Value: mem})
		}
	}

	out = append(out,
		metric("k8s_container_cpu_usage", helpContainerCPUUsage, []string{"namespace", "pod", "container"}, cpuUse),
		metric("k8s_container_memory_usage", helpContainerMemoryUsage, []string{"namespace", "pod", "container"}, memUse),
	)
	return out
}
