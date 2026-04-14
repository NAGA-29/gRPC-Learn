# Step 06: Metadata / Deadline

gRPC のメタデータ（HTTP ヘッダー相当）とデッドライン（タイムアウト）を学ぶステップ。

## 概要

| 概念 | 説明 |
|------|------|
| **Metadata** | リクエスト/レスポンスに付随するキー・バリューペア（HTTP ヘッダー相当） |
| **Deadline** | クライアントが設定する「この時刻までに応答がなければキャンセル」の絶対時刻 |
| **Timeout** | デッドラインの相対時間表現（例: 2000ms 以内に完了せよ） |

---

## gRPC Metadata とは

HTTP/2 の世界では、gRPC メタデータは HTTP ヘッダーとして伝送される。  
`x-request-id` や `authorization` のようなカスタムキーを使って、認証情報・トレース ID・テナント情報などを伝達できる。

### Go でのメタデータ取得方法

```go
import "google.golang.org/grpc/metadata"

func (s *Server) Echo(ctx context.Context, req *pb.EchoRequest) (*pb.EchoResponse, error) {
    // 受信コンテキストからメタデータを取得する
    md, ok := metadata.FromIncomingContext(ctx)
    if ok {
        // md は map[string][]string 型
        values := md.Get("x-request-id") // []string
    }
    // ...
}
```

### TypeScript でのメタデータ送信方法（@connectrpc/connect）

```typescript
const res = await client.echo(
  { message: "Hello" },
  {
    // headers オプションで gRPC メタデータを設定する
    headers: {
      "x-request-id": "req-001",
      authorization: "Bearer token",
      "x-trace-id":  "trace-xyz",
    },
  }
);
```

---

## Deadline と Timeout の違い

```
Timeout  = 相対時間  例: 「2000ms 以内に応答せよ」
Deadline = 絶対時刻  例: 「2026-04-01T12:00:02.000Z までに応答せよ」
```

`timeoutMs` を指定すると、クライアントが現在時刻から計算した Deadline を  
gRPC ヘッダー（`grpc-timeout`）に乗せてサーバーに送信する。

---

## Context キャンセルのカスケード（client → server への伝播）

```
クライアント                         サーバー
  │                                    │
  ├─ timeoutMs=500 設定                │
  ├─ Echo(delay_ms=2000) ──────────── ► Echo() 実行開始
  │                                    ├─ time.Sleep(2000ms) 開始
  │  [500ms 経過]                      │
  ├─ DeadlineExceeded エラー受信       │
  │                                    ├─ ctx.Done() が閉じられる
                                       ├─ select で ctx.Done() 検出
                                       └─ 早期リターン（処理中断）
```

サーバー実装では `select` を使って context のキャンセルを検知する:

```go
select {
case <-time.After(delay):
    // 正常完了
case <-ctx.Done():
    // デッドライン超過 → 早期リターン
    return nil, ctx.Err()
}
```

---

## 実用例

| ヘッダーキー | 用途 |
|---|---|
| `authorization` | Bearer トークン / API キーによる認証 |
| `x-request-id` | リクエスト追跡 ID（ログ相関に使う） |
| `x-trace-id` | 分散トレーシング ID（Jaeger / Zipkin 等） |
| `x-tenant-id` | マルチテナント環境でのテナント識別 |
| `accept-language` | クライアントの言語設定 |

---

## ディレクトリ構成

```
step06-metadata-deadline/
├── server-go/          # Go gRPC サーバー
│   ├── main.go         # サーバーエントリポイント（:50051）
│   ├── handler/
│   │   └── metadata.go # MetadataService.Echo 実装
│   ├── gen/step06/     # buf generate で生成（.gitkeep のみ管理）
│   └── go.mod
├── client-ts/          # TypeScript gRPC クライアント
│   ├── src/
│   │   └── client.ts   # 4 パターンのデモ
│   ├── gen/step06/     # buf generate で生成（.gitkeep のみ管理）
│   ├── package.json
│   └── tsconfig.json
└── README.md
```

---

## 実行方法

### 1. Proto ファイルからコード生成

```bash
# リポジトリルートで実行
buf generate
```

### 2. Go サーバーを起動

```bash
cd step06-metadata-deadline/server-go
go mod download
go run .
# → Go gRPC サーバー起動 port=50051
```

### 3. TypeScript クライアントを実行

```bash
cd step06-metadata-deadline/client-ts
npm install
npm start
```

### 期待される出力例

```
=== Step 06: Metadata / Deadline デモ ===

--- 1. カスタムメタデータ付きリクエスト（遅延なし） ---
レスポンス受信:
  メッセージ: Hello with metadata!
  処理完了時刻: 2026-04-01T12:00:00Z
  サーバーが受け取ったメタデータ:
    x-request-id: req-001-abc
    authorization: Bearer my-secret-token
    x-trace-id: trace-xyz-789

--- 4. 2000ms 遅延 + 500ms デッドライン（タイムアウト期待） ---
タイムアウトエラー受信（期待通り）:
  code: 4
  message: [deadline_exceeded] ...
  → クライアントのデッドラインがサーバーに伝播し、処理が中断されました
```
