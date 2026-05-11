package combat

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/refdata"
)

// CreateZoneInput holds parameters for creating an encounter zone.
type CreateZoneInput struct {
	EncounterID           uuid.UUID
	SourceCombatantID     uuid.UUID
	SourceSpell           string
	Shape                 string
	OriginCol             string
	OriginRow             int32
	Dimensions            json.RawMessage
	AnchorMode            string
	AnchorCombatantID     uuid.NullUUID
	ZoneType              string
	OverlayColor          string
	MarkerIcon            string
	RequiresConcentration bool
	ExpiresAtRound        sql.NullInt32
	Triggers              []ZoneTrigger
}

// ZoneInfo holds the result of creating or listing a zone.
type ZoneInfo struct {
	ID                    uuid.UUID
	EncounterID           uuid.UUID
	SourceCombatantID     uuid.UUID
	SourceSpell           string
	Shape                 string
	OriginCol             string
	OriginRow             int32
	Dimensions            json.RawMessage
	AnchorMode            string
	AnchorCombatantID     uuid.NullUUID
	ZoneType              string
	OverlayColor          string
	MarkerIcon            string
	RequiresConcentration bool
	ExpiresAtRound        sql.NullInt32
	Triggers              []ZoneTrigger
}

// ZoneTriggerResult holds the outcome of a zone trigger check.
type ZoneTriggerResult struct {
	ZoneID      uuid.UUID
	SourceSpell string
	ZoneType    string
	Effect      string
	Trigger     string
	Details     map[string]interface{}
}

// parseTriggers extracts zone triggers from a nullable JSON field.
func parseTriggers(raw pqtype.NullRawMessage) []ZoneTrigger {
	if !raw.Valid || len(raw.RawMessage) == 0 {
		return nil
	}
	var triggers []ZoneTrigger
	_ = json.Unmarshal(raw.RawMessage, &triggers)
	return triggers
}

// parseTriggeredMap extracts the per-combatant trigger tracking map.
func parseTriggeredMap(raw pqtype.NullRawMessage) map[string]bool {
	triggered := make(map[string]bool)
	if !raw.Valid || len(raw.RawMessage) == 0 {
		return triggered
	}
	_ = json.Unmarshal(raw.RawMessage, &triggered)
	return triggered
}

// zoneInfoFromModel converts a refdata.EncounterZone to a ZoneInfo.
func zoneInfoFromModel(z refdata.EncounterZone) ZoneInfo {
	markerIcon := ""
	if z.MarkerIcon.Valid {
		markerIcon = z.MarkerIcon.String
	}
	return ZoneInfo{
		ID:                    z.ID,
		EncounterID:           z.EncounterID,
		SourceCombatantID:     z.SourceCombatantID,
		SourceSpell:           z.SourceSpell,
		Shape:                 z.Shape,
		OriginCol:             z.OriginCol,
		OriginRow:             z.OriginRow,
		Dimensions:            z.Dimensions,
		AnchorMode:            z.AnchorMode,
		AnchorCombatantID:     z.AnchorCombatantID,
		ZoneType:              z.ZoneType,
		OverlayColor:          z.OverlayColor,
		MarkerIcon:            markerIcon,
		RequiresConcentration: z.RequiresConcentration,
		ExpiresAtRound:        z.ExpiresAtRound,
		Triggers:              parseTriggers(z.ZoneTriggers),
	}
}

// CreateZone inserts a new encounter zone.
func (s *Service) CreateZone(ctx context.Context, input CreateZoneInput) (ZoneInfo, error) {
	triggersJSON, err := json.Marshal(input.Triggers)
	if err != nil {
		return ZoneInfo{}, fmt.Errorf("marshalling triggers: %w", err)
	}

	zone, err := s.store.CreateEncounterZone(ctx, refdata.CreateEncounterZoneParams{
		EncounterID:       input.EncounterID,
		SourceCombatantID: input.SourceCombatantID,
		SourceSpell:       input.SourceSpell,
		Shape:             input.Shape,
		OriginCol:         input.OriginCol,
		OriginRow:         input.OriginRow,
		Dimensions:        input.Dimensions,
		AnchorMode:        input.AnchorMode,
		AnchorCombatantID: input.AnchorCombatantID,
		ZoneType:          input.ZoneType,
		OverlayColor:      input.OverlayColor,
		MarkerIcon:        sql.NullString{String: input.MarkerIcon, Valid: input.MarkerIcon != ""},
		RequiresConcentration: input.RequiresConcentration,
		ExpiresAtRound:        input.ExpiresAtRound,
		ZoneTriggers: pqtype.NullRawMessage{
			RawMessage: triggersJSON,
			Valid:      true,
		},
		TriggeredThisRound: pqtype.NullRawMessage{
			RawMessage: json.RawMessage(`{}`),
			Valid:      true,
		},
	})
	if err != nil {
		return ZoneInfo{}, fmt.Errorf("creating encounter zone: %w", err)
	}

	// Phase 118: Silence zones break concentration for any concentrating
	// caster (V/S spell) standing in the new zone's footprint at creation
	// time. Movement-based entry is handled separately in
	// Service.UpdateCombatantPosition.
	if input.ZoneType == "silence" {
		if err := s.breakSilenceCaughtConcentrators(ctx, input.EncounterID); err != nil {
			return ZoneInfo{}, fmt.Errorf("processing silence-zone concentration breaks: %w", err)
		}
	}

	return zoneInfoFromModel(zone), nil
}

