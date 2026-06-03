package main

import (
	"agent/Collect"
	"agent/Middleware"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"
)

var Version string // 版本号变量

const (
	PIDFILE     = "work.pid"
	RESTARTFLAG = "restart.flag"
)

func main() {
	if Version == "" {
		Version = "1.0"
	}
	log.Printf("当前版本号：%s\n", Version)

	daemonMode := flag.Bool("d", false, "守护模式运行（自动重启+防多开）")
	flag.Parse()

	if *daemonMode {
		runForever()
	} else {
		work()
	}
}

func writePID() {
	pid := os.Getpid()
	file, err := os.OpenFile(PIDFILE, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		log.Fatalf("无法创建 PID 文件: %v", err)
	}
	defer func() { _ = file.Close() }()

	_, err = file.WriteString(fmt.Sprintf("%d\n", pid))
	if err != nil {
		log.Fatalf("无法写入 PID 文件: %v", err)
	}
}

// 清理PID文件
func cleanPID() {
	if err := os.Remove(PIDFILE); err != nil && !os.IsNotExist(err) {
		log.Printf("清理 PID 文件失败: %v", err)
	}
}

// 守护模式：监控工作进程，支持自动重启和更新
func runForever() {
	log.Println("守护模式启动")

	// 检查是否已有进程运行（防止多开）
	if isAlreadyRunning() {
		log.Fatalf("已有实例在运行，退出")
	}

	writePID()
	defer cleanPID()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 获取当前可执行文件路径
	exePath, err := os.Executable()
	if err != nil {
		log.Fatalf("获取可执行文件路径失败: %v", err)
	}

	for {
		_ = os.Remove(RESTARTFLAG)

		log.Println("启动工作进程...")
		cmd := exec.Command(exePath)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Start(); err != nil {
			log.Printf("启动失败: %v，5秒后重试", err)
			time.Sleep(5 * time.Second)
			continue
		}

		log.Printf("工作进程PID: %d", cmd.Process.Pid)

		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()

		select {
		case <-sigChan:
			log.Println("收到退出信号，正在停止...")
			if cmd.Process != nil {
				_ = cmd.Process.Signal(syscall.SIGTERM)
				select {
				case <-done:
				case <-time.After(10 * time.Second):
					_ = cmd.Process.Kill()
				}
			}
			log.Println("已停止")
			return

		case err := <-done:
			if _, e := os.Stat(RESTARTFLAG); e == nil {
				log.Println("检测到更新，立即重启...")
				continue
			}
			if err != nil {
				log.Printf("工作进程异常退出: %v，5秒后重启", err)
			} else {
				log.Println("工作进程退出，5秒后重启")
			}
			time.Sleep(5 * time.Second)
		}
	}
}

// 检查是否已有进程运行
func isAlreadyRunning() bool {
	data, err := os.ReadFile(PIDFILE)
	if err != nil {
		return false
	}
	var pid int
	if _, err := fmt.Sscanf(string(data), "%d", &pid); err != nil {
		return false
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return process.Signal(syscall.Signal(0)) == nil
}

func work() {
	// 加载配置
	config, err := Middleware.LoadConfig()
	if err != nil {
		log.Fatalf("加载配置文件失败: %v", err)
	}

	// 设置信号处理，支持优雅退出
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 主循环，保持程序运行
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	// 如果启用了自动更新，启动心跳检查
	if config.Agent.AutoUpdate {
		log.Printf("已开启自动更新")
		go Middleware.AutoChecks(Version)
	}

	log.Printf("Agent 已启动，PID: %d", os.Getpid())

	// 主循环处理数据收集和发送
	for {
		select {
		case sig := <-sigChan:
			log.Printf("收到信号 %v，正在优雅退出...", sig)
			cleanPID()
			log.Println("Agent 已停止")
			return
		case <-ticker.C:
			// 收集并发送数据
			Collect.CollectAndSendMetrics(Version, config)
		}
	}
}
