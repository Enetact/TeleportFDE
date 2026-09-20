package main

import (
	"fmt"
	"io"
	"time"
)

// monitor requires a successful initial sample before announcing readiness.
// Controlled runs fail on the overall deadline, including when a finish signal
// arrives too late to cover the complete post-upgrade observation period.
func monitor(duration, interval, settle time.Duration, controlled bool, finish <-chan struct{}, check func() error, ready func(), out io.Writer) error {
	expires := time.Now().Add(duration)
	deadline := time.NewTimer(duration)
	defer deadline.Stop()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	var settled <-chan time.Time
	var settleTimer *time.Timer
	defer func() {
		if settleTimer != nil {
			settleTimer.Stop()
		}
	}()
	total, failed := 0, 0
	var longest time.Duration
	defer func() { fmt.Fprintf(out, "requests=%d failures=%d longest=%s\n", total, failed, longest) }()
	sample := func() error {
		start := time.Now()
		err := check()
		if elapsed := time.Since(start); elapsed > longest {
			longest = elapsed
		}
		total++
		if err != nil {
			failed++
			return fmt.Errorf("request %d failed: %w", total, err)
		}
		return nil
	}
	if err := sample(); err != nil {
		return err
	}
	ready()
	fmt.Fprintln(out, "probe started: authenticated request succeeded")
	for {
		select {
		case <-deadline.C:
			if controlled {
				return fmt.Errorf("probe deadline reached before confirmed rollout and settling completed")
			}
			return nil
		case <-finish:
			finish = nil
			fmt.Fprintln(out, "rollout finished: observing post-upgrade availability")
			settleTimer = time.NewTimer(settle)
			settled = settleTimer.C
		case <-settled:
			if controlled && !time.Now().Before(expires) {
				return fmt.Errorf("probe deadline reached during post-upgrade observation")
			}
			return sample()
		case <-ticker.C:
			if err := sample(); err != nil {
				return err
			}
		}
	}
}
