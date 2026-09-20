// Package service centralizes validation for both REST and gRPC.
package service

import (
	"context"
	"example.com/replica-control/internal/model"
)

// Service validates requests before delegating to its configured Backend.
// Set Backend before use and do not replace it while requests are running.
// Concurrent use requires a backend that supports concurrent calls.
type Service struct{ Backend model.Backend }

// Get validates the target and returns the backend view, which may be cached.
func (s *Service) Get(ctx context.Context, t model.Target) (model.Deployment, error) {
	if err := t.Validate(); err != nil {
		return model.Deployment{}, err
	}
	return s.Backend.Get(ctx, t)
}

// List validates the optional namespace filter and returns matching Deployments.
func (s *Service) List(ctx context.Context, namespace string) ([]model.Deployment, error) {
	if err := model.ValidateNamespace(namespace, true); err != nil {
		return nil, err
	}
	return s.Backend.List(ctx, namespace)
}

// Set validates presence, bounds and version length before requesting a write.
// The backend chooses direct scaling or durable intent based on the runtime level.
func (s *Service) Set(ctx context.Context, t model.Target, r model.SetRequest) (model.SetResult, error) {
	if err := t.Validate(); err != nil {
		return model.SetResult{}, err
	}
	if r.Replicas == nil {
		return model.SetResult{}, model.E(model.InvalidArgument, "replicas is required (zero is valid)", nil)
	}
	if err := model.ValidateReplicas(*r.Replicas); err != nil {
		return model.SetResult{}, err
	}
	if len(r.ExpectedVersion) > 256 {
		return model.SetResult{}, model.E(model.InvalidArgument, "expectedVersion is too long", nil)
	}
	return s.Backend.Set(ctx, t, *r.Replicas, r.ExpectedVersion)
}
