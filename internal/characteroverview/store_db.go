package characteroverview

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/refdata"
	"github.com/ab/dndnd/internal/rest"
)

// StatusApproved is the player_characters.status value used for the
// read-only character overview — only approved party members are exposed.
const StatusApproved = "approved"

// RefdataQueries is the minimal subset of *refdata.Queries that DBStore
// depends on. Declared as an interface so tests can substitute a fake.
type RefdataQueries interface {
	ListPlayerCharactersByStatus(ctx context.Context, arg refdata.ListPlayerCharactersByStatusParams) ([]refdata.ListPlayerCharactersByStatusRow, error)
	GetCharacter(ctx context.Context, id uuid.UUID) (refdata.Character, error)
	GetActiveCombatantByCharacterID(ctx context.Context, characterID uuid.NullUUID) (refdata.Combatant, error)
	UpdateCharacterVitals(ctx context.Context, arg refdata.UpdateCharacterVitalsParams) (refdata.Character, error)
	UpdateCharacterSpellSlots(ctx context.Context, arg refdata.UpdateCharacterSpellSlotsParams) (refdata.Character, error)
	UpdateCharacterPactMagicSlots(ctx context.Context, arg refdata.UpdateCharacterPactMagicSlotsParams) (refdata.Character, error)
	UpdateCharacterFeatureUses(ctx context.Context, arg refdata.UpdateCharacterFeatureUsesParams) (refdata.Character, error)
	DeleteCharacter(ctx context.Context, id uuid.UUID) error
}

// DBStore is a Store implementation backed by sqlc-generated refdata queries.
type DBStore struct {
	q RefdataQueries
}

// NewDBStore constructs a DBStore wrapping the given refdata queries.
func NewDBStore(q RefdataQueries) *DBStore {
	return &DBStore{q: q}
}

// ListApprovedPartyCharacters returns the approved player characters of a
// campaign, shaped into CharacterSheet domain structs for the dashboard.
func (s *DBStore) ListApprovedPartyCharacters(ctx context.Context, campaignID uuid.UUID) ([]CharacterSheet, error) {
	rows, err := s.q.ListPlayerCharactersByStatus(ctx, refdata.ListPlayerCharactersByStatusParams{
		CampaignID: campaignID,
		Status:     StatusApproved,
	})
	if err != nil {
		return nil, err
	}
	out := make([]CharacterSheet, 0, len(rows))
	for _, r := range rows {
		sheet := sheetFromRefdata(r)
		s.overlayLiveCombatHP(ctx, &sheet)
		out = append(out, sheet)
	}
	return out, nil
}

// overlayLiveCombatHP replaces the static character-row HP with the live combat
// snapshot when the character is in an active encounter. Combat carries HP in at
// start and never writes it back, so the character row is stale mid-fight; the
// active combatant is the source of truth. Best-effort and read-only: a missing
// combatant (out of combat) or any lookup error leaves the character-row HP
// untouched. HP/temp HP only — conditions/exhaustion overlay is out of scope.
func (s *DBStore) overlayLiveCombatHP(ctx context.Context, sheet *CharacterSheet) {
	if sheet.CharacterID == uuid.Nil {
		return
	}
	cb, err := s.q.GetActiveCombatantByCharacterID(ctx, uuid.NullUUID{UUID: sheet.CharacterID, Valid: true})
	if err != nil {
		return
	}
	sheet.HPMax = cb.HpMax
	sheet.HPCurrent = cb.HpCurrent
	sheet.TempHP = cb.TempHp
}

// GetCharacterStatusContext loads the campaign, character_data and active-combat
// flag for an out-of-combat status edit.
func (s *DBStore) GetCharacterStatusContext(ctx context.Context, characterID uuid.UUID) (CharacterStatusContext, error) {
	ch, err := s.q.GetCharacter(ctx, characterID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return CharacterStatusContext{}, ErrCharacterNotFound
		}
		return CharacterStatusContext{}, err
	}

	inCombat, err := s.characterInActiveCombat(ctx, characterID)
	if err != nil {
		return CharacterStatusContext{}, err
	}

	var charData []byte
	if ch.CharacterData.Valid {
		charData = ch.CharacterData.RawMessage
	}
	return CharacterStatusContext{
		CampaignID:     ch.CampaignID,
		CharacterData:  charData,
		InActiveCombat: inCombat,
	}, nil
}

