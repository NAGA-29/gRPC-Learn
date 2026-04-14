# gRPC Learning Lab

Go + TypeScript + Python で gRPC をステップ別に学ぶ実践的なリポジトリです。

## ゴール

- Go で gRPC サーバーを構築できる
- TypeScript (Node.js) · Python でクライアントを実装できる
- Unary / Streaming を理解・実装できる
- Metadata / Deadline / Error Handling を扱える
- Interceptor で共通処理を実装できる
- マイクロサービス構成で gRPC を利用できる

---

## カリキュラム

| ステップ | テーマ | 使用言語 |
|---------|--------|---------|
| [Step 01](./step01-protobuf-basics/) | Protobuf 基礎 — message / enum / nested / repeated | proto のみ |
| [Step 02](./step02-unary-go-ts/) | Unary 通信 — Go サーバー + TypeScript クライアント | Go / TS |
| [Step 03](./step03-cross-language-go-py/) | クロス言語通信 — Go サーバー + Python クライアント | Go / Python |
| [Step 04](./step04-unary-design/) | Unary 設計 — FieldMask / ページネーション / Money 型 | Go / TS |
| [Step 05](./step05-streaming/) | Streaming — サーバー / クライアント / 双方向 | Go / TS |
| [Step 06](./step06-metadata-deadline/) | Metadata / Deadline — ヘッダー相当・タイムアウト制御 | Go / TS |
| [Step 07](./step07-error-handling/) | Error Handling — gRPC ステータスコード設計 | Go / TS |
| [Step 08](./step08-interceptors/) | Interceptor — logging / auth / metrics | Go / TS |
| [Step 09](./step09-multi-service/) | マルチサービス — BFF → Service → Worker | Go / TS |
| [Step 10](./step10-production/) | 本番対応 — TLS / ヘルスチェック / Observability | Go / TS |

---

## 前提条件

| ツール | バージョン | 用途 |
|--------|----------|------|
| Go | 1.24+ | サーバー実装 |
| Node.js | 20+ | TypeScript クライアント |
| Python | 3.10+ | Python クライアント |
| [buf](https://buf.build/docs/installation) | 1.x | proto コード生成 |
| [grpcurl](https://github.com/fullstorydev/grpcurl) | 任意 | CLI テスト |

```bash
# buf のインストール（macOS）
brew install bufbuild/buf/buf

# buf のインストール（Linux）
curl -sSL https://github.com/bufbuild/buf/releases/latest/download/buf-Linux-x86_64 -o /usr/local/bin/buf
chmod +x /usr/local/bin/buf
```

---

## リポジトリ構成

```
grpc-learning-lab/
├── buf.yaml              # buf v2 ワークスペース設定
├── buf.gen.yaml          # コード生成設定
├── scripts/
│   └── gen-python.sh     # Python スタブ生成スクリプト
├── proto/                # 全 .proto ファイル（ステップ別）
│   ├── step01/
│   ├── step02/
│   └── ...
├── step01-protobuf-basics/
├── step02-unary-go-ts/
│   ├── server-go/        # Go gRPC サーバー
│   └── client-ts/        # TypeScript クライアント
└── ...
```

---

## コード生成について

**生成済みコードはリポジトリに含まれていません。** 各ステップを実行する前に、必ずコード生成を行ってください。

各 step の `gen/` ディレクトリ（例: `step02-unary-go-ts/server-go/gen/step02/`）は `.gitkeep` のみで空の状態です。
`buf generate` を実行することで Go スタブおよび TypeScript クライアントコードが生成されます。

```bash
# Go + TypeScript の生成（リポジトリルートで実行）
buf generate

# Python の生成（step03 のみ）
bash scripts/gen-python.sh
```

> **注意:** `buf generate` を実行しないまま `go run .` や `npm start` を行うと、
> `gen/` 配下のパッケージが存在しないため import エラーが発生します。

---

## クイックスタート

```bash
# 最初にコード生成を実行（リポジトリルートで）
buf generate

# Step 02: Go サーバー起動
cd step02-unary-go-ts/server-go
go run .

# Step 02: TypeScript クライアント実行（別ターミナル）
cd step02-unary-go-ts/client-ts
npm install
npm start

# Step 03: Python クライアント実行（Go サーバー起動後）
cd step03-cross-language-go-py/client-py
pip install -r requirements.txt
python client.py

# grpcurl でサービス一覧確認
grpcurl -plaintext localhost:50051 list
```

---

## 学習の進め方

1. 各ステップの `README.md` を先に読む
2. proto ファイルを確認して API 設計を理解する
3. Go サーバーのコードを読む
4. クライアントを実行して動作確認する
5. `grpcurl` でリクエストを手動送信して理解を深める
6. 自分でコードを改変して実験する
