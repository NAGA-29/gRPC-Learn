# Step 02 — Unary 通信: Go サーバー + TypeScript クライアント

最初の実動作する gRPC サービス。Unary RPC（1リクエスト / 1レスポンス）の基本を学びます。

---

## 学習内容

- `buf generate` でコードを生成する流れ
- Go で gRPC サーバーを実装する
- TypeScript（@connectrpc/connect）で gRPC クライアントを実装する
- gRPC リフレクションを使って `grpcurl` でテストする

---

## ディレクトリ構成

```
step02-unary-go-ts/
├── server-go/
│   ├── go.mod
│   ├── main.go        ← GreeterService の実装
│   └── gen/step02/    ← buf generate で生成（初回のみ実行が必要）
└── client-ts/
    ├── package.json
    ├── tsconfig.json
    ├── src/client.ts  ← TypeScript クライアント
    └── gen/step02/    ← buf generate で生成（初回のみ実行が必要）
```

---

## セットアップ

### 1. コード生成（初回のみ）

```bash
# リポジトリルートで実行
buf generate
```

生成されるファイル:
- `server-go/gen/step02/greeter.pb.go` — protobuf メッセージ
- `server-go/gen/step02/greeter_grpc.pb.go` — gRPC サービス
- `client-ts/gen/step02/greeter_pb.ts` — TypeScript 型定義

### 2. Go の依存関係を解決

```bash
cd step02-unary-go-ts/server-go
go mod tidy
```

### 3. TypeScript の依存関係をインストール

```bash
cd step02-unary-go-ts/client-ts
npm install
```

---

## 実行

### Go サーバーを起動

```bash
cd step02-unary-go-ts/server-go
go run .
```

```
time=2024-01-01T00:00:00Z level=INFO msg="gRPC サーバー起動" port=50051
time=2024-01-01T00:00:00Z level=INFO msg="grpcurl でテスト: grpcurl -plaintext localhost:50051 list"
```

### TypeScript クライアントを実行（別ターミナル）

```bash
cd step02-unary-go-ts/client-ts
npm start
```

```
=== Step 02: Unary gRPC 通信デモ ===

1. 日本語でリクエスト送信...
   レスポンス: こんにちは、太郎 さん！
   サーバー時刻: 2024-01-01T00:00:00Z

2. 英語でリクエスト送信...
   レスポンス: Hello, Alice!
   サーバー時刻: 2024-01-01T00:00:01Z

3. スペイン語でリクエスト送信...
   レスポンス: ¡Hola, Carlos!
   サーバー時刻: 2024-01-01T00:00:02Z

=== 完了 ===
```

---

## grpcurl でテスト

```bash
# サービス一覧
grpcurl -plaintext localhost:50051 list

# GreeterService のメソッド一覧
grpcurl -plaintext localhost:50051 list step02.GreeterService

# SayHello を呼び出す
grpcurl -plaintext -d '{"name": "World", "language": "en"}' \
  localhost:50051 step02.GreeterService/SayHello
```

---

## コードの解説

### Go サーバー (`server-go/main.go`)

```go
// 1. proto で生成された interface を埋め込む
type greeterServer struct {
    pb.UnimplementedGreeterServiceServer
}

// 2. RPC メソッドを実装する
func (s *greeterServer) SayHello(ctx context.Context, req *pb.SayHelloRequest) (*pb.SayHelloResponse, error) {
    // req から入力を取得し、レスポンスを返す
    return &pb.SayHelloResponse{Message: "Hello, " + req.Name}, nil
}

// 3. サーバーを起動する
server := grpc.NewServer()
pb.RegisterGreeterServiceServer(server, &greeterServer{})
reflection.Register(server) // grpcurl 用
server.Serve(lis)
```

**`UnimplementedGreeterServiceServer` を埋め込む理由:**  
将来的に proto に新しい RPC が追加されても、コンパイルエラーにならないようにするため（前方互換性）。

### TypeScript クライアント (`client-ts/src/client.ts`)

```typescript
// 1. gRPC トランスポートを設定
const transport = createGrpcTransport({ baseUrl: "http://localhost:50051" });

// 2. 型安全なクライアントを生成
const client = createClient(GreeterService, transport);

// 3. 通常の async/await で呼び出す（gRPC の詳細を意識しない）
const res = await client.sayHello({ name: "World", language: "en" });
console.log(res.message);
```

**@connectrpc/connect を選ぶ理由:**
- buf が生成した TypeScript と完全に統合される
- `async/await` でシンプルに書ける
- ブラウザでも同じコードが動く（gRPC-Web 対応）

---

## REST との比較

| | REST (HTTP/1.1) | gRPC (HTTP/2) |
|--|----------------|--------------|
| プロトコル | JSON over HTTP | Protobuf over HTTP/2 |
| 型安全性 | なし（OpenAPI 等で補完） | proto による強い型安全 |
| コード生成 | 任意 | `buf generate` で自動 |
| ストリーミング | 限定的（SSE など） | ネイティブサポート |
| ペイロードサイズ | 大きい（テキスト） | 小さい（バイナリ） |

---

## 次のステップ

[Step 03](../step03-cross-language-go-py/) で Python クライアントから同様のサービスを呼び出してみましょう。
