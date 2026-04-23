package open5e

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/refdata"
)

// CacheStore is the minimal refdata surface Cache needs. It is defined
// as an interface so unit tests can swap in an in-memory fake.
type CacheStore interface {
	UpsertCreature(ctx context.Context, arg refdata.UpsertCreatureParams) error
	UpsertSpell(ctx context.Context, arg refdata.UpsertSpellParams) error
}

// Cache persists fetched Open5e content into the refdata tables with a
// source attribution like "open5e:<document_slug>" so that campaign-
// aware filtering in statblocklibrary can show/hide by document.
type Cache struct {
	store  CacheStore
	logger *slog.Logger
}

// NewCache returns a Cache backed by the given store.
func NewCache(store CacheStore) *Cache {
	return &Cache{store: store, logger: slog.Default()}
}

// NewCacheWithLogger is like NewCache but uses the given logger for
// warnings about partial Open5e payloads. Handy in tests.
func NewCacheWithLogger(store CacheStore, logger *slog.Logger) *Cache {
	if logger == nil {
		logger = slog.Default()
	}
	return &Cache{store: store, logger: logger}
}

// idPrefix prefixes the Open5e slug so cached rows cannot collide with
// existing SRD ids (e.g. SRD "goblin" vs Open5e "goblin" from Tome of
// Beasts). All DnDnD code that dereferences a creature id by slug uses
// the SRD-only slug, so Open5e rows sit in a disjoint namespace.
const idPrefix = "open5e_"

// CacheMonster upserts an Open5e monster into the creatures table. The
// row is global (campaign_id NULL) and uses source = open5e:<slug>.
// Returns the cached creature id.
func (c *Cache) CacheMonster(ctx context.Context, m Monster) (string, error) {
	if strings.TrimSpace(m.Slug) == "" {
		return "", errors.New("open5e: cache monster with empty slug")
	}
	id := idPrefix + m.Slug
	params := c.monsterToParams(id, m)
	if err := c.store.UpsertCreature(ctx, params); err != nil {
		return "", fmt.Errorf("open5e: upsert creature %s: %w", id, err)
	}
	return id, nil
}

// CacheSpell upserts an Open5e spell into the spells table.
func (c *Cache) CacheSpell(ctx context.Context, s Spell) (string, error) {
	if strings.TrimSpace(s.Slug) == "" {
		return "", errors.New("open5e: cache spell with empty slug")
	}
	id := idPrefix + s.Slug
	params := c.spellToParams(id, s)
	if err := c.store.UpsertSpell(ctx, params); err != nil {
		return "", fmt.Errorf("open5e: upsert spell %s: %w", id, err)
	}
	return id, nil
}

// SourceAttribution returns the source column value for a given document
// slug ("open5e:<slug>"), falling back to "open5e:unknown" if empty.
func SourceAttribution(documentSlug string) string {
	slug := strings.TrimSpace(documentSlug)
	if slug == "" {
		return "open5e:unknown"
	}
	return "open5e:" + slug
}

// --- Monster → UpsertCreatureParams ---

// defaultAbilityScores returns a {10,10,10,10,10,10} block used when an
// Open5e monster omits some ability scores.
func defaultAbilityScores() json.RawMessage {
	return json.RawMessage(`{"str":10,"dex":10,"con":10,"int":10,"wis":10,"cha":10}`)
}

// defaultSpeed returns a basic walk speed used when Open5e omits speed.
func defaultSpeed() json.RawMessage {
	return json.RawMessage(`{"walk":30}`)
}

// defaultCR is the CR used when Open5e sends nothing.
const defaultCR = "0"

// monsterToParams deterministically maps an Open5e monster onto the
// refdata creatures columns, applying sane defaults for missing fields
// and logging a warning when a caller should be aware.
func (c *Cache) monsterToParams(id string, m Monster) refdata.UpsertCreatureParams {
	size := strings.TrimSpace(m.Size)
	if size == "" {
		c.logger.Warn("open5e monster missing size; defaulting to Medium", "slug", m.Slug)
		size = "Medium"
	}
	typ := strings.TrimSpace(m.Type)
	if typ == "" {
		c.logger.Warn("open5e monster missing type; defaulting to beast", "slug", m.Slug)
		typ = "beast"
	}
	cr := strings.TrimSpace(m.ChallengeRating)
	if cr == "" {
		c.logger.Warn("open5e monster missing CR; defaulting to 0", "slug", m.Slug)
		cr = defaultCR
	}
	speed := m.Speed
	if len(speed) == 0 {
		speed = defaultSpeed()
	}
	abilityScores := abilityScoresJSON(m)

	return refdata.UpsertCreatureParams{
		ID:            id,
		CampaignID:    uuid.NullUUID{Valid: false},
		Name:          m.Name,
		Size:          size,
		Type:          typ,
		Alignment:     nullString(m.Alignment),
		Ac:            int32(m.ArmorClass),
		AcType:        nullString(m.ArmorDesc),
		HpFormula:     m.HitDice,
		HpAverage:     int32(m.HitPoints),
		Speed:         speed,
		AbilityScores: abilityScores,
		// DnDnD's creatures.attacks is a JSON array of parsed attacks;
		// Open5e ships raw "actions" and "special_abilities" as prose.
		// We stash the raw JSON array (or [] fallback) so that downstream
		// stat-block rendering can display at least something without
		// trying to parse damage dice out of natural language.
		Attacks:   nonEmptyJSON(m.Actions),
		Abilities: nullRawMessage(m.SpecialAbilities),
		Cr:        cr,
		Homebrew:  sql.NullBool{Bool: false, Valid: true},
		Source:    sql.NullString{String: SourceAttribution(m.DocumentSlug), Valid: true},
	}
}

