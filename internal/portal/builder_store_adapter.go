package portal

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/refdata"
	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
)

// CharacterCreator is the subset of refdata.Queries for character creation.
type CharacterCreator interface {
	CreateCharacter(ctx context.Context, arg refdata.CreateCharacterParams) (refdata.Character, error)
	CreatePlayerCharacter(ctx context.Context, arg refdata.CreatePlayerCharacterParams) (refdata.PlayerCharacter, error)
	GetPlayerCharacterByDiscordUser(ctx context.Context, arg refdata.GetPlayerCharacterByDiscordUserParams) (refdata.PlayerCharacter, error)
	RelinkPlayerCharacter(ctx context.Context, arg refdata.RelinkPlayerCharacterParams) (refdata.PlayerCharacter, error)
	UpsertCharacterDraft(ctx context.Context, arg refdata.UpsertCharacterDraftParams) error
	GetCharacterDraft(ctx context.Context, arg refdata.GetCharacterDraftParams) (json.RawMessage, error)
	// Edit-mode dependencies.
	GetCharacter(ctx context.Context, id uuid.UUID) (refdata.Character, error)
	GetPlayerCharacterByCharacter(ctx context.Context, arg refdata.GetPlayerCharacterByCharacterParams) (refdata.PlayerCharacter, error)
	UpdateCharacter(ctx context.Context, arg refdata.UpdateCharacterParams) (refdata.Character, error)
	UpdatePlayerCharacterStatus(ctx context.Context, arg refdata.UpdatePlayerCharacterStatusParams) (refdata.PlayerCharacter, error)
	GetActiveEncounterIDByCharacterID(ctx context.Context, characterID uuid.NullUUID) (uuid.UUID, error)
}

type campaignGetter interface {
	GetCampaignByID(ctx context.Context, id uuid.UUID) (refdata.Campaign, error)
}

// BuilderStoreAdapter adapts refdata.Queries + TokenService to BuilderStore.
type BuilderStoreAdapter struct {
	q        CharacterCreator
	tokenSvc *TokenService
}

// resolveClassEntries returns the multiclass entries to persist. When the
// caller supplied a non-empty Classes slice it is filtered to drop empty
// rows; otherwise a single ClassEntry is constructed from Class/Subclass
// at level 1 (the legacy single-class path).
func resolveClassEntries(p CreateCharacterParams) []character.ClassEntry {
	if len(p.Classes) > 0 {
		out := make([]character.ClassEntry, 0, len(p.Classes))
		for _, c := range p.Classes {
			if c.Class == "" {
				continue
			}
			lvl := max(c.Level, 1)
			out = append(out, character.ClassEntry{Class: c.Class, Subclass: c.Subclass, Level: lvl, IsPrimary: c.IsPrimary})
		}
		if len(out) > 0 {
			ensurePrimary(out)
			return out
		}
	}
	return []character.ClassEntry{{Class: p.Class, Subclass: p.Subclass, Level: 1, IsPrimary: true}}
}

// ensurePrimary sets IsPrimary on the first entry if no entry has it set.
func ensurePrimary(classes []character.ClassEntry) {
	for _, c := range classes {
		if c.IsPrimary {
			return
		}
	}
	classes[0].IsPrimary = true
}

// NewBuilderStoreAdapter creates a new BuilderStoreAdapter.
func NewBuilderStoreAdapter(q CharacterCreator, tokenSvc *TokenService) *BuilderStoreAdapter {
	return &BuilderStoreAdapter{q: q, tokenSvc: tokenSvc}
}

