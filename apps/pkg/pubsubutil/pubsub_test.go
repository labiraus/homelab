package pubsubutil

import (
	"context"
	"net"
	"testing"

	"cloud.google.com/go/pubsub/v2"
	"cloud.google.com/go/pubsub/v2/apiv1/pubsubpb"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

type topicServer struct {
	pubsubpb.UnimplementedPublisherServer
	lookup        codes.Code
	gets, creates int
}

func (s *topicServer) GetTopic(_ context.Context, r *pubsubpb.GetTopicRequest) (*pubsubpb.Topic, error) {
	s.gets++
	if s.lookup != codes.OK {
		return nil, status.Error(s.lookup, "fixture refusal")
	}
	return &pubsubpb.Topic{Name: r.Topic}, nil
}

func (s *topicServer) CreateTopic(_ context.Context, r *pubsubpb.Topic) (*pubsubpb.Topic, error) {
	s.creates++
	return r, nil
}

func TestPublisherProvisioning(t *testing.T) {
	for _, tc := range []struct {
		name              string
		lookup            codes.Code
		create, wantError bool
		creates           int
	}{
		{"existing", codes.OK, false, false, 0},
		{"missing refused", codes.NotFound, false, true, 0},
		{"missing provisioned", codes.NotFound, true, false, 1},
		{"permission denied", codes.PermissionDenied, true, true, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			listener := bufconn.Listen(1024 * 1024)
			server := grpc.NewServer()
			fixture := &topicServer{lookup: tc.lookup}
			pubsubpb.RegisterPublisherServer(server, fixture)
			go server.Serve(listener)
			t.Cleanup(server.Stop)
			conn, err := grpc.NewClient("passthrough:///fixture", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { conn.Close() })
			client, err = pubsub.NewClient(ctx, "fixture", option.WithGRPCConn(conn))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { client.Close() })
			config = PubsubConfig{Topics: map[string]Topic{"test": {Name: "topic", CreateTopic: tc.create}}}
			topics = make(map[string]*pubsub.Publisher)
			publisher, err := GetTopic(ctx, "test")
			if (err != nil) != tc.wantError {
				t.Fatalf("GetTopic error = %v", err)
			}
			if fixture.creates != tc.creates {
				t.Fatalf("created %d topics, want %d", fixture.creates, tc.creates)
			}
			if err == nil {
				again, err := GetTopic(ctx, "test")
				if err != nil || again != publisher || fixture.gets != 1 {
					t.Fatalf("publisher was not cached: %v", err)
				}
				publisher.Stop()
			}
		})
	}
}
