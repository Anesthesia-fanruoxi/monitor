package main

import (
	"context"

	"agent-plugs/sdk"
)

const (
	helpDeploymentReplicas  = "k8s deployment 期望副本数"
	helpDeploymentAvailable = "k8s deployment 可用副本数"
	helpStsReplicas         = "k8s statefulset 期望副本数"
	helpStsReadyReplicas    = "k8s statefulset 就绪副本数"
	helpDsDesired           = "k8s daemonset 期望节点数"
	helpDsReady             = "k8s daemonset 就绪节点数"
)

// appendWorkloads 工作负载域：deployment / statefulset / daemonset 副本数。
func appendWorkloads(out []sdk.DeclaredMetric, ctx context.Context, allow []string) []sdk.DeclaredMetric {
	var deps k8sWorkloadList
	if err := kc.getJSON(ctx, "/apis/apps/v1/deployments", &deps); err != nil {
		sdk.Fprintln("[k8s] list deployments:", err)
	} else {
		var rep, avail []sdk.Sample
		for _, d := range deps.Items {
			ns, name := d.Metadata.Namespace, d.Metadata.Name
			if !nsAllowed(ns, allow) {
				continue
			}
			rep = append(rep, sdk.Sample{
				Labels: map[string]string{"namespace": ns, "deployment": name},
				Value:  float64(replicasOrDefault(d.Spec.Replicas)),
			})
			avail = append(avail, sdk.Sample{
				Labels: map[string]string{"namespace": ns, "deployment": name},
				Value:  float64(d.Status.AvailableReplicas),
			})
		}
		out = append(out,
			metric("k8s_deployment_replicas", helpDeploymentReplicas, []string{"namespace", "deployment"}, rep),
			metric("k8s_deployment_available_replicas", helpDeploymentAvailable, []string{"namespace", "deployment"}, avail),
		)
	}

	var sts k8sWorkloadList
	if err := kc.getJSON(ctx, "/apis/apps/v1/statefulsets", &sts); err != nil {
		sdk.Fprintln("[k8s] list statefulsets:", err)
	} else {
		var rep, ready []sdk.Sample
		for _, s := range sts.Items {
			ns, name := s.Metadata.Namespace, s.Metadata.Name
			if !nsAllowed(ns, allow) {
				continue
			}
			rep = append(rep, sdk.Sample{
				Labels: map[string]string{"namespace": ns, "statefulset": name},
				Value:  float64(replicasOrDefault(s.Spec.Replicas)),
			})
			ready = append(ready, sdk.Sample{
				Labels: map[string]string{"namespace": ns, "statefulset": name},
				Value:  float64(s.Status.ReadyReplicas),
			})
		}
		out = append(out,
			metric("k8s_statefulset_replicas", helpStsReplicas, []string{"namespace", "statefulset"}, rep),
			metric("k8s_statefulset_ready_replicas", helpStsReadyReplicas, []string{"namespace", "statefulset"}, ready),
		)
	}

	var ds k8sWorkloadList
	if err := kc.getJSON(ctx, "/apis/apps/v1/daemonsets", &ds); err != nil {
		sdk.Fprintln("[k8s] list daemonsets:", err)
	} else {
		var desired, ready []sdk.Sample
		for _, d := range ds.Items {
			ns, name := d.Metadata.Namespace, d.Metadata.Name
			if !nsAllowed(ns, allow) {
				continue
			}
			desired = append(desired, sdk.Sample{
				Labels: map[string]string{"namespace": ns, "daemonset": name},
				Value:  float64(d.Status.DesiredNumberScheduled),
			})
			ready = append(ready, sdk.Sample{
				Labels: map[string]string{"namespace": ns, "daemonset": name},
				Value:  float64(d.Status.NumberReady),
			})
		}
		out = append(out,
			metric("k8s_daemonset_desired", helpDsDesired, []string{"namespace", "daemonset"}, desired),
			metric("k8s_daemonset_ready", helpDsReady, []string{"namespace", "daemonset"}, ready),
		)
	}

	return out
}
