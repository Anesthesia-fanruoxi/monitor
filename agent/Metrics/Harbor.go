package Metrics

import (
	"agent/Middleware"
)

// 获取 Harbor 任务信息
// ============================================harbor
func GetHarborInfo() ([]Middleware.HarborInfo, error) {
	// 获取当前登录用户数
	loginUserCount := GetLoginUserCount()
	hostName, err := GetHostName()
	if err != nil {
		return nil, err
	}
	// 创建 Nginx 状态结构体并填充数据
	harborInfo := Middleware.HarborInfo{
		// 当前登录的用户数量，表示系统中当前通过终端登录的用户数
		LoginUserCount: loginUserCount, // 当前登录用户数
		HostName:       hostName,
	}

	// 返回一个 NginxStatus 结构体的切片
	return []Middleware.HarborInfo{harborInfo}, nil
}
