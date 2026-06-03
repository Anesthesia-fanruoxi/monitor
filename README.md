# Monitor 监控系统

基于 Go 语言开发的 Agent-Server 架构监控采集系统，Agent 部署在被监控主机上采集指标数据，Server 集中接收并转换为 Prometheus 指标供监控平台拉取。

## 系统架构

```
┌──────────────┐       加密压缩上报        ┌──────────────┐       Prometheus 拉取       ┌──────────────┐
│              │  ─────────────────────▶   │              │  ◀─────────────────────   │              │
│    Agent     │       HTTP POST          │    Server    │       /metrics            │  Prometheus   │
│  (多主机部署) │                           │  (中央接收)   │                           │   Grafana    │
│              │  ◀─────────────────────  │              │                           │              │
└──────────────┘       版本检查/更新        └──────────────┘                           └──────────────┘
```

## 目录结构

```
monitor/
├── agent/                          # Agent 端 - 部署在被监控主机
│   ├── main.go                     # 入口，支持守护模式
│   ├── go.mod
│   ├── Collect/
│   │   └── CollectData.go          # 采集调度（差异化频率）
│   ├── Metrics/
│   │   ├── Active.go               # 心跳上报
│   │   ├── Hard.go                 # 硬件信息采集（CPU/内存/磁盘/负载）
│   │   ├── Nginx.go                # Nginx 运行状态 + 网络连接统计
│   │   ├── Ssl.go                  # SSL 证书到期检测
│   │   ├── K8sContainer.go         # K8s Pod 容器资源采集
│   │   ├── k8sController.go        # K8s 控制器（Deployment/DaemonSet/StatefulSet）
│   │   └── Harbor.go               # Harbor 服务信息
│   └── Middleware/
│       ├── Send.go                 # 数据发送（加密 + 压缩）
│       ├── CheckVersion.go         # 自动更新
│       ├── CreateConf.go           # 配置加载（带缓存）
│       └── Modles.go               # 数据结构定义
│
├── server/                         # Server 端 - 中央数据接收
│   ├── main.go                     # HTTP 服务入口
│   ├── go.mod
│   ├── Handers/
│   │   ├── hander.go               # 请求处理（解密/解压/分发）
│   │   ├── HanderData.go           # 7 种数据类型处理
│   │   ├── UnRegisterHeart.go      # 心跳超时注销
│   │   ├── UnRegisterHard.go       # 硬件指标超时注销
│   │   ├── UnRegisterNginx.go      # Nginx 指标超时注销
│   │   ├── UnRegisterSsl.go        # SSL 指标超时注销
│   │   ├── UnRegisterContainer.go  # 容器指标超时注销
│   │   ├── UnRegisterController.go # 控制器指标超时注销
│   │   └── UnRegisterTrafficSwitching.go  # 流量切换指标超时注销
│   ├── Metrics/
│   │   ├── Metric.go               # 自定义 Prometheus Registry
│   │   ├── HardMetric.go           # 硬件相关指标定义
│   │   ├── NginxMetric.go          # Nginx 相关指标定义
│   │   ├── SslMetric.go            # SSL 相关指标定义
│   │   ├── HeartMetric.go          # 心跳指标定义
│   │   ├── ContainerMetric.go      # 容器指标定义
│   │   ├── ControllerMetric.go     # 控制器指标定义
│   │   └── TrafficSwitchingMetric.go  # 流量切换指标定义
│   ├── IpPass/
│   │   └── IpPass.go               # IP 白名单中间件
│   └── Modles/
│       └── modle.go                # Server 端数据结构定义
│
└── README.md
```

## 采集频率

| 数据类型 | 采集频率 | 说明 |
|---------|---------|------|
| 心跳 (heart) | 每 15 秒 | Agent 存活状态 + 版本号 |
| 硬件 (hard) | 每 15 秒 | CPU / 内存 / 磁盘 / 负载 |
| Nginx (nginx) | 每 15 秒 | 运行状态 + 网络连接统计（可关闭） |
| K8s 容器 (k8s) | 每 60 秒 | Pod 容器 CPU/内存 限制与使用（可关闭） |
| K8s 控制器 (k8sController) | 每 60 秒 | 副本数与就绪状态（随 K8s 开关） |
| SSL 证书 (ssl) | 每 5 分钟 | 证书到期天数检测（可关闭） |

## 数据传输流程

```
Agent 端:                                Server 端:
数据结构 → JSON 序列化                   请求体 → AES-GCM 解密
         → Gzip 压缩                              → Gzip 解压
         → AES-GCM 加密                            → JSON 解析
         → HTTP POST                               → 校验 project/source
                                                      → Worker Pool 异步写入 Prometheus GaugeVec
```

## Agent 配置

配置文件 `config.yaml`，不存在时自动生成默认配置：

```yaml
# 基础配置
agent:
  # 项目名称，用于区分项目
  project: ""

  # 接受数据地址（Server 端地址）
  metrics_url: ""

  # 是否开启自动更新
  auto_update: true

# 采集开关
metrics:
  # SSL 证书到期时间采集
  ssl:
    enable: false

  # Nginx 服务器信息采集
  nginx:
    enable: false

  # Harbor 服务信息采集
  harbor:
    enable: false

  # K8s 集群 Pod 资源信息采集
  k8s:
    enable: false
    # kubeconfig 文件路径，集群内运行留空
    config_path: ""

# 加密盐，数据加密传输（需 16/24/32 字节）
encrypted: ""
```

