# D-PROMPTS implementation worklog (batch 2)

## Status per task

### D-46-rage-spellcasting-block — DONE
- `internal/combat/spellcasting.go` Cast(): early-return error
  "you cannot cast spells while raging" when `caster.IsRaging` (before slot
  deduction, before turn resource consumption).
- `internal/combat/rage.go` ActivateRage(): after persisting rage,
  best-effort `breakStoredConcentration(ctx, ragedCombatant, "raging")` to
  drop any active concentration spell (full BreakConcentrationFully —
  clears columns, strips spell-sourced conditions, dismisses summons).
- Tests: `TestService_Cast_BlockedWhileRaging`,
  `TestService_ActivateRage_DropsConcentration`.

### D-46-rage-auto-end-quiet — DONE
- `internal/combat/rage.go` new helpers `markRageAttacked` and
  `markRageTookDamage` persist the per-round tracking flags.
- `Attack()`, `attackImprovised()`, `OffhandAttack()`, and
  `MartialArtsBonusAttack()` now call `markRageAttacked` after
  `resolveAndPersistAttack`.
- `ApplyDamage()` calls `markRageTookDamage` when adjusted damage > 0
  (immunity / temp-HP absorption don't count).
- Tests: `TestService_Attack_SetsRageAttackedThisRound`,
  `TestService_Attack_NonRagingDoesNotMarkRage`,
  `TestService_ApplyDamage_SetsRageTookDamageThisRound`.

### D-46-rage-end-on-unconscious — DONE
- `internal/combat/rage.go` new helper `maybeEndRageOnUnconscious` runs
  `ShouldRageEndOnUnconscious` + `ClearRageFromCombatant` + persist.
- `ApplyDamage()` invokes it after `applyDamageHP` (so the dying-condition
  bundle has already been applied).
- Carries the input target's IsRaging/RageRounds/RageAttackedThisRound/
  RageTookDamageThisRound forward because `UpdateCombatantHP` only writes
  HP / temp_hp / is_alive.
- Test: `TestService_ApplyDamage_RageEndsOnUnconscious`.

### D-48a-unarmored-movement-consumer — DONE
- `internal/combat/turnresources.go` `ResolveTurnResources()` now folds
  `ProcessEffects(TriggerOnTurnStart, EffectContext{WearingArmor: ...})`
  SpeedModifier into the base speed BEFORE exhaustion/condition halving.
- New helper `turnStartSpeedBonus(char)` builds FeatureDefinitions from
  the character's classes/features and returns the SpeedModifier total.
- Tests: `TestService_ResolveTurnResources_UnarmoredMonkGetsSpeedBonus`
  (L6 monk: 30+15=45), `TestService_ResolveTurnResources_ArmoredMonkNoSpeedBonus`.

### D-48b-stunning-strike-prompt — DONE (service-side)
- `internal/combat/attack.go` `AttackResult` gains
  `PromptStunningStrikeEligible` + `PromptStunningStrikeKiAvailable`.
- `internal/combat/class_feature_prompt.go` new helper
  `populatePostHitPrompts` sets the flags when attacker is a Monk, the hit
  is melee, and ki > 0.
- Wired into `Service.Attack`, `attackImprovised`, `OffhandAttack`, and
  `MartialArtsBonusAttack`.
- **Follow-up filed**: `.fix-state/tasks/D-48b-49-51-followup-discord-prompts.md`
  for the Discord-side consumer.
- Tests: `TestService_Attack_MonkHit_SurfacesStunningStrikePrompt`,
  `TestService_Attack_MonkOutOfKi_NoStunningStrikePrompt`.

### D-49-bardic-inspiration-prompt — DONE (service-side)
- `AttackResult` gains `PromptBardicInspirationEligible` +
  `PromptBardicInspirationDie`. Populated by `populatePostHitPrompts`
  when `CombatantHasBardicInspiration(attacker)` is true — independent of
  hit/miss because the holder can spend the die regardless.
- Follow-up filed (same file as D-48b).
- Test: `TestService_Attack_HolderHasBardicInspiration_SurfacesPrompt`.

### D-51-divine-smite-prompt — DONE (service-side)
- `AttackResult` gains `PromptDivineSmiteEligible` +
  `PromptDivineSmiteSlots` (sorted ascending). Populated when the attacker
  is a paladin with the Divine Smite feature, the hit was melee, and at
  least one spell slot is available.
- Follow-up filed (same file as D-48b).
- Tests: `TestService_Attack_PaladinHit_SurfacesDivineSmitePrompt`,
  `TestService_Attack_PaladinMiss_NoDivineSmitePrompt`.

## Files touched
- `internal/combat/attack.go` (AttackResult fields + post-hit calls)
- `internal/combat/class_feature_prompt.go` (NEW)
- `internal/combat/damage.go` (rage-took-damage + end-on-unconscious hooks)
- `internal/combat/d_prompts_test.go` (NEW — all 7 tasks' tests)
- `internal/combat/monk.go` (MartialArtsBonusAttack post-hit wiring)
- `internal/combat/rage.go` (markRageAttacked/TookDamage/maybeEndOnUnconscious + drop concentration on activate)
- `internal/combat/spellcasting.go` (Cast IsRaging guard)
- `internal/combat/turnresources.go` (FES TriggerOnTurnStart consumer)

## Validation
- `go test ./internal/combat/` — pass
- `make test` — no FAILs
- `make cover-check` — combat 93.1% (threshold 85%)
- `make build` — clean

## Out-of-zone follow-up
- `.fix-state/tasks/D-48b-49-51-followup-discord-prompts.md` —
  Discord-side `attack_handler.go` must read the new AttackResult fields
  and invoke `ClassFeaturePromptPoster.PromptStunningStrike/DivineSmite/
  BardicInspiration` accordingly. Service-side eligibility data is now
  available; only the UI dispatch remains.
