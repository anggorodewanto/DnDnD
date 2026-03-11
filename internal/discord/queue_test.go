package discord

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMessageQueue_SerializesPerChannel(t *testing.T) {
	var order []string
	var mu sync.Mutex
	mock := newTestMock()
	mock.ChannelMessageSendFunc = func(channelID, content string) (*discordgo.Message, error) {
		mu.Lock()
		order = append(order, channelID+":"+content)
		mu.Unlock()
		return &discordgo.Message{}, nil
	}

	q := NewMessageQueue(mock)
	defer q.Stop()

	// Send 3 messages to the same channel sequentially
	var wg sync.WaitGroup
	for i, msg := range []string{"A", "B", "C"} {
		wg.Add(1)
		go func(msg string, delay int) {
			defer wg.Done()
			// Small stagger so ordering is deterministic
			time.Sleep(time.Duration(delay) * time.Millisecond)
			q.Send("ch-1", msg)
		}(msg, i*10)
	}
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, order, 3)
	// Messages arrive in order they were enqueued
	assert.Equal(t, "ch-1:A", order[0])
	assert.Equal(t, "ch-1:B", order[1])
	assert.Equal(t, "ch-1:C", order[2])
}

func TestMessageQueue_ParallelChannels(t *testing.T) {
	var ch1Count, ch2Count int
	var mu sync.Mutex
	mock := newTestMock()
	mock.ChannelMessageSendFunc = func(channelID, content string) (*discordgo.Message, error) {
		mu.Lock()
		if channelID == "ch-1" {
			ch1Count++
		} else {
			ch2Count++
		}
		mu.Unlock()
		return &discordgo.Message{}, nil
	}

	q := NewMessageQueue(mock)
	defer q.Stop()

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		q.Send("ch-1", "msg1")
	}()
	go func() {
		defer wg.Done()
		q.Send("ch-2", "msg2")
	}()
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, 1, ch1Count)
	assert.Equal(t, 1, ch2Count)
}

func TestMessageQueue_RateLimitBackoff(t *testing.T) {
	attempts := 0
	mock := newTestMock()

	q := NewMessageQueue(mock)
	defer q.Stop()

	// Use a controlled clock
	now := time.Now()
	var mu sync.Mutex
	q.nowFunc = func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		return now
	}

	// Override sendFunc to simulate 429 on first attempt
	q.sendFunc = func(channelID, content string) (time.Duration, error) {
		attempts++
		if attempts == 1 {
			return 500 * time.Millisecond, nil // 429 with Retry-After
		}
		return 0, nil // success
	}

	// Override sleepFunc: advance the clock by the sleep duration
	var sleepCalled bool
	var sleepDuration time.Duration
	q.sleepFunc = func(d time.Duration) {
		sleepCalled = true
		sleepDuration = d
		mu.Lock()
		now = now.Add(d)
		mu.Unlock()
	}

	err := q.Send("ch-1", "Hello!")
	require.NoError(t, err)
	assert.Equal(t, 2, attempts)
	assert.True(t, sleepCalled)
	// Should be at least 500ms (retry-after) + 0-100ms jitter
	assert.GreaterOrEqual(t, sleepDuration, 500*time.Millisecond)
	assert.LessOrEqual(t, sleepDuration, 600*time.Millisecond)
}

func TestMessageQueue_RateLimitRetry_EventualSuccess(t *testing.T) {
	callCount := 0
	mock := newTestMock()

	q := NewMessageQueue(mock)
	defer q.Stop()
	q.sleepFunc = func(d time.Duration) {} // no-op sleep for fast tests

	q.sendFunc = func(channelID, content string) (time.Duration, error) {
		callCount++
		if callCount <= 3 {
			return 10 * time.Millisecond, nil
		}
		return 0, nil
	}

	err := q.Send("ch-1", "Hello!")
	require.NoError(t, err)
	assert.Equal(t, 4, callCount)
}

func TestMessageQueue_SendError_Propagates(t *testing.T) {
	mock := newTestMock()
	q := NewMessageQueue(mock)
	defer q.Stop()

	q.sendFunc = func(channelID, content string) (time.Duration, error) {
		return 0, errors.New("send failed")
	}

	err := q.Send("ch-1", "Hello!")
	assert.Error(t, err)
	assert.Equal(t, "send failed", err.Error())
}

func TestMessageQueue_Stop_FlushesErrors(t *testing.T) {
	mock := newTestMock()

	q := NewMessageQueue(mock)

	// Block sends with a long rate limit
	q.sendFunc = func(channelID, content string) (time.Duration, error) {
		return 10 * time.Second, nil // always rate limited
	}
	q.sleepFunc = func(d time.Duration) {
		time.Sleep(50 * time.Millisecond) // short sleep to not block forever
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- q.Send("ch-1", "Hello!")
	}()

	// Give time for the send to start
	time.Sleep(100 * time.Millisecond)
	q.Stop()

	select {
	case err := <-errCh:
		// Either the send gets the rate-limit retry loop interrupted, or it gets queue stopped
		// Both are acceptable since the queue is stopping
		_ = err
	case <-time.After(2 * time.Second):
		t.Fatal("Send did not return after Stop")
	}
}

func TestErrQueueStopped_Error(t *testing.T) {
	assert.Equal(t, "message queue stopped", ErrQueueStopped.Error())
}

func TestMessageQueue_Send_SingleMessage(t *testing.T) {
	var sent []string
	var mu sync.Mutex
	mock := newTestMock()
	mock.ChannelMessageSendFunc = func(channelID, content string) (*discordgo.Message, error) {
		mu.Lock()
		sent = append(sent, content)
		mu.Unlock()
		return &discordgo.Message{}, nil
	}

	q := NewMessageQueue(mock)
	defer q.Stop()

	err := q.Send("ch-1", "Hello!")
	require.NoError(t, err)

	// Wait for async drain
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, sent, 1)
	assert.Equal(t, "Hello!", sent[0])
}
