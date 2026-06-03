package main

import (
	"fmt"
	"github.com/fsnotify/fsnotify"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
	"log"
	"monitor-server/Handers"
	"monitor-server/IpPass"
	"monitor-server/Metrics"
	"net/http"
	"os"
	"time"
)

// 配置结构体
type Config struct {
	Encrypted string   `yaml:"encrypted"` // 加密盐
	IpPass    []string `yaml:"ipPass"`    // IP 白名单
}

// 读取配置文件的函数
func loadConfig(configFile string) (*Config, error) {
	file, err := os.Open(configFile)
	if err != nil {
		return nil, fmt.Errorf("打开配置文件失败: %v", err)
	}
	defer func(file *os.File) {
		if err := file.Close(); err != nil {
			log.Printf("关闭配置文件失败: %v", err)
		}
	}(file)

	var config Config
	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %v", err)
	}
	return &config, nil
}

// 动态加载配置并监听变化
func loadConfigWithViper() {
	viper.SetConfigFile("config/config.yaml")
	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("配置文件读取失败: %v", err)
	}

	// 监听配置文件变化
	viper.WatchConfig()
	viper.OnConfigChange(func(e fsnotify.Event) {
		log.Printf("配置文件已更新: %v", e.Name)
		Handers.SetEncryptionKey(viper.GetString("encrypted"))
		IpPass.SetAllowedDomains(viper.GetStringSlice("ipPass"))
	})
}

// 启动定时任务和心跳检查
func startHeartbeatChecks() {
	// 启动 goroutine
	go func() {
		for {
			Handers.CheckContainerHeartbeats()
			time.Sleep(5 * time.Second) // 每 5 秒检查一次
		}
	}()
	go func() {
		for {
			Handers.CheckHardHeartbeats()
			time.Sleep(5 * time.Second) // 每 5 秒检查一次
		}
	}()
	go func() {
		for {
			Handers.CheckHeartbeats()
			time.Sleep(5 * time.Second) // 每 5 秒检查一次
		}
	}()
	go func() {
		for {
			Handers.CheckSSLHeartbeats()
			time.Sleep(5 * time.Second) // 每 5 秒检查一次
		}
	}()
	go func() {
		for {
			Handers.CheckControllerHeartbeats()
			time.Sleep(5 * time.Second) // 每 5 秒检查一次
		}
	}()
	go func() {
		for {
			Handers.CheckNginxHeartbeats()
			time.Sleep(5 * time.Second) // 每 5 秒检查一次
		}
	}()
	go func() {
		for {
			IpPass.RefreshDomainIPCache()
			time.Sleep(5 * time.Minute) // 每 5 分钟刷新一次域名解析缓存
		}
	}()
	go func() {
		for {
			Handers.CheckTrafficSwitchingHeartbeats()
			time.Sleep(5 * time.Second) // 每 5 秒检查一次
		}
	}()
}

func main() {

	//var wg sync.WaitGroup
	// 加载配置文件
	config, err := loadConfig("config/config.yaml")
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 设置加密盐
	Handers.SetEncryptionKey(config.Encrypted)

	// 启动域名解析缓存的定时刷新功能
	err = Handers.LoadProjectDict("config/projects.json")
	if err != nil {
		log.Fatal(err)
	}

	// 设置 IP 白名单
	IpPass.SetAllowedDomains(config.IpPass)

	// 启动动态配置加载
	go loadConfigWithViper()

	// 启动定时任务和心跳检查
	go startHeartbeatChecks()

	// 暴露自定义指标
	metricsHandler := promhttp.HandlerFor(
		Metrics.CustomRegistry, // 使用自定义的 Registry
		promhttp.HandlerOpts{},
	)

	// 注册 `/metrics` 路径（带 IP 限制）
	http.Handle("/metrics", IpPass.IpRestrictionMiddleware(metricsHandler))

	// 设置 HTTP 接收端，传递 CustomRegistry 给 MetricsHandler
	http.HandleFunc("/metrics_data", func(w http.ResponseWriter, r *http.Request) {
		Handers.MetricsHandler(w, r, Metrics.CustomRegistry)
	})

	// 创建自定义 HTTP 服务器（配置超时）
	server := &http.Server{
		Addr:         ":8080",
		Handler:      nil,
		ReadTimeout:  30 * time.Second,  // 读取请求超时
		WriteTimeout: 30 * time.Second,  // 写入响应超时
		IdleTimeout:  120 * time.Second, // 空闲连接超时
	}

	// 启动 HTTP 服务
	log.Println("服务启动，监听端口 8080...")
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("HTTP 服务启动失败: %v", err)
	}

}
