// Package health は gRPC ヘルスチェックの実装を提供する。
//
// # gRPC ヘルスチェックプロトコル（grpc.health.v1）
//
// gRPC の標準ヘルスチェックプロトコルは GRPC Health Checking Protocol として
// 仕様が定められています（https://github.com/grpc/grpc/blob/master/doc/health-checking.md）。
//
// 標準プロトコルを使う利点:
//   - Kubernetes の grpc livenessProbe/readinessProbe と互換
//   - Envoy などのサービスメッシュのヘルスチェックと互換
//   - grpc_health_probe ツールで簡単に確認できる
//
// 標準実装（google.golang.org/grpc/health）も利用可能ですが、
// ここではプロトコルの理解を深めるために独自実装しています。
//
// ServingStatus の意味:
//   - UNKNOWN     (0): ステータス不明（起動直後など）
//   - SERVING     (1): 正常稼働中、リクエストを受け付けられる
//   - NOT_SERVING (2): 一時的にサービス不能（デプロイ中、過負荷など）
package health

import (
	"context"
	"time"

	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/grpc-learn/step10/server-go/gen/step10"
)

// Checker はヘルスチェックのロジックを提供する。
// 実際のアプリケーションでは、データベース接続や依存サービスの疎通確認を行う。
type Checker struct {
	logger *zap.Logger
	// startTime はサーバーの起動時刻を記録する（稼働時間の計算用）
	startTime time.Time
}

// NewChecker は新しい Checker を作成する。
func NewChecker(logger *zap.Logger) *Checker {
	return &Checker{
		logger:    logger,
		startTime: time.Now(),
	}
}

// Check はサービスの稼働状況を確認し、HealthCheckResponse を返す。
//
// このシンプル実装では常に SERVING を返すが、
// 実務では以下のチェックを追加する:
//   - データベース接続の確認（SELECT 1 など）
//   - 依存する内部サービスへの疎通確認
//   - メモリ使用量のしきい値チェック
//   - ディスク空き容量のチェック
func (c *Checker) Check(
	ctx context.Context,
	req *pb.HealthCheckRequest,
) (*pb.HealthCheckResponse, error) {
	// サービス名でチェック対象を切り替える（空文字列は全体のヘルスを示す）
	serviceName := req.Service
	if serviceName == "" {
		serviceName = "ProductionService"
	}

	// 稼働時間を計算
	uptime := time.Since(c.startTime).Round(time.Second)

	c.logger.Debug("ヘルスチェック実行",
		zap.String("service", serviceName),
		zap.Duration("uptime", uptime),
	)

	// ここでは常に SERVING を返す（シンプル実装）
	// 実務では依存サービスのチェックを行い、問題があれば NOT_SERVING を返す
	return &pb.HealthCheckResponse{
		Status:    pb.HealthCheckResponse_SERVING,
		Message:   "稼働中 (uptime: " + uptime.String() + ")",
		CheckedAt: timestamppb.Now(),
	}, nil
}
