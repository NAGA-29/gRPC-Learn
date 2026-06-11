# Step 01 — Protobuf 基礎

サービス定義は持たず、**メッセージ設計**だけに集中するステップです。

---

## 学習内容

- `message` / `enum` / `repeated` / nested message の書き方
- `oneof` による多態的なフィールド
- フィールド番号と後方互換性
- proto スタイルガイドの基本（`UNSPECIFIED = 0` など）

---

## ファイル

| ファイル | 内容 |
|---------|------|
| `proto/step01/user.proto` | User / Address / UserRole |
| `proto/step01/order.proto` | Order / OrderItem / PaymentMethod（oneof） |

---

## 演習: buf コマンドを動かす

```bash
# リポジトリルートで実行

# 1. lint: スタイルガイド違反を確認（--path で step01 だけに絞る）
buf lint --path proto/step01

# 2. build: proto ファイルが正しくコンパイルできるか確認（ワークスペース全体）
buf build

# 3. breaking: 互換性チェック（git の main ブランチと比較）
buf breaking --against '.git#branch=main' --path proto/step01
```

---

## フィールド番号のルール

```protobuf
message Example {
  string id   = 1;  // フィールド番号は変更・再利用 NG（後方互換性が壊れる）
  string name = 2;
  // フィールドを削除する場合は reserved で予約する
  // reserved 3;
  // reserved "old_field";
}
```

フィールド番号 `1〜15` は 1 バイトでエンコードされるため、頻繁に使うフィールドに割り当てる。

---

## enum のスタイルガイド

```protobuf
enum UserRole {
  USER_ROLE_UNSPECIFIED = 0;  // ✅ 必ずゼロ値に UNSPECIFIED を置く
  USER_ROLE_ADMIN       = 1;
  USER_ROLE_MEMBER      = 2;
}
```

- `UNSPECIFIED = 0` がないと、proto3 のデフォルト値（0）が意味を持ってしまう
- 値の前に `ENUM_NAME_` プレフィックスをつける（他の enum との衝突防止）

---

## oneof の特性

```protobuf
message PaymentMethod {
  oneof method {
    CreditCard   credit_card   = 1;
    BankTransfer bank_transfer = 2;
  }
}
```

- `oneof` の中でセットできるのは**1つのフィールドだけ**
- 新しいフィールドをセットすると、他のフィールドは自動的にクリアされる
- メモリ効率が良い（値を 1 つしか保持しない）

---

## 次のステップ

[Step 02](../step02-unary-go-ts/) で、実際に gRPC サーバーとクライアントを動かしてみましょう。
