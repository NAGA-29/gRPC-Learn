// Step 09: InventoryService サーバー（在庫管理内部サービス）
//
// Gateway からのみ呼ばれる内部サービス。
// インメモリの在庫マップで商品在庫を管理し、ReserveStock リクエストに応答する。
//
// 実務では:
//   - データベース（PostgreSQL など）で永続化する
//   - 楽観的ロックや在庫テーブルのロウロックで同時実行を制御する
//   - イベントソーシングで在庫変動履歴を管理する
package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"

	pb "github.com/grpc-learn/step09/inventory-svc/gen/step09"
)

// inventoryServer は InventoryService の実装。
// mu で在庫マップへのアクセスを保護する（並行リクエスト対応）。
type inventoryServer struct {
	pb.UnimplementedInventoryServiceServer
	mu    sync.Mutex
	stock map[string]int32 // product_id → 在庫数
}

// newInventoryServer は初期在庫を持つサーバーを作成する。
func newInventoryServer() *inventoryServer {
	return &inventoryServer{
		stock: map[string]int32{
			"product-A": 100,
			"product-B": 50,
			"product-C": 10,
			"product-D": 0, // 在庫なし（テスト用）
		},
	}
}

// ReserveStock は在庫を確保する。
//
// 在庫が足りる場合:
//   - reserved = true
//   - 在庫数を減算する
//   - remaining_stock に残り在庫を返す
//
// 在庫が足りない場合:
//   - reserved = false
//   - 在庫数は変更しない
//   - remaining_stock に現在の在庫数を返す
//
// 存在しない product_id の場合は NotFound エラーを返す。
func (s *inventoryServer) ReserveStock(
	ctx context.Context,
	req *pb.ReserveStockRequest,
) (*pb.ReserveStockResponse, error) {
	if req.ProductId == "" {
		return nil, status.Error(codes.InvalidArgument, "product_id は必須です")
	}
	if req.Quantity <= 0 {
		return nil, status.Error(codes.InvalidArgument, "quantity は 1 以上が必要です")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// 在庫マップに商品が存在するか確認
	current, exists := s.stock[req.ProductId]
	if !exists {
		slog.Warn("存在しない商品の在庫確保リクエスト", "product_id", req.ProductId)
		return nil, status.Errorf(codes.NotFound, "商品が見つかりません: %s", req.ProductId)
	}

	slog.Info("ReserveStock リクエスト",
		"product_id", req.ProductId,
		"requested", req.Quantity,
		"current_stock", current,
		"order_id", req.OrderId,
	)

	// 在庫が足りるか確認
	if current < req.Quantity {
		slog.Warn("在庫不足",
			"product_id", req.ProductId,
			"requested", req.Quantity,
			"available", current,
		)
		return &pb.ReserveStockResponse{
			Reserved:       false,
			RemainingStock: current,
		}, nil
	}

	// 在庫を減算して確保
	s.stock[req.ProductId] = current - req.Quantity

	slog.Info("在庫確保成功",
		"product_id", req.ProductId,
		"reserved", req.Quantity,
		"remaining", s.stock[req.ProductId],
	)

	return &pb.ReserveStockResponse{
		Reserved:       true,
		RemainingStock: s.stock[req.ProductId],
	}, nil
}

func main() {
	// 構造化ログのセットアップ
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	slog.SetDefault(logger)

	// TCP リスナーを :50052 で作成
	lis, err := net.Listen("tcp", ":50052")
	if err != nil {
		slog.Error("Listen 失敗", "error", err)
		os.Exit(1)
	}

	// gRPC サーバーの作成
	srv := grpc.NewServer()

	// InventoryService ハンドラーを登録
	srv.RegisterService(&pb.InventoryService_ServiceDesc, newInventoryServer())

	// リフレクション登録
	reflection.Register(srv)

	slog.Info("InventoryService サーバー起動",
		"port", 50052,
		"products", []string{"product-A(100)", "product-B(50)", "product-C(10)", "product-D(0)"},
	)

	// ブロッキングでサーバーを起動
	if err := srv.Serve(lis); err != nil {
		slog.Error("Serve 失敗", "error", err)
		os.Exit(1)
	}
}
