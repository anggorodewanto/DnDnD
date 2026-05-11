# D-PROMPTS review worklog (batch 2)

Reviewer: independent re-read. Working tree HEAD = 3cb4189. Read-only.

## Per-task verdict

| Task | Verdict | Verification |
|---|---|---|
| D-46 rage spellcasting block | PASS | `spellcasting.go:367` `if caster.IsRaging { return ... "you cannot cast spells while raging" }` is BEFORE slot/turn deduction. `rage.go:281` `ActivateRage` calls `breakStoredConcentration(ctx, ragedCombatant, "raging")` after persist. Tests `TestService_Cast_BlockedWhileRaging`, `TestService_ActivateRage_DropsConcentration`. |
| D-46 rage auto-end quiet | PASS | `rage.go` adds `markRageAttacked` (`IsRaging` guard + `persistRageState`) and `markRageTookDamage`. Wired in `Attack`, `attackImprovised`, `OffhandAttack`, `MartialArtsBonusAttack` after `resolveAndPersistAttack`; in `damage.go:280` after `applyDamageHP` gated by `adjusted > 0`. Tests cover raging-set, non-raging-skip, damage-set. |
| D-46 rage end on unconscious | PASS | `rage.go:323` `maybeEndRageOnUnconscious` runs `ShouldRageEndOnUnconscious` -> `ClearRageFromCombatant` -> `persistRageState`. Invoked in `damage.go:283` after HP persist. Carry-over of `IsRaging`/`RageRoundsRemaining`/per-round flags onto `postHPCombatant` is correct because `UpdateCombatantHP` only writes hp/temp_hp/is_alive. Test `TestService_ApplyDamage_RageEndsOnUnconscious`. |
| D-48a Unarmored Movement consumer | PASS | `turnresources.go:238-264` folds `ProcessEffects(TriggerOnTurnStart, EffectContext{WearingArmor: ...})` `SpeedModifier` into base speed BEFORE `EffectiveSpeedWithExhaustion`. Tests L6 monk 30+15=45 unarmored; armored monk = 30. |
| D-48b Stunning Strike prompt | PASS (service) | `AttackResult.PromptStunningStrikeEligible/KiAvailable` populated by `populatePostHitPrompts` gated on `Hit && IsMelee && Monk level>0 && ki>0`. Wired in all 4 attack paths. Tests hit/out-of-ki. Discord consumer = filed follow-up. |
| D-49 Bardic Inspiration prompt | PASS (service) | `AttackResult.PromptBardicInspirationEligible/Die` set when `CombatantHasBardicInspiration(attacker)`. Independent of hit/miss — matches "die can be spent regardless." Test `TestService_Attack_HolderHasBardicInspiration_SurfacesPrompt`. |
| D-51 Divine Smite prompt | PASS (service) | `AttackResult.PromptDivineSmiteEligible/Slots` set on melee-hit + `HasFeatureByName("Divine Smite")` + `len(AvailableSmiteSlots) > 0`. Tests paladin-hit / paladin-miss. |

## Findings

- Red-before-green: tests assert exact behavior the wiring introduces; mentally removing any call (`markRageAttacked`, `populatePostHitPrompts`, `breakStoredConcentration`, FES `turnStartSpeedBonus`) makes the corresponding test fail.
- Batch-1 work intact: cover in `attack.go` (`Cover CoverLevel`, `effectiveAC := EffectiveAC(input.TargetAC, input.Cover)`), Phase-43 death-save routing in `damage.go:246`, `breakStoredConcentration` in `concentration.go:454`, retire flow untouched.
- Best-effort error swallowing in `markRageAttacked`/`markRageTookDamage`/`maybeEndRageOnUnconscious`/`ActivateRage` post-rage concentration drop is correct — never blocks the primary action.
- Discord consumer for the three Prompt*Eligible flags is correctly filed as `D-48b-49-51-followup-discord-prompts.md` (parent links present). Service-level closure is correct.

## Verification

- `make build` clean
- `make test` no FAIL lines (only cached PASS)
- `make cover-check` PASS — combat 93.0%, discord 86.7% (both above 85% per-pkg threshold)

## Next steps

1. Mark all 7 tasks `status: done` in `.fix-state/TASKS.md`.
2. Implement filed Discord follow-up (`D-48b-49-51-followup-discord-prompts.md`) to wire `attack_handler.go` to read the new `AttackResult.Prompt*` fields and dispatch the corresponding `ClassFeaturePromptPoster` methods.

VERDICT: APPROVE — all 7 tasks meet service-level acceptance criteria.
