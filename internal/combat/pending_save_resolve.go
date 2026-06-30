package combat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/refdata"
)

// ISSUE-043: DM-side resolution of a MONSTER/NPC's pending AoE saving throw.
//
// Players resolve their own AoE saves via the Discord /save command
// (RecordAoEPendingSaveRoll). Monsters had no path, so a monster's pending_saves
// row stayed "pending" forever and the all-rows-resolved gate in
// ResolveAoEPendingSaves never fired — blocking AoE damage from landing on
// anyone in the blast. ResolveMonsterPendingSave is the DM-driven counterpart:
// it rolls the monster's save, writes the row via the same RecordAoEPendingSaveRoll
// path players use, and runs the apply step once the cast is fully resolved.

// pending_saves.status lifecycle values. status is a free-text column (no DB
// enum), so 'applied' (ISSUE-044) needs no migration. Lifecycle:
// pending → rolled → applied.
const (
	pendingSaveStatusPending = "pending"
	pendingSaveStatusRolled  = "rolled"
	pendingSaveStatusApplied = "applied"
)

// ErrPendingSaveNotFound signals the pending_saves row could not be loaded; the
// handler maps it to 404.
var ErrPendingSaveNotFound = errors.New("pending save not found")

// ErrSaveWrongEncounter signals the pending_saves row belongs to a different
// encounter than the one in the request path; the handler maps it to 400.
var ErrSaveWrongEncounter = errors.New("pending save does not belong to this encounter")

// ErrSaveAlreadyResolved signals the pending_saves row is no longer "pending"
// (already rolled or forfeited); the handler maps it to 409.
var ErrSaveAlreadyResolved = errors.New("pending save is already resolved")

// ErrPlayerSaveViaDiscord signals the pending save belongs to a player
// character, whose saves are rolled in Discord via /save — not from the DM
// dashboard; the handler maps it to 409.
var ErrPlayerSaveViaDiscord = errors.New("player saving throws are rolled in Discord via /save")

// ErrSaveNotAoE signals the pending_saves row is not an AoE-cast save (e.g. a
// concentration or DM-prompted save), which this resolver does not handle.
var ErrSaveNotAoE = errors.New("pending save is not an AoE save")

// MonsterSaveResolution is the outcome of a DM-resolved monster AoE save: the
// rolled save (natural d20, bonus, cover, total) measured against the DC, plus
// the AoE damage outcome when this roll completed the cast (nil while other
// targets are still pending, or for a non-damaging AoE).
type MonsterSaveResolution struct {
	SaveID        uuid.UUID
	CombatantID   uuid.UUID
	CombatantName string
	Ability       string
	DC            int
	NaturalRoll   int
	SaveBonus     int
	CoverBonus    int
	Total         int
	Success       bool
	Damage        *AoEDamageResult
}

// selfDamage returns the damage dealt to the resolving combatant by the AoE,
// if it has landed. It deliberately surfaces only DamageDealt — never HP — so
// the combat-log line built from it cannot leak a monster's secret HP total.
func (r MonsterSaveResolution) selfDamage() (int, bool) {
	if r.Damage == nil {
		return 0, false
	}
	for _, t := range r.Damage.Targets {
		if t.CombatantID == r.CombatantID {
			return t.DamageDealt, true
		}
	}
	return 0, false
}

// ResolveMonsterPendingSave rolls and records a monster's pending AoE saving
// throw, then applies AoE damage once every target in the cast has resolved.
// A natural 1 is NOT an auto-fail on a saving throw in 5e, so autoFail is
// always false.
func (s *Service) ResolveMonsterPendingSave(ctx context.Context, encounterID, saveID uuid.UUID) (MonsterSaveResolution, error) {
	row, err := s.store.GetPendingSave(ctx, saveID)
	if err != nil {
		return MonsterSaveResolution{}, fmt.Errorf("%w: %w", ErrPendingSaveNotFound, err)
	}
	if row.EncounterID != encounterID {
		return MonsterSaveResolution{}, ErrSaveWrongEncounter
	}
	if !IsAoEPendingSaveSource(row.Source) {
		return MonsterSaveResolution{}, ErrSaveNotAoE
	}
	// ISSUE-044: 'applied' is the terminal state — damage already landed.
	if row.Status == pendingSaveStatusApplied {
		return MonsterSaveResolution{}, ErrSaveAlreadyResolved
	}

	combatant, err := s.store.GetCombatant(ctx, row.CombatantID)
	if err != nil {
		return MonsterSaveResolution{}, fmt.Errorf("getting combatant for save: %w", err)
	}
	// A monster has a CreatureRefID; a player character has only a CharacterID.
	// Player saves are rolled in Discord via /save, never from the dashboard.
	if combatant.CharacterID.Valid && !combatant.CreatureRefID.Valid {
		return MonsterSaveResolution{}, ErrPlayerSaveViaDiscord
	}

	// ISSUE-044 recovery: the save already rolled but the original bug left its
	// damage unapplied. Re-drive the apply step using the STORED roll — do NOT
	// re-roll the d20 and do NOT change the recorded success — then report the
	// stored outcome plus the resulting damage.
	if row.Status == pendingSaveStatusRolled {
		return s.recoverRolledMonsterSave(ctx, encounterID, row, combatant)
	}
	// Anything other than 'pending' here (e.g. 'forfeited') is terminal — never
	// roll a fresh save for it.
	if row.Status != pendingSaveStatusPending {
		return MonsterSaveResolution{}, ErrSaveAlreadyResolved
	}

	bonus, err := s.resolveCombatantSaveBonus(ctx, combatant, row.Ability)
	if err != nil {
		return MonsterSaveResolution{}, err
	}

	d20, err := s.roller.Roll("1d20")
	if err != nil {
		return MonsterSaveResolution{}, fmt.Errorf("rolling save d20: %w", err)
	}
	natural := d20.Total
	// RecordAoEPendingSaveRoll re-adds CoverBonus internally, so pass the bare
	// d20 + bonus (no cover) and let it own the success comparison.
	spellID, resolved, err := s.RecordAoEPendingSaveRoll(ctx, row.CombatantID, row.Ability, natural+bonus, false)
	if err != nil {
		return MonsterSaveResolution{}, err
	}

	cover := int(row.CoverBonus)
	total := natural + bonus + cover
	resolution := MonsterSaveResolution{
		SaveID:        saveID,
		CombatantID:   combatant.ID,
		CombatantName: combatant.DisplayName,
		Ability:       row.Ability,
		DC:            int(row.Dc),
		NaturalRoll:   natural,
		SaveBonus:     bonus,
		CoverBonus:    cover,
		Total:         total,
		Success:       total >= int(row.Dc),
	}

	if !resolved {
		return resolution, nil
	}
	// Drive the apply step. Returns nil when other targets are still pending
	// (multi-target cast) or the AoE is non-damaging — both are fine.
	damage, err := s.ResolveAoEPendingSaves(ctx, encounterID, spellID, s.roller)
	if err != nil {
		return MonsterSaveResolution{}, err
	}
	resolution.Damage = damage
	return resolution, nil
}

