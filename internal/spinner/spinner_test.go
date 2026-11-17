package spinner

import (
	"testing"
	"time"
)

func TestSpinnerStartsAndStops(t *testing.T) {
	s := New(WithSequence(Sequence2), WithSpeed(5*time.Millisecond))
	s.Start()
	time.Sleep(30 * time.Millisecond)
	done := make(chan struct{})
	go func() { s.Stop(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop did not return")
	}
}
