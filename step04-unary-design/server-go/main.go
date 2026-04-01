// Step 04: Unary 設計デモ サーバー
// FieldMask / ページネーション / Money 型 を実装した ProductService を提供する。
package main

import (
	"log/slog"
	"net"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	pb "github.com/grpc-learn/step04/server-go/gen/step04"
	"github.com/grpc-learn/step04/server-go/handler"
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

	// gRPC サーバーを生成し、ProductService を登録
	server := grpc.NewServer()
	pb.RegisterProductServiceServer(server, handler.NewProductServer(logger))

	// grpcurl などのツールで利用できるようにリフレクションを登録
	reflection.Register(server)

	logger.Info("Go gRPC サーバー起動", "port", 50051)

	// ブロッキングでサーバーを起動
	if err := server.Serve(lis); err != nil {
		logger.Error("Serve 失敗", "error", err)
		os.Exit(1)
	}
}
