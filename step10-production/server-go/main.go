// Step 10: 本番対応 Go gRPC サーバー
//
// 本番環境での gRPC サーバー運用に必要な要素を実装する:
//   - TLS の有効/無効切り替え（TLS_ENABLED 環境変数）
//   - zap による構造化ログ
//   - Prometheus メトリクスエンドポイント（:8080/metrics）
//   - OpenTelemetry トレーシング（stdout エクスポーター）
//   - グレースフルシャットダウン（SIGINT/SIGTERM シグナル処理）
//   - gRPC リフレクション登録
package main

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"

	pb "github.com/grpc-learn/step10/server-go/gen/step10"
	"github.com/grpc-learn/step10/server-go/health"
	"github.com/grpc-learn/step10/server-go/observability"
)

// productionServer は ProductionService の実装。
// Check と GetMetrics の2つの RPC を提供する。
type productionServer struct {
	pb.UnimplementedProductionServiceServer
	logger  *zap.Logger
	checker *health.Checker
}

// Check はヘルスチェックリクエストを処理する。
// grpc.health.v1 プロトコルに準拠した実装を health パッケージに委譲する。
func (s *productionServer) Check(
	ctx context.Context,
	req *pb.HealthCheckRequest,
) (*pb.HealthCheckResponse, error) {
	// メトリクスを記録
	observability.RecordRequest("Check")

	s.logger.Info("ヘルスチェックリクエスト", zap.String("service", req.Service))
	return s.checker.Check(ctx, req)
}

// GetMetrics は Prometheus で収集したメトリクスを gRPC レスポンスとして返す。
// クライアントがメトリクスを直接取得したい場合に利用する。
// 通常は Prometheus サーバーが :8080/metrics をスクレイプする。
func (s *productionServer) GetMetrics(
	ctx context.Context,
	req *pb.MetricsRequest,
) (*pb.MetricsResponse, error) {
	observability.RecordRequest("GetMetrics")

	s.logger.Info("メトリクスリクエスト")

	// observability パッケージからメトリクス値を取得
	counters, gauges := observability.GetCurrentMetrics()

	return &pb.MetricsResponse{
		Counters: counters,
		Gauges:   gauges,
	}, nil
}

func main() {
	// --- zap ロガーの初期化 ---
	// 本番用（JSON 形式、INFO レベル以上）
	logger, err := zap.NewProduction()
	if err != nil {
		panic("zap 初期化失敗: " + err.Error())
	}
	defer logger.Sync() //nolint:errcheck

	// --- OpenTelemetry トレーシングの初期化 ---
	// InitTracer は cleanup 関数を返す。defer で確実にフラッシュする。
	cleanup := observability.InitTracer()
	defer cleanup()

	// --- TLS 設定 ---
	// TLS_ENABLED=true の場合、TLS ありで起動する。
	// certs/ ディレクトリに server.crt と server.key が必要（generate.sh で生成）。
	tlsEnabled := os.Getenv("TLS_ENABLED") == "true"

	var serverOpts []grpc.ServerOption

	if tlsEnabled {
		logger.Info("TLS モードで起動します",
			zap.String("cert", "certs/server.crt"),
			zap.String("key", "certs/server.key"),
		)
		// 証明書と秘密鍵を読み込む
		cert, err := tls.LoadX509KeyPair("certs/server.crt", "certs/server.key")
		if err != nil {
			logger.Fatal("TLS 証明書の読み込み失敗",
				zap.Error(err),
				zap.String("hint", "cd certs && bash generate.sh で証明書を生成してください"),
			)
		}
		tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{cert},
			// TLS 1.2 以上を要求（セキュリティのベストプラクティス）
			MinVersion: tls.VersionTLS12,
		}
		serverOpts = append(serverOpts, grpc.Creds(credentials.NewTLS(tlsConfig)))
	} else {
		logger.Info("Insecure モードで起動します（TLS_ENABLED=true で TLS 有効化）")
		// 学習用のため insecure で起動。本番では TLS を使うこと。
		_ = insecure.NewCredentials() // insecure パッケージの使用を示すためのコメント
	}

	// --- TCP リスナー作成 ---
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		logger.Fatal("Listen 失敗", zap.Error(err))
	}

	// --- gRPC サーバーの作成 ---
	srv := grpc.NewServer(serverOpts...)

	// ProductionService ハンドラーを登録
	pb.RegisterProductionServiceServer(srv, &productionServer{
		logger:  logger,
		checker: health.NewChecker(logger),
	})

	// grpcurl などのツールで利用できるようにリフレクションを登録
	reflection.Register(srv)

	// --- Prometheus メトリクスエンドポイントを別ポートで公開 ---
	// gRPC サーバー（:50051）とは独立した HTTP サーバー（:8080）で公開する。
	// Prometheus サーバーは http://localhost:8080/metrics をスクレイプする。
	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
			// Kubernetes の livenessProbe / readinessProbe 用
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		})
		httpServer := &http.Server{
			Addr:         ":8080",
			Handler:      mux,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
		}
		logger.Info("Prometheus メトリクスエンドポイント起動", zap.String("addr", "http://localhost:8080/metrics"))
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP サーバーエラー", zap.Error(err))
		}
	}()

	// --- シグナルハンドリング（グレースフルシャットダウン） ---
	// SIGINT（Ctrl+C）または SIGTERM（コンテナ停止シグナル）を受信したら
	// 進行中のリクエストを完了させてからサーバーを停止する。
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// gRPC サーバーをバックグラウンドで起動
	go func() {
		logger.Info("本番対応 gRPC サーバー起動",
			zap.Int("port", 50051),
			zap.Bool("tls", tlsEnabled),
		)
		if err := srv.Serve(lis); err != nil {
			logger.Error("Serve 終了", zap.Error(err))
		}
	}()

	// シグナルを待つ
	sig := <-quit
	logger.Info("シグナル受信、グレースフルシャットダウン開始",
		zap.String("signal", sig.String()),
	)

	// GracefulStop はすべての進行中の RPC が完了するまで待つ。
	// 新規リクエストの受付は即座に停止する。
	srv.GracefulStop()

	logger.Info("サーバーのシャットダウン完了")
}
