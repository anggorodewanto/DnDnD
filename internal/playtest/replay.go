package playtest

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Dispatcher delivers a parsed slash command to whatever surface is
// driving the replay (the Phase 120 e2e harness in production, a fake
// in tests). Implementations should block until the command has been
// accepted (the harness has called the underlying handler) so the
// Replayer's next Wait sees a stable transcript ordering.
type Dispatcher interface {
	Dispatch(cmd ParsedCommand) error
}

// Observer surfaces the next outbound message the system under test
// produced. Wait should block until a message is observed or the
// timeout expires; it returns the message content (after any callee-
// side normalization, e.g. ephemeral prefixing).
type Observer interface {
	Wait(timeout time.Duration) (string, error)
}

// Clicker performs a component (button) interaction identified by a stable
// selector (a CustomID prefix). The implementation resolves the actual
// button from the system-under-test's recent output and triggers it. It is
// only consulted for DirectionClick entries.
type Clicker interface {
	Click(selector string) error
}

// ReplayOptions configures Replay. WaitTimeout caps how long Replay
// waits for each expected observation; Normalize is applied to both
// the expected content (from the transcript) and the observed content
// before comparison so the same normalizer governs both sides.
type ReplayOptions struct {
	WaitTimeout time.Duration
	Normalize   func(string) string
	// Clicker handles DirectionClick entries. It may be nil for transcripts
	// that contain only dispatch/observed entries; a click entry with no
	// Clicker configured is a replay error.
	Clicker Clicker
}

// DefaultNormalize trims surrounding whitespace, collapses internal
// runs of whitespace to single spaces, and replaces every UUID with a
// stable `<uuid>` placeholder so transcripts captured at different
// times still compare equal.
func DefaultNormalize(s string) string {
	s = uuidRE.ReplaceAllString(s, "<uuid>")
	s = strings.TrimSpace(s)
	s = wsRE.ReplaceAllString(s, " ")
	return s
}

var (
	uuidRE = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
	wsRE   = regexp.MustCompile(`\s+`)
)

// Replay drives transcript through dispatcher + observer in order.
// For each dispatch entry the parsed command is sent; for each click
// entry the configured Clicker resolves and triggers the button; for
// each observed entry the next observation is awaited and compared
// (substring match after normalization). Returns nil if every entry
// matched, otherwise an error pinpointing the first divergence.
func Replay(transcript []TranscriptEntry, dispatcher Dispatcher, observer Observer, opts ReplayOptions) error {
	if opts.WaitTimeout <= 0 {
		opts.WaitTimeout = 5 * time.Second
	}
	if opts.Normalize == nil {
		opts.Normalize = DefaultNormalize
	}
	for i, e := range transcript {
		switch e.Direction {
		case DirectionDispatch:
			cmd, err := Parse(e.Command)
			if err != nil {
				return fmt.Errorf("entry %d: parse %q: %w", i, e.Command, err)
			}
			if err := dispatcher.Dispatch(cmd); err != nil {
				return fmt.Errorf("entry %d: dispatch %q: %w", i, e.Command, err)
			}
		case DirectionClick:
			if opts.Clicker == nil {
				return fmt.Errorf("entry %d: click %q but no Clicker configured", i, e.Command)
			}
			if err := opts.Clicker.Click(e.Command); err != nil {
				return fmt.Errorf("entry %d: click %q: %w", i, e.Command, err)
			}
		case DirectionObserved:
			got, err := observer.Wait(opts.WaitTimeout)
			if err != nil {
				return fmt.Errorf("entry %d: wait for %q: %w", i, e.Content, err)
			}
			want := opts.Normalize(e.Content)
			gotN := opts.Normalize(got)
			if !strings.Contains(gotN, want) {
				return fmt.Errorf("entry %d: drift\n  want substring: %q\n  got:            %q", i, want, gotN)
			}
		default:
			return fmt.Errorf("entry %d: unknown direction %q", i, e.Direction)
		}
	}
	return nil
}
