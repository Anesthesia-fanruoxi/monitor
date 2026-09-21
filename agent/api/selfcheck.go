package api

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"agent/config"
)

// Checker 启动自检：配置有效性 / 密钥长度 / 插件目录 / service 可达性。
type Checker struct {
	cfg          *config.ConfigCenter
	pluginsDir   string
	serviceURL   string
	allowOffline bool
}

// NewChecker 构造。
func NewChecker(cfg *config.ConfigCenter, pluginsDir, serviceURL string, allowOffline bool) *Checker {
	if pluginsDir == "" {
		pluginsDir = filepath.Join(filepath.Dir(exePath()), "plugins")
	}
	return &Checker{
		cfg:          cfg,
		pluginsDir:   pluginsDir,
		serviceURL:   serviceURL,
		allowOffline: allowOffline,
	}
}

// exePath 是 os.Executable 的薄封装，方便测试替换。
var exePath = func() string {
	p, _ := os.Executable()
	return p
}

// Run 逐项检查。返回 nil 表示 OK。
// allow_offline + service 不可达时，仅告警 + 标注 offline_mode（不退出）。
func (c *Checker) Run() error {
	if err := c.checkConfigBasics(); err != nil {
		return fmt.Errorf("config: %w", err)
	}
	if err := c.checkEncryptionKey(); err != nil {
		return fmt.Errorf("encryption_key: %w", err)
	}
	if err := c.checkPluginsDir(); err != nil {
		return fmt.Errorf("plugins_dir: %w", err)
	}
	if err := c.checkService(); err != nil {
		if c.allowOffline {
			log.Printf("[shell] service 不可达：%v，allow_offline=true，进入 offline_mode", err)
			c.cfg.SetOffline(true)
		} else {
			return fmt.Errorf("service: %w", err)
		}
	}
	return nil
}

// checkConfigBasics 配置最基本必填项：project 非空，metrics_url 非空（allow_offline 除外）。
func (c *Checker) checkConfigBasics() error {
	cfg := c.cfg.Get()
	if strings.TrimSpace(cfg.Project) == "" {
		return errors.New("project 不能为空")
	}
	if !c.allowOffline && strings.TrimSpace(cfg.MetricsURL) == "" {
		return errors.New("metrics_url 不能为空")
	}
	return nil
}

// checkEncryptionKey 密钥长度校验（空=允许明文，16/24/32=合法 AES 密钥）。
func (c *Checker) checkEncryptionKey() error {
	cfg := c.cfg.Get()
	if cfg.EncryptionKey == "" {
		log.Println("[shell] encryption_key 为空，上报将使用明文+gzip")
		return nil
	}
	switch len([]byte(cfg.EncryptionKey)) {
	case 16, 24, 32:
		return nil
	default:
		return fmt.Errorf("长度 %d，仅允许 0/16/24/32 字节", len([]byte(cfg.EncryptionKey)))
	}
}

// checkPluginsDir 插件目录必须存在且可写。
func (c *Checker) checkPluginsDir() error {
	if err := os.MkdirAll(c.pluginsDir, 0o755); err != nil {
		return err
	}
	// 写权限：尝试 touch 一个临时文件
	tmp := filepath.Join(c.pluginsDir, ".shell_check_write")
	if err := os.WriteFile(tmp, []byte("ok"), 0o644); err != nil {
		return err
	}
	return os.Remove(tmp)
}

// checkService 用 2s 超时 ping service。HEAD 优先，404/405 也算可达（网络层 OK）。
func (c *Checker) checkService() error {
	url := strings.TrimSpace(c.serviceURL)
	if url == "" {
		return errors.New("service_url 空")
	}
	client := &http.Client{Timeout: 2 * time.Second}
	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	if resp.StatusCode >= 500 {
		return fmt.Errorf("service 响应 %d", resp.StatusCode)
	}
	return nil
}
