// Package grpcapi adapts the shared service to the level-5 development gRPC contract.
package grpcapi

import (
	"context"
	"errors"
	"log/slog"
	"time"

	pb "example.com/replica-control/gen/replicas/v1"
	"example.com/replica-control/internal/model"
	"example.com/replica-control/internal/service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Server implements the generated ReplicaService handlers.
// Configure Service before registration; the gRPC listener owns transport security.
type Server struct {
	pb.UnimplementedReplicaServiceServer
	Service *service.Service
}

// GetReplicas returns the cached Deployment view supplied by the level-5 backend.
func (s *Server) GetReplicas(ctx context.Context, r *pb.GetReplicasRequest) (*pb.Deployment, error) {
	d, err := s.Service.Get(ctx, model.Target{Namespace: r.GetNamespace(), Name: r.GetName()})
	if err != nil {
		return nil, convertError(err)
	}
	return toProto(d), nil
}

// SetReplicas validates input and persists desired intent through the service.
func (s *Server) SetReplicas(ctx context.Context, r *pb.SetReplicasRequest) (*pb.SetReplicasResponse, error) {
	if r == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	d, err := s.Service.Set(ctx, model.Target{Namespace: r.GetNamespace(), Name: r.GetName()}, model.SetRequest{Replicas: r.Replicas, ExpectedVersion: r.GetExpectedVersion()})
	if err != nil {
		return nil, convertError(err)
	}
	return &pb.SetReplicasResponse{Namespace: d.Namespace, Name: d.Name, Replicas: d.Replicas, Version: d.Version, Accepted: d.Accepted}, nil
}

// ListDeployments returns cached views for one namespace or the whole cluster.
func (s *Server) ListDeployments(ctx context.Context, r *pb.ListDeploymentsRequest) (*pb.ListDeploymentsResponse, error) {
	ds, err := s.Service.List(ctx, r.GetNamespace())
	if err != nil {
		return nil, convertError(err)
	}
	out := &pb.ListDeploymentsResponse{Deployments: make([]*pb.Deployment, 0, len(ds))}
	for _, d := range ds {
		out.Deployments = append(out.Deployments, toProto(d))
	}
	return out, nil
}
func toProto(d model.Deployment) *pb.Deployment {
	return &pb.Deployment{Namespace: d.Namespace, Name: d.Name, Uid: d.UID, ResourceVersion: d.ResourceVersion,
		Replicas: d.Replicas, ReadyReplicas: d.ReadyReplicas, AvailableReplicas: d.AvailableReplicas,
		DesiredReplicas: d.DesiredReplicas, IntentVersion: d.IntentVersion, ReconcilePhase: d.ReconcilePhase,
		Generation: d.Generation, ObservedGeneration: d.ObservedGeneration}
}
func convertError(err error) error {
	if errors.Is(err, context.Canceled) {
		return status.Error(codes.Canceled, "request canceled")
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return status.Error(codes.DeadlineExceeded, "request timed out")
	}
	code := map[model.Code]codes.Code{
		model.InvalidArgument: codes.InvalidArgument, model.NotFound: codes.NotFound,
		model.Conflict: codes.Aborted, model.Forbidden: codes.PermissionDenied,
		model.Unavailable: codes.Unavailable, model.FailedPrecondition: codes.FailedPrecondition,
		model.Internal: codes.Internal,
	}[model.ErrorCode(err)]
	return status.Error(code, model.PublicError(err))
}

// Interceptor bounds server work even when the caller omits a deadline.
func Interceptor(log *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		defer func() {
			if p := recover(); p != nil {
				log.Error("gRPC handler panic", "method", info.FullMethod, "panic", p)
				err = status.Error(codes.Internal, "internal server error")
			}
		}()
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		return handler(ctx, req)
	}
}
