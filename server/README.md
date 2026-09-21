# monitor-server
> 自用监控服务端
## 一、背景
> 因为项目众多，而每个项目需要一套完整的prometheus+grafana+alertmanager等众多组件。
+ 基础硬件信息（cpu，内存，硬盘） 
+ 服务监控（服务运行状态）
+ 证书监控
+ k8s集群监控
+ pod状态
## 二、实现效果
> 
> agnet采集，发送server端，统一接受数据处理，从而只需要部署一套完整的监控，从而实现项目快速扩展，从一套项目部署完成后，到实现监控告警，只需要几分钟。

## 三、已完成进度
> 服务端实现

+ 实现数据格式化，对数据进行处理成prometheus可以采集的状态。
+ 实现对prometheus指标自管理，由于是静态指标，所以需要实现对指标进行过期处理。
+ 实现对数据加密处理，对接受到的数据，agent采集数据完成后使用加密，发送给服务端，服务端接受后解密，解密完成后再对数据格式化处理。
+ 实现心跳管理，对agnet进行心跳监控。
+ 实现限制IP请求/metrics
+ 支持监听端口配置（`port`）与密钥环境变量注入（`MONITOR_ENCRYPTED` 优先于配置文件），密钥可以不必进仓库；密钥长度非法时启动即失败，不会出现"服务活着但所有上报都 400"。
+ 支持 Agent 自动更新的文件下发端点 `/version`、`/agent/agent`（默认关闭，使用独立的 `update.ipPass` 白名单，未配置时才回退到 `ipPass`）。**重构后此通道废弃**：壳不再自更新，制品下载端点只服务插件制品。
+ 解压后体积设 64MB 上限防压缩炸弹；心跳超时标记后回收时间戳条目，长时间运行不会持续占用内存。
+ 上报校验：`timestamp` 允许 ±5 分钟偏差（抵御密文重放）、单次数据条数 ≤20000、标签值长度 ≤512 字节。
+ 注销阈值按 source 独立配置（`unregister_timeout`），默认取 Agent 采集周期的 3 倍；此前统一硬编码 20 秒，导致 SSL（5 分钟周期）的指标约 93% 时间不存在。
+ IP 白名单支持域名 / 字面 IP / CIDR；仅在直连方可信（白名单内或本机回环）时采信 `X-Real-IP`、`X-Forwarded-For`，直连客户端无法伪造请求头绕过。
+ 补齐了此前被丢弃的指标：`memory_buffered` / `memory_cached` / `memory_shared` / `memory_available`、`container_cpu_request` / `container_memory_request`。

## 四、后续
> 其中研究过influxdb，使用influxdb进行存储，但是由于influxdb第一次使用，导致出现无法实现告警通知。后续有时间再写influxdb的，在某些情况下，influxdb对比tsdb要好的多。
