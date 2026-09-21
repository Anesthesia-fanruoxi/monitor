package store

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// ===== 插件下载 =====
//
// 极简协议：server dist/ 平铺二进制，GET /plugin/download?name=&ts=&nonce=&sig= 直接下发。
// 签名材料 name|ts|nonce（与服务端对齐），密钥复用上报加密盐 encryption_key。
// 不做内容完整性校验；响应头 X-Plugin-Sha256 仅用于更新检测（与本地记录比对）。

const downloadTimeout = 60 * time.Second

// ServiceBase 从上报 URL 推导服务根地址（剥掉 /metrics_data 尾巴）。
func ServiceBase(metricsURL string) string {
	u := strings.TrimSuffix(metricsURL, "/metrics_data")
	return strings.TrimRight(u, "/")
}

func signDownloadURL(base, name string, key []byte) string {
	tsMs := time.Now().UnixMilli()
	nonce := strconv.FormatInt(time.Now().UnixNano(), 36)
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(name + "|" + strconv.FormatInt(tsMs, 10) + "|" + nonce))

	return base + "/plugin/download?name=" + name +
		"&ts=" + strconv.FormatInt(tsMs, 10) +
		"&nonce=" + nonce +
		"&sig=" + hex.EncodeToString(mac.Sum(nil))
}

// remoteSHA256 HEAD 请求拿远端二进制的 sha256（不下载 body，轻量检查更新）。
func remoteSHA256(base, name string, key []byte) (string, error) {
	client := &http.Client{Timeout: downloadTimeout}
	req, err := http.NewRequest(http.MethodHead, signDownloadURL(base, name, key), nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status %d", resp.StatusCode)
	}
	return strings.ToLower(resp.Header.Get("X-Plugin-Sha256")), nil
}

// InstallPlugin 下载插件二进制到 plugins/<name>/<name>（tmp + rename 原子替换）。
func InstallPlugin(name, base string, key []byte, pluginsDir string) (PluginInfo, error) {
	client := &http.Client{Timeout: downloadTimeout}
	resp, err := client.Get(signDownloadURL(base, name, key))
	if err != nil {
		return PluginInfo{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return PluginInfo{}, fmt.Errorf("下载 status %d", resp.StatusCode)
	}

	destDir := filepath.Join(pluginsDir, name)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return PluginInfo{}, err
	}
	tmp := filepath.Join(destDir, name+".tmp")
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0755)
	if err != nil {
		return PluginInfo{}, err
	}
	h := sha256.New()
	_, copyErr := io.Copy(io.MultiWriter(f, h), resp.Body)
	closeErr := f.Close()
	if copyErr != nil || closeErr != nil {
		os.Remove(tmp)
		return PluginInfo{}, fmt.Errorf("写文件: %v / %v", copyErr, closeErr)
	}
	if err := os.Rename(tmp, filepath.Join(destDir, name)); err != nil {
		os.Remove(tmp)
		return PluginInfo{}, err
	}

	info := PluginInfo{SHA256: hex.EncodeToString(h.Sum(nil))}
	if err := SavePluginInfo(pluginsDir, name, info); err != nil {
		log.Printf("[shell] %s 状态记录失败: %v", name, err)
	}
	return info, nil
}

// UpdateIfNew 远端 sha256 与本地记录不同时重新下载（HEAD 轻量比对，不走版本号）。
// 返回是否发生了更新。
func UpdateIfNew(name, base string, key []byte, pluginsDir, localSHA string) (bool, error) {
	remote, err := remoteSHA256(base, name, key)
	if err != nil {
		return false, err
	}
	if remote == "" || remote == localSHA {
		return false, nil
	}
	log.Printf("[shell] %s 远端制品已变化，开始更新", name)
	if _, err := InstallPlugin(name, base, key, pluginsDir); err != nil {
		return false, err
	}
	return true, nil
}
