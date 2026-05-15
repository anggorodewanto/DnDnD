package exploration

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/gamemap/renderer"
	"github.com/ab/dndnd/internal/refdata"
)

// Store is the database surface the exploration service depends on.
// It intentionally overlaps with combat.Store (same sqlc package) but is
// scoped to just the queries exploration mode uses.
type Store interface {
	CreateExplorationEncounter(ctx context.Context, arg refdata.CreateExplorationEncounterParams) (refdata.Encounter, error)
	GetEncounter(ctx context.Context, id uuid.UUID) (refdata.Encounter, error)
	UpdateEncounterStatus(ctx context.Context, arg refdata.UpdateEncounterStatusParams) (refdata.Encounter, error)
	UpdateEncounterMode(ctx context.Context, arg refdata.UpdateEncounterModeParams) (refdata.Encounter, error)

	GetMapByIDUnchecked(ctx context.Context, id uuid.UUID) (refdata.Map, error)
	GetCharacter(ctx context.Context, id uuid.UUID) (refdata.Character, error)

	CreateCombatant(ctx context.Context, arg refdata.CreateCombatantParams) (refdata.Combatant, error)
	ListCombatantsByEncounterID(ctx context.Context, encounterID uuid.UUID) ([]refdata.Combatant, error)
	UpdateCombatantPosition(ctx context.Context, arg refdata.UpdateCombatantPositionParams) (refdata.Combatant, error)
}

// Service orchestrates exploration-mode encounters.
type Service struct {
	store Store
}

// NewService creates a new exploration Service.
func NewService(store Store) *Service {
	return &Service{store: store}
}

// StartInput describes parameters for starting an exploration encounter.
type StartInput struct {
	CampaignID   uuid.UUID
	MapID        uuid.UUID
	Name         string
	DisplayName  string
	CharacterIDs []uuid.UUID
}

// StartResult is the outcome of StartExploration.
type StartResult struct {
	Encounter refdata.Encounter
	// PCs maps character ID to its assigned combatant-style position
	// (col is a renderer column label like "A", row is 1-indexed).
	PCs map[uuid.UUID]combat.Position
}

// TransitionResult is the captured handoff from exploration mode to combat.
type TransitionResult struct {
	Encounter refdata.Encounter
	Positions map[uuid.UUID]combat.Position
}

// StartExploration creates a new exploration-mode encounter on the given map,
// reads spawn zones from the map's tiled_json, and seats each provided PC at
// a player spawn tile (row-major, deterministic).
func (s *Service) StartExploration(ctx context.Context, input StartInput) (StartResult, error) {
	m, err := s.store.GetMapByIDUnchecked(ctx, input.MapID)
	if err != nil {
		return StartResult{}, fmt.Errorf("getting map: %w", err)
	}

	zones, err := ParseSpawnZones(m.TiledJson)
	if err != nil {
		return StartResult{}, fmt.Errorf("parsing spawn zones: %w", err)
	}

	// Deterministic PC ID order = input order; assignment goes row-major.
	pcKeys := make([]string, len(input.CharacterIDs))
	for i, id := range input.CharacterIDs {
		pcKeys[i] = id.String()
	}
	assigned, err := AssignPCsToSpawnZones(zones, pcKeys)
	if err != nil {
		return StartResult{}, err
	}

	var displayName sql.NullString
	if input.DisplayName != "" {
		displayName = sql.NullString{String: input.DisplayName, Valid: true}
	}

	enc, err := s.store.CreateExplorationEncounter(ctx, refdata.CreateExplorationEncounterParams{
		CampaignID:  input.CampaignID,
		MapID:       uuid.NullUUID{UUID: input.MapID, Valid: true},
		Name:        input.Name,
		DisplayName: displayName,
	})
	if err != nil {
		return StartResult{}, fmt.Errorf("creating exploration encounter: %w", err)
	}

	out := StartResult{
		Encounter: enc,
		PCs:       make(map[uuid.UUID]combat.Position, len(input.CharacterIDs)),
	}

	for _, charID := range input.CharacterIDs {
		tile, ok := assigned[charID.String()]
		if !ok {
			continue
		}
		char, err := s.store.GetCharacter(ctx, charID)
		if err != nil {
			return StartResult{}, fmt.Errorf("getting character %s: %w", charID, err)
		}
		col := renderer.ColumnLabel(tile.Col)
		row := int32(tile.Row + 1) // tile.Row is 0-indexed; combatant rows are 1-indexed
		params := combat.CombatantFromCharacter(char, combat.ShortIDFromName(char.Name), col, row)
		if _, err := s.createCombatantFromParams(ctx, enc.ID, params); err != nil {
			return StartResult{}, fmt.Errorf("creating PC combatant for %s: %w", char.Name, err)
		}
		out.PCs[charID] = combat.Position{Col: col, Row: row}
	}

	return out, nil
}

