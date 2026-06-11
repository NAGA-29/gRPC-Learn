# Step 03 — クロス言語通信: Go サーバー + Python クライアント

同じ `.proto` 定義から Go サーバーと Python クライアントを生成し、**言語をまたいだ通信**を体験します。

---

## 学習内容

- Python で gRPC クライアントを実装する（`grpcio`）
- 同期クライアント vs 非同期クライアント（`grpc.aio`）
- proto が保証する型安全性（Go ↔ Python 間でも型が一致）
- エラーハンドリングの基本（`grpc.RpcError`）

---

## セットアップ

### 1. Python スタブを生成

```bash
# リポジトリルートで実行
bash scripts/gen-python.sh
```

生成されるファイル:
- `client-py/gen/step03/greeter_pb2.py` — メッセージクラス
- `client-py/gen/step03/greeter_pb2_grpc.py` — gRPC スタブ

### 2. Go の依存関係を解決

```bash
cd step03-cross-language-go-py/server-go
go mod tidy
```

> **注意:** Go スタブは `bash scripts/gen.sh`（リポジトリルートで実行）で生成できます。

### 3. Python の依存関係をインストール

```bash
cd step03-cross-language-go-py/client-py
pip install -r requirements.txt
```

---

## 実行

### Go サーバーを起動

```bash
cd step03-cross-language-go-py/server-go
go run .
```

### Python クライアントを実行（別ターミナル）

```bash
cd step03-cross-language-go-py/client-py
python client.py
```

```
=== 同期クライアント ===
サーバーの返答: こんにちは、太郎 さん！（Go サーバーより）
サーバーホスト: my-macbook.local
サーバーの返答: こんにちは、Alice さん！（Go サーバーより）
サーバーホスト: my-macbook.local
...

=== 非同期クライアント ===
非同期レスポンス: こんにちは、非同期ユーザー さん！（Go サーバーより）
サーバーホスト: my-macbook.local

=== 完了 ===
```

---

## Python の gRPC コードパターン

### 同期クライアント（基本）

```python
import os
import sys

import grpc

# 生成コードは `from step03 import ...` という絶対インポートを行うため
# gen/ を sys.path に追加してからインポートする
sys.path.insert(0, os.path.join(os.path.dirname(os.path.abspath(__file__)), "gen"))
from step03 import greeter_pb2, greeter_pb2_grpc

with grpc.insecure_channel("localhost:50051") as channel:
    stub = greeter_pb2_grpc.GreeterServiceStub(channel)
    response = stub.SayHello(greeter_pb2.SayHelloRequest(name="World"))
    print(response.message)
```

### 非同期クライアント（async/await）

```python
import asyncio
import grpc.aio

async def main():
    async with grpc.aio.insecure_channel("localhost:50051") as channel:
        stub = greeter_pb2_grpc.GreeterServiceStub(channel)
        response = await stub.SayHello(greeter_pb2.SayHelloRequest(name="World"))
        print(response.message)

asyncio.run(main())
```

### エラーハンドリング

```python
try:
    response = stub.SayHello(request)
except grpc.RpcError as e:
    print(f"gRPC エラー:")
    print(f"  コード: {e.code()}")           # grpc.StatusCode.NOT_FOUND など
    print(f"  詳細: {e.details()}")          # エラーメッセージ
    print(f"  メタデータ: {e.trailing_metadata()}")
```

---

## Python vs TypeScript の比較

| | Python (`grpcio`) | TypeScript (`@connectrpc/connect`) |
|--|------------------|---------------------------------|
| スタブ生成 | `grpc_tools.protoc` | `buf generate` |
| インポート | `greeter_pb2`, `greeter_pb2_grpc` | `GreeterService` (1ファイル) |
| 同期呼び出し | ✅ デフォルト | ❌（常に async） |
| 非同期呼び出し | `grpc.aio` | ✅ デフォルト |
| 型チェック | mypy/pyright で補完 | TypeScript コンパイラで保証 |

---

## Python でコードを手動生成する方法

`buf` なしで生成する場合:

```bash
pip install grpcio-tools

# リポジトリルートで実行（scripts/gen-python.sh と同じレイアウトで生成される）
python3 -m grpc_tools.protoc \
  --proto_path=proto \
  --python_out=step03-cross-language-go-py/client-py/gen \
  --grpc_python_out=step03-cross-language-go-py/client-py/gen \
  step03/greeter.proto
```

---

## 次のステップ

[Step 04](../step04-unary-design/) で、より実践的な Unary API 設計（FieldMask・ページネーション・Money 型）を学びましょう。
