/**
 * Step 05: TypeScript gRPC ストリーミングクライアント（@connectrpc/connect 使用）
 *
 * 実演内容:
 *   1. WatchStock   - サーバーストリーミング: 株価を10回受信して終了
 *   2. UploadFile   - クライアントストリーミング: 3チャンクを送信してサマリを受け取る
 *   3. Chat         - 双方向ストリーミング: 3メッセージを送受信
 *
 * 前提: Go サーバーが localhost:50052 で起動していること
 *   cd ../server-go && go run .
 */
import { createClient } from "@connectrpc/connect";
import { createGrpcTransport } from "@connectrpc/connect-node";

// buf generate で生成されるファイルをインポート
// 事前に `npm run gen` を実行してください
import { StreamService } from "../gen/step05/stream_pb.js";

async function main() {
  // gRPC トランスポートを設定（HTTP/2）
  const transport = createGrpcTransport({
    baseUrl: "http://localhost:50052",
    httpVersion: "2",
  });

  // StreamService クライアントを生成
  const client = createClient(StreamService, transport);

  console.log("=== Step 05: ストリーミング デモ（サーバー/クライアント/双方向） ===\n");

  // ----------------------------------------------------------------
  // 1. WatchStock: サーバーストリーミング
  //    サーバーが非同期に株価を送り続ける。async iterator で受信する。
  // ----------------------------------------------------------------
  console.log("--- 1. WatchStock（サーバーストリーミング: 10回受信して終了） ---");
  console.log("リクエスト送信: symbol=GRPC");

  let receiveCount = 0;
  const maxReceive = 10;

  // @connectrpc/connect のサーバーストリーミングは async iterable で受信する
  for await (const stockPrice of client.watchStock({ symbol: "GRPC" })) {
    receiveCount++;
    const priceStr = stockPrice.price.toFixed(2);
    const ts = new Date(Number(stockPrice.timestamp) * 1000).toISOString();
    console.log(`  [${receiveCount}/${maxReceive}] ${stockPrice.symbol}: ¥${priceStr} (${ts})`);

    // 10回受信したらループを抜ける（ストリームをキャンセル）
    if (receiveCount >= maxReceive) {
      console.log("  → 10回受信完了。ストリームを終了します。");
      break;
    }
  }

  console.log();

  // ----------------------------------------------------------------
  // 2. UploadFile: クライアントストリーミング
  //    クライアントが複数のチャンクを送り、最後にサマリを受け取る。
  // ----------------------------------------------------------------
  console.log("--- 2. UploadFile（クライアントストリーミング: 3チャンクを送信） ---");

  // async generator でチャンクを生成して送信する
  async function* generateChunks() {
    const chunks = [
      { fileName: "sample.txt", data: new TextEncoder().encode("Hello, gRPC! ") },
      { fileName: "sample.txt", data: new TextEncoder().encode("This is chunk 2. ") },
      { fileName: "sample.txt", data: new TextEncoder().encode("Final chunk here.") },
    ];

    for (let i = 0; i < chunks.length; i++) {
      const chunk = chunks[i];
      console.log(`  チャンク ${i + 1}/${chunks.length} 送信: ${chunk.data.length} バイト`);
      yield chunk;

      // チャンク送信間隔（100ms）
      await new Promise((resolve) => setTimeout(resolve, 100));
    }
  }

  // uploadFile はクライアントストリーミング: async iterable を渡すと最後にサマリが返る
  const summary = await client.uploadFile(generateChunks());
  console.log("サマリ受信:");
  console.log(`  ファイル名: ${summary.fileName}`);
  console.log(`  合計バイト数: ${summary.totalBytes}`);
  console.log(`  チャンク数: ${summary.chunkCount}`);
  console.log(`  メッセージ: ${summary.message}`);
  console.log();

  // ----------------------------------------------------------------
  // 3. Chat: 双方向ストリーミング
  //    クライアントがメッセージを送りながら、サーバーからの応答を非同期で受け取る。
  // ----------------------------------------------------------------
  console.log("--- 3. Chat（双方向ストリーミング: 3メッセージを送受信） ---");

  const chatMessages = [
    { room: "grpc-study", sender: "TypeScript-Client", text: "こんにちは！gRPC 双方向ストリーミングのテストです。" },
    { room: "grpc-study", sender: "TypeScript-Client", text: "サーバーストリーミング と クライアントストリーミング も学びました！" },
    { room: "grpc-study", sender: "TypeScript-Client", text: "最後のメッセージです。ありがとうございました！" },
  ];

  // async generator でメッセージを順番に送信する
  async function* generateMessages() {
    for (let i = 0; i < chatMessages.length; i++) {
      const msg = chatMessages[i];
      console.log(`  送信 [${i + 1}/${chatMessages.length}]: "${msg.text}"`);
      yield msg;

      // メッセージ送信間隔（200ms）
      await new Promise((resolve) => setTimeout(resolve, 200));
    }
  }

  // chat は双方向ストリーミング: async iterable を渡すと async iterable が返る
  console.log("受信メッセージ:");
  let chatReceiveCount = 0;
  for await (const received of client.chat(generateMessages())) {
    chatReceiveCount++;
    console.log(`  受信 [${chatReceiveCount}]: [${received.room}] ${received.sender}: "${received.text}"`);

    // 送信数分受信したら完了
    if (chatReceiveCount >= chatMessages.length) {
      break;
    }
  }

  console.log("\n=== 完了 ===");
}

main().catch((err) => {
  console.error("エラー:", err.message ?? err);
  process.exit(1);
});