// createCombatantFromParams translates a combat.CombatantParams into the sqlc
// CreateCombatantParams shape. This mirrors the combat service's AddCombatant
// helper but without the full set of concentration/bardic-inspiration tracking
// that isn't used in exploration.
func (s *Service) createCombatantFromParams(ctx context.Context, encID uuid.UUID, p combat.CombatantParams) (refdata.Combatant, error) {
	var charID uuid.NullUUID
	if p.CharacterID != "" {
		if parsed, err := uuid.Parse(p.CharacterID); err == nil {
			charID = uuid.NullUUID{UUID: parsed, Valid: true}
		}
	}
	return s.store.CreateCombatant(ctx, refdata.CreateCombatantParams{
		EncounterID:     encID,
		CharacterID:     charID,
		ShortID:         p.ShortID,
		DisplayName:     p.DisplayName,
		HpMax:           p.HPMax,
		HpCurrent:       p.HPCurrent,
		TempHp:          p.TempHP,
		Ac:              p.AC,
		PositionCol:     p.PositionCol,
		PositionRow:     p.PositionRow,
		Conditions:      json.RawMessage(`[]`),
		ExhaustionLevel: p.ExhaustionLevel,
		IsAlive:         p.IsAlive,
		IsVisible:       p.IsVisible,
		IsNpc:           p.IsNPC,
		DeathSaves:      nullRaw(p.DeathSaves),
	})
}

func nullRaw(r json.RawMessage) pqtype.NullRawMessage {
	if len(r) == 0 {
		return pqtype.NullRawMessage{}
	}
	return pqtype.NullRawMessage{RawMessage: r, Valid: true}
}

// EndExploration marks an exploration encounter as completed.
// Returns ErrEncounterNotExploration if the encounter is in combat mode.
func (s *Service) EndExploration(ctx context.Context, encounterID uuid.UUID) error {
	enc, err := s.store.GetEncounter(ctx, encounterID)
	if err != nil {
		return fmt.Errorf("getting encounter: %w", err)
	}
	if enc.Mode != "exploration" {
		return ErrEncounterNotExploration
	}
	_, err = s.store.UpdateEncounterStatus(ctx, refdata.UpdateEncounterStatusParams{
		ID:     encounterID,
		Status: "completed",
	})
	return err
}

// CapturePositions reads the current PC positions from an exploration encounter
// and returns them in combat.Position form, keyed by character ID, so they can
// be fed into combat.StartCombatInput.CharacterPositions. Non-PC combatants
// (creatures without character_id) are ignored.
func (s *Service) CapturePositions(ctx context.Context, encounterID uuid.UUID) (map[uuid.UUID]combat.Position, error) {
	enc, err := s.store.GetEncounter(ctx, encounterID)
	if err != nil {
		return nil, fmt.Errorf("getting encounter: %w", err)
	}
	if enc.Mode != "exploration" {
		return nil, ErrEncounterNotExploration
	}
	combatants, err := s.store.ListCombatantsByEncounterID(ctx, encounterID)
	if err != nil {
		return nil, fmt.Errorf("listing combatants: %w", err)
	}
	out := make(map[uuid.UUID]combat.Position, len(combatants))
	for _, c := range combatants {
		if !c.CharacterID.Valid {
			continue
		}
		out[c.CharacterID.UUID] = combat.Position{Col: c.PositionCol, Row: c.PositionRow}
	}
	return out, nil
}

// TransitionToCombat captures current PC positions and flips the encounter to
// combat mode so turn structure can be added by the combat dashboard flow.
func (s *Service) TransitionToCombat(ctx context.Context, encounterID uuid.UUID) (TransitionResult, error) {
	positions, err := s.CapturePositions(ctx, encounterID)
	if err != nil {
		return TransitionResult{}, err
	}
	enc, err := s.store.UpdateEncounterMode(ctx, refdata.UpdateEncounterModeParams{
		ID:   encounterID,
		Mode: "combat",
	})
	if err != nil {
		return TransitionResult{}, fmt.Errorf("updating encounter mode: %w", err)
	}
	return TransitionResult{
		Encounter: enc,
		Positions: positions,
	}, nil
}

// ApplyPositionOverrides returns a new map that is base + overrides (overrides
// take precedence). Nil/empty overrides yields base unchanged.
func ApplyPositionOverrides(base, overrides map[uuid.UUID]combat.Position) map[uuid.UUID]combat.Position {
	out := make(map[uuid.UUID]combat.Position, len(base))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range overrides {
		out[k] = v
	}
	return out
}
