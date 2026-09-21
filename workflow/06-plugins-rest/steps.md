# 06 其余插件（`nginx` / `ssl` / `k8s`） — 步骤

> 目标：把 `nginx` / `ssl` / `k8s` 三个采集器做成插件，**指标名与标签集合与 `§5` 映射表一字不差**，对应 Grafana 面板零改动。
> 前置：01、03、04、05（需要一个能跑起来的新 agent 做验证）
> 参考：`docs/插件设计.md §5.2 §5.3 §5.4`（**唯一权威**）、`§六`（实现要点）、`§10`
> **前置决策已定（2026-09-20）**：`k8s` 加两个配置项 `plugins.k8s.namespaces`（空 = 全部 namespace）与 `only_with_limits`（默认 `false`），**默认行为与现状一致**。

---

## 步骤

### S1 `nginx` 插件（13 个指标）

- **动作**：对照 `§5.2` 写采集逻辑。`source=nginx`、标签 `{hostName, project}`、timeout `45s`（周期 15s）。
- **验收（逐字核对）**：
  - 13 个指标名与 agent 字段的对应关系正确，**重点核对词序交叉的那一对**：
    - agent 字段 `tcpTotal` → 指标 `nginx_tcp_total`
    - agent 字段 `totaltcp` → 指标 `nginx_total_tcp`
    - 这对曾经写错过，实现时逐字核对。
  - `nginx_is_run`：`systemctl is-active nginx` 的 stdout 精确等于 `active` → 1，否则 0；
  - **命令不存在或执行失败时，对应指标上报 0 并只告警一次（warn-once），不让整个插件失败。**
  - 清醒知道代价：这意味着缺少这些命令的精简镜像里会产出一堆 0——**这是现状行为，保持不变**。
- **产出**：`agent-plugs/nginx/{main.go,default.yaml}`
- **timeout 注意**：`§5.2` 列出的外部命令依赖（`ss`/`who`/`systemctl`）**本次保持现状**；改为读 procfs 属**范围外**待拍板项（`docs/README.md §四「本次范围外」`），不在本任务做。

### S2 `ssl` 插件（1 个指标）

- **动作**：对照 `§5.3`。`source=ssl`、标签 `{domain, comment, status, resolve, project}`、timeout `15m`（周期 5min）、**无 hostName 标签**（现状如此）。
- **验收**：
  - `expiration`（RFC3339）**丢弃、不声明**——agent 上报过但服务端从未定义对应指标，没有面板依赖它；
  - `domain` / `comment` / `status`（固定 `"正常"`）/ `resolve`（固定 `true`）→ 标签；
  - `domains.txt` 改为配置项 `plugins.ssl.domains_file`（**这是唯一建议在本次改的行为**，因为它依赖 CWD，而壳的 CWD 不可控）；
  - 域名来源三处按顺序去重合并：① 递归遍历 `/etc/nginx/conf.d` 下所有 `.conf` 的 `server_name\s+(.*);`；② `/etc/hosts` 非 localhost 条目；③ 配置项指向的文件；
  - 并发上限 20、单域名 5 秒超时；
  - **探测失败的域名本轮不上报**（跳过，不填 `days_left = -1`）；
  - 全部失败 → 本轮 `metrics` 为空数组（壳收到空 `metrics` 正常返回，**不记为失败**）。

### S3 `k8s` 插件（11 个指标，原两个采集器合并）

- **动作**：把 `k8sContainer` 与 `k8sController` 合并为**一个插件**，共用同一个 clientset。
  - `source=k8s`（原 `k8sController` 这个 source 名随之退役）
  - timeout `3m`（周期 60s）
  - 客户端：`kubeconfig` 为空 → `rest.InClusterConfig()`；非空 → `clientcmd.BuildConfigFromFlags("", path)`。**只初始化一次**（长驻进程的红利）
  - API：`CoreV1().Pods("").List`、`MetricsV1beta1().PodMetricses("").List`、`AppsV1().Deployments/DaemonSets/StatefulSets("").List`，全 namespace，上下文超时 30s
- **验收（逐字核对）**：
  - 容器组 8 个指标的标签集合 `{namespace, podName, container, controllerName, project}`；控制器组 3 个指标的标签集合 `{namespace, container, controllerType, project}`；
  - `lastTerminationTime` **单位是分钟**（现有代码注释写"秒"但实现是分钟，Help 也写分钟——**以分钟为准**；差值 <60s 记 0）；
  - `controllerName` 取 `Pod.OwnerReferences` 中 `Kind ∈ {ReplicaSet, Deployment, StatefulSet}` 的 `Name`，无控制器时为**空串**；
  - `controller_replicas*` 三指标按 Deployment/DaemonSet/StatefulSet 三种口径取值，`unavailable` 在 StatefulSet 下是 `Replicas - ReadyReplicas`（**负值截断为 0**）；
  - `container` 这个标签**承载的是控制器名**——字段名与语义错位，但它是既有契约，**保持不动**。
- **已知遗留（本次不动）**：`controllerName` 取的是 ReplicaSet 名 → 每次滚动发布产生新标签值、容器指标序列碎片化。修法是向上多走一层取 Deployment 名，**但会改标签值 → 属范围外、单独拍板**（`docs/README.md §四「本次范围外」`）。
- **配置项（已定）**：`plugins.k8s.namespaces`（空 = 全部 namespace）与 `only_with_limits`（默认 `false`，只报声明了 request/limit 的容器）。**默认全量 = 与现状一致**；它同时是「减小载荷」的主要抓手——收紧后服务端 gzip 与 JSON 成本同步下降。
- **产出**：`agent-plugs/k8s/{main.go,default.yaml}`

### S4 指标契约对齐验证（逐字）

- **动作**：三个插件逐个在测试节点跑起来，与 `§5` 映射表对照。
- **验收（每个插件一次）**：
  - 逐条对照 `§5` 映射表，**指标名 / 标签集合 / Help 三者一字不差**；
  - 同一面板查询在插件上线后曲线连续（无断点、无新序列、无缺失序列）；
  - 静态信息缓存（如 CPU 型号 / OS / 内核 30 分钟缓存）在长驻进程里正常工作。

---

## 退出条件

- [ ] 三个插件的指标名 / 标签集合与 `§5` 逐字一致
- [ ] 每个插件上线后，对应 Grafana 面板曲线连续
- [ ] `nginx` 缺命令时上报 0 + warn-once（不失败）；`ssl` 探测失败跳过；`k8s` 两个采集器合并后指标数不变
- [ ] 通用规则 5 的门禁全过
