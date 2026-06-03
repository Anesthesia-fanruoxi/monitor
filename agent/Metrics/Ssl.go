package Metrics

import (
	"agent/Middleware"
	"bufio"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// 获取 SSL 证书的到期时间
func getSSLCertificateExpiration(domain string) (time.Time, error) {
	dialer := &net.Dialer{Timeout: 5 * time.Second} // 设置超时时间
	conn, err := tls.DialWithDialer(dialer, "tcp", domain+":443", &tls.Config{
		InsecureSkipVerify: true,
	})
	if err != nil {
		return time.Time{}, err
	}
	defer func() { _ = conn.Close() }()

	certificates := conn.ConnectionState().PeerCertificates
	if len(certificates) == 0 {
		return time.Time{}, fmt.Errorf("未找到证书")
	}
	return certificates[0].NotAfter, nil
}

// 计算剩余天数
func daysUntilExpiration(expiration time.Time) int {
	return int(time.Until(expiration).Hours() / 24)
}

// 更新域名信息状态
func updateDomainInfo(domainInfo Middleware.DomainInfo, expiration time.Time, err error) Middleware.DomainInfo {
	if err != nil {
		domainInfo.Status = "解析失败"
		domainInfo.Resolve = false
		domainInfo.Expiration = time.Time{}
		domainInfo.DaysLeft = -1
	} else {
		domainInfo.Expiration = expiration
		domainInfo.DaysLeft = daysUntilExpiration(expiration)
		domainInfo.Status = "正常"
		domainInfo.Resolve = true
	}
	return domainInfo
}

// 从文件中提取域名和备注
func extractDomainsFromFile(filePath string, re *regexp.Regexp, excludeLocal bool) ([]Middleware.DomainInfo, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	var domainInfos []Middleware.DomainInfo
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "#") || line == "" { // 跳过注释和空行
			continue
		}

		matches := re.FindStringSubmatch(line)
		if len(matches) > 1 {
			domain := matches[1]
			comment := "未备注"
			if len(matches) > 2 && matches[2] != "" {
				comment = strings.TrimSpace(matches[2])
				if strings.HasPrefix(comment, "#") {
					comment = strings.TrimPrefix(comment, "#")
				}
			}

			// 排除 localhost 或 .local
			if excludeLocal && (domain == "localhost" || strings.HasSuffix(domain, ".local")) {
				continue
			}
			domainInfos = append(domainInfos, Middleware.DomainInfo{
				Domain:  domain,
				Comment: comment,
			})
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return domainInfos, nil
}

// 获取 SSL 信息
func GetSslInfo() (string, error) {
	var allDomainInfos []Middleware.DomainInfo
	seenDomains := make(map[string]bool) // 用于去重

	// 提取域名来源: Nginx 配置文件
	nginxDirectory := "/etc/nginx/conf.d"
	err := filepath.Walk(nginxDirectory, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filepath.Ext(path) == ".conf" {
			re := regexp.MustCompile(`server_name\s+(.*);.*?(#.*)?$`)
			domainInfos, err := extractDomainsFromFile(path, re, false)
			if err != nil {
				log.Printf("读取配置文件 %s 失败: %v", path, err)
				return nil
			}
			// 去重
			for _, domainInfo := range domainInfos {
				if !seenDomains[domainInfo.Domain] {
					allDomainInfos = append(allDomainInfos, domainInfo)
					seenDomains[domainInfo.Domain] = true
				}
			}
		}
		return nil
	})
	if err != nil {
		return "", err
	}

	// 提取域名来源: /etc/hosts 文件
	hostsPath := "/etc/hosts"
	reHosts := regexp.MustCompile(`^\s*\d+\.\d+\.\d+\.\d+\s+([^\s#]+).*?(#.*)?$`)
	hostDomainInfos, err := extractDomainsFromFile(hostsPath, reHosts, true)
	if err != nil {
		log.Printf("读取 hosts 文件失败: %v", err)
	} else {
		// 去重并排除 localhost 行
		for _, domainInfo := range hostDomainInfos {
			if domainInfo.Domain != "localhost" && !seenDomains[domainInfo.Domain] {
				allDomainInfos = append(allDomainInfos, domainInfo)
				seenDomains[domainInfo.Domain] = true
			}
		}
	}

	// 提取域名来源: 自定义域名文件
	domainPath := "domains.txt"

	// 检查域名文件是否存在
	if _, err := os.Stat(domainPath); err == nil {
		// 如果文件存在，调用解析函数，读取文件内容
		reDomains := regexp.MustCompile(`^\s*([^\s#]+)\s*(#.*)?$`) // 新的正则表达式
		domainFileInfos, err := extractDomainsFromFile(domainPath, reDomains, true)
		if err != nil {
			log.Printf("读取域名文件 %s 失败: %v", domainPath, err)
		} else {
			// 去重
			for _, domainInfo := range domainFileInfos {
				if !seenDomains[domainInfo.Domain] {
					allDomainInfos = append(allDomainInfos, domainInfo)
					seenDomains[domainInfo.Domain] = true
				}
			}
		}
	} else if !os.IsNotExist(err) {
		log.Printf("检查域名文件 %s 时出错: %v", domainPath, err)
	} else {
		log.Printf("域名文件 %s 不存在", domainPath)
	}

	// 并发检查 SSL 信息
	results := make(chan Middleware.DomainInfo, len(allDomainInfos))
	wg := sync.WaitGroup{}

	for _, domainInfo := range allDomainInfos {
		wg.Add(1)
		go func(domainInfo Middleware.DomainInfo) {
			defer wg.Done()
			expiration, err := getSSLCertificateExpiration(domainInfo.Domain)
			results <- updateDomainInfo(domainInfo, expiration, err)
		}(domainInfo)
	}

	wg.Wait()
	close(results)

	// 收集结果
	var updatedDomainInfos []Middleware.DomainInfo
	for info := range results {
		updatedDomainInfos = append(updatedDomainInfos, info)
	}

	// 转换为 JSON
	jsonData, err := json.MarshalIndent(updatedDomainInfos, "", "  ")
	if err != nil {
		return "", err
	}
	return string(jsonData), nil
}
