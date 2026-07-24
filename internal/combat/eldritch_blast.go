package combat

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
)

// BeamOutcome captures the independent resolution of one Eldritch Blast beam.
// Eldritch Blast fires 1/2/3/4 beams at character levels 1/5/11/17; each beam is
// a separate ranged spell attack that may target a different creature, so each
// carries its own attack roll and damage (COV-14).
type BeamOutcome struct {
	Index       int       // 1-based beam number
	TargetID    uuid.UUID // this beam's target combatant (credits its defender in stats)
	TargetName  string    // display name of this beam's target
	AttackRoll  int       // chosen d20 face (before modifiers)
	AttackTotal int       // face + spell attack modifier
	Hit         bool
	Crit        bool // chosen d20 face was a natural 20
	Damage      int  // damage applied by this beam (0 on a miss)
}

// eldritchBlastResult bundles the aggregate outcome of resolving every beam so
// the caller can fold it into the CastResult in one place.
type eldritchBlastResult struct {
	Outcomes    []BeamOutcome
	DamageTotal int
	Breakdown   []DamageComponent
	DownLine    string
	Pushed      bool
}

// IsEldritchBlast reports whether the spell is Eldritch Blast — the only cantrip
// that fires multiple independent attack beams (other scaling cantrips add dice
// to a single attack roll against one target).
func IsEldritchBlast(spell refdata.Spell) bool {
	return spell.ID == eldritchBlastSpellID
}

// anyBeamHit reports whether at least one beam connected.
func anyBeamHit(beams []BeamOutcome) bool {
	for _, b := range beams {
		if b.Hit {
			return true
		}
	}
	return false
}

// beamAccumulator gathers every beam that landed on a single target so the
// target's HP is written once (multiple beams on one creature must not each
// re-read a stale snapshot and clobber the earlier hit's HP).
type beamAccumulator struct {
	comb        refdata.Combatant
	rawTotal    int
	agonizing   int
	hex         int
	huntersMark int
	repelHits   int
}

