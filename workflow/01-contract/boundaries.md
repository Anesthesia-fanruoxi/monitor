# 01 契约落地 — 边界

## 做什么

- 抽 `pkg/plugproto`（独立 module）：三段请求 + 4 字段 Envelope + 指标线格式
- 落 `agent/internal/proto` 防腐层（≤110 行）
- 宽容解析与缺省降级表实现 + 单测
- 依赖白名单校验

## 不做什么

- **不给 Envelope 加第 5 个字段。** 字段数在 2026-09-18 由 9 收缩到 4，移出的是 `name` / `version` / `interval` / `timeout` / `status`。理由：**壳能从别处（`config.yaml` / `manifest.yaml`）拿到的，就不该问插件要；壳拿不到的，也不要。**
- 不实现壳的其它模块（配置、监管、调度、上报）——那是 05。
- 不实现插件 SDK——那是 04。本任务只定义类型，不实现 `Run()`。
- 不引入 protobuf。本次仍走 JSON（线格式已冻结）；protobuf 属**范围外待拍板项**（见 `docs/README.md §四「本次范围外」`），需单独评审。
- 不用 codegen 框架。类型手写，`json.RawMessage` 是刻意选择。

## 红线

- **`Metrics` 永远不解码。** 一旦 `internal/proto` 里出现 `Unmarshal(&e.Metrics, ...)`，壳就从"搬运工"变成了"解析器"，4 字段的边界当场破掉。
- 壳不得 import `agent-plugs` 的任何包（除 `plugproto`）。
- 不新增第三方依赖（契约 I3 只允许 stdlib + `yaml.v3`）。
- 不改 `docs/壳契约-v1.md`。若实现中确实发现契约有问题，**停下询问**，走契约 §12 的 8 条评审清单，不要自行放宽。

## 产出物

| 文件 | 内容 |
|---|---|
| `agent-plugs/plugproto/go.mod` | 独立 module |
| `agent-plugs/plugproto/{types,encode,decode}.go` | 唯一协议定义处 |
| `agent/internal/proto/decode.go` | 壳的防腐层（≤110 行，恰好 4 个 json tag） |
| 单测 | 宽容解析 + 缺省降级表 |

## 依赖

- 上：无（第一个任务）
- 下：02（service 用同一份类型）、04（SDK 用它编解码）、05（壳用它防腐）

## 冻结提醒

本任务产出的是**契约的代码形态**。契约一旦落地为代码，改动就不再是"改个文档"——三端都要动。所以每次想动它，先问：这个字段能不能从 `config.yaml` / `manifest.yaml` 拿到？能，就不该进信封。
