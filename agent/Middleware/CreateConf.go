package Middleware

import (
	"gopkg.in/yaml.v3"
	"log"
	"os"
	"sync"
	"time"
)

var (
	cachedConfig     *ConfigFile
	configMutex      sync.RWMutex
	configLastLoaded time.Time
	configCacheTTL   = 30 * time.Second // 配置缓存有效期
)

// 加载配置函数（带缓存，避免重复读取文件）
func LoadConfig() (ConfigFile, error) {
	configMutex.RLock()
	// 检查缓存是否有效
	if cachedConfig != nil && time.Since(configLastLoaded) < configCacheTTL {
		config := *cachedConfig
		configMutex.RUnlock()
		return config, nil
	}
	configMutex.RUnlock()

	// 缓存失效，重新加载
	configMutex.Lock()
	defer configMutex.Unlock()

	// 双重检查
	if cachedConfig != nil && time.Since(configLastLoaded) < configCacheTTL {
		return *cachedConfig, nil
	}

	var config ConfigFile
	filePath := "config.yaml"

	// 检查配置文件是否存在
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		log.Println("配置文件不存在，创建默认配置文件...")

		// 创建带注释的默认配置文件内容
		defaultConfig := `# 配置文件示例
# 基础配置
agent:

  # 项目名称，用于区分项目
  project: ""

  # 接受数据地址
  metrics_url: ""

  # 是否开启自动更新
  auto_update: true

# 是否开启采集,true为开启，false为不开启
metrics:

  # 是否开启采集ssl证书到期时间
  ssl:
    enable: false

  # 是否开启Nginx服务器信息采集
  nginx: 
    enable: false

  # 是否开启harbor服务信息采集
  harbor:
    enable: false

  # 是否开启采集k8s集群pod资源信息
  k8s: 
    enable: false
    # 开启之后需要填入路径，如果是当前路径直接写admin.conf,如果不是就写绝对路径
    config_path: ""

# 加密盐，数据加密传输
encrypted: ""`

		// 创建并写入配置文件
		file, err := os.Create(filePath)
		if err != nil {
			return config, err
		}
		defer func() { _ = file.Close() }()

		// 写入带注释的默认配置内容
		if _, err := file.WriteString(defaultConfig); err != nil {
			return config, err
		}

		log.Println("默认配置文件已创建，请修改后重新运行程序。")
		return config, nil
	}

	// 如果配置文件存在，读取配置
	file, err := os.Open(filePath)
	if err != nil {
		return config, err
	}
	defer func() { _ = file.Close() }()

	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		return config, err
	}

	// 更新缓存
	cachedConfig = &config
	configLastLoaded = time.Now()

	return config, nil
}
