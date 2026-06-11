#!/usr/bin/env bash
# 全ステップの Go / TypeScript スタブを一括生成するスクリプト
#
# 使い方（リポジトリルートで実行）:
#   bash scripts/gen.sh
#
# 仕組み:
#   - Go      → buf.gen.go.yaml を使い、各 step の server-go/gen/ などに生成する
#   - TS      → buf.gen.ts.yaml を使い、各 step の client-ts/gen/ に生成する
#   - Python  → scripts/gen-python.sh を別途実行する（step03 のみ）
#
# buf の `-o` フラグで出力先ディレクトリを、`--path` で対象 proto を指定している。

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${REPO_ROOT}"

# buf generate のラッパー
#   gen_go <stepXX> <出力先ディレクトリ>
gen_go() {
  echo "--> Go: proto/$1 -> $2/gen/$1"
  buf generate proto --template buf.gen.go.yaml --path "proto/$1" -o "$2"
}

#   gen_ts <stepXX> <出力先ディレクトリ>
gen_ts() {
  echo "--> TS: proto/$1 -> $2/gen/$1"
  buf generate proto --template buf.gen.ts.yaml --path "proto/$1" -o "$2"
}

echo "==> Go / TypeScript スタブを生成します..."

# step02
gen_go step02 step02-unary-go-ts/server-go
gen_ts step02 step02-unary-go-ts/client-ts

# step03（Python スタブは scripts/gen-python.sh で生成）
gen_go step03 step03-cross-language-go-py/server-go

# step04
gen_go step04 step04-unary-design/server-go
gen_ts step04 step04-unary-design/client-ts

# step05
gen_go step05 step05-streaming/server-go
gen_ts step05 step05-streaming/client-ts

# step06
gen_go step06 step06-metadata-deadline/server-go
gen_ts step06 step06-metadata-deadline/client-ts

# step07
gen_go step07 step07-error-handling/server-go
gen_ts step07 step07-error-handling/client-ts

# step08
gen_go step08 step08-interceptors/server-go
gen_ts step08 step08-interceptors/client-ts

# step09（Go サービスが 3 つある）
gen_go step09 step09-multi-service/gateway
gen_go step09 step09-multi-service/inventory-svc
gen_go step09 step09-multi-service/notification-worker
gen_ts step09 step09-multi-service/client-ts

# step10
gen_go step10 step10-production/server-go
gen_ts step10 step10-production/client-ts

# step11（応用編: テスト）
gen_go step11 step11-advanced-testing/server-go

echo "==> 生成完了"
echo "    Python スタブ（step03）は別途 bash scripts/gen-python.sh を実行してください"
