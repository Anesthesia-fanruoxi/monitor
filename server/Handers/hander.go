package Handers

import (
	"bytes"
	"compress/gzip"
	"crypto/aes"
	"crypto/cipher"
	"encoding/json"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"io"
	"log"
	"net/http"
	"sync"
)

// 全局变量存储加密盐
var encryptionKey []byte
var encryptionKeyMu sync.RWMutex // 使用读写锁，允许并发读取

// 分片锁：按 source+project 组合分锁，大幅提升并发能力
const shardCount = 256 // 分片数量，2的幂次方便取模

type shardedMutex struct {
	shards [shardCount]sync.Mutex
}

func (sm *shardedMutex) getShard(key string) *sync.Mutex {
	// FNV-1a 哈希算法，快速且分布均匀
	hash := uint32(2166136261)
	for i := 0; i < len(key); i++ {
		hash ^= uint32(key[i])
		hash *= 16777619
	}
	return &sm.shards[hash%shardCount]
}

var (
	nginxShards            shardedMutex
	hardShards             shardedMutex
	sslShards              shardedMutex
	containerShards        shardedMutex
	heartShards            shardedMutex
	controllerShards       shardedMutex
	trafficSwitchingShards shardedMutex
)

// Worker Pool 配置
const workerPoolSize = 500  // 并发 worker 数量
const taskQueueSize = 10000 // 任务队列缓冲大小

var taskQueue chan func()

func init() {
	taskQueue = make(chan func(), taskQueueSize)
	// 启动 worker pool
	for i := 0; i < workerPoolSize; i++ {
		go func() {
			for task := range taskQueue {
				safeExecute(task)
			}
		}()
	}
}

// safeExecute 安全执行任务，捕获 panic 防止 worker 退出
func safeExecute(task func()) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Worker panic 恢复: %v", r)
		}
	}()
	task()
}

// 允许的 source 类型白名单
var allowedSources = map[string]bool{
	"nginx":            true,
	"hard":             true,
	"ssl":              true,
	"k8s":              true,
	"heart":            true,
	"k8sController":    true,
	"trafficSwitching": true,
}

// 验证 project 名称是否合法
func isValidProject(project string) bool {
	// 限制长度（按 rune 计算，支持中文）
	runeLen := len([]rune(project))
	if runeLen == 0 || runeLen > 64 {
		return false
	}
	// 允许字母、数字、下划线、中划线、点、中文字符
	for _, c := range project {
		isAlphaNum := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
		isSpecialChar := c == '_' || c == '-' || c == '.'
		isChinese := c >= 0x4E00 && c <= 0x9FFF // 中文基本区
		if !isAlphaNum && !isSpecialChar && !isChinese {
			return false
		}
	}
	return true
}

// 验证 source 类型是否允许
func isValidSource(source string) bool {
	return allowedSources[source]
}

// 请求体大小限制（10MB）
const MaxRequestBodySize = 10 * 1024 * 1024

// SetEncryptionKey 设置加密盐（验证密钥长度）
func SetEncryptionKey(key string) {
	keyLen := len(key)
	if keyLen != 16 && keyLen != 24 && keyLen != 32 {
		log.Printf("警告: AES 密钥长度应为 16/24/32 字节，当前: %d 字节", keyLen)
	}
	encryptionKeyMu.Lock()
	defer encryptionKeyMu.Unlock()
	encryptionKey = []byte(key)
}

// GetEncryptionKey 获取当前加密盐（线程安全）
func GetEncryptionKey() []byte {
	encryptionKeyMu.RLock()
	defer encryptionKeyMu.RUnlock()
	return encryptionKey
}

// 解密数据
func Decrypt(ciphertext []byte) ([]byte, error) {
	key := GetEncryptionKey() // 获取加密盐
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	if len(ciphertext) < gcm.NonceSize() {
		return nil, fmt.Errorf("密文过短")
	}

	nonce, ciphertext := ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}
	return plaintext, nil
}

