package api

import (
	"bytes"
	"compress/gzip"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"agent/config"
	"agent/modle"
)

// Transport 负责把插件输出的 Metrics 组装成 DeclarativePayload，
// AES-GCM + gzip 压缩后 POST 到 MetricsURL。
//
// 壳负责填 Project / Source / Timestamp / Schema，插件完全不知道 project 存在。
// 壳不解析 Metrics 内容，只做透传。
type Transport struct {
	cfg  *config.ConfigCenter
	http *http.Client
	// 批量队列：OnCollect 投递 metrics，flush 消费
	queue chan rawCollect
}

type rawCollect struct {
	pluginName string
	metrics    json.RawMessage
}

// NewTransport 创建传输器，HTTP client 默认 30s。
func NewTransport(cfg *config.ConfigCenter) *Transport {
	return &Transport{
		cfg: cfg,
		http: &http.Client{
			Timeout: 30 * time.Second,
		},
		queue: make(chan rawCollect, 64),
	}
}

// OnCollect 调度器回调，投递一条插件采集结果。
// 无数据（null / 空数组 / 空消息）直接丢弃，避免无意义上报。
func (t *Transport) OnCollect(pluginName string, metrics json.RawMessage) {
	if len(metrics) == 0 || bytes.Equal(metrics, []byte("null")) || bytes.Equal(metrics, []byte("[]")) {
		return
	}
	select {
	case t.queue <- rawCollect{pluginName: pluginName, metrics: metrics}:
	default:
		log.Printf("[shell] transport queue 满，丢弃 %s", pluginName)
	}
}

// Run 启动一个消费 goroutine，每条插件输出独立组包上报：
// agent 对 metrics 字节零改动（不解析、不合并），仅加信封后压缩加密转发。
func (t *Transport) Run(ctx config.Done) {
	for {
		select {
		case <-ctx.Done():
			return
		case item := <-t.queue:
			t.send(item)
		}
	}
}

// send 组装 DeclarativePayload → 压缩 → 加密 → POST。
func (t *Transport) send(item rawCollect) {
	cfg := t.cfg.Get()
	if cfg.MetricsURL == "" {
		log.Println("[shell] metrics_url 未配置，跳过上报")
		return
	}
	if t.cfg.IsOffline() {
		return
	}

	payload := modle.DeclarativePayload{
		Project:   cfg.Project,
		Source:    "agent",
		Timestamp: time.Now().UnixMilli(),
		Schema:    "v2",
		Metrics:   item.metrics, // 插件输出原样字节，零改动
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		log.Printf("[shell] transport: %s metrics 非法 JSON: %v", item.pluginName, err)
		return
	}

	// gzip 压缩
	compressed, err := gzipBytes(raw)
	if err != nil {
		log.Printf("[shell] transport gzip: %v", err)
		return
	}

	// AES-GCM 加密（密钥空则跳过加密）
	body := compressed
	key := []byte(cfg.EncryptionKey)
	switch len(key) {
	case 0:
		// 明文 + gzip
	default:
		enc, err := aesGCMEncrypt(key, compressed)
		if err != nil {
			log.Printf("[shell] transport encrypt: %v", err)
			return
		}
		body = enc
	}

	url := strings.TrimRight(cfg.MetricsURL, "/")
	// 上报路径：metrics_url 兼容两种写法（带或不带 /metrics_data 后缀），避免双拼 404
	if !strings.HasSuffix(url, "/metrics_data") {
		url += "/metrics_data"
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		log.Printf("[shell] transport new request: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	if len(key) > 0 {
		req.Header.Set("X-Encrypted", "aes-gcm")
	} else {
		req.Header.Set("X-Compression", "gzip")
	}

	resp, err := t.http.Do(req)
	if err != nil {
		log.Printf("[shell] transport post: %v", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		log.Printf("[shell] transport 上报失败 status=%d body=%s", resp.StatusCode, string(respBody))
	}
}

func gzipBytes(src []byte) ([]byte, error) {
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := w.Write(src); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func aesGCMEncrypt(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("nonce: %w", err)
	}
	sealed := gcm.Seal(nonce, nonce, plaintext, nil)
	return sealed, nil
}
