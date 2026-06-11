# Step 10: 本番対応 (TLS / ヘルスチェック / Observability)

gRPC サーバーを本番環境で運用するために必要な要素を実装するステップです。
TLS による通信の暗号化、標準ヘルスチェックプロトコル、Prometheus メトリクス、
OpenTelemetry トレーシング、グレースフルシャットダウンを学びます。

## ディレクトリ構成

```
step10-production/
├── server-go/
│   ├── main.go                    # サーバーエントリーポイント
│   ├── go.mod
│   ├── health/
│   │   └── checker.go             # ヘルスチェック実装
│   ├── observability/
│   │   ├── metrics.go             # Prometheus メトリクス
│   │   └── tracing.go             # OpenTelemetry トレーシング
│   └── gen/step10/                # buf generate で生成
├── certs/
│   ├── generate.sh                # 自己署名証明書生成スクリプト
│   ├── server.crt                 # (生成後) TLS 証明書
│   └── server.key                 # (生成後) TLS 秘密鍵
└── client-ts/
    ├── src/client.ts
    ├── package.json
    ├── tsconfig.json
    └── gen/step10/                # buf generate で生成
```

## TLS の設定方法

### 1. 自己署名証明書の生成

```bash
cd step10-production/certs
bash generate.sh
# 生成されるファイル:
#   server.crt  (証明書)
#   server.key  (秘密鍵)
```

### 2. サーバー側の TLS 設定

`TLS_ENABLED` 環境変数で TLS を有効化します。
証明書ファイルは `../certs/server.crt` と `../certs/server.key`（`server-go/` から見た相対パス）から読み込みます。

```bash
# TLS なし（insecure）で起動
cd step10-production/server-go
go run .

# TLS ありで起動
TLS_ENABLED=true go run .
```

サーバーコードでの TLS 設定:

```go
cert, err := tls.LoadX509KeyPair("../certs/server.crt", "../certs/server.key")
tlsConfig := &tls.Config{
    Certificates: []tls.Certificate{cert},
    MinVersion:   tls.VersionTLS12, // TLS 1.2 以上を要求
}
grpc.NewServer(grpc.Creds(credentials.NewTLS(tlsConfig)))
```

### 3. クライアント側の TLS 設定

自己署名証明書を使う場合、クライアントは証明書を信頼するよう設定が必要です。

```go
// Go クライアント（自己署名証明書を信頼する場合）
creds, err := credentials.NewClientTLSFromFile("../certs/server.crt", "localhost")
conn, err := grpc.NewClient("localhost:50051", grpc.WithTransportCredentials(creds))
```

TypeScript クライアント（@connectrpc/connect-node）では:

```typescript
// CA 証明書を指定する場合（Node.js）
import * as fs from "fs";
const transport = createGrpcTransport({
  baseUrl: "https://localhost:50051",
  nodeOptions: {
    ca: fs.readFileSync("../certs/server.crt"),
  },
});
```

## gRPC ヘルスチェックプロトコル

> **注意**: このステップの `ProductionService.Check` RPC は学習用の **独自実装** です。
> `grpc.health.v1.Health` サービス（標準プロトコル）とは互換性がないため、
> `grpc_health_probe` コマンドや Kubernetes の `grpc` probe はそのままでは動作しません。
> 標準互換が必要な場合は `google.golang.org/grpc/health` パッケージを使い、
> `grpc.health.v1.Health` サービスをサーバーに別途登録してください。

### grpc.health.v1 標準プロトコルについて