// breakSilenceCaughtConcentrators iterates every combatant in the encounter
// and runs CheckSilenceBreaksConcentration. Used by the zone-creation hook.
func (s *Service) breakSilenceCaughtConcentrators(ctx context.Context, encounterID uuid.UUID) error {
	combatants, err := s.store.ListCombatantsByEncounterID(ctx, encounterID)
	if err != nil {
		return fmt.Errorf("listing combatants: %w", err)
	}
	for _, c := range combatants {
		if _, err := s.CheckSilenceBreaksConcentration(ctx, c.ID); err != nil {
			return fmt.Errorf("silence check for %s: %w", c.DisplayName, err)
		}
	}
	return nil
}

// DeleteZone removes a zone by ID (DM manual removal).
func (s *Service) DeleteZone(ctx context.Context, zoneID uuid.UUID) error {
	return s.store.DeleteEncounterZone(ctx, zoneID)
}

// CleanupConcentrationZones deletes all concentration zones for a combatant
// and returns the number of rows deleted.
func (s *Service) CleanupConcentrationZones(ctx context.Context, combatantID uuid.UUID) (int64, error) {
	return s.store.DeleteConcentrationZonesByCombatant(ctx, combatantID)
}

// CleanupExpiredZones deletes zones that have expired by the given round.
func (s *Service) CleanupExpiredZones(ctx context.Context, encounterID uuid.UUID, currentRound int32) error {
	return s.store.DeleteExpiredZones(ctx, refdata.DeleteExpiredZonesParams{
		EncounterID:    encounterID,
		ExpiresAtRound: sql.NullInt32{Int32: currentRound, Valid: true},
	})
}

// CleanupEncounterZones deletes all zones for an encounter (encounter end cleanup).
func (s *Service) CleanupEncounterZones(ctx context.Context, encounterID uuid.UUID) error {
	return s.store.DeleteEncounterZonesByEncounterID(ctx, encounterID)
}

// UpdateZoneAnchor updates the origin of all zones anchored to a combatant.
func (s *Service) UpdateZoneAnchor(ctx context.Context, combatantID uuid.UUID, newCol string, newRow int32) error {
	combatant, err := s.store.GetCombatant(ctx, combatantID)
	if err != nil {
		return fmt.Errorf("getting combatant: %w", err)
	}

	zones, err := s.store.ListEncounterZonesByEncounterID(ctx, combatant.EncounterID)
	if err != nil {
		return fmt.Errorf("listing zones: %w", err)
	}

	for _, z := range zones {
		if z.AnchorMode != "combatant" || !z.AnchorCombatantID.Valid || z.AnchorCombatantID.UUID != combatantID {
			continue
		}
		if _, err := s.store.UpdateEncounterZoneOrigin(ctx, refdata.UpdateEncounterZoneOriginParams{
			ID:        z.ID,
			OriginCol: newCol,
			OriginRow: newRow,
		}); err != nil {
			return fmt.Errorf("updating zone origin %s: %w", z.ID, err)
		}
	}
	return nil
}

