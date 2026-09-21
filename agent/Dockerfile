# monitor-agent 运行镜像（不编译，二进制由本地交叉编译产出）
#
# 前置产物（构建上下文 = 仓库根）：
#   agent/build/monitor-agent      GOOS=linux GOARCH=amd64 交叉编译的二进制
#
# 插件不进镜像：启动时按启用项从 server /plugin/download 自动下载到 /app/plugins
# config.yaml 不进镜像：由 ConfigMap 挂载到 /app/config.yaml
#
# 构建与推送：agent/release.ps1
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata \
    && cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime \
    && echo "Asia/Shanghai" > /etc/timezone

WORKDIR /app

# agent 二进制（main 找同目录 config.yaml；插件目录为同目录 plugins/）
COPY agent/build/monitor-agent .

# 直接工作进程模式（不带 -d）：容器内 PID 1 即工作进程，SIGTERM 直达优雅退出；
# 进程崩溃由 k8s restartPolicy 拉起，不需要守护模式
ENTRYPOINT ["/app/monitor-agent"]
