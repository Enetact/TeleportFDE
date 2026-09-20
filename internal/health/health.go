// Package health separates process liveness from dependency readiness.
package health

import (
	"context"
	"net/http"
	"sync/atomic"
	"time"
)

type State struct {
	synced    func() bool
	liveCheck func(context.Context) error
	lastGood  atomic.Int64
	lastOK    atomic.Bool
	draining  atomic.Bool
}

func New(synced func() bool, check func(context.Context) error) *State {
	return &State{synced: synced, liveCheck: check}
}
func (s *State) Drain() { s.draining.Store(true) }
func (s *State) Ready() bool {
	last := s.lastGood.Load()
	return !s.draining.Load() && s.synced() && s.lastOK.Load() && last > 0 && time.Since(time.Unix(0, last)) < 12*time.Second
}
func (s *State) Run(ctx context.Context) {
	probe := func() {
		checkCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		err := s.liveCheck(checkCtx)
		s.lastOK.Store(err == nil)
		if err == nil {
			s.lastGood.Store(time.Now().UnixNano())
		}
	}
	probe()
	timer := time.NewTicker(3 * time.Second)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			probe()
		}
	}
}
func (s *State) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /livez", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})
	ready := func(w http.ResponseWriter, r *http.Request) {
		if !s.Ready() {
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte("ok\n"))
	}
	mux.HandleFunc("GET /readyz", ready)
	mux.HandleFunc("GET /healthz", ready)
	return mux
}
