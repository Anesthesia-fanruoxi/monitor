package main

import (
	"context"
	"strings"
	"sync"
	"time"

	"agent-plugs/sdk"
)

const (
	apiTimeout     = 30 * time.Second
	masterLabel    = "node-role.kubernetes.io/control-plane"
	masterLabelOld = "node-role.kubernetes.io/master"
)

// ===== 客户端单例（长驻进程红利，只初始化一次）=====
var (
	clientMu  sync.Mutex
	kc        *kubeClient
	clientErr error
	warnOnce  sync.Once
)

// initClients 惰性初始化 kube 客户端；失败不缓存，下一轮采集重试
// （kubeconfig 可能后到位，或初次失败修复后无需重启插件即可恢复）。
func initClients() {
	if kc != nil {
		return
	}
	clientMu.Lock()
	defer clientMu.Unlock()
	if kc != nil {
		return
	}

	// 读 kubeconfig 文件（轻量客户端，不依赖 client-go）。
	path := configKubeconfig()
	pc := sdk.LoadConfig()
	insecure := false
	if v, ok := pc["insecure_skip_tls_verify"].(bool); ok && v {
		insecure = true
	}
	serverName := ""
	if v, ok := pc["tls_server_name"].(string); ok {
		serverName = strings.TrimSpace(v)
	}

	c, err := loadKubeClient(path, insecure, serverName)
	if err != nil {
		clientErr = err
		return
	}
	kc = c
	// 启动诊断：配置注入与否一目了然（不打印敏感内容）
	sdk.Fprintln("[k8s] 配置: kubeconfig=", path, " insecure=", insecure, " server_name=", serverName, " server=", c.server)
}

func warnNoCluster() {
	warnOnce.Do(func() {
		sdk.Fprintln("[k8s] kubeconfig 不可用，本轮无样本：", clientErr)
	})
}

// ===== 配置 =====
// configKubeconfig：kubeconfig 文件路径（插件配置 kubeconfig 键），
// 缺省 /root/.kube/config。
func configKubeconfig() string {
	cfg := sdk.LoadConfig()
	if v, ok := cfg["kubeconfig"].(string); ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return "/root/.kube/config"
}

// configNamespaces：空切片 = 全部 namespace；非空 = 白名单。
func configNamespaces() []string {
	cfg := sdk.LoadConfig()
	v, ok := cfg["namespaces"].([]any)
	if !ok || len(v) == 0 {
		return nil
	}
	out := make([]string, 0, len(v))
	for _, x := range v {
		if s, ok := x.(string); ok && strings.TrimSpace(s) != "" {
			out = append(out, s)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// onlyWithLimits：true=仅给有 limit 的 pod 报 limit；false（缺省）=全部 pod 报（无 limit 则 0）。
func onlyWithLimits() bool {
	cfg := sdk.LoadConfig()
	if v, ok := cfg["only_with_limits"].(bool); ok {
		return v
	}
	return false
}

func nsAllowed(ns string, allow []string) bool {
	if len(allow) == 0 {
		return true
	}
	for _, a := range allow {
		if a == ns {
			return true
		}
	}
	return false
}

// ===== 状态判定 =====
func isMaster(n k8sNode) bool {
	_, ok1 := n.Metadata.Labels[masterLabel]
	_, ok2 := n.Metadata.Labels[masterLabelOld]
	return ok1 || ok2
}

func nodeReady(n k8sNode) bool {
	for _, c := range n.Status.Conditions {
		if c.Type == "Ready" {
			return c.Status == "True"
		}
	}
	return false
}

// metric 构造一条 DeclaredMetric（labels + samples 由调用方填）。
func metric(name, help string, labels []string, samples []sdk.Sample) sdk.DeclaredMetric {
	return sdk.DeclaredMetric{
		Name: name, Help: help, Type: sdk.MetricTypeGauge,
		Labels: labels, Samples: samples,
	}
}

func boolF(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

func replicasOrDefault(r *int32) int32 {
	if r == nil {
		return 1
	}
	return *r
}

// Collect 主入口。
// kubeconfig 不可用 → warn-once + 返回 nil（本轮无样本，不让插件失败）。
// 各接口失败 → 该域无样本 + stderr 一行，其余域照常上报。
// 域实现在 node.go / pod.go / workload.go / usage.go / storage.go。
func Collect() []sdk.DeclaredMetric {
	initClients()
	if kc == nil {
		warnNoCluster()
		return nil
	}
	allow := configNamespaces()
	limitOnly := onlyWithLimits()

	ctx, cancel := context.WithTimeout(context.Background(), apiTimeout)
	defer cancel()

	out := make([]sdk.DeclaredMetric, 0, 25)
	out = appendNodes(out, ctx)
	out = appendPods(out, ctx, allow, limitOnly)
	out = appendWorkloads(out, ctx, allow)
	out = appendUsage(out, ctx, allow)
	out = appendStorage(out, ctx, allow)
	return out
}
