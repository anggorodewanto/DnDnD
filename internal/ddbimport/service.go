package ddbimport

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ab/dndnd/internal/refdata"
	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
)

// CharacterStore abstracts the database operations needed for import.
type CharacterStore interface {
	CreateCharacter(ctx context.Context, params refdata.CreateCharacterParams) (refdata.Character, error)
	GetCharacterByDdbURL(ctx context.Context, params refdata.GetCharacterByDdbURLParams) (refdata.Character, error)
	UpdateCharacterFull(ctx context.Context, params refdata.UpdateCharacterFullParams) (refdata.Character, error)
	UpsertPendingDDBImport(ctx context.Context, params refdata.UpsertPendingDDBImportParams) error
	GetPendingDDBImport(ctx context.Context, id uuid.UUID) (refdata.PendingDdbImport, error)
	DeletePendingDDBImport(ctx context.Context, id uuid.UUID) error
}

// ImportResult contains the result of an import operation.
type ImportResult struct {
	Character refdata.Character
	Parsed    *ParsedCharacter
	Warnings  []Warning
	Preview   string
	IsResync  bool
	Changes   []string // non-empty for re-sync

	// PendingImportID is non-nil when Import has staged a re-sync update that
	// requires DM approval. The DM-side approver passes this id to
	// Service.ApproveImport (or Service.DiscardImport on reject). For new
	// imports and no-change re-syncs the id is uuid.Nil because there is
	// nothing waiting for approval.
	PendingImportID uuid.UUID
}

// ErrPendingImportNotFound is returned by ApproveImport when the requested
// import id is unknown, has already been consumed, or has expired.
var ErrPendingImportNotFound = errors.New("pending import not found or expired")

// pendingImportTTL is how long a staged re-sync stays pending before
// ApproveImport refuses to apply it. 24h matches the portal-token TTL and
// gives a DM a full day to review on Discord.
const pendingImportTTL = 24 * time.Hour

// pendingImport is an internal record holding the UpdateCharacterFull params
// for a staged re-sync.
type pendingImport struct {
	params  refdata.UpdateCharacterFullParams
	created time.Time
}

// Service orchestrates the DDB import flow.
//
// Re-syncs of an existing DDB character are not applied to the DB inside
// Import: they are staged as pending rows keyed by a freshly-minted import id
// (returned in ImportResult.PendingImportID). The caller is expected to surface
// that id to the DM (#dm-queue) and only call ApproveImport once the DM has
// explicitly approved the change. Pending entries time out after pendingImportTTL.
type Service struct {
	client Client
	store  CharacterStore
	now    func() time.Time

	mu      sync.Mutex
	pending map[uuid.UUID]pendingImport
}

// NewService creates a new import service backed by the system clock.
func NewService(client Client, store CharacterStore) *Service {
	return NewServiceWithClock(client, store, time.Now)
}

// NewServiceWithClock creates a Service with an injectable clock. Tests use
// this to fast-forward past pendingImportTTL.
func NewServiceWithClock(client Client, store CharacterStore, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{
		client:  client,
		store:   store,
		now:     now,
		pending: make(map[uuid.UUID]pendingImport),
	}
}