// characterInActiveCombat reports whether the character currently has an active
// combatant row. A missing row (sql.ErrNoRows) means "not in combat".
func (s *DBStore) characterInActiveCombat(ctx context.Context, characterID uuid.UUID) (bool, error) {
	_, err := s.q.GetActiveCombatantByCharacterID(ctx, uuid.NullUUID{UUID: characterID, Valid: true})
	if err == nil {
		return true, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return false, err
}

// UpdateCharacterStatus persists a resolved out-of-combat status edit.
func (s *DBStore) UpdateCharacterStatus(ctx context.Context, p PersistStatusParams) error {
	_, err := s.q.UpdateCharacterVitals(ctx, refdata.UpdateCharacterVitalsParams{
		ID:            p.CharacterID,
		HpMax:         p.HPMax,
		HpCurrent:     p.HPCurrent,
		TempHp:        p.TempHP,
		Conditions:    p.Conditions,
		CharacterData: pqtype.NullRawMessage{RawMessage: p.CharacterData, Valid: len(p.CharacterData) > 0},
	})
	return err
}

// GetCharacterSlotsContext loads the campaign, current spell/pact slots and the
// active-combat flag for an out-of-combat slot edit.
func (s *DBStore) GetCharacterSlotsContext(ctx context.Context, characterID uuid.UUID) (SlotsContext, error) {
	ch, err := s.q.GetCharacter(ctx, characterID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return SlotsContext{}, ErrCharacterNotFound
		}
		return SlotsContext{}, err
	}

	inCombat, err := s.characterInActiveCombat(ctx, characterID)
	if err != nil {
		return SlotsContext{}, err
	}

	spell, err := parseSpellSlotsContext(ch.SpellSlots)
	if err != nil {
		return SlotsContext{}, err
	}

	return SlotsContext{
		CampaignID:     ch.CampaignID,
		InActiveCombat: inCombat,
		SpellSlots:     spell,
		PactMagicSlots: parsePactSlotsValue(ch.PactMagicSlots),
		FeatureUses:    featureUsesValue(ch.FeatureUses),
	}, nil
}

// featureUsesValue returns the raw feature_uses JSON, or nil when the column is
// NULL/empty. Passed straight through to the GetFeatureUses response.
func featureUsesValue(fu pqtype.NullRawMessage) json.RawMessage {
	if !fu.Valid || len(fu.RawMessage) == 0 {
		return nil
	}
	return fu.RawMessage
}

// UpdateCharacterSlots persists a resolved slot edit, touching only the stores
// the caller provided (nil RawMessage = leave that store untouched).
func (s *DBStore) UpdateCharacterSlots(ctx context.Context, p PersistSlotsParams) error {
	if p.SpellSlots != nil {
		if _, err := s.q.UpdateCharacterSpellSlots(ctx, refdata.UpdateCharacterSpellSlotsParams{
			ID:         p.CharacterID,
			SpellSlots: pqtype.NullRawMessage{RawMessage: p.SpellSlots, Valid: true},
		}); err != nil {
			return err
		}
	}
	if p.PactMagicSlots != nil {
		if _, err := s.q.UpdateCharacterPactMagicSlots(ctx, refdata.UpdateCharacterPactMagicSlotsParams{
			ID:             p.CharacterID,
			PactMagicSlots: pqtype.NullRawMessage{RawMessage: p.PactMagicSlots, Valid: true},
		}); err != nil {
			return err
		}
	}
	return nil
}

