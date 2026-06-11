// handler/task_test.go
// bufconn を使った gRPC ハンドラーの単体テスト。
//
// bufconn はネットワークを使わないインメモリの net.Listener 実装。
// 実際の gRPC サーバー/クライアントのコードパス（シリアライズ、
// インターセプター、ステータスコード変換など）を通しつつ、
// ポート確保が不要で高速・安定したテストが書ける。
//
// 実行方法:
//   cd step11-advanced-testing/server-go
//   go test ./...
package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	pb "github.com/grpc-learn/step11/server-go/gen/step11"
)

// startBufconnServer は bufconn 上で TaskService サーバーを起動し、
// 接続済みのクライアントとクリーンアップ関数を返すテストヘルパー。
func startBufconnServer(t *testing.T, dialOpts ...grpc.DialOption) pb.TaskServiceClient {
	t.Helper()

	// bufconn.Listen のバッファサイズ（1MB あれば十分）
	lis := bufconn.Listen(1024 * 1024)

	srv := grpc.NewServer()
	pb.RegisterTaskServiceServer(srv, NewTaskServer(slog.Default()))

	go func() {
		if err := srv.Serve(lis); err != nil {
			// テスト終了時の Close によるエラーは無視する
			t.Logf("bufconn サーバー終了: %v", err)
		}
	}()

	// bufconn へのダイヤラー: TCP の代わりにインメモリパイプへ接続する
	opts := append([]grpc.DialOption{
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}, dialOpts...)

	// "passthrough:///bufnet" はダミーのターゲット名（実際の解決はダイヤラーが行う）
	conn, err := grpc.NewClient("passthrough:///bufnet", opts...)
	if err != nil {
		t.Fatalf("bufconn への接続失敗: %v", err)
	}

	t.Cleanup(func() {
		_ = conn.Close()
		srv.Stop()
		_ = lis.Close()
	})

	return pb.NewTaskServiceClient(conn)
}

// TestCreateTask はテーブル駆動テストでバリデーションを検証する。
func TestCreateTask(t *testing.T) {
	client := startBufconnServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tests := []struct {
		name     string
		title    string
		wantCode codes.Code // codes.OK なら成功を期待
	}{
		{name: "正常系: タイトルあり", title: "牛乳を買う", wantCode: codes.OK},
		{name: "異常系: タイトル空", title: "", wantCode: codes.InvalidArgument},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := client.CreateTask(ctx, &pb.CreateTaskRequest{Title: tt.title})

			if got := status.Code(err); got != tt.wantCode {
				t.Fatalf("status code = %v, want %v (err=%v)", got, tt.wantCode, err)
			}
			if tt.wantCode != codes.OK {
				return // エラー期待ケースはここまで
			}

			if resp.Task.Id == "" {
				t.Error("task.id が空です")
			}
			if resp.Task.Title != tt.title {
				t.Errorf("task.title = %q, want %q", resp.Task.Title, tt.title)
			}
			if resp.Task.Done {
				t.Error("作成直後の task.done は false であるべきです")
			}
		})
	}
}

// TestGetTask は作成 → 取得のラウンドトリップと NotFound を検証する。
func TestGetTask(t *testing.T) {
	client := startBufconnServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	created, err := client.CreateTask(ctx, &pb.CreateTaskRequest{Title: "テスト用タスク"})
	if err != nil {
		t.Fatalf("CreateTask 失敗: %v", err)
	}

	t.Run("正常系: 作成したタスクを取得できる", func(t *testing.T) {
		got, err := client.GetTask(ctx, &pb.GetTaskRequest{Id: created.Task.Id})
		if err != nil {
			t.Fatalf("GetTask 失敗: %v", err)
		}
		if got.Task.Title != "テスト用タスク" {
			t.Errorf("task.title = %q, want %q", got.Task.Title, "テスト用タスク")
		}
	})

	t.Run("異常系: 存在しない ID は NotFound", func(t *testing.T) {
		_, err := client.GetTask(ctx, &pb.GetTaskRequest{Id: "task-999"})
		if got := status.Code(err); got != codes.NotFound {
			t.Fatalf("status code = %v, want %v", got, codes.NotFound)
		}
	})
}

// retryServiceConfig はクライアント側リトライポリシー（service config）。
// Unavailable を受け取ったら最大 4 回まで指数バックオフでリトライする。
var retryServiceConfig = mustJSON(map[string]any{
	"methodConfig": []map[string]any{
		{
			"name": []map[string]any{
				{"service": "step11.TaskService"},
			},
			"retryPolicy": map[string]any{
				"maxAttempts":          4, // 初回 + リトライ 3 回
				"initialBackoff":       "0.01s",
				"maxBackoff":           "0.1s",
				"backoffMultiplier":    2.0,
				"retryableStatusCodes": []string{"UNAVAILABLE"},
			},
		},
	},
})

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}

// TestFlakyEchoWithRetry はリトライポリシーの有無で結果が変わることを検証する。
func TestFlakyEchoWithRetry(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	t.Run("リトライなし: 1回目の Unavailable で失敗する", func(t *testing.T) {
		client := startBufconnServer(t)

		_, err := client.FlakyEcho(ctx, &pb.FlakyEchoRequest{
			Message:   "no-retry",
			FailCount: 2,
		})
		if got := status.Code(err); got != codes.Unavailable {
			t.Fatalf("status code = %v, want %v", got, codes.Unavailable)
		}
	})

	t.Run("リトライあり: 自動リトライで最終的に成功する", func(t *testing.T) {
		client := startBufconnServer(t,
			grpc.WithDefaultServiceConfig(retryServiceConfig),
		)

		// 2 回失敗 → 3 回目で成功（maxAttempts=4 の範囲内）
		resp, err := client.FlakyEcho(ctx, &pb.FlakyEchoRequest{
			Message:   "with-retry",
			FailCount: 2,
		})
		if err != nil {
			t.Fatalf("FlakyEcho 失敗（リトライで成功するはず）: %v", err)
		}
		if resp.Attempts != 3 {
			t.Errorf("attempts = %d, want 3（2 回失敗 + 1 回成功）", resp.Attempts)
		}
	})
}
