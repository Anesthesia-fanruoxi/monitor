package Metrics

import (
	"agent/Middleware"
	"fmt"
)

// IsActive 方法：接收一个字符串，返回一个切片和错误信息
func IsActive(input string, version float64) ([]Middleware.HeartSource, error) {
	// 获取主机名
	hostName, err := GetHostName()
	if err != nil {
		return nil, fmt.Errorf("获取主机名失败: %w", err)
	}
	// 构建返回的响应
	response := []Middleware.HeartSource{
		{
			Project:  input,
			Hostname: hostName,
			IsActive: 1,
			Version:  version,
		},
	}

	return response, nil
}
