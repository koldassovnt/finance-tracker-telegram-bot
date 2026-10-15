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

		timer := time.NewTimer(time.Until(next))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}

		runCtx, cancel := context.WithTimeout(ctx, timeout)
		if err := job(runCtx, next); err != nil {
			slog.Error("daily job failed", "err", err)
		}
		cancel()
	}
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
