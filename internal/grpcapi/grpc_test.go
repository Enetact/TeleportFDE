package grpcapi

import (
	"context"
	pb "example.com/replica-control/gen/replicas/v1"
	"example.com/replica-control/internal/model"
	"example.com/replica-control/internal/service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"net"
	"testing"
	"time"
)

type stub struct{}

func (stub) Get(_ context.Context, t model.Target) (model.Deployment, error) {
	if t.Name == "missing" {
		return model.Deployment{}, model.E(model.NotFound, "missing", nil)
	}
	return model.Deployment{Target: t, Replicas: 2}, nil
}
func (stub) List(context.Context, string) ([]model.Deployment, error) {
	return []model.Deployment{{Target: model.Target{Namespace: "default", Name: "demo"}, Replicas: 2}}, nil
}
func (stub) Set(_ context.Context, t model.Target, n int32, v string) (model.SetResult, error) {
	if v == "stale" {
		return model.SetResult{}, model.E(model.Conflict, "stale", nil)
	}
	return model.SetResult{Target: t, Replicas: n, Accepted: true}, nil
}
func TestGRPCWire(t *testing.T) {
	listener := bufconn.Listen(1 << 20)
	server := grpc.NewServer()
	pb.RegisterReplicaServiceServer(server, &Server{Service: &service.Service{Backend: stub{}}})
	go func() { _ = server.Serve(listener) }()
	defer server.Stop()
	// In-memory transport ONLY. The production server and CLI require mTLS.
	conn, err := grpc.NewClient("passthrough:///test", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	client := pb.NewReplicaServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	d, err := client.GetReplicas(ctx, &pb.GetReplicasRequest{Namespace: "default", Name: "demo"})
	if err != nil || d.GetReplicas() != 2 {
		t.Fatalf("d=%v err=%v", d, err)
	}
	_, err = client.GetReplicas(ctx, &pb.GetReplicasRequest{Namespace: "default", Name: "missing"})
	if status.Code(err) != codes.NotFound {
		t.Fatal(err)
	}
	_, err = client.SetReplicas(ctx, &pb.SetReplicasRequest{Namespace: "default", Name: "demo"})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatal("omitted replicas accepted")
	}
	zero := int32(0)
	set, err := client.SetReplicas(ctx, &pb.SetReplicasRequest{Namespace: "default", Name: "demo", Replicas: &zero})
	if err != nil || set.Replicas != 0 || !set.Accepted {
		t.Fatalf("zero: %v %v", set, err)
	}
	_, err = client.SetReplicas(ctx, &pb.SetReplicasRequest{Namespace: "default", Name: "demo", Replicas: &zero, ExpectedVersion: "stale"})
	if status.Code(err) != codes.Aborted {
		t.Fatal(err)
	}
	list, err := client.ListDeployments(ctx, &pb.ListDeploymentsRequest{})
	if err != nil || len(list.Deployments) != 1 {
		t.Fatalf("list: %v %v", list, err)
	}
}
