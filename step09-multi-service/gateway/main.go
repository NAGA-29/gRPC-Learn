// Step 09: Gateway サーバー（BFF パターン）
//
// アーキテクチャ:
//   TypeScript クライアント → GatewayService(:50051)
//                              ├─ InventoryService(:50052)  在庫確保
//                              └─ NotificationService(:50053) 通知送信
//
// Gateway は外部クライアントからの PlaceOrder リクエストを受け取り、
// 内部の InventoryService と NotificationService を呼び出してオーケストレーションする。
// これは BFF（Backend For Frontend）または API Gateway パターンと呼ばれる。
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"

	pb "github.com/grpc-learn/step09/gateway/gen/step09"
)

// gatewayServer は GatewayService の実装。
// 内部サービスへの gRPC クライアントを保持する。
type gatewayServer struct {
	pb.UnimplementedGatewayServiceServer
	// inventoryClient は InventoryService への gRPC クライアント
	inventoryClient pb.InventoryServiceClient
	// notificationClient は NotificationService への gRPC クライアント
	notificationClient pb.NotificationServiceClient
}

// PlaceOrder は外部クライアントからの注文リクエストを処理する。
//
// 処理フロー:
//  1. InventoryService.ReserveStock を呼び出し、在庫を確保する
//  2. 在庫確保成功なら order_id を生成する
//  3. NotificationService.Notify を呼び出す（失敗しても警告ログのみ）
//  4. レスポンスを返す
//
// InventoryService のエラーはクライアントに伝播させる。
// NotificationService のエラーは注文完了には影響させない（非同期的扱い）。
func (s *gatewayServer) PlaceOrder(
	ctx context.Context,
	req *pb.PlaceOrderRequest,
) (*pb.PlaceOrderResponse, error) {
	slog.Info("PlaceOrder 開始",
		"user_id", req.UserId,
		"product_id", req.ProductId,
		"quantity", req.Quantity,
	)

	// --- Step 1: 在庫確保 ---
	// order_id は在庫確保後に生成するため、この時点では空文字列を渡す。
	// 実務では事前に仮 order_id を生成して冪等性を確保するケースが多い。
	reserveResp, err := s.inventoryClient.ReserveStock(ctx, &pb.ReserveStockRequest{
		ProductId: req.ProductId,
		Quantity:  req.Quantity,
		OrderId:   "", // 後で生成するため空
	})
	if err != nil {
		slog.Error("InventoryService.ReserveStock 失敗", "error", err)
		// InventoryService のエラーをそのままクライアントに返す
		return nil, fmt.Errorf("在庫確保に失敗しました: %w", err)
	}

	// 在庫不足の場合はビジネスロジックエラーとして返す
	if !reserveResp.Reserved {
		slog.Warn("在庫不足",
			"product_id", req.ProductId,
			"requested", req.Quantity,
			"remaining", reserveResp.RemainingStock,
		)
		return nil, status.Errorf(
			codes.ResourceExhausted,
			"在庫が不足しています (product_id=%s, 残り在庫=%d)",
			req.ProductId, reserveResp.RemainingStock,
		)
	}

	// --- Step 2: order_id 生成 ---
	// uuid パッケージを避けるため time.Now() から簡易 ID を生成する。
	// 実務では github.com/google/uuid などを使うこと。
	orderID := fmt.Sprintf("order-%d-%s", time.Now().UnixNano(), req.ProductId)

	slog.Info("在庫確保成功",
		"order_id", orderID,
		"remaining_stock", reserveResp.RemainingStock,
	)

	// --- Step 3: 通知送信 ---
	// 通知の失敗は注文完了に影響させない。
	// 実務では通知をメッセージキューで非同期化するパターンが多い。
	notifyCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	notifyResp, notifyErr := s.notificationClient.Notify(notifyCtx, &pb.NotifyRequest{
		OrderId: orderID,
		UserId:  req.UserId,
		Channel: "email",
		Message: fmt.Sprintf("ご注文 %s を承りました。商品: %s x %d",
			orderID, req.ProductId, req.Quantity),
	})
	if notifyErr != nil {
		// 通知失敗は警告ログのみ。注文は完了扱いにする。
		slog.Warn("NotificationService.Notify 失敗（警告のみ）", "error", notifyErr)
	} else {
		slog.Info("通知送信成功", "sent", notifyResp.Sent, "sent_at", notifyResp.SentAt)
	}

	// --- Step 4: レスポンス返却 ---
	// estimated_delivery は簡易的に3日後を設定する
	delivery := time.Now().AddDate(0, 0, 3).Format("2006-01-02")

	return &pb.PlaceOrderResponse{
		OrderId:           orderID,
		Status:            "confirmed",
		EstimatedDelivery: delivery,
	}, nil
}

func main() {
	// 構造化ログのセットアップ（JSON 形式で出力）
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	slog.SetDefault(logger)

	// --- 内部サービスへの接続 ---
	// 学習用のため insecure（TLS なし）で接続する。
	// 本番では TLS + サービスメッシュ or 証明書を使うこと。

	// InventoryService クライアントの作成
	invConn, err := grpc.NewClient(
		"localhost:50052",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		slog.Error("InventoryService への接続失敗", "error", err)
		os.Exit(1)
	}
	defer invConn.Close()

	// NotificationService クライアントの作成
	notifConn, err := grpc.NewClient(
		"localhost:50053",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		slog.Error("NotificationService への接続失敗", "error", err)
		os.Exit(1)
	}
	defer notifConn.Close()

	// TCP リスナーを :50051 で作成
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		slog.Error("Listen 失敗", "error", err)
		os.Exit(1)
	}

	// gRPC サーバーの作成
	srv := grpc.NewServer()

	// GatewayService ハンドラーを登録
	pb.RegisterGatewayServiceServer(srv, &gatewayServer{
		inventoryClient:    pb.NewInventoryServiceClient(invConn),
		notificationClient: pb.NewNotificationServiceClient(notifConn),
	})

	// grpcurl などのツールで利用できるようにリフレクションを登録
	reflection.Register(srv)

	slog.Info("Gateway サーバー起動",
		"port", 50051,
		"inventory_addr", "localhost:50052",
		"notification_addr", "localhost:50053",
	)

	// ブロッキングでサーバーを起動
	if err := srv.Serve(lis); err != nil {
		slog.Error("Serve 失敗", "error", err)
		os.Exit(1)
	}
}
