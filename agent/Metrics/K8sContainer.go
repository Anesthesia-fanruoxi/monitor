package Metrics

import (
	"context"
	"fmt"
	"sync"
	"time"

	"agent/Middleware"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	metricsclient "k8s.io/metrics/pkg/client/clientset/versioned"
)

// K8s 客户端缓存
var (
	k8sClientset     *kubernetes.Clientset
	k8sMetricsClient *metricsclient.Clientset
	k8sClientMutex   sync.RWMutex
	k8sLastConfig    string
)

// 初始化 Kubernetes 客户端和 Metrics 客户端（带缓存复用）
func InitializeClients(kubeconfig string) (*kubernetes.Clientset, *metricsclient.Clientset, error) {
	// 检查缓存
	k8sClientMutex.RLock()
	if k8sClientset != nil && k8sMetricsClient != nil && k8sLastConfig == kubeconfig {
		clientset := k8sClientset
		metricsClient := k8sMetricsClient
		k8sClientMutex.RUnlock()
		return clientset, metricsClient, nil
	}
	k8sClientMutex.RUnlock()

	// 缓存未命中，重新创建
	k8sClientMutex.Lock()
	defer k8sClientMutex.Unlock()

	// 双重检查
	if k8sClientset != nil && k8sMetricsClient != nil && k8sLastConfig == kubeconfig {
		return k8sClientset, k8sMetricsClient, nil
	}

	// 构建 Kubernetes 配置
	var config *rest.Config
	var err error

	// 如果没有提供 kubeconfig 参数，则使用集群内的默认配置（InClusterConfig）
	if kubeconfig == "" {
		config, err = rest.InClusterConfig() // 从集群内部环境读取配置
		if err != nil {
			return nil, nil, fmt.Errorf("无法获取集群内部配置: %v", err)
		}
	} else {
		// 使用 kubeconfig 文件的配置
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, nil, fmt.Errorf("无法构建 Kubernetes 配置: %v", err)
		}
	}

	// 创建 Kubernetes 客户端
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, nil, fmt.Errorf("无法创建 Kubernetes 客户端: %v", err)
	}

	// 创建 Metrics 客户端
	metricsClient, err := metricsclient.NewForConfig(config)
	if err != nil {
		return nil, nil, fmt.Errorf("无法创建 Metrics 客户端: %v", err)
	}

	// 更新缓存
	k8sClientset = clientset
	k8sMetricsClient = metricsClient
	k8sLastConfig = kubeconfig

	return clientset, metricsClient, nil
}

