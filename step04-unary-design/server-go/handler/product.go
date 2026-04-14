// handler/product.go
// ProductService の実装。
// インメモリのダミーデータに対して GetProduct / ListProducts / UpdateProduct を提供する。
package handler

import (
	"context"
	"log/slog"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	pb "github.com/grpc-learn/step04/server-go/gen/step04"
)

// ProductServer は ProductService の gRPC ハンドラー実装。
type ProductServer struct {
	pb.UnimplementedProductServiceServer
	logger   *slog.Logger
	products map[string]*pb.Product // インメモリストア（キー: product ID）
}

// NewProductServer は ProductServer を初期化して返す。
// ダミーデータを5件セットアップする。
func NewProductServer(logger *slog.Logger) *ProductServer {
	// ダミー商品データ（Money 型で価格を表現）
	products := map[string]*pb.Product{
		"prod-001": {
			Id:          "prod-001",
			Name:        "Go プログラミング入門",
			Description: "Go 言語の基礎から応用まで学べる書籍",
			Price: &pb.Money{
				CurrencyCode: "JPY",
				Units:        3200,
				Nanos:        0,
			},
			Category: "書籍",
			InStock:  true,
		},
		"prod-002": {
			Id:          "prod-002",
			Name:        "メカニカルキーボード",
			Description: "赤軸・フルサイズ・日本語配列",
			Price: &pb.Money{
				CurrencyCode: "JPY",
				Units:        12800,
				Nanos:        0,
			},
			Category: "周辺機器",
			InStock:  true,
		},
		"prod-003": {
			Id:          "prod-003",
			Name:        "USB-C ハブ 7-in-1",
			Description: "HDMI / USB3.0 / PD 充電対応",
			Price: &pb.Money{
				CurrencyCode: "JPY",
				Units:        4500,
				Nanos:        0,
			},
			Category: "周辺機器",
			InStock:  false,
		},
		"prod-004": {
			Id:          "prod-004",
			Name:        "クラウドネイティブ設計パターン",
			Description: "マイクロサービス・コンテナ・Kubernetes の実践書",
			Price: &pb.Money{
				CurrencyCode: "JPY",
				Units:        4200,
				Nanos:        0,
			},
			Category: "書籍",
			InStock:  true,
		},
		"prod-005": {
			Id:          "prod-005",
			Name:        "モニターアーム シングル",
			Description: "VESA 対応・高さ調節可能",
			Price: &pb.Money{
				CurrencyCode: "JPY",
				Units:        8900,
				Nanos:        0,
			},
			Category: "デスク周辺",
			InStock:  true,
		},
	}

	return &ProductServer{
		logger:   logger,
		products: products,
	}
}

// GetProduct は ID を指定して単一の商品を返す。
// 存在しない ID の場合は codes.NotFound を返す。
func (s *ProductServer) GetProduct(ctx context.Context, req *pb.GetProductRequest) (*pb.GetProductResponse, error) {
	s.logger.Info("GetProduct 受信", "id", req.Id)

	product, ok := s.products[req.Id]
	if !ok {
		s.logger.Warn("商品が見つかりません", "id", req.Id)
		return nil, status.Errorf(codes.NotFound, "商品 ID %q が見つかりません", req.Id)
	}

	// FieldMask が指定されていれば、必要なフィールドだけ返す
	if req.ReadMask != nil && len(req.ReadMask.Paths) > 0 {
		product = applyReadMask(product, req.ReadMask)
	}

	s.logger.Info("GetProduct 返信", "name", product.Name)
	return &pb.GetProductResponse{Product: product}, nil
}

