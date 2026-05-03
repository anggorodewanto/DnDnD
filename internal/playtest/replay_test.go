package playtest_test

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/playtest"
)

type recordingDispatcher struct {
	got []playtest.ParsedCommand
	err error
}

func (r *recordingDispatcher) Dispatch(cmd playtest.ParsedCommand) error {
	r.got = append(r.got, cmd)
	return r.err
}

type queueObserver struct {
	queue []string
	err   error
}

func (q *queueObserver) Wait(time.Duration) (string, error) {
	if q.err != nil {
		return "", q.err
	}
	if len(q.queue) == 0 {
		return "", errors.New("no more observations")
	}
	out := q.queue[0]
	q.queue = q.queue[1:]
	return out, nil
}

func TestDefaultNormalize(t *testing.T) {
	in := "Aria  moved to A1 (id=550e8400-e29b-41d4-a716-446655440000)\n"
	got := playtest.DefaultNormalize(in)
	assert.Equal(t, "Aria moved to A1 (id=<uuid>)", got)
}

func TestReplay_Roundtrip(t *testing.T) {
	transcript := []playtest.TranscriptEntry{
		{Direction: playtest.DirectionDispatch, Command: "/move A1"},
		{Direction: playtest.DirectionObserved, Content: "Aria moved to A1."},
		{Direction: playtest.DirectionDispatch, Command: "/done"},
		{Direction: playtest.DirectionObserved, Content: "Turn ended."},
	}
	disp := &recordingDispatcher{}
	obs := &queueObserver{queue: []string{"Aria moved to A1.", "Turn ended."}}

	err := playtest.Replay(transcript, disp, obs, playtest.ReplayOptions{})
	require.NoError(t, err)

	require.Len(t, disp.got, 2)
	assert.Equal(t, "move", disp.got[0].Name)
	assert.Equal(t, []string{"A1"}, disp.got[0].Args)
	assert.Equal(t, "done", disp.got[1].Name)
}

func TestReplay_NormalizesUUIDs(t *testing.T) {
	transcript := []playtest.TranscriptEntry{
		{Direction: playtest.DirectionDispatch, Command: "/status"},
		{Direction: playtest.DirectionObserved, Content: "Aria (id=00000000-0000-0000-0000-000000000000) is OK."},
	}
	disp := &recordingDispatcher{}
	obs := &queueObserver{queue: []string{
		"Aria (id=550e8400-e29b-41d4-a716-446655440000) is OK.",
	}}
	require.NoError(t, playtest.Replay(transcript, disp, obs, playtest.ReplayOptions{}))
}

func TestReplay_DriftFailsLoudly(t *testing.T) {
	transcript := []playtest.TranscriptEntry{
		{Direction: playtest.DirectionDispatch, Command: "/move A1"},
		{Direction: playtest.DirectionObserved, Content: "Aria moved to A1."},
	}
	disp := &recordingDispatcher{}
	obs := &queueObserver{queue: []string{"Aria fell into the pit."}}

	err := playtest.Replay(transcript, disp, obs, playtest.ReplayOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "drift")
	assert.Contains(t, err.Error(), "Aria moved to A1.")
}

func TestReplay_DispatchError(t *testing.T) {
	transcript := []playtest.TranscriptEntry{
		{Direction: playtest.DirectionDispatch, Command: "/move A1"},
	}
	disp := &recordingDispatcher{err: errors.New("router down")}
	err := playtest.Replay(transcript, disp, &queueObserver{}, playtest.ReplayOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "router down")
}

func TestReplay_BadCommand(t *testing.T) {
	transcript := []playtest.TranscriptEntry{
		{Direction: playtest.DirectionDispatch, Command: "not a slash"},
	}
	err := playtest.Replay(transcript, &recordingDispatcher{}, &queueObserver{}, playtest.ReplayOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse")
}

func TestReplay_ObserverError(t *testing.T) {
	transcript := []playtest.TranscriptEntry{
		{Direction: playtest.DirectionObserved, Content: "x"},
	}
	err := playtest.Replay(transcript, &recordingDispatcher{}, &queueObserver{err: errors.New("timeout")}, playtest.ReplayOptions{})
	require.Error(t, err)
}

func TestReplay_UnknownDirection(t *testing.T) {
	transcript := []playtest.TranscriptEntry{
		{Direction: playtest.Direction("weird")},
	}
	err := playtest.Replay(transcript, &recordingDispatcher{}, &queueObserver{}, playtest.ReplayOptions{})
	require.Error(t, err)
}

func TestReplay_CustomNormalize(t *testing.T) {
	called := false
	norm := func(s string) string { called = true; return s }
	transcript := []playtest.TranscriptEntry{
		{Direction: playtest.DirectionObserved, Content: "ok"},
	}
	obs := &queueObserver{queue: []string{"ok"}}
	require.NoError(t, playtest.Replay(transcript, &recordingDispatcher{}, obs, playtest.ReplayOptions{Normalize: norm, WaitTimeout: time.Second}))
	assert.True(t, called)
}
