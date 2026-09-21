package main

import (
	"strconv"
	"strings"
)

// ===== 简化资源类型（JSON 只取用到的字段）=====

type k8sNode struct {
	Metadata struct {
		Name   string            `json:"name"`
		Labels map[string]string `json:"labels"`
	} `json:"metadata"`
	Status struct {
		Conditions []struct {
			Type   string `json:"type"`
			Status string `json:"status"`
		} `json:"conditions"`
		Capacity    map[string]string `json:"capacity"`
		Allocatable map[string]string `json:"allocatable"`
	} `json:"status"`
}

type k8sNodeList struct {
	Items []k8sNode `json:"items"`
}

type k8sContainerSpec struct {
	Name      string `json:"name"`
	Resources struct {
		Requests map[string]string `json:"requests"`
		Limits   map[string]string `json:"limits"`
	} `json:"resources"`
}

type k8sContainerStatus struct {
	Name         string `json:"name"`
	Ready        bool   `json:"ready"`
	RestartCount int    `json:"restartCount"`
	State        struct {
		Waiting *struct {
			Reason string `json:"reason"`
		} `json:"waiting"`
	} `json:"state"`
}

type k8sPod struct {
	Metadata struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"metadata"`
	Spec struct {
		NodeName   string             `json:"nodeName"`
		Containers []k8sContainerSpec `json:"containers"`
	} `json:"spec"`
	Status struct {
		Phase             string               `json:"phase"`
		ContainerStatuses []k8sContainerStatus `json:"containerStatuses"`
	} `json:"status"`
}

type k8sPodList struct {
	Items []k8sPod `json:"items"`
}

// k8sWorkload 覆盖 deployment / statefulset / daemonset 的公共字段
type k8sWorkload struct {
	Metadata struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"metadata"`
	Spec struct {
		Replicas *int32 `json:"replicas"`
	} `json:"spec"`
	Status struct {
		AvailableReplicas      int32 `json:"availableReplicas"`
		ReadyReplicas          int32 `json:"readyReplicas"`
		DesiredNumberScheduled int32 `json:"desiredNumberScheduled"`
		NumberReady            int32 `json:"numberReady"`
	} `json:"status"`
}

type k8sWorkloadList struct {
	Items []k8sWorkload `json:"items"`
}

type k8sPVC struct {
	Metadata struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"metadata"`
	Status struct {
		Phase string `json:"phase"`
	} `json:"status"`
}

type k8sPVCList struct {
	Items []k8sPVC `json:"items"`
}

// k8sPodMetrics pod 实时用量（metrics.k8s.io）
type k8sPodMetrics struct {
	Metadata struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"metadata"`
	Containers []struct {
		Name  string            `json:"name"`
		Usage map[string]string `json:"usage"`
	} `json:"containers"`
}

type k8sPodMetricsList struct {
	Items []k8sPodMetrics `json:"items"`
}

// parseQuantity 解析 k8s 资源数量字符串为基础单位浮点值：
// CPU → 核（"100m" = 0.1）；内存 → bytes（"10Mi"）。
// 覆盖常见格式：Ki|Mi|Gi|Ti / k|K|M|G / m / 纯数字。
func parseQuantity(s string) float64 {
	if s == "" {
		return 0
	}
	for _, p := range []struct {
		suf string
		mul float64
	}{
		{"Ki", 1 << 10}, {"Mi", 1 << 20}, {"Gi", 1 << 30}, {"Ti", 1 << 40},
		{"k", 1e3}, {"K", 1e3}, {"M", 1e6}, {"G", 1e9}, {"T", 1e12},
	} {
		if strings.HasSuffix(s, p.suf) {
			n, _ := strconv.ParseFloat(strings.TrimSuffix(s, p.suf), 64)
			return n * p.mul
		}
	}
	if strings.HasSuffix(s, "m") {
		n, _ := strconv.ParseFloat(strings.TrimSuffix(s, "m"), 64)
		return n / 1000
	}
	n, _ := strconv.ParseFloat(s, 64)
	return n
}
