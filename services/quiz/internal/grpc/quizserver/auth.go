package quizserver

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

const serviceTokenMetadata = "x-quizgo-service-token"

type AccessTokenVerifier interface {
	VerifyAccessToken(string) (int, error)
}

type Principal struct {
	UserID int
}

type principalKey struct{}

func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	value, ok := ctx.Value(principalKey{}).(Principal)
	return value, ok
}

func UnaryAuthInterceptor(verifier AccessTokenVerifier, serviceToken, allowedClientURI string) grpc.UnaryServerInterceptor {
	wantServiceToken := sha256.Sum256([]byte(serviceToken))
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if strings.HasPrefix(info.FullMethod, "/grpc.health.v1.Health/") {
			return handler(ctx, req)
		}
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok || !validServiceToken(md.Get(serviceTokenMetadata), wantServiceToken) {
			return nil, status.Error(codes.Unauthenticated, "invalid service credentials")
		}
		if allowedClientURI != "" && !hasClientURI(ctx, allowedClientURI) {
			return nil, status.Error(codes.Unauthenticated, "invalid client certificate")
		}
		authorization := md.Get("authorization")
		if len(authorization) != 1 || verifier == nil {
			return nil, status.Error(codes.Unauthenticated, "invalid user credentials")
		}
		parts := strings.Fields(authorization[0])
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			return nil, status.Error(codes.Unauthenticated, "invalid user credentials")
		}
		userID, err := verifier.VerifyAccessToken(parts[1])
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid user credentials")
		}
		ctx = context.WithValue(ctx, principalKey{}, Principal{UserID: userID})
		return handler(ctx, req)
	}
}

func validServiceToken(values []string, want [32]byte) bool {
	if len(values) != 1 {
		return false
	}
	got := sha256.Sum256([]byte(values[0]))
	return subtle.ConstantTimeCompare(got[:], want[:]) == 1
}

func hasClientURI(ctx context.Context, want string) bool {
	p, ok := peer.FromContext(ctx)
	if !ok {
		return false
	}
	tlsInfo, ok := p.AuthInfo.(credentials.TLSInfo)
	if !ok || len(tlsInfo.State.VerifiedChains) == 0 || len(tlsInfo.State.VerifiedChains[0]) == 0 {
		return false
	}
	for _, uri := range tlsInfo.State.VerifiedChains[0][0].URIs {
		if uri.String() == want {
			return true
		}
	}
	return false
}
