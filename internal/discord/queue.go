package discord

import (
	"math/rand/v2"
	"sync"
	"time"
)

// queueItem represents a message waiting to be sent.
type queueItem struct {
	channelID string
	content   string
	errCh     chan error
}

// channelQueue manages a per-channel FIFO queue with rate-limit awareness.
type channelQueue struct {
	items    []queueItem
	draining bool
	backoff  time.Time // when the channel is unblocked after a 429
}

// MessageQueue provides per-channel outbound message queuing with rate-limit backoff.
type MessageQueue struct {
	session  Session
	mu       sync.Mutex
	channels map[string]*channelQueue
	done     chan struct{}

	// sleepFunc is injectable for testing (defaults to time.Sleep).
	sleepFunc func(time.Duration)
	// nowFunc is injectable for testing (defaults to time.Now).
	nowFunc func() time.Time
	// sendFunc wraps the actual send call, injectable for 429 simulation.
	sendFunc func(channelID, content string) (retryAfter time.Duration, err error)
}

// NewMessageQueue creates a new MessageQueue that sends via the given session.
func NewMessageQueue(s Session) *MessageQueue {
	mq := &MessageQueue{
		session:   s,
		channels:  make(map[string]*channelQueue),
		done:      make(chan struct{}),
		sleepFunc: time.Sleep,
		nowFunc:   time.Now,
	}
	mq.sendFunc = mq.defaultSend
	return mq
}

// Stop signals the queue to stop processing.
func (mq *MessageQueue) Stop() {
	select {
	case <-mq.done:
	default:
		close(mq.done)
	}
}

// Send enqueues a message for the given channel and returns when it has been sent (or fails).
func (mq *MessageQueue) Send(channelID, content string) error {
	errCh := make(chan error, 1)
	item := queueItem{
		channelID: channelID,
		content:   content,
		errCh:     errCh,
	}

	mq.mu.Lock()
	cq, ok := mq.channels[channelID]
	if !ok {
		cq = &channelQueue{}
		mq.channels[channelID] = cq
	}
	cq.items = append(cq.items, item)
	if !cq.draining {
		cq.draining = true
		go mq.drain(channelID)
	}
	mq.mu.Unlock()

	return <-errCh
}

// drain processes items from a channel's queue sequentially.
func (mq *MessageQueue) drain(channelID string) {
	for {
		select {
		case <-mq.done:
			mq.flushErrors(channelID, ErrQueueStopped)
			return
		default:
		}

		mq.mu.Lock()
		cq := mq.channels[channelID]
		if len(cq.items) == 0 {
			cq.draining = false
			mq.mu.Unlock()
			return
		}

		// Check backoff
		if now := mq.nowFunc(); now.Before(cq.backoff) {
			wait := cq.backoff.Sub(now)
			mq.mu.Unlock()
			// Add jitter: 0-100ms
			jitter := time.Duration(rand.Int64N(100)) * time.Millisecond
			mq.sleepFunc(wait + jitter)
			continue
		}

		item := cq.items[0]
		cq.items = cq.items[1:]
		mq.mu.Unlock()

		retryAfter, err := mq.sendFunc(channelID, item.content)
		if retryAfter > 0 {
			// Rate limited — re-enqueue at front and set backoff
			mq.mu.Lock()
			cq := mq.channels[channelID]
			cq.items = append([]queueItem{item}, cq.items...)
			cq.backoff = mq.nowFunc().Add(retryAfter)
			mq.mu.Unlock()
			continue
		}

		item.errCh <- err
	}
}

func (mq *MessageQueue) flushErrors(channelID string, err error) {
	mq.mu.Lock()
	cq := mq.channels[channelID]
	items := cq.items
	cq.items = nil
	cq.draining = false
	mq.mu.Unlock()

	for _, item := range items {
		item.errCh <- err
	}
}

func (mq *MessageQueue) defaultSend(channelID, content string) (time.Duration, error) {
	_, err := mq.session.ChannelMessageSend(channelID, content)
	return 0, err
}

// ErrQueueStopped is returned when the queue is stopped while messages are pending.
var ErrQueueStopped = errQueueStopped{}

type errQueueStopped struct{}

func (errQueueStopped) Error() string { return "message queue stopped" }
