// Package observability（続き）: OpenTelemetry トレーシング
//
// # OpenTelemetry トレーシング
//
// OpenTelemetry（OTel）は観測可能性のための業界標準フレームワーク。
// トレーシングでは「スパン」を使ってリクエストの処理フローを追跡する。
//
// このファイルでは学習用に stdout エクスポーターを使い、
// トレースデータをコンソールに出力する。
//
// 本番での典型的なエクスポーター:
//   - Jaeger: オープンソースの分散トレーシングシステム
//   - Zipkin: Twitter 発の分散トレーシング
//   - OTLP:   OpenTelemetry Protocol（Datadog, Grafana Tempo など）
//
// スパンの概念:
//
//	スパン = 一つの処理単位（例: HTTP リクエスト、DB クエリ、gRPC 呼び出し）
//	トレース = 一連のスパンの集まり（例: API リクエストの全処理フロー）
//
//	[Gateway PlaceOrder スパン]
//	  ├── [InventoryService ReserveStock スパン]
//	  └── [NotificationService Notify スパン]
package observability

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// InitTracer は OpenTelemetry のトレーサープロバイダーを初期化する。
//
// 戻り値の cleanup 関数を defer で呼び出すことで、
// バッファされたスパンデータを確実にフラッシュしてシャットダウンする。
//
// 使用例:
//
//	cleanup := observability.InitTracer()
//	defer cleanup()
func InitTracer() (cleanup func()) {
	// stdout エクスポーター: トレースデータを標準出力に JSON 形式で出力する
	// 学習用のため stdout を使うが、本番では Jaeger や OTLP エクスポーターを使う
	exporter, err := stdouttrace.New(
		// PrettyPrint で読みやすい JSON 出力にする（学習用）
		stdouttrace.WithPrettyPrint(),
	)
	if err != nil {
		// トレーシング初期化失敗はサーバー起動を止めない（観測可能性は補助的なもの）
		// 実務では起動失敗にするかどうかはポリシーによる
		otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
			// エラーを無視（stdout エクスポーターなので実際には失敗しない）
			_ = err
		}))
		return func() {}
	}

	// リソース属性: トレースデータにサービス情報を付与する
	// Prometheus のラベルに相当する概念
	res := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName("step10-production-server"),
		semconv.ServiceVersion("1.0.0"),
	)

	// トレーサープロバイダーの作成
	// AlwaysSample: 全リクエストをサンプリング（学習用）
	// 本番では ParentBased(TraceIDRatioBased(0.1)) などで 10% サンプリングにする
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)

	// グローバルトレーサープロバイダーとして登録
	// これにより otel.Tracer("xxx") でどこからでもトレーサーを取得できる
	otel.SetTracerProvider(tp)

	// cleanup 関数: サーバー停止時にスパンをフラッシュしてシャットダウンする
	return func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			// シャットダウンエラーは無視（プロセス終了時なので）
			_ = err
		}
	}
}
