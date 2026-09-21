package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// ===== 轻量 k8s REST 客户端 =====
// 只实现 kubeconfig 解析 + 只读 GET，替代 client-go 全家桶（制品 34MB → ~5MB）。
// 支持的认证：客户端证书（admin.conf 标准方式）、Bearer Token。

// ---- kubeconfig 文件结构（只取需要的字段）----

type kcCluster struct {
	Server                string `yaml:"server"`
	CAFile                string `yaml:"certificate-authority"`
	CAData                string `yaml:"certificate-authority-data"`
	InsecureSkipTLSVerify bool   `yaml:"insecure-skip-tls-verify"`
	TLSServerName         string `yaml:"tls-server-name"`
}

type kcUser struct {
	ClientCertFile string `yaml:"client-certificate"`
	ClientCertData string `yaml:"client-certificate-data"`
	ClientKeyFile  string `yaml:"client-key"`
	ClientKeyData  string `yaml:"client-key-data"`
	Token          string `yaml:"token"`
}

type kcContext struct {
	Cluster string `yaml:"cluster"`
	User    string `yaml:"user"`
}

type kubeConfigFile struct {
	CurrentContext string `yaml:"current-context"`
	Clusters       []struct {
		Name    string    `yaml:"name"`
		Cluster kcCluster `yaml:"cluster"`
	} `yaml:"clusters"`
	Users []struct {
		Name string `yaml:"name"`
		User kcUser `yaml:"user"`
	} `yaml:"users"`
	Contexts []struct {
		Name    string    `yaml:"name"`
		Context kcContext `yaml:"context"`
	} `yaml:"contexts"`
}

// ---- 客户端 ----

type kubeClient struct {
	server string
	bearer string
	hc     *http.Client
}

// getJSON GET path 并解析 JSON 到 out（超时由 ctx 控制）。
func (c *kubeClient) getJSON(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.server+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if c.bearer != "" {
		req.Header.Set("Authorization", "Bearer "+c.bearer)
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("GET %s: %s: %s", path, resp.Status, strings.TrimSpace(string(b)))
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// loadKubeClient 解析 kubeconfig 并构造客户端。
// insecureSkip / serverName 来自插件配置，优先级高于文件内设置。
func loadKubeClient(path string, insecureSkip bool, serverName string) (*kubeClient, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read kubeconfig: %w", err)
	}
	var kc kubeConfigFile
	if err := yaml.Unmarshal(raw, &kc); err != nil {
		return nil, fmt.Errorf("parse kubeconfig: %w", err)
	}

	var cluster *kcCluster
	var user *kcUser
	for _, e := range kc.Contexts {
		if e.Name != kc.CurrentContext {
			continue
		}
		for i := range kc.Clusters {
			if kc.Clusters[i].Name == e.Context.Cluster {
				cluster = &kc.Clusters[i].Cluster
			}
		}
		for i := range kc.Users {
			if kc.Users[i].Name == e.Context.User {
				user = &kc.Users[i].User
			}
		}
		break
	}
	if cluster == nil {
		return nil, fmt.Errorf("current-context %q 未找到对应 cluster", kc.CurrentContext)
	}
	if cluster.Server == "" {
		return nil, fmt.Errorf("kubeconfig 缺少 server 地址")
	}

	tlsCfg := &tls.Config{
		InsecureSkipVerify: insecureSkip || cluster.InsecureSkipTLSVerify,
	}
	if s := firstNonEmpty(serverName, cluster.TLSServerName); s != "" {
		tlsCfg.ServerName = s
	}

	if user != nil {
		certPEM, err := readB64orFile(user.ClientCertData, user.ClientCertFile)
		if err != nil {
			return nil, fmt.Errorf("client-certificate: %w", err)
		}
		keyPEM, err := readB64orFile(user.ClientKeyData, user.ClientKeyFile)
		if err != nil {
			return nil, fmt.Errorf("client-key: %w", err)
		}
		if certPEM != nil && keyPEM != nil {
			cert, err := tls.X509KeyPair(certPEM, keyPEM)
			if err != nil {
				return nil, fmt.Errorf("x509 keypair: %w", err)
			}
			tlsCfg.Certificates = []tls.Certificate{cert}
		}
	}

	// CA 池（insecure 时不加载，二者互斥）
	if !tlsCfg.InsecureSkipVerify {
		caPEM, err := readB64orFile(cluster.CAData, cluster.CAFile)
		if err != nil {
			return nil, fmt.Errorf("certificate-authority: %w", err)
		}
		if caPEM != nil {
			pool := x509.NewCertPool()
			if !pool.AppendCertsFromPEM(caPEM) {
				return nil, fmt.Errorf("certificate-authority 解析失败")
			}
			tlsCfg.RootCAs = pool
		}
	}

	bearer := ""
	if user != nil {
		bearer = user.Token
	}

	return &kubeClient{
		server: strings.TrimRight(cluster.Server, "/"),
		bearer: bearer,
		hc:     &http.Client{Transport: &http.Transport{TLSClientConfig: tlsCfg}},
	}, nil
}

// readB64orFile 优先取 base64 内嵌数据，否则读文件；都没有返回 nil。
func readB64orFile(b64, file string) ([]byte, error) {
	if b64 != "" {
		return base64.StdEncoding.DecodeString(strings.TrimSpace(b64))
	}
	if file != "" {
		return os.ReadFile(file)
	}
	return nil, nil
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return strings.TrimSpace(a)
	}
	return strings.TrimSpace(b)
}