// resolveEldritchBlastBeams fires each Eldritch Blast beam one by one: every
// beam is an independent ranged spell attack (its own d20 roll) against its own
// target, dealing its own die of force damage plus the per-beam riders —
// Agonizing Blast (spellcasting modifier per beam), Hex, and Hunter's Mark. Beam
// i targets cmd.BeamTargetIDs[i] when supplied, otherwise the primary target;
// beams beyond the supplied list fall back to the first target. Damage is summed
// per distinct target and applied once, and Repelling Blast pushes each struck
// target 10 ft per beam that hit it. COV-14.
func (s *Service) resolveEldritchBlastBeams(
	ctx context.Context,
	cmd CastCommand,
	spell refdata.Spell,
	char refdata.Character,
	caster refdata.Combatant,
	primaryTarget refdata.Combatant,
	spellAbilityScore int,
	attackMod int,
	baseDamageDice string,
	damageType string,
	roller *dice.Roller,
) (eldritchBlastResult, error) {
	beamCount := CantripDiceMultiplier(int(char.Level))

	targetIDs := cmd.BeamTargetIDs
	if len(targetIDs) == 0 {
		targetIDs = []uuid.UUID{cmd.TargetID}
	}

	// Resolve each beam target once, validating range + line-of-sight for any
	// target other than the already-validated primary. The cache means a repeated
	// target is fetched and validated a single time.
	combByID := make(map[uuid.UUID]refdata.Combatant)
	resolve := func(id uuid.UUID) (refdata.Combatant, error) {
		if c, ok := combByID[id]; ok {
			return c, nil
		}
		if id == primaryTarget.ID {
			combByID[id] = primaryTarget
			return primaryTarget, nil
		}
		c, err := s.store.GetCombatant(ctx, id)
		if err != nil {
			return refdata.Combatant{}, fmt.Errorf("getting beam target: %w", err)
		}
		distFt := combatantDistance(caster, c)
		if err := ValidateSpellRange(applyEldritchSpearRange(spell, char), distFt); err != nil {
			return refdata.Combatant{}, fmt.Errorf("beam target out of range: %w", err)
		}
		if err := ValidateSeeTarget(spell, c); err != nil {
			return refdata.Combatant{}, err
		}
		combByID[id] = c
		return c, nil
	}

	repelling := castTriggersRepellingBlast(spell, char)
	hasAgonizing := HasInvocation(char.Features, agonizingBlastEffectID)
	agonizingPerBeam := AbilityModifier(spellAbilityScore)
	dmgDice := strings.ReplaceAll(baseDamageDice, "+mod", fmt.Sprintf("+%d", AbilityModifier(spellAbilityScore)))

	accs := make(map[uuid.UUID]*beamAccumulator)
	var order []uuid.UUID
	outcomes := make([]BeamOutcome, 0, beamCount)

	for j := 0; j < beamCount; j++ {
		idx := j
		if idx >= len(targetIDs) {
			idx = 0
		}
		tid := targetIDs[idx]
		tc, err := resolve(tid)
		if err != nil {
			return eldritchBlastResult{}, err
		}

		d20Result, err := roller.RollD20(attackMod, cmd.SpellAttackRollMode)
		if err != nil {
			return eldritchBlastResult{}, fmt.Errorf("rolling beam attack: %w", err)
		}
		beam := BeamOutcome{
			Index:       j + 1,
			TargetID:    tid,
			TargetName:  tc.DisplayName,
			AttackRoll:  d20Result.Chosen,
			AttackTotal: d20Result.Total,
			Hit:         d20Result.Total >= int(tc.Ac),
			Crit:        d20Result.CriticalHit,
		}

		if beam.Hit {
			dmgRoll, err := roller.Roll(dmgDice)
			if err != nil {
				return eldritchBlastResult{}, fmt.Errorf("rolling beam damage: %w", err)
			}
			beamDmg := dmgRoll.Total

			a := accs[tid]
			if a == nil {
				a = &beamAccumulator{comb: tc}
				accs[tid] = a
				order = append(order, tid)
			}

			// Agonizing Blast adds the spellcasting modifier to EACH beam.
			if hasAgonizing && agonizingPerBeam != 0 {
				beamDmg += agonizingPerBeam
				a.agonizing += agonizingPerBeam
			}
			if targetHexedBy(tc.Conditions, caster.ID) {
				hexRoll, err := roller.Roll("1d6")
				if err != nil {
					return eldritchBlastResult{}, fmt.Errorf("rolling hex damage: %w", err)
				}
				beamDmg += hexRoll.Total
				a.hex += hexRoll.Total
			}
			if targetHuntersMarkedBy(tc.Conditions, caster.ID) {
				hmRoll, err := roller.Roll("1d6")
				if err != nil {
					return eldritchBlastResult{}, fmt.Errorf("rolling hunter's mark damage: %w", err)
				}
				beamDmg += hmRoll.Total
				a.huntersMark += hmRoll.Total
			}
			if repelling {
				a.repelHits++
			}
			a.rawTotal += beamDmg
			beam.Damage = beamDmg
		}
		outcomes = append(outcomes, beam)
	}

	var (
		downLines   []string
		damageTotal int
		pushed      bool
		agTotal     int
		hexTotal    int
		hmTotal     int
	)
	for _, tid := range order {
		a := accs[tid]
		if a.rawTotal > 0 {
			dmgOut, err := s.ApplyDamage(ctx, ApplyDamageInput{
				EncounterID:  a.comb.EncounterID,
				Target:       a.comb,
				RawDamage:    a.rawTotal,
				DamageType:   damageType,
				DeferDownLog: true,
			})
			if err != nil {
				return eldritchBlastResult{}, fmt.Errorf("applying beam damage: %w", err)
			}
			damageTotal += a.rawTotal
			if dmgOut.DownLogLine != "" {
				downLines = append(downLines, dmgOut.DownLogLine)
			}
		}
		if repelling && a.repelHits > 0 {
			// Each beam that hit pushes 10 ft (2 squares) straight away.
			if err := s.applyPushEffect(ctx, caster, a.comb, 2*a.repelHits); err != nil {
				return eldritchBlastResult{}, fmt.Errorf("applying repelling blast push: %w", err)
			}
			pushed = true
		}
		agTotal += a.agonizing
		hexTotal += a.hex
		hmTotal += a.huntersMark
	}

	var breakdown []DamageComponent
	if agTotal != 0 {
		breakdown = append(breakdown, DamageComponent{SourceName: "Agonizing Blast", Amount: agTotal, DamageType: damageType})
	}
	if hexTotal != 0 {
		breakdown = append(breakdown, DamageComponent{SourceName: "Hex", Amount: hexTotal, DamageType: "necrotic"})
	}
	if hmTotal != 0 {
		breakdown = append(breakdown, DamageComponent{SourceName: "Hunter's Mark", Amount: hmTotal, DamageType: "force"})
	}

	return eldritchBlastResult{
		Outcomes:    outcomes,
		DamageTotal: damageTotal,
		Breakdown:   breakdown,
		DownLine:    strings.Join(downLines, "\n"),
		Pushed:      pushed,
	}, nil
}
