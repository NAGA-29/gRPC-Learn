/**
 * Step 06: TypeScript gRPC クライアント（Metadata / Deadline）
 *
 * 実演内容:
 *   1. カスタムメタデータ（x-request-id, authorization）を付けてリクエスト送信
 *   2. 遅延なし（正常ケース）
 *   3. 500ms 遅延 + 2秒デッドライン（正常完了）
 *   4. 2000ms 遅延 + 500ms デッドライン（タイムアウト期待）
 *
 * 前提: Go サーバーが localhost:50051 で起動していること
 *   cd ../server-go && go run .
 */
import { createClient, ConnectError } from "@connectrpc/connect";
import { createGrpcTransport } from "@connectrpc/connect-node";

// buf generate で生成されるファイルをインポート
// 事前に `npm run gen` を実行してください
import { MetadataService } from "../gen/step06/deadline_pb.js";

async function main() {
  // gRPC トランスポートを設定（HTTP/2）
  const transport = createGrpcTransport({
    baseUrl: "http://localhost:50051",
  });

  // MetadataService クライアントを生成
  const client = createClient(MetadataService, transport);

  console.log("=== Step 06: Metadata / Deadline デモ ===\n");

  // ----------------------------------------------------------------
  // 1. カスタムメタデータを付けてリクエスト送信
  // gRPC のメタデータは HTTP ヘッダー相当。
  // @connectrpc/connect では headers オプションで設定する。
  // ----------------------------------------------------------------
  console.log("--- 1. カスタムメタデータ付きリクエスト（遅延なし） ---");
  console.log("リクエスト送信: メタデータ x-request-id, authorization を付与");

  try {
    const res1 = await client.echo(
      {
        message: "Hello with metadata!",
        delayMs: 0,
      },
      {
        // headers オプションでメタデータ（HTTP ヘッダー）を設定する
        headers: {
          "x-request-id": "req-001-abc",
          authorization: "Bearer my-secret-token",
          "x-trace-id": "trace-xyz-789",
        },
      }
    );

    console.log("レスポンス受信:");
    console.log(`  メッセージ: ${res1.message}`);
    console.log(`  処理完了時刻: ${res1.processedAt}`);
    console.log("  サーバーが受け取ったメタデータ:");
    for (const [key, value] of Object.entries(res1.receivedMetadata)) {
      // gRPC システムメタデータ（:path 等）は除外して表示
      if (!key.startsWith(":")) {
        console.log(`    ${key}: ${value}`);
      }
    }
  } catch (err) {
    const connectErr = ConnectError.from(err);
    console.error(`  エラー: code=${connectErr.code}, message=${connectErr.message}`);
  }

  console.log();

  // ----------------------------------------------------------------
  // 2. 遅延なし（正常ケース）- メタデータなし
  // ----------------------------------------------------------------
  console.log("--- 2. 遅延なし・メタデータなし（正常ケース） ---");
  console.log("リクエスト送信: delay_ms=0");

  try {
    const res2 = await client.echo({
      message: "No delay, no metadata",
      delayMs: 0,
    });

    console.log("レスポンス受信:");
    console.log(`  メッセージ: ${res2.message}`);
    console.log(`  処理完了時刻: ${res2.processedAt}`);
  } catch (err) {
    const connectErr = ConnectError.from(err);
    console.error(`  エラー: code=${connectErr.code}, message=${connectErr.message}`);
  }

  console.log();

  // ----------------------------------------------------------------
  // 3. 500ms 遅延 + 2秒デッドライン（正常完了）
  // delay_ms(500) < timeoutMs(2000) なので、タイムアウトせず正常に完了する。
  // ----------------------------------------------------------------
  console.log("--- 3. 500ms 遅延 + 2秒デッドライン（正常完了） ---");
  console.log("リクエスト送信: delay_ms=500, timeoutMs=2000");

  try {
    const res3 = await client.echo(
      {
        message: "Slow but within deadline",
        delayMs: 500,
      },
      {
        // timeoutMs オプションでデッドライン（タイムアウト）を設定する
        // 指定した時間（ms）以内に応答がなければ DeadlineExceeded エラーになる
        timeoutMs: 2000,
      }
    );

    console.log("レスポンス受信（期待通り正常完了）:");
    console.log(`  メッセージ: ${res3.message}`);
    console.log(`  処理完了時刻: ${res3.processedAt}`);
  } catch (err) {
    const connectErr = ConnectError.from(err);
    console.error(`  エラー（予期しない）: code=${connectErr.code}, message=${connectErr.message}`);
  }

  console.log();

  // ----------------------------------------------------------------
  // 4. 2000ms 遅延 + 500ms デッドライン（タイムアウト期待）
  // delay_ms(2000) > timeoutMs(500) なので、デッドラインを超えてタイムアウトする。
  // サーバー側では context.Done() チャネルが閉じられて早期リターンされる。
  // ----------------------------------------------------------------
  console.log("--- 4. 2000ms 遅延 + 500ms デッドライン（タイムアウト期待） ---");
  console.log("リクエスト送信: delay_ms=2000, timeoutMs=500");

  try {
    await client.echo(
      {
        message: "This will timeout",
        delayMs: 2000,
      },
      {
        timeoutMs: 500,
      }
    );

    console.log("  ※ タイムアウトが発生しませんでした（予期しない結果）");
  } catch (err) {
    // デッドライン超過は ConnectError として受け取れる
    const connectErr = ConnectError.from(err);
    console.log("タイムアウトエラー受信（期待通り）:");
    console.log(`  code: ${connectErr.code}`);
    console.log(`  message: ${connectErr.message}`);
    console.log("  → クライアントのデッドラインがサーバーに伝播し、処理が中断されました");
  }

  console.log("\n=== 完了 ===");
}

main().catch((err) => {
  console.error("予期しないエラー:", err.message);
  process.exit(1);
});
