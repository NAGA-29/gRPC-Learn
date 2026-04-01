# Step 04: Unary 設計（FieldMask / ページネーション / Money 型）

## 学習内容

- **FieldMask**: 部分更新・部分取得（REST の PATCH / 疎なフィールド取得相当）
- **ページネーション**: `page_token` ベースのカーソルページネーション
- **Money 型**: float を使わずに通貨を安全に扱う方法
- **REST vs gRPC 設計比較**: 設計思想の違いを理解する

---

## ディレクトリ構成

```
step04-unary-design/
├── proto/               # .proto ファイル（step03 の buf ワークスペース共有または独立配置）
├── server-go/           # Go gRPC サーバー
│   ├── main.go
│   ├── go.mod
│   ├── handler/
│   │   └── product.go  # ProductService 実装
│   └── gen/step04/     # buf generate で生成（.gitkeep のみコミット）
└── client-ts/           # TypeScript gRPC クライアント
    ├── src/
    │   └── client.ts
    ├── package.json
    ├── tsconfig.json
    └── gen/step04/     # buf generate で生成（.gitkeep のみコミット）
```

---

## セットアップ手順

### 1. Proto ファイルの生成

```bash
# リポジトリルートまたは proto/ ディレクトリで実行
buf generate
```

生成物は `server-go/gen/step04/` と `client-ts/gen/step04/` に配置されます。

### 2. Go サーバー

```bash
cd server-go
go mod tidy
go run .
```

### 3. TypeScript クライアント

```bash
cd client-ts
npm install
npm start
```

---

## 実行手順

1. ターミナル A でサーバーを起動する:
   ```bash
   cd step04-unary-design/server-go && go run .
   ```

2. ターミナル B でクライアントを実行する:
   ```bash
   cd step04-unary-design/client-ts && npm start
   ```

3. `grpcurl` での手動確認（リフレクション登録済み）:
   ```bash
   # サービス一覧
   grpcurl -plaintext localhost:50051 list

   # GetProduct
   grpcurl -plaintext -d '{"id": "prod-001"}' localhost:50051 step04.ProductService/GetProduct

   # ListProducts（ページサイズ 2）
   grpcurl -plaintext -d '{"page_size": 2}' localhost:50051 step04.ProductService/ListProducts
   ```

---

## FieldMask の説明

### 概要

`FieldMask` は Protocol Buffers の標準型（`google.protobuf.FieldMask`）で、操作対象のフィールドを明示的に指定する仕組みです。

### 部分更新（UpdateMask）

REST の `PATCH` と同等の部分更新を実現します。

```protobuf
message UpdateProductRequest {
  Product product = 1;
  google.protobuf.FieldMask update_mask = 2; // 更新するフィールドを指定
}
```

```go
// Go サーバー側での FieldMask 処理例
for _, path := range req.UpdateMask.Paths {
    switch path {
    case "name":
        existing.Name = req.Product.Name
    case "price":
        existing.Price = req.Product.Price
    // ...
    }
}
```

```typescript
// TypeScript クライアント側での使用例
await client.updateProduct({
  product: { id: "prod-003", price: { currencyCode: "JPY", units: BigInt(3980), nanos: 0 } },
  updateMask: { paths: ["price"] }, // price フィールドのみ更新
});
```

**メリット:**
- 不要なフィールドの送信を省ける（帯域削減）
- 同時更新時の意図しない上書きを防止できる
- クライアントが「何を変更したか」を明示できる

### 部分取得（ReadMask）

レスポンスに含めるフィールドを絞り込みます。大きなオブジェクトから必要な情報だけを取得する際に有効です。

```typescript
// name と price だけ取得
await client.getProduct({
  id: "prod-001",
  readMask: { paths: ["name", "price"] },
});
```

---

## ページネーショントークンの説明

### cursor（トークン）ベース vs offset ベース

| 方式 | 仕組み | 特徴 |
|---|---|---|
| offset ベース | `?page=2&size=10` | シンプルだが、データ挿入・削除でズレが発生する |
| cursor ベース | `page_token` を次リクエストに渡す | データの追加・削除に強く、大規模データでも効率的 |

### このステップの実装

```protobuf
message ListProductsRequest {
  int32 page_size = 1;
  string page_token = 2; // 前回レスポンスの next_page_token を渡す
}

message ListProductsResponse {
  repeated Product products = 1;
  string next_page_token = 2; // 空文字列 = 最終ページ
  int32 total_size = 3;
}
```

**ページネーションのループ例（TypeScript）:**

```typescript
let pageToken = "";
do {
  const res = await client.listProducts({ pageSize: 2, pageToken });
  // res.products を処理...
  pageToken = res.nextPageToken;
} while (pageToken !== "");
```

---

## Money 型を float の代わりに使う理由

### 浮動小数点の精度問題

```javascript
// JavaScript での浮動小数点誤差の例
0.1 + 0.2 === 0.3  // false!  → 0.30000000000000004
```

金融計算で `float` / `double` を使うと丸め誤差が蓄積し、会計上の不整合が発生します。

### Money 型の設計

`google.type.Money` に倣った設計:

```protobuf
message Money {
  string currency_code = 1; // "JPY", "USD" など ISO 4217
  int64 units = 2;          // 整数部（例: 1234 → 1234円）
  int32 nanos = 3;          // 小数部（ナノ単位: 0.5円 → 500_000_000）
}
```

**例: 12.50 USD**
- `units = 12`
- `nanos = 500_000_000`（0.5 × 10^9）

**メリット:**
- 整数演算のみで完結し、丸め誤差が発生しない
- 通貨コードを型に含めるため、通貨の取り違えを防止できる
- proto3 の `int64` は TypeScript では `bigint` にマッピングされ、大きな金額も扱える

---

## REST vs gRPC 設計比較表

| 項目 | REST | gRPC |
|---|---|---|
| プロトコル | HTTP/1.1（主） | HTTP/2 |
| フォーマット | JSON（テキスト） | Protocol Buffers（バイナリ） |
| 型安全 | スキーマ任意（OpenAPI で補完） | Proto 定義が single source of truth |
| 部分更新 | `PATCH` + JSON Merge Patch | `UpdateRequest` + `FieldMask` |
| 部分取得 | クエリパラメータ `?fields=` | `ReadMask`（FieldMask） |
| ページネーション | `?page=1&per_page=10` など | `page_token` / `page_size` |
| エラー表現 | HTTP ステータスコード | `google.rpc.Status` + `codes.*` |
| 通貨表現 | 慣習に依存（float / string） | `google.type.Money` で標準化 |
| ストリーミング | Server-Sent Events / WebSocket | ネイティブサポート（双方向） |
| ブラウザ対応 | ネイティブ | grpc-web / Connect が必要 |
| 人間が読める | そのまま読める | `protoc` / grpcurl が必要 |
| コード生成 | OpenAPI Generator（任意） | `buf generate`（推奨） |

### いつ gRPC を選ぶか

- マイクロサービス間通信（内部 API）
- 型安全を重視したい場合
- ストリーミングが必要な場合
- 多言語クライアントが存在する場合

### いつ REST を選ぶか

- ブラウザから直接呼ぶ外部 API
- サードパーティが利用する公開 API（人間可読性が重要）
- 既存の HTTP インフラを活かしたい場合
