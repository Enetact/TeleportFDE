package health

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"
	"time"
)

// TestHealthStates controls observations directly to isolate readiness decisions.
// It does not simulate a Kubernetes network outage.
func TestHealthStates(t *testing.T) {
	synced := false
	s := New(func() bool { return synced }, func(context.Context) error { return nil })
	if s.Ready() {
		t.Fatal("ready before initial probe")
	}
	s.lastGood.Store(time.Now().UnixNano())
	s.lastOK.Store(true)
	if s.Ready() {
		t.Fatal("ready before cache sync")
	}
	synced = true
	if !s.Ready() {
		t.Fatal("should be ready")
	}
	s.lastOK.Store(false)
	if s.Ready() {
		t.Fatal("dependency failure ignored")
	}
	s.lastOK.Store(true)
	s.lastGood.Store(time.Now().Add(-time.Minute).UnixNano())
	if s.Ready() {
		t.Fatal("stale probe accepted")
	}
	s.lastGood.Store(time.Now().UnixNano())
	s.Drain()
	if s.Ready() {
		t.Fatal("ready while draining")
	}
	for _, tc := range []struct {
		path string
		code int
	}{{"/livez", 200}, {"/readyz", 503}, {"/healthz", 503}} {
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, httptest.NewRequest("GET", tc.path, nil))
		if w.Code != tc.code {
			t.Fatalf("%s: %d", tc.path, w.Code)
		}
	}
}
func TestLiveProbeFailure(t *testing.T) {
	s := New(func() bool { return true }, func(context.Context) error { return errors.New("cluster down") })
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s.Run(ctx)
	if s.Ready() {
		t.Fatal("failed probe was healthy")
	}
}
