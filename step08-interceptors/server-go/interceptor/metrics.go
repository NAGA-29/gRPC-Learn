// interceptor/metrics.go
// メトリクスインターセプター（学習用のシンプルなカウンター実装）。
// sync.Mutex + map でリクエスト数・エラー数をメソッド単位で集計する。
// 本番環境では Prometheus の prometheus/client_golang を使い、
// /metrics エンドポイントで Grafana 等に公開するのが一般的。
package interceptor

import (
	"context"
	"sync"

	"google.golang.org/grpc"
)

// MethodMetrics は 1 メソッド分のカウンター。
type MethodMetrics struct {
	// RequestCount はそのメソッドへの総リクエスト数。
	RequestCount int64
	// ErrorCount はエラーが返ったリクエスト数。
	ErrorCount int64
}

// MetricsInterceptor はリクエスト数・エラー数を集計するインターセプター。
// インスタンスを保持することで GetMetrics() でカウンターを参照できる。
type MetricsInterceptor struct {
	mu      sync.Mutex
	metrics map[string]*MethodMetrics
}

// NewMetricsInterceptor は MetricsInterceptor を初期化して返す。
func NewMetricsInterceptor() *MetricsInterceptor {
	return &MetricsInterceptor{
		metrics: make(map[string]*MethodMetrics),
	}
}

// Unary は grpc.UnaryServerInterceptor として使用できるメソッドを返す。
// 次のハンドラーの呼び出し前後でカウンターを更新する。
func (m *MetricsInterceptor) Unary() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		// 次のハンドラーへ委譲（実際の RPC 処理を行う）
		resp, err := handler(ctx, req)

		// カウンターを更新する（排他制御）
		m.mu.Lock()
		defer m.mu.Unlock()

		if _, exists := m.metrics[info.FullMethod]; !exists {
			m.metrics[info.FullMethod] = &MethodMetrics{}
		}

		m.metrics[info.FullMethod].RequestCount++
		if err != nil {
			m.metrics[info.FullMethod].ErrorCount++
		}

		return resp, err
	}
}

// GetMetrics はメソッドごとのカウンターのスナップショットを返す。
// 呼び出し時点でのカウンターをコピーして返すため、並行アクセスに安全。
func (m *MetricsInterceptor) GetMetrics() map[string]MethodMetrics {
	m.mu.Lock()
	defer m.mu.Unlock()

	// スナップショットをコピーして返す（元の map の参照を外部に渡さない）
	snapshot := make(map[string]MethodMetrics, len(m.metrics))
	for method, mc := range m.metrics {
		snapshot[method] = *mc
	}
	return snapshot
}
