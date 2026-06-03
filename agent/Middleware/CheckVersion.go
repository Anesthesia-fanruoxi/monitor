package Middleware

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

var mu sync.Mutex // 用于锁定并发执行

// 从服务器获取最新版本号
func getVersionFromServer(url string) (float64, error) {
	// 设置超时时间为5秒
	client := &http.Client{
		Timeout: 5 * time.Second,
	}
	resp, err := client.Get(url) // 发起 HTTP GET 请求
	if err != nil {
		return 0, fmt.Errorf("获取版本号失败: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// 读取响应体内容
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("读取响应体失败: %v", err)
	}

	// 假设返回的版本号是纯文本（如 "1.1"）
	remoteVersionStr := string(body)

	// 将返回的版本号字符串转换为 float64
	remoteVersion, err := strconv.ParseFloat(strings.TrimSpace(remoteVersionStr), 64)
	if err != nil {
		return 0, fmt.Errorf("转换版本号失败: %v", err)
	}

	return remoteVersion, nil
}

// 判断本地版本和远程版本是否不匹配
func isVersionsNotMatching(localVersion, remoteVersion float64) bool {
	// 检查 localVersion 或 remoteVersion 是否为默认值 0
	if localVersion == 0 || remoteVersion == 0 {
		// 如果其中一个版本为 0，则跳过
		return false
	}
	return localVersion != remoteVersion
}

// 执行更新：下载新版本并替换
func executeUpdate(version float64, url string) error {
	mu.Lock()
	defer mu.Unlock()

	downloadURL := fmt.Sprintf("%s/agent/agent", url)
	newBinary := "./agent-new"
	currentBinary := "./agent"

	log.Printf("开始下载新版本 %.2f...", version)

	// 下载新版本
	resp, err := http.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("下载失败: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载失败，HTTP状态码: %d", resp.StatusCode)
	}

	// 创建新文件
	out, err := os.OpenFile(newBinary, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return fmt.Errorf("创建文件失败: %v", err)
	}

	_, err = io.Copy(out, resp.Body)
	_ = out.Close()
	if err != nil {
		_ = os.Remove(newBinary)
		return fmt.Errorf("写入文件失败: %v", err)
	}

	log.Println("下载完成，正在替换二进制文件...")

	// 替换二进制文件
	if err := os.Rename(newBinary, currentBinary); err != nil {
		_ = os.Remove(newBinary)
		return fmt.Errorf("替换文件失败: %v", err)
	}

	log.Printf("更新完成，新版本: %.2f，正在原地重启...", version)

	// 使用 syscall.Exec 原地替换当前进程，PID不变，容器无感知
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("获取可执行文件路径失败: %v", err)
	}

	// syscall.Exec 会用新程序替换当前进程，不会返回
	err = syscall.Exec(exePath, os.Args, os.Environ())
	if err != nil {
		return fmt.Errorf("执行新版本失败: %v", err)
	}

	return nil
}

// 启动一个线程定期检查版本号
func CheckVersion(version string, url string) {
	for {
		remoteVersion, err := getVersionFromServer(fmt.Sprintf("%s/version", url))
		if err != nil {
			log.Printf("获取版本号失败: %v\n", err)
			time.Sleep(5 * time.Second) // 如果失败，稍后重试
			continue
		}
		localVersion, err := strconv.ParseFloat(version, 64)
		if err != nil {
			// 处理转换错误
			log.Println("转换错误:", err)
			return
		}

		// 比较本地版本与远程版本
		if isVersionsNotMatching(localVersion, remoteVersion) {
			log.Printf("发现新版本! 本地版本: %.2f, 远程版本: %.2f\n", localVersion, remoteVersion)
			if err := executeUpdate(remoteVersion, url); err != nil {
				log.Printf("更新失败: %v", err)
				continue
			}
		}

		// 等待 10 秒钟再检查一次
		time.Sleep(10 * time.Second)
	}
}

func AutoChecks(Version string) {
	config, err := LoadConfig()
	if err != nil {
		log.Fatalf("加载配置文件失败: %v", err)
	}
	// 启动单个 goroutine 执行版本检查（CheckVersion 内部已有循环）
	go CheckVersion(Version, config.Agent.MetricsURL)
}
