package api

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"

	"monitor-server/modle"
	"monitor-server/common"

	"github.com/prometheus/client_golang/prometheus"
)

// MetricsHandler HTTP 入口：接收 Agent 上报
//
// 签名 func MetricsHandler(w http.ResponseWriter, r *http.Request, reg *prometheus.Registry)
// 由 router 包在 /metrics_data 路由上调用，传入 store.CustomRegistry。
func MetricsHandler(w http.ResponseWriter, r *http.Request, reg *prometheus.Registry) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "仅支持 POST 请求")
		return
	}

	// 读取请求体（限制大小防止 DoS 攻击）
	body, err := io.ReadAll(io.LimitReader(r.Body, MaxRequestBodySize))
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "读取请求体失败")
		return
	}
	defer func(Body io.ReadCloser) {
		if err := Body.Close(); err != nil {
			log.Printf("关闭请求体失败: %v", err)
		}
	}(r.Body)

	// 检查请求体是否超出限制
	if len(body) >= MaxRequestBodySize {
		writeJSONError(w, http.StatusRequestEntityTooLarge, "请求体过大")
		return
	}

	// 解密数据（错误信息不暴露内部细节）
	decryptedData, err := common.Decrypt(body)
	if err != nil {
		log.Printf("解密失败: %v", err)
		writeJSONError(w, http.StatusBadRequest, "数据解密失败")
		return
	}

	// 解压数据
	decompressedData, err := common.Decompress(decryptedData)
	if err != nil {
		log.Printf("解压失败: %v", err)
		writeJSONError(w, http.StatusBadRequest, "数据解压失败")
		return
	}

	// 将解压后的 JSON 解析为 typed struct
	// Data 字段保持 json.RawMessage（原始字节），后续传给各 Handle 函数再按 source 做 typed decode
	var payload modle.SendPayload
	decoder := json.NewDecoder(bytes.NewReader(decompressedData))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		log.Printf("JSON 解析失败: %v", err)
		writeJSONError(w, http.StatusBadRequest, "数据格式错误")
		return
	}

	// project 校验
	if payload.Project == "" {
		writeJSONError(w, http.StatusBadRequest, "缺少或无效的 project 字段")
		return
	}
	if !isValidProject(payload.Project) {
		log.Printf("无效的 project 名称: %s", payload.Project)
		writeJSONError(w, http.StatusBadRequest, "无效的 project 名称")
		return
	}

	// ===== 协议分流 =====
	// 带 metrics 字段的是插件化 Agent 的声明式上报：指标定义由上报方携带，
	// 走独立链路（校验失败会带原因返回 400）。
	// 旧 data 路径已废弃，返回 410 Gone 提示升级 Agent。
	if len(payload.Metrics) > 0 {
		HandleDeclarativePayload(w, payload, reg)
		return
	}

	log.Printf("收到已废弃的 data 路径请求: source=%s project=%s，请升级 Agent", payload.Source, payload.Project)
	writeJSONError(w, http.StatusGone, "已废弃 data 路径，请升级 agent")
}

// writeJSONError 响应 JSON 错误
func writeJSONError(w http.ResponseWriter, statusCode int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	response := map[string]interface{}{
		"code": statusCode,
		"msg":  msg,
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("响应失败: %v", err)
	}
}
