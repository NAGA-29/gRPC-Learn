/**
 * Step 08: TypeScript gRPC クライアント（インターセプター動作確認）
 *
 * 実演内容:
 *   1. Ping を呼び出す（認証不要・常に成功）
 *   2. GetSecret を認証なしで呼び出す（エラー期待: Unauthenticated）
 *   3. GetSecret を正しいトークンで呼び出す（成功期待）
 *   4. GetSecret を間違ったトークンで呼び出す（エラー期待: Unauthenticated）
 *
 * クライアント側のヘッダー設定:
 *   @connectrpc/connect では headers オプションで gRPC メタデータを設定する。
 *   サーバーの auth インターセプターが "authorization" キーを読み取る。
 *
 * 前提: Go サーバーが localhost:50051 で起動していること
 *   cd ../server-go && go run .
 */
import { createClient, ConnectError, Code } from "@connectrpc/connect";
import { createGrpcTransport } from "@connectrpc/connect-node";

// buf generate で生成されるファイルをインポート
// 事前に `npm run gen` を実行してください
import { InterceptorService } from "../gen/step08/interceptor_pb.js";

async function main() {
  // gRPC トランスポートを設定（HTTP/2）
  const transport = createGrpcTransport({
    baseUrl: "http://localhost:50051",
  });

  // InterceptorService クライアントを生成
  const client = createClient(InterceptorService, transport);

  console.log("=== Step 08: インターセプター動作確認デモ ===\n");

  // ----------------------------------------------------------------
  // 1. Ping（認証不要の公開エンドポイント）
  // auth インターセプターは GetSecret 以外をスキップするため、
  // Authorization ヘッダーなしで呼び出せる。
  // ----------------------------------------------------------------
  console.log("--- 1. Ping（認証不要） ---");
  try {
    const res = await client.ping({ message: "Hello Interceptor!" });
    console.log("成功:");
    console.log(`  メッセージ: ${res.message}`);
    console.log(`  サーバーID: ${res.serverId}`);
  } catch (err) {
    if (err instanceof ConnectError) {
      console.error(`エラー [${Code[err.code]}]: ${err.message}`);
    } else {
      console.error("予期せぬエラー:", err);
    }
  }
  console.log();

  // ----------------------------------------------------------------
  // 2. GetSecret（認証なし → Unauthenticated エラー期待）
  // Authorization ヘッダーを付けずに保護エンドポイントを呼び出す。
  // auth インターセプターが "authorization ヘッダーが必要です" を返す。
  // ----------------------------------------------------------------
  console.log("--- 2. GetSecret（認証なし → エラー期待） ---");
  try {
    const res = await client.getSecret({});
    // ここには到達しないはず
    console.log("予期せぬ成功:", res.secretData);
  } catch (err) {
    if (err instanceof ConnectError) {
      // ConnectError からエラーコード名を取得して表示する
      console.log(`期待通りのエラー [${Code[err.code]}]: ${err.message}`);
    } else {
      console.error("予期せぬエラー:", err);
    }
  }
  console.log();

  // ----------------------------------------------------------------
  // 3. GetSecret（正しいトークン → 成功期待）
  // headers オプションで "authorization" キーに Bearer トークンを設定する。
  // gRPC メタデータは HTTP ヘッダー相当で、サーバー側は metadata パッケージで読み取る。
  // ----------------------------------------------------------------
  console.log("--- 3. GetSecret（正しいトークン → 成功期待） ---");
  try {
    const res = await client.getSecret(
      {},
      {
        // Authorization ヘッダーを設定する（gRPC メタデータとして送信される）
        headers: {
          authorization: "Bearer secret-token-12345",
        },
      }
    );
    console.log("成功:");
    console.log(`  機密データ: ${res.secretData}`);
  } catch (err) {
    if (err instanceof ConnectError) {
      console.error(`エラー [${Code[err.code]}]: ${err.message}`);
    } else {
      console.error("予期せぬエラー:", err);
    }
  }
  console.log();

  // ----------------------------------------------------------------
  // 4. GetSecret（間違ったトークン → Unauthenticated エラー期待）
  // 有効でないトークンを送ると auth インターセプターが弾く。
  // ハンドラーには到達しないため、機密データは返されない。
  // ----------------------------------------------------------------
  console.log("--- 4. GetSecret（間違ったトークン → エラー期待） ---");
  try {
    const res = await client.getSecret(
      {},
      {
        headers: {
          authorization: "Bearer wrong-token-99999",
        },
      }
    );
    // ここには到達しないはず
    console.log("予期せぬ成功:", res.secretData);
  } catch (err) {
    if (err instanceof ConnectError) {
      console.log(`期待通りのエラー [${Code[err.code]}]: ${err.message}`);
    } else {
      console.error("予期せぬエラー:", err);
    }
  }
  console.log();

  console.log("=== デモ完了 ===");
  console.log("サーバー側ログでインターセプターの実行順序（logging→auth→metrics）を確認してください。");
}

main().catch((err) => {
  console.error("main() でキャッチされないエラー:", err);
  process.exit(1);
});
