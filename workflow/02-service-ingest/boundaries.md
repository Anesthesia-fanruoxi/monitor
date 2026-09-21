# 02 service 接收链路 — 边界

## 做什么

- **删旧链路**（80 个静态定义 / 旧 `data` 路径 / `HanderData.go` / 7 个 `UnRegister*.go` / `timeouts.go` / `allowedSources`）
- 抽 `internal/registry` / `internal/ingest`
- 声明式路径端到端打通
- 动态注册防护全套 + 全局序列闸门
- 自监控指标 `monitor_ingest_*`
- 批量写 + 队列满丢弃计数
- 服务端换 `klauspost/compress/gzip`
- 压测基线（硬闸门）

## 不做什么

- **不动 agent**。本任务全程在 `server/` 内。
- **不做兼容层，也不做"双轨并存"。** 没有新旧并存的窗口：静态定义**直接删**（`docs/服务端设计.md §2.2 / §5.5`）。删它不是因为"迁移完成"，而是因为**它挡着新链路**——同名指标在同一个 Registry 里只能注册一次。
- 不做 03 的制品仓库。
- 不改指标名（通用规则 8）。`docs/插件设计.md §5` 的映射表是唯一权威。
- 不引入必须部署的中间件（Kafka / Redis / etcd）。明确不做。

## 红线

- **压测不达标不进入下一阶段。** 这不是"尽量"，是闸门：接收端扛不住时后面的壳与插件全部无意义。
- 不得放宽校验来让测试变绿（比如取消基数上限、把 `schema:v1` 放过）。
- 不得把"队列满"改回静默降级。
- **旧 `data` 路径必须显式 4xx + 日志**，不得静默丢弃——静默丢弃等于"数据不见了但没人知道"。
- `klauspost/compress/gzip` 只加在 `server/go.mod`，**不得加进 `agent/go.mod`**（壳契约 I3）。
- 不动生产环境；压测在测试节点跑。

## 产出物

| 文件 | 内容 |
|---|---|
| 删除清单 | 逐个文件确认的删除记录（S1 的产出） |
| `server/internal/registry/` | 动态指标注册表 + 防护 + 通用注销器 |
| `server/internal/ingest/` | 接收管道（解密/解压/校验/分流） |
| `server/Metrics/monitor.go`（或同类） | `monitor_ingest_*` 自监控 |
| 压测客户端（Go） | 基线压测 |
| 单测 | `§12.1` 表中 `registry` / `ingest/decode` / `ingest/validate` 三行 |

## 依赖

- 上：01（共享协议类型）
- 下：03、04（03 的制品仓库复用已有的 HTTP 层惯例；04 的 SDK 复用本任务落定的线格式类型）

## 已知坑

- Windows 上模拟"非可信来源"用 `net.Dialer{LocalAddr: <本机非回环 IP>}` 连到该 IP 自身。
- 判断指标值解析行尾数字，别子串匹配（`") 0"` 这种写法会误判）。
- 中文控制台 GBK，读 stdout 要显式 decode。
- 删旧链路后 `/metrics` 会短暂"空"，这是预期的——S3 之后由声明式路径重新填满。**不要为了让 `/metrics` 好看而保留静态定义。**
