package Collect

import (
	"agent/Metrics"
	"agent/Middleware"
	"encoding/json"
	"log"
	"strconv"
	"time"
)

// 采集数据并发送数据的封装方法
// 差异化采集频率：心跳/硬件 15秒，Nginx 15秒，K8s 60秒，SSL 5分钟
func CollectAndSendMetrics(Version string, config Middleware.ConfigFile) {
	currentSecond := time.Now().Second()
	currentMinute := time.Now().Minute()

	// 每15秒：心跳、硬件、Nginx
	if currentSecond%15 == 0 {
		collectBaseMetrics(Version, config)
	}

	// 每60秒（整分钟）：K8s
	if currentSecond == 0 && config.Metrics.K8S.Enable {
		collectK8sMetrics(config)
	}

	// 每5分钟：SSL证书
	if currentSecond == 0 && currentMinute%5 == 0 && config.Metrics.Ssl.Enable {
		collectSslMetrics(config)
	}
}

// 基础数据采集：心跳、硬件、Nginx（15秒）
func collectBaseMetrics(Version string, config Middleware.ConfigFile) {
	// 获取主机信息
	hostInfoSlice, err := Metrics.GetHostInfo()
	if err != nil {
		log.Printf("获取主机信息失败: %v", err)
	} else {
		CollectAndSendData("hard", hostInfoSlice, config)
	}

	// 转换版本号为浮动类型
	versionFloat, err := strconv.ParseFloat(Version, 64)
	if err != nil {
		log.Printf("转换版本号失败: %v", err)
		return
	}

	// 获取心跳信息
	ActiveInfo, err := Metrics.IsActive(config.Agent.Project, versionFloat)
	if err != nil {
		log.Printf("获取心跳数据失败: %v", err)
	} else {
		CollectAndSendData("heart", ActiveInfo, config)
	}

	// 如果 Nginx 采集被启用
	if config.Metrics.Nginx.Enable {
		NginxInfo, err := Metrics.GetNginxInfo()
		if err != nil {
			log.Printf("获取 nginx 信息失败: %v", err)
		} else {
			CollectAndSendData("nginx", NginxInfo, config)
		}
	}
}

// K8s数据采集（60秒）
func collectK8sMetrics(config Middleware.ConfigFile) {
	clientset, metricsClient, err := Metrics.InitializeClients(config.Metrics.K8S.ConfigPath)
	if err != nil {
		log.Printf("初始化 Kubernetes 客户端失败: %v", err)
		return
	}

	containerResources, err := Metrics.GetPodResources(clientset, metricsClient)
	if err != nil {
		log.Printf("获取 Kubernetes 资源信息失败: %v", err)
	} else {
		CollectAndSendData("k8s", containerResources, config)
	}

	controllerResources, err := Metrics.GetControllerResources(clientset)
	if err != nil {
		log.Printf("获取 Kubernetes 控制器资源信息失败: %v", err)
	} else {
		CollectAndSendData("k8sController", controllerResources, config)
	}
}

// SSL证书数据采集（5分钟）
func collectSslMetrics(config Middleware.ConfigFile) {
	SslInfos, err := Metrics.GetSslInfo()
	if err != nil {
		log.Printf("获取 Ssl 信息失败: %v", err)
		return
	}
	var SslData []map[string]interface{}
	err = json.Unmarshal([]byte(SslInfos), &SslData)
	if err != nil {
		log.Printf("解析 Ssl 信息失败: %v", err)
		return
	}
	CollectAndSendData("ssl", SslData, config)
}

// 采集数据并发送数据的封装方法（简单异步发送，失败直接丢弃）
func CollectAndSendData(source string, data interface{}, config Middleware.ConfigFile) {
	metricsURL := config.Agent.MetricsURL + "/metrics_data"
	project := config.Agent.Project
	key := []byte(config.Encrypted)

	// 异步发送，不阻塞采集，监控数据可丢失
	go func() {
		_ = Middleware.SendData(metricsURL, project, data, key, source)
	}()
}
