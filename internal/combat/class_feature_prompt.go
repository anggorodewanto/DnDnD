package combat

import (
	"context"

	"github.com/ab/dndnd/internal/refdata"
)

// populatePostHitPrompts fills in the post-hit class-feature prompt
// eligibility hints on an AttackResult after a melee hit lands. It is
// purely data-shaping — the Discord layer reads these fields and decides
// whether to post the corresponding reaction prompt.
//
// Wired by:
//   - D-46 (rage spell block / drop concentration are NOT post-hit — handled
//     in Cast/ActivateRage directly)
//   - D-48b Stunning Strike — monk melee hit, ki available
//   - D-49 Bardic Inspiration — holder makes an attack with an un-expired die
//   - D-51 Divine Smite — paladin melee hit, at least one slot available
//
// Best-effort: lookup or parse errors leave the prompt fields false / empty
// so a transient store failure never breaks the attack pipeline.
//
// ctx is reserved for future store lookups (e.g. cross-encounter inspiration
// checks); it is unused today.
func (s *Service) populatePostHitPrompts(_ context.Context, result *AttackResult, attacker refdata.Combatant, char *refdata.Character) {
	// Bardic Inspiration is independent of hit/miss — the holder can spend
	// the die on the attack roll regardless of outcome. The Discord layer
	// is expected to surface the prompt BEFORE applying damage so the player
	// can also use the die on a saving throw later (this helper marks
	// eligibility either way).
	if CombatantHasBardicInspiration(attacker) {
		result.PromptBardicInspirationEligible = true
		result.PromptBardicInspirationDie = attacker.BardicInspirationDie.String
	}

	if !result.Hit {
		return
	}

	if char == nil {
		return
	}

	// Stunning Strike — monk + melee hit + ki available.
	if result.IsMelee {
		if monkLevel := ClassLevelFromJSON(char.Classes, "Monk"); monkLevel > 0 {
			if ki := remainingKi(*char); ki > 0 {
				result.PromptStunningStrikeEligible = true
				result.PromptStunningStrikeKiAvailable = ki
			}
		}
	}

	// Divine Smite — paladin + melee hit + at least one slot available.
	// Gated on the character actually having the Divine Smite feature so a
	// fresh paladin (level <2) doesn't get the prompt before they unlock it.
	if result.IsMelee && HasFeatureByName(char.Features.RawMessage, "Divine Smite") {
		slots, err := ParseSpellSlots(char.SpellSlots.RawMessage)
		if err == nil {
			available := AvailableSmiteSlots(slots)
			if len(available) > 0 {
				result.PromptDivineSmiteEligible = true
				result.PromptDivineSmiteSlots = available
			}
		}
	}
}

// remainingKi returns the current ki points on a character, or 0 when
// feature_uses is unset / unparseable / lacks a "ki" entry.
func remainingKi(char refdata.Character) int {
	_, ki, err := ParseFeatureUses(char, FeatureKeyKi)
	if err != nil {
		return 0
	}
	return ki
}
