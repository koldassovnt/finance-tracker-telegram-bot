// Package scheduler runs a job once a day at a fixed wall-clock time.
package scheduler

import (
	"context"
	"log/slog"
	"time"
)

// Job is the work to run; now is the scheduled time in the scheduler's location.
type Job func(ctx context.Context, now time.Time) error

// Daily runs job every day at offset after midnight in loc until ctx is done.
// A run that is missed (the process was down) is skipped, not caught up.
func Daily(ctx context.Context, loc *time.Location, offset, timeout time.Duration, job Job) {
	for {
		next := NextRun(time.Now().In(loc), offset)
		slog.Info("next daily run scheduled", "at", next)

		if !sleepUntil(ctx, next) {
			return
		}

		runCtx, cancel := context.WithTimeout(ctx, timeout)
		if err := job(runCtx, next); err != nil {
			slog.Error("daily job failed", "err", err)
		}
		cancel()
	}
}

// until reports how long remains until t. Tests replace it to simulate a timer
// that fires before the wall clock reaches t.
var until = time.Until

// sleepUntil blocks until the wall clock reaches t and reports true, or returns
// false once ctx is done. Timers run on the monotonic clock, which can drift
// from the wall clock (e.g. inside the Docker Desktop VM) and fire a few
// seconds early; running then would make NextRun return the same t again and
// the job would run twice. t comes from time.Date and carries no monotonic
// reading, so time.Until compares wall clocks.
func sleepUntil(ctx context.Context, t time.Time) bool {
	for d := until(t); d > 0; d = until(t) {
		timer := time.NewTimer(d)
		select {
		case <-ctx.Done():
			timer.Stop()
			return false
		case <-timer.C:
		}
	}
	return ctx.Err() == nil
}

// NextRun returns the first moment strictly after now that is offset after a
// local midnight. It uses calendar arithmetic, so DST shifts don't skew it.
func NextRun(now time.Time, offset time.Duration) time.Time {
	loc := now.Location()
	h, m := int(offset/time.Hour), int(offset%time.Hour/time.Minute)
	next := time.Date(now.Year(), now.Month(), now.Day(), h, m, 0, 0, loc)
	if !next.After(now) {
		next = time.Date(now.Year(), now.Month(), now.Day()+1, h, m, 0, 0, loc)
	}
	return next
}
