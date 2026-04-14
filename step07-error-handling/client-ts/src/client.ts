/**
 * Step 07: TypeScript gRPC クライアント（Error Handling）
 *
 * 実演内容:
 *   全エラーシナリオを順番に呼び出し、ConnectError で受け取る。
 *   - NOT_FOUND          → codes.NotFound (5)
 *   - PERMISSION_DENIED  → codes.PermissionDenied (7)
 *   - INVALID_ARGUMENT   → codes.InvalidArgument (3)（BadRequestDetail 付き）
 *   - UNAVAILABLE        → codes.Unavailable (14)
 *   - RESOURCE_EXHAUSTED → codes.ResourceExhausted (8)
 *   - INTERNAL           → codes.Internal (13)
 *   - UNSPECIFIED        → 正常レスポンス
 *
 * 前提: Go サーバーが localhost:50051 で起動していること
 *   cd ../server-go && go run .
 */
import { createClient, ConnectError } from "@connectrpc/connect";
import { createGrpcTransport } from "@connectrpc/connect-node";

// buf generate で生成されるファイルをインポート
// 事前に `npm run gen` を実行してください
import { ErrorService, ErrorScenario } from "../gen/step07/error_pb.js";

async function main() {
  // gRPC トランスポートを設定（HTTP/2）
  const transport = createGrpcTransport({
    baseUrl: "http://localhost:50051",
    httpVersion: "2",
  });

  // ErrorService クライアントを生成
  const client = createClient(ErrorService, transport);

  console.log("=== Step 07: Error Handling デモ ===\n");

  // ----------------------------------------------------------------
  // 各シナリオを順番に呼び出す
  // ----------------------------------------------------------------

  const scenarios: Array<{
    label: string;
    scenario: ErrorScenario;
    resourceId: string;
  }> = [
    {
      label: "1. NOT_FOUND（リソースが存在しない）",
      scenario: ErrorScenario.NOT_FOUND,
      resourceId: "item-42",
    },
    {
      label: "2. PERMISSION_DENIED（アクセス権限なし）",
      scenario: ErrorScenario.PERMISSION_DENIED,
      resourceId: "",
    },
    {
      label: "3. INVALID_ARGUMENT（不正なパラメータ + BadRequestDetail）",
      scenario: ErrorScenario.INVALID_ARGUMENT,
      resourceId: "",
    },
    {
      label: "4. UNAVAILABLE（一時的なサービス停止 / リトライ可能）",
      scenario: ErrorScenario.UNAVAILABLE,
      resourceId: "",
    },
    {
      label: "5. RESOURCE_EXHAUSTED（レートリミット超過）",
      scenario: ErrorScenario.RESOURCE_EXHAUSTED,
      resourceId: "",
    },
    {
      label: "6. INTERNAL（内部エラー）",
      scenario: ErrorScenario.INTERNAL,
      resourceId: "",
    },
    {
      label: "7. UNSPECIFIED（正常ケース）",
      scenario: ErrorScenario.UNSPECIFIED,
      resourceId: "",
    },
  ];

  for (const { label, scenario, resourceId } of scenarios) {
    console.log(`--- ${label} ---`);

    try {
      const res = await client.triggerError({
        scenario,
        resourceId,
      });

      // 正常レスポンス（UNSPECIFIED の場合のみここに到達する）
      console.log(`  成功: ${res.result}`);
    } catch (err) {
      // gRPC エラーは ConnectError として受け取る
      const connectErr = ConnectError.from(err);

      console.log(`  エラー受信:`);
      console.log(`    code:    ${connectErr.code} (${connectErr.code})`);
      console.log(`    message: ${connectErr.message}`);

      // INVALID_ARGUMENT の場合は Rich Error Details を表示する
      // ConnectError.details() で Any 型から proto メッセージを取り出せる
      if (connectErr.details.length > 0) {
        console.log(`    details: ${connectErr.details.length} 件の詳細情報が含まれています`);
        // 注意: details のデコードには proto スキーマ情報が必要なため
        // ここでは件数のみ表示する（実際の実装では findDetails() を使う）
      }
    }

    console.log();
  }

  console.log("=== 完了 ===");
  console.log();
  console.log("【学習ポイント】");
  console.log("  - gRPC エラーは ConnectError.code で数値コードを確認できる");
  console.log("  - code=5(NOT_FOUND), 7(PERMISSION_DENIED), 3(INVALID_ARGUMENT)");
  console.log("  - code=14(UNAVAILABLE) はリトライ可能なエラー");
  console.log("  - INVALID_ARGUMENT には BadRequestDetail でフィールドの詳細を添付できる");
}

main().catch((err) => {
  console.error("予期しないエラー:", err.message);
  process.exit(1);
});
