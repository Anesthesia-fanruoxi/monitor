# monitor-server 运行镜像（不编译，二进制由本地交叉编译产出）
#
# 前置产物（构建上下文 = 仓库根）：
#   server/build/monitor-server      GOOS=linux GOARCH=amd64 交叉编译的二进制
#
# 插件制品不进镜像：dist/ 由部署侧挂载（./dist:/app/dist），
# 插件更新 = 替换宿主机文件，无需重建容器/镜像。
#
# 构建与推送：server/release.ps1
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata \
    && cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime \
    && echo "Asia/Shanghai" > /etc/timezone

WORKDIR /app

# 服务端二进制（main 读取相对路径 config/config.yaml，WORKDIR 必须是 /app）
COPY server/build/monitor-server .

# config.yaml 不进镜像，由部署侧挂载 /app/config/config.yaml 提供

EXPOSE 8080

ENTRYPOINT ["/app/monitor-server"]