gRPC の標準ヘルスチェックプロトコルは [GRPC Health Checking Protocol](https://github.com/grpc/grpc/blob/master/doc/health-checking.md) として定められています。

```
service Health {
  rpc Check(HealthCheckRequest) returns (HealthCheckResponse);
  rpc Watch(HealthCheckRequest) returns (stream HealthCheckResponse);
}
```

**ServingStatus**:
| 値 | 意味 |
|----|------|
| `UNKNOWN` (0) | ステータス不明（起動直後など） |
| `SERVING` (1) | 正常稼働中、リクエストを受け付けられる |
| `NOT_SERVING` (2) | 一時的にサービス不能（デプロイ中、過負荷など） |

### 標準互換にする場合の実装例

```go
import "google.golang.org/grpc/health"
import healthpb "google.golang.org/grpc/health/grpc_health_v1"

healthServer := health.NewServer()
healthpb.RegisterHealthServer(srv, healthServer)
healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
```

### HTTP ヘルスチェック（このステップの実装）

このステップでは HTTP ヘルスエンドポイントも用意しています（Kubernetes の livenessProbe / readinessProbe に対応）:

```bash
# HTTP ヘルスチェック（標準対応）
curl http://localhost:8080/healthz
# 出力: ok
```

```yaml
# Kubernetes での HTTP ヘルスチェック設定例
livenessProbe:
  httpGet:
    path: /healthz
    port: 8080
  initialDelaySeconds: 10
  periodSeconds: 30

readinessProbe:
  httpGet:
    path: /healthz
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 10
```

## Prometheus メトリクスの確認

### メトリクスエンドポイント

サーバー起動後、以下の URL でメトリクスを確認できます:

```bash
curl http://localhost:8080/metrics | grep grpc_

# 出力例:
# HELP grpc_requests_total gRPC リクエストの累積数
# TYPE grpc_requests_total counter
grpc_requests_total{method="Check"} 3
grpc_requests_total{method="GetMetrics"} 1
# HELP grpc_active_connections 現在のアクティブな gRPC 接続数
# TYPE grpc_active_connections gauge
grpc_active_connections 0
```

### Prometheus 設定例

`prometheus.yml` にスクレイプ設定を追加:

```yaml
scrape_configs:
  - job_name: 'grpc-step10'
    static_configs:
      - targets: ['localhost:8080']
    scrape_interval: 15s
```

### Grafana ダッシュボード

PromQL クエリ例:

```promql
# 5 分間の RPS（リクエスト毎秒）
rate(grpc_requests_total[5m])

# メソッドごとのリクエスト数合計
sum by (method)(grpc_requests_total)

# アクティブ接続数
grpc_active_connections
```

## OpenTelemetry トレーシング

### セットアップ概要

```go
// 初期化
cleanup := observability.InitTracer()
defer cleanup() // シャットダウン時にスパンをフラッシュ

// スパンの作成
tracer := otel.Tracer("my-service")
ctx, span := tracer.Start(ctx, "operation-name")
defer span.End()
```

### 学習用: stdout エクスポーター

このステップでは stdout エクスポーターを使います。
サーバーのコンソールに JSON 形式のトレースデータが出力されます。

```json
{
  "Name": "Check",
  "SpanContext": {
    "TraceID": "...",
    "SpanID": "..."
  },
  "StartTime": "...",
  "EndTime": "...",
  "Attributes": [...]
}
```

### 本番: Jaeger エクスポーター

```go
import "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"

exporter, _ := otlptracehttp.New(ctx,
    otlptracehttp.WithEndpoint("jaeger:4318"),
)
```

## グレースフルシャットダウンの重要性

### なぜグレースフルシャットダウンが必要か

サーバーが突然終了すると、進行中のリクエストが中断され、
クライアントはエラーを受け取ります。
Kubernetes の Pod 再起動やデプロイ時に特に重要です。

### 実装パターン

```go
// シグナルハンドリング
quit := make(chan os.Signal, 1)
signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

// バックグラウンドでサーバー起動
go srv.Serve(lis)

// シグナルを待つ
<-quit

// グレースフルシャットダウン
// - 新規リクエストの受付を停止
// - 進行中のリクエストが完了するまで待つ
srv.GracefulStop()
```

### タイムアウト付きグレースフルシャットダウン

長時間実行される RPC がある場合、タイムアウトを設けることを推奨:

```go
done := make(chan struct{})
go func() {
    srv.GracefulStop()
    close(done)
}()

select {
case <-done:
    logger.Info("グレースフルシャットダウン完了")
case <-time.After(30 * time.Second):
    // タイムアウト: 強制終了
    srv.Stop()
    logger.Warn("強制シャットダウン（タイムアウト）")
}
```

## 本番デプロイ時のチェックリスト

### セキュリティ

- [ ] TLS を有効化している（`TLS_ENABLED=true`）
- [ ] 有効期限付きの証明書を使用している（Let's Encrypt または商用 CA）
- [ ] 証明書の自動更新を設定している（cert-manager など）
- [ ] TLS 1.2 以上を最小バージョンに設定している
- [ ] 秘密鍵をコードリポジトリに含めていない（`.gitignore` に追加済み）
- [ ] 認証/認可インターセプターを設定している（Step 08 参照）

### 観測可能性

- [ ] 構造化ログ（JSON 形式）を出力している
- [ ] Prometheus メトリクスエンドポイント（`/metrics`）を公開している
- [ ] ヘルスチェックエンドポイント（gRPC + HTTP）を実装している
- [ ] 分散トレーシングを設定している（Jaeger, Zipkin, または OTLP）
- [ ] エラーレートのアラートを設定している

### 運用

- [ ] グレースフルシャットダウンを実装している
- [ ] gRPC リフレクションを本番で無効化している（セキュリティ上の理由）
- [ ] 適切なタイムアウトとデッドラインを設定している（Step 06 参照）
- [ ] リソースリーク（goroutine、接続）がないことを確認している
- [ ] Kubernetes の livenessProbe / readinessProbe を設定している

### パフォーマンス

- [ ] 接続プールを適切に設定している
- [ ] Keep-Alive 設定を調整している
- [ ] メッセージサイズの上限を設定している（`grpc.MaxRecvMsgSize`）
- [ ] 負荷テストを実施している

## 起動方法

```bash
# 0. コード生成（初回のみ・リポジトリルートで実行）
bash scripts/gen.sh

# 1. 証明書の生成（初回のみ）
cd step10-production/certs && bash generate.sh

# 2. サーバーの起動（TLS なし）
cd step10-production/server-go && go run .

# 3. TypeScript クライアントの起動
cd step10-production/client-ts
npm install
npm start
```