// Import performs the full import flow:
//   - parse URL → fetch → parse JSON → validate → build params
//   - on a fresh import: create the character row immediately (no DM gate;
//     a brand-new row is harmless to "preview").
//   - on a re-sync of an existing DDB character: stage the
//     UpdateCharacterFullParams and return a PendingImportID. The
//     DB row is NOT mutated until ApproveImport is called.
func (s *Service) Import(ctx context.Context, campaignID uuid.UUID, ddbURL string) (*ImportResult, error) {
	charID, err := ParseCharacterURL(ddbURL)
	if err != nil {
		return nil, fmt.Errorf("invalid DDB URL: %w", err)
	}

	rawJSON, err := s.client.FetchCharacter(ctx, charID)
	if err != nil {
		return nil, fmt.Errorf("fetching character: %w", err)
	}

	parsed, err := ParseDDBJSON(rawJSON)
	if err != nil {
		return nil, fmt.Errorf("parsing DDB JSON: %w", err)
	}

	warnings, err := Validate(parsed)
	if err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	params, err := buildCreateParams(campaignID, ddbURL, parsed)
	if err != nil {
		return nil, fmt.Errorf("building character params: %w", err)
	}

	result := &ImportResult{
		Parsed:   parsed,
		Warnings: warnings,
	}

	existing, getErr := s.store.GetCharacterByDdbURL(ctx, refdata.GetCharacterByDdbURLParams{
		CampaignID: campaignID,
		DdbUrl:     sql.NullString{String: ddbURL, Valid: true},
	})
	if getErr != nil {
		// Fresh import — create the row now.
		char, createErr := s.store.CreateCharacter(ctx, params)
		if createErr != nil {
			return nil, fmt.Errorf("creating character: %w", createErr)
		}
		result.Character = char
		result.Preview = FormatPreviewWithWarnings(parsed, warnings)
		return result, nil
	}

	// Re-sync — diff in-memory; DO NOT mutate the DB.
	result.IsResync = true
	result.Character = existing
	oldParsed := characterToParseResult(&existing)
	result.Changes = GenerateDiff(oldParsed, parsed)
	result.Preview = FormatPreviewWithWarnings(parsed, warnings)

	if len(result.Changes) == 0 {
		// Nothing to apply — no pending entry needed.
		return result, nil
	}

	updateParams := buildUpdateParams(existing.ID, params)
	importID := uuid.New()
	created := s.now()
	paramsJSON, err := json.Marshal(updateParams)
	if err != nil {
		return nil, fmt.Errorf("marshaling pending import: %w", err)
	}
	if err := s.store.UpsertPendingDDBImport(ctx, refdata.UpsertPendingDDBImportParams{
		ID:          importID,
		CharacterID: existing.ID,
		ParamsJson:  paramsJSON,
		CreatedAt:   created,
	}); err != nil {
		return nil, fmt.Errorf("persisting pending import: %w", err)
	}

	s.mu.Lock()
	s.pending[importID] = pendingImport{
		params:  updateParams,
		created: created,
	}
	s.mu.Unlock()
	result.PendingImportID = importID

	return result, nil
}

// ApproveImport applies a previously-staged re-sync. It is the only path that
// calls UpdateCharacterFull for re-syncs. On success the pending entry is
// consumed (a second Approve with the same id returns ErrPendingImportNotFound).
func (s *Service) ApproveImport(ctx context.Context, importID uuid.UUID) (refdata.Character, error) {
	entry, ok, err := s.loadPendingImport(ctx, importID)
	if err != nil {
		return refdata.Character{}, err
	}
	if !ok {
		return refdata.Character{}, ErrPendingImportNotFound
	}

	s.mu.Lock()
	delete(s.pending, importID)
	s.mu.Unlock()
	if err := s.store.DeletePendingDDBImport(ctx, importID); err != nil {
		return refdata.Character{}, fmt.Errorf("deleting pending import: %w", err)
	}

	updated, err := s.store.UpdateCharacterFull(ctx, entry.params)
	if err != nil {
		return refdata.Character{}, fmt.Errorf("updating character: %w", err)
	}
	return updated, nil
}

// DiscardImport removes a staged re-sync without applying it. Intended for
// use when the DM rejects the diff. Unknown ids are silently ignored.
func (s *Service) DiscardImport(importID uuid.UUID) {
	s.mu.Lock()
	delete(s.pending, importID)
	s.mu.Unlock()
	_ = s.store.DeletePendingDDBImport(context.Background(), importID)
}

