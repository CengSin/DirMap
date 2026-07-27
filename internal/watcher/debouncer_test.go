package watcher

import (
	"testing"
	"time"
)

func TestDebouncerStopDoesNotRequireInputChannelClose(t *testing.T) {
	in := make(chan string)
	d := NewDebouncer(time.Hour, time.Hour, in)

	done := make(chan struct{})
	go func() {
		d.Stop()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Stop blocked while the input channel remained open")
	}
}
