package collab

import (
	"errors"
	"sync"
	"time"
)

// maxQueued bounds the bytes waiting for a slow peer; a peer that falls
// that far behind is dropped.
const maxQueued = 512 << 20

var errTooSlow = errors.New("the connection is too slow to keep up with the edits")

type frame struct {
	kind    byte
	payload []byte
}

// outbox queues frames for a connection without blocking the sender; one
// goroutine writes them in order.
type outbox struct {
	mu     sync.Mutex
	wake   *sync.Cond
	frames []frame
	queued int
	closed bool
	err    error
}

func newOutbox() *outbox {
	box := &outbox{}
	box.wake = sync.NewCond(&box.mu)
	return box
}

func (box *outbox) push(kind byte, payload []byte) {
	box.mu.Lock()
	defer box.mu.Unlock()
	if box.closed {
		return
	}
	if box.queued+len(payload) > maxQueued {
		box.err = errTooSlow
		box.closed = true
		box.wake.Broadcast()
		return
	}
	box.frames = append(box.frames, frame{kind, payload})
	box.queued += len(payload)
	box.wake.Signal()
}

func (box *outbox) close() {
	box.mu.Lock()
	box.closed = true
	box.wake.Broadcast()
	box.mu.Unlock()
}

// run writes the queued frames to c until the outbox closes and is empty or
// a write fails. It sends a ping when nothing was sent for a while, so the
// other side knows the connection is alive.
func (box *outbox) run(c *conn) error {
	stopPing := make(chan struct{})
	defer close(stopPing)
	go func() {
		ticker := time.NewTicker(pingEvery)
		defer ticker.Stop()
		for {
			select {
			case <-stopPing:
				return
			case <-ticker.C:
				box.mu.Lock()
				if len(box.frames) == 0 && !box.closed {
					box.frames = append(box.frames, frame{kind: kindPing})
					box.wake.Signal()
				}
				box.mu.Unlock()
			}
		}
	}()
	for {
		box.mu.Lock()
		for len(box.frames) == 0 && !box.closed {
			box.wake.Wait()
		}
		if len(box.frames) == 0 || box.err != nil {
			err := box.err
			box.mu.Unlock()
			return err
		}
		next := box.frames[0]
		box.frames[0] = frame{}
		box.frames = box.frames[1:]
		box.queued -= len(next.payload)
		box.mu.Unlock()
		if err := c.send(next.kind, next.payload); err != nil {
			box.mu.Lock()
			box.closed = true
			box.frames = nil
			box.mu.Unlock()
			return err
		}
	}
}
