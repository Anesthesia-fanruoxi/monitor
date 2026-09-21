#Requires -Version 7
<#
monitor-server 发布脚本（需 pwsh 7 运行）
流程：1) 交叉构建 linux/amd64  2) build 镜像（插件不进镜像，由宿主机 dist/ 挂载）
      3) push hub 仓库        4) 删除本地镜像

用法：
  ./release.ps1               # tag 自动用时间戳，如 20260920-1745
  ./release.ps1 -Tag 1.1      # 指定 tag

镜像双 tag：发布 tag（版本追溯）+ latest（服务器 pull 用）。
#>
param([string]$Tag = (Get-Date -Format "yyyyMMdd-HHmm"))

$ErrorActionPreference = "Stop"
$Image = "hub.hzbxhd.com/middleware/monitor-server"
$repoRoot = Split-Path $PSScriptRoot -Parent

Write-Host "==> [1/4] 交叉构建 linux/amd64 (tag=$Tag)"
$env:GOWORK = "off"   # 单模块构建，避免工作区模式的模块图解析问题
$env:GOOS = "linux"; $env:GOARCH = "amd64"; $env:CGO_ENABLED = "0"
try {
    Push-Location (Join-Path $repoRoot "server")
    go build -ldflags "-s -w" -o build/monitor-server .
    if ($LASTEXITCODE -ne 0) { throw "go build 失败" }
} finally {
    Pop-Location
    Remove-Item env:GOWORK, env:GOOS, env:GOARCH, env:CGO_ENABLED -ErrorAction SilentlyContinue
}

Write-Host "==> [2/4] build 镜像（仅服务端二进制 + projects.json）"
docker build -f (Join-Path $repoRoot "server/Dockerfile") -t "${Image}:${Tag}" -t "${Image}:latest" $repoRoot
if ($LASTEXITCODE -ne 0) { throw "docker build 失败" }

Write-Host "==> [3/4] push 到 hub"
docker push "${Image}:${Tag}"
if ($LASTEXITCODE -ne 0) { throw "push ${Tag} 失败" }
docker push "${Image}:latest"
if ($LASTEXITCODE -ne 0) { throw "push latest 失败" }

$digest = docker inspect --format '{{index .RepoDigests 0}}' "${Image}:latest"

Write-Host "==> [4/4] 清理本地镜像（本仓库全部 tag + 悬空层）"
$ids = docker images --filter "reference=${Image}" -q
if ($ids) { $ids | Select-Object -Unique | ForEach-Object { docker rmi -f $_ | Out-Null } }
docker image prune -f | Out-Null

Write-Host ""
Write-Host "发布完成: ${Image}:${Tag}"
Write-Host "digest  : $digest"
Write-Host "服务器更新:"
Write-Host "  docker pull ${Image}:latest"
Write-Host "  docker rm -f monitor-server; docker compose up -d"
