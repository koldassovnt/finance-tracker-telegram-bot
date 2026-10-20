package scheduler

import (
	"context"
	"testing"
	"time"
)

// TestSleepUntilEarlyTimer simulates a timer that fires while the wall clock
// is still short of the target: sleepUntil must sleep again rather than return.
func TestSleepUntilEarlyTimer(t *testing.T) {
	remaining := []time.Duration{time.Millisecond, time.Millisecond, time.Millisecond, 0}
	calls := 0
	until = func(time.Time) time.Duration {
		d := remaining[calls]
		calls++
		return d
	}
	defer func() { until = time.Until }()

	if !sleepUntil(context.Background(), time.Now()) {
		t.Fatal("sleepUntil returned false with a live context")
	}
	if calls != len(remaining) {
		t.Errorf("until called %d times; want %d (sleepUntil returned before the wall clock reached the target)", calls, len(remaining))
	}
}

func TestSleepUntilCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if sleepUntil(ctx, time.Now().Add(time.Hour)) {
		t.Error("sleepUntil returned true after ctx was canceled")
	}
	if sleepUntil(ctx, time.Now().Add(-time.Hour)) {
		t.Error("sleepUntil returned true for a past target after ctx was canceled")
	}
}

func TestNextRun(t *testing.T) {
	almaty, err := time.LoadLocation("Asia/Almaty")
	if err != nil {
		t.Fatal(err)
	}
	ten := 10 * time.Hour

	tests := []struct {
		name string
		now  time.Time
		want time.Time
	}{
		{"before", time.Date(2026, 10, 15, 9, 59, 0, 0, almaty), time.Date(2026, 10, 15, 10, 0, 0, 0, almaty)},
		{"exactly at", time.Date(2026, 10, 15, 10, 0, 0, 0, almaty), time.Date(2026, 10, 16, 10, 0, 0, 0, almaty)},
		{"after", time.Date(2026, 10, 15, 18, 0, 0, 0, almaty), time.Date(2026, 10, 16, 10, 0, 0, 0, almaty)},
		{"month end", time.Date(2026, 10, 31, 11, 0, 0, 0, almaty), time.Date(2026, 11, 1, 10, 0, 0, 0, almaty)},
	}
	for _, tt := range tests {
		if got := NextRun(tt.now, ten); !got.Equal(tt.want) {
			t.Errorf("%s: NextRun(%v) = %v; want %v", tt.name, tt.now, got, tt.want)
		}
	}
}