// ListProducts はページネーション付きで商品一覧を返す。
// page_token は前回レスポンスの next_page_token を使う。
// page_size が 0 の場合はデフォルト 3 件を返す。
func (s *ProductServer) ListProducts(ctx context.Context, req *pb.ListProductsRequest) (*pb.ListProductsResponse, error) {
	s.logger.Info("ListProducts 受信", "page_size", req.PageSize, "page_token", req.PageToken)

	// 全商品を順序付きスライスに変換（マップは順序不定なので固定順序にする）
	ids := []string{"prod-001", "prod-002", "prod-003", "prod-004", "prod-005"}

	// ページサイズのデフォルトは 3
	pageSize := int(req.PageSize)
	if pageSize <= 0 {
		pageSize = 3
	}

	// ページトークンは開始インデックス（文字列）として扱う
	startIdx := 0
	if req.PageToken != "" {
		// トークンから開始インデックスを解析
		for i, id := range ids {
			if id == req.PageToken {
				startIdx = i
				break
			}
		}
	}

	// 該当ページのアイテムを取得
	var products []*pb.Product
	nextPageToken := ""

	for i := startIdx; i < len(ids) && len(products) < pageSize; i++ {
		if p, ok := s.products[ids[i]]; ok {
			products = append(products, p)
		}
	}

	// 次のページが存在する場合はトークンをセット
	nextIdx := startIdx + len(products)
	if nextIdx < len(ids) {
		nextPageToken = ids[nextIdx]
	}

	s.logger.Info("ListProducts 返信", "count", len(products), "next_token", nextPageToken)
	return &pb.ListProductsResponse{
		Products:      products,
		NextPageToken: nextPageToken,
		TotalSize:     int32(len(s.products)),
	}, nil
}

// UpdateProduct は FieldMask で指定されたフィールドのみを更新する。
// FieldMask を使うことで REST の PATCH 相当の部分更新を実現する。
func (s *ProductServer) UpdateProduct(ctx context.Context, req *pb.UpdateProductRequest) (*pb.UpdateProductResponse, error) {
	s.logger.Info("UpdateProduct 受信", "id", req.Product.Id)

	existing, ok := s.products[req.Product.Id]
	if !ok {
		s.logger.Warn("更新対象の商品が見つかりません", "id", req.Product.Id)
		return nil, status.Errorf(codes.NotFound, "商品 ID %q が見つかりません", req.Product.Id)
	}

	// UpdateMask が指定されていれば、そのフィールドだけ更新する
	if req.UpdateMask != nil && len(req.UpdateMask.Paths) > 0 {
		for _, path := range req.UpdateMask.Paths {
			switch path {
			case "name":
				existing.Name = req.Product.Name
			case "description":
				existing.Description = req.Product.Description
			case "price":
				existing.Price = req.Product.Price
			case "category":
				existing.Category = req.Product.Category
			case "in_stock":
				existing.InStock = req.Product.InStock
			default:
				s.logger.Warn("未知のフィールドパスを無視します", "path", path)
			}
		}
	} else {
		// FieldMask 未指定の場合は全フィールドを置き換える
		s.logger.Info("FieldMask 未指定: 全フィールドを更新します")
		existing.Name = req.Product.Name
		existing.Description = req.Product.Description
		existing.Price = req.Product.Price
		existing.Category = req.Product.Category
		existing.InStock = req.Product.InStock
	}

	s.products[existing.Id] = existing
	s.logger.Info("UpdateProduct 完了", "id", existing.Id)

	return &pb.UpdateProductResponse{Product: existing}, nil
}

// applyReadMask は FieldMask に基づいて不要なフィールドをゼロ値にする。
// 元のオブジェクトを変更しないようコピーして返す。
func applyReadMask(p *pb.Product, mask *fieldmaskpb.FieldMask) *pb.Product {
	result := &pb.Product{}
	for _, path := range mask.Paths {
		switch path {
		case "id":
			result.Id = p.Id
		case "name":
			result.Name = p.Name
		case "description":
			result.Description = p.Description
		case "price":
			result.Price = p.Price
		case "category":
			result.Category = p.Category
		case "in_stock":
			result.InStock = p.InStock
		}
	}
	return result
}