// characterColumns holds the derived, marshalled column values shared by the
// create and update persistence paths. Building them once keeps the INSERT and
// UPDATE in lock-step so an edit derives stats identically to a fresh build.
type characterColumns struct {
	campID           uuid.UUID
	classesJSON      []byte
	level            int32
	scoresJSON       []byte
	hpMax            int32
	ac               int32
	acFormula        sql.NullString
	speedFt          int32
	profBonus        int32
	equippedMainHand sql.NullString
	equippedOffHand  sql.NullString
	equippedArmor    sql.NullString
	spellSlotsMsg    pqtype.NullRawMessage
	pactMagicMsg     pqtype.NullRawMessage
	hitDiceJSON      []byte
	featureUsesMsg   pqtype.NullRawMessage
	featuresMsg      pqtype.NullRawMessage
	profJSON         []byte
	languages        []string
	inventoryMsg     pqtype.NullRawMessage
	charDataMsg      pqtype.NullRawMessage
}

// buildCharacterColumns derives every persistable column from a submission's
// CreateCharacterParams. Shared by CreateCharacterRecord (INSERT) and
// UpdateCharacterRecord (UPDATE).
func buildCharacterColumns(p CreateCharacterParams) (characterColumns, error) {
	scoresJSON, _ := json.Marshal(p.AbilityScores)
	classEntries := resolveClassEntries(p)
	classesJSON, _ := json.Marshal(classEntries)
	hitDice := make(map[string]int, len(classEntries))
	totalLevel := 0
	for _, ce := range classEntries {
		hitDice[ce.Class] = ce.Level
		totalLevel += ce.Level
	}
	if totalLevel < 1 {
		totalLevel = 1
	}
	hitDiceJSON, _ := json.Marshal(hitDice)
	profJSON, _ := json.Marshal(character.Proficiencies{
		Skills:    p.Skills,
		Saves:     p.Saves,
		Expertise: p.Expertise,
	})

	var inventoryMsg pqtype.NullRawMessage
	allEquipment := p.Equipment
	if bgItems := BackgroundEquipmentPack(p.Background); len(bgItems) > 0 {
		allEquipment = append(allEquipment, bgItems...)
	}
	if items := EquipmentToInventoryWithEquipped(allEquipment, p.EquippedWeapon, p.WornArmor); len(items) > 0 {
		invJSON, _ := json.Marshal(items)
		inventoryMsg = pqtype.NullRawMessage{RawMessage: invJSON, Valid: true}
	}

	var equippedMainHand sql.NullString
	if p.EquippedWeapon != "" {
		equippedMainHand = sql.NullString{String: p.EquippedWeapon, Valid: true}
	}
	var equippedArmor sql.NullString
	if p.WornArmor != "" {
		equippedArmor = sql.NullString{String: p.WornArmor, Valid: true}
	}
	// ISSUE-011: an equipped shield (flagged off_hand by the inventory builder)
	// must also populate the dedicated equipped_off_hand column.
	var equippedOffHand sql.NullString
	if hasEquipmentItem(p.Equipment, "shield") {
		equippedOffHand = sql.NullString{String: "shield", Valid: true}
	}

	var featureUsesMsg pqtype.NullRawMessage
	if fu := InitFeatureUses(classEntries, p.AbilityScores); len(fu) > 0 {
		fuJSON, _ := json.Marshal(fu)
		featureUsesMsg = pqtype.NullRawMessage{RawMessage: fuJSON, Valid: true}
	}

	var pactMagicMsg pqtype.NullRawMessage
	if pact := pactMagicSlotsForClasses(classEntries); pact != nil {
		pactJSON, _ := json.Marshal(pact)
		pactMagicMsg = pqtype.NullRawMessage{RawMessage: pactJSON, Valid: true}
	}

	// Standard spell slots (ISSUE-002), canonical string-keyed {current,max}.
	var spellSlotsMsg pqtype.NullRawMessage
	if slots := spellSlotsForClasses(classEntries); slots != nil {
		slotsJSON, _ := json.Marshal(slots)
		spellSlotsMsg = pqtype.NullRawMessage{RawMessage: slotsJSON, Valid: true}
	}

	var featuresMsg pqtype.NullRawMessage
	if len(p.Features) > 0 {
		featJSON, _ := json.Marshal(p.Features)
		featuresMsg = pqtype.NullRawMessage{RawMessage: featJSON, Valid: true}
	}

	// character_data carries spells, weapon masteries, subrace, background, and
	// optional display-only appearance/backstory (no dedicated columns).
	charData := map[string]any{}
	if len(p.Spells) > 0 {
		charData["spells"] = p.Spells
	}
	if len(p.WeaponMasteries) > 0 {
		charData["weapon_masteries"] = p.WeaponMasteries
	}
	if p.Subrace != "" {
		charData["subrace"] = p.Subrace
	}
	if p.Background != "" {
		charData["background"] = p.Background
	}
	if appearance := strings.TrimSpace(p.Appearance); appearance != "" {
		charData["appearance"] = appearance
	}
	if backstory := strings.TrimSpace(p.Backstory); backstory != "" {
		charData["backstory"] = backstory
	}
	var charDataMsg pqtype.NullRawMessage
	if len(charData) > 0 {
		charDataJSON, _ := json.Marshal(charData)
		charDataMsg = pqtype.NullRawMessage{RawMessage: charDataJSON, Valid: true}
	}

	campID, err := uuid.Parse(p.CampaignID)
	if err != nil {
		return characterColumns{}, fmt.Errorf("invalid campaign_id %q: %w", p.CampaignID, err)
	}

	// ISSUE-004: persist the Unarmored Defense ac_formula. NULL for armored /
	// non-UD classes so they are unchanged.
	var acFormula sql.NullString
	if f := unarmoredDefenseFormula(classEntries, p.WornArmor, hasEquipmentItem(p.Equipment, "shield")); f != "" {
		acFormula = sql.NullString{String: f, Valid: true}
	}

	// characters.languages is TEXT[] NOT NULL — a nil slice writes SQL NULL and
	// violates the constraint, so default to a non-nil empty array.
	languages := p.Languages
	if languages == nil {
		languages = []string{}
	}

	return characterColumns{
		campID:           campID,
		classesJSON:      classesJSON,
		level:            int32(totalLevel),
		scoresJSON:       scoresJSON,
		hpMax:            int32(p.HPMax),
		ac:               int32(p.AC),
		acFormula:        acFormula,
		speedFt:          int32(p.SpeedFt),
		profBonus:        int32(p.ProfBonus),
		equippedMainHand: equippedMainHand,
		equippedOffHand:  equippedOffHand,
		equippedArmor:    equippedArmor,
		spellSlotsMsg:    spellSlotsMsg,
		pactMagicMsg:     pactMagicMsg,
		hitDiceJSON:      hitDiceJSON,
		featureUsesMsg:   featureUsesMsg,
		featuresMsg:      featuresMsg,
		profJSON:         profJSON,
		languages:        languages,
		inventoryMsg:     inventoryMsg,
		charDataMsg:      charDataMsg,
	}, nil
}

