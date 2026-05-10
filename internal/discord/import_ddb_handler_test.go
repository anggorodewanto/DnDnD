package discord

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/ddbimport"
	"github.com/ab/dndnd/internal/refdata"
)

// --- DDB Import mock ---

type mockDDBImporter struct {
	ImportFunc func(ctx context.Context, campaignID uuid.UUID, ddbURL string) (*ddbimport.ImportResult, error)
}

func (m *mockDDBImporter) Import(ctx context.Context, campaignID uuid.UUID, ddbURL string) (*ddbimport.ImportResult, error) {
	return m.ImportFunc(ctx, campaignID, ddbURL)
}

func TestImportHandler_DDB_Success(t *testing.T) {
	mock := newTestMock()
	rc := captureResponse(mock)

	var dmQueueMessage string
	mock.ChannelMessageSendFunc = func(channelID, content string) (*discordgo.Message, error) {
		dmQueueMessage = content
		return &discordgo.Message{}, nil
	}

	charID := testCharacterID()
	importer := &mockDDBImporter{
		ImportFunc: func(_ context.Context, _ uuid.UUID, ddbURL string) (*ddbimport.ImportResult, error) {
			return &ddbimport.ImportResult{
				Character: refdata.Character{ID: charID, Name: "Thorin Ironforge"},
				Preview:   "**Thorin Ironforge** — Human\nLevel 5 — Fighter (Champion) 5\nHP: 39/44 | AC: 18",
			}, nil
		},
	}

	regService := newMockRegService()
	regService.ImportFunc = func(_ context.Context, _ uuid.UUID, _ string, _ uuid.UUID) (*refdata.PlayerCharacter, error) {
		return &refdata.PlayerCharacter{ID: testPCID(), Status: "pending", CreatedVia: "import"}, nil
	}

	handler := NewImportHandler(mock, regService, newMockCampaignProvider(), nil,
		staticDMQueueFunc("dm-queue-chan"), staticDMUserFunc("dm-user-1"),
		WithDDBImporter(importer))

	handler.Handle(makeInteraction("import", "player-1", "guild-1", stringOption("ddb-url", "https://www.dndbeyond.com/characters/12345")))

	if !strings.Contains(rc.Content, "Thorin Ironforge") {
		t.Errorf("expected character name in preview, got: %s", rc.Content)
	}
	if !strings.Contains(rc.Content, "DM approval") {
		t.Errorf("expected DM approval mention, got: %s", rc.Content)
	}
	if !strings.Contains(dmQueueMessage, "/import") {
		t.Errorf("expected import via in dm-queue, got: %s", dmQueueMessage)
	}
}

func TestImportHandler_DDB_ImportError(t *testing.T) {
	mock := newTestMock()
	rc := captureResponse(mock)

	importer := &mockDDBImporter{
		ImportFunc: func(_ context.Context, _ uuid.UUID, _ string) (*ddbimport.ImportResult, error) {
			return nil, fmt.Errorf("character not public")
		},
	}

	handler := NewImportHandler(mock, newMockRegService(), newMockCampaignProvider(), nil,
		staticDMQueueFunc(""), staticDMUserFunc(""),
		WithDDBImporter(importer))

	handler.Handle(makeInteraction("import", "player-1", "guild-1", stringOption("ddb-url", "https://www.dndbeyond.com/characters/12345")))

	if !strings.Contains(rc.Content, "Import error") {
		t.Errorf("expected import error, got: %s", rc.Content)
	}
}

