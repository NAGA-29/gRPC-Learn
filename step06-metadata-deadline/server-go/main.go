// Step 06: Metadata / Deadline サーバー
// MetadataService を実装した gRPC サーバー。
// クライアントから送られたメタデータをエコーバックし、
// delay_ms で意図的に遅延を発生させてデッドライン動作を確認できる。
package main

import (
	"log/slog"
	"net"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	pb "github.com/grpc-learn/step06/server-go/gen/step06"
	"github.com/grpc-learn/step06/server-go/handler"
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

	// gRPC サーバーを生成し、MetadataService を登録
	server := grpc.NewServer()
	pb.RegisterMetadataServiceServer(server, handler.NewMetadataServer(logger))

	// grpcurl などのツールで利用できるようにリフレクションを登録
	reflection.Register(server)

	logger.Info("Go gRPC サーバー起動", "port", 50051)

	// ブロッキングでサーバーを起動
	if err := server.Serve(lis); err != nil {
		logger.Error("Serve 失敗", "error", err)
		os.Exit(1)
	}
}