// 获取所有 Pod 和容器的资源状态
func GetPodResources(clientset *kubernetes.Clientset, metricsClient *metricsclient.Clientset) ([]Middleware.ContainerResource, error) {
	//start := time.Now() // 记录获取 Pod 资源信息的开始时间

	var containerResources []Middleware.ContainerResource

	// 获取所有 namespaces 下的所有 Pods
	pods, err := clientset.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("无法获取所有 Pods: %v", err)
	}

	// 获取所有 namespaces 下的所有 PodMetrics
	podMetrics, err := metricsClient.MetricsV1beta1().PodMetricses("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("无法获取所有 PodMetrics: %v", err)
	}

	// 遍历所有 Pods 获取资源信息
	for _, pod := range pods.Items {
		controllerName := getControllerName(&pod)
		for _, container := range pod.Spec.Containers {
			// 获取 CPU 和内存限制与请求
			limitCpu := getCpuLimitOrRequest(container.Resources.Limits[corev1.ResourceCPU])
			limitMemory := getMemoryLimitOrRequest(container.Resources.Limits[corev1.ResourceMemory])
			requestCpu := getCpuLimitOrRequest(container.Resources.Requests[corev1.ResourceCPU])
			requestMemory := getMemoryLimitOrRequest(container.Resources.Requests[corev1.ResourceMemory])

			// 查找对应的 PodMetrics
			var cpuUsage, memUsage int64
			for _, podMetrics := range podMetrics.Items {
				if podMetrics.Name == pod.Name && podMetrics.Namespace == pod.Namespace {
					for _, containerMetrics := range podMetrics.Containers {
						if containerMetrics.Name == container.Name {
							// 获取 CPU 和内存使用情况
							cpuUsage = getCpuUsage(containerMetrics.Usage["cpu"])
							memUsage = getMemoryUsage(containerMetrics.Usage["memory"])
						}
					}
				}
			}

			// 获取容器的重启次数
			restartCount := getRestartCount(container.Name, pod.Status.ContainerStatuses)

			// 获取容器的上一次终止时间
			var lastTerminationTimeUnix int64
			var lastTerminationTime int64 // 用于存储时间差（以分钟为单位）
			for _, containerStatus := range pod.Status.ContainerStatuses {
				if containerStatus.Name == container.Name && containerStatus.LastTerminationState.Terminated != nil {
					// 转换为 Unix 毫秒时间戳
					lastTerminationTimeUnix = containerStatus.LastTerminationState.Terminated.FinishedAt.UnixMilli()

					// 计算当前时间与上次终止时间的时间差（单位：毫秒）
					now := time.Now().UnixMilli() // 当前时间的毫秒时间戳
					timeDiff := now - lastTerminationTimeUnix

					// 如果时间差大于等于一分钟，计算分钟数
					if timeDiff >= 60000 { // 如果大于等于一分钟
						lastTerminationTime = timeDiff / 60000 // 转换为分钟
					} else {
						lastTerminationTime = 0 // 小于一分钟，忽略
					}
				}
			}

			// 创建容器资源数据
			containerResource := Middleware.ContainerResource{
				Namespace:           pod.Namespace,
				PodName:             pod.Name,
				ControllerName:      controllerName,
				Container:           container.Name,
				LimitCpu:            limitCpu,
				LimitMemory:         limitMemory,
				RequestCpu:          requestCpu,
				RequestMemory:       requestMemory,
				UseCpu:              float64(cpuUsage) / 1000.0,
				UseMemory:           memUsage,
				RestartCount:        int(restartCount),   // 设置重启次数
				LastTerminationTime: lastTerminationTime, // 新增字段，转换为秒
			}

			// 添加到结果集中
			containerResources = append(containerResources, containerResource)
		}
	}

	//log.Printf("获取所有 Pod 资源信息总耗时: %v", time.Since(start)) // 打印获取 Pod 资源的总耗时
	return containerResources, nil
}

// 获取容器的重启次数
func getRestartCount(containerName string, containerStatuses []corev1.ContainerStatus) int32 {
	for _, status := range containerStatuses {
		if status.Name == containerName {
			return status.RestartCount // 返回容器的重启次数
		}
	}
	return 0 // 如果没有找到对应容器，返回 0
}

// 获取控制器名称（如 ReplicaSet、Deployment 等）
func getControllerName(pod *corev1.Pod) string {
	for _, ownerRef := range pod.OwnerReferences {
		if ownerRef.Kind == "ReplicaSet" || ownerRef.Kind == "Deployment" || ownerRef.Kind == "StatefulSet" {
			return ownerRef.Name
		}
	}
	return "无控制器"
}

// 获取 CPU 的资源限制或请求（返回核心数）
func getCpuLimitOrRequest(quantity resource.Quantity) float64 {
	if quantity.IsZero() {
		return 0.0
	}
	return quantity.AsApproximateFloat64() // 返回核心数
}

// 获取内存的资源限制或请求（返回字节数）
func getMemoryLimitOrRequest(quantity resource.Quantity) int64 {
	if quantity.IsZero() {
		return 0
	}
	return quantity.Value() // 返回字节数
}

// 获取 CPU 使用量（返回毫核）
func getCpuUsage(cpu resource.Quantity) int64 {
	if cpu.IsZero() {
		return 0
	}
	return cpu.MilliValue() // 返回毫核
}

// 获取内存使用量（返回字节数）
func getMemoryUsage(memory resource.Quantity) int64 {
	if memory.IsZero() {
		return 0
	}
	return memory.Value() // 返回字节数
}