// CreateCharacterRecord creates a character in the database.
func (a *BuilderStoreAdapter) CreateCharacterRecord(ctx context.Context, p CreateCharacterParams) (string, error) {
	c, err := buildCharacterColumns(p)
	if err != nil {
		return "", err
	}
	ch, err := a.q.CreateCharacter(ctx, refdata.CreateCharacterParams{
		CampaignID:       c.campID,
		Name:             p.Name,
		Race:             p.Race,
		Classes:          c.classesJSON,
		Level:            c.level,
		AbilityScores:    c.scoresJSON,
		HpMax:            c.hpMax,
		HpCurrent:        c.hpMax,
		TempHp:           0,
		Ac:               c.ac,
		AcFormula:        c.acFormula,
		SpeedFt:          c.speedFt,
		ProficiencyBonus: c.profBonus,
		EquippedMainHand: c.equippedMainHand,
		EquippedOffHand:  c.equippedOffHand,
		EquippedArmor:    c.equippedArmor,
		HitDiceRemaining: c.hitDiceJSON,
		FeatureUses:      c.featureUsesMsg,
		SpellSlots:       c.spellSlotsMsg,
		PactMagicSlots:   c.pactMagicMsg,
		Features:         c.featuresMsg,
		Proficiencies:    pqtype.NullRawMessage{RawMessage: c.profJSON, Valid: true},
		Languages:        c.languages,
		Inventory:        c.inventoryMsg,
		CharacterData:    c.charDataMsg,
		Gold:             0,
		Homebrew:         sql.NullBool{Bool: false, Valid: true},
	})
	if err != nil {
		return "", err
	}
	return ch.ID.String(), nil
}

