package open5e

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/assert"

	"github.com/ab/dndnd/internal/refdata"
)

type fakeCampaignReader struct {
	campaign refdata.Campaign
	err      error
}

func (f *fakeCampaignReader) GetCampaignByID(ctx context.Context, id uuid.UUID) (refdata.Campaign, error) {
	if f.err != nil {
		return refdata.Campaign{}, f.err
	}
	return f.campaign, nil
}

func mustJSON(t *testing.T, v any) pqtype.NullRawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return pqtype.NullRawMessage{RawMessage: b, Valid: true}
}

func TestCampaignSourceLookup_ReturnsEnabledSources(t *testing.T) {
	settings := map[string]any{
		"open5e_sources": []string{"tome-of-beasts", "creature-codex"},
	}
	reader := &fakeCampaignReader{campaign: refdata.Campaign{Settings: mustJSON(t, settings)}}
	lookup := NewCampaignSourceLookup(reader)
	sources := lookup.EnabledOpen5eSources(uuid.New())
	assert.ElementsMatch(t, []string{"tome-of-beasts", "creature-codex"}, sources)
}

func TestCampaignSourceLookup_ZeroUUIDReturnsNil(t *testing.T) {
	lookup := NewCampaignSourceLookup(&fakeCampaignReader{})
	assert.Nil(t, lookup.EnabledOpen5eSources(uuid.Nil))
}

func TestCampaignSourceLookup_ReaderErrorReturnsNil(t *testing.T) {
	reader := &fakeCampaignReader{err: errors.New("db")}
	lookup := NewCampaignSourceLookup(reader)
	assert.Nil(t, lookup.EnabledOpen5eSources(uuid.New()))
}

func TestCampaignSourceLookup_NullSettingsReturnsNil(t *testing.T) {
	reader := &fakeCampaignReader{campaign: refdata.Campaign{Settings: pqtype.NullRawMessage{Valid: false}}}
	lookup := NewCampaignSourceLookup(reader)
	assert.Nil(t, lookup.EnabledOpen5eSources(uuid.New()))
}

func TestCampaignSourceLookup_MalformedSettingsReturnsNil(t *testing.T) {
	reader := &fakeCampaignReader{campaign: refdata.Campaign{Settings: pqtype.NullRawMessage{RawMessage: []byte("not json"), Valid: true}}}
	lookup := NewCampaignSourceLookup(reader)
	assert.Nil(t, lookup.EnabledOpen5eSources(uuid.New()))
}

func TestCampaignSourceLookup_SettingsWithoutOpen5eReturnsNil(t *testing.T) {
	settings := map[string]any{"turn_timeout_hours": 24}
	reader := &fakeCampaignReader{campaign: refdata.Campaign{Settings: mustJSON(t, settings)}}
	lookup := NewCampaignSourceLookup(reader)
	assert.Empty(t, lookup.EnabledOpen5eSources(uuid.New()))
}
