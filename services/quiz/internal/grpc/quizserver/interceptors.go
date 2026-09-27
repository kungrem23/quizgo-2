package quizserver

import (
	"context"
	"log"
	"runtime/debug"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func UnaryLoggingInterceptor(logger *log.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		started := time.Now()
		response, err := handler(ctx, req)
		requestID := ""
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			values := md.Get("x-request-id")
			if len(values) == 1 {
				requestID = values[0]
			}
		}
		logger.Printf("grpc_request method=%q code=%q duration_ms=%.3f request_id=%q",
			info.FullMethod, status.Code(err), float64(time.Since(started).Microseconds())/1000, requestID)
		return response, err
	}
}

func UnaryRecoveryInterceptor(logger *log.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (response any, err error) {
		defer func() {
			if recovered := recover(); recovered != nil {
				logger.Printf("grpc_panic method=%q panic=%q stack=%q", info.FullMethod, recovered, debug.Stack())
				response = nil
				err = status.Error(codes.Internal, "internal error")
			}
		}()
		return handler(ctx, req)
	}
}
