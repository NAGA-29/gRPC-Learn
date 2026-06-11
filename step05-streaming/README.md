# Step 05: Streaming（サーバー/クライアント/双方向）

## 学習内容

- **サーバーストリーミング**: サーバーが1リクエストに対して複数レスポンスを返す
- **クライアントストリーミング**: クライアントが複数リクエストを送り、サーバーが最後に1レスポンスを返す
- **双方向ストリーミング**: 両者が独立して送受信できる
- **Go の実装パターン**: `context.Done()` の監視、`sync.Mutex` によるgoroutine-safe な実装
- **TypeScript での async iterator の使い方**

---

## ディレクトリ構成

```
step05-streaming/        # ※ .proto は リポジトリルートの proto/step05/ に配置
├── server-go/           # Go gRPC サーバー
│   ├── main.go
│   ├── go.mod
│   ├── handler/
│   │   └── stream.go   # StreamService 実装
│   └── gen/step05/     # buf generate で生成（.gitkeep のみコミット）
└── client-ts/           # TypeScript gRPC クライアント
    ├── src/
    │   └── client.ts
    ├── package.json
    ├── tsconfig.json
    └── gen/step05/     # buf generate で生成（.gitkeep のみコミット）
```

---

## 3種類のストリーミング

```
サーバーストリーミング（WatchStock）:
  Client ──── リクエスト(1) ────────────────────────────→ Server
  Client ←─── レスポンス(1) ──────────────────────────── Server
  Client ←─── レスポンス(2) ──────────────────────────── Server
  Client ←─── レスポンス(3) ──────────────────────────── Server
  Client ←─── レスポンス(...) ─────────────────────────── Server
  Client ←─── ストリーム終了 ──────────────────────────── Server

クライアントストリーミング（UploadFile）:
  Client ──── リクエスト(1) ────────────────────────────→ Server
  Client ──── リクエスト(2) ────────────────────────────→ Server
  Client ──── リクエスト(3) ────────────────────────────→ Server
  Client ──── ストリーム終了 ───────────────────────────→ Server
  Client ←─── レスポンス(1) ──────────────────────────── Server

双方向ストリーミング（Chat）:
  Client ──── メッセージ(A1) ───────────────────────────→ Server
  Client ←─── メッセージ(A1 ブロードキャスト) ────────── Server
  Client ──── メッセージ(A2) ───────────────────────────→ Server
  Client ←─── メッセージ(A2 ブロードキャスト) ────────── Server
  （送受信は独立して非同期に実行される）
```

---

## セットアップ手順

### 1. Proto ファイルの生成

```bash
# リポジトリルートで実行
bash scripts/gen.sh
```

### 2. Go サーバー

```bash
cd server-go
go mod tidy
go run .
```

### 3. TypeScript クライアント

```bash
cd client-ts
npm install
npm start
```

---

## 実行手順

1. ターミナル A でサーバーを起動する:
   ```bash
   cd step05-streaming/server-go && go run .
   ```

2. ターミナル B でクライアントを実行する:
   ```bash
   cd step05-streaming/client-ts && npm start
   ```

3. `grpcurl` での手動確認:
   ```bash
   # サーバーストリーミング（Ctrl+C で停止）
   grpcurl -plaintext -d '{"symbol": "GRPC"}' localhost:50052 step05.StreamService/WatchStock
   ```

---

## 各パターンのユースケース

| パターン | ユースケース例 |
|---|---|
| サーバーストリーミング | 株価・センサーデータのリアルタイム配信、ログのテール、進捗通知 |
| クライアントストリーミング | ファイルアップロード、バッチデータの一括送信、センサーデータの収集 |
| 双方向ストリーミング | チャット、オンラインゲーム、リアルタイムコラボレーション、AI ストリーミング応答 |

---

## Go の実装パターン

### context.Done() の監視（サーバーストリーミング）

サーバーストリーミングでは、クライアントが接続を切断した際に `context.Done()` チャネルにシグナルが届きます。
`select` 文でポーリングすることで、リソースのリークを防げます。

