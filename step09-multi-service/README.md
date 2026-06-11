# Step 09: マルチサービス (BFF → Service → Worker)

複数の gRPC サービスを組み合わせたマイクロサービスアーキテクチャを学ぶステップです。
**BFF（Backend For Frontend）パターン**と**サービス間の同期 gRPC 呼び出し**を実装します。

## アーキテクチャ

```
┌──────────────────┐
│  TypeScript      │
│  クライアント     │  PlaceOrder
│  :クライアント   │──────────────────────────────┐
└──────────────────┘                              │
                                                  ▼
                                    ┌─────────────────────────┐
                                    │   GatewayService (Go)   │
                                    │   :50051                │
                                    │                         │
                                    │  1. ReserveStock 呼出   │
                                    │  2. order_id 生成       │
                                    │  3. Notify 呼出         │
                                    └────────────┬────────────┘
                                                 │
                             ┌───────────────────┴───────────────────┐
                             │                                       │
                             ▼                                       ▼
              ┌──────────────────────────┐         ┌──────────────────────────┐
              │   InventoryService (Go)  │         │ NotificationService (Go)  │
              │   :50052                 │         │   :50053                  │
              │                         │         │                           │
              │  ReserveStock           │         │  Notify                   │
              │  - 在庫確認              │         │  - ログ出力（模擬）         │
              │  - 在庫減算              │         │  - email/sms/push         │
              └──────────────────────────┘         └──────────────────────────┘
```

## ディレクトリ構成

```
step09-multi-service/
├── gateway/               # GatewayService (BFF, :50051)
│   ├── main.go
│   ├── go.mod
│   └── gen/step09/        # bash scripts/gen.sh で生成
├── inventory-svc/         # InventoryService (:50052)
│   ├── main.go
│   ├── go.mod
│   └── gen/step09/        # bash scripts/gen.sh で生成
├── notification-worker/   # NotificationService (:50053)
│   ├── main.go
│   ├── go.mod
│   └── gen/step09/        # bash scripts/gen.sh で生成
└── client-ts/             # TypeScript クライアント
    ├── src/client.ts
    ├── package.json
    ├── tsconfig.json
    └── gen/step09/        # bash scripts/gen.sh で生成（npm run gen でも可）
```

## 起動順序

サービス間の依存関係があるため、**以下の順番で起動してください**。

### 1. InventoryService（依存なし）

```bash
cd step09-multi-service/inventory-svc
go run .
# "InventoryService サーバー起動 port=50052" と表示されれば OK
```

### 2. NotificationService（依存なし）

```bash
cd step09-multi-service/notification-worker
go run .
# "NotificationService サーバー起動 port=50053" と表示されれば OK
```

### 3. GatewayService（InventoryService と NotificationService に依存）

```bash
cd step09-multi-service/gateway
go run .
# "Gateway サーバー起動 port=50051" と表示されれば OK
```

### 4. TypeScript クライアント

```bash
cd step09-multi-service/client-ts
npm install
npm start
```

## サービス間通信のパターン

### 同期 gRPC 呼び出し

Gateway は InventoryService と NotificationService を**同期的に**呼び出します。

```
クライアント                Gateway               Inventory         Notification
    │                         │                       │                  │
    │── PlaceOrder ──────────>│                       │                  │
    │                         │── ReserveStock ──────>│                  │
    │                         │<── Response ──────────│                  │
    │                         │── Notify ─────────────────────────────>  │
    │                         │<── Response ──────────────────────────── │
    │<── PlaceOrderResponse ──│                       │                  │
```

### 非同期化のパターン（実務）

通知など「失敗してもよい」処理は非同期化するのが一般的です。

```
Gateway → Kafka/RabbitMQ/SQS → NotificationWorker（非同期消費）
```

このステップでは学習のため同期呼び出しを使っています。
NotificationService の失敗は警告ログのみで、注文完了には影響しません。

## エラー伝播

### InventoryService エラー → Gateway がクライアントに伝える

```
InventoryService が在庫不足を返す
    ↓
Gateway が ResourceExhausted エラーを生成
    ↓
クライアントが ResourceExhausted エラーを受信
```

`InventoryService.ReserveStock` でエラーが発生した場合（在庫不足、存在しない商品など）、
Gateway はそのエラーをクライアントに伝播させます。

```go
// gateway/main.go の PlaceOrder 内
if !reserveResp.Reserved {
    return nil, status.Errorf(
        codes.ResourceExhausted,
        "在庫が不足しています (product_id=%s, 残り在庫=%d)",
        req.ProductId, reserveResp.RemainingStock,
    )
}
```

### NotificationService エラー → 警告ログのみ

```go
// gateway/main.go の PlaceOrder 内
if notifyErr != nil {
    // 通知失敗は警告ログのみ。注文は完了扱いにする。
    slog.Warn("NotificationService.Notify 失敗（警告のみ）", "error", notifyErr)
}
```

## 実務での考慮点

### サービスディスカバリ

このステップでは `localhost:50052` のようにアドレスをハードコードしていますが、
実務では動的なアドレス解決が必要です。

- **Kubernetes**: Service リソースの DNS（`inventory-svc.default.svc.cluster.local:50052`）
- **Consul**: HashiCorp Consul によるサービスレジストリ
- **etcd**: CoreOS etcd を使った分散設定管理
- **環境変数**: `INVENTORY_ADDR` のような環境変数で設定

### 負荷分散

gRPC の負荷分散には2種類のアプローチがあります。

| アプローチ | 説明 | 例 |
|-----------|------|-----|
| クライアント側 | クライアントが複数のサーバーに振り分ける | `grpc.WithDefaultServiceConfig` でラウンドロビン |
| プロキシ側 | Envoy, NGINX, Linkerd などのプロキシが振り分ける | Kubernetes の Envoy サイドカー |

gRPC は HTTP/2 の単一 TCP コネクションを多重化するため、
L4 ロードバランサー（NLB など）では適切に分散できません。
L7 ロードバランサー（ALB, Envoy）の使用を推奨します。

### Circuit Breaker（サーキットブレーカー）

内部サービスが不安定な場合、Gateway がリクエストを滞留させてカスケード障害が発生します。
サーキットブレーカーパターンで早期に失敗させることで障害の拡大を防ぎます。

```
InventoryService が連続して失敗
    ↓
サーキットブレーカーが OPEN 状態になる
    ↓
一定期間、Gateway は InventoryService に接続せず即座にエラーを返す
    ↓
HALF-OPEN 状態でテストリクエストを送信
    ↓
成功すれば CLOSED（通常）状態に戻る
```

Go の実装例: `github.com/sony/gobreaker`, `github.com/afex/hystrix-go`

### タイムアウト設定

このステップの Gateway では通知送信に 3 秒のタイムアウトを設定しています。

```go
// リクエストの ctx を親にすることで、クライアントのキャンセルも伝播する
notifyCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
```

本番では:
- 各サービス間の SLA に応じてタイムアウトを設定する
- `context` を伝播して、クライアントのキャンセルをサービスチェーン全体に反映させる
- デッドライン（`grpc.WithDeadline`）でエンドツーエンドの制限を設ける
