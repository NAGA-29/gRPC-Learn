/**
 * Step 02: TypeScript gRPC クライアント（@connectrpc/connect 使用）
 *
 * 前提: Go サーバーが localhost:50051 で起動していること
 *   cd ../server-go && go run .
 */
import { createClient } from "@connectrpc/connect";
import { createGrpcTransport } from "@connectrpc/connect-node";

// buf generate で生成されるファイルをインポート
// 事前に `npm run gen` を実行してください
import { GreeterService } from "../gen/step02/greeter_pb.js";

async function main() {
  // gRPC トランスポートを設定（HTTP/2）
  const transport = createGrpcTransport({
    baseUrl: "http://localhost:50051",
  });

  // クライアントを生成
  const client = createClient(GreeterService, transport);

  console.log("=== Step 02: Unary gRPC 通信デモ ===\n");

  // --- 日本語で挨拶 ---
  console.log("1. 日本語でリクエスト送信...");
  const res1 = await client.sayHello({ name: "太郎", language: "ja" });
  console.log(`   レスポンス: ${res1.message}`);
  console.log(`   サーバー時刻: ${res1.serverTime}\n`);

  // --- 英語で挨拶 ---
  console.log("2. 英語でリクエスト送信...");
  const res2 = await client.sayHello({ name: "Alice", language: "en" });
  console.log(`   レスポンス: ${res2.message}`);
  console.log(`   サーバー時刻: ${res2.serverTime}\n`);

  // --- スペイン語で挨拶 ---
  console.log("3. スペイン語でリクエスト送信...");
  const res3 = await client.sayHello({ name: "Carlos", language: "es" });
  console.log(`   レスポンス: ${res3.message}`);
  console.log(`   サーバー時刻: ${res3.serverTime}\n`);

  console.log("=== 完了 ===");
}

main().catch((err) => {
  console.error("エラー:", err.message);
  process.exit(1);
});
