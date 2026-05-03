package playtest_test

import (
	"bytes"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/playtest"
)

func fixedClock(t time.Time) func() time.Time {
	return func() time.Time { return t }
}

func TestRecorder_DispatchAndObserve(t *testing.T) {
	var buf bytes.Buffer
	clk := fixedClock(time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC))
	rec := playtest.NewRecorder(&buf, clk)

	cmd, err := playtest.Parse("/move A1")
	require.NoError(t, err)
	require.NoError(t, rec.Dispatch("chan-1", "Aria#0001", cmd))
	require.NoError(t, rec.Observe("chan-2", "DnDnD-Bot", "Aria moved to A1."))

	entries, err := playtest.LoadTranscript(strings.NewReader(buf.String()))
	require.NoError(t, err)
	require.Len(t, entries, 2)

	assert.Equal(t, playtest.DirectionDispatch, entries[0].Direction)
	assert.Equal(t, "chan-1", entries[0].ChannelID)
	assert.Equal(t, "Aria#0001", entries[0].Author)
	assert.Equal(t, "/move A1", entries[0].Command)

	assert.Equal(t, playtest.DirectionObserved, entries[1].Direction)
	assert.Equal(t, "chan-2", entries[1].ChannelID)
	assert.Equal(t, "Aria moved to A1.", entries[1].Content)
}

func TestRecorder_ConcurrentSafe(t *testing.T) {
	var buf bytes.Buffer
	rec := playtest.NewRecorder(&buf, nil)

	var wg sync.WaitGroup
	cmd, _ := playtest.Parse("/status")
	for range 50 {
		wg.Add(2)
		go func() { defer wg.Done(); _ = rec.Dispatch("c", "p", cmd) }()
		go func() { defer wg.Done(); _ = rec.Observe("c", "b", "ok") }()
	}
	wg.Wait()

	entries, err := playtest.LoadTranscript(&buf)
	require.NoError(t, err)
	assert.Len(t, entries, 100)
}

func TestLoadTranscript_RejectsMalformed(t *testing.T) {
	_, err := playtest.LoadTranscript(strings.NewReader("{not json}\n"))
	assert.Error(t, err)
}

type errWriter struct{}

func (errWriter) Write([]byte) (int, error) { return 0, assert.AnError }

func TestRecorder_PropagatesWriteError(t *testing.T) {
	rec := playtest.NewRecorder(errWriter{}, nil)
	cmd, _ := playtest.Parse("/status")
	err := rec.Dispatch("c", "p", cmd)
	assert.Error(t, err)
}
