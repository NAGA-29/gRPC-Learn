// Step 11: 応用編 — Go gRPC クライアント（リトライポリシー / デッドラインの実演）
//
// 実演内容:
//   1. リトライポリシーなしで FlakyEcho → 1 回目の Unavailable で即失敗
//   2. リトライポリシーありで FlakyEcho → 自動リトライで成功
//   3. デッドライン付きで FlakyEcho → リトライ中にデッドライン超過
//
// 前提: Go サーバーが localhost:50051 で起動していること
//   cd ../server-go && go run .
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	pb "github.com/grpc-learn/step11/client-go/gen/step11"
)

// retryServiceConfig はクライアント側リトライポリシー（service config）。
//
// gRPC のリトライは「クライアントの接続設定」として JSON で宣言する。
// アプリケーションコードに for ループを書かなくても、
// retryableStatusCodes に該当するエラーを自動でリトライしてくれる。
const retryServiceConfig = `{
  "methodConfig": [{
    "name": [{"service": "step11.TaskService"}],
    "retryPolicy": {
      "maxAttempts": 4,
      "initialBackoff": "0.1s",
      "maxBackoff": "1s",
      "backoffMultiplier": 2.0,
      "retryableStatusCodes": ["UNAVAILABLE"]
    }
  }]
}`

func main() {
	fmt.Println("=== Step 11: リトライポリシー / デッドライン デモ ===")
	fmt.Println()

	// ----------------------------------------------------------------
	// 1. リトライポリシーなしのクライアント
	// ----------------------------------------------------------------
	fmt.Println("--- 1. リトライなし（fail_count=2 → 即失敗） ---")

	plainConn, err := grpc.NewClient(
		"localhost:50051",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		fmt.Println("接続失敗:", err)
		os.Exit(1)
	}
	defer plainConn.Close()
	plainClient := pb.NewTaskServiceClient(plainConn)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err = plainClient.FlakyEcho(ctx, &pb.FlakyEchoRequest{
		Message:   fmt.Sprintf("no-retry-%d", time.Now().UnixNano()), // 毎回ユニークにして試行回数をリセット
		FailCount: 2,
	})
	if err != nil {
		st, _ := status.FromError(err)
		fmt.Printf("  期待通りのエラー: code=%s message=%s\n", st.Code(), st.Message())
	} else {
		fmt.Println("  ※ エラーが発生しませんでした（予期しない結果）")
	}
	fmt.Println()

	// ----------------------------------------------------------------
	// 2. リトライポリシーありのクライアント
	// ----------------------------------------------------------------
	fmt.Println("--- 2. リトライあり（fail_count=2 → 3 回目で自動成功） ---")

	retryConn, err := grpc.NewClient(
		"localhost:50051",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultServiceConfig(retryServiceConfig),
	)
	if err != nil {
		fmt.Println("接続失敗:", err)
		os.Exit(1)
	}
	defer retryConn.Close()
	retryClient := pb.NewTaskServiceClient(retryConn)

	resp, err := retryClient.FlakyEcho(ctx, &pb.FlakyEchoRequest{
		Message:   fmt.Sprintf("with-retry-%d", time.Now().UnixNano()),
		FailCount: 2,
	})
	if err != nil {
		fmt.Println("  失敗（予期しない結果）:", err)
	} else {
		fmt.Printf("  成功: message=%q attempts=%d（2 回失敗 + 1 回成功）\n", resp.Message, resp.Attempts)
		fmt.Println("  → アプリケーションコードにループを書かずに自動リトライされた")
	}
	fmt.Println()

	// ----------------------------------------------------------------
	// 3. リトライ + デッドラインの組み合わせ
	// リトライ中でもデッドラインは全体に適用される。
	// fail_count を大きくしてリトライ上限（maxAttempts=4）を超えさせる。
	// ----------------------------------------------------------------
	fmt.Println("--- 3. リトライ上限超え（fail_count=10 → maxAttempts=4 で失敗） ---")

	_, err = retryClient.FlakyEcho(ctx, &pb.FlakyEchoRequest{
		Message:   fmt.Sprintf("too-flaky-%d", time.Now().UnixNano()),
		FailCount: 10,
	})
	if err != nil {
		st, _ := status.FromError(err)
		fmt.Printf("  期待通りのエラー: code=%s message=%s\n", st.Code(), st.Message())
		fmt.Println("  → 4 回（初回 + リトライ 3 回）試行してもすべて Unavailable のため失敗")
	} else {
		fmt.Println("  ※ 成功してしまいました（予期しない結果）")
	}
	fmt.Println()

	fmt.Println("=== 完了 ===")
	fmt.Println("単体テストの実行: cd ../server-go && go test ./... -v")
}
