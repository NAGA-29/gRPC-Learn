# Step 11 — 応用編: gRPC のテストとクライアント側の信頼性

Step 01〜10 で学んだ gRPC を「実務品質」に近づけるための応用編です。

- **bufconn** によるネットワーク不要の gRPC 単体テスト
- **テーブル駆動テスト** でのバリデーション検証
- **リトライポリシー（service config）** によるクライアント側の自動リトライ

---

## 学習内容

| トピック | 内容 |
|---------|------|
| bufconn | インメモリの `net.Listener` で gRPC サーバーをテストする |
| テーブル駆動テスト | Go の定番パターンで正常系/異常系を網羅する |
| `status.Code(err)` | テストで gRPC ステータスコードを検証する |
| リトライポリシー | service config JSON で `UNAVAILABLE` を自動リトライする |
| リトライの限界 | `maxAttempts` 超過・リトライ不能なコードの扱い |

---

## ディレクトリ構成

```
step11-advanced-testing/   # ※ .proto は リポジトリルートの proto/step11/ に配置
├── server-go/             # Go gRPC サーバー
│   ├── main.go
│   ├── go.mod
│   ├── handler/
│   │   ├── task.go        # TaskService 実装（FlakyEcho 含む）
│   │   └── task_test.go   # bufconn を使った単体テスト ★このステップの主役
│   └── gen/step11/        # bash scripts/gen.sh で生成
└── client-go/              # Go クライアント（リトライポリシーの実演）
    ├── main.go
    ├── go.mod
    └── gen/step11/        # bash scripts/gen.sh で生成
```

> このステップは Go のテスト・クライアント設定が主題のため、クライアントも Go で実装しています。

---

## セットアップと実行

### 1. コード生成（初回のみ・リポジトリルートで）

```bash
bash scripts/gen.sh
```

### 2. 単体テストの実行（サーバー起動不要）

```bash
cd step11-advanced-testing/server-go
go mod tidy
go test ./... -v
```

```
=== RUN   TestCreateTask
=== RUN   TestCreateTask/正常系:_タイトルあり
=== RUN   TestCreateTask/異常系:_タイトル空
--- PASS: TestCreateTask (0.00s)
=== RUN   TestGetTask
--- PASS: TestGetTask (0.00s)
=== RUN   TestFlakyEchoWithRetry
=== RUN   TestFlakyEchoWithRetry/リトライなし:_1回目の_Unavailable_で失敗する
=== RUN   TestFlakyEchoWithRetry/リトライあり:_自動リトライで最終的に成功する
--- PASS: TestFlakyEchoWithRetry (0.04s)
PASS
```

### 3. リトライデモの実行（サーバー + クライアント）

```bash
# ターミナル A
cd step11-advanced-testing/server-go && go run .

# ターミナル B
cd step11-advanced-testing/client-go && go mod tidy && go run .
```

```
--- 1. リトライなし（fail_count=2 → 即失敗） ---
  期待通りのエラー: code=Unavailable message=一時的に利用できません（1/2 回目の失敗）

--- 2. リトライあり（fail_count=2 → 3 回目で自動成功） ---
  成功: message="with-retry-..." attempts=3（2 回失敗 + 1 回成功）

--- 3. リトライ上限超え（fail_count=10 → maxAttempts=4 で失敗） ---
  期待通りのエラー: code=Unavailable message=一時的に利用できません（4/10 回目の失敗）
```

---

## bufconn とは

`google.golang.org/grpc/test/bufconn` は **インメモリの `net.Listener`** です。
TCP ポートを使わずに、本物の gRPC サーバー/クライアントのコードパスを通せます。

```
通常:     クライアント → TCP ソケット → サーバー
bufconn:  クライアント → インメモリパイプ → サーバー
```

### なぜハンドラーを直接呼ばないのか

ハンドラー関数を直接呼び出すテストも書けますが、bufconn を使うと以下も検証できます:

- protobuf の **シリアライズ / デシリアライズ**
- `status.Error` が **正しいステータスコード** としてクライアントに届くこと
- **インターセプター** を含めた完全なリクエストパス
- **リトライポリシー** などのクライアント設定（このステップの Test 参照）

### テストヘルパーの基本形

```go
func startBufconnServer(t *testing.T) pb.TaskServiceClient {
    lis := bufconn.Listen(1024 * 1024)

    srv := grpc.NewServer()
    pb.RegisterTaskServiceServer(srv, NewTaskServer(slog.Default()))
    go srv.Serve(lis)

    conn, _ := grpc.NewClient("passthrough:///bufnet",
        grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
            return lis.DialContext(ctx) // TCP の代わりにインメモリパイプへ
        }),
        grpc.WithTransportCredentials(insecure.NewCredentials()),
    )

    t.Cleanup(func() { conn.Close(); srv.Stop(); lis.Close() })
    return pb.NewTaskServiceClient(conn)
}
```

### ステータスコードの検証

```go
_, err := client.GetTask(ctx, &pb.GetTaskRequest{Id: "task-999"})
if got := status.Code(err); got != codes.NotFound {
    t.Fatalf("status code = %v, want %v", got, codes.NotFound)
}
```

---

## クライアント側リトライポリシー（service config）

gRPC のリトライは **接続時の設定として JSON で宣言** します。
アプリケーションコードに for ループを書く必要はありません。

```go
const retryServiceConfig = `{
  "methodConfig": [{
    "name": [{"service": "step11.TaskService"}],
    "retryPolicy": {
      "maxAttempts": 4,
      "initialBackoff": "0.1s",
      "maxBackoff": "1s",
      "backoffMultiplier": 2.0,
      "retryableStatusCodes": ["UNAVAILABLE"]
    }
  }]
}`

conn, _ := grpc.NewClient("localhost:50051",
    grpc.WithTransportCredentials(insecure.NewCredentials()),
    grpc.WithDefaultServiceConfig(retryServiceConfig),
)
```

| 設定 | 意味 |
|------|------|
| `maxAttempts` | 初回を含む最大試行回数（4 = 初回 + リトライ 3 回） |
| `initialBackoff` | 1 回目のリトライまでの待機時間 |
| `backoffMultiplier` | リトライごとに待機時間を何倍にするか（指数バックオフ） |
| `maxBackoff` | 待機時間の上限 |
| `retryableStatusCodes` | このコードのときだけリトライする |

### リトライ設計の注意点

1. **リトライしてよいのは安全なコードだけ**
   `UNAVAILABLE` は典型的なリトライ対象。`INVALID_ARGUMENT` や `NOT_FOUND` をリトライしても結果は変わらない（Step 07 参照）。

2. **冪等性（べきとうせい）を確認する**
   「注文を作成する」のような RPC をリトライすると二重実行のリスクがある。
   冪等キー（idempotency key）を渡すか、参照系のみリトライするのが安全。

3. **デッドラインと組み合わせる**
   リトライ中も全体のデッドライン（Step 06）は適用される。
   「リトライで粘りつつ、全体では 10 秒で諦める」のような設計にする。

4. **サーバー側の保護**
   全クライアントが一斉にリトライすると負荷が倍増する（リトライストーム）。
   指数バックオフ + ジッターと、サーバー側のレートリミットで保護する。

---

## さらに先へ進むには

このリポジトリでカバーしていない実務トピック:

| トピック | キーワード |
|---------|-----------|
| REST との共存 | grpc-gateway / Connect（gRPC + REST を 1 実装で提供） |
| スキーマ互換性の CI | `buf breaking` を GitHub Actions に組み込む |
| 相互 TLS（mTLS） | クライアント証明書による双方向認証 |
| 負荷試験 | ghz（gRPC 用ベンチマークツール） |
| ストリーミングのテスト | bufconn + goroutine で双方向ストリームを検証 |