// GetEditContext returns ownership/DM/status facts for an existing character,
// or ErrCharacterNotFound when it does not exist. OwnerID/PlayerCharacterID are
// empty when no player_characters row links the character (unclaimed DM build).
func (a *BuilderStoreAdapter) GetEditContext(ctx context.Context, characterID string) (*EditContext, error) {
	charID, err := uuid.Parse(characterID)
	if err != nil {
		return nil, fmt.Errorf("invalid character_id %q: %w", characterID, err)
	}
	ch, err := a.q.GetCharacter(ctx, charID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrCharacterNotFound
		}
		return nil, fmt.Errorf("getting character: %w", err)
	}
	ec := &EditContext{CampaignID: ch.CampaignID.String()}

	pc, err := a.q.GetPlayerCharacterByCharacter(ctx, refdata.GetPlayerCharacterByCharacterParams{
		CampaignID:  ch.CampaignID,
		CharacterID: ch.ID,
	})
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("getting player character: %w", err)
	}
	if err == nil {
		ec.OwnerID = pc.DiscordUserID
		ec.PlayerCharacterID = pc.ID.String()
		ec.Status = pc.Status
	}

	if getter, ok := a.q.(campaignGetter); ok {
		if camp, cerr := getter.GetCampaignByID(ctx, ch.CampaignID); cerr == nil {
			ec.DMUserID = camp.DmUserID
		}
	}
	return ec, nil
}

// UpdateCharacterRecord rewrites a character from a fresh build (derived
// identically to creation) while preserving live-play state: damage taken
// (hp_current capped to the new max), temp HP, gold, attunement, ddb_url,
// homebrew, and any expended spell slots.
func (a *BuilderStoreAdapter) UpdateCharacterRecord(ctx context.Context, characterID string, p CreateCharacterParams) error {
	charID, err := uuid.Parse(characterID)
	if err != nil {
		return fmt.Errorf("invalid character_id %q: %w", characterID, err)
	}
	existing, err := a.q.GetCharacter(ctx, charID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrCharacterNotFound
		}
		return fmt.Errorf("getting character: %w", err)
	}
	c, err := buildCharacterColumns(p)
	if err != nil {
		return err
	}

	hpCurrent := min(existing.HpCurrent, c.hpMax)

	_, err = a.q.UpdateCharacter(ctx, refdata.UpdateCharacterParams{
		ID:               charID,
		Name:             p.Name,
		Race:             p.Race,
		Classes:          c.classesJSON,
		Level:            c.level,
		AbilityScores:    c.scoresJSON,
		HpMax:            c.hpMax,
		HpCurrent:        hpCurrent,
		TempHp:           existing.TempHp,
		Ac:               c.ac,
		AcFormula:        c.acFormula,
		SpeedFt:          c.speedFt,
		ProficiencyBonus: c.profBonus,
		EquippedMainHand: c.equippedMainHand,
		EquippedOffHand:  c.equippedOffHand,
		EquippedArmor:    c.equippedArmor,
		SpellSlots:       preserveExpendedSlots(existing.SpellSlots, c.spellSlotsMsg),
		PactMagicSlots:   c.pactMagicMsg,
		HitDiceRemaining: c.hitDiceJSON,
		FeatureUses:      c.featureUsesMsg,
		Features:         c.featuresMsg,
		Proficiencies:    pqtype.NullRawMessage{RawMessage: c.profJSON, Valid: true},
		Gold:             existing.Gold,
		AttunementSlots:  existing.AttunementSlots,
		Languages:        c.languages,
		Inventory:        c.inventoryMsg,
		CharacterData:    c.charDataMsg,
		DdbUrl:           existing.DdbUrl,
		Homebrew:         existing.Homebrew,
	})
	return err
}

