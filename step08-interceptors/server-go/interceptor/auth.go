// interceptor/auth.go
// 認証インターセプター。
// "GetSecret" メソッドにのみ認証を適用する選択的な認証ミドルウェア。
// metadata の "authorization" キーから "Bearer secret-token-12345" を検証し、
// 一致しない場合は codes.Unauthenticated エラーを返す。
package interceptor

import (
	"context"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// validToken は受け付ける Bearer トークン（学習用のハードコード値）。
// 本番環境では JWT 検証・DB 照合などを行う。
const validToken = "Bearer secret-token-12345"

// protectedMethods は認証が必要なメソッドのフルネームのセット。
// 必要に応じてメソッドを追加するだけで保護範囲を拡張できる。
var protectedMethods = map[string]struct{}{
	"/step08.InterceptorService/GetSecret": {},
}

// NewAuthInterceptor は Unary RPC 用の認証インターセプターを返す。
// protectedMethods に含まれないメソッドはトークン検証をスキップする。
func NewAuthInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		// 保護対象メソッドでなければ認証をスキップして次のハンドラーへ進む
		if _, protected := protectedMethods[info.FullMethod]; !protected {
			return handler(ctx, req)
		}

		// gRPC メタデータ（HTTP ヘッダー相当）をコンテキストから取得する
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "メタデータが見つかりません")
		}

		// "authorization" キーの値を取得する（小文字に正規化されている）
		values := md.Get("authorization")
		if len(values) == 0 {
			return nil, status.Error(codes.Unauthenticated, "authorization ヘッダーが必要です")
		}

		// トークンを検証する（大文字小文字を区別して比較）
		token := strings.TrimSpace(values[0])
		if token != validToken {
			return nil, status.Errorf(codes.Unauthenticated,
				"無効なトークンです: %q", token)
		}

		// 認証成功 → 次のハンドラーへ処理を委譲する
		return handler(ctx, req)
	}
}
