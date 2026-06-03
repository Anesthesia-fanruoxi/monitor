package Metrics

import (
	"context"
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1" // 正确的包名
	"k8s.io/client-go/kubernetes"
)

// 获取所有控制器的信息，包括 Deployment, DaemonSet, StatefulSet 等，并扁平化为 JSON 格式
func GetControllerResources(clientset *kubernetes.Clientset) ([]map[string]interface{}, error) {
	//start := time.Now() // 记录获取控制器信息的开始时间

	var controllerInfos []map[string]interface{}

	// 获取所有 Deployments
	deployments, err := clientset.AppsV1().Deployments("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("无法获取 Deployments: %v", err)
	}

	// 获取所有 DaemonSets
	daemonSets, err := clientset.AppsV1().DaemonSets("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("无法获取 DaemonSets: %v", err)
	}

	// 获取所有 StatefulSets
	statefulSets, err := clientset.AppsV1().StatefulSets("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("无法获取 StatefulSets: %v", err)
	}

	// 处理 Deployments
	for _, deployment := range deployments.Items {
		controllerInfos = append(controllerInfos, map[string]interface{}{
			"namespace":                   deployment.Namespace,
			"container":                   deployment.Name,
			"controllerType":              "Deployment",
			"replicas":                    *deployment.Spec.Replicas,
			"status_replicas_available":   deployment.Status.AvailableReplicas,
			"status_replicas_unavailable": deployment.Status.UnavailableReplicas,
		})
	}

	// 处理 DaemonSets
	for _, daemonSet := range daemonSets.Items {
		controllerInfos = append(controllerInfos, map[string]interface{}{
			"namespace":                   daemonSet.Namespace,
			"container":                   daemonSet.Name,
			"controllerType":              "DaemonSet",
			"replicas":                    daemonSet.Status.DesiredNumberScheduled, // 使用 DesiredNumberScheduled
			"status_replicas_available":   daemonSet.Status.NumberAvailable,        // 使用 NumberAvailable
			"status_replicas_unavailable": daemonSet.Status.NumberUnavailable,      // 使用 NumberUnavailable
		})
	}

	// 处理 StatefulSets
	for _, statefulSet := range statefulSets.Items {
		controllerInfos = append(controllerInfos, map[string]interface{}{
			"namespace":                   statefulSet.Namespace,
			"container":                   statefulSet.Name,
			"controllerType":              "StatefulSet",
			"replicas":                    *statefulSet.Spec.Replicas,
			"status_replicas_available":   statefulSet.Status.ReadyReplicas,                               // 使用 ReadyReplicas
			"status_replicas_unavailable": statefulSet.Status.Replicas - statefulSet.Status.ReadyReplicas, // 计算不可用副本数
		})
	}

	//log.Printf("获取控制器信息总耗时: %v", time.Since(start)) // 打印获取控制器信息的总耗时
	return controllerInfos, nil
}
