# task crit-03 — Damage pipeline bypasses Phase 42 (R/I/V, temp HP, exhaustion)

## Finding (verbatim from chunk4_conditions_classes.md, Phase 42)

> `damage.go:13 ApplyDamageResistances` correctly implements I trumps all → R+V cancel → R alone → V alone → petrified-as-resistance. **Unused outside tests.**
> `damage.go:49 AbsorbTempHP`, `:57 GrantTempHP` correct semantics. **Unused outside tests.**
> Exhaustion ladder: `damage.go:66 ExhaustionEffectiveSpeed`, `:78 ExhaustionEffectiveMaxHP` (level 4+ halves), `:99 IsExhaustedToDeath` (6 = death). Only the speed-and-saves slice is wired. Max-HP halving (level 4+) and death (level 6) are never applied — no caller.
> Net: the R/V/temp-HP/HP-halving slices of Phase 42 are essentially dead code in production. Any damage applied via `applyDamageHP` (`internal/combat/concentration.go:281`) skips these helpers entirely.

> Cross-cutting risk: every damage path (`internal/combat/aoe.go:554`, `channel_divinity.go:269`, `concentration.go:281`, `dm_dashboard_undo.go:189,369`, `dm_dashboard_handler.go:338`, `turn_builder_handler.go:291`) writes raw damage directly to `hp_current` via `applyDamageHP`. None route through `ApplyDamageResistances` or `AbsorbTempHP`.

Spec section: "Damage Processing" (Phase 42 in `docs/phases.md`); coverage map line 865 maps to Phase 42.

