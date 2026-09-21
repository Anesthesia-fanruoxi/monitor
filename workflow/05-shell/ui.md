# 05 壳 — 界面设计（`statusapi` 只读状态端点）

> **本项目没有图形界面。** 唯一"给人看"的东西是壳的**只读状态端点**，因此本文件就是它的接口形态说明。
> 权威出处：`docs/壳设计.md §十二`（字段与 JSON 形状）、`§7.1`（状态机枚举）。**与本文冲突时以文档为准。**

---

## 一、定位

它是排查「**为什么这台机器的图少了一条线**」的第一入口。设计约束：

- **只读。** 无任何写操作、无控制能力（不做远程管理）。
- **默认只监听 `127.0.0.1`**，可配。暴露公网会把"哪些插件、什么版本、什么时候采的"泄漏出去。
- **请求/响应不含任何指标数据**（那是 `/metrics` 的事），只有状态。
- **每一项都是壳自己掌握的事实**，没有任何一项来自插件的自述——插件自述状态那条线已整体砍掉（契约 §2.5）。

---

## 二、响应形状

```json
{
  "agent_version": "1.0.0",
  "project": "jiexianghua",
  "uptime_seconds": 86400,
  "offline_mode": false,
  "plugins": [
    {
      "name": "hard", "version": "1.2.0",
      "interval": "15s", "interval_source": "config",
      "timeout": "45s", "timeout_source": "manifest",
      "state": "running",
      "last_collect_at": "2026-09-18T15:30:00+08:00",
      "last_collect_ok": true, "last_error": "",
      "skip_count": 0, "restart_count": 0
    },
    { "name": "k8s", "version": "0.9.1", "interval": "60s", "interval_source": "manifest",
      "state": "backoff", "restart_count": 3, "last_error": "handshake timeout" },
    { "name": "ssl", "version": "1.0.0", "state": "disabled" }
  ]
}
```

### 字段归属（必须严格照此实现）

| 字段 | 来源 |
|---|---|
| `name` | `config.yaml` 的键名 / 目录名（**两者必须一致，启动时校验**） |
| `version` | `agent-plugs/<name>/manifest.yaml` |
| `interval` / `timeout` | manifest 默认值 + config 覆盖，算完后**带上来源标记** |
| `interval_source` / `timeout_source` | 枚举只有 `config` / `manifest` / `default` 三个值 |
| `state` / `last_collect_at` / `last_collect_ok` / `last_error` / `restart_count` / `skip_count` | 壳观测（进程活着吗、每轮有没有按时回、失败几次、跳过几次） |
| `agent_version` / `project` / `uptime_seconds` / `offline_mode` | 壳自身（`offline_mode` 来自 `config.yaml` 的 `allow_offline`，见 `docs/壳设计.md §八` 第 5 项） |

**`*_source` 的存在意义**：排查"为什么这台机器采得比别人勤"时，一眼看出周期是被人改过、还是插件作者给的、还是兜底值。它不涉及任何插件知识，纯壳侧信息。

**`last_error` 是非结构化文本**：来自插件的 `stderr`（壳加前缀转发）或壳自己的失败判定，**不是**插件主动上报的结构化信息。这是刻意取舍——让插件把诊断写进日志就够了，不为它发明协议字段。

---

## 三、状态枚举的展示规则（`§7.1`）

| `state` | 含义 | 是否出现在端点里 |
|---|---|---|
| `disabled` | 配置里 `enable=false`，**不下载** | 是 |
| `resolving` | 查 `index.json` 取版本与 sha256 | 是 |
| `downloading` | 下载 `.tmp` | 是（带进度） |
| `verifying` | 校验 sha256 | 是 |
| `installing` | 解压 + 原子替换 | 是 |
| `starting` | 拉起进程 + 握手 | 是 |
| `running` | 正常，含"上一轮采集是否成功" | 是 |
| `backoff` | 崩溃后指数退避中（1s→2s→4s…上限 60s） | 是 |
| `failed` | 连败 ≥5 次且从未健康跑过，**停止自动重启** | 是（需人工介入） |

**`failed` 必须在界面上可辨认**：没有它，一个必然崩溃的插件会让壳永远在"重启→崩溃"之间循环，打爆日志和 CPU。看这个端点的人需要一眼知道"这个插件已经不会自己好了"。

---

## 四、验收清单（人工核查动作）

按顺序 `curl` 并逐项核对：

- [ ] 默认监听在 `127.0.0.1`，从别的机器访问不通（除非显式改配置）
- [ ] `enable=false` 的插件出现在列表里且 `state=disabled`，磁盘上没有它的制品（懒下载）
- [ ] 正常运行的插件：`state=running`、`last_collect_ok=true`、`last_collect_at` 与周期吻合
- [ ] 改 `config.yaml` 里的 `plugins.<name>.interval` → `interval_source` 变成 `config`，且**不重启进程**
- [ ] 杀掉插件进程 → `state` 走到 `backoff`、`restart_count` 递增；连败 ≥5 次且从未健康 → `failed`
- [ ] 插件往 stderr 打一行错 → `last_error` 里能看到（带插件名前缀）
- [ ] 响应里**没有任何指标数据**、**没有任何字段来自插件自述**
- [ ] `allow_offline: true` 且故意让 service 不可达时：壳**能启动**且 `offline_mode=true`；而 `allow_offline: false` 时**必须退出**（退出码非 0）
- [ ] `offline_mode=true` 时心跳仍照常上报（它只是标注状态，不是停上报）

---

## 五、不做

- 不做写操作 / 控制按钮 / 远程指令。
- 不做认证体系（默认只监听回环已足够；要跨机看需显式配置并自行保证网络隔离）。
- 不做指标展示（那是 Prometheus / Grafana 的事）。
- 不引入前端构建链、不引入模板引擎、不引入任何依赖（壳的依赖白名单：stdlib + `yaml.v3`）。
