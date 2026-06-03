package Metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	//ssl
	SslDaysLeftMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "ssl_domain_days_left", // SSL 证书剩余天数
			Help: "SSL 证书到期前的剩余天数",
		},
		[]string{"domain", "comment", "status", "resolve", "project"}, // 标签
	)
)