## Server 配置

配置文件 `config/config.yaml`：

```yaml
# 加密盐（需与 Agent 端一致）
encrypted: ""

# IP 白名单（允许访问 /metrics 的域名，定期 DNS 解析刷新）
ipPass:
  - prometheus.example.com
```

项目名称映射文件 `config/projects.json`：

```json
{
  "project-key": "项目显示名称"
}
```

## 运行方式

### Agent

```bash
# 编译
cd agent
go build -ldflags "-X main.Version=1.0" -o agent .

# 普通模式运行
./agent

# 守护模式运行（自动重启 + 防多开）
./agent -d
```

### Server

```bash
# 编译
cd server
go build -o monitor-server .

# 运行
./monitor-server
```

服务默认监听 `:8080`，提供以下端点：

| 端点 | 方法 | 说明 |
|------|------|------|
| `/metrics` | GET | Prometheus 拉取指标（IP 白名单限制） |
| `/metrics_data` | POST | Agent 数据上报入口 |

## Prometheus 指标

### 硬件指标

| 指标名 | 标签 | 说明 |
|--------|------|------|
| `cpu_percent` | hostName, project, cpu_model, os_version, kernel_version | CPU 使用率 |
| `cpu_load_1` / `cpu_load_5` / `cpu_load_15` | 同上 | 1/5/15 分钟负载 |
| `cpu_total` | 同上 | CPU 核心数 |
| `disk_total` / `disk_used` / `disk_free` | 同上 | 磁盘空间 |
| `disk_used_percent` | 同上 | 磁盘使用率 |
| `memory_total` / `memory_used` / `memory_free` | 同上 | 内存 |
| `memory_used_percent` | 同上 | 内存使用率 |

### Nginx 指标

| 指标名 | 标签 | 说明 |
|--------|------|------|
| `nginx_is_run` | hostName, project | 运行状态 |
| `nginx_login_user_count` | 同上 | 登录用户数 |
| `nginx_tcp_estab` / `nginx_tcp_closed` / `nginx_tcp_timewait` | 同上 | TCP 连接状态 |
| `nginx_udptotal` / `nginx_raw_total` / `nginx_inet_total` | 同上 | 协议连接数 |

### SSL 指标

| 指标名 | 标签 | 说明 |
|--------|------|------|
| `ssl_domain_days_left` | domain, comment, status, resolve, project | 证书剩余天数 |

### K8s 容器指标

| 指标名 | 标签 | 说明 |
|--------|------|------|
| `container_cpu_usage` / `container_cpu_limit` | namespace, podName, container, controllerName, project | CPU 使用/限制 |
| `container_memory_usage` / `container_memory_limit` | 同上 | 内存使用/限制 |
| `container_restart_count` | 同上 | 重启次数 |
| `container_last_termination_time` | 同上 | 上次终止时间 |

### K8s 控制器指标

| 指标名 | 标签 | 说明 |
|--------|------|------|
| `controller_replicas` | namespace, container, controllerType, project | 副本数 |
| `controller_replicas_available` / `controller_replicas_unavailable` | 同上 | 可用/不可用副本 |

### Agent 心跳指标

| 指标名 | 标签 | 说明 |
|--------|------|------|
| `is_active` | hostName, project | 存活状态 (1/0) |
| `agent_version` | hostName, project | 版本号 |

### 流量切换指标

| 指标名前缀 | 说明 |
|------------|------|
| `trafficswitching_total_*` | 累计统计（请求数/成功数/错误数/成功率） |
| `trafficswitching_today_*` | 今日统计（请求数/成功数/错误数/状态码分布） |
| `trafficswitching_realtime_*` | 实时统计（QPS/连接数/延迟） |
| `trafficswitching_error_*` | 错误类型分布 |
| `trafficswitching_runtime_*` | Go 运行时（goroutine/内存/GC） |
| `trafficswitching_transport_*` | 传输层配置 |
| `trafficswitching_timestamp` | 上报时间戳 |

## 关键设计

- **加密传输**：AES-GCM 加密 + Gzip 压缩，保障数据安全与传输效率
- **差异化采集**：高频数据（心跳/硬件）15 秒，低频数据（K8s）60 秒，SSL 5 分钟
- **自动更新**：Agent 定期检查 Server 版本号，发现新版本自动下载并原地重启（`syscall.Exec`，PID 不变）
- **守护模式**：`-d` 参数启动守护进程，自动重启异常退出的工作进程，防多开
- **超时注销**：Server 端 20 秒未收到数据的指标自动清理，避免残留
- **高并发处理**：Worker Pool（500 workers）+ 分片锁（256 分片），同项目串行、不同项目并发
- **IP 白名单**：基于域名 DNS 解析缓存，5 分钟自动刷新
- **连接池复用**：Agent HTTP 客户端带连接池，Server 端响应体及时读取释放连接

## 依赖

### Agent
- `k8s.io/client-go` / `k8s.io/metrics` — Kubernetes 客户端
- `gopkg.in/yaml.v3` — YAML 配置解析

### Server
- `github.com/prometheus/client_golang` — Prometheus 指标库
- `github.com/spf13/viper` + `github.com/fsnotify/fsnotify` — 配置热加载
- `github.com/mitchellh/mapstructure` — map → struct 映射
- `gopkg.in/yaml.v3` — YAML 配置解析
