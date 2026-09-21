package main

import (
	"context"

	"agent-plugs/sdk"
)

const (
	helpPVCStatus = "k8s PVC 状态 (1=处于该 phase)"
)

// appendStorage 存储域：PVC 状态（Bound / Pending / Lost，Pending 和 Lost 是存储坑）。
func appendStorage(out []sdk.DeclaredMetric, ctx context.Context, allow []string) []sdk.DeclaredMetric {
	var list k8sPVCList
	if err := kc.getJSON(ctx, "/api/v1/persistentvolumeclaims", &list); err != nil {
		sdk.Fprintln("[k8s] list pvcs:", err)
		return out
	}

	var status []sdk.Sample
	for _, p := range list.Items {
		ns, name := p.Metadata.Namespace, p.Metadata.Name
		if !nsAllowed(ns, allow) {
			continue
		}
		phase := p.Status.Phase
		if phase == "" {
			phase = "Unknown"
		}
		status = append(status, sdk.Sample{
			Labels: map[string]string{"namespace": ns, "pvc": name, "phase": phase}, Value: 1,
		})
	}

	out = append(out, metric("k8s_pvc_status", helpPVCStatus, []string{"namespace", "pvc", "phase"}, status))
	return out
}