// CheckZoneTriggers checks if a combatant at the given position triggers any zone effects.
func (s *Service) CheckZoneTriggers(ctx context.Context, combatantID uuid.UUID, col, row int, encounterID uuid.UUID, triggerType string) ([]ZoneTriggerResult, error) {
	zones, err := s.store.ListEncounterZonesByEncounterID(ctx, encounterID)
	if err != nil {
		return nil, fmt.Errorf("listing zones: %w", err)
	}

	combatantKey := combatantID.String()
	var results []ZoneTriggerResult
	for _, z := range zones {
		triggers := parseTriggers(z.ZoneTriggers)

		// Find matching trigger for the requested type
		var matchingTrigger *ZoneTrigger
		for i := range triggers {
			if triggers[i].Trigger == triggerType {
				matchingTrigger = &triggers[i]
				break
			}
		}
		if matchingTrigger == nil {
			continue
		}

		// Parse triggered-this-round map once for both check and update
		triggered := parseTriggeredMap(z.TriggeredThisRound)
		if triggered[combatantKey] {
			continue
		}

		if !tileInSet(col, row, zoneAffectedTiles(z)) {
			continue
		}

		// Mark as triggered for this combatant this round
		triggered[combatantKey] = true
		triggeredJSON, _ := json.Marshal(triggered)

		if _, err := s.store.UpdateEncounterZoneTriggeredThisRound(ctx, refdata.UpdateEncounterZoneTriggeredThisRoundParams{
			ID: z.ID,
			TriggeredThisRound: pqtype.NullRawMessage{
				RawMessage: triggeredJSON,
				Valid:      true,
			},
		}); err != nil {
			return nil, fmt.Errorf("updating triggered_this_round: %w", err)
		}

		results = append(results, ZoneTriggerResult{
			ZoneID:      z.ID,
			SourceSpell: z.SourceSpell,
			ZoneType:    z.ZoneType,
			Effect:      matchingTrigger.Effect,
			Trigger:     matchingTrigger.Trigger,
			Details:     matchingTrigger.Details,
		})
	}

	return results, nil
}

// ResetZoneTriggersForRound resets all triggered_this_round tracking for an encounter.
func (s *Service) ResetZoneTriggersForRound(ctx context.Context, encounterID uuid.UUID) error {
	return s.store.ResetAllTriggeredThisRound(ctx, encounterID)
}

// ListZonesForEncounter returns all active zones for an encounter.
func (s *Service) ListZonesForEncounter(ctx context.Context, encounterID uuid.UUID) ([]ZoneInfo, error) {
	zones, err := s.store.ListEncounterZonesByEncounterID(ctx, encounterID)
	if err != nil {
		return nil, fmt.Errorf("listing zones: %w", err)
	}
	result := make([]ZoneInfo, len(zones))
	for i, z := range zones {
		result[i] = zoneInfoFromModel(z)
	}
	return result, nil
}

// zoneAffectedTiles calculates the tiles affected by a zone based on its shape and dimensions.
func zoneAffectedTiles(z refdata.EncounterZone) []GridPos {
	return ZoneAffectedTilesFromShape(z.Shape, z.OriginCol, z.OriginRow, z.Dimensions)
}

// ZoneAffectedTilesFromShape is the exported helper used by the discord map
// renderer to compute zone-overlay tile coverage without needing the full
// refdata.EncounterZone row. (E-67-zone-render-on-map)
func ZoneAffectedTilesFromShape(shape, originColLetter string, originRow int32, dimensions []byte) []GridPos {
	originCol := colToIndex(originColLetter)
	originRowIdx := int(originRow) - 1

	var dims struct {
		RadiusFt int `json:"radius_ft"`
		WidthFt  int `json:"width_ft"`
		HeightFt int `json:"height_ft"`
		LengthFt int `json:"length_ft"`
		SideFt   int `json:"side_ft"`
	}
	_ = json.Unmarshal(dimensions, &dims)

	switch shape {
	case "circle":
		return SphereAffectedTiles(originCol, originRowIdx, dims.RadiusFt)
	case "square":
		return SquareAffectedTiles(originCol, originRowIdx, dims.SideFt)
	case "rectangle":
		return SquareAffectedTiles(originCol, originRowIdx, dims.WidthFt)
	default:
		return []GridPos{{Col: originCol, Row: originRowIdx}}
	}
}

// ZoneOriginIndex returns the 0-based (col, row) for the given zone origin
// in letter/row form ("C", 3) — useful for the discord map renderer that
// only needs the origin marker placement. (E-67-zone-render-on-map)
func ZoneOriginIndex(originColLetter string, originRow int32) (int, int) {
	return colToIndex(originColLetter), int(originRow) - 1
}

// tileInSet checks if a position (0-based col, 0-based row) is in the set of affected tiles.
func tileInSet(col, row int, tiles []GridPos) bool {
	for _, t := range tiles {
		if t.Col == col && t.Row == row {
			return true
		}
	}
	return false
}
