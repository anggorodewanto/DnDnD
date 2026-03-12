package renderer

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestRenderQueue_SingleRequest(t *testing.T) {
	var renderCount atomic.Int32
	renderFn := func(md *MapData) ([]byte, error) {
		renderCount.Add(1)
		return []byte("png"), nil
	}

	q := NewRenderQueue(50*time.Millisecond, renderFn)
	defer q.Stop()

	md := &MapData{Width: 3, Height: 3, TileSize: 48}
	q.Enqueue("enc1", md)

	// Wait for debounce + render
	time.Sleep(150 * time.Millisecond)

	if renderCount.Load() != 1 {
		t.Errorf("expected 1 render, got %d", renderCount.Load())
	}
}

func TestRenderQueue_DebounceCoalescesMultipleRequests(t *testing.T) {
	var renderCount atomic.Int32
	renderFn := func(md *MapData) ([]byte, error) {
		renderCount.Add(1)
		return []byte("png"), nil
	}

	q := NewRenderQueue(100*time.Millisecond, renderFn)
	defer q.Stop()

	md := &MapData{Width: 3, Height: 3, TileSize: 48}

	// Rapid-fire enqueue within debounce window
	for i := 0; i < 5; i++ {
		q.Enqueue("enc1", md)
		time.Sleep(10 * time.Millisecond)
	}

	// Wait for debounce to settle + render
	time.Sleep(200 * time.Millisecond)

	count := renderCount.Load()
	if count > 2 {
		t.Errorf("expected at most 2 renders (debounced), got %d", count)
	}
}

func TestRenderQueue_DifferentEncountersRenderedSeparately(t *testing.T) {
	var mu sync.Mutex
	renderCalls := map[string]int{}
	renderFn := func(md *MapData) ([]byte, error) {
		mu.Lock()
		// Use width as identifier since we control test data
		key := "unknown"
		if md.Width == 3 {
			key = "enc1"
		} else if md.Width == 5 {
			key = "enc2"
		}
		renderCalls[key]++
		mu.Unlock()
		return []byte("png"), nil
	}

	q := NewRenderQueue(50*time.Millisecond, renderFn)
	defer q.Stop()

	q.Enqueue("enc1", &MapData{Width: 3, Height: 3, TileSize: 48})
	q.Enqueue("enc2", &MapData{Width: 5, Height: 5, TileSize: 48})

	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if renderCalls["enc1"] < 1 || renderCalls["enc2"] < 1 {
		t.Errorf("expected both encounters to render, got %v", renderCalls)
	}
}

func TestRenderQueue_CallbackReceivesResult(t *testing.T) {
	renderFn := func(md *MapData) ([]byte, error) {
		return []byte("test-png-data"), nil
	}

	q := NewRenderQueue(50*time.Millisecond, renderFn)
	defer q.Stop()

	var gotData []byte
	var wg sync.WaitGroup
	wg.Add(1)

	q.OnComplete("enc1", func(data []byte, err error) {
		gotData = data
		wg.Done()
	})

	q.Enqueue("enc1", &MapData{Width: 3, Height: 3, TileSize: 48})

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		if string(gotData) != "test-png-data" {
			t.Errorf("callback data = %q, want %q", gotData, "test-png-data")
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for callback")
	}
}

func TestRenderQueue_Stop(t *testing.T) {
	renderFn := func(md *MapData) ([]byte, error) {
		return []byte("png"), nil
	}

	q := NewRenderQueue(50*time.Millisecond, renderFn)
	q.Stop()

	// Enqueue after stop should not panic
	q.Enqueue("enc1", &MapData{Width: 3, Height: 3, TileSize: 48})
}
