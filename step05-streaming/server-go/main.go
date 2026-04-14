// Step 05: ストリーミング デモ サーバー
// サーバーストリーミング / クライアントストリーミング / 双方向ストリーミング を実装した
// StreamService を提供する。
package main

import (
	"log/slog"
	"net"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	pb "github.com/grpc-learn/step05/server-go/gen/step05"
	"github.com/grpc-learn/step05/server-go/handler"
)

func main() {
	// 構造化ログの初期化
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// TCP リスナーを :50052 で作成（step04 と別ポートを使用）
	lis, err := net.Listen("tcp", ":50052")
	if err != nil {
		logger.Error("Listen 失敗", "error", err)
		os.Exit(1)
	}

	// gRPC サーバーを生成し、StreamService を登録
	server := grpc.NewServer()
	pb.RegisterStreamServiceServer(server, handler.NewStreamServer(logger))

	// grpcurl などのツールで利用できるようにリフレクションを登録
	reflection.Register(server)

	logger.Info("Go gRPC ストリーミングサーバー起動", "port", 50052)

	// ブロッキングでサーバーを起動
	if err := server.Serve(lis); err != nil {
		logger.Error("Serve 失敗", "error", err)
		os.Exit(1)
	}
}
