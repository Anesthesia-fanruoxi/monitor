#!/usr/bin/env bash
# agent-plugs/build.sh — 编译所有插件为 linux amd64 二进制，平铺到 dist/
#
# 产物布局（dist/）：
#   dist/<plugin>          # 每个插件一个二进制
#
# 用法：
#   ./build.sh             # 编译全部
#   ./build.sh hard nginx  # 指定插件
#
# 依赖：Go 1.23+
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PLUGINS_DIR="${SCRIPT_DIR}/plugins"
DIST_DIR="${SCRIPT_DIR}/dist"

if [[ $# -eq 0 ]]; then
    targets=()
    for d in "${PLUGINS_DIR}"/*/; do
        [[ -f "${d}manifest.yaml" ]] && targets+=("$(basename "${d}")")
    done
else
    targets=("$@")
fi

[[ ${#targets[@]} -eq 0 ]] && { echo "未找到任何插件（plugins/<name>/manifest.yaml 缺失）" >&2; exit 1; }

export CGO_ENABLED=0
export GOWORK=off   # 单模块构建，避免工作区模式下的模块图解析问题；多模块 IDE 解析交给 go.work
LDFLAGS="-s -w"
mkdir -p "${DIST_DIR}"

failures=0
for plugin in "${targets[@]}"; do
    plugin_dir="${PLUGINS_DIR}/${plugin}"
    [[ -f "${plugin_dir}/main.go" ]] || { echo "[${plugin}] 缺少 main.go" >&2; failures=$((failures + 1)); continue; }

    echo "==> [${plugin}] linux-amd64"
    ( cd "${plugin_dir}" && \
      GOOS=linux GOARCH=amd64 go build -ldflags="${LDFLAGS}" -o "${DIST_DIR}/${plugin}" . ) \
        || failures=$((failures + 1))
done

[[ ${failures} -gt 0 ]] && { echo "完成，${failures} 个插件编译失败"; exit 1; }
echo "完成，制品在 ${DIST_DIR}"