// recoverRolledMonsterSave applies the AoE damage for a save that already
// rolled but never landed (the ISSUE-044 bug). It re-drives ResolveAoEPendingSaves
// using the existing stored roll — no fresh d20, no change to the recorded
// success — and reports the stored roll total/DC/success alongside the damage.
// NaturalRoll and SaveBonus are left zero because only the combined total is
// persisted (RecordAoEPendingSaveRoll stored natural+bonus+cover in roll_result).
func (s *Service) recoverRolledMonsterSave(ctx context.Context, encounterID uuid.UUID, row refdata.PendingSafe, combatant refdata.Combatant) (MonsterSaveResolution, error) {
	damage, err := s.ResolveAoEPendingSaves(ctx, encounterID, SpellIDFromAoEPendingSaveSource(row.Source), s.roller)
	if err != nil {
		return MonsterSaveResolution{}, err
	}
	total := 0
	if row.RollResult.Valid {
		total = int(row.RollResult.Int32)
	}
	return MonsterSaveResolution{
		SaveID:        row.ID,
		CombatantID:   combatant.ID,
		CombatantName: combatant.DisplayName,
		Ability:       row.Ability,
		DC:            int(row.Dc),
		CoverBonus:    int(row.CoverBonus),
		Total:         total, // stored roll already includes the cover bonus
		Success:       row.Success.Valid && row.Success.Bool,
		Damage:        damage,
	}, nil
}

// resolveCombatantSaveBonus resolves the given ability's saving-throw bonus for
// a combatant. For a creature it prefers an explicit saving_throws.<ability>
// entry, else falls back to the ability-score modifier; for a PC it uses the
// character's ability-score modifier. An NPC without a creature ref defaults
// to +0.
func (s *Service) resolveCombatantSaveBonus(ctx context.Context, target refdata.Combatant, ability string) (int, error) {
	if target.CreatureRefID.Valid && target.CreatureRefID.String != "" {
		creature, err := s.store.GetCreature(ctx, target.CreatureRefID.String)
		if err != nil {
			return 0, fmt.Errorf("getting creature for save: %w", err)
		}
		if bonus, ok := creatureSaveBonus(creature, ability); ok {
			return bonus, nil
		}
		scores, err := ParseAbilityScores(creature.AbilityScores)
		if err != nil {
			return 0, fmt.Errorf("parsing creature ability scores: %w", err)
		}
		return AbilityModifier(scores.ScoreByName(ability)), nil
	}

	// PC target
	if target.CharacterID.Valid {
		char, err := s.store.GetCharacter(ctx, target.CharacterID.UUID)
		if err != nil {
			return 0, fmt.Errorf("getting target character: %w", err)
		}
		scores, err := ParseAbilityScores(char.AbilityScores)
		if err != nil {
			return 0, fmt.Errorf("parsing target ability scores: %w", err)
		}
		return AbilityModifier(scores.ScoreByName(ability)), nil
	}

	// NPC without creature ref — default to +0
	return 0, nil
}

// creatureSaveBonus reads an explicit saving_throws.<ability> bonus from a
// creature's saving_throws JSON. The bool is false when there is no JSON or no
// entry for the ability, signalling the caller to fall back to the ability mod.
func creatureSaveBonus(creature refdata.Creature, ability string) (int, bool) {
	if !creature.SavingThrows.Valid || len(creature.SavingThrows.RawMessage) == 0 {
		return 0, false
	}
	var saves map[string]int
	if err := json.Unmarshal(creature.SavingThrows.RawMessage, &saves); err != nil {
		return 0, false
	}
	bonus, ok := saves[ability]
	return bonus, ok
}
