package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"time"

	pb "github.com/grpc-learn/step02/server-go/gen/step02"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// greeterServer は GreeterService の実装
type greeterServer struct {
	pb.UnimplementedGreeterServiceServer
	logger *slog.Logger
}

// SayHello: Unary RPC の実装
func (s *greeterServer) SayHello(ctx context.Context, req *pb.SayHelloRequest) (*pb.SayHelloResponse, error) {
	s.logger.Info("SayHello リクエスト受信", "name", req.Name, "language", req.Language)

	msg := buildGreeting(req.Name, req.Language)

	resp := &pb.SayHelloResponse{
		Message:    msg,
		ServerTime: time.Now().Format(time.RFC3339),
	}

	s.logger.Info("SayHello レスポンス送信", "message", resp.Message)
	return resp, nil
}

// buildGreeting は言語コードに応じた挨拶文を生成する
func buildGreeting(name, language string) string {
	switch language {
	case "ja":
		return fmt.Sprintf("こんにちは、%s さん！", name)
	case "es":
		return fmt.Sprintf("¡Hola, %s!", name)
	default: // "en" またはその他
		return fmt.Sprintf("Hello, %s!", name)
	}
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	port := "50051"
	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		logger.Error("Listen 失敗", "error", err)
		os.Exit(1)
	}

	server := grpc.NewServer()
	pb.RegisterGreeterServiceServer(server, &greeterServer{logger: logger})

	// gRPC リフレクションを登録（grpcurl でサービス一覧を確認できる）
	reflection.Register(server)

	logger.Info("gRPC サーバー起動", "port", port)
	logger.Info("grpcurl でテスト: grpcurl -plaintext localhost:50051 list")

	if err := server.Serve(lis); err != nil {
		logger.Error("Serve 失敗", "error", err)
		os.Exit(1)
	}
}
