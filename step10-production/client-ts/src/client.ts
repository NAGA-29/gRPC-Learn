/**
 * Step 10: TypeScript gRPC クライアント（本番対応機能の動作確認）
 *
 * 実演内容:
 *   1. Check（ヘルスチェック）を呼び出す
 *   2. GetMetrics（メトリクス取得）を呼び出す
 *   3. 複数回 Check を呼び出してメトリクスが増加することを確認する
 *
 * 接続設定:
 *   - TLS なし（insecure）で localhost:50051 に接続
 *   - TLS 有効化サーバーに接続する場合は baseUrl を https:// に変更
 *
 * 前提: サーバーが起動していること
 *   cd ../server-go && go run .
 *   # TLS 有効化の場合:
 *   # cd ../server-go && TLS_ENABLED=true go run .
 */
import { createClient, ConnectError, Code } from "@connectrpc/connect";
import { createGrpcTransport } from "@connectrpc/connect-node";
// google.protobuf.Timestamp を Date に変換するヘルパー（protobuf-es v2）
import { timestampDate } from "@bufbuild/protobuf/wkt";

// buf generate で生成されるファイルをインポート
// 事前に `npm run gen` を実行してください
import { ProductionService } from "../gen/step10/production_pb.js";

async function main() {
  // gRPC トランスポートを設定（HTTP/2, TLS なし）
  // TLS 有効化サーバーに接続する場合は baseUrl を "https://localhost:50051" に変更し、
  // nodeOptions で CA 証明書を指定する（README 参照）
  const transport = createGrpcTransport({
    baseUrl: "http://localhost:50051",
  });

  // ProductionService クライアントを生成
  const client = createClient(ProductionService, transport);

  console.log("=== Step 10: 本番対応 gRPC サーバー動作確認デモ ===\n");

  // ----------------------------------------------------------------
  // 1. ヘルスチェック（全体）
  // service="" は全体のヘルスチェックを示す
  // ----------------------------------------------------------------
  console.log("--- 1. ヘルスチェック（service=\"\"）---");
  try {
    const res = await client.check({ service: "" });
    console.log("ヘルスチェック成功:");
    console.log(`  ステータス:   ${res.status} (1=SERVING, 2=NOT_SERVING)`);
    console.log(`  メッセージ:   ${res.message}`);
    if (res.checkedAt) {
      console.log(`  チェック日時: ${timestampDate(res.checkedAt).toISOString()}`);
    }
  } catch (err) {
    if (err instanceof ConnectError) {
      console.error(`エラー [${Code[err.code]}]: ${err.message}`);
    } else {
      console.error("予期せぬエラー:", err);
    }
  }
  console.log();

  // ----------------------------------------------------------------
  // 2. 特定サービスのヘルスチェック
  // grpc.health.v1 プロトコルでは service 名でチェック対象を絞れる
  // ----------------------------------------------------------------
  console.log("--- 2. ヘルスチェック（service=\"ProductionService\"） ---");
  try {
    const res = await client.check({ service: "ProductionService" });
    console.log("ヘルスチェック成功:");
    console.log(`  ステータス:   ${res.status}`);
    console.log(`  メッセージ:   ${res.message}`);
  } catch (err) {
    if (err instanceof ConnectError) {
      console.error(`エラー [${Code[err.code]}]: ${err.message}`);
    } else {
      console.error("予期せぬエラー:", err);
    }
  }
  console.log();

  // ----------------------------------------------------------------
  // 3. メトリクス取得（Check 呼び出し前）
  // grpc_requests_total カウンターが 2（上の2回分）になっているはず
  // ----------------------------------------------------------------
  console.log("--- 3. メトリクス取得 ---");
  try {
    const res = await client.getMetrics({});
    console.log("メトリクス取得成功:");
    console.log("  カウンター（リクエスト数）:");
    for (const [method, count] of Object.entries(res.counters)) {
      console.log(`    ${method}: ${count}`);
    }
    console.log("  ゲージ（現在値）:");
    for (const [name, value] of Object.entries(res.gauges)) {
      console.log(`    ${name}: ${value}`);
    }
    console.log();
    console.log("  Prometheus の生メトリクスは http://localhost:8080/metrics で確認できます");
  } catch (err) {
    if (err instanceof ConnectError) {
      console.error(`エラー [${Code[err.code]}]: ${err.message}`);
    } else {
      console.error("予期せぬエラー:", err);
    }
  }
  console.log();

  console.log("=== デモ完了 ===");
  console.log("サーバー側ログで以下を確認してください:");
  console.log("  - zap の構造化ログ（JSON 形式）");
  console.log("  - OpenTelemetry のスパンデータ（stdout に JSON で出力）");
  console.log("  - http://localhost:8080/metrics で Prometheus メトリクスを確認");
}

main().catch((err) => {
  console.error("main() でキャッチされないエラー:", err);
  process.exit(1);
});
