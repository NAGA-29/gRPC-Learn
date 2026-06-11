/**
 * Step 09: TypeScript gRPC クライアント（マルチサービス動作確認）
 *
 * 実演内容:
 *   1. PlaceOrder を在庫あり商品 (product-A, quantity=5) で呼び出す → 成功期待
 *   2. PlaceOrder を在庫あり商品 (product-B, quantity=30) で呼び出す → 成功期待
 *   3. PlaceOrder を在庫なし商品 (product-D, quantity=1) で呼び出す → ResourceExhausted エラー期待
 *
 * アーキテクチャ:
 *   TypeScript クライアント → GatewayService(:50051)
 *                              ├─ InventoryService(:50052)  在庫確保
 *                              └─ NotificationService(:50053) 通知送信
 *
 * 前提: 各サービスが起動していること
 *   1. cd ../inventory-svc && go run .
 *   2. cd ../notification-worker && go run .
 *   3. cd ../gateway && go run .
 *   4. npm start (このクライアント)
 */
import { createClient, ConnectError, Code } from "@connectrpc/connect";
import { createGrpcTransport } from "@connectrpc/connect-node";

// buf generate で生成されるファイルをインポート
// 事前に `npm run gen` を実行してください
import { GatewayService } from "../gen/step09/gateway_pb.js";

async function main() {
  // GatewayService への gRPC トランスポートを設定（HTTP/2, TLS なし）
  const transport = createGrpcTransport({
    baseUrl: "http://localhost:50051",
  });

  // GatewayService クライアントを生成
  const client = createClient(GatewayService, transport);

  console.log("=== Step 09: マルチサービス (BFF → Service → Worker) デモ ===\n");
  console.log("アーキテクチャ: TypeScript クライアント → GatewayService → InventoryService + NotificationService\n");

  // ----------------------------------------------------------------
  // 1. 在庫あり商品の注文（成功ケース）
  // product-A の初期在庫: 100
  // ----------------------------------------------------------------
  console.log("--- 1. 在庫あり商品の注文（product-A, quantity=5） ---");
  try {
    const res = await client.placeOrder({
      userId: "user-001",
      productId: "product-A",
      quantity: 5,
    });
    console.log("注文成功:");
    console.log(`  注文ID:       ${res.orderId}`);
    console.log(`  ステータス:   ${res.status}`);
    console.log(`  配送予定日:   ${res.estimatedDelivery}`);
  } catch (err) {
    if (err instanceof ConnectError) {
      console.error(`エラー [${Code[err.code]}]: ${err.message}`);
    } else {
      console.error("予期せぬエラー:", err);
    }
  }
  console.log();

  // ----------------------------------------------------------------
  // 2. 在庫あり商品の注文（成功ケース）
  // product-B の初期在庫: 50
  // ----------------------------------------------------------------
  console.log("--- 2. 在庫あり商品の注文（product-B, quantity=30） ---");
  try {
    const res = await client.placeOrder({
      userId: "user-002",
      productId: "product-B",
      quantity: 30,
    });
    console.log("注文成功:");
    console.log(`  注文ID:       ${res.orderId}`);
    console.log(`  ステータス:   ${res.status}`);
    console.log(`  配送予定日:   ${res.estimatedDelivery}`);
  } catch (err) {
    if (err instanceof ConnectError) {
      console.error(`エラー [${Code[err.code]}]: ${err.message}`);
    } else {
      console.error("予期せぬエラー:", err);
    }
  }
  console.log();

  // ----------------------------------------------------------------
  // 3. 在庫なし商品の注文（ResourceExhausted エラー期待）
  // product-D の初期在庫: 0
  // Gateway は InventoryService から在庫不足を受け取り、
  // ResourceExhausted エラーをクライアントに返す。
  // ----------------------------------------------------------------
  console.log("--- 3. 在庫なし商品の注文（product-D, quantity=1） → エラー期待 ---");
  try {
    const res = await client.placeOrder({
      userId: "user-003",
      productId: "product-D",
      quantity: 1,
    });
    // ここには到達しないはず
    console.log("予期せぬ成功:", res.orderId);
  } catch (err) {
    if (err instanceof ConnectError) {
      // ResourceExhausted = 在庫不足
      console.log(`期待通りのエラー [${Code[err.code]}]: ${err.message}`);
    } else {
      console.error("予期せぬエラー:", err);
    }
  }
  console.log();

  console.log("=== デモ完了 ===");
  console.log("各サービスのログで内部呼び出しの流れを確認してください。");
  console.log("  Gateway ログ:             注文処理フロー全体");
  console.log("  InventoryService ログ:    在庫確保の詳細");
  console.log("  NotificationService ログ: 通知送信の模擬");
}

main().catch((err) => {
  console.error("main() でキャッチされないエラー:", err);
  process.exit(1);
});
