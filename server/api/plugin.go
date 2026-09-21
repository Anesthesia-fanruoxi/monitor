package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
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

	"monitor-server/common"
)

// ===== 插件下载端点 =====
//
// GET /plugin/download?name=hard&ts=...&nonce=...&sig=...
// dist/ 平铺二进制，按 name 直接下发；签名材料 name|ts|nonce，sig 为 hex。
// 防护：HMAC 签名 + ts 时效 + nonce 防重放 + Base(name) 路径穿越防御。
// 不做内容校验（自用，下载通道本身受签名保护）；
// 响应头 X-Plugin-Sha256 仅供 agent 做更新检测（与本地文件比对）。

// PluginDownload GET /plugin/download，distDir 为插件二进制目录。
func PluginDownload(distDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet && req.Method != http.MethodHead {
			http.Error(w, "仅支持 GET/HEAD", http.StatusMethodNotAllowed)
			return
		}
		if err := verifyDownloadRequest(req, time.Now()); err != nil {
			log.Printf("[pluginrepo] 拒绝下载: %v", err)
			http.Error(w, "下载请求无效", http.StatusBadRequest)
			return
		}

		// Base 收敛防路径穿越：无论传什么，最终只在 dist/ 下一层找文件
		name := filepath.Base(strings.TrimSpace(req.URL.Query().Get("name")))
		if name == "" || name == "." || name == "/" {
			http.Error(w, "name 无效", http.StatusBadRequest)
			return
		}
		abs := filepath.Join(distDir, name)
		st, err := os.Stat(abs)
		if err != nil || st.IsDir() {
			http.NotFound(w, req)
			return
		}

		sum, err := fileSHA256(abs)
		if err != nil {
			http.Error(w, "读取失败", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Length", strconv.FormatInt(st.Size(), 10))
		w.Header().Set("X-Plugin-Sha256", hex.EncodeToString(sum))

		// HEAD 只返回头部，供 agent 轻量检查更新
		if req.Method == http.MethodHead {
			return
		}

		f, err := os.Open(abs)
		if err != nil {
			http.Error(w, "读取失败", http.StatusInternalServerError)
			return
		}
		defer f.Close()
		if _, err := io.Copy(w, f); err != nil {
			log.Printf("[pluginrepo] 下载中断: %s (%v)", name, err)
		}
	}
}

// fileSHA256 计算文件哈希（仅用于更新检测响应头）
func fileSHA256(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return nil, err
	}
	return h.Sum(nil), nil
}

// verifyDownloadRequest 校验下载请求：ts 时效 + nonce 防重放 + HMAC 签名。
// 签名材料（'|' 分隔）：name | ts | nonce，sig 为 hex(HMAC-SHA256(key, material))。
func verifyDownloadRequest(r *http.Request, now time.Time) error {
	q := r.URL.Query()
	name := strings.TrimSpace(q.Get("name"))
	ts := strings.TrimSpace(q.Get("ts"))
	nonce := strings.TrimSpace(q.Get("nonce"))
	sig := strings.TrimSpace(q.Get("sig"))

	if name == "" || ts == "" || nonce == "" || sig == "" {
		return fmt.Errorf("缺少必填参数")
	}

	var tsMs int64
	if _, err := fmt.Sscanf(ts, "%d", &tsMs); err != nil || tsMs <= 0 {
		return fmt.Errorf("ts 参数非法")
	}
	skew := now.Sub(time.UnixMilli(tsMs))
	if skew < 0 {
		skew = -skew
	}
	if skew > common.MaxSkew {
		return fmt.Errorf("ts 过期 %s", skew.Truncate(time.Second))
	}
	if !common.CheckNonce(nonce, now) {
		return fmt.Errorf("nonce 重复")
	}

	key := common.GetHMACKey()
	if key == nil {
		return fmt.Errorf("HMAC 密钥未配置")
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(name + "|" + ts + "|" + nonce))
	expected := mac.Sum(nil)

	sigBytes, err := hex.DecodeString(sig)
	if err != nil {
		return fmt.Errorf("sig 不是合法的 hex")
	}
	if subtle.ConstantTimeCompare(sigBytes, expected) != 1 {
		return fmt.Errorf("签名不匹配")
	}
	return nil
}
