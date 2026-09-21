# 01 契约落地：plugproto + 壳防腐层 — 步骤

> 目标：协议类型定义收成**唯一一份**，壳侧只留**一个**允许读插件输出的文件。
> 前置：无（这是第一个任务）；工程门禁由 `workflow.md` 通用规则 5 约束。
> 参考：`docs/壳契约-v1.md`（§2 四字段、§2.2/§2.3 宽容与降级、§4 八条不变式、§6）、`docs/壳设计.md §六`、`docs/插件化架构设计.md §10`
> **本任务的一切以 `docs/壳契约-v1.md` 为准**；与其它文档冲突时以契约为准。

---

## 步骤

### S1 抽 `pkg/plugproto` 独立 module

- **动作**：新建 `agent-plugs/plugproto/`（独立 `go.mod`，`module plugproto`），放协议类型：
  - 请求三段：`{"id":1,"cmd":"hello","proto":1}` / `{"id":2,"cmd":"collect"}` / `{"id":3,"cmd":"shutdown"}`
  - 编码函数：`EncodeHello(id)` / `EncodeCollect(id)` / `EncodeShutdown(id)`
  - 4 字段 `Envelope`：`ID uint64` / `OK bool` / `Proto int` / `Metrics json.RawMessage`
  - 指标声明与上报报文的线格式（`schema`/`metrics`/`name`/`help`/`labels`/`common`/`timeout`/`samples`）
- **为什么独立 module**：agent 需要协议类型，但 agent **绝不能依赖 `agent-plugs`**（否则插件代码被链接进壳，产物膨胀、依赖污染）。
- **产出**：`agent-plugs/plugproto/{types.go,encode.go,decode.go,go.mod}`
- **验收**：类型定义唯一——三端（service / 壳 / 插件 SDK）都 import 它；`grep -rn "type Envelope struct" .` 全仓库只 1 处命中。

### S2 落壳侧防腐层 `agent/internal/proto`（≤110 行）

- **动作**：壳里唯一允许读插件输出的文件。只 decode 4 个字段，`Metrics` 保持 `json.RawMessage`，**永不 `Unmarshal`**。壳对 `Metrics` 只做两件事：测长度、算 sha256（可选，排障用）。
- **产出**：`agent/internal/proto/decode.go`
- **验收**：`grep -o 'json:"[a-z_]*"' agent/internal/proto/decode.go | sort -u` **恰好 4 行**（契约 I1）。

### S3 落地 I1 的另一半：业务字面量禁止项

- **动作**：`internal/proto` 之外不得出现 `"samples"` / `"labels"` / `"common"` / `"help"` 字面量。
- **验收**：`grep -rn '"samples"\|"labels"\|"common"\|"help"' agent/ --include=*.go`（排除 `internal/proto` 包）为空。
- **备注**：这是**领域无关性**的机械保证。新增壳的测试用例时若需要构造真实 nginx / k8s 数据，说明业务知识漏进壳了。

### S4 宽容解析实现 + 单测

- **动作**：按契约 §2.2 / §2.3 实现：
  | 情形 | 取值 |
  |---|---|
  | `id` 数字 / 字符串数字 / 缺失 | 缺失按"当前在途请求"匹配（壳一次只允许一个在途请求） |
  | `ok` 缺失 | 视为 `false`（保守） |
  | `proto` 缺失 | 视为 `1` |
  | `proto > 1` | **显式拒绝**并提示"需升级 agent"，不静默降级 |
  | `metrics` 缺失/为空 | 本轮无数据，**不记为失败**（`ok:true` 且无数据合法） |
  | `ok:false` | 判该轮失败 + 计数，`metrics` 内容一律忽略 |
  | `id` 不匹配 | 丢弃该响应 + 计一次失败，不 panic、不串轮 |
  | 整行不是 JSON | 判该插件失败（按"写坏 stdout"处理） |
  | 其它任何键 | 一律忽略 |
- **产出**：解析实现 + 表驱动单测。
- **验收**：上表**每一行各一条用例**。
- **必须做到**：所有降级走 **warn-once**（每插件每字段只告警一次，避免刷屏）。**宽容却不告警，等于把插件的 bug 永久藏起来。**

### S5 校验依赖白名单（契约 I3）

- **验收**：`cd agent && go list -m all`，除自身与 `plugproto` 外只应出现 `gopkg.in/yaml.v3`。
- **备注**：壳的依赖越少，越不会被上游破坏性变更波及。这条是硬约束，不是建议。

---

## 退出条件

- [ ] 类型定义全仓库只有一份，三端共享
- [ ] 契约 I1 / I3 的 grep 可执行且通过
- [ ] 宽容解析与降级表的每一行都有用例
- [ ] 两份 `go build ./...` + 交叉编译 + `go vet` 干净（通用规则 5 的门禁）
