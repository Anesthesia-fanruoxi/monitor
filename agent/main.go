package main

import (
	"agent/common"
	"agent/router"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

var Version string // 版本号变量

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

// 守护模式重启退避：进程一起步就挂（如配置错误）时，避免每 5 秒无限重启刷日志
const (
	minRestartBackoff = 1 * time.Second
	maxRestartBackoff = 60 * time.Second
	// 子进程连续运行超过这个时长视为"健康跑过"，重置退避
	healthyRunDuration = 1 * time.Minute
	// 新版本连续 crash 达到这个次数，且每次都没健康跑过，自动回滚到 .bak
	rollbackThreshold = 5
)

func writePID() {
	pid := os.Getpid()
	pidFile := common.WorkPIDFile()
	file, err := os.OpenFile(pidFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		log.Fatalf("无法创建 PID 文件 %s: %v", pidFile, err)
	}
	defer func() { _ = file.Close() }()

	_, err = file.WriteString(fmt.Sprintf("%d\n", pid))
	if err != nil {
		log.Fatalf("无法写入 PID 文件 %s: %v", pidFile, err)
	}
}

// cleanPIDOwned 只删除本进程写入的 PID 文件
// 之前的实现无条件删除，非守护模式下退出会把守护进程的 work.pid 一并删掉
func cleanPIDOwned() {
	pidFile := common.WorkPIDFile()
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return
	}
	var pid int
	if _, err := fmt.Sscanf(string(data), "%d", &pid); err != nil {
		return
	}
	if pid != os.Getpid() {
		return
	}
	if err := os.Remove(pidFile); err != nil && !os.IsNotExist(err) {
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
	defer cleanPIDOwned()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 获取当前可执行文件路径
	exePath, err := os.Executable()
	if err != nil {
		log.Fatalf("获取可执行文件路径失败: %v", err)
	}

	backoff := minRestartBackoff
	crashCount := 0
	exeDir := filepath.Dir(exePath)
	baseName := filepath.Base(exePath)
	backupBinary := filepath.Join(exeDir, baseName+".bak")

	for {
		_ = os.Remove(common.RestartFlagPath())

		log.Println("启动工作进程...")
		cmd := exec.Command(exePath)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		// 关键：子进程 CWD 设为二进制所在目录
		// 这样 LoadConfig() 找 config.yaml、RestartFlag、PID 文件等
		// 都能正确落在同一位置，不依赖守护进程启动时的 CWD
		cmd.Dir = exeDir
		cmd.Env = append(os.Environ(), common.EnvDaemonChild+"=1")
		if err := cmd.Start(); err != nil {
			log.Printf("启动失败: %v，%s 后重试", err, backoff)
			time.Sleep(backoff)
			backoff = nextBackoff(backoff)
			continue
		}

		startedAt := time.Now()
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
			// 更新重启：新版本写 RestartFlag 后 Exit(0)，不计入 crash
			if flagData, e := os.ReadFile(common.RestartFlagPath()); e == nil {
				version := strings.TrimSpace(string(flagData))
				if version != "" {
					log.Printf("检测到更新 %s，立即重启...", version)
				} else {
					log.Println("检测到更新，立即重启...")
				}
				crashCount = 0
				backoff = minRestartBackoff
				continue
			}

			// 自动回滚：新版本连续 crash N 次（每次都没健康跑过），且 .bak 还在
			if time.Since(startedAt) < healthyRunDuration {
				crashCount++
			} else {
				crashCount = 0
			}
			if crashCount >= rollbackThreshold {
				if _, rollbackErr := os.Stat(backupBinary); rollbackErr == nil {
					log.Printf("⚠️  新版本连续崩溃 %d 次，自动回滚到备份 %s", crashCount, backupBinary)
					if renameErr := os.Rename(backupBinary, exePath); renameErr != nil {
						log.Printf("自动回滚失败: %v", renameErr)
					} else {
						log.Printf("已回滚，下次启动运行旧版本")
					}
				} else {
					log.Printf("新版本连续崩溃 %d 次，但无备份可回滚", crashCount)
				}
				crashCount = 0
				backoff = minRestartBackoff
				continue
			}

			if err != nil {
				log.Printf("工作进程异常退出: %v，%s 后重启（连续崩溃 %d/5）", err, backoff, crashCount)
			} else {
				log.Printf("工作进程退出，%s 后重启（连续崩溃 %d/5）", backoff, crashCount)
			}
			if time.Since(startedAt) > healthyRunDuration {
				backoff = minRestartBackoff
			}
			time.Sleep(backoff)
			backoff = nextBackoff(backoff)
		}
	}
}

// nextBackoff 指数退避，上限 maxRestartBackoff
func nextBackoff(current time.Duration) time.Duration {
	next := current * 2
	if next > maxRestartBackoff {
		return maxRestartBackoff
	}
	return next
}

// 检查是否已有进程运行
func isAlreadyRunning() bool {
	pidFile := common.WorkPIDFile()
	data, err := os.ReadFile(pidFile)
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

// work 是壳的工作进程入口。壳侧自检 + 配置 + 插件监管 + 调度 / 传输 / 心跳
// 全部由 router.Run 编排，main 只负责 PID 抢占、信号转 ctx 取消、版本号注入。
//
// 注意：壳不做自更新（容器内会造成镜像与进程版本漂移），更新交给 systemd /
// 容器编排，因此这里不再启动旧版自更新检查。
func work() {
	// 非守护子进程：自行抢占 PID 文件并参与防多开
	// 之前只有守护模式写 PID，直接运行 Agent 既不查重也不记录，
	// 而且退出时还会误删守护进程的 PID 文件
	if !common.IsDaemonChild() {
		if isAlreadyRunning() {
			log.Fatalf("已有实例在运行（%s），退出", common.WorkPIDFile())
		}
		writePID()
		defer cleanPIDOwned()
	}

	// 信号 → ctx 取消：router.Run 内部所有 goroutine 监听 ctx.Done() 优雅退出
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		sig := <-sigChan
		log.Printf("收到信号 %v，正在优雅退出...", sig)
		cancel()
	}()

	// config.NewConfigCenter 内部完成"配置加载 + 钳制 + 启动自检"
	// 配置缺关键项会在 selfcheck 阶段直接返回错误 → work() 返回 →
	// 守护进程会指数退避重启（保留旧版"配置错误也不整机停摆"的语义）
	log.Printf("Agent 已启动，PID: %d 版本 %s", os.Getpid(), Version)
	if err := router.Run(ctx); err != nil {
		log.Printf("壳退出: %v", err)
		os.Exit(1)
	}
	log.Println("Agent 已停止")
}
