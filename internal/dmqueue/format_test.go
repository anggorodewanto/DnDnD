package dmqueue

import "testing"

func TestFormatEvent(t *testing.T) {
	tests := []struct {
		name  string
		event Event
		want  string
	}{
		{
			name: "freeform action",
			event: Event{
				Kind:        KindFreeformAction,
				PlayerName:  "Thorn",
				Summary:     `"flip the table"`,
				ResolvePath: "/dashboard/queue/abc",
			},
			want: `🎭 **Action** — Thorn: "flip the table" — [Resolve →](/dashboard/queue/abc)`,
		},
		{
			name: "reaction declaration",
			event: Event{
				Kind:        KindReactionDeclaration,
				PlayerName:  "Aria",
				Summary:     `"Shield if I get hit"`,
				ResolvePath: "/dashboard/queue/xyz",
			},
			want: `⚡ **Reaction** — Aria: "Shield if I get hit" — [Resolve →](/dashboard/queue/xyz)`,
		},
		{
			name: "rest request short",
			event: Event{
				Kind:        KindRestRequest,
				PlayerName:  "Kael",
				Summary:     "requests a short rest",
				ResolvePath: "/dashboard/queue/r1",
			},
			want: `🛏️ **Rest** — Kael requests a short rest — [Resolve →](/dashboard/queue/r1)`,
		},
		{
			name: "skill check narration",
			event: Event{
				Kind:        KindSkillCheckNarration,
				PlayerName:  "Thorn",
				Summary:     "Athletics 18 (awaiting narration)",
				ResolvePath: "/dashboard/queue/c1",
			},
			want: `🎲 **Check** — Thorn: Athletics 18 (awaiting narration) — [Resolve →](/dashboard/queue/c1)`,
		},
		{
			name: "consumable",
			event: Event{
				Kind:        KindConsumable,
				PlayerName:  "Aria",
				Summary:     "uses Ball Bearings",
				ResolvePath: "/dashboard/queue/i1",
			},
			want: `🧪 **Item** — Aria uses Ball Bearings — [Resolve →](/dashboard/queue/i1)`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := FormatEvent(tc.event)
			if got != tc.want {
				t.Errorf("FormatEvent() =\n  %q\nwant\n  %q", got, tc.want)
			}
		})
	}
}

func TestFormatCancelled(t *testing.T) {
	original := `🎭 **Action** — Thorn: "flip the table" — [Resolve →](/dashboard/queue/abc)`
	got := FormatCancelled(original)
	want := `~~🎭 **Action** — Thorn: "flip the table"~~ Cancelled by player`
	if got != want {
		t.Errorf("FormatCancelled() =\n  %q\nwant\n  %q", got, want)
	}
}

func TestFormatResolved(t *testing.T) {
	original := `🎭 **Action** — Thorn: "flip the table" — [Resolve →](/dashboard/queue/abc)`
	got := FormatResolved(original, "table is flipped, enemies prone")
	want := `✅ 🎭 **Action** — Thorn: "flip the table" — table is flipped, enemies prone`
	if got != want {
		t.Errorf("FormatResolved() =\n  %q\nwant\n  %q", got, want)
	}
}

func TestFormatEvent_UnknownKind(t *testing.T) {
	got := FormatEvent(Event{Kind: EventKind("mystery"), PlayerName: "X", Summary: "s", ResolvePath: "/p"})
	want := `📨 **Notification** — X: s — [Resolve →](/p)`
	if got != want {
		t.Errorf("unknown kind: got %q want %q", got, want)
	}
}
