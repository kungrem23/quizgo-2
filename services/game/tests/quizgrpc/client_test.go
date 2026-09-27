package quizgrpc_test

import (
	"context"
	"testing"

	quizv1 "github.com/kungrem23/quizgo/gen/quiz/v1"
	. "github.com/kungrem23/quizgo/services/game/internal/client/quizgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type connectionStub struct {
	metadata metadata.MD
}

func (c *connectionStub) Invoke(ctx context.Context, _ string, _ any, reply any, _ ...grpc.CallOption) error {
	c.metadata, _ = metadata.FromOutgoingContext(ctx)
	response := reply.(*quizv1.GetPlayableQuizResponse)
	response.Quiz = &quizv1.PlayableQuiz{
		Id: 1, Revision: 2, Title: "Go", OwnerUserId: 3,
		Questions: []*quizv1.PlayableQuestion{{
			Id: 4, Text: "Q", TimeLimitSeconds: 15,
			Answers: []*quizv1.PlayableAnswer{{Id: 5, Text: "A", IsCorrect: true}},
		}},
	}
	return nil
}

func (c *connectionStub) NewStream(context.Context, *grpc.StreamDesc, string, ...grpc.CallOption) (grpc.ClientStream, error) {
	return nil, nil
}

func TestClientMapsSnapshotAndCredentials(t *testing.T) {
	connection := &connectionStub{}
	snapshot, err := New(connection, "service-token").GetPlayableQuiz(context.Background(), 1, "access-token")
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.ID != 1 || snapshot.Questions[0].Answers[0].ID != 5 || !snapshot.Questions[0].Answers[0].IsCorrect {
		t.Fatalf("unexpected snapshot: %#v", snapshot)
	}
	if got := connection.metadata.Get("x-quizgo-service-token"); len(got) != 1 || got[0] != "service-token" {
		t.Fatalf("service token metadata = %v", got)
	}
	if got := connection.metadata.Get("authorization"); len(got) != 1 || got[0] != "Bearer access-token" {
		t.Fatalf("authorization metadata = %v", got)
	}
}
