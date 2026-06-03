package Middleware

import (
	"bytes"
	"compress/gzip"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"io"
	"net/http"
	"time"
)

// 全局 HTTP 客户端，带超时和连接池复用
var httpClient = &http.Client{
	Timeout: 30 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	},
}

// 加密数据
func encrypt(data []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	ciphertext := gcm.Seal(nonce, nonce, data, nil)
	return ciphertext, nil
}

// 压缩数据
func compress(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)
	if _, err := writer.Write(data); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// 发送数据到指定的 URL
func SendData(url string, project string, data interface{}, key []byte, source string) error {
	// 创建要发送的数据结构
	sendData := SendDataType{
		PROJECT:   project,
		Data:      data,
		Timestamp: time.Now().UnixMilli(),
		SOURCE:    source,
	}

	// 序列化数据
	jsonData, err := json.Marshal(sendData)
	if err != nil {
		return err
	}

	// 压缩数据
	compressedData, err := compress(jsonData)
	if err != nil {
		return err
	}

	// 加密数据
	encryptedData, err := encrypt(compressedData, key)
	if err != nil {
		return err
	}

	// 发送加密压缩后的数据（使用全局带超时的客户端）
	resp, err := httpClient.Post(url, "application/octet-stream", bytes.NewBuffer(encryptedData))
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	// 必须读取并丢弃 response body，否则连接无法复用
	_, _ = io.Copy(io.Discard, resp.Body)

	return nil
}