// abilityScoresJSON emits a DnDnD-shaped JSON block. Missing scores
// fall back to 10 so the column remains non-null and parseable.
func abilityScoresJSON(m Monster) json.RawMessage {
	if m.Strength == 0 && m.Dexterity == 0 && m.Constitution == 0 &&
		m.Intelligence == 0 && m.Wisdom == 0 && m.Charisma == 0 {
		return defaultAbilityScores()
	}
	b, _ := json.Marshal(map[string]int{
		"str": nz(m.Strength), "dex": nz(m.Dexterity), "con": nz(m.Constitution),
		"int": nz(m.Intelligence), "wis": nz(m.Wisdom), "cha": nz(m.Charisma),
	})
	return b
}

// nz returns v if non-zero, else 10 (sensible D&D default).
func nz(v int) int {
	if v == 0 {
		return 10
	}
	return v
}

// --- Spell → UpsertSpellParams ---

// spellToParams maps an Open5e spell onto refdata.UpsertSpellParams,
// applying defaults for missing fields.
func (c *Cache) spellToParams(id string, s Spell) refdata.UpsertSpellParams {
	school := strings.TrimSpace(s.School)
	if school == "" {
		c.logger.Warn("open5e spell missing school; defaulting to evocation", "slug", s.Slug)
		school = "evocation"
	}
	castingTime := strings.TrimSpace(s.CastingTime)
	if castingTime == "" {
		castingTime = "1 action"
	}
	rangeType := strings.TrimSpace(s.Range)
	if rangeType == "" {
		rangeType = "self"
	}
	duration := strings.TrimSpace(s.Duration)
	if duration == "" {
		duration = "Instantaneous"
	}
	return refdata.UpsertSpellParams{
		ID:                  id,
		Name:                s.Name,
		Level:               int32(s.LevelInt),
		School:              school,
		CastingTime:         castingTime,
		RangeType:           rangeType,
		Components:          parseComponents(s.Components),
		MaterialDescription: nullString(s.Material),
		Duration:            duration,
		Concentration:       sql.NullBool{Bool: s.Concentration, Valid: true},
		Ritual:              sql.NullBool{Bool: s.Ritual, Valid: true},
		Description:         s.Description,
		HigherLevels:        nullString(s.HigherLevel),
		// Resolution mode is DnDnD's internal classification for how a
		// spell's effect resolves. Open5e does not provide this. Default
		// to "manual" so spells need DM adjudication unless upgraded later.
		ResolutionMode:    "manual",
		Classes:           parseClasses(s.DndClass),
		ConditionsApplied: []string{},
		CampaignID:        uuid.NullUUID{Valid: false},
		Homebrew:          sql.NullBool{Bool: false, Valid: true},
		Source:            sql.NullString{String: SourceAttribution(s.DocumentSlug), Valid: true},
	}
}

// parseComponents splits "V, S, M" into ["V","S","M"], uppercasing each.
func parseComponents(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return []string{}
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		t := strings.ToUpper(strings.TrimSpace(p))
		if t == "" {
			continue
		}
		out = append(out, t)
	}
	return out
}

// parseClasses splits "Sorcerer, Wizard" into ["sorcerer","wizard"].
func parseClasses(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return []string{}
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		t := strings.ToLower(strings.TrimSpace(p))
		if t == "" {
			continue
		}
		out = append(out, t)
	}
	return out
}

// --- small helpers ---

func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

func nonEmptyJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage(`[]`)
	}
	return raw
}

func nullRawMessage(raw json.RawMessage) pqtype.NullRawMessage {
	if len(raw) == 0 {
		return pqtype.NullRawMessage{}
	}
	return pqtype.NullRawMessage{RawMessage: raw, Valid: true}
}
