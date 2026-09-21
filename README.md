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
│   │   └── k8sController.go        # K8s 控制器（Deployment/DaemonSet/StatefulSet）
│   └── Middleware/
│       ├── Send.go                 # 数据发送（加密 + 压缩）
│       ├── CheckVersion.go         # 自动更新（重构后删除）
│       ├── CreateConf.go           # 配置加载（带缓存）
│       └── Modles.go               # 数据结构定义
│
├── server/                         # Server 端 - 中央数据接收
│   ├── main.go                     # HTTP 服务入口
│   ├── go.mod
│   ├── Handers/
│   │   ├── hander.go               # 请求处理（解密/解压/校验/分发）
│   │   ├── HanderData.go           # 7 种数据类型处理
│   │   ├── timeouts.go             # 各类数据的注销阈值（可配置）
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
│   │   └── IpPass.go               # IP 白名单中间件（支持域名 / IP / CIDR）
│   └── Modles/
│       └── modle.go                # Server 端数据结构定义
│
└── README.md
```

## 采集频率与注销阈值

| 数据类型 | 采集频率 | 注销阈值（默认） | 说明 |
|---------|---------|----------------|------|
| 心跳 (heart) | 每 15 秒 | 45 秒 | Agent 存活状态 + 版本号 |
| 硬件 (hard) | 每 15 秒 | 45 秒 | CPU / 内存 / 磁盘 / 负载 |
| Nginx (nginx) | 每 15 秒 | 45 秒 | 运行状态 + 网络连接统计（可关闭） |
| K8s 容器 (k8s) | 每 60 秒 | 3 分钟 | Pod 容器 CPU/内存 限制与使用（可关闭） |
| K8s 控制器 (k8sController) | 每 60 秒 | 3 分钟 | 副本数与就绪状态（随 K8s 开关） |
| SSL 证书 (ssl) | 每 5 分钟 | 15 分钟 | 证书到期天数检测（可关闭） |
| 流量切换 (trafficSwitching) | 由外部平台上报 | 3 分钟 | 无需 Agent 采集 |

> **注销阈值必须大于采集周期**，否则时间序列会在两次上报之间被清掉（例如 SSL 若按 20 秒注销，指标每个 5 分钟周期里只存在 20 秒，面板几乎全是断点）。默认阈值取采集周期的 3 倍，即允许连续丢 2 个周期；可通过 Server 的 `unregister_timeout` 覆盖。

三类采集任务由各自独立的 Ticker 驱动，互不阻塞，启动时会先立即执行一次；某类任务上一轮尚未结束时会跳过本轮，不会出现任务堆叠。每轮采集都会重新读取配置（内部 30 秒缓存），修改 `config.yaml` 无需重启 Agent。

## 数据传输流程

```
Agent 端:                                Server 端:
数据结构 → JSON 序列化                   请求体 → AES-GCM 解密
         → Gzip 压缩                              → Gzip 解压（64MB 上限）
         → AES-GCM 加密                            → JSON 解析
         → HTTP POST                               → 校验 project / source
                                                      → 校验 timestamp（防重放）
                                                      → 校验数据条数与标签值长度
                                                      → Worker Pool 异步写入 Prometheus GaugeVec
```

## Agent 配置

配置文件 `config.yaml`，不存在时自动生成默认配置：

```yaml
# 基础配置
agent:
  # 项目名称，用于区分项目（必填）
  project: ""

  # 接受数据地址（Server 端地址，必填）
  metrics_url: ""

  # 是否开启自动更新（需要 Server 托管 version 与 agent/agent）
  # 重构后：语义收窄为"自动更新插件制品"，壳自身改由 systemd / 容器编排更新
  auto_update: false

  # 重构后新增（2026-09-20 定）：启动自检连不上 service 时是否允许继续跑
  #   false（默认）= 直接退出，交给 systemd / 容器编排重启
  #   true          = 仅告警并继续，状态端点与心跳标注 offline_mode
  # allow_offline: false

# 采集开关
metrics:
  # SSL 证书到期时间采集
  ssl:
    enable: false

  # Nginx 服务器信息采集
  nginx:
    enable: false

  # K8s 集群 Pod 资源信息采集
  k8s:
    enable: false
    # kubeconfig 文件路径，集群内运行留空
    config_path: ""

