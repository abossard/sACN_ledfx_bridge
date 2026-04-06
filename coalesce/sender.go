package coalesce

import "sync"

// Sender dispatches values to a handler function, ensuring only the
// latest value is processed. If a new value arrives while the handler
// is running, it replaces any pending value. Intermediate values are
// skipped. The handler is never called concurrently.
type Sender[T any] struct {
	ch   chan T
	done chan struct{}
	once sync.Once
}

// New creates a Sender that calls handler for each value.
// A background goroutine is started immediately.
func New[T any](handler func(T)) *Sender[T] {
	s := &Sender[T]{
		ch:   make(chan T, 1),
		done: make(chan struct{}),
	}
	go func() {
		defer close(s.done)
		for val := range s.ch {
			handler(val)
		}
	}()
	return s
}

// Send enqueues a value. If a value is already pending, it is replaced.
// Never blocks.
func (s *Sender[T]) Send(val T) {
	// Drain any pending value, then send the new one.
	select {
	case <-s.ch:
	default:
	}
	select {
	case s.ch <- val:
	default:
		// Channel was just filled between drain and send (rare race).
		// Drain again and retry.
		select {
		case <-s.ch:
		default:
		}
		s.ch <- val
	}
}

// Close stops the sender and waits for any in-flight handler to finish.
func (s *Sender[T]) Close() {
	s.once.Do(func() {
		close(s.ch)
	})
	<-s.done
}
