# 04 插件 SDK 与 hard 打样 — 边界

## 做什么

- `sdk/plugin.go`（`Run` / `Plugin{Collect}` / 三段命令 / proto 协商）
- `sdk/stdout_guard.go`（stdout 保护）
- `sdk/config.go`（读 `config.yaml` 的 `plugins.<name>`、跳保留键、浅合并、mtime 缓存、插件名推导）
- 指标声明 API + SDK 自动收集
- `sdk.Hostname()`
- `hard` 插件 17 指标 + 进程级单测

## 不做什么

- **不实现 `describe` 命令**。协议已收窄为三段（`hello`/`collect`/`shutdown`）；插件名从目录名推、版本与周期阈值读 `manifest.yaml`。
- **不给 Envelope 加字段。** 响应只有 `{id, ok, proto, metrics}`。**状态、版本、周期、诊断一律不进信封**——诊断走 stderr。
- **不实现其它插件**（nginx / ssl / k8s 是 06）。
- 不做加密、不做网络、不做重试。插件只输出明文 JSON。
- 不实现壳侧逻辑（05）。
- **不依赖其它插件**（明确不支持插件间依赖）。

## 红线

- **stdout 只走协议。** 这条必须在 SDK 里强制，不能靠自觉。
- **插件不知道 `project`，也读不到它。** `project` 由壳从 `config.yaml` 读出后写进上报报文。SDK 不提供 `Project()` 之类的 API。
- **不手写第二份指标清单。** 指标定义就用 `sdk.Gauge(...)` 写在采集逻辑旁边；再手写一份必然不同步。
- **`help` 必须是常量。** 拼接时间戳会让服务端每轮重注册。
- **`memory_*` 保持 kB 原样。** 顺手"修正"单位会让面板曲线差 1024 倍。
- 插件进程不得写 stdout 之外的任何文件（配置只读）。
- 不把 `plugproto` 之外的依赖带进插件（SDK 要刻意做窄）。

## 产出物

| 文件 | 内容 |
|---|---|
| `agent-plugs/sdk/{plugin,config,stdout_guard,host,metric}.go` | SDK |
| `agent-plugs/sdk/README.md` | 面向非 Go 语言的协议说明（`§9` 的落点） |
| `agent-plugs/hard/{main.go,default.yaml}` | 打样插件 |
| 单测 | 协议应答 + 配置读取 + hostname 一致性 + 解析固定输入输出 |

## 依赖

- 上：01（`plugproto` 的协议类型）
- 下：03（制品仓库打包的对象就是本任务的 `hard`——**因此 03 不能先于 04**）、05（壳要用 `hard` 做全链路验证）、06（其余插件复用同一 SDK）

## 关键纪律

**指标名只增不改。** 加指标不用动 Grafana 面板与告警规则，改名则要动——面板与告警引用的是名字。要改就新增 `xxx_v2`，不要改 `xxx` 的含义。这是"描述随每轮上报 + 动态注册"这套契约的配套纪律：**描述可以变，名字不能变。**
