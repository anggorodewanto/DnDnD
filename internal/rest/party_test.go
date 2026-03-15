package rest

import (
	"strings"
	"testing"
)

// --- TDD Cycle 1: FormatPartyRestSummary for short rest ---

func TestFormatPartyRestSummary_ShortRest(t *testing.T) {
	msg := FormatPartyRestSummary("short", []string{"Kael", "Aria", "Thorn"}, []string{"Zara"})
	if !strings.Contains(msg, "Party Short Rest") {
		t.Errorf("expected 'Party Short Rest' in message, got: %s", msg)
	}
	if !strings.Contains(msg, "Kael, Aria, Thorn rested") {
		t.Errorf("expected rested names in message, got: %s", msg)
	}
	if !strings.Contains(msg, "Zara kept watch") {
		t.Errorf("expected excluded names in message, got: %s", msg)
	}
}

// --- TDD Cycle 2: FormatPartyRestSummary for long rest ---

func TestFormatPartyRestSummary_LongRest(t *testing.T) {
	msg := FormatPartyRestSummary("long", []string{"Kael", "Aria"}, nil)
	if !strings.Contains(msg, "Party Long Rest") {
		t.Errorf("expected 'Party Long Rest' in message, got: %s", msg)
	}
	if !strings.Contains(msg, "Kael, Aria rested") {
		t.Errorf("expected rested names in message, got: %s", msg)
	}
	// No excluded names — should not contain "kept watch"
	if strings.Contains(msg, "kept watch") {
		t.Errorf("did not expect 'kept watch' when no excluded, got: %s", msg)
	}
}

// --- TDD Cycle 3: FormatInterruptNotification short rest interrupted ---

func TestFormatInterruptNotification_ShortRest_NoBenefits(t *testing.T) {
	msg := FormatInterruptNotification("Kael", "short", "Ambush!", false)
	if !strings.Contains(msg, "short rest was interrupted") {
		t.Errorf("expected 'short rest was interrupted' in message, got: %s", msg)
	}
	if !strings.Contains(msg, "Ambush!") {
		t.Errorf("expected reason in message, got: %s", msg)
	}
	if !strings.Contains(msg, "No benefits granted") {
		t.Errorf("expected 'No benefits granted' in message, got: %s", msg)
	}
}

// --- TDD Cycle 4: FormatInterruptNotification long rest with benefits ---

func TestFormatInterruptNotification_LongRest_WithBenefits(t *testing.T) {
	msg := FormatInterruptNotification("Aria", "long", "Wolves attack", true)
	if !strings.Contains(msg, "long rest was interrupted") {
		t.Errorf("expected 'long rest was interrupted' in message, got: %s", msg)
	}
	if !strings.Contains(msg, "Wolves attack") {
		t.Errorf("expected reason in message, got: %s", msg)
	}
	if !strings.Contains(msg, "short rest benefits") {
		t.Errorf("expected 'short rest benefits' in message, got: %s", msg)
	}
}

// --- TDD Cycle 5: InterruptRest short rest ---

func TestInterruptRest_ShortRest(t *testing.T) {
	result := InterruptRest("short", false)
	if result.Benefits != "none" {
		t.Errorf("Benefits = %q, want %q", result.Benefits, "none")
	}
}

// --- TDD Cycle 6: InterruptRest long rest, 1 hour elapsed ---

func TestInterruptRest_LongRest_OneHourElapsed(t *testing.T) {
	result := InterruptRest("long", true)
	if result.Benefits != "short" {
		t.Errorf("Benefits = %q, want %q", result.Benefits, "short")
	}
}

// --- TDD Cycle 7: InterruptRest long rest, less than 1 hour ---

func TestInterruptRest_LongRest_LessThanOneHour(t *testing.T) {
	result := InterruptRest("long", false)
	if result.Benefits != "none" {
		t.Errorf("Benefits = %q, want %q", result.Benefits, "none")
	}
}

// --- Edge cases ---

func TestFormatPartyRestSummary_SingleCharacter(t *testing.T) {
	msg := FormatPartyRestSummary("short", []string{"Kael"}, nil)
	if !strings.Contains(msg, "Kael rested") {
		t.Errorf("expected 'Kael rested' in message, got: %s", msg)
	}
}

func TestFormatPartyRestSummary_AllExcluded(t *testing.T) {
	msg := FormatPartyRestSummary("long", []string{}, []string{"Zara"})
	// Edge: no one rested
	if !strings.Contains(msg, "rested") {
		t.Errorf("expected 'rested' in message, got: %s", msg)
	}
}

func TestFormatInterruptNotification_LongRest_NoBenefits(t *testing.T) {
	msg := FormatInterruptNotification("Kael", "long", "Dragon breath", false)
	if !strings.Contains(msg, "No benefits granted") {
		t.Errorf("expected 'No benefits granted' in message, got: %s", msg)
	}
}

func TestInterruptRest_ShortRest_OneHourElapsedIgnored(t *testing.T) {
	// Short rest ignores oneHourElapsed — always no benefits
	result := InterruptRest("short", true)
	if result.Benefits != "none" {
		t.Errorf("Benefits = %q, want %q", result.Benefits, "none")
	}
}
