package main

import (
	"agent-plugs/sdk"
)

func init() {
	sdk.SetName("ssl")
	sdk.SetVersion("0.1.0")

	// 1 个指标，逐字来自任务 06-S2（docs/插件设计.md §5.3）。
	// ssl 插件无 hostName 标签（docs §5.3 现状如此）。
	sdk.RegisterMetric("ssl_cert_expires_days", []string{"domain"}, helpCertExpiresDays, nil)
}

func main() {
	sdk.SetCollect(Collect)
	sdk.Run()
}
