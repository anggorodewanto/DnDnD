package narration

import (
	"strings"
	"testing"
)

func TestRenderDiscord_PassthroughPlainText(t *testing.T) {
	got := RenderDiscord("Hello, adventurers!")
	if got.Body != "Hello, adventurers!" {
		t.Fatalf("body = %q, want %q", got.Body, "Hello, adventurers!")
	}
	if len(got.Embeds) != 0 {
		t.Fatalf("expected no embeds, got %d", len(got.Embeds))
	}
}

func TestRenderDiscord_PassthroughBoldItalicBlockquote(t *testing.T) {
	// Discord markdown is a subset: **bold**, *italic*, > quote. Source passes
	// through unchanged because Discord renders it natively.
	src := "**bold** and *italic*\n> quoted line"
	got := RenderDiscord(src)
	if got.Body != src {
		t.Fatalf("body = %q, want %q", got.Body, src)
	}
}

func TestRenderDiscord_ReadAloudBecomesEmbed(t *testing.T) {
	src := "Before block.\n:::read-aloud\nThe stone door grinds open.\n:::\nAfter block."
	got := RenderDiscord(src)

	// Body should contain the before/after text, with the read-aloud block
	// replaced by nothing or a separator (we'll allow empty line where block was).
	if !strings.Contains(got.Body, "Before block.") {
		t.Fatalf("body missing 'Before block.': %q", got.Body)
	}
	if !strings.Contains(got.Body, "After block.") {
		t.Fatalf("body missing 'After block.': %q", got.Body)
	}
	if strings.Contains(got.Body, ":::") {
		t.Fatalf("body still contains fence markers: %q", got.Body)
	}
	if strings.Contains(got.Body, "The stone door grinds open.") {
		t.Fatalf("body should not contain read-aloud text; it belongs in embed: %q", got.Body)
	}

	if len(got.Embeds) != 1 {
		t.Fatalf("expected 1 embed, got %d", len(got.Embeds))
	}
	if got.Embeds[0].Description != "The stone door grinds open." {
		t.Fatalf("embed description = %q", got.Embeds[0].Description)
	}
	if got.Embeds[0].Color == 0 {
		t.Fatalf("embed color should be non-zero (left border accent)")
	}
}

func TestRenderDiscord_MultipleReadAloudBlocks(t *testing.T) {
	src := ":::read-aloud\nFirst.\n:::\nmiddle\n:::read-aloud\nSecond.\n:::"
	got := RenderDiscord(src)
	if len(got.Embeds) != 2 {
		t.Fatalf("expected 2 embeds, got %d", len(got.Embeds))
	}
	if got.Embeds[0].Description != "First." {
		t.Fatalf("embed[0] = %q", got.Embeds[0].Description)
	}
	if got.Embeds[1].Description != "Second." {
		t.Fatalf("embed[1] = %q", got.Embeds[1].Description)
	}
	if !strings.Contains(got.Body, "middle") {
		t.Fatalf("body missing middle text: %q", got.Body)
	}
}

func TestRenderDiscord_CollapsesMultipleBlankLines(t *testing.T) {
	src := "A\n\n\n\n\nB"
	got := RenderDiscord(src)
	// Three+ blanks should collapse to 1.
	if got.Body != "A\n\nB" {
		t.Fatalf("body = %q", got.Body)
	}
}

func TestRenderDiscord_TrimsSurroundingBlankLines(t *testing.T) {
	src := "\n\nhello\n\n"
	got := RenderDiscord(src)
	if got.Body != "hello" {
		t.Fatalf("body = %q", got.Body)
	}
}

func TestRenderDiscord_UnclosedReadAloudIsLiteral(t *testing.T) {
	// Graceful: if the block is not closed, leave the source alone.
	src := ":::read-aloud\nNo closer here"
	got := RenderDiscord(src)
	if len(got.Embeds) != 0 {
		t.Fatalf("expected 0 embeds for unclosed fence, got %d", len(got.Embeds))
	}
	if !strings.Contains(got.Body, "No closer here") {
		t.Fatalf("body lost content: %q", got.Body)
	}
}

