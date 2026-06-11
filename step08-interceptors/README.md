# Step 08: gRPC インターセプター（Interceptor）

## gRPC インターセプターとは

インターセプターは gRPC における **ミドルウェアパターン** の実装です。  
RPC ハンドラーの前後に処理を差し込むことで、横断的関心事（Cross-Cutting Concerns）を  
ハンドラーのビジネスロジックから分離できます。

```
クライアント
    ↓ リクエスト
[インターセプター 1: logging]   ← 開始ログを出力
    ↓
[インターセプター 2: auth]      ← トークンを検証（失敗すれば Unauthenticated を返す）
    ↓
[インターセプター 3: metrics]   ← カウンターをインクリメント
    ↓
[ハンドラー]                    ← ビジネスロジック（レスポンスを返す）
    ↑ レスポンス
[インターセプター 3: metrics]   ← エラー数を更新
    ↑
[インターセプター 2: auth]      ← スキップ（後処理なし）
    ↑
[インターセプター 1: logging]   ← 終了ログを出力（duration_ms, code）
    ↑
クライアント
```

## このステップで実装する 3 種類のインターセプター

### 1. ロギングインターセプター (`interceptor/logging.go`)

- すべての Unary RPC に対して開始・終了ログを記録する
- `go.uber.org/zap` を使った構造化ログ（step02〜07 の `log/slog` からのアップグレード例）
- ログに含まれる情報: `method`（メソッド名）、`duration_ms`（処理時間）、`code`（gRPC ステータス）

```go
// zap.NewDevelopment() は開発時の読みやすいカラー付き出力
// 本番では zap.NewProduction()（JSON 形式）を使う
logger, _ := zap.NewDevelopment()
loggingInterceptor := interceptor.NewLoggingInterceptor(logger)
```

### 2. 認証インターセプター (`interceptor/auth.go`)

- **選択的認証**: `GetSecret` メソッドのみトークンを検証し、他はスキップする
- gRPC メタデータの `authorization` キーから `Bearer secret-token-12345` を検証する
- 検証失敗時は `codes.Unauthenticated` を返し、ハンドラーには到達させない

```go
// protectedMethods マップで保護対象メソッドを管理する
var protectedMethods = map[string]struct{}{
    "/step08.InterceptorService/GetSecret": {},
}
```

### 3. メトリクスインターセプター (`interceptor/metrics.go`)

- `sync.Mutex` + `map` でリクエスト数・エラー数をメソッド単位で集計する（学習用シンプル実装）
- `GetMetrics()` でカウンターのスナップショットを取得できる
- 本番では [go-grpc-middleware の Prometheus インテグレーション](https://github.com/grpc-ecosystem/go-grpc-middleware) を使う

```go
metricsInterceptor := interceptor.NewMetricsInterceptor()
// メトリクスの取得例
snapshot := metricsInterceptor.GetMetrics()
```

## インターセプターのチェーン順序の重要性

`grpc.ChainUnaryInterceptor` は **引数の順に外から内へ** ラップします。

```go
server := grpc.NewServer(
    grpc.ChainUnaryInterceptor(
        loggingInterceptor,       // 1番目（最外）: 必ずログを記録したい
        authInterceptor,          // 2番目: ログの後・ハンドラーの前に認証
        metricsInterceptor.Unary(), // 3番目（最内）: ハンドラー直前でカウント
    ),
)
```

**順序の設計判断:**

| 順序 | インターセプター | 理由 |
|------|----------------|------|
| 1st  | logging        | エラーを含む **全リクエスト** を記録するため先頭に配置 |
| 2nd  | auth           | 不正リクエストをハンドラーに **届ける前に弾く** ため |
| 3rd  | metrics        | ハンドラーの結果（成功/失敗）を正確にカウントするため |

## Go の実装パターン

`grpc.UnaryServerInterceptor` の関数シグネチャ:

```go
type UnaryServerInterceptor func(
    ctx     context.Context,    // リクエストのコンテキスト
    req     any,                // リクエストメッセージ
    info    *grpc.UnaryServerInfo, // メソッド名などのサーバー情報
    handler grpc.UnaryHandler,  // 次のインターセプターまたはハンドラー
) (any, error)
```

インターセプターの基本構造:

```go
func NewMyInterceptor() grpc.UnaryServerInterceptor {
    return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
        // --- 前処理 ---

        resp, err := handler(ctx, req) // 次のインターセプター or ハンドラーへ委譲

        // --- 後処理 ---

        return resp, err
    }
}
```

## TypeScript クライアント側でのヘッダー設定方法

`@connectrpc/connect` では `headers` オプションで gRPC メタデータ（HTTP ヘッダー）を設定します。

```typescript
// 正しいトークンで GetSecret を呼び出す
const res = await client.getSecret(
  {},
  {
    headers: {
      authorization: "Bearer secret-token-12345", // サーバーの auth インターセプターが検証する
    },
  }
);
```

## 動作確認手順

```bash
# 1. プロトファイルからコードを生成
bash scripts/gen.sh   # リポジトリルートで実行

# 2. Go サーバーを起動
cd step08-interceptors/server-go
go run .

# 3. TypeScript クライアントを実行（別ターミナル）
cd step08-interceptors/client-ts
npm install
npm start
```

**期待される実行結果:**

```
=== Step 08: インターセプター動作確認デモ ===

--- 1. Ping（認証不要） ---
成功:
  メッセージ: pong: Hello Interceptor!
  サーバーID: server-<hostname>

--- 2. GetSecret（認証なし → エラー期待） ---
期待通りのエラー [Unauthenticated]: authorization ヘッダーが必要です

--- 3. GetSecret（正しいトークン → 成功期待） ---
成功:
  機密データ: 機密情報: TOP_SECRET_DATA_XYZ_2024

--- 4. GetSecret（間違ったトークン → エラー期待） ---
期待通りのエラー [Unauthenticated]: 無効なトークンです: "Bearer wrong-token-99999"
```

## 本番での活用例

[grpc-ecosystem/go-grpc-middleware](https://github.com/grpc-ecosystem/go-grpc-middleware) が提供するインターセプター:

| パッケージ | 用途 |
|-----------|------|
| `middleware/logging` | 構造化ロギング（zap / logrus 対応） |
| `middleware/auth` | JWT / OAuth2 検証 |
| `middleware/ratelimit` | レートリミット |
| `middleware/recovery` | パニックリカバリー |
| `middleware/retry` | クライアント側リトライ |

```go
// go-grpc-middleware v2 の使用例
import (
    grpcmiddleware "github.com/grpc-ecosystem/go-grpc-middleware/v2"
    grpclogging "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
)
```

## ファイル構成

```
step08-interceptors/
├── server-go/
│   ├── main.go                  # サーバーエントリーポイント（インターセプターチェーン設定）
│   ├── go.mod
│   ├── handler/
│   │   └── service.go           # InterceptorService 実装
│   ├── interceptor/
│   │   ├── logging.go           # ロギングインターセプター
│   │   ├── auth.go              # 認証インターセプター（GetSecret のみ）
│   │   └── metrics.go           # メトリクスインターセプター
│   └── gen/step08/              # buf generate で生成（.gitkeep のみコミット）
└── client-ts/
    ├── package.json
    ├── tsconfig.json
    ├── src/
    │   └── client.ts            # 4パターンの呼び出しを実演
    └── gen/step08/              # buf generate で生成（.gitkeep のみコミット）
```