// preserveExpendedSlots carries forward spell slots already spent before an
// edit. The fresh build supplies new {current=max} per level; for any level
// that existed before, the number expended (oldMax-oldCurrent) is re-applied to
// the new max so a mid-day edit does not silently refill the caster. Falls back
// to the freshly-derived slots when either side is absent or unparseable.
func preserveExpendedSlots(existing, fresh pqtype.NullRawMessage) pqtype.NullRawMessage {
	if !fresh.Valid || !existing.Valid {
		return fresh
	}
	var oldSlots, newSlots map[string]character.SlotInfo
	if err := json.Unmarshal(existing.RawMessage, &oldSlots); err != nil {
		return fresh
	}
	if err := json.Unmarshal(fresh.RawMessage, &newSlots); err != nil {
		return fresh
	}
	for level, ns := range newSlots {
		os, ok := oldSlots[level]
		if !ok {
			continue
		}
		expended := max(os.Max-os.Current, 0)
		ns.Current = max(ns.Max-expended, 0)
		newSlots[level] = ns
	}
	merged, err := json.Marshal(newSlots)
	if err != nil {
		return fresh
	}
	return pqtype.NullRawMessage{RawMessage: merged, Valid: true}
}

// SetPlayerCharacterPending flips a player_characters row back to pending,
// clearing any prior DM feedback (a player's edit needs fresh review).
func (a *BuilderStoreAdapter) SetPlayerCharacterPending(ctx context.Context, playerCharacterID string) error {
	id, err := uuid.Parse(playerCharacterID)
	if err != nil {
		return fmt.Errorf("invalid player_character_id %q: %w", playerCharacterID, err)
	}
	_, err = a.q.UpdatePlayerCharacterStatus(ctx, refdata.UpdatePlayerCharacterStatusParams{
		ID:     id,
		Status: "pending",
	})
	return err
}

// LoadEditSubmission reconstructs a builder submission from a saved character
// for edit-mode prefill, or ErrCharacterNotFound when it does not exist.
func (a *BuilderStoreAdapter) LoadEditSubmission(ctx context.Context, characterID string) (CharacterSubmission, error) {
	charID, err := uuid.Parse(characterID)
	if err != nil {
		return CharacterSubmission{}, fmt.Errorf("invalid character_id %q: %w", characterID, err)
	}
	ch, err := a.q.GetCharacter(ctx, charID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return CharacterSubmission{}, ErrCharacterNotFound
		}
		return CharacterSubmission{}, fmt.Errorf("getting character: %w", err)
	}
	return submissionFromCharacter(ch), nil
}

