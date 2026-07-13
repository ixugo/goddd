#!/bin/bash
# github.com/ixugo/goddd
# 建议 token 作为环境变量，APIFOX_PROJECT_ID 可写到项目目录下的 CLAUDE.md/AGENTS.md 文件中

# apifox.sh - 上传 OpenAPI 文件到 Apifox 并解析导入结果
# 单文件: ./apifox.sh projectid=1111 token=2222 file=./docs/api/banner.go.yaml
# 目录:   ./apifox.sh projectid=1111 token=2222 dir=./docs/api
# https://apifox-openapi.apifox.cn/api-173409873

set -euo pipefail

# 解析命令行参数
for arg in "$@"; do
    case $arg in
        projectid=*)
            APIFOX_PROJECTID="${arg#*=}"
            ;;
        token=*)
            APIFOX_ACCESS_TOKEN="${arg#*=}"
            ;;
        file=*)
            FILE_PATH="${arg#*=}"
            ;;
        dir=*)
            DIR_PATH="${arg#*=}"
            ;;
        *)
            echo "未知参数: $arg" >&2
            exit 1
            ;;
    esac
done

# 检查必要参数
if [[ -z "${APIFOX_PROJECTID:-}" ]]; then
    echo "错误: 未提供 projectid" >&2
    exit 1
fi

if [[ -z "${APIFOX_ACCESS_TOKEN:-}" ]]; then
    echo "错误: 未提供 token" >&2
    exit 1
fi

if [[ -z "${FILE_PATH:-}" ]] && [[ -z "${DIR_PATH:-}" ]]; then
    echo "错误: 未提供 file 或 dir" >&2
    exit 1
fi

# 上传单个文件，打印统计结果；遇到失败接口时返回非零值
upload_file() {
    local file_path="$1"

    if [[ ! -f "$file_path" ]]; then
        echo "错误: 文件不存在: $file_path" >&2
        return 1
    fi

    echo "正在上传: $file_path"

    INPUT_CONTENT=$(python3 -c "
import sys, json
content = open(sys.argv[1], 'r', encoding='utf-8').read()
print(json.dumps(content))
" "$file_path")

    REQUEST_BODY=$(cat <<EOF
{
  "input": $INPUT_CONTENT,
  "options": {
    "targetEndpointFolderId": 0,
    "targetSchemaFolderId": 0,
    "endpointOverwriteBehavior": "OVERWRITE_EXISTING",
    "schemaOverwriteBehavior": "OVERWRITE_EXISTING",
    "updateFolderOfChangedEndpoint": false,
    "prependBasePath": false
  }
}
EOF
)

    RESPONSE=$(curl --silent --location -g \
      --request POST "https://api.apifox.com/v1/projects/${APIFOX_PROJECTID}/import-openapi?locale=zh-CN" \
      --header 'X-Apifox-Api-Version: 2024-03-28' \
      --header "Authorization: Bearer ${APIFOX_ACCESS_TOKEN}" \
      --header 'Content-Type: application/json' \
      --data-raw "$REQUEST_BODY")

    CLEAN_RESPONSE=$(echo "$RESPONSE" | sed 's/}%.*$/}/' | python3 -c "
import sys, json
try:
    data = json.load(sys.stdin)
    counters = data.get('data', {}).get('counters', {})
    print(json.dumps(counters))
except Exception as e:
    print('{}')
")

    if [[ "$CLEAN_RESPONSE" == "{}" ]]; then
        echo "错误: 无法解析 Apifox 响应。原始响应如下：" >&2
        echo "$RESPONSE" >&2
        return 1
    fi

    extract_field() {
        local field="$1"
        if command -v jq >/dev/null 2>&1; then
            echo "$CLEAN_RESPONSE" | jq -r ".$field // 0"
        else
            python3 -c "import sys, json; d=json.loads(sys.argv[1]); print(d.get('$field', 0))" "$CLEAN_RESPONSE"
        fi
    }

    endpointCreated=$(extract_field "endpointCreated")
    endpointUpdated=$(extract_field "endpointUpdated")
    endpointFailed=$(extract_field "endpointFailed")
    schemaCreated=$(extract_field "schemaCreated")
    schemaUpdated=$(extract_field "schemaUpdated")
    schemaFailed=$(extract_field "schemaFailed")

    echo "✅ 导入完成！"
    echo "  新增接口数 (endpointCreated): $endpointCreated"
    echo "  更新接口数 (endpointUpdated): $endpointUpdated"
    echo "  失败接口数 (endpointFailed): $endpointFailed"
    echo "  新增模型数 (schemaCreated): $schemaCreated"
    echo "  更新模型数 (schemaUpdated): $schemaUpdated"
    echo "  失败模型数 (schemaFailed): $schemaFailed"

    if [[ $endpointFailed -gt 0 ]]; then
        return 1
    fi
}

# 将相对路径转为绝对路径（以调用时的 cwd 为基准）
abspath() {
    python3 -c "import os, sys; print(os.path.abspath(sys.argv[1]))" "$1"
}

# 单文件模式
if [[ -n "${FILE_PATH:-}" ]]; then
    FILE_PATH=$(abspath "$FILE_PATH")
    upload_file "$FILE_PATH"
    exit $?
fi

# 目录模式：遍历目录下（非递归）所有 *.go.yaml 文件
DIR_PATH=$(abspath "$DIR_PATH")
if [[ ! -d "$DIR_PATH" ]]; then
    echo "错误: 目录不存在: $DIR_PATH" >&2
    exit 1
fi

shopt -s nullglob
FILES=("$DIR_PATH"/*.go.yaml)
shopt -u nullglob

if [[ ${#FILES[@]} -eq 0 ]]; then
    echo "错误: 目录 $DIR_PATH 下未找到 *.go.yaml 文件" >&2
    exit 1
fi

FAILED=0
for f in "${FILES[@]}"; do
    upload_file "$f" || FAILED=$((FAILED + 1))
    echo ""
done

if [[ $FAILED -gt 0 ]]; then
    echo "共 $FAILED 个文件上传失败" >&2
    exit 1
fi