func (s *Service) loadPendingImport(ctx context.Context, importID uuid.UUID) (pendingImport, bool, error) {
	s.mu.Lock()
	entry, ok := s.pending[importID]
	if ok {
		if s.now().Sub(entry.created) > pendingImportTTL {
			delete(s.pending, importID)
			s.mu.Unlock()
			_ = s.store.DeletePendingDDBImport(ctx, importID)
			return pendingImport{}, false, nil
		}
		s.mu.Unlock()
		return entry, true, nil
	}
	s.mu.Unlock()

	row, err := s.store.GetPendingDDBImport(ctx, importID)
	if errors.Is(err, sql.ErrNoRows) {
		return pendingImport{}, false, nil
	}
	if err != nil {
		return pendingImport{}, false, fmt.Errorf("loading pending import: %w", err)
	}
	if s.now().Sub(row.CreatedAt) > pendingImportTTL {
		_ = s.store.DeletePendingDDBImport(ctx, importID)
		return pendingImport{}, false, nil
	}

	var params refdata.UpdateCharacterFullParams
	if err := json.Unmarshal(row.ParamsJson, &params); err != nil {
		return pendingImport{}, false, fmt.Errorf("unmarshaling pending import: %w", err)
	}
	return pendingImport{params: params, created: row.CreatedAt}, true, nil
}

func marshalField(name string, v interface{}) ([]byte, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshaling %s: %w", name, err)
	}
	return data, nil
}

func buildCreateParams(campaignID uuid.UUID, ddbURL string, pc *ParsedCharacter) (refdata.CreateCharacterParams, error) {
	scoresJSON, err := marshalField("ability scores", pc.AbilityScores)
	if err != nil {
		return refdata.CreateCharacterParams{}, err
	}

	classesJSON, err := marshalField("classes", pc.Classes)
	if err != nil {
		return refdata.CreateCharacterParams{}, err
	}

	inventoryJSON, err := marshalField("inventory", pc.Inventory)
	if err != nil {
		return refdata.CreateCharacterParams{}, err
	}

	profsJSON, err := marshalField("proficiencies", pc.Proficiencies)
	if err != nil {
		return refdata.CreateCharacterParams{}, err
	}

	featuresJSON, err := marshalField("features", pc.Features)
	if err != nil {
		return refdata.CreateCharacterParams{}, err
	}

	charDataJSON, err := marshalField("character data", map[string]interface{}{"spells": pc.Spells})
	if err != nil {
		return refdata.CreateCharacterParams{}, err
	}

	// Compute proficiency bonus from level
	profBonus := int32(2)
	if pc.Level >= 17 {
		profBonus = 6
	} else if pc.Level >= 13 {
		profBonus = 5
	} else if pc.Level >= 9 {
		profBonus = 4
	} else if pc.Level >= 5 {
		profBonus = 3
	}

	// Build hit dice remaining
	hitDice := make(map[string]int)
	for _, c := range pc.Classes {
		die := classHitDie(c.Class)
		if die != "" {
			hitDice[die] += c.Level
		}
	}
	hitDiceJSON, _ := json.Marshal(hitDice)

	langs := pc.Languages
	if len(langs) == 0 {
		langs = []string{"Common"}
	}

	return refdata.CreateCharacterParams{
		CampaignID:       campaignID,
		Name:             pc.Name,
		Race:             pc.Race,
		Classes:          classesJSON,
		Level:            int32(pc.Level),
		AbilityScores:    scoresJSON,
		HpMax:            int32(pc.HPMax),
		HpCurrent:        int32(pc.HPCurrent),
		TempHp:           int32(pc.TempHP),
		Ac:               int32(pc.AC),
		SpeedFt:          int32(pc.SpeedFt),
		ProficiencyBonus: profBonus,
		HitDiceRemaining: hitDiceJSON,
		Gold:             int32(pc.Gold),
		Languages:        langs,
		Inventory:        pqtype.NullRawMessage{RawMessage: inventoryJSON, Valid: true},
		Proficiencies:    pqtype.NullRawMessage{RawMessage: profsJSON, Valid: true},
		Features:         pqtype.NullRawMessage{RawMessage: featuresJSON, Valid: true},
		CharacterData:    pqtype.NullRawMessage{RawMessage: charDataJSON, Valid: true},
		DdbUrl:           sql.NullString{String: ddbURL, Valid: true},
	}, nil
}

