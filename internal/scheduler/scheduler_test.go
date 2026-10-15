package scheduler

import (
	"testing"
	"time"
)

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
