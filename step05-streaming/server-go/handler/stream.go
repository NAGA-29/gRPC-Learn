// handler/stream.go
// StreamService の実装。
// サーバーストリーミング（WatchStock）/ クライアントストリーミング（UploadFile）/
// 双方向ストリーミング（Chat）の3パターンを実装する。
package handler

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"sync"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/grpc-learn/step05/server-go/gen/step05"
)

// StreamServer は StreamService の gRPC ハンドラー実装。
type StreamServer struct {
	pb.UnimplementedStreamServiceServer
	logger *slog.Logger

	// Chat 用: ルーム名 → 参加者ストリームのスライス（goroutine-safe）
	mu    sync.Mutex
	rooms map[string][]pb.StreamService_ChatServer
}

// NewStreamServer は StreamServer を初期化して返す。
func NewStreamServer(logger *slog.Logger) *StreamServer {
	return &StreamServer{
		logger: logger,
		rooms:  make(map[string][]pb.StreamService_ChatServer),
	}
}

// ----------------------------------------------------------------
// WatchStock: サーバーストリーミング
// 500ms ごとに株価を送信し、クライアントがキャンセルするまで継続する。
// ----------------------------------------------------------------
func (s *StreamServer) WatchStock(req *pb.WatchStockRequest, stream pb.StreamService_WatchStockServer) error {
	s.logger.Info("WatchStock 開始", "symbol", req.Symbol)

	// 初期価格をランダムに設定（100〜200の範囲）
	price := 100.0 + rand.Float64()*100.0

	for {
		// context がキャンセルされたら終了（クライアントが切断した場合）
		select {
		case <-stream.Context().Done():
			s.logger.Info("WatchStock 終了（クライアント切断）", "symbol", req.Symbol)
			return nil
		default:
		}

		// 価格をランダムに変動させる（±2% の変動）
		change := (rand.Float64()*4 - 2) / 100.0
		price = price * (1 + change)

		// 株価をクライアントに送信
		resp := &pb.StockPrice{
			Symbol:    req.Symbol,
			Price:     price,
			Timestamp: time.Now().Unix(),
		}

		if err := stream.Send(resp); err != nil {
			s.logger.Error("WatchStock 送信エラー", "error", err, "symbol", req.Symbol)
			return err
		}

		s.logger.Info("WatchStock 送信", "symbol", req.Symbol, "price", fmt.Sprintf("%.2f", price))

		// 500ms 待機
		time.Sleep(500 * time.Millisecond)
	}
}

// ----------------------------------------------------------------
// UploadFile: クライアントストリーミング
// クライアントからファイルチャンクを受け取り、完了時にサマリを返す。
// ----------------------------------------------------------------
func (s *StreamServer) UploadFile(stream pb.StreamService_UploadFileServer) error {
	s.logger.Info("UploadFile 開始")

	var (
		totalBytes int64
		chunkCount int32
		fileName   string
	)

	for {
		// クライアントからチャンクを受信
		chunk, err := stream.Recv()
		if err != nil {
			// io.EOF はストリーム正常終了を示す
			// grpc の場合は status.Code で判定
			if status.Code(err) == codes.OK || isEOF(err) {
				break
			}
			s.logger.Error("UploadFile 受信エラー", "error", err)
			return err
		}

		// 最初のチャンクからファイル名を取得
		if chunkCount == 0 {
			fileName = chunk.FileName
			s.logger.Info("UploadFile ファイル名取得", "file_name", fileName)
		}

		// バイト数をカウント
		totalBytes += int64(len(chunk.Data))
		chunkCount++
		s.logger.Info("UploadFile チャンク受信",
			"chunk_number", chunkCount,
			"bytes", len(chunk.Data),
			"total_bytes", totalBytes,
		)
	}

	s.logger.Info("UploadFile 完了",
		"file_name", fileName,
		"total_bytes", totalBytes,
		"chunk_count", chunkCount,
	)

	// アップロードサマリをクライアントに返す
	return stream.SendAndClose(&pb.UploadFileSummary{
		FileName:   fileName,
		TotalBytes: totalBytes,
		ChunkCount: chunkCount,
		Message:    fmt.Sprintf("ファイル '%s' のアップロードが完了しました（%d バイト、%d チャンク）", fileName, totalBytes, chunkCount),
	})
}

// ----------------------------------------------------------------
// Chat: 双方向ストリーミング
// 受け取ったメッセージを同一ルームの全参加者にブロードキャストする。
// ----------------------------------------------------------------
func (s *StreamServer) Chat(stream pb.StreamService_ChatServer) error {
	s.logger.Info("Chat 接続開始")

	// 最初のメッセージを受信してルームと送信者を特定
	firstMsg, err := stream.Recv()
	if err != nil {
		s.logger.Error("Chat 初回受信エラー", "error", err)
		return err
	}

	roomName := firstMsg.Room
	sender := firstMsg.Sender
	s.logger.Info("Chat ルーム参加", "room", roomName, "sender", sender)

	// ルームにストリームを登録
	s.joinRoom(roomName, stream)
	defer s.leaveRoom(roomName, stream)

	// 最初のメッセージをルームにブロードキャスト
	s.broadcast(roomName, firstMsg, stream)

	// 以降のメッセージを受信してブロードキャスト
	for {
		// context がキャンセルされたら終了
		select {
		case <-stream.Context().Done():
			s.logger.Info("Chat 接続終了（コンテキストキャンセル）", "sender", sender)
			return nil
		default:
		}

		msg, err := stream.Recv()
		if err != nil {
			if isEOF(err) {
				s.logger.Info("Chat 接続正常終了", "sender", sender)
				return nil
			}
			s.logger.Error("Chat 受信エラー", "error", err, "sender", sender)
			return err
		}

		s.logger.Info("Chat メッセージ受信", "room", msg.Room, "sender", msg.Sender, "text", msg.Text)

		// 受信したメッセージをルーム内の全参加者にブロードキャスト
		s.broadcast(msg.Room, msg, stream)
	}
}

// joinRoom はルームにストリームを追加する（goroutine-safe）。
func (s *StreamServer) joinRoom(room string, stream pb.StreamService_ChatServer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rooms[room] = append(s.rooms[room], stream)
}

// leaveRoom はルームからストリームを削除する（goroutine-safe）。
func (s *StreamServer) leaveRoom(room string, stream pb.StreamService_ChatServer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	streams := s.rooms[room]
	for i, st := range streams {
		if st == stream {
			s.rooms[room] = append(streams[:i], streams[i+1:]...)
			break
		}
	}
	if len(s.rooms[room]) == 0 {
		delete(s.rooms, room)
	}
}

// broadcast はルーム内の全参加者にメッセージを送信する（goroutine-safe）。
// 自分自身（送信元）にも送り返すことで「エコー確認」を実現する。
func (s *StreamServer) broadcast(room string, msg *pb.ChatMessage, _ pb.StreamService_ChatServer) {
	s.mu.Lock()
	streams := make([]pb.StreamService_ChatServer, len(s.rooms[room]))
	copy(streams, s.rooms[room])
	s.mu.Unlock()

	for _, st := range streams {
		if err := st.Send(msg); err != nil {
			s.logger.Warn("Chat ブロードキャスト送信エラー", "error", err)
		}
	}
}

// isEOF は gRPC ストリーム終了エラー（io.EOF 相当）かどうかを判定する。
func isEOF(err error) bool {
	return errors.Is(err, io.EOF)
}
