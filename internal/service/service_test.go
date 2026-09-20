package service

import (
	"context"
	"example.com/replica-control/internal/model"
	"testing"
)

type fakeBackend struct {
	calls    int
	replicas int32
}

func (f *fakeBackend) Get(_ context.Context, t model.Target) (model.Deployment, error) {
	f.calls++
	return model.Deployment{Target: t}, nil
}
func (f *fakeBackend) List(context.Context, string) ([]model.Deployment, error) {
	f.calls++
	return nil, nil
}
func (f *fakeBackend) Set(_ context.Context, t model.Target, n int32, v string) (model.SetResult, error) {
	f.calls++
	f.replicas = n
	return model.SetResult{Target: t, Replicas: n}, nil
}
func TestSetPresenceAndBounds(t *testing.T) {
	for _, tc := range []struct {
		name  string
		n     *int32
		valid bool
	}{
		{"missing", nil, false}, {"zero", ptr(0), true}, {"negative", ptr(-1), false}, {"one", ptr(1), true}, {"maximum", ptr(1000), true}, {"too large", ptr(1001), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := &fakeBackend{}
			s := &Service{Backend: b}
			_, err := s.Set(context.Background(), model.Target{Namespace: "default", Name: "demo"}, model.SetRequest{Replicas: tc.n})
			if (err == nil) != tc.valid {
				t.Fatalf("err=%v", err)
			}
			if !tc.valid && b.calls != 0 {
				t.Fatal("invalid input reached Kubernetes")
			}
		})
	}
}
func TestInvalidNamespaceNoBackendCall(t *testing.T) {
	b := &fakeBackend{}
	s := &Service{Backend: b}
	_, err := s.List(context.Background(), "bad/namespace")
	if err == nil || b.calls != 0 {
		t.Fatal("bad namespace accepted")
	}
}
func ptr(n int32) *int32 { return &n }