// 解压数据
func Decompress(data []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := reader.Close(); err != nil {
			log.Printf("关闭 gzip reader 失败: %v", err)
		}
	}()

	// 读取解压缩后的内容
	return io.ReadAll(reader)
}

// 响应 JSON 错误
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

func MetricsHandler(w http.ResponseWriter, r *http.Request, CustomRegistry *prometheus.Registry) {
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
	//log.Printf("收到请求，数据长度: %d 字节", len(body))

	// 解密数据（错误信息不暴露内部细节）
	decryptedData, err := Decrypt(body)
	if err != nil {
		log.Printf("解密失败: %v", err) // 日志记录详细错误
		writeJSONError(w, http.StatusBadRequest, "数据解密失败")
		return
	}

	// 解压数据
	decompressedData, err := Decompress(decryptedData)
	if err != nil {
		log.Printf("解压失败: %v", err)
		writeJSONError(w, http.StatusBadRequest, "数据解压失败")
		return
	}

	// 将解压后的 JSON 解析为 map
	var payload map[string]interface{}
	if err := json.Unmarshal(decompressedData, &payload); err != nil {
		log.Printf("JSON 解析失败: %v", err)
		writeJSONError(w, http.StatusBadRequest, "数据格式错误")
		return
	}

	// 提取并验证 project 字段
	project, ok := payload["project"].(string)
	if !ok || project == "" {
		writeJSONError(w, http.StatusBadRequest, "缺少或无效的 project 字段")
		return
	}
	if !isValidProject(project) {
		log.Printf("无效的 project 名称: %s", project)
		writeJSONError(w, http.StatusBadRequest, "无效的 project 名称")
		return
	}

	// 提取并验证 source 字段
	source, ok := payload["source"].(string)
	if !ok || source == "" {
		writeJSONError(w, http.StatusBadRequest, "缺少或无效的 source 字段")
		return
	}
	if !isValidSource(source) {
		log.Printf("不支持的 source 类型: %s", source)
		writeJSONError(w, http.StatusBadRequest, "不支持的数据类型")
		return
	}

	// 提取 data 字段
	data, ok := payload["data"].([]interface{})
	if !ok || len(data) == 0 {
		writeJSONError(w, http.StatusBadRequest, "缺少或无效的 data 字段")
		return
	}

	// 立即返回成功响应
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	response := map[string]interface{}{"code": 200, "msg": "ok"}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("响应失败: %v", err)
	}

	// 使用 worker pool 处理，按 project 分片锁，同项目同类型串行，不同项目并发
	task := func() {
		switch source {
		case "nginx":
			mu := nginxShards.getShard(project)
			mu.Lock()
			HandleNginxData(data, project)
			mu.Unlock()
		case "hard":
			mu := hardShards.getShard(project)
			mu.Lock()
			HandleHardData(data, project)
			mu.Unlock()
		case "ssl":
			mu := sslShards.getShard(project)
			mu.Lock()
			HandleSSLData(data, project)
			mu.Unlock()
		case "k8s":
			mu := containerShards.getShard(project)
			mu.Lock()
			HandleContainerResourceData(data, project)
			mu.Unlock()
		case "heart":
			mu := heartShards.getShard(project)
			mu.Lock()
			HandleHeartData(data, project)
			mu.Unlock()
		case "k8sController":
			mu := controllerShards.getShard(project)
			mu.Lock()
			HandleControllertResourceData(data, project)
			mu.Unlock()
		case "trafficSwitching":
			mu := trafficSwitchingShards.getShard(project)
			mu.Lock()
			HandleTrafficSwitchingData(data, project)
			mu.Unlock()
		}
	}

	// 非阻塞提交任务，队列满时降级为同步处理
	select {
	case taskQueue <- task:
		// 成功提交到 worker pool
	default:
		// 队列满，直接执行避免请求堆积
		go task()
	}
}
