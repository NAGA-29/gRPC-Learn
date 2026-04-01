// handler/error.go
// ErrorService の実装。
// クライアントが指定したエラーシナリオに対応する gRPC ステータスエラーを返す。
// INVALID_ARGUMENT シナリオでは Rich Error Details（BadRequestDetail）を添付する。
package handler

import (
	"context"
	"log/slog"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/grpc-learn/step07/server-go/gen/step07"
)

// ErrorServer は ErrorService の gRPC ハンドラー実装。
type ErrorServer struct {
	pb.UnimplementedErrorServiceServer
	logger *slog.Logger
}

// NewErrorServer は ErrorServer を初期化して返す。
func NewErrorServer(logger *slog.Logger) *ErrorServer {
	return &ErrorServer{logger: logger}
}

// TriggerError は指定されたシナリオに対応する gRPC エラーを返す。
// ERROR_SCENARIO_UNSPECIFIED の場合のみ正常レスポンスを返す。
func (s *ErrorServer) TriggerError(ctx context.Context, req *pb.TriggerErrorRequest) (*pb.TriggerErrorResponse, error) {
	s.logger.Info("TriggerError 受信", "scenario", req.Scenario, "resource_id", req.ResourceId)

	switch req.Scenario {

	// NOT_FOUND: 指定されたリソースが存在しない
	case pb.ErrorScenario_ERROR_SCENARIO_NOT_FOUND:
		s.logger.Info("NOT_FOUND エラーを返却", "resource_id", req.ResourceId)
		return nil, status.Errorf(codes.NotFound, "リソース '%s' が見つかりません", req.ResourceId)

	// PERMISSION_DENIED: 操作する権限がない
	case pb.ErrorScenario_ERROR_SCENARIO_PERMISSION_DENIED:
		s.logger.Info("PERMISSION_DENIED エラーを返却")
		return nil, status.Errorf(codes.PermissionDenied, "アクセス権限がありません")

	// INVALID_ARGUMENT: リクエストのパラメータが不正（Rich Error Details を添付）
	case pb.ErrorScenario_ERROR_SCENARIO_INVALID_ARGUMENT:
		s.logger.Info("INVALID_ARGUMENT エラーを返却（BadRequestDetail 付き）")

		// BadRequestDetail に不正フィールドの詳細を設定する
		detail := &pb.BadRequestDetail{
			Violations: []*pb.FieldViolation{
				{
					Field:       "resource_id",
					Description: "resource_id は空にできません。有効な ID を指定してください。",
				},
				{
					Field:       "scenario",
					Description: "存在しないシナリオ番号が指定されました。",
				},
			},
		}

		// status に Rich Error Details を添付する
		// status.WithDetails を使うと Any 型でメッセージを埋め込める
		st, err := status.New(codes.InvalidArgument, "リクエストパラメータが不正です").
			WithDetails(detail)
		if err != nil {
			// WithDetails が失敗した場合はシンプルなエラーにフォールバック
			s.logger.Error("WithDetails 失敗", "error", err)
			return nil, status.Errorf(codes.InvalidArgument, "リクエストパラメータが不正です")
		}
		return nil, st.Err()

	// UNAVAILABLE: サービスが一時的に停止中（リトライ可能なエラー）
	case pb.ErrorScenario_ERROR_SCENARIO_UNAVAILABLE:
		s.logger.Info("UNAVAILABLE エラーを返却")
		return nil, status.Errorf(codes.Unavailable, "サービスが一時的に利用できません")

	// RESOURCE_EXHAUSTED: レートリミット超過やクォータ枯渇
	case pb.ErrorScenario_ERROR_SCENARIO_RESOURCE_EXHAUSTED:
		s.logger.Info("RESOURCE_EXHAUSTED エラーを返却")
		return nil, status.Errorf(codes.ResourceExhausted, "レートリミット超過")

	// INTERNAL: サーバー内部エラー（原因不明の障害）
	case pb.ErrorScenario_ERROR_SCENARIO_INTERNAL:
		s.logger.Info("INTERNAL エラーを返却")
		return nil, status.Errorf(codes.Internal, "内部エラーが発生しました")

	// UNSPECIFIED（デフォルト）: 正常レスポンスを返す
	default:
		s.logger.Info("正常レスポンスを返却")
		return &pb.TriggerErrorResponse{
			Result: "エラーなし: 正常に処理が完了しました",
		}, nil
	}
}
