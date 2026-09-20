package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestMonitor(t *testing.T) {
	for _, tc := range []struct {
		name                                           string
		finish, initialFailure, lateFailure, wantError bool
	}{
		{"entire rollout and settling", true, false, false, false},
		{"missing finish fails deadline", false, false, false, true},
		{"failed initial request never ready", true, true, false, true},
		{"outage after Helm completion", true, false, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			finished := make(chan struct{})
			var out bytes.Buffer
			ready, samples := false, 0
			check := func() error {
				samples++
				if tc.initialFailure || (tc.lateFailure && samples > 1) {
					return errors.New("service unavailable")
				}
				return nil
			}
			start := time.Now()
			err := monitor(100*time.Millisecond, 5*time.Millisecond, 20*time.Millisecond, true, finished, check, func() {
				ready = true
				if tc.finish {
					close(finished)
				}
			}, &out)
			if (err != nil) != tc.wantError {
				t.Fatalf("error=%v log=%s", err, out.String())
			}
			if tc.initialFailure && (ready || strings.Contains(out.String(), "probe started")) {
				t.Fatal("announced readiness without a successful request")
			}
			if !tc.wantError && (time.Since(start) < 20*time.Millisecond || samples < 2) {
				t.Fatal("settling observation was skipped")
			}
		})
	}
}
