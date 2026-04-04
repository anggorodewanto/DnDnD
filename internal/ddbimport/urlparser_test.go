package ddbimport

import (
	"testing"
)

func TestParseCharacterURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantID  string
		wantErr bool
	}{
		{
			name:   "standard URL with www",
			url:    "https://www.dndbeyond.com/characters/12345678",
			wantID: "12345678",
		},
		{
			name:   "URL without www",
			url:    "https://dndbeyond.com/characters/12345678",
			wantID: "12345678",
		},
		{
			name:   "URL with character name slug",
			url:    "https://www.dndbeyond.com/characters/12345678/some-name",
			wantID: "12345678",
		},
		{
			name:    "empty URL",
			url:     "",
			wantErr: true,
		},
		{
			name:    "invalid URL - no characters path",
			url:     "https://www.dndbeyond.com/monsters/12345678",
			wantErr: true,
		},
		{
			name:    "invalid URL - non-numeric ID",
			url:     "https://www.dndbeyond.com/characters/abc",
			wantErr: true,
		},
		{
			name:    "invalid URL - wrong domain",
			url:     "https://example.com/characters/12345678",
			wantErr: true,
		},
		{
			name:   "URL with trailing slash",
			url:    "https://www.dndbeyond.com/characters/12345678/",
			wantID: "12345678",
		},
		{
			name:   "URL with query params",
			url:    "https://www.dndbeyond.com/characters/12345678?sharing=true",
			wantID: "12345678",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotID, err := ParseCharacterURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseCharacterURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotID != tt.wantID {
				t.Errorf("ParseCharacterURL() = %q, want %q", gotID, tt.wantID)
			}
		})
	}
}
