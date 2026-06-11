# Step 07: Error Handling

gRPC の標準ステータスコードと Rich Error Details を学ぶステップ。

## gRPC 主要ステータスコード一覧

| コード | 数値 | 意味 | 使用シーン |
|--------|------|------|-----------|
| `OK` | 0 | 成功 | 正常完了 |
| `CANCELLED` | 1 | キャンセル | クライアントがキャンセル |
| `UNKNOWN` | 2 | 不明なエラー | 詳細不明の障害 |
| `INVALID_ARGUMENT` | 3 | 不正な引数 | バリデーションエラー |
| `DEADLINE_EXCEEDED` | 4 | デッドライン超過 | タイムアウト |
| `NOT_FOUND` | 5 | リソースなし | 存在しない ID を指定 |
| `ALREADY_EXISTS` | 6 | 既に存在 | 重複作成の試み |
| `PERMISSION_DENIED` | 7 | 権限なし | 認可エラー |
| `RESOURCE_EXHAUSTED` | 8 | リソース枯渇 | レートリミット超過 |
| `FAILED_PRECONDITION` | 9 | 前提条件違反 | 不正な状態遷移 |
| `INTERNAL` | 13 | 内部エラー | サーバー内部の予期しない障害 |
| `UNAVAILABLE` | 14 | 利用不可 | 一時的なサービス停止 |
| `UNAUTHENTICATED` | 16 | 未認証 | 認証情報なし/不正 |

---

## Go でのエラーの返し方

```go
import (
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"
)

// シンプルなエラー
return nil, status.Errorf(codes.NotFound, "リソース '%s' が見つかりません", id)

// Rich Error Details 付きエラー（BadRequest 詳細を添付）
st, err := status.New(codes.InvalidArgument, "パラメータが不正").
    WithDetails(&pb.BadRequestDetail{
        Violations: []*pb.FieldViolation{
            {Field: "name", Description: "空にできません"},
        },
    })
if err != nil {
    // WithDetails が失敗した場合のフォールバック
    return nil, status.Errorf(codes.InvalidArgument, "パラメータが不正")
}
return nil, st.Err()
```

---

## TypeScript でのエラーの受け取り方（@connectrpc/connect）

```typescript
import { ConnectError } from "@connectrpc/connect";

try {
    const res = await client.triggerError({ scenario, resourceId });
    console.log("成功:", res.result);
} catch (err) {
    // gRPC エラーは ConnectError として受け取る
    const connectErr = ConnectError.from(err);

    console.log("code:   ", connectErr.code);    // 数値コード (例: 5)
    console.log("message:", connectErr.message); // エラーメッセージ

    // Rich Error Details が含まれている場合
    if (connectErr.details.length > 0) {
        // findDetails() で特定の proto メッセージ型を取り出す
        // const details = connectErr.findDetails(BadRequestDetail);
    }
}
```

---

## Rich Error Details とは

標準の gRPC エラーにはコードとメッセージしか含まれないが、  
`status.WithDetails()` を使うことで `google.protobuf.Any` 型として  
追加情報を埋め込むことができる。

### 典型的な使用例

```proto
// 不正なフィールドの詳細情報
message BadRequestDetail {
    repeated FieldViolation violations = 1;
}

message FieldViolation {
    string field       = 1; // 不正なフィールド名
    string description = 2; // エラー詳細
}
```

```go
// サーバー側: 詳細を添付してエラーを返す
st, _ := status.New(codes.InvalidArgument, "バリデーションエラー").
    WithDetails(&pb.BadRequestDetail{...})
return nil, st.Err()
```

```typescript
// クライアント側: 詳細を取り出す
const details = connectErr.findDetails(BadRequestDetail);
```

---

## リトライ可能なエラー vs 不可能なエラー

### リトライ可能（TRANSIENT）

| コード | 理由 |
|--------|------|
| `UNAVAILABLE` | サービスが一時停止中。しばらく待ってリトライ |
| `RESOURCE_EXHAUSTED` | レートリミット。バックオフ後にリトライ |
| `DEADLINE_EXCEEDED` | タイムアウト。べき等な操作ならリトライ可 |

### リトライ不可（PERMANENT）

| コード | 理由 |
|--------|------|
| `NOT_FOUND` | リソースが存在しない。リトライしても変わらない |
| `PERMISSION_DENIED` | 権限がない。リトライしても変わらない |
| `INVALID_ARGUMENT` | リクエスト自体が不正。修正が必要 |
| `ALREADY_EXISTS` | 既に存在する。重複作成はリトライ不要 |
| `INTERNAL` | 原因不明。安易なリトライは避ける |

---

## エラー設計のベストプラクティス

1. **適切なコードを選ぶ**  
   `INTERNAL` は「原因不明」の最後の手段。可能な限り具体的なコードを使う。

2. **メッセージはデバッグ用**  
   エラーメッセージはログやデバッグのためのもの。エンドユーザーには表示しない。

3. **Rich Error Details で詳細を伝える**  
   バリデーションエラーは `INVALID_ARGUMENT` + `BadRequest` で不正フィールドを明示する。

4. **`UNAUTHENTICATED` vs `PERMISSION_DENIED`**  
   - 未認証（トークンなし/期限切れ）→ `UNAUTHENTICATED`  
   - 認証済みだが権限なし → `PERMISSION_DENIED`

5. **`NOT_FOUND` の情報漏洩に注意**  
   セキュリティ上の理由で存在確認を避けたい場合は `PERMISSION_DENIED` を使うことも検討する。

---

## ディレクトリ構成

```
step07-error-handling/
├── server-go/            # Go gRPC サーバー
│   ├── main.go           # サーバーエントリポイント（:50051）
│   ├── handler/
│   │   └── error.go      # ErrorService.TriggerError 実装
│   ├── gen/step07/       # buf generate で生成（.gitkeep のみ管理）
│   └── go.mod
├── client-ts/            # TypeScript gRPC クライアント
│   ├── src/
│   │   └── client.ts     # 全シナリオのデモ
│   ├── gen/step07/       # buf generate で生成（.gitkeep のみ管理）
│   ├── package.json
│   └── tsconfig.json
└── README.md
```

---

## 実行方法

### 1. Proto ファイルからコード生成

```bash
# リポジトリルートで実行
bash scripts/gen.sh
```

### 2. Go サーバーを起動

```bash
cd step07-error-handling/server-go
go mod download
go run .
# → Go gRPC エラーハンドリングサーバー起動 port=50051
```

### 3. TypeScript クライアントを実行

```bash
cd step07-error-handling/client-ts
npm install
npm start
```

### 期待される出力例

```
=== Step 07: Error Handling デモ ===

--- 1. NOT_FOUND（リソースが存在しない） ---
  エラー受信:
    code:    5 (NotFound)
    message: [not_found] リソース 'item-42' が見つかりません

--- 3. INVALID_ARGUMENT（不正なパラメータ + BadRequestDetail） ---
  エラー受信:
    code:    3 (InvalidArgument)
    message: [invalid_argument] リクエストパラメータが不正です
    details: 1 件の詳細情報が含まれています

--- 7. UNSPECIFIED（正常ケース） ---
  成功: エラーなし: 正常に処理が完了しました

=== 完了 ===
```