# 加密盐，数据加密传输（需 16/24/32 字节）
encrypted: ""
```

> 启动时会校验 `agent.project`、`agent.metrics_url`、`encrypted` 三项，缺失或密钥长度非法会直接退出并给出提示，避免"进程活着但数据发不出去"。采集与发送失败（含 Server 返回非 2xx）都会打印带 source 前缀的日志。

## Server 配置

配置文件 `config/config.yaml`：

```yaml
# 加密盐（需与 Agent 端一致，16/24/32 字节）
# 优先读环境变量 MONITOR_ENCRYPTED，便于密钥不进仓库
encrypted: ""

# 监听端口，默认 8080
port: "8080"

# 是否识别反向代理传来的 X-Real-IP / X-Forwarded-For，默认 true
# 只有直连方本身可信（在白名单内或本机回环）时才会采信代理头
trustProxy: true

# IP 白名单（允许访问 /metrics 的域名、IP 或 CIDR，5 分钟重新解析一次）
ipPass:
  - prometheus.example.com
  - 192.168.100.0/24

# 按 source 覆盖指标注销阈值（支持 45s / 3m / 1h30m 等 Go duration 格式）
#unregister_timeout:
#  ssl: "21m"
#  k8s: "4m"

# Agent 自动更新文件下发（默认关闭）
# 开启后 dir 下需有 version（纯文本版本号）与 agent/agent（二进制），
# 可选 agent/agent.sha256，提供后 Agent 会强制校验完整性
update:
  enable: false
  dir: "./dist"
  # 拉取更新文件的来源白名单；留空则复用 ipPass（此时 Agent 的 IP 必须也在 ipPass 中）
  ipPass: []
