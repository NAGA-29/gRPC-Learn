// Step 08: Interceptor サーバー
// logging → auth → metrics の順にインターセプターをチェーンした gRPC サーバー。
// log/slog の代わりに go.uber.org/zap を採用し、構造化ログの本番的なパターンを示す。
package main

import (
	"net"
	"os"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	pb "github.com/grpc-learn/step08/server-go/gen/step08"
	"github.com/grpc-learn/step08/server-go/handler"
	"github.com/grpc-learn/step08/server-go/interceptor"
)

func main() {
	// zap.NewDevelopment() は開発用のカラー付き読みやすいフォーマットでログを出力する。
	// 本番では zap.NewProduction()（JSON 形式）を使うのが一般的。
	logger, err := zap.NewDevelopment()
	if err != nil {
		// zap の初期化失敗は起動不能なのでそのまま終了する
		panic("zap 初期化失敗: " + err.Error())
	}
	// プログラム終了時にバッファされたログをフラッシュする
	defer logger.Sync() //nolint:errcheck

	// TCP リスナーを :50051 で作成
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		logger.Fatal("Listen 失敗", zap.Error(err))
	}

	// --- インターセプターの準備 ---

	// ロギングインターセプター: すべての RPC のリクエスト/レスポンスを記録する
	loggingInterceptor := interceptor.NewLoggingInterceptor(logger)

	// 認証インターセプター: GetSecret メソッドのみ Bearer トークンを検証する
	authInterceptor := interceptor.NewAuthInterceptor()

	// メトリクスインターセプター: メソッドごとのリクエスト数・エラー数を集計する
	metricsInterceptor := interceptor.NewMetricsInterceptor()

	// --- インターセプターのチェーン順序 ---
	// ChainUnaryInterceptor は引数の順に「外から内へ」ラップする。
	// 実行順: logging（開始ログ）→ auth（トークン検証）→ metrics（カウント）→ ハンドラー
	//         ← metrics（カウント更新）← auth（スキップ）← logging（終了ログ）
	//
	// logging を先頭に置く理由: エラーを含むすべての RPC を確実に記録するため。
	// auth を logging の後・metrics の前に置く理由: 不正リクエストをハンドラーに届ける前に弾くため。
	server := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			loggingInterceptor,
			authInterceptor,
			metricsInterceptor.Unary(),
		),
	)

	// InterceptorService ハンドラーを登録する
	pb.RegisterInterceptorServiceServer(server, handler.NewInterceptorServer(logger))

	// grpcurl などのツールで利用できるようにリフレクションを登録する
	reflection.Register(server)

	logger.Info("Go gRPC インターセプターサーバー起動",
		zap.Int("port", 50051),
		zap.Strings("interceptors", []string{"logging", "auth", "metrics"}),
	)

	// ブロッキングでサーバーを起動する
	if err := server.Serve(lis); err != nil {
		logger.Fatal("Serve 失敗", zap.Error(err))
		os.Exit(1)
	}
}
