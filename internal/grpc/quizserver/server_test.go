package quizserver

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net/url"
	"testing"

	quizv1 "github.com/kungrem23/quizgo/gen/quiz/v1"
	"github.com/kungrem23/quizgo/internal/domain/quiz"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

type verifierStub struct {
	userID int
	err    error
}

func TestHasClientURIRequiresVerifiedExactSAN(t *testing.T) {
	uri, err := url.Parse("spiffe://quizgo.local/service/realtime")
	if err != nil {
		t.Fatal(err)
	}
	certificate := &x509.Certificate{URIs: []*url.URL{uri}}
	ctx := peer.NewContext(context.Background(), &peer.Peer{AuthInfo: credentials.TLSInfo{State: tls.ConnectionState{
		VerifiedChains: [][]*x509.Certificate{{certificate}},
	}}})
	if !hasClientURI(ctx, uri.String()) {
		t.Fatal("expected matching verified URI SAN")
	}
	if hasClientURI(ctx, "spiffe://quizgo.local/service/other") {
		t.Fatal("accepted a different URI SAN")
	}
}

func (v verifierStub) VerifyAccessToken(string) (int, error) { return v.userID, v.err }

type quizServiceStub struct {
	content quiz.QuizContent
	err     error
	userID  int
}

func (s *quizServiceStub) GetPlayableQuiz(_ context.Context, _ int, userID int) (quiz.QuizContent, error) {
	s.userID = userID
	return s.content, s.err
}

func TestUnaryAuthInterceptorEstablishesPrincipal(t *testing.T) {
	interceptor := UnaryAuthInterceptor(verifierStub{userID: 42}, "service-secret", "")
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		serviceTokenMetadata, "service-secret",
		"authorization", "Bearer user-token",
	))
	_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{}, func(ctx context.Context, _ any) (any, error) {
		principal, ok := PrincipalFromContext(ctx)
		if !ok || principal.UserID != 42 {
			t.Fatalf("principal = %#v, %v", principal, ok)
		}
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestUnaryAuthInterceptorRejectsCredentials(t *testing.T) {
	tests := []struct {
		name     string
		metadata metadata.MD
		verifier verifierStub
	}{
		{name: "missing service token", metadata: metadata.Pairs("authorization", "Bearer user-token"), verifier: verifierStub{userID: 42}},
		{name: "wrong service token", metadata: metadata.Pairs(serviceTokenMetadata, "wrong", "authorization", "Bearer user-token"), verifier: verifierStub{userID: 42}},
		{name: "invalid user token", metadata: metadata.Pairs(serviceTokenMetadata, "service-secret", "authorization", "Bearer bad"), verifier: verifierStub{err: errors.New("bad token")}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			interceptor := UnaryAuthInterceptor(tt.verifier, "service-secret", "")
			ctx := metadata.NewIncomingContext(context.Background(), tt.metadata)
			_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{}, func(context.Context, any) (any, error) {
				t.Fatal("handler was called")
				return nil, nil
			})
			if status.Code(err) != codes.Unauthenticated {
				t.Fatalf("code = %v, error = %v", status.Code(err), err)
			}
		})
	}
}

func TestGetPlayableQuizMapsSnapshot(t *testing.T) {
	service := &quizServiceStub{content: quiz.QuizContent{
		QuizSummary: quiz.QuizSummary{ID: 7, Revision: 3, Title: "Quiz", AuthorID: 42},
		Questions: []quiz.ContentQuestion{{ID: 8, TextContent: "Question", ImageID: "image", TimeLimit: 20,
			Answers: []quiz.ContentAnswer{{ID: 9, TextContent: "Answer", IsCorrect: true}}}},
	}}
	server := New(service)
	ctx := context.WithValue(context.Background(), principalKey{}, Principal{UserID: 42})
	response, err := server.GetPlayableQuiz(ctx, &quizv1.GetPlayableQuizRequest{QuizId: 7})
	if err != nil {
		t.Fatal(err)
	}
	if service.userID != 42 || response.GetQuiz().GetRevision() != 3 || !response.GetQuiz().GetQuestions()[0].GetAnswers()[0].GetIsCorrect() {
		t.Fatalf("unexpected response: %#v", response)
	}
}

func TestGetPlayableQuizMapsDomainErrors(t *testing.T) {
	server := New(&quizServiceStub{err: quiz.ErrQuizNotPlayable})
	ctx := context.WithValue(context.Background(), principalKey{}, Principal{UserID: 42})
	_, err := server.GetPlayableQuiz(ctx, &quizv1.GetPlayableQuizRequest{QuizId: 7})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("code = %v", status.Code(err))
	}
}