```

项目名称映射文件 `config/projects.json`：

```json
{
  "project-key": "项目显示名称"
}
```

> **部署注意**：上报校验会拒绝与服务端时间相差超过 ±5 分钟的报文（防重放）。请确保被监控主机与 Server 的时钟同步（NTP / chrony），否则会出现"Agent 日志显示上报 400 但服务器日志只说时间戳无效"的情况。该窗口与校验逻辑在 `server/Handers/hander.go` 的 `MaxClockSkew` 中调整。

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
| `/version` | GET | Agent 自动更新用版本号（需 `update.enable`，走 `update.ipPass` 白名单）—— **重构后废弃** |
| `/agent/agent` | GET | Agent 二进制下发（需 `update.enable`，走 `update.ipPass` 白名单）—— **重构后废弃**（壳改由 systemd / 容器编排更新） |
| `/agent/agent.sha256` | GET | 二进制校验文件（可选，提供后 Agent 强制校验）—— **重构后废弃** |

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
| `memory_buffered` / `memory_cached` | 同上 | Buffers / Cached |
| `memory_shared` / `memory_available` | 同上 | Shmem / MemAvailable |

### Nginx 指标

> 以下连接类指标均取自 `ss -s` 的 socket 统计，与 Nginx 自身的请求计数无关，配置告警时请注意语义。

| 指标名 | 标签 | 说明 |
|--------|------|------|
| `nginx_is_run` | hostName, project | 运行状态 |
| `nginx_login_user_count` | 同上 | 登录用户数 |
| `nginx_re_total` | 同上 | 所有协议 socket 总数（`ss -s` 的 Total 行） |
| `nginx_tcp_total` / `nginx_udptotal` / `nginx_raw_total` / `nginx_inet_total` / `nginx_frag_total` | 同上 | 各协议 socket 数 |
| `nginx_total_tcp` | 同上 | TCP 各状态合计 |
| `nginx_tcp_estab` / `nginx_tcp_closed` / `nginx_tcp_orphaned` / `nginx_tcp_timewait` | 同上 | TCP 连接状态 |

### SSL 指标

| 指标名 | 标签 | 说明 |
|--------|------|------|
| `ssl_domain_days_left` | domain, comment, status, resolve, project | 证书剩余天数 |

> 探测失败的域名**不会上报**（而不是上报 `days_left=-1`），避免网络抖动被误判为"证书已过期"。面板上表现为数据缺口，Agent 日志中会有明确原因。

### K8s 容器指标

| 指标名 | 标签 | 说明 |
|--------|------|------|
| `container_cpu_usage` / `container_cpu_limit` / `container_cpu_request` | namespace, podName, container, controllerName, project | CPU 使用 / 限制 / 请求 |
| `container_memory_usage` / `container_memory_limit` / `container_memory_request` | 同上 | 内存使用 / 限制 / 请求 |
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
| `agent_version` | hostName, project | 版本号（数值型，多段版本号只取 major.minor） |

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

- **加密传输**：AES-GCM 加密 + Gzip 压缩；Server 对压缩包设 10MB、解压后设 64MB 上限，防御压缩炸弹
- **上报校验**：校验 `project` / `source` 白名单、`timestamp`（允许 ±5 分钟偏差，抵御密文重放）、单次数据条数（≤20000）与标签值长度（≤512 字节，防止时间序列基数与内存被打爆）
- **差异化采集**：心跳/硬件/Nginx 15 秒，K8s 60 秒，SSL 5 分钟；三类任务各自独立 Ticker 驱动
- **差异化注销**：注销阈值按 source 独立配置（默认 3× 采集周期），避免指标在两次上报之间被清掉；心跳只置 `is_active=0`，便于面板呈现离线主机
- **自动更新**（**重构后删除**：容器里会造成镜像与进程内版本漂移，改由 systemd / 容器编排负责）：Agent 定期对比 `<metrics_url>/version`，远端版本更高时下载新二进制并校验 sha256（远端提供 `.sha256` 时强制校验）；更新基于 `os.Executable()` 的绝对路径，替换前保留 `.bak` 备份，守护模式下写 `restart.flag` 由父进程拉起，普通模式用 `syscall.Exec` 原地重启（PID 不变）
- **守护模式**：`-d` 参数启动守护进程，自动重启异常退出的工作进程；重启采用指数退避（1s → 60s），避免配置错误导致无限快速重启刷日志；PID 文件只由写入者清理
- **高并发处理**：Worker Pool（500 workers）+ 分片锁（256 分片），同项目串行、不同项目并发
- **IP 白名单**：支持域名（DNS 解析缓存 5 分钟刷新）、字面 IP 与 CIDR；仅在直连方可信时采信代理头，直连客户端无法伪造 `X-Real-IP` 绕过
- **连接池复用**：Agent HTTP 客户端带连接池，Server 端响应体及时读取释放连接
- **采集几乎无外部命令依赖**：CPU 核心数/型号、内核版本、内存、负载均直读 procfs，不 fork 子进程；硬件各字段独立降级，单项失败不会导致整份指标丢失（唯一例外是磁盘，见「已知限制」）

## 已知限制

- 磁盘容量采集依赖 `df -P -k` 命令：为了不引入 `*_linux.go` / `*_other.go` 平台拆分文件，此处未使用 `syscall.Statfs`。Agent 若运行在无 `df` 的精简镜像（distroless / scratch）中，磁盘字段会为空，其余硬件指标不受影响
- `controllerName` 标签取自 ReplicaSet 名，滚动发布后标签值会变化，造成时间序列碎片化
- `cleanNamespace` 会剥离 namespace 的 `-v1` / `-v2` 后缀，与集群中的实际 namespace 名不一致
- 标签命名风格混用：`hostName` / `podName` 为驼峰，`os_version` / `kernel_version` 为蛇形
- `/metrics_data` 无请求签名，防护依赖 AES-GCM 密文 + 时间戳窗口校验

## 依赖

### Agent
- `k8s.io/client-go` / `k8s.io/metrics` — Kubernetes 客户端
- `gopkg.in/yaml.v3` — YAML 配置解析

### Server
- `github.com/prometheus/client_golang` — Prometheus 指标库
- `github.com/spf13/viper` + `github.com/fsnotify/fsnotify` — 配置热加载
- `github.com/mitchellh/mapstructure` — map → struct 映射
- `gopkg.in/yaml.v3` — YAML 配置解析