// buildUpdateParams converts CreateCharacterParams to UpdateCharacterFullParams with the given ID.
func buildUpdateParams(id uuid.UUID, p refdata.CreateCharacterParams) refdata.UpdateCharacterFullParams {
	return refdata.UpdateCharacterFullParams{
		ID:               id,
		Name:             p.Name,
		Race:             p.Race,
		Classes:          p.Classes,
		Level:            p.Level,
		AbilityScores:    p.AbilityScores,
		HpMax:            p.HpMax,
		HpCurrent:        p.HpCurrent,
		TempHp:           p.TempHp,
		Ac:               p.Ac,
		AcFormula:        p.AcFormula,
		SpeedFt:          p.SpeedFt,
		ProficiencyBonus: p.ProficiencyBonus,
		EquippedMainHand: p.EquippedMainHand,
		EquippedOffHand:  p.EquippedOffHand,
		EquippedArmor:    p.EquippedArmor,
		SpellSlots:       p.SpellSlots,
		PactMagicSlots:   p.PactMagicSlots,
		HitDiceRemaining: p.HitDiceRemaining,
		FeatureUses:      p.FeatureUses,
		Features:         p.Features,
		Proficiencies:    p.Proficiencies,
		Gold:             p.Gold,
		AttunementSlots:  p.AttunementSlots,
		Languages:        p.Languages,
		Inventory:        p.Inventory,
		CharacterData:    p.CharacterData,
		DdbUrl:           p.DdbUrl,
		Homebrew:         p.Homebrew,
	}
}

func classHitDie(className string) string {
	switch className {
	case "Barbarian":
		return "d12"
	case "Fighter", "Paladin", "Ranger":
		return "d10"
	case "Bard", "Cleric", "Druid", "Monk", "Rogue", "Warlock":
		return "d8"
	case "Sorcerer", "Wizard":
		return "d6"
	}
	return "d8" // default
}

// characterToParseResult converts a refdata.Character back to ParsedCharacter for diffing.
func characterToParseResult(c *refdata.Character) *ParsedCharacter {
	pc := &ParsedCharacter{
		Name:      c.Name,
		Race:      c.Race,
		Level:     int(c.Level),
		HPMax:     int(c.HpMax),
		HPCurrent: int(c.HpCurrent),
		TempHP:    int(c.TempHp),
		AC:        int(c.Ac),
		SpeedFt:   int(c.SpeedFt),
		Gold:      int(c.Gold),
	}

	if len(c.AbilityScores) > 0 {
		_ = json.Unmarshal(c.AbilityScores, &pc.AbilityScores)
	}
	if len(c.Classes) > 0 {
		_ = json.Unmarshal(c.Classes, &pc.Classes)
	}
	if c.Inventory.Valid && len(c.Inventory.RawMessage) > 0 {
		_ = json.Unmarshal(c.Inventory.RawMessage, &pc.Inventory)
	}
	if c.Proficiencies.Valid && len(c.Proficiencies.RawMessage) > 0 {
		_ = json.Unmarshal(c.Proficiencies.RawMessage, &pc.Proficiencies)
	}

	pc.Languages = c.Languages

	if c.Features.Valid && len(c.Features.RawMessage) > 0 {
		_ = json.Unmarshal(c.Features.RawMessage, &pc.Features)
	}
	if c.CharacterData.Valid && len(c.CharacterData.RawMessage) > 0 {
		var charData map[string]json.RawMessage
		if err := json.Unmarshal(c.CharacterData.RawMessage, &charData); err == nil {
			if spellsRaw, ok := charData["spells"]; ok {
				_ = json.Unmarshal(spellsRaw, &pc.Spells)
			}
		}
	}

	return pc
}
