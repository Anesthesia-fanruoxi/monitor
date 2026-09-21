package router

import (
	"log"
	"net/http"
	"strings"

	"monitor-server/api"
	"monitor-server/config"
	"monitor-server/ippass"
	"monitor-server/store"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Register 注册全部 HTTP 路由
func Register(cfg *config.Config) {
	// 暴露自定义指标 /metrics（带 IP 限制）
	metricsHandler := promhttp.HandlerFor(
		store.CustomRegistry, // 使用自定义的 Registry
		promhttp.HandlerOpts{},
	)
	http.Handle("/metrics", ippass.IpRestrictionMiddleware(metricsHandler))

	// HTTP 接收端 /metrics_data，传递 CustomRegistry 给 MetricsHandler
	http.HandleFunc("/metrics_data", func(w http.ResponseWriter, r *http.Request) {
		api.MetricsHandler(w, r, store.CustomRegistry)
	})

	// 插件制品仓库端点（默认关闭）
	if cfg.PluginRepo.Enable {
		registerPluginRepoEndpoints(cfg)
	}
}

// registerPluginRepoEndpoints 注册插件下载端点
//
// GET /plugin/download?name=... — dist/ 平铺二进制按 name 下发，
// 仅靠 HMAC 签名校验（错签/过期/重放/穿越均拒绝），不挂 IP 白名单：
// agent 部署在任意客户机，IP 不可预知。
func registerPluginRepoEndpoints(cfg *config.Config) {
	dir := strings.TrimSpace(cfg.PluginRepo.Dir)
	if dir == "" {
		log.Printf("pluginRepo.enable 为 true 但未配置 pluginRepo.dir，跳过端点注册")
		return
	}

	http.Handle("/plugin/download", http.HandlerFunc(api.PluginDownload(dir)))

	log.Printf("已开启插件下载，dist 目录: %s", dir)
}
