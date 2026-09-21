# 04 插件 SDK 与 hard 打样 — 步骤

> 目标：插件作者（你）只写采集逻辑，SDK 负责协议、配置读取、stdout 保护、指标声明收集。用最简单的 `hard` 插件把整条路走通。
> 前置：01（协议类型）
> 参考：`docs/插件设计.md` §2.1、§3（协议插件侧视角）、§4（SDK）、§5.1（hard 17 指标）、§7、§8
> **本任务的一切以 `docs/插件设计.md` 为准。**

---

## 步骤

### S1 `agent-plugs/sdk/plugin.go`：协议与生命周期

- **动作**：
  ```go
  type Plugin interface {
      Collect(ctx context.Context) error   // 唯一方法，不返回指标列表
  }
  func Run(p Plugin)
  ```
  - 三段命令分发：`hello` / `collect` / `shutdown`
  - `proto` 协商：收到 `proto > 1` → 明确拒绝并提示"需升级 agent"
  - 无并发（`§3.5`）：同一时刻只有一个 `collect` 在处理
- **产出**：`agent-plugs/sdk/plugin.go`
- **验收**：手工 `echo '{"id":1,"cmd":"hello","proto":1}' | ./hard` 正确应答 `{"id":1,"ok":true,"proto":1}`。
- **注意**：`describe` 已取消（配置改读 `manifest.yaml`），协议只有三段。

### S2 `sdk/stdout_guard.go`：stdout 只走协议

- **动作**：把 `fmt.Println` / `fmt.Printf` 之类重定向到 stderr。插件作者随手打一行日志不该毁掉整条采集链。
- **验收**：插件里 `fmt.Println("x")` 后，壳侧协议解析仍正常（不是"解析失败但没报错"，而是日志出现在 stderr、协议行干净）。
- **为什么必须做**：`docs/插件化架构设计.md §十三` 风险表第一条就是这个——"插件 stdout 被日志污染 → 协议解析失败，采集全断"。

### S3 `sdk/config.go`：读 agent 的 `config.yaml`

- **动作**：读 `MONITOR_CONFIG` 指向的 `config.yaml`，取 `plugins.<name>` 子树：
  - **跳过三个保留键**：`enable` / `interval` / `timeout`（壳读的，不是插件参数）
  - 与制品自带的 `default.yaml` 做**浅合并**（config 覆盖 default）
  - 带 **mtime 缓存**：每轮 collect 调一次，只有文件改动才重新解析
  - **插件名推导**：从 `MONITOR_PLUGIN_DIR` 的 basename 推（目录名 = 配置键名 = `manifest.name`）
  - **读不到配置 → 回落纯默认值 + 打一行 stderr，不退出**
- **产出**：`agent-plugs/sdk/config.go`
- **验收**：
  - 改 `config.yaml` 后**下一轮 collect 生效且不重启进程**；
  - 删掉 `config.yaml` 后插件仍能采集（用默认值）；
  - 三个保留键不会出现在插件读到的配置项里。

### S4 指标声明 API + 自动收集

- **动作**：
  ```go
  sdk.Gauge("cpu_percent", "CPU使用率", "cpu")     // 在采集逻辑旁边顺手声明
  ```
  SDK 自动收集 `(name, help, labels, timeout)` 随每轮上报携带。**插件不手写第二份清单。**
- **纪律（必须体现在 API 或 lint 上）**：
  - `help` 必须是**常量**，不能拼时间戳/主机名（否则每轮描述都在变，服务端会反复重注册）；
  - `timeout` 由 SDK 从 `MONITOR_PLUGIN_TIMEOUT` 填，插件不填。
- **验收**：插件代码里搜不到手写的指标清单；`help` 拼接会被拦住（lint 或编译期）。

### S5 `sdk.Hostname()`

- **动作**：`os.Hostname()` → 失败退回读 `/etc/hostname` → **5 分钟缓存**。
- **验收**：与壳侧同名算法的一致性断言测试通过（两份 30 行算法必须同结果）。
- **纪律**：**不要改成"环境变量优先"。** 已经删掉了 `MONITOR_HOSTNAME` 注入，回归注入就是走回头路。

### S6 `hard` 插件：17 个指标

- **动作**：逐条对照 `docs/插件设计.md §5.1` 写采集逻辑。
- **验收（逐字核对）**：
  - 指标名 / 标签集合 / Help 三者与 `§5.1` 的映射表**一字不差**；
  - `cpu_count`（agent 字段名）→ `cpu_total`（指标名）**不要混淆**；
  - `memory_*` **单位是 kB 且原样上报**（没乘 1024），`disk_*` 是字节——**不要"顺手修"**；
  - `cpu_total` 的 Help 保持小写 `cpu 核心数`；
  - 静态信息（CPU 型号 / OS / 内核）保留 30 分钟缓存（长驻进程正好适合）；
  - `cpu_percent` 首次 / 计数器回绕 / 距上轮超 1 分钟 → 只重建基线并返回"该轮无数据"。
- **标签声明方式**：`labels = ["hostName","cpu_model","os_version","kernel_version"]`，`common = {cpu_model, os_version, kernel_version}`，每条 sample 只带 `hostName`。
- **timeout**：`45s`（周期 15s）。

### S7 插件进程级单测

- **动作**：testdata 固定输入 → 固定输出（`§8`）。
- **验收**：`§8` 的用例全绿；`§7.4` 的本地开发自检流程可用（脱离壳直接跑插件）。

---

## 退出条件

- [ ] 手工 `echo '{"id":1,"cmd":"hello","proto":1}' | ./hard` 正确应答
- [ ] `fmt.Println` 不污染 stdout
- [ ] 改配置下一轮生效且不重启进程；配置缺失不退出
- [ ] `hard` 17 个指标与 `§5.1` 逐字一致
- [ ] 通用规则 5 的门禁全过（含交叉编译）