// UpdateCharacterFeatureUses persists a resolved feature_uses edit: the whole
// feature_uses map, already encoded. An empty payload writes SQL NULL.
func (s *DBStore) UpdateCharacterFeatureUses(ctx context.Context, p PersistFeatureUsesParams) error {
	_, err := s.q.UpdateCharacterFeatureUses(ctx, refdata.UpdateCharacterFeatureUsesParams{
		ID:          p.CharacterID,
		FeatureUses: pqtype.NullRawMessage{RawMessage: p.FeatureUses, Valid: len(p.FeatureUses) > 0},
	})
	return err
}

// DeleteCharacter permanently removes the character row.
func (s *DBStore) DeleteCharacter(ctx context.Context, id uuid.UUID) error {
	return s.q.DeleteCharacter(ctx, id)
}

// parseSpellSlotsContext parses stored spell slots into an int-keyed map. A
// null/empty column yields an empty (non-nil) map.
func parseSpellSlotsContext(nrm pqtype.NullRawMessage) (map[int]character.SlotInfo, error) {
	if !nrm.Valid {
		return map[int]character.SlotInfo{}, nil
	}
	return character.ParseSpellSlotsJSON(nrm.RawMessage)
}

// parsePactSlotsValue parses stored pact magic into a value. A null/empty/invalid
// column yields the zero value (no pact magic).
func parsePactSlotsValue(nrm pqtype.NullRawMessage) character.PactMagicSlots {
	if !nrm.Valid || len(nrm.RawMessage) == 0 {
		return character.PactMagicSlots{}
	}
	var p character.PactMagicSlots
	if err := json.Unmarshal(nrm.RawMessage, &p); err != nil {
		return character.PactMagicSlots{}
	}
	return p
}

// spellSlotsStringKeyed parses stored spell slots for the wire — kept string-keyed
// ({"1":{...}}). A null/empty/invalid column yields an empty (non-nil) map.
func spellSlotsStringKeyed(nrm pqtype.NullRawMessage) map[string]character.SlotInfo {
	out := map[string]character.SlotInfo{}
	if !nrm.Valid || len(nrm.RawMessage) == 0 {
		return out
	}
	var m map[string]character.SlotInfo
	if err := json.Unmarshal(nrm.RawMessage, &m); err != nil {
		return out
	}
	return m
}

// pactSlotsPtr parses stored pact magic into a pointer for the sheet view. A
// null/empty/invalid column yields nil (no pact magic).
func pactSlotsPtr(nrm pqtype.NullRawMessage) *character.PactMagicSlots {
	if !nrm.Valid || len(nrm.RawMessage) == 0 {
		return nil
	}
	var p character.PactMagicSlots
	if err := json.Unmarshal(nrm.RawMessage, &p); err != nil {
		return nil
	}
	return &p
}

func sheetFromRefdata(r refdata.ListPlayerCharactersByStatusRow) CharacterSheet {
	ddbURL := ""
	if r.DdbUrl.Valid {
		ddbURL = r.DdbUrl.String
	}
	languages := r.Languages
	if languages == nil {
		languages = []string{}
	}
	exhaustion := 0
	if r.CharacterData.Valid {
		exhaustion, _ = rest.ExhaustionLevelFromCharacterData(r.CharacterData.RawMessage)
	}
	conditions := conditionNamesFromJSON(r.Conditions)
	if conditions == nil {
		conditions = []string{}
	}
	return CharacterSheet{
		PlayerCharacterID: r.ID,
		CharacterID:       r.CharacterID,
		DiscordUserID:     r.DiscordUserID,
		Name:              r.CharacterName,
		Race:              r.Race,
		Level:             r.Level,
		Classes:           r.Classes,
		HPMax:             r.HpMax,
		HPCurrent:         r.HpCurrent,
		TempHP:            r.TempHp,
		AC:                r.Ac,
		SpeedFt:           r.SpeedFt,
		AbilityScores:     r.AbilityScores,
		Languages:         languages,
		DDBURL:            ddbURL,
		ExhaustionLevel:   int32(exhaustion),
		Conditions:        conditions,
		SpellSlots:        spellSlotsStringKeyed(r.SpellSlots),
		PactMagicSlots:    pactSlotsPtr(r.PactMagicSlots),
	}
}