// submissionFromCharacter maps a persisted character back into the builder's
// CharacterSubmission shape. It is the inverse of buildCharacterColumns:
// background-pack items are dropped from Equipment (the build re-adds them) so
// an edit round-trip does not duplicate them. AbilityMethod is left empty since
// the generation method is not persisted; a player edit is then re-validated as
// point-buy (DM edits use the looser range check).
func submissionFromCharacter(ch refdata.Character) CharacterSubmission {
	sub := CharacterSubmission{
		Name:      ch.Name,
		Race:      ch.Race,
		Languages: ch.Languages,
	}

	var classes []character.ClassEntry
	if len(ch.Classes) > 0 {
		_ = json.Unmarshal(ch.Classes, &classes)
	}
	sub.Classes = classes
	if primary := primaryClassEntry(classes); primary != nil {
		sub.Class = primary.Class
		sub.Subclass = primary.Subclass
	}

	var scores character.AbilityScores
	if len(ch.AbilityScores) > 0 {
		_ = json.Unmarshal(ch.AbilityScores, &scores)
	}
	sub.AbilityScores = PointBuyScoresFromCharacter(scores)

	if ch.Proficiencies.Valid {
		var prof character.Proficiencies
		if json.Unmarshal(ch.Proficiencies.RawMessage, &prof) == nil {
			sub.Skills = prof.Skills
			sub.Expertise = prof.Expertise
		}
	}

	var background string
	if ch.CharacterData.Valid {
		var cd struct {
			Spells          []string `json:"spells"`
			WeaponMasteries []string `json:"weapon_masteries"`
			Subrace         string   `json:"subrace"`
			Background      string   `json:"background"`
			Appearance      string   `json:"appearance"`
			Backstory       string   `json:"backstory"`
		}
		if json.Unmarshal(ch.CharacterData.RawMessage, &cd) == nil {
			sub.Spells = cd.Spells
			sub.WeaponMasteries = cd.WeaponMasteries
			sub.Subrace = cd.Subrace
			sub.Background = cd.Background
			sub.Appearance = cd.Appearance
			sub.Backstory = cd.Backstory
			background = cd.Background
		}
	}

	if ch.Inventory.Valid {
		bgPack := make(map[string]bool)
		for _, id := range BackgroundEquipmentPack(background) {
			bgPack[id] = true
		}
		var items []character.InventoryItem
		if json.Unmarshal(ch.Inventory.RawMessage, &items) == nil {
			for _, it := range items {
				if bgPack[it.ItemID] {
					continue
				}
				sub.Equipment = append(sub.Equipment, it.ItemID)
			}
		}
	}
	if ch.EquippedMainHand.Valid {
		sub.EquippedWeapon = ch.EquippedMainHand.String
	}
	if ch.EquippedArmor.Valid {
		sub.WornArmor = ch.EquippedArmor.String
	}
	return sub
}

