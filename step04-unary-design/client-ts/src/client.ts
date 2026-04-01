/**
 * Step 04: TypeScript gRPC クライアント（@connectrpc/connect 使用）
 *
 * 実演内容:
 *   1. GetProduct    - ID 指定で単一商品を取得（ReadMask あり）
 *   2. ListProducts  - ページネーションで商品一覧を取得
 *   3. UpdateProduct - FieldMask を使って部分更新（REST PATCH 相当）
 *
 * 前提: Go サーバーが localhost:50051 で起動していること
 *   cd ../server-go && go run .
 */
import { createClient } from "@connectrpc/connect";
import { createGrpcTransport } from "@connectrpc/connect-node";

// buf generate で生成されるファイルをインポート
// 事前に `npm run gen` を実行してください
import { ProductService } from "../gen/step04/product_pb.js";

async function main() {
  // gRPC トランスポートを設定（HTTP/2）
  const transport = createGrpcTransport({
    baseUrl: "http://localhost:50051",
    httpVersion: "2",
  });

  // ProductService クライアントを生成
  const client = createClient(ProductService, transport);

  console.log("=== Step 04: Unary 設計デモ（FieldMask / ページネーション / Money 型） ===\n");

  // ----------------------------------------------------------------
  // 1. GetProduct: ID を指定して商品を取得（ReadMask で name と price のみ取得）
  // ----------------------------------------------------------------
  console.log("--- 1. GetProduct（ReadMask: name, price） ---");
  console.log("リクエスト送信: id=prod-001, read_mask=[name, price]");

  const getRes = await client.getProduct({
    id: "prod-001",
    readMask: {
      paths: ["name", "price"],
    },
  });

  console.log("レスポンス受信:");
  console.log(`  商品名: ${getRes.product?.name}`);
  // Money 型: units（整数部）+ nanos（小数部）で精度を保つ
  const price = getRes.product?.price;
  if (price) {
    console.log(`  価格: ${price.units} ${price.currencyCode}`);
  }
  console.log(`  説明（ReadMask 未指定のため空）: "${getRes.product?.description}"\n`);

  // ----------------------------------------------------------------
  // 2. GetProduct: 存在しない ID でエラーを確認
  // ----------------------------------------------------------------
  console.log("--- 2. GetProduct（存在しない ID でエラー確認） ---");
  console.log("リクエスト送信: id=prod-999");
  try {
    await client.getProduct({ id: "prod-999" });
  } catch (err: unknown) {
    const error = err as { message: string; code: string };
    console.log(`  エラー受信（期待通り）: code=${error.code}, message=${error.message}\n`);
  }

  // ----------------------------------------------------------------
  // 3. ListProducts: ページネーションで商品一覧を取得
  // ----------------------------------------------------------------
  console.log("--- 3. ListProducts（ページネーション: page_size=2） ---");
  let pageToken = "";
  let pageNum = 1;

  do {
    console.log(`\n  ページ ${pageNum} 取得中... (token="${pageToken}")`);
    const listRes = await client.listProducts({
      pageSize: 2,
      pageToken: pageToken,
    });

    console.log(`  取得件数: ${listRes.products.length} / 総件数: ${listRes.totalSize}`);
    for (const p of listRes.products) {
      const p_price = p.price;
      const priceStr = p_price ? `${p_price.units} ${p_price.currencyCode}` : "N/A";
      console.log(`    - [${p.id}] ${p.name} / ${priceStr} / 在庫: ${p.inStock}`);
    }

    pageToken = listRes.nextPageToken;
    pageNum++;
  } while (pageToken !== "");

  console.log("\n  全ページ取得完了\n");

  // ----------------------------------------------------------------
  // 4. UpdateProduct: FieldMask で price と in_stock のみ更新
  // ----------------------------------------------------------------
  console.log("--- 4. UpdateProduct（FieldMask: price, in_stock のみ更新） ---");
  console.log("リクエスト送信: id=prod-003, update_mask=[price, in_stock]");

  const updateRes = await client.updateProduct({
    product: {
      id: "prod-003",
      name: "この名前は FieldMask に含まれないので無視される",
      price: {
        currencyCode: "JPY",
        units: BigInt(3980),
        nanos: 0,
      },
      inStock: true, // 在庫なし → あり に更新
    },
    updateMask: {
      // price と in_stock だけを更新。name や description は変更しない。
      paths: ["price", "in_stock"],
    },
  });

  console.log("レスポンス受信（更新後の商品）:");
  const updated = updateRes.product;
  if (updated) {
    console.log(`  商品名（変更なし）: ${updated.name}`);
    const uprice = updated.price;
    if (uprice) {
      console.log(`  価格（更新後）: ${uprice.units} ${uprice.currencyCode}`);
    }
    console.log(`  在庫状況（更新後）: ${updated.inStock}`);
  }

  console.log("\n=== 完了 ===");
}

main().catch((err) => {
  console.error("エラー:", err.message);
  process.exit(1);
});
