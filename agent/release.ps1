#Requires -Version 7
<#
monitor-agent 发布脚本（需 pwsh 7 运行）
流程：1) 交叉构建 linux/amd64  2) build 镜像  3) push hub 仓库  4) 清理本地旧版本

用法：
  ./release.ps1                # tag 自动用时间戳，如 20260921-1500
  ./release.ps1 -Tag 1.3       # 指定 tag

与 server 的差异：本地只保留本次发布镜像（1.3 + latest），旧版本自动删除，
方便本地联调时不囤积镜像。
#>
param([string]$Tag = (Get-Date -Format "yyyyMMdd-HHmm"))

$ErrorActionPreference = "Stop"
$Image = "hub.hzbxhd.com/monitoring/agent"
$repoRoot = Split-Path $PSScriptRoot -Parent

Write-Host "==> [1/4] 交叉构建 linux/amd64 (tag=$Tag)"
$env:GOWORK = "off"   # 单模块构建，避免工作区模式的模块图解析问题
$env:GOOS = "linux"; $env:GOARCH = "amd64"; $env:CGO_ENABLED = "0"
try {
    Push-Location (Join-Path $repoRoot "agent")
    New-Item -ItemType Directory -Force build | Out-Null   # go build -o 不会自建父目录
    go build -ldflags "-s -w" -o build/monitor-agent .
    if ($LASTEXITCODE -ne 0) { throw "go build 失败" }
} finally {
    Pop-Location
    Remove-Item env:GOWORK, env:GOOS, env:GOARCH, env:CGO_ENABLED -ErrorAction SilentlyContinue
}

Write-Host "==> [2/4] build 镜像（仅 agent 二进制，插件运行时自动下载）"
docker build -f (Join-Path $repoRoot "agent/Dockerfile") -t "${Image}:${Tag}" -t "${Image}:latest" $repoRoot
if ($LASTEXITCODE -ne 0) { throw "docker build 失败" }

Write-Host "==> [3/4] push 到 hub"
docker push "${Image}:${Tag}"
if ($LASTEXITCODE -ne 0) { throw "push ${Tag} 失败" }
docker push "${Image}:latest"
if ($LASTEXITCODE -ne 0) { throw "push latest 失败" }

$digest = docker inspect --format '{{index .RepoDigests 0}}' "${Image}:latest"

Write-Host "==> [4/4] 清理本地旧版本（只保留 ${Tag} + latest）"
docker images "${Image}" --format "{{.Tag}}" | Where-Object { $_ -notin @($Tag, "latest") } | ForEach-Object {
    docker rmi -f "${Image}:$_" | Out-Null
}
docker image prune -f | Out-Null

Write-Host ""
Write-Host "发布完成: ${Image}:${Tag}"
Write-Host "digest  : $digest"
Write-Host "集群更新:"
Write-Host "  kubectl -n monitor rollout restart daemonset/monitor-agent-node"
