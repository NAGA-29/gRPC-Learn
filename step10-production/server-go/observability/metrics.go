// Package observability は Prometheus メトリクスと OpenTelemetry トレーシングを提供する。
//
// # Prometheus メトリクス
//
// このファイルでは prometheus/client_golang を使って2種類のメトリクスを定義する:
//
//   - grpc_requests_total (Counter): gRPC リクエストの累積数
//     ラベル: method（メソッド名で集計可能）
//
//   - grpc_active_connections (Gauge): 現在のアクティブ接続数
//     インターセプターや接続フックから増減させる
//
// メトリクスの確認:
//
//	curl http://localhost:8080/metrics | grep grpc_
package observability

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// grpcRequestsTotal は gRPC リクエストの累積カウンターを定義する。
// method ラベルでメソッドごとの呼び出し数を集計できる。
//
// Prometheus クエリ例:
//
//	rate(grpc_requests_total[5m])  # 5 分間の RPS
//	sum by (method)(grpc_requests_total)  # メソッドごとの合計
var grpcRequestsTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "grpc_requests_total",
		Help: "gRPC リクエストの累積数",
	},
	[]string{"method"}, // ラベル名
)

// grpcActiveConnections は現在のアクティブ接続数のゲージを定義する。
// 接続開始時に Inc()、切断時に Dec() を呼び出す。
//
// Prometheus クエリ例:
//
//	grpc_active_connections  # 現在のアクティブ接続数
var grpcActiveConnections = promauto.NewGauge(
	prometheus.GaugeOpts{
		Name: "grpc_active_connections",
		Help: "現在のアクティブな gRPC 接続数",
	},
)

// メトリクス値をキャッシュするための内部状態
var (
	mu           sync.RWMutex
	requestCounts = make(map[string]float64)
	activeConns   float64
)

// RecordRequest は指定したメソッドのリクエスト数を1増やす。
// サーバーインターセプターから各 RPC 呼び出し時に呼び出す。
//
// 使用例:
//
//	func (s *server) Check(ctx context.Context, req *pb.HealthCheckRequest) (...) {
//	    observability.RecordRequest("Check")
//	    ...
//	}
func RecordRequest(method string) {
	// Prometheus カウンターをインクリメント
	grpcRequestsTotal.WithLabelValues(method).Inc()

	// 内部キャッシュも更新（GetMetrics RPC で返すため）
	mu.Lock()
	requestCounts[method]++
	mu.Unlock()
}

// IncrementConnections はアクティブ接続数を1増やす。
// gRPC サーバーの接続フック（grpc.StatsHandler）から呼び出す。
func IncrementConnections() {
	grpcActiveConnections.Inc()

	mu.Lock()
	activeConns++
	mu.Unlock()
}

// DecrementConnections はアクティブ接続数を1減らす。
func DecrementConnections() {
	grpcActiveConnections.Dec()

	mu.Lock()
	if activeConns > 0 {
		activeConns--
	}
	mu.Unlock()
}

// GetCurrentMetrics は現在のメトリクス値を counters と gauges のマップで返す。
// GetMetrics RPC のレスポンス生成に使用する。
func GetCurrentMetrics() (counters map[string]float64, gauges map[string]float64) {
	mu.RLock()
	defer mu.RUnlock()

	// カウンターのコピー
	counters = make(map[string]float64, len(requestCounts))
	for k, v := range requestCounts {
		counters[k] = v
	}

	// ゲージのマップ
	gauges = map[string]float64{
		"active_connections": activeConns,
	}

	return counters, gauges
}
