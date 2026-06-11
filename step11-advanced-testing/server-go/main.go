// Step 11: 応用編 — テスト & リトライ デモ サーバー
// TaskService を提供する。FlakyEcho でクライアント側リトライの動作を確認できる。
package main

import (
	"log/slog"
	"net"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	pb "github.com/grpc-learn/step11/server-go/gen/step11"
	"github.com/grpc-learn/step11/server-go/handler"
)

func main() {
	// 構造化ログの初期化
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// TCP リスナーを :50051 で作成
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		logger.Error("Listen 失敗", "error", err)
		os.Exit(1)
	}

	// gRPC サーバーを生成し、TaskService を登録
	server := grpc.NewServer()
	pb.RegisterTaskServiceServer(server, handler.NewTaskServer(logger))

	// grpcurl などのツールで利用できるようにリフレクションを登録
	reflection.Register(server)

	logger.Info("Go gRPC テスト&リトライ デモサーバー起動", "port", 50051)
	logger.Info("クライアント実行: cd ../client-go && go run .")

	// ブロッキングでサーバーを起動
	if err := server.Serve(lis); err != nil {
		logger.Error("Serve 失敗", "error", err)
		os.Exit(1)
	}
}
