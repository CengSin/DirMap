package watcher

import (
	"sync"
	"time"
)

type Debouncer struct {
	interval time.Duration
	maxWait  time.Duration
	inCh     <-chan string
	OutCh    chan []string
	mu       sync.Mutex
	pending  map[string]struct{}
	timer    *time.Timer
	firstAt  time.Time
	done     chan struct{}
	stop     chan struct{}
	stopOnce sync.Once
}

func NewDebouncer(interval, maxWait time.Duration, inCh <-chan string) *Debouncer {
	d := &Debouncer{
		interval: interval,
		maxWait:  maxWait,
		inCh:     inCh,
		OutCh:    make(chan []string, 10),
		pending:  make(map[string]struct{}),
		done:     make(chan struct{}),
		stop:     make(chan struct{}),
	}
	go d.run()
	return d
}

func (d *Debouncer) Stop() {
	d.stopOnce.Do(func() { close(d.stop) })
	<-d.done
}

func (d *Debouncer) run() {
	defer close(d.done)

	for {
		var path string
		var ok bool
		select {
		case <-d.stop:
			d.mu.Lock()
			d.flushLocked()
			d.mu.Unlock()
			return
		case path, ok = <-d.inCh:
			if !ok {
				d.mu.Lock()
				d.flushLocked()
				d.mu.Unlock()
				return
			}
		}

		d.mu.Lock()
		if len(d.pending) == 0 {
			d.firstAt = time.Now()
		}
		d.pending[path] = struct{}{}

		elapsed := time.Since(d.firstAt)
		if elapsed >= d.maxWait {
			d.flushLocked()
		} else {
			if d.timer != nil {
				d.timer.Stop()
			}
			d.timer = time.AfterFunc(d.interval, d.flush)
		}
		d.mu.Unlock()
	}
}

func (d *Debouncer) flush() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.flushLocked()
}

func (d *Debouncer) flushLocked() {
	if len(d.pending) == 0 {
		return
	}

	paths := make([]string, 0, len(d.pending))
	for p := range d.pending {
		paths = append(paths, p)
	}
	d.pending = make(map[string]struct{})

	if d.timer != nil {
		d.timer.Stop()
		d.timer = nil
	}

	select {
	case d.OutCh <- paths:
	case <-d.stop:
	}
}
