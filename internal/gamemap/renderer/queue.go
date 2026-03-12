package renderer

import (
	"sync"
	"time"
)

// RenderFunc is the function that performs the actual rendering.
type RenderFunc func(md *MapData) ([]byte, error)

// CompletionCallback is called when a render completes.
type CompletionCallback func(data []byte, err error)

// RenderQueue manages per-encounter render requests with debouncing.
type RenderQueue struct {
	debounce  time.Duration
	renderFn  RenderFunc
	mu        sync.Mutex
	timers    map[string]*time.Timer
	latest    map[string]*MapData
	callbacks map[string][]CompletionCallback
	stopped   bool
}

// NewRenderQueue creates a new render queue with the given debounce duration.
func NewRenderQueue(debounce time.Duration, renderFn RenderFunc) *RenderQueue {
	return &RenderQueue{
		debounce:  debounce,
		renderFn:  renderFn,
		timers:    make(map[string]*time.Timer),
		latest:    make(map[string]*MapData),
		callbacks: make(map[string][]CompletionCallback),
	}
}

// Enqueue schedules a render for the given encounter. If a render is already
// pending, it resets the debounce timer and uses the latest map data.
func (q *RenderQueue) Enqueue(encounterID string, md *MapData) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.stopped {
		return
	}

	q.latest[encounterID] = md

	if timer, ok := q.timers[encounterID]; ok {
		timer.Stop()
	}

	q.timers[encounterID] = time.AfterFunc(q.debounce, func() {
		q.executeRender(encounterID)
	})
}

// OnComplete registers a callback for when a render completes for an encounter.
func (q *RenderQueue) OnComplete(encounterID string, cb CompletionCallback) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.callbacks[encounterID] = append(q.callbacks[encounterID], cb)
}

// Stop cancels all pending timers and prevents new enqueues.
func (q *RenderQueue) Stop() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.stopped = true
	for _, timer := range q.timers {
		timer.Stop()
	}
	q.timers = make(map[string]*time.Timer)
}

// executeRender runs the render function for an encounter and notifies callbacks.
func (q *RenderQueue) executeRender(encounterID string) {
	q.mu.Lock()
	md := q.latest[encounterID]
	delete(q.timers, encounterID)
	delete(q.latest, encounterID)
	cbs := q.callbacks[encounterID]
	q.mu.Unlock()

	if md == nil {
		return
	}

	data, err := q.renderFn(md)

	for _, cb := range cbs {
		cb(data, err)
	}
}
