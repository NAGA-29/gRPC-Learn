// interceptor/logging.go
// ロギングインターセプター。
// すべての Unary RPC に対して、リクエスト開始・終了時に zap でログを出力する。
// method 名、処理時間（ms）、gRPC ステータスコードを構造化ログとして記録する。
package interceptor

import (
	"context"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// NewLoggingInterceptor は zap.Logger を受け取り、Unary RPC 用のロギングインターセプターを返す。
// インターセプターチェーンの先頭に配置することで、すべてのリクエスト/レスポンスを記録できる。
func NewLoggingInterceptor(logger *zap.Logger) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		// リクエスト開始ログ
		start := time.Now()
		logger.Info("RPC 開始",
			zap.String("method", info.FullMethod),
		)

		// 次のハンドラー（またはインターセプター）へ処理を委譲する
		resp, err := handler(ctx, req)

		// 処理時間と結果コードを計算する
		duration := time.Since(start).Milliseconds()
		code := codes.OK
		if err != nil {
			code = status.Code(err)
		}

		// リクエスト終了ログ（エラーがあれば Warn レベルで出力）
		if err != nil {
			logger.Warn("RPC 終了（エラーあり）",
				zap.String("method", info.FullMethod),
				zap.Int64("duration_ms", duration),
				zap.String("code", code.String()),
				zap.Error(err),
			)
		} else {
			logger.Info("RPC 終了",
				zap.String("method", info.FullMethod),
				zap.Int64("duration_ms", duration),
				zap.String("code", code.String()),
			)
		}

		return resp, err
	}
}
