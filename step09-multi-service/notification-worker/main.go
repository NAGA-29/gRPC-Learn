// Step 09: NotificationService サーバー（通知ワーカー内部サービス）
//
// Gateway からのみ呼ばれる内部サービス。
// 実際のメール/SMS/プッシュ送信は行わず、ログに出力するだけ（学習用）。
//
// 実務では:
//   - SendGrid, AWS SES などのメールプロバイダーを呼び出す
//   - Twilio などで SMS を送信する
//   - FCM/APNs でプッシュ通知を送信する
//   - 失敗時のリトライとデッドレターキューを実装する
//   - 通知履歴をデータベースに記録する
package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"

	pb "github.com/grpc-learn/step09/notification-worker/gen/step09"
)

// notificationServer は NotificationService の実装。
type notificationServer struct {
	pb.UnimplementedNotificationServiceServer
}

// Notify は通知リクエストを受け取り、ログに出力する。
//
// channel フィールドで送信先チャネルを区別する:
//   - "email": メール通知
//   - "sms":   SMS 通知
//   - "push":  プッシュ通知
//
// 実際の送信は行わず、受信したリクエストをログに出力する。
func (s *notificationServer) Notify(
	ctx context.Context,
	req *pb.NotifyRequest,
) (*pb.NotifyResponse, error) {
	if req.OrderId == "" {
		return nil, status.Error(codes.InvalidArgument, "order_id は必須です")
	}
	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id は必須です")
	}

	// 送信チャネルに応じたログ出力
	// 実際の実装ではここで外部サービスを呼び出す
	switch req.Channel {
	case "email":
		slog.Info("[通知] メール送信（模擬）",
			"order_id", req.OrderId,
			"user_id", req.UserId,
			"message", req.Message,
		)
	case "sms":
		slog.Info("[通知] SMS 送信（模擬）",
			"order_id", req.OrderId,
			"user_id", req.UserId,
			"message", req.Message,
		)
	case "push":
		slog.Info("[通知] プッシュ通知送信（模擬）",
			"order_id", req.OrderId,
			"user_id", req.UserId,
			"message", req.Message,
		)
	default:
		slog.Warn("[通知] 未知のチャネル",
			"channel", req.Channel,
			"order_id", req.OrderId,
		)
	}

	// 送信時刻を記録して返す
	sentAt := time.Now().Format(time.RFC3339)

	slog.Info("Notify 完了",
		"order_id", req.OrderId,
		"channel", req.Channel,
		"sent_at", sentAt,
	)

	return &pb.NotifyResponse{
		Sent:   true,
		SentAt: sentAt,
	}, nil
}

func main() {
	// 構造化ログのセットアップ
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	slog.SetDefault(logger)

	// TCP リスナーを :50053 で作成
	lis, err := net.Listen("tcp", ":50053")
	if err != nil {
		slog.Error("Listen 失敗", "error", err)
		os.Exit(1)
	}

	// gRPC サーバーの作成
	srv := grpc.NewServer()

	// NotificationService ハンドラーを登録
	srv.RegisterService(&pb.NotificationService_ServiceDesc, &notificationServer{})

	// リフレクション登録
	reflection.Register(srv)

	slog.Info("NotificationService サーバー起動",
		"port", 50053,
		"note", "実際の送信は行わず、ログに出力します（学習用）",
	)

	// ブロッキングでサーバーを起動
	if err := srv.Serve(lis); err != nil {
		slog.Error("Serve 失敗", "error", err)
		os.Exit(1)
	}
}