// HasActiveEncounter reports whether the character is in an active encounter.
func (a *BuilderStoreAdapter) HasActiveEncounter(ctx context.Context, characterID string) (bool, error) {
	charID, err := uuid.Parse(characterID)
	if err != nil {
		return false, fmt.Errorf("invalid character_id %q: %w", characterID, err)
	}
	_, err = a.q.GetActiveEncounterIDByCharacterID(ctx, uuid.NullUUID{UUID: charID, Valid: true})
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// CreatePlayerCharacterRecord creates a player_characters row.
func (a *BuilderStoreAdapter) CreatePlayerCharacterRecord(ctx context.Context, p CreatePlayerCharacterParams) (string, error) {
	campID, err := uuid.Parse(p.CampaignID)
	if err != nil {
		return "", fmt.Errorf("invalid campaign_id %q: %w", p.CampaignID, err)
	}
	charID, err := uuid.Parse(p.CharacterID)
	if err != nil {
		return "", fmt.Errorf("invalid character_id %q: %w", p.CharacterID, err)
	}

	pc, err := a.q.CreatePlayerCharacter(ctx, refdata.CreatePlayerCharacterParams{
		CampaignID:    campID,
		CharacterID:   charID,
		DiscordUserID: p.DiscordUserID,
		Status:        p.Status,
		CreatedVia:    p.CreatedVia,
	})
	if err != nil {
		return "", err
	}
	return pc.ID.String(), nil
}

// ActivePlayerCharacter returns the non-retired player_characters row for the
// (campaign, player), or (nil, nil) when none exists.
func (a *BuilderStoreAdapter) ActivePlayerCharacter(ctx context.Context, campaignID, discordUserID string) (*ActivePlayerCharacter, error) {
	campID, err := uuid.Parse(campaignID)
	if err != nil {
		return nil, err
	}
	pc, err := a.q.GetPlayerCharacterByDiscordUser(ctx, refdata.GetPlayerCharacterByDiscordUserParams{
		CampaignID:    campID,
		DiscordUserID: discordUserID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &ActivePlayerCharacter{ID: pc.ID.String(), Status: pc.Status}, nil
}

// RelinkPlayerCharacterRecord re-points an existing row at a freshly built
// character and resets it to pending.
func (a *BuilderStoreAdapter) RelinkPlayerCharacterRecord(ctx context.Context, pcID, characterID, createdVia string) (string, error) {
	id, err := uuid.Parse(pcID)
	if err != nil {
		return "", err
	}
	charID, err := uuid.Parse(characterID)
	if err != nil {
		return "", err
	}
	pc, err := a.q.RelinkPlayerCharacter(ctx, refdata.RelinkPlayerCharacterParams{
		ID:          id,
		CharacterID: charID,
		CreatedVia:  createdVia,
	})
	if err != nil {
		return "", err
	}
	return pc.ID.String(), nil
}

// ValidateToken checks that the token is valid and returns the token record.
func (a *BuilderStoreAdapter) ValidateToken(ctx context.Context, token string) (*PortalToken, error) {
	if a.tokenSvc == nil {
		return nil, nil
	}
	return a.tokenSvc.ValidateToken(ctx, token)
}

// RedeemToken marks the token as used.
func (a *BuilderStoreAdapter) RedeemToken(ctx context.Context, token string) error {
	if a.tokenSvc == nil {
		return nil
	}
	return a.tokenSvc.RedeemToken(ctx, token)
}

// SaveCharacterDraft upserts the in-progress builder draft for the
// (campaign, player, mode) key (T11 / Finding 4·b).
func (a *BuilderStoreAdapter) SaveCharacterDraft(ctx context.Context, campaignID, discordUserID, mode string, draft json.RawMessage) error {
	campID, err := uuid.Parse(campaignID)
	if err != nil {
		return fmt.Errorf("invalid campaign_id %q: %w", campaignID, err)
	}
	return a.q.UpsertCharacterDraft(ctx, refdata.UpsertCharacterDraftParams{
		CampaignID:    campID,
		DiscordUserID: discordUserID,
		Mode:          mode,
		Draft:         draft,
	})
}

// LoadCharacterDraft returns the stored builder draft for (campaign, player,
// mode), or (nil, nil) when no draft row exists.
func (a *BuilderStoreAdapter) LoadCharacterDraft(ctx context.Context, campaignID, discordUserID, mode string) (json.RawMessage, error) {
	campID, err := uuid.Parse(campaignID)
	if err != nil {
		return nil, fmt.Errorf("invalid campaign_id %q: %w", campaignID, err)
	}
	draft, err := a.q.GetCharacterDraft(ctx, refdata.GetCharacterDraftParams{
		CampaignID:    campID,
		DiscordUserID: discordUserID,
		Mode:          mode,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return draft, nil
}

// AllowedAbilityScoreMethods reads campaign settings for creation method gating.
func (a *BuilderStoreAdapter) AllowedAbilityScoreMethods(ctx context.Context, campaignID string) ([]AbilityScoreMethod, error) {
	getter, ok := a.q.(campaignGetter)
	if !ok {
		return DefaultAbilityScoreMethods(), nil
	}
	id, err := uuid.Parse(campaignID)
	if err != nil {
		return DefaultAbilityScoreMethods(), nil
	}
	campaign, err := getter.GetCampaignByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !campaign.Settings.Valid {
		return DefaultAbilityScoreMethods(), nil
	}
	return AbilityScoreMethodsFromSettings(campaign.Settings.RawMessage), nil
}

// parseEquipmentEntry splits an equipment token into its item ID, quantity, and
// whether the quantity was set explicitly. A trailing ":N" sets the quantity
// ("crossbow-bolt:20" -> 20, explicit); a missing or malformed suffix yields 1
// (not explicit), letting the caller substitute the catalog's default quantity.
func parseEquipmentEntry(entry string) (id string, qty int, explicit bool) {
	id, qty = entry, 1
	if i := strings.LastIndex(entry, ":"); i >= 0 {
		if n, err := strconv.Atoi(entry[i+1:]); err == nil && n > 0 {
			id, qty, explicit = entry[:i], n, true
		}
	}
	return id, qty, explicit
}

// EquipmentToInventory converts equipment ID strings to InventoryItem structs.
func EquipmentToInventory(equipment []string) []character.InventoryItem {
	return EquipmentToInventoryWithEquipped(equipment, "", "")
}

// EquipmentToInventoryWithEquipped converts equipment IDs to InventoryItems,
// marking the equipped weapon and worn armor items. Unresolved placeholder IDs
// (e.g. "any-martial", "any-simple-melee") are skipped — the client must
// resolve these to concrete item IDs before submission.
func EquipmentToInventoryWithEquipped(equipment []string, equippedWeapon, wornArmor string) []character.InventoryItem {
	if len(equipment) == 0 {
		return nil
	}
	// Resolve item name / type / default quantity from the canonical catalog
	// (ISSUE-017 phase 3) instead of the old hand-maintained knownWeapons/
	// knownArmor/knownAmmo maps. The contract test
	// TestItemCatalog_CoversAllBuilderEquipmentIDs guarantees every concrete
	// builder id has a catalog row, so the "gear"/raw-id fallback below only
	// ever applies to genuinely off-catalog ids (e.g. magic items).
	catalog := refdata.ItemCatalogByID()
	items := make([]character.InventoryItem, 0, len(equipment))
	for _, raw := range equipment {
		// A token may batch comma-separated items ("light-crossbow:1,crossbow-bolt:20")
		// and each may carry a ":N" quantity. Split both so a quiver lands as 20.
		for entry := range strings.SplitSeq(raw, ",") {
			id, qty, explicit := parseEquipmentEntry(strings.TrimSpace(entry))
			if id == "" || strings.HasPrefix(id, "any-") {
				continue // skip empty / unresolved placeholder
			}
			name, typ := id, "gear"
			if e, ok := catalog[id]; ok {
				name, typ = e.Name, e.Category
				if !explicit {
					qty = e.DefaultQuantity // catalog default unless a ":N" overrides it
				}
			}
			item := character.InventoryItem{
				ItemID:   id,
				Name:     name,
				Quantity: qty,
				Type:     typ,
			}
			if equippedWeapon != "" && strings.EqualFold(id, equippedWeapon) {
				item.Equipped = true
				item.EquipSlot = "main_hand"
			}
			if wornArmor != "" && strings.EqualFold(id, wornArmor) {
				item.Equipped = true
				item.EquipSlot = "armor"
			}
			if strings.EqualFold(id, "shield") {
				item.Equipped = true
				item.EquipSlot = "off_hand"
			}
			items = append(items, item)
		}
	}
	return items
}

// raceSpeed maps race IDs to their base walking speed in feet.
var raceSpeed = map[string]int{
	"dwarf":    25,
	"halfling": 25,
	"gnome":    25,
}

// DeriveSpeed returns the base walking speed for a race (30 ft default).
func DeriveSpeed(race string) int {
	if spd, ok := raceSpeed[strings.ToLower(race)]; ok {
		return spd
	}
	return 30
}

// ClassHitDie returns the hit die string for a class.
func ClassHitDie(class string) string {
	switch strings.ToLower(class) {
	case "barbarian":
		return "d12"
	case "fighter", "paladin", "ranger":
		return "d10"
	case "sorcerer", "wizard":
		return "d6"
	default:
		return "d8"
	}
}

// DeriveHP calculates HP for a level 1 character.
func DeriveHP(class string, scores character.AbilityScores) int {
	classes := []character.ClassEntry{{Class: class, Level: 1}}
	hitDice := map[string]string{class: ClassHitDie(class)}
	return character.CalculateHP(classes, hitDice, scores)
}

// DeriveHPMulticlass calculates HP for a multiclass character using all class entries.
func DeriveHPMulticlass(classes []character.ClassEntry, scores character.AbilityScores) int {
	hitDice := make(map[string]string, len(classes))
	for _, c := range classes {
		hitDice[c.Class] = ClassHitDie(c.Class)
	}
	return character.CalculateHP(classes, hitDice, scores)
}

// DeriveAC calculates AC for a character with no armor.
func DeriveAC(scores character.AbilityScores) int {
	return character.CalculateAC(scores, nil, false, "")
}
