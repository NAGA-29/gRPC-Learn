// handler/task.go
// TaskService の実装。
// インメモリストアでタスクを管理し、FlakyEcho では意図的に失敗を返して
// クライアント側リトライの動作確認を可能にする。
package handler

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/grpc-learn/step11/server-go/gen/step11"
)

// TaskServer は TaskService の gRPC ハンドラー実装。
type TaskServer struct {
	pb.UnimplementedTaskServiceServer
	logger *slog.Logger

	mu     sync.Mutex
	tasks  map[string]*pb.Task // タスク ID → タスク
	nextID int                 // 採番用カウンター

	// FlakyEcho 用: message ごとの累計試行回数
	attempts map[string]int32
}

// NewTaskServer は TaskServer を初期化して返す。
func NewTaskServer(logger *slog.Logger) *TaskServer {
	return &TaskServer{
		logger:   logger,
		tasks:    make(map[string]*pb.Task),
		nextID:   1,
		attempts: make(map[string]int32),
	}
}

// CreateTask はタスクを作成する。
// title が空の場合は codes.InvalidArgument を返す。
func (s *TaskServer) CreateTask(ctx context.Context, req *pb.CreateTaskRequest) (*pb.CreateTaskResponse, error) {
	if req.Title == "" {
		return nil, status.Error(codes.InvalidArgument, "title は必須です")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	task := &pb.Task{
		Id:        fmt.Sprintf("task-%03d", s.nextID),
		Title:     req.Title,
		Done:      false,
		CreatedAt: time.Now().Format(time.RFC3339),
	}
	s.nextID++
	s.tasks[task.Id] = task

	s.logger.Info("CreateTask 完了", "id", task.Id, "title", task.Title)
	return &pb.CreateTaskResponse{Task: task}, nil
}

// GetTask は ID を指定してタスクを返す。
// 存在しない ID の場合は codes.NotFound を返す。
func (s *TaskServer) GetTask(ctx context.Context, req *pb.GetTaskRequest) (*pb.GetTaskResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, ok := s.tasks[req.Id]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "タスク ID %q が見つかりません", req.Id)
	}
	return &pb.GetTaskResponse{Task: task}, nil
}

// FlakyEcho は同一 message に対して fail_count 回 codes.Unavailable を返し、
// fail_count + 1 回目で成功する。クライアント側リトライポリシーの確認用。
//
// Unavailable は「一時的な障害」を表し、リトライ可能なコードの代表例。
// gRPC のリトライポリシー（service config）は retryableStatusCodes に
// 指定されたコードのときだけ自動リトライする。
func (s *TaskServer) FlakyEcho(ctx context.Context, req *pb.FlakyEchoRequest) (*pb.FlakyEchoResponse, error) {
	s.mu.Lock()
	s.attempts[req.Message]++
	attempts := s.attempts[req.Message]
	s.mu.Unlock()

	if attempts <= req.FailCount {
		s.logger.Warn("FlakyEcho 意図的に失敗",
			"message", req.Message,
			"attempts", attempts,
			"fail_count", req.FailCount,
		)
		return nil, status.Errorf(codes.Unavailable,
			"一時的に利用できません（%d/%d 回目の失敗）", attempts, req.FailCount)
	}

	s.logger.Info("FlakyEcho 成功", "message", req.Message, "attempts", attempts)
	return &pb.FlakyEchoResponse{
		Message:  req.Message,
		Attempts: attempts,
	}, nil
}