```go
func (s *StreamServer) WatchStock(req *pb.WatchStockRequest, stream pb.StreamService_WatchStockServer) error {
    for {
        // context がキャンセルされたら即座に終了
        select {
        case <-stream.Context().Done():
            return nil // 正常終了
        default:
        }

        // データを送信
        if err := stream.Send(&pb.StockPrice{...}); err != nil {
            return err // 送信エラー（クライアント切断など）
        }

        time.Sleep(500 * time.Millisecond)
    }
}
```

### クライアントストリーミングの受信ループ

`stream.Recv()` が `io.EOF` に相当するエラーを返したらストリーム終了です。
最後に `stream.SendAndClose()` でレスポンスを返します。

```go
func (s *StreamServer) UploadFile(stream pb.StreamService_UploadFileServer) error {
    var totalBytes int64
    for {
        chunk, err := stream.Recv()
        if err != nil {
            if isEOF(err) {
                break // 正常終了
            }
            return err // エラー終了
        }
        totalBytes += int64(len(chunk.Data))
    }
    // 最後にレスポンスを1つ送って終了
    return stream.SendAndClose(&pb.UploadFileSummary{TotalBytes: totalBytes})
}
```

### goroutine を使ったブロードキャスト（双方向ストリーミング）

複数クライアントへのブロードキャストは `sync.Mutex` でストリームのスライスを保護します。

```go
type StreamServer struct {
    mu    sync.Mutex
    rooms map[string][]pb.StreamService_ChatServer // ルーム → 参加者ストリーム
}

func (s *StreamServer) broadcast(room string, msg *pb.ChatMessage) {
    s.mu.Lock()
    streams := make([]pb.StreamService_ChatServer, len(s.rooms[room]))
    copy(streams, s.rooms[room]) // コピーしてからロック解放
    s.mu.Unlock()

    for _, st := range streams {
        st.Send(msg) // ロック外で送信（ブロッキングを避ける）
    }
}
```

---

## TypeScript での async iterator の使い方

`@connectrpc/connect` はストリーミングを async iterable / async generator として抽象化しています。

### サーバーストリーミング（受信）

```typescript
// for await...of で非同期イテレーションする
for await (const stockPrice of client.watchStock({ symbol: "GRPC" })) {
    console.log(stockPrice.price);
    // 途中で break すると gRPC キャンセルが送信される
    if (shouldStop) break;
}
```

### クライアントストリーミング（送信）

```typescript
// async generator でチャンクを生成して渡す
async function* generateChunks() {
    yield { fileName: "file.txt", data: new TextEncoder().encode("chunk 1") };
    yield { fileName: "file.txt", data: new TextEncoder().encode("chunk 2") };
}

// async iterable を渡すと、全チャンク送信後にレスポンスが返る
const summary = await client.uploadFile(generateChunks());
```

### 双方向ストリーミング（送受信）

```typescript
// 送信側: async generator
async function* sendMessages() {
    yield { room: "chat", sender: "Alice", text: "Hello!" };
    yield { room: "chat", sender: "Alice", text: "How are you?" };
}

// 受信側: for await...of
for await (const msg of client.chat(sendMessages())) {
    console.log(msg.text); // ブロードキャストされたメッセージを受信
}
```

---

## ストリーミングと Unary の使い分け基準

| 状況 | 推奨パターン |
|---|---|
| 1リクエスト → 1レスポンス | Unary |
| サーバーが継続的にデータを push したい | サーバーストリーミング |
| クライアントが大量データを送りたい（ファイルなど） | クライアントストリーミング |
| チャット・ゲームなどリアルタイム双方向通信 | 双方向ストリーミング |
| レスポンスが非常に大きい（ページネーションで十分） | まず Unary + ページネーション |
| レスポンスが大きくリアルタイム性が必要 | サーバーストリーミング |

### Unary を使い続けるべき理由

- シンプルで実装・デバッグしやすい
- HTTP キャッシュ、ロードバランサーとの相性が良い
- エラーハンドリングが明確
- ポーリング（定期的な Unary 呼び出し）で多くのユースケースは対応できる

### ストリーミングが必要になるサイン

- データサイズが大きくメモリに載らない
- リアルタイム性が求められる（100ms 以下の遅延）
- ロングポーリングで実装が複雑になっている
- 双方向の状態同期が必要
