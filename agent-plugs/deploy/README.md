# deploy

容器化部署清单：daemonset-agent-hard.yaml（每节点硬件采集）。

agent 镜像构建在 agent 模块（与 server 同模式）：agent/Dockerfile + agent/release.ps1，
产物：hub.hzbxhd.com/monitoring/agent:1.3（latest 同步）。

k8s 集群级指标（节点/pod/工作负载/PVC）不在集群内采集，继续由集群外 agent（如 harbor 裸机）通过 kubeconfig 采集。
