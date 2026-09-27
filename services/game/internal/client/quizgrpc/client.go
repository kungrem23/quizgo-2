package quizgrpc

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"time"

	quizv1 "github.com/kungrem23/quizgo/gen/quiz/v1"
	game "github.com/kungrem23/quizgo/services/game/internal/domain"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

const serviceTokenMetadata = "x-quizgo-service-token"

type TLSConfig struct {
	CAFile     string
	CertFile   string
	KeyFile    string
	ServerName string
}

type Client struct {
	rpc          quizv1.QuizCatalogServiceClient
	serviceToken string
}

func New(connection grpc.ClientConnInterface, serviceToken string) *Client {
	return &Client{rpc: quizv1.NewQuizCatalogServiceClient(connection), serviceToken: serviceToken}
}

func Dial(address string, tlsSettings TLSConfig) (*grpc.ClientConn, error) {
	credentialsOption, err := transportCredentials(tlsSettings)
	if err != nil {
		return nil, err
	}
	connection, err := grpc.NewClient(address, grpc.WithTransportCredentials(credentialsOption))
	if err != nil {
		return nil, fmt.Errorf("create quiz gRPC client: %w", err)
	}
	return connection, nil
}

func (c *Client) GetPlayableQuiz(ctx context.Context, quizID int64, accessToken string) (game.QuizSnapshot, error) {
	if c == nil || c.rpc == nil || quizID < 1 || accessToken == "" {
		return game.QuizSnapshot{}, errors.New("invalid playable quiz request")
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
	}
	ctx = metadata.AppendToOutgoingContext(ctx,
		serviceTokenMetadata, c.serviceToken,
		"authorization", "Bearer "+accessToken,
	)
	response, err := c.rpc.GetPlayableQuiz(ctx, &quizv1.GetPlayableQuizRequest{QuizId: quizID})
	if err != nil {
		return game.QuizSnapshot{}, err
	}
	if response.GetQuiz() == nil {
		return game.QuizSnapshot{}, errors.New("quiz service returned an empty snapshot")
	}
	return mapSnapshot(response.GetQuiz()), nil
}

func mapSnapshot(source *quizv1.PlayableQuiz) game.QuizSnapshot {
	result := game.QuizSnapshot{
		ID: source.GetId(), Revision: source.GetRevision(), Title: source.GetTitle(), OwnerUserID: source.GetOwnerUserId(),
		Questions: make([]game.Question, 0, len(source.GetQuestions())),
	}
	for _, question := range source.GetQuestions() {
		mapped := game.Question{
			ID: question.GetId(), Text: question.GetText(), ImageID: question.GetImageId(),
			TimeLimitSeconds: question.GetTimeLimitSeconds(), Answers: make([]game.Answer, 0, len(question.GetAnswers())),
		}
		for _, answer := range question.GetAnswers() {
			mapped.Answers = append(mapped.Answers, game.Answer{ID: answer.GetId(), Text: answer.GetText(), IsCorrect: answer.GetIsCorrect()})
		}
		result.Questions = append(result.Questions, mapped)
	}
	return result
}

func transportCredentials(settings TLSConfig) (credentials.TransportCredentials, error) {
	if settings.CAFile == "" && settings.CertFile == "" && settings.KeyFile == "" {
		return insecure.NewCredentials(), nil
	}
	certificate, err := tls.LoadX509KeyPair(settings.CertFile, settings.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("load game client certificate: %w", err)
	}
	caPEM, err := os.ReadFile(settings.CAFile)
	if err != nil {
		return nil, fmt.Errorf("read quiz server CA: %w", err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(caPEM) {
		return nil, errors.New("parse quiz server CA: no certificates found")
	}
	return credentials.NewTLS(&tls.Config{
		MinVersion: tls.VersionTLS13, RootCAs: roots, Certificates: []tls.Certificate{certificate}, ServerName: settings.ServerName,
	}), nil
}
