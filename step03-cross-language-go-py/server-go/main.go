package main

import (
	"context"
	"log/slog"
	"net"
	"os"

	pb "github.com/grpc-learn/step03/server-go/gen/step03"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type greeterServer struct {
	pb.UnimplementedGreeterServiceServer
	logger   *slog.Logger
	hostname string
}

func (s *greeterServer) SayHello(ctx context.Context, req *pb.SayHelloRequest) (*pb.SayHelloResponse, error) {
	s.logger.Info("SayHello 受信", "name", req.Name)

	resp := &pb.SayHelloResponse{
		Message:    "こんにちは、" + req.Name + " さん！（Go サーバーより）",
		ServerHost: s.hostname,
	}

	s.logger.Info("SayHello 返信", "message", resp.Message, "host", resp.ServerHost)
	return resp, nil
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	hostname, _ := os.Hostname()

	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		logger.Error("Listen 失敗", "error", err)
		os.Exit(1)
	}

	server := grpc.NewServer()
	pb.RegisterGreeterServiceServer(server, &greeterServer{
		logger:   logger,
		hostname: hostname,
	})
	reflection.Register(server)

	logger.Info("Go gRPC サーバー起動", "port", 50051, "host", hostname)
	logger.Info("Python クライアントで接続: cd client-py && python client.py")

	if err := server.Serve(lis); err != nil {
		logger.Error("Serve 失敗", "error", err)
		os.Exit(1)
	}
}
