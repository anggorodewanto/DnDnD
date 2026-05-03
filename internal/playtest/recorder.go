package playtest

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"
)

// Direction marks whether a TranscriptEntry was sent by the player
// (`player→bot`) or observed from the bot (`bot→player`). The Phase 120
// replay bridge uses Direction to decide whether to inject the entry as
// an interaction or assert it against the harness's outbound buffer.
type Direction string

const (
	// DirectionDispatch is a slash command the player issued (or the CLI
	// printed for the human to paste).
	DirectionDispatch Direction = "dispatch"
	// DirectionObserved is a message the bot posted that the CLI saw via
	// the gateway.
	DirectionObserved Direction = "observed"
)

// TranscriptEntry is one line in the JSON-lines transcript file. The
// schema is intentionally narrow: timestamps, channel ID, author tag,
// the canonical command form (for dispatch) or the message content
// (for observed). Replays normalize Timestamp + IDs before comparing.
type TranscriptEntry struct {
	Timestamp time.Time `json:"ts"`
	Direction Direction `json:"dir"`
	ChannelID string    `json:"channel_id"`
	Author    string    `json:"author,omitempty"`
	Command   string    `json:"command,omitempty"`
	Content   string    `json:"content,omitempty"`
}

// Recorder appends TranscriptEntry rows as JSON-lines to an io.Writer.
// All methods are safe for concurrent use — the gateway observer
// goroutine and the REPL goroutine both call Append.
type Recorder struct {
	mu  sync.Mutex
	w   io.Writer
	enc *json.Encoder
	now func() time.Time
}

// NewRecorder wraps w in a Recorder. Callers own w (closing it is the
// caller's responsibility). Pass nil for `now` to use time.Now.
func NewRecorder(w io.Writer, now func() time.Time) *Recorder {
	if now == nil {
		now = time.Now
	}
	return &Recorder{w: w, enc: json.NewEncoder(w), now: now}
}

// Dispatch records a slash command the player just issued (or the CLI
// printed for paste). Returns the underlying writer's error so the REPL
// can fail loudly if the transcript file becomes unwritable mid-session.
func (r *Recorder) Dispatch(channelID, author string, cmd ParsedCommand) error {
	return r.append(TranscriptEntry{
		Timestamp: r.now().UTC(),
		Direction: DirectionDispatch,
		ChannelID: channelID,
		Author:    author,
		Command:   Format(cmd),
	})
}

// Observe records a message the gateway observer just saw.
func (r *Recorder) Observe(channelID, author, content string) error {
	return r.append(TranscriptEntry{
		Timestamp: r.now().UTC(),
		Direction: DirectionObserved,
		ChannelID: channelID,
		Author:    author,
		Content:   content,
	})
}

func (r *Recorder) append(e TranscriptEntry) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.enc.Encode(e); err != nil {
		return fmt.Errorf("transcript encode: %w", err)
	}
	return nil
}

// LoadTranscript decodes a JSON-lines transcript stream into a slice
// of TranscriptEntry rows. Used by the Phase 120 replay bridge.
func LoadTranscript(r io.Reader) ([]TranscriptEntry, error) {
	dec := json.NewDecoder(r)
	var out []TranscriptEntry
	for dec.More() {
		var e TranscriptEntry
		if err := dec.Decode(&e); err != nil {
			return nil, fmt.Errorf("transcript decode: %w", err)
		}
		out = append(out, e)
	}
	return out, nil
}
