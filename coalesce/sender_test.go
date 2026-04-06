package coalesce

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSender(t *testing.T) {
	tests := []struct {
		name      string
		sends     []int
		wantFinal int
	}{
		{"single value", []int{42}, 42},
		{"two values", []int{1, 2}, 2},
		{"rapid burst", []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, 10},
		{"same value", []int{7, 7, 7}, 7},
		{"zero value", []int{0}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var lastSeen atomic.Int64
			var callCount atomic.Int64

			s := New(func(v int) {
				lastSeen.Store(int64(v))
				callCount.Add(1)
				// Simulate slow handler
				time.Sleep(5 * time.Millisecond)
			})

			for _, v := range tt.sends {
				s.Send(v)
			}
			s.Close()

			if got := int(lastSeen.Load()); got != tt.wantFinal {
				t.Errorf("last seen = %d, want %d", got, tt.wantFinal)
			}
			if got := int(callCount.Load()); got < 1 {
				t.Errorf("handler never called")
			}
		})
	}
}

func TestSender_LatestWinsDuringSlow(t *testing.T) {
	// Handler is slow; send many values. Only the latest should survive.
	var mu sync.Mutex
	var seen []int

	s := New(func(v int) {
		time.Sleep(50 * time.Millisecond)
		mu.Lock()
		seen = append(seen, v)
		mu.Unlock()
	})

	// Send 100 values instantly — handler can process maybe 1-2
	for i := 1; i <= 100; i++ {
		s.Send(i)
	}
	s.Close()

	mu.Lock()
	defer mu.Unlock()

	if len(seen) == 0 {
		t.Fatal("handler never called")
	}
	// The final value seen must be 100
	if seen[len(seen)-1] != 100 {
		t.Errorf("final value = %d, want 100", seen[len(seen)-1])
	}
	// Should have been called far fewer than 100 times
	if len(seen) > 10 {
		t.Errorf("handler called %d times, expected far fewer than 100", len(seen))
	}
	t.Logf("handler called %d times (out of 100 sends)", len(seen))
}

func TestSender_CloseFlushes(t *testing.T) {
	var got atomic.Int64
	s := New(func(v int) {
		got.Store(int64(v))
	})
	s.Send(99)
	s.Close()
	if got.Load() != 99 {
		t.Errorf("got %d, want 99 (Close should flush pending)", got.Load())
	}
}

func TestSender_CloseIdempotent(t *testing.T) {
	s := New(func(v int) {})
	s.Close()
	s.Close() // should not panic
}
