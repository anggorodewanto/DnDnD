package character

import "testing"

func TestProfileFromCharacterData(t *testing.T) {
	tests := []struct {
		name          string
		raw           string
		wantAppearance string
		wantBackstory  string
	}{
		{"both fields", `{"appearance":"tall, scarred","backstory":"orphan of the moor"}`, "tall, scarred", "orphan of the moor"},
		{"ignores other keys", `{"spells":["fireball"],"background":"sage","appearance":"short and stout"}`, "short and stout", ""},
		{"empty blob", ``, "", ""},
		{"null literal", `null`, "", ""},
		{"malformed json", `{not json`, "", ""},
		{"absent keys", `{"spells":[]}`, "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ProfileFromCharacterData([]byte(tt.raw))
			if got.Appearance != tt.wantAppearance {
				t.Errorf("Appearance = %q, want %q", got.Appearance, tt.wantAppearance)
			}
			if got.Backstory != tt.wantBackstory {
				t.Errorf("Backstory = %q, want %q", got.Backstory, tt.wantBackstory)
			}
		})
	}
}

func TestCharacterProfile_IsEmpty(t *testing.T) {
	if !(CharacterProfile{}).IsEmpty() {
		t.Error("zero profile should be empty")
	}
	if !(CharacterProfile{Appearance: "  \t ", Backstory: " "}).IsEmpty() {
		t.Error("whitespace-only profile should be empty")
	}
	if (CharacterProfile{Appearance: "x"}).IsEmpty() {
		t.Error("profile with appearance should not be empty")
	}
	if (CharacterProfile{Backstory: "y"}).IsEmpty() {
		t.Error("profile with backstory should not be empty")
	}
}
