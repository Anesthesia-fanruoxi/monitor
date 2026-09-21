package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"monitor-server/common"
	"monitor-server/config"
	"monitor-server/ippass"
	"monitor-server/router"
	"monitor-server/store"
)

func init() {
	// GOMAXPROCS 显式设为物理核数，与 Go 1.21 前默认行为一致
	// 显式设置的好处：启动日志一眼能看到运行时意图，运维侧如果需要调优也有明确入口
	_ = runtime.GOMAXPROCS(runtime.NumCPU())
}

func main() {
	// 加载配置文件
	cfg, err := config.LoadConfig("config/config.yaml")
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 设置加密盐（环境变量优先，便于密钥不进仓库）
	// 密钥非法时直接退出：空密钥会让 AES 初始化失败，表现为"所有上报都 400"
	if err := common.SetEncryptionKey(config.ResolveEncrypted(cfg.Encrypted)); err != nil {
		log.Fatalf("加密密钥配置错误: %v。请在 config/config.yaml 的 encrypted 或环境变量 %s 中配置 16/24/32 字节的密钥", err, config.EnvEncrypted)
	}

	// 插件制品仓库下载签名（HMAC key 复用 AES 密钥）
	common.SetHMACKey([]byte(config.ResolveEncrypted(cfg.Encrypted)))

	// 设置 IP 白名单与代理头识别策略
	ippass.SetTrustProxy(cfg.TrustProxyEnabled())
	ippass.SetAllowedDomains(cfg.IpPass)

	// 启动动态配置加载
	go config.LoadConfigWithViper()

	// 注册全部 HTTP 路由
	router.Register(cfg)

	// 定时刷新域名解析缓存（声明式路径不再需要 per-source 心跳检查，
	// 动态指标的超时注销由 store.StartDynamicSweeper 统一处理）
	go func() {
		for {
			ippass.RefreshDomainIPCache()
			time.Sleep(5 * time.Minute)
		}
	}()

	// 动态指标的通用超时注销
	// 插件化 Agent 上报的指标自带注销阈值，由这里统一清扫，
	// 因此新增采集器不再需要在接收链路下新增心跳检查文件
	sweeperStop := make(chan struct{})
	store.StartDynamicSweeper(5*time.Second, sweeperStop)

	// 监听地址，默认 8080
	addr := ":8080"
	if p := strings.TrimSpace(cfg.Port); p != "" {
		if strings.HasPrefix(p, ":") {
			addr = p
		} else {
			addr = ":" + p
		}
	}

	// 创建自定义 HTTP 服务器（配置超时）
	server := &http.Server{
		Addr:           addr,
		Handler:        nil,
		ReadTimeout:    30 * time.Second,  // 读取请求超时
		WriteTimeout:   30 * time.Second,  // 写入响应超时
		IdleTimeout:    120 * time.Second, // 空闲连接超时
		MaxHeaderBytes: 64 * 1024,         // header 上限 64KB（Go 默认 1MB，有人发超大 header 可以打崩进程内存）
	}

	// 启动前打印系统环境：文件描述符限制和 listen backlog，
	// 低时显式告警，避免 1000+ Agent 同时上报时出现 connection refused
	printServerReady(addr)

	// 启动 HTTP 服务
	log.Printf("服务启动，监听端口 %s...", addr)
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("HTTP 服务启动失败: %v", err)
	}

}

// printServerReady 启动时打印与并发承载相关的系统参数，并在明显偏低时告警
func printServerReady(addr string) {
	log.Printf("=== 环境信息 ===")
	log.Printf("  Go 版本:    %s", runtime.Version())
	log.Printf("  OS/Arch:    %s %s", runtime.GOOS, runtime.GOARCH)
	log.Printf("  CPU 核心:   %d", runtime.NumCPU())

	// FD 限制 + somaxconn 检查：实现走 procfs 文本读取而非 syscall，
	// 因此无需 build tag 拆分文件；非 Linux 平台读不到文件会自动跳过
	printPlatformLimits()

	log.Printf("  监听地址:   %s", addr)
	log.Printf("================")
}

// printPlatformLimits 打印与并发承载相关的内核限制，明显偏低时给出调优建议
// 实现全部走 procfs 文本读取（不使用 syscall），因此在任何平台都能编译通过，
// 非 Linux 平台读不到这些文件会自动跳过，不需要 build tag 拆分文件
func printPlatformLimits() {
	// 文件描述符限制：从 /proc/self/limits 取 "Max open files" 行
	// 形如：Max open files            1024                 1048576              files
	if data, err := os.ReadFile("/proc/self/limits"); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if !strings.HasPrefix(line, "Max open files") {
				continue
			}
			fields := strings.Fields(line)
			if len(fields) >= 5 {
				log.Printf("  FD 限制:    soft=%s hard=%s", fields[3], fields[4])
				var soft int64
				// 软限制可能写成 "unlimited"，解析失败就跳过告警
				if _, err := fmt.Sscanf(fields[3], "%d", &soft); err == nil && soft < 2048 {
					log.Printf("  ⚠️  软限制过低（%d），建议调高到 8192+ 以支持高并发 Agent 上报", soft)
					log.Printf("     sysctl -w fs.file-max=65536  # 系统级")
					log.Printf("     ulimit -n 8192               # 当前 shell")
				}
			}
			break
		}
	}

	// Listen backlog（somaxconn）
	if data, err := os.ReadFile("/proc/sys/net/core/somaxconn"); err == nil {
		somax := strings.TrimSpace(string(data))
		if somax == "" {
			return
		}
		log.Printf("  somaxconn:  %s", somax)
		var v int
		if _, err := fmt.Sscanf(somax, "%d", &v); err == nil && v < 2048 {
			log.Printf("  ⚠️  somaxconn 偏低，建议调高到 4096+ 以避免突发连接被拒绝")
			log.Printf("     sysctl -w net.core.somaxconn=4096")
		}
	}
}
