package narration

import (
	"reflect"
	"sort"
	"testing"
)

func TestExtractPlaceholders_FindsTokens(t *testing.T) {
	body := "Hello {player_name}, welcome to {location}. {player_name}!"
	got := ExtractPlaceholders(body)
	sort.Strings(got)
	want := []string{"location", "player_name"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestExtractPlaceholders_IgnoresInvalidTokens(t *testing.T) {
	body := "Curly { spaced } and {1bad} but {good_one} works"
	got := ExtractPlaceholders(body)
	want := []string{"good_one"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestExtractPlaceholders_EmptyBody(t *testing.T) {
	if got := ExtractPlaceholders(""); len(got) != 0 {
		t.Fatalf("expected empty, got %v", got)
	}
}

func TestSubstitutePlaceholders_ReplacesKnownTokens(t *testing.T) {
	body := "Hello {player_name}, you are in {location}."
	out := SubstitutePlaceholders(body, map[string]string{
		"player_name": "Aragorn",
		"location":    "Bree",
	})
	want := "Hello Aragorn, you are in Bree."
	if out != want {
		t.Fatalf("got %q, want %q", out, want)
	}
}

func TestSubstitutePlaceholders_LeavesUnknownTokensUntouched(t *testing.T) {
	body := "Hello {player_name}, value {unknown}."
	out := SubstitutePlaceholders(body, map[string]string{"player_name": "Aragorn"})
	want := "Hello Aragorn, value {unknown}."
	if out != want {
		t.Fatalf("got %q, want %q", out, want)
	}
}

func TestSubstitutePlaceholders_NilMap(t *testing.T) {
	body := "Hello {a}."
	out := SubstitutePlaceholders(body, nil)
	if out != body {
		t.Fatalf("got %q, want %q", out, body)
	}
}
