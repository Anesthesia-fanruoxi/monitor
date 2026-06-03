package Metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	// Nginx指标
	NginxIsRunMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "nginx_is_run", // Nginx 是否运行
			Help: "表示 Nginx 是否正在运行",
		},
		[]string{"hostName", "project"},
	)

	NginxReTotalMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "nginx_re_total", // Nginx 总请求数
			Help: "Nginx 总请求数",
		},
		[]string{"hostName", "project"},
	)

	NginxLoginUserCountMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "nginx_login_user_count", // 已登录用户数量
			Help: "已登录用户数量",
		},
		[]string{"hostName", "project"},
	)

	NginxRawTotalMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "nginx_raw_total", // Nginx 原始请求总数
			Help: "Nginx 处理的原始请求总数",
		},
		[]string{"hostName", "project"},
	)

	NginxUdptotalMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "nginx_udptotal", // Nginx UDP 请求总数
			Help: "Nginx 处理的 UDP 请求总数",
		},
		[]string{"hostName", "project"},
	)

	NginxTcpTotalMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "nginx_tcp_total", // Nginx TCP 请求总数
			Help: "Nginx 处理的 TCP 请求总数",
		},
		[]string{"hostName", "project"},
	)

	NginxTotalTcpMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "nginx_total_tcp", // Nginx TCP 连接总数
			Help: "Nginx 处理的 TCP 连接总数",
		},
		[]string{"hostName", "project"},
	)

	NginxInetTotalMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "nginx_inet_total", // Nginx 互联网连接总数
			Help: "Nginx 处理的互联网连接总数",
		},
		[]string{"hostName", "project"},
	)

	NginxFragTotalMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "nginx_frag_total", // Nginx 碎片包总数
			Help: "Nginx 处理的碎片包总数",
		},
		[]string{"hostName", "project"},
	)

	NginxTcpEstabMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "nginx_tcp_estab", // 已建立的 TCP 连接总数
			Help: "已建立的 TCP 连接总数",
		},
		[]string{"hostName", "project"},
	)

	NginxTcpClosedMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "nginx_tcp_closed", // 关闭的 TCP 连接总数
			Help: "关闭的 TCP 连接总数",
		},
		[]string{"hostName", "project"},
	)

	NginxTcpOrphanedMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "nginx_tcp_orphaned", // 孤立的 TCP 连接总数
			Help: "孤立的 TCP 连接总数",
		},
		[]string{"hostName", "project"},
	)

	NginxTcpTimewaitMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "nginx_tcp_timewait", // 处于 TIME-WAIT 状态的 TCP 连接总数
			Help: "处于 TIME-WAIT 状态的 TCP 连接总数",
		},
		[]string{"hostName", "project"},
	)
)
