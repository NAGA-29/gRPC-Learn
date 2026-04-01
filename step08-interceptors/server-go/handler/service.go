// handler/service.go
// InterceptorService の実装。
// 認証・ロギング・メトリクスはすべてインターセプター層が担うため、
// ハンドラーはビジネスロジック（レスポンスの生成）のみに集中できる。
package handler

import (
	"context"
	"fmt"
	"os"

	"go.uber.org/zap"

	pb "github.com/grpc-learn/step08/server-go/gen/step08"
)

// InterceptorServer は InterceptorService の gRPC ハンドラー実装。
type InterceptorServer struct {
	pb.UnimplementedInterceptorServiceServer
	logger   *zap.Logger
	serverID string // このサーバーインスタンスの識別子
}

// NewInterceptorServer は InterceptorServer を初期化して返す。
// serverID はホスト名から取得し、レスポンスに埋め込む。
func NewInterceptorServer(logger *zap.Logger) *InterceptorServer {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}
	return &InterceptorServer{
		logger:   logger,
		serverID: fmt.Sprintf("server-%s", hostname),
	}
}

// Ping は公開エンドポイント（認証不要）。
// "pong: <message>" を返し、server_id を付加する。
// latency_ms は logging インターセプターが計測するため、ここでは 0 を返す。
func (s *InterceptorServer) Ping(ctx context.Context, req *pb.PingRequest) (*pb.PingResponse, error) {
	s.logger.Debug("Ping ハンドラー呼び出し", zap.String("message", req.Message))

	return &pb.PingResponse{
		Message:  "pong: " + req.Message,
		ServerId: s.serverID,
		// LatencyMs は logging インターセプターで計測する（ここでは設定しない）
		LatencyMs: 0,
	}, nil
}

// GetSecret は保護エンドポイント（要 Authorization ヘッダー）。
// auth インターセプターがトークン検証を済ませているため、
// ここに到達した時点で認証は成功している。
func (s *InterceptorServer) GetSecret(ctx context.Context, req *pb.GetSecretRequest) (*pb.GetSecretResponse, error) {
	s.logger.Debug("GetSecret ハンドラー呼び出し（認証済み）")

	return &pb.GetSecretResponse{
		SecretData: "機密情報: TOP_SECRET_DATA_XYZ_2024",
	}, nil
}
