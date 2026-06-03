package Metrics

import (
	"agent/Middleware"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// checkNginxStatus 用于检查 Nginx 是否运行
func checkNginxStatus() int {
	cmd := exec.Command("systemctl", "is-active", "nginx")
	err := cmd.Run()
	if err != nil {
		if strings.Contains(err.Error(), "inactive") || strings.Contains(err.Error(), "unknown") {
			return 0 // Nginx 未运行
		}
		return 0 // 出现其他错误，认为未运行
	}
	return 1 // Nginx 正在运行
}

// 获取当前登录用户数
func GetLoginUserCount() int {
	cmd := exec.Command("who", "-q")
	output, err := cmd.Output()
	if err != nil {
		return 0 // 获取失败，返回 0
	}

	// 打印输出内容以供调试
	//fmt.Printf("who -q 输出：%s\n", output)

	// 分割输出的每一行
	lines := strings.Split(string(output), "\n")
	if len(lines) < 2 {
		return 0 // 输出格式不符合预期，返回 0
	}

	// 获取第二行（格式为 "# users=3"）
	parts := strings.Fields(lines[1])
	if len(parts) < 2 {
		return 0 // 无法解析时返回 0
	}

	// 提取用户数
	var userCount int
	_, err = fmt.Sscanf(parts[1], "users=%d", &userCount)
	if err != nil {
		return 0 // 解析失败，返回 0
	}

	return userCount
}

// 获取 ss -s 命令输出的统计信息，若出错返回默认值
func getSSStats() *Middleware.NginxStatus {
	cmd := exec.Command("ss", "-s")
	output, err := cmd.Output()
	if err != nil {
		return &Middleware.NginxStatus{} // 返回默认空结构体
	}

	outputStr := string(output)
	stats := &Middleware.NginxStatus{}

	// 获取 Total 值
	reTotal := regexp.MustCompile(`Total: (\d+)`)
	matchTotal := reTotal.FindStringSubmatch(outputStr)
	if len(matchTotal) > 1 {
		stats.ReTotal, _ = strconv.Atoi(matchTotal[1])
	}

	// 获取各协议的 total 值
	reProtocolTotal := regexp.MustCompile(`(\w+)\s+(\d+)`)
	protocols := reProtocolTotal.FindAllStringSubmatch(outputStr, -1)
	for _, protocol := range protocols {
		if len(protocol) > 2 {
			total, _ := strconv.Atoi(protocol[2])
			switch protocol[1] {
			case "RAW":
				stats.RawTotal = total
			case "UDP":
				stats.Udptotal = total
			case "TCP":
				stats.TcpTotal = total
			case "INET":
				stats.InetTotal = total
			case "FRAG":
				stats.FragTotal = total
			}
		}
	}

	// 获取 TCP 状态的详细数值
	reTCPStats := regexp.MustCompile(`TCP:\s+(\d+)\s+\(estab (\d+),\s+closed (\d+),\s+orphaned (\d+),\s+timewait (\d+)\)`)
	matchTCPStats := reTCPStats.FindStringSubmatch(outputStr)
	if len(matchTCPStats) > 5 {
		stats.TotalTcp, _ = strconv.Atoi(matchTCPStats[1])
		stats.TcpEstab, _ = strconv.Atoi(matchTCPStats[2])
		stats.TcpClosed, _ = strconv.Atoi(matchTCPStats[3])
		stats.TcpOrphaned, _ = strconv.Atoi(matchTCPStats[4])
		stats.TcpTimewait, _ = strconv.Atoi(matchTCPStats[5])
	}

	return stats
}

func GetNginxInfo() ([]Middleware.NginxStatus, error) {
	// 获取 Nginx 是否运行
	isRunning := checkNginxStatus()

	// 获取当前登录用户数
	loginUserCount := GetLoginUserCount()

	// 获取 ss -s 命令输出的统计信息
	stats := getSSStats()
	hostName, err := GetHostName()
	if err != nil {
		return nil, err
	}

	// 创建 Nginx 状态结构体并填充数据
	nginxStatus := Middleware.NginxStatus{
		// 是否正在运行 Nginx 服务，true 表示运行中，false 表示没有运行
		IsRun: isRunning, // 是否运行

		// 当前登录的用户数量，表示系统中当前通过终端登录的用户数
		LoginUserCount: loginUserCount, // 当前登录用户数

		// 所有协议总连接
		ReTotal: stats.RawTotal,

		// RAW 协议的总连接数，表示当前使用 RAW 协议的连接数
		RawTotal: stats.RawTotal, // RAW 协议连接数

		// UDP 协议的总连接数，表示当前使用 UDP 协议的连接数
		Udptotal: stats.Udptotal, // UDP 协议连接数

		// TCP 协议的总连接数，表示当前使用 TCP 协议的连接数（包括所有状态的连接）
		TcpTotal: stats.TcpTotal, // TCP 协议连接数

		// TCP 协议的总连接数，`TotalTcp` 是用于记录总的 TCP 连接数
		TotalTcp: stats.TotalTcp, // TCP 协议总连接数（包括所有状态）

		// INET 协议的总连接数，表示当前使用 INET 协议的连接数
		InetTotal: stats.InetTotal, // INET 协议连接数

		// FRAG 协议的总连接数，表示当前使用 FRAG 协议的连接数
		FragTotal: stats.FragTotal, // FRAG 协议连接数

		// 当前处于 ESTABLISHED 状态的 TCP 连接数，表示当前处于正常连接状态的 TCP 连接
		TcpEstab: stats.TcpEstab, // TCP ESTABLISHED 连接数

		// 当前处于 CLOSED 状态的 TCP 连接数，表示已经关闭的 TCP 连接
		TcpClosed: stats.TcpClosed, // TCP CLOSED 连接数

		// 当前处于 ORPHANED 状态的 TCP 连接数，表示没有对应父进程的 TCP 连接
		TcpOrphaned: stats.TcpOrphaned, // TCP ORPHANED 连接数

		// 当前处于 TIME_WAIT 状态的 TCP 连接数，表示等待完全关闭的 TCP 连接
		TcpTimewait: stats.TcpTimewait, // TCP TIMEWAIT 连接数

		// 主机名称
		HostName: hostName,
	}

	// 返回一个 NginxStatus 结构体的切片
	return []Middleware.NginxStatus{nginxStatus}, nil
}
