#!/usr/bin/env bash
# Python gRPC スタブを生成するスクリプト
# 使用前に grpcio-tools をインストールしてください:
#   pip install grpcio-tools==1.71.0
#
# 実行方法:
#   bash scripts/gen-python.sh

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

echo "==> Python gRPC スタブを生成します..."

# step03 の生成
echo "--> step03"
mkdir -p "${REPO_ROOT}/step03-cross-language-go-py/client-py/gen/step03"
touch "${REPO_ROOT}/step03-cross-language-go-py/client-py/gen/__init__.py"
touch "${REPO_ROOT}/step03-cross-language-go-py/client-py/gen/step03/__init__.py"

python3 -m grpc_tools.protoc \
  --proto_path="${REPO_ROOT}/proto/step03" \
  --python_out="${REPO_ROOT}/step03-cross-language-go-py/client-py/gen/step03" \
  --grpc_python_out="${REPO_ROOT}/step03-cross-language-go-py/client-py/gen/step03" \
  "${REPO_ROOT}/proto/step03/greeter.proto"

echo "==> 生成完了"
