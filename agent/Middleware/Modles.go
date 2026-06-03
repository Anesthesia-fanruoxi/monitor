package Middleware

import "time"

// FlatSystemInfo 用于扁平化系统信息
type FlatSystemInfo struct {
	CPUPercent        float64 `json:"cpu_percent"`         // CPU 使用率百分比
	DiskTotal         uint64  `json:"disk_total"`          // 总磁盘空间（字节）
	DiskUsed          uint64  `json:"disk_used"`           // 已用磁盘空间（字节）
	DiskFree          uint64  `json:"disk_free"`           // 剩余磁盘空间（字节）
	DiskUsedPercent   float64 `json:"disk_used_percent"`   // 磁盘使用百分比
	MemoryTotal       uint64  `json:"memory_total"`        // 总内存（字节）
	MemoryUsed        uint64  `json:"memory_used"`         // 已用内存（字节）
	MemoryFree        uint64  `json:"memory_free"`         // 剩余内存（字节）
	MemoryBuffered    uint64  `json:"memory_buffered"`     // 缓存内存（字节）
	MemoryCached      uint64  `json:"memory_cached"`       // 被缓存的内存（字节）
	MemoryShared      uint64  `json:"memory_shared"`       // 共享内存（字节）
	MemoryAvailable   uint64  `json:"memory_available"`    // 可用内存（字节）
	MemoryUsedPercent float64 `json:"memory_used_percent"` // 内存使用百分比
	CPULoad1          float64 `json:"cpu_load_1"`          // 1分钟CPU负载
	CPULoad5          float64 `json:"cpu_load_5"`          // 5分钟CPU负载
	CPULoad15         float64 `json:"cpu_load_15"`         // 15分钟CPU负载
	HostName          string  `json:"hostName"`            // 主机名
	CPUCount          int     `json:"cpu_count"`           // CPU 核心数
	CPUModel          string  `json:"cpu_model"`           // CPU 型号
	OSVersion         string  `json:"os_version"`          // 操作系统版本
	KernelVersion     string  `json:"kernel_version"`      // 内核版本
}

// DomainInfo 用于保存域名、备注和证书信息的结构体
type DomainInfo struct {
	Domain     string    `json:"domain"`     // 域名
	Comment    string    `json:"comment"`    // 备注
	Expiration time.Time `json:"expiration"` // 证书过期时间
	DaysLeft   int       `json:"days_left"`  // 距离证书过期的天数
	Status     string    `json:"status"`     // 证书状态（有效、过期等）
	Resolve    bool      `json:"resolve"`    // 域名解析状态
}

// HarborInfo 用于保存任务信息
type HarborInfo struct {
	HostName       string `json:"hostName"`       // 主机名
	LoginUserCount int    `json:"loginUserCount"` // 登录用户数量
}

// SendData 用于发送数据的结构体
type SendDataType struct {
	PROJECT   string      `json:"project"`   // 项目名称
	Data      interface{} `json:"data"`      // 数据内容
	Timestamp int64       `json:"timestamp"` // 时间戳
	SOURCE    string      `json:"source"`    // 数据来源
}

// Config 用于存储配置信息
type Config struct {
	PROJECT string // 项目名称
	URL     string // 配置的 URL
	Ssl     bool   // 是否启用 SSL
	Nginx   bool   // 是否启用 Nginx
	Harbor  bool   // 是否启用 Harbor
	K8S     bool   // 是否启用 Kubernetes
}

// NginxStatus 定义了 Nginx 服务的状态
type NginxStatus struct {
	IsRun          int    `json:"isRun"`          // Nginx 是否正在运行
	ReTotal        int    `json:"reTotal"`        // 重启总次数
	LoginUserCount int    `json:"loginUserCount"` // 登录用户数量
	RawTotal       int    `json:"rawTotal"`       // 原始连接数
	Udptotal       int    `json:"udptotal"`       // UDP 连接数
	TcpTotal       int    `json:"tcpTotal"`       // TCP 连接数
	TotalTcp       int    `json:"totaltcp"`       // 总 TCP 连接数
	InetTotal      int    `json:"inetTotal"`      // Inet 连接数
	FragTotal      int    `json:"fragTotal"`      // 碎片总数
	TcpEstab       int    `json:"tcpEstab"`       // 已建立的 TCP 连接数
	TcpClosed      int    `json:"tcpClosed"`      // 已关闭的 TCP 连接数
	TcpOrphaned    int    `json:"tcpOrphaned"`    // 孤儿 TCP 连接数
	TcpTimewait    int    `json:"tcpTimewait"`    // TIME_WAIT 状态的 TCP 连接数
	HostName       string `json:"hostName"`       // 主机名
}

// HeartSource 定义心跳数据
type HeartSource struct {
	IsActive int     `json:"isActive"` // 是否活跃（1：活跃，0：不活跃）
	Project  string  `json:"project"`  // 项目名称
	Hostname string  `json:"hostname"` // 主机名
	Version  float64 `json:"version"`  //当前版本号
}

// ContainerResource 定义容器资源信息
type ContainerResource struct {
	Namespace           string  `json:"namespace"`           // Kubernetes 命名空间
	PodName             string  `json:"podName"`             // Pod 名称
	ControllerName      string  `json:"controllerName"`      // 控制器名称
	Container           string  `json:"container"`           // 容器名称
	LimitCpu            float64 `json:"limitCpu"`            // CPU 限制（核心数）
	LimitMemory         int64   `json:"limitMemory"`         // 内存限制（字节）
	RequestCpu          float64 `json:"requestCpu"`          // 请求的 CPU（核心数）
	RequestMemory       int64   `json:"requestMemory"`       // 请求的内存（字节）
	UseCpu              float64 `json:"useCpu"`              // 已使用的 CPU（核心数）
	UseMemory           int64   `json:"useMemory"`           // 已使用的内存（字节）
	RestartCount        int     `json:"restartCount"`        // 重启次数
	LastTerminationTime int64   `json:"lastTerminationTime"` //上一次终止时间
}

// ControllerResource 定义控制器资源信息
type ControllerResource struct {
	Namespace      string `json:"namespace"`       // Kubernetes 命名空间
	ControllerName string `json:"controller_name"` // 控制器名称
	Container      string `json:"container"`       // 容器名称
	ReplicaCount   int32  `json:"replica"`         // 副本数
}

// 配置结构体
type ConfigFile struct {
	Agent struct {
		Project    string `yaml:"project"`
		MetricsURL string `yaml:"metrics_url"`
		AutoUpdate bool   `yaml:"auto_update"`
	} `yaml:"agent"`
	Metrics struct {
		Ssl    struct{ Enable bool } `yaml:"ssl"`
		Nginx  struct{ Enable bool } `yaml:"nginx"`
		Harbor struct{ Enable bool } `yaml:"harbor"`
		K8S    struct {
			Enable     bool   `yaml:"enable"`
			ConfigPath string `yaml:"config_path"`
		} `yaml:"k8s"`
	} `yaml:"metrics"`
	Encrypted string `yaml:"encrypted"` // 加密密钥
}