Recommended approach (chunk4 follow-up #2): "Route every damage write through a new `ApplyDamage(...)` helper that calls `ApplyDamageResistances` → `AbsorbTempHP` before `applyDamageHP`. Replace each direct `applyDamageHP` call site (8 production locations) with the new helper, supplying the target's resistances/immunities/vulnerabilities/conditions. Add exhaustion HP-halving and level-6 death."

## Plan

1. Add `Service.ApplyDamage(ctx, ApplyDamageInput) ApplyDamageResult` wrapper in `internal/combat/damage.go`. Wrapper resolves R/I/V (creature lookup for NPCs, empty for PCs), parses conditions, runs:
   - exhaustion-6 instant-death short-circuit;
   - exhaustion 4+ HP-halving cap on currentHP;
   - `ApplyDamageResistances` → `AbsorbTempHP` → newHP math;
   - PC-vs-NPC isAlive resolution;
   - then funnels through existing `applyDamageHP` so Phase 118 concentration / unconscious-at-0 hooks still fire.
2. Add an `Override bool` flag for DM-dashboard / undo paths that need raw HP delta semantics (no R/I/V, no temp HP soak).
3. Replace each direct `applyDamageHP` call site (6 production locations) with `ApplyDamage`. Per-attack damage type plumbed where available; `Override=true` for the destroy-undead, undo, and manual override paths.
4. Add red-then-green TDD tests covering: NPC R/I/V (resistance halves, immunity zeroes, vulnerability doubles), petrified-condition resistance, temp-HP absorption (full-soak + partial spill), R/I/V-then-temp-HP ordering, exhaustion 4 HP-halving, exhaustion 6 instant death, PC dying-vs-NPC dying, Override skipping R/I/V + temp HP, PC no-creature-ref skip, concentration-save still fires through wrapper. Plus AoE-integration tests proving an NPC with creature R/I/V actually halves Fireball / immune-to-poison no-ops.

## Files touched

- `internal/combat/damage.go` — new `ApplyDamageInput` / `ApplyDamageResult` types, new `Service.ApplyDamage`, new `resolveDamageProfile` helper.
- `internal/combat/aoe.go` — `ResolveAoESaves` damage write now routes through `Service.ApplyDamage` with `input.DamageType`.
- `internal/combat/channel_divinity.go` — Destroy Undead routes through `ApplyDamage` with `Override=true` (destruction effect ignores damage typing).
- `internal/combat/dm_dashboard_undo.go` — undo-of-heal and `OverrideCombatantHP` HP-decrease paths route through `ApplyDamage` with `Override=true`, then issue a follow-up `UpdateCombatantHP` so the explicit tempHP / isAlive snapshot is honored exactly.
- `internal/combat/dm_dashboard_handler.go` — `applyDamageEffect` (DM dashboard pending-action damage effect) routes through `ApplyDamage` (no Override — temp HP is now absorbed by the wrapper instead of pre-deducted).
- `internal/combat/turn_builder_handler.go` — `ExecuteEnemyTurn` queues hits per-attack (instead of aggregating to a damage map) so per-attack `damage_type` flows into `ApplyDamage`. Result map still aggregated for the response.
- `internal/combat/damage_test.go` — new ApplyDamage wrapper tests.
- `internal/combat/aoe_test.go` — new NPC-resistance and NPC-immunity AoE integration tests.

## Tests added

In `internal/combat/damage_test.go`:
- `TestApplyDamage_NPCResistanceHalvesIncomingDamage`
- `TestApplyDamage_NPCImmunityZeroes`
- `TestApplyDamage_NPCVulnerabilityDoubles`
- `TestApplyDamage_TempHPAbsorbsBeforeCurrent`
- `TestApplyDamage_TempHPPartialSpillsIntoHP`
- `TestApplyDamage_ResistanceAppliedBeforeTempHP`
- `TestApplyDamage_ExhaustionLevel4HalvesMaxHP`
- `TestApplyDamage_ExhaustionLevel6KillsImmediately`
- `TestApplyDamage_PCAtZeroStaysAliveDying`
- `TestApplyDamage_NPCDiesAtZero`
- `TestApplyDamage_OverrideSkipsRIVAndTempHP`
- `TestApplyDamage_PCNoCreatureRefHasNoRIV` (asserts `getCreature` is NEVER called for PCs)
- `TestApplyDamage_PetrifiedConditionGrantsResistance`
- `TestApplyDamage_NegativeDamageRejected`
- `TestApplyDamage_InvalidConditionsJSONErrors`
- `TestApplyDamage_NPCCreatureLookupErrorPropagates`
- `TestApplyDamage_NPCMissingCreatureRowSkipsRIV`
- `TestApplyDamage_FiresConcentrationSave` (integration — proves Phase 118 hooks still fire through the wrapper)

In `internal/combat/aoe_test.go`:
- `TestResolveAoESaves_NPCResistanceHalvesAoEDamage` (proves Fireball vs fire-resistant NPC gets halved end-to-end)
- `TestResolveAoESaves_NPCImmunityZeroesAoEDamage` (proves poison-immune skeleton takes 0 from a poison AoE)

## Implementation notes

- The wrapper sits ABOVE `applyDamageHP`; `applyDamageHP` is intentionally left in place because the Phase 118 concentration-save / unconscious-at-0-HP hooks live there and apply to every damage event regardless of typing.
- For PCs, `resolveDamageProfile` returns empty R/I/V slices (the PC schema does not yet carry per-character damage modifiers — Tiefling fire resistance, Heavy Armor Master, etc. plumb through here once a per-character resistance accumulator is added). Once that's in place, only `resolveDamageProfile` needs to be extended; `ApplyDamage` and all call sites are agnostic.
- Exhaustion-4 HP-halving applies a *cap on currentHP at apply time* (not a max-HP rewrite) — the schema doesn't carry an "effective max HP" column, and the rule is interesting only when damage is being applied. Cap is gated on `ExhaustionLevel >= 4` so test fixtures with `HpMax = 0` aren't accidentally clamped.
- Exhaustion-6 short-circuits to `applyDamageHP(currentHP, 0, 0, isAlive=false)`. NPC dies, PC dies (rule: exhaustion 6 = death, no save). Concentration / unconscious hooks still fire via applyDamageHP for the (rare) edge case of a PC concentrating at the moment they hit exhaustion 6.
- `Override=true` is the documented escape hatch for DM-dashboard manual HP adjustments and the undo/redo paths where the caller is asserting an *exact prior or target HP*. R/I/V + temp HP must NOT apply there. The two undo/override paths additionally issue a follow-up `UpdateCombatantHP` to set the explicit `temp_hp` / `is_alive` snapshot fields that the wrapper otherwise computes.
- `turn_builder_handler.ExecuteEnemyTurn` was previously aggregating multi-attack damage by target-id into a single `damageApplied` map, then issuing one HP write per target. That collapsed mixed damage types (slashing + fire from a flame-tongue dual-wielder, e.g.) into one untyped hit. The fix queues per-attack `pendingHit{targetID, amount, damageType}` records and applies each through `ApplyDamage`; the aggregated `damageApplied` summary is rebuilt from the post-R/I/V results so the existing JSON response shape is preserved.
- Coverage on `internal/combat` rose to 94.1%; new `ApplyDamage` is at 97.2%, `resolveDamageProfile` at 100%.
- Out-of-scope follow-ups (logged in `.fix-state/log.md` for the orchestrator):
  - Heavy Armor Master, Tiefling-fire-resistance, etc. require a per-character R/I/V accumulator — `ApplyDamage` is ready, the data plumbing isn't.
  - Crit-01a's in-flight `attack_fes_test.go` + `attack.go` mods leave the combat package's *test* build broken (undefined `nearestAllyDistanceFt`, `attackAbilityUsed`, `fesDamageDice`). Production build is clean and my own tests are verified by stash-and-restore. Not a regression from crit-03.

## Review (reviewer fills) — Verdict: PASS | REVISIT

STATUS: READY_FOR_REVIEW

## Review

Verdict: PASS

- All six production caller sites (`aoe.go:543`, `channel_divinity.go:269`, `dm_dashboard_handler.go:338`, `dm_dashboard_undo.go:189,369`, `turn_builder_handler.go:291`) now route through `Service.ApplyDamage`. `grep -n applyDamageHP internal/combat/*.go` confirms only `damage.go` (the wrapper, lines 170 and 222) and `concentration.go` (the helper definition + tests) reference `applyDamageHP` directly. The `concentration.go:281` call site referenced in the original finding was the wrapper itself; it remains as the underlying seam.
- All 18 named `TestApplyDamage_*` cases plus the two `TestResolveAoESaves_NPC*` cases are present and exercise R/I/V resolution (resistance/immunity/vulnerability/petrified), temp-HP absorb (full + partial spill), R/I/V-then-temp-HP ordering, exhaustion-4 HP-halving, exhaustion-6 instant death, PC-vs-NPC dying semantics, Override skipping R/I/V + temp HP, PC-no-creature-ref short-circuit (with `t.Fatalf` guard on `getCreature`), error propagation, and missing-creature-row fallback.
- `TestApplyDamage_FiresConcentrationSave` (damage_test.go:922) verifies Phase 118 concentration save still fires through the wrapper. `applyDamageHP` (concentration.go:281) is preserved beneath the wrapper — the unconscious-at-0 hook (concentration.go:304-312) is untouched.
- `Override=true` correctly bypasses R/I/V and temp HP and uses the raw delta. Both DM-dashboard `OverrideCombatantHP` and undo-of-heal paths additionally issue a follow-up `UpdateCombatantHP` to write the explicit tempHp/isAlive snapshot. The follow-up does not touch conditions, so any unconscious condition applied by the wrapper's underlying `applyDamageHP` is preserved. The follow-up correctly honors the snapshot's `tempHp` (which Override leaves untouched at the pre-undo value) and the snapshot's `isAlive` (which may differ from `hpCurrent>0||!IsNpc` for stabilize-undo cases).
- `turn_builder_handler.ExecuteEnemyTurn` queues `pendingHit{targetID, amount, damageType}` per attack and applies each through `ApplyDamage`, preserving per-attack damage type. The aggregated `damageApplied` map is rebuilt from post-R/I/V `dmgRes.FinalDamage` for the response shape.
- Scope is well-bounded: PC-side R/I/V is correctly deferred — `resolveDamageProfile` returns empty slices for PCs and `t.Fatalf`'s if `getCreature` is invoked for a PC, locking in the contract.
- Coverage: `go test -cover ./internal/combat/...` reports 94.1% (>= 85% threshold). Combat tests pass (`go test ./internal/combat/...` is green); the worker's note about crit-01a's in-flight test build state does not affect this package today.
