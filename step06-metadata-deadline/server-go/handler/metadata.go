// handler/metadata.go
// MetadataService の実装。
// クライアントから送られたメタデータを受け取り、レスポンスにそのまま返す。
// delay_ms 分だけ sleep することでデッドライン（タイムアウト）動作を確認できる。
package handler

import (
	"context"
	"log/slog"
	"time"

	"google.golang.org/grpc/metadata"

	pb "github.com/grpc-learn/step06/server-go/gen/step06"
)

// MetadataServer は MetadataService の gRPC ハンドラー実装。
type MetadataServer struct {
	pb.UnimplementedMetadataServiceServer
	logger *slog.Logger
}

// NewMetadataServer は MetadataServer を初期化して返す。
func NewMetadataServer(logger *slog.Logger) *MetadataServer {
	return &MetadataServer{logger: logger}
}

// Echo はクライアントのメタデータを受け取り、delay_ms 後にエコーバックする。
// クライアントがデッドラインを設定している場合、delay_ms > deadline のとき
// context がキャンセルされ、クライアント側でタイムアウトエラーが発生する。
func (s *MetadataServer) Echo(ctx context.Context, req *pb.EchoRequest) (*pb.EchoResponse, error) {
	s.logger.Info("Echo 受信", "message", req.Message, "delay_ms", req.DelayMs)

	// リクエストのメタデータを取得する（HTTP ヘッダー相当）
	// metadata.FromIncomingContext は gRPC メタデータを map[string][]string として返す
	md, ok := metadata.FromIncomingContext(ctx)
	if ok {
		s.logger.Info("受信メタデータ", "metadata", md)
	}

	// delay_ms > 0 の場合は意図的に sleep する（デッドラインテスト用）
	if req.DelayMs > 0 {
		delay := time.Duration(req.DelayMs) * time.Millisecond
		s.logger.Info("スリープ開始", "delay_ms", req.DelayMs)

		select {
		case <-time.After(delay):
			// 正常に sleep 完了
			s.logger.Info("スリープ完了")
		case <-ctx.Done():
			// クライアントがデッドラインを超えてキャンセルした場合
			s.logger.Warn("Context キャンセル検出（デッドライン超過）", "error", ctx.Err())
			return nil, ctx.Err()
		}
	}

	// context がキャンセルされていないか確認する
	if err := ctx.Err(); err != nil {
		s.logger.Warn("Context エラー検出", "error", err)
		return nil, err
	}

	// 受け取ったメタデータをフラットな map[string]string に変換して返す
	// gRPC メタデータは同一キーに複数の値を持てるが、ここでは最初の値のみ使用する
	receivedMetadata := make(map[string]string)
	for key, values := range md {
		if len(values) > 0 {
			receivedMetadata[key] = values[0]
		}
	}

	processedAt := time.Now().Format(time.RFC3339)
	s.logger.Info("Echo 返信", "processed_at", processedAt)

	return &pb.EchoResponse{
		Message:          req.Message,
		ReceivedMetadata: receivedMetadata,
		ProcessedAt:      processedAt,
	}, nil
}