func TestImportHandler_DDB_Resync(t *testing.T) {
	mock := newTestMock()
	rc := captureResponse(mock)
	mock.ChannelMessageSendFunc = func(_, _ string) (*discordgo.Message, error) {
		return &discordgo.Message{}, nil
	}

	importer := &mockDDBImporter{
		ImportFunc: func(_ context.Context, _ uuid.UUID, _ string) (*ddbimport.ImportResult, error) {
			return &ddbimport.ImportResult{
				Character: refdata.Character{ID: testCharacterID(), Name: "Thorin Ironforge"},
				Preview:   "**Thorin Ironforge** — Human\nLevel 6",
				IsResync:  true,
				Changes:   []string{"Level: 5 -> 6", "HP Max: 44 -> 55"},
			}, nil
		},
	}

	regService := newMockRegService()
	regService.ImportFunc = func(_ context.Context, _ uuid.UUID, _ string, _ uuid.UUID) (*refdata.PlayerCharacter, error) {
		return &refdata.PlayerCharacter{ID: testPCID(), Status: "pending"}, nil
	}

	handler := NewImportHandler(mock, regService, newMockCampaignProvider(), nil,
		staticDMQueueFunc("dm-chan"), staticDMUserFunc("dm-user"),
		WithDDBImporter(importer))

	handler.Handle(makeInteraction("import", "player-1", "guild-1", stringOption("ddb-url", "https://www.dndbeyond.com/characters/12345")))

	if !strings.Contains(rc.Content, "Re-import") {
		t.Errorf("expected re-import label, got: %s", rc.Content)
	}
	if !strings.Contains(rc.Content, "changes detected") {
		t.Errorf("expected changes mention, got: %s", rc.Content)
	}
	// Phase 90 fix: message must explicitly say nothing has been applied yet.
	if !strings.Contains(rc.Content, "no changes applied yet") {
		t.Errorf("expected 'no changes applied yet' (DB not mutated until DM approves), got: %s", rc.Content)
	}
}

func TestImportHandler_DDB_ResyncNoChanges(t *testing.T) {
	mock := newTestMock()
	rc := captureResponse(mock)
	mock.ChannelMessageSendFunc = func(_, _ string) (*discordgo.Message, error) {
		return &discordgo.Message{}, nil
	}

	importer := &mockDDBImporter{
		ImportFunc: func(_ context.Context, _ uuid.UUID, _ string) (*ddbimport.ImportResult, error) {
			return &ddbimport.ImportResult{
				Character: refdata.Character{ID: testCharacterID(), Name: "Thorin"},
				Preview:   "**Thorin** — Human",
				IsResync:  true,
				Changes:   nil,
			}, nil
		},
	}

	regService := newMockRegService()
	regService.ImportFunc = func(_ context.Context, _ uuid.UUID, _ string, _ uuid.UUID) (*refdata.PlayerCharacter, error) {
		return &refdata.PlayerCharacter{ID: testPCID(), Status: "pending"}, nil
	}

	handler := NewImportHandler(mock, regService, newMockCampaignProvider(), nil,
		staticDMQueueFunc("dm-chan"), staticDMUserFunc("dm-user"),
		WithDDBImporter(importer))

	handler.Handle(makeInteraction("import", "player-1", "guild-1", stringOption("ddb-url", "https://www.dndbeyond.com/characters/12345")))

	if !strings.Contains(rc.Content, "no changes") {
		t.Errorf("expected 'no changes' message, got: %s", rc.Content)
	}
}

func TestImportHandler_DDB_RegServiceError(t *testing.T) {
	mock := newTestMock()
	rc := captureResponse(mock)

	importer := &mockDDBImporter{
		ImportFunc: func(_ context.Context, _ uuid.UUID, _ string) (*ddbimport.ImportResult, error) {
			return &ddbimport.ImportResult{
				Character: refdata.Character{ID: testCharacterID(), Name: "Test"},
				Preview:   "preview",
			}, nil
		},
	}

	regService := newMockRegService()
	regService.ImportFunc = func(_ context.Context, _ uuid.UUID, _ string, _ uuid.UUID) (*refdata.PlayerCharacter, error) {
		return nil, fmt.Errorf("duplicate registration")
	}

	handler := NewImportHandler(mock, regService, newMockCampaignProvider(), nil,
		staticDMQueueFunc(""), staticDMUserFunc(""),
		WithDDBImporter(importer))

	handler.Handle(makeInteraction("import", "player-1", "guild-1", stringOption("ddb-url", "https://www.dndbeyond.com/characters/12345")))

	if !strings.Contains(rc.Content, "Import error") {
		t.Errorf("expected import error, got: %s", rc.Content)
	}
}
