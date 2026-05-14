# Batch 05: Pathfinding, movement, attack (Phases 29–38)

## Summary

All ten phases are implemented and well-tested (~6.5k lines of unit + integration tests across the touched files). Pathfinding, movement, altitude, distance, cover, and the bulk of the attack stack land on-spec. Several minor divergences and one functional bug were identified:

- The "Reckless Attack must be declared on the first attack" gate (attack.go:870) uses `cmd.Turn.ActionUsed`, but `Service.Attack` never sets `ActionUsed=true` — only `AttacksRemaining` is decremented. The gate therefore never trips, so Reckless can be re-declared on every swing. No test exercises the negative path.
- `/attack --offhand` is implemented as a flag on `/attack` rather than the spec's `/bonus offhand` subcommand (functionally equivalent but a UX divergence).
- `/fly` does not surface the spec's "fall damage on lose-fly-speed"; only the prone-while-airborne hook (`applyFallDamageOnProne` invoked from `ApplyCondition`) is wired. Voluntary `/fly 0` correctly costs 1 ft per ft (no damage), which matches the spec.

Coverage of the spec's mechanical surface is otherwise comprehensive: A* costs, OAs on `/move`, multi-class max-of `attacks_per_action`, 3D distance, DMG-corner cover with creature-granted half cover, advantage cancellation, GWM/Sharpshooter/Reckless validation, loading-weapon Crossbow Expert override, thrown weapon hand-clearing, half-ammo recovery on `EndCombat`.

## Per-phase findings

### Phase 29 — Pathfinding (A*)

- Status: **matches**
- Key files:
  - /home/ab/projects/DnDnD/internal/pathfinding/pathfinding.go
  - /home/ab/projects/DnDnD/internal/pathfinding/pathfinding_test.go (535 lines)
- Findings:
  - A* with binary-heap priority queue, Chebyshev heuristic ×5ft. Diagonals cost 5ft (matches the deliberate-simplification call-out in spec line 319).
  - `tileCost` (line 283) returns 5 / 10 (difficult) and adds +5 when `IsProne` — yields the spec's ×2 difficult, ×2 prone, ×3 stacked.
  - Walls block cardinal moves only; diagonal corner-cutting is intentionally allowed (line 242, matches spec line 1391).
  - `canPassThrough` (line 276): allies always pass, enemies require `abs(moverSize-occSize) ≥ 2` — matches spec.
  - Flying occupants (altitude > 0) skipped from blocking via `buildOccupantMap` (line 142) — matches the "tokens at different altitudes don't block ground tile" rule.
  - Axis-aligned wall preprocessing is fine; non-axis walls silently ignored (spec only describes axis-aligned walls).

### Phase 30 — Movement (`/move`)

- Status: **matches**
- Key files:
  - /home/ab/projects/DnDnD/internal/combat/movement.go
  - /home/ab/projects/DnDnD/internal/combat/movement_test.go (788 lines)
  - /home/ab/projects/DnDnD/internal/discord/move_handler.go
  - /home/ab/projects/DnDnD/internal/combat/opportunity_attack.go (per-PC reach lookup at line 79)
- Findings:
  - `ValidateMove` parses dest, runs A*, computes cost, deducts only on confirm. Split movement falls out for free because the turn row carries `MovementRemainingFt`.
  - Ephemeral confirmation (`FormatMoveConfirmation`) renders "includes difficult terrain" when applicable.
  - Cannot end on occupied tile at same altitude — movement.go:84.
  - `ValidateEndTurnPosition` (movement.go:211) blocks `/done` while sharing a tile.
  - Prone movement: `ValidateProneMoveStandAndMove` (half max speed via `StandFromProneCost`) and `ValidateProneMoveCrawl` (`IsProne=true` → x2 cost) are both wired.
  - **OAs in /move:** med-24 (commit 0a9ef2d) added `fireOpportunityAttacks` (move_handler.go:680) which uses `DetectOpportunityAttacksWithReach`. Honors per-PC reach (`lookupPCReach` reads "reach" property), NPC reach from creature attacks, and `HasDisengaged`. Queue-and-continue model: prompt is posted to `#your-turn` after the move commits — matches spec's queue-and-continue OA model (spec line 1418).

### Phase 31 — Altitude & Flying (`/fly`)

- Status: **partial**
- Key files:
  - /home/ab/projects/DnDnD/internal/combat/altitude.go
  - /home/ab/projects/DnDnD/internal/combat/altitude_test.go (281 lines)
  - /home/ab/projects/DnDnD/internal/discord/fly_handler.go
- Findings:
  - `ValidateFly` charges movement 1:1, rejects negative altitude, and uses the confirmation/cancel button flow. ✓
  - 3D distance via `Distance3D` rounded to nearest 5ft — matches spec line 333.
  - `FallDamage` rolls 1d6 per 10ft (truncated). Triggered by `applyFallDamageOnProne` from `ApplyCondition` when the new condition is `prone` and `AltitudeFt > 0` (combat/condition.go:180).
  - **Gap:** spec lists two fall triggers — "knocked prone OR loses fly speed" (spec line 336). Only the prone trigger is wired. There is no hook for "loses fly speed" (e.g., dispelled `fly` spell, polymorph reverting). No test exists for the loses-fly-speed branch.
  - Stacked token rendering is rendering-layer (not in this batch); confirmation message format matches.

### Phase 32 — Distance Awareness (`/distance`)

- Status: **matches**
- Key files:
  - /home/ab/projects/DnDnD/internal/combat/distance.go
  - /home/ab/projects/DnDnD/internal/discord/distance_handler.go
- Findings:
  - `/distance G1` → "You are Xft from <target>"; `/distance G1 AR` → "<from> is Xft from <to>". Both phrasings match spec lines 452–453.
  - `combatantDistance` uses `Distance3D` (attack.go:1230) so attack-log distances include altitude.
  - Attack log includes distance: `FormatAttackLog` prints `(<dist>ft)` when ranged or melee > 5ft (attack.go:734).
  - Range-rejection error from `ResolveAttack` (attack.go:447) is parsed back by `rangeRejectionMessage` (attack_handler.go:321) and reformatted via `FormatRangeRejection` — produces "Target is out of range — Xft away (max Yft)" (matches spec line 449).

### Phase 33 — Cover Calculation

- Status: **matches**
- Key files:
  - /home/ab/projects/DnDnD/internal/combat/cover.go
  - /home/ab/projects/DnDnD/internal/combat/cover_test.go (379 lines)
- Findings:
  - DMG grid variant: `CalculateCover` (cover.go:74) iterates the four attacker corners, lines to four target corners, picks the **best (least)** cover for the attacker — matches spec line 1381.
  - Blocked → cover mapping: 0=None, 1-2=Half, 3=Three-Quarters, 4=Full (cover.go:155).
  - Creature-granted half cover from intervening occupants (cover.go:170, `linePassesThroughTile`).
  - `EffectiveAC` adds +2/+5 AC; `DEXSaveBonus` mirrors the AC bonus for AoE saves.
  - Full cover short-circuits via `ErrTargetFullyCovered` BEFORE consuming attack resources (attack.go:1411, 878).
  - AoE-cover uses `CalculateCoverFromOrigin` with origin-corner closest to target.

### Phase 34 — Basic Attack Resolution (`/attack`)

- Status: **matches**
- Key files:
  - /home/ab/projects/DnDnD/internal/combat/attack.go (~1.6k lines)
  - /home/ab/projects/DnDnD/internal/combat/attack_test.go (3.3k lines)
  - /home/ab/projects/DnDnD/internal/discord/attack_handler.go
- Findings:
  - Weapon resolution order: explicit override → `equipped_main_hand` → unarmed strike (attack.go:1213).
  - Finesse auto-select: `abilityModForWeapon` picks max(STR, DEX) when `finesse` is present (attack.go:84). Same for Monk weapons when `monkLevel > 0`.
  - Critical hits: nat 20 always hits, doubles dice via `RollDamage(_, true)`. Nat 1 always misses (attack.go:579).
  - Auto-crit on melee within 5ft against paralyzed/unconscious — `CheckAutoCrit` (attack.go:707). Note: spec calls this "within 5ft"; the implementation checks `distFt <= 5` and the target conditions.
  - Range validation triggers a reject error before resource consumption (attack.go:447); Discord layer reformats with actual distance + allowed range.
  - Unarmed strike: flat 1 + STR mod (attack.go:67, damage="0"). Monk martial arts die override wired.
  - Combat log distance line: includes (Xft) for ranged or melee >5ft (attack.go:734).
  - `AttacksRemaining` decremented per swing via `UseAttack` (turnresources.go:158).

### Phase 35 — Advantage/Disadvantage Auto-Detection

- Status: **matches**
- Key files:
  - /home/ab/projects/DnDnD/internal/combat/advantage.go
  - /home/ab/projects/DnDnD/internal/combat/advantage_test.go (496 lines)
- Findings:
  - All condition-based sources covered (blinded, invisible, poisoned, prone, restrained, stunned, paralyzed, unconscious, petrified) — advantage.go:62-124.
  - Combat-context sources: reckless (line 33), hidden attacker (37-43), DM override (46-52), heavy weapon + Small/Tiny (line 88), ranged with hostile within 5ft (line 79), long range (line 84), obscurement (line 55-60), target prone within/beyond 5ft (line 110).
  - **Cancellation rule:** `resolveMode` (line 130) returns `AdvantageAndDisadvantage` when both lists are non-empty, which `Roller.RollD20` then resolves as a single normal d20 (dice/d20.go:49). Matches spec line 690 — one of each cancels.
  - Hostile-within-5ft detection uses Chebyshev `gridDistance <= 1` (attack.go:1304); excludes incapacitated foes (RAW correct).
  - **Target reckless** branch (advantage.go:117): attackers targeting a recklessly-attacking enemy get advantage. The transient `reckless` condition is applied to the attacker post-attack via `applyRecklessMarker` (attack.go:1332) and expires at start_of_turn.

### Phase 36 — Extra Attack & Two-Weapon Fighting

- Status: **matches**
- Key files:
  - /home/ab/projects/DnDnD/internal/combat/turnresources.go (AttacksPerActionForLevel, resolveAttacksPerAction)
  - /home/ab/projects/DnDnD/internal/combat/attack.go (OffhandAttack at line 1096)
  - /home/ab/projects/DnDnD/internal/refdata/seed_classes.go (Fighter `{1:1, 5:2, 11:3, 20:4}` at line 171)
- Findings:
  - `AttacksPerActionForLevel` (turnresources.go:137) returns the highest threshold ≤ level — defaults to 1.
  - `resolveAttacksPerAction` (line 272) iterates a character's classes and keeps the max (multiclass highest-wins, matches spec line 461).
  - Unused attacks forfeit on `/done` implicitly — `AttacksRemaining` is reset on each new turn via `ResolveTurnResources` and not persisted across the boundary.
  - `OffhandAttack` (attack.go:1096) validates both `equipped_main_hand` and `equipped_off_hand` are `light`; charges the bonus action; omits the ability mod from damage unless `HasFightingStyle("two_weapon_fighting")` (line 1146). Matches spec line 463.
  - Discord wiring: `/attack target:X offhand:true` (attack_handler.go:170, 232) — divergent surface vs spec's `/bonus offhand`, but the service path is correct.

### Phase 37 — Weapon Properties (versatile, reach, heavy, loading, thrown, ammunition, improvised)

- Status: **matches**
- Key files:
  - /home/ab/projects/DnDnD/internal/combat/attack.go
  - /home/ab/projects/DnDnD/internal/combat/ammunition.go
  - /home/ab/projects/DnDnD/internal/refdata/seeder.go (weapon seeds at line 87 — all properties present)
- Findings:
  - **Versatile:** `--twohanded` flag rejected when off-hand is occupied (attack.go:433); damage swaps to `versatile_damage` via `VersatileDamageExpression` / `resolveWeaponDamage(_,_,_,twoHanded,_)`. Matches spec line 661.
  - **Reach:** `MaxRange` returns 10ft for reach weapons (attack.go:118). OA detection also honors reach via `lookupPCReach`.
  - **Heavy:** disadvantage applied automatically when `AttackerSize` is "Small"/"Tiny" and weapon has `heavy` (advantage.go:88). `populateAttackContext` (attack.go:1317) auto-fills `AttackerSize` from creature row or defaults PCs to Medium.
  - **Loading:** `ApplyLoadingLimit` (attack.go:266) caps `AttacksRemaining` to 1 unless Crossbow Expert; called from Service.Attack at line 893.
  - **Thrown:** range validated via `ThrownMaxRange`. After a thrown attack, `EquippedMainHand` is cleared (attack.go:1003) — matches "weapon is removed from the character's hand" (spec line 669).
  - **Ammunition:** `DeductAmmunition` decrements on each swing (attack.go:903); `RecordAmmoSpent` feeds an `AmmoSpentTracker`, and `recoverEncounterAmmunition` (ammunition.go:117) restores half on `EndCombat` (service.go:1073). Matches spec line 667.
  - **Improvised:** `attackImprovised` path (attack.go:1021) uses 1d4 bludgeoning, 0 prof bonus unless Tavern Brawler (attack.go:455), supports `--thrown` 20/60ft range (attack.go:442). No inventory consumption.

### Phase 38 — Attack Modifier Flags (GWM, Sharpshooter, Reckless)

- Status: **partial — Reckless first-attack gate is broken**
- Key files:
  - /home/ab/projects/DnDnD/internal/combat/attack.go (validation at lines 460-476, 855-872)
  - /home/ab/projects/DnDnD/internal/combat/modifierflags_test.go (603 lines)
- Findings:
  - **GWM:** validates heavy melee weapon at the input boundary (attack.go:460) and that the character has the feat (attack.go:855). -5 to hit, +10 damage applied at attack.go:486.
  - **Sharpshooter:** validates ranged weapon (line 465) and feat (line 858). -5 / +10.
  - **Reckless:**
    - Validates Barbarian class (attack.go:861), melee weapon (line 470), STR-based attack — rejects finesse weapons where DEX > STR (line 473). ✓
    - Self-side advantage flows through `DetectAdvantage` via `input.Reckless`. ✓
    - Target-side half (enemies get adv vs reckless attacker) wired via the transient `reckless` condition on the attacker (`applyRecklessMarker`, attack.go:1332). ✓
    - **Bug (divergent):** the "first attack of the turn" gate at attack.go:870 checks `cmd.Turn.ActionUsed`. But `Service.Attack` only decrements `AttacksRemaining` via `UseAttack` — it never sets `ActionUsed = true`. The gate therefore never trips, so Reckless can be re-declared on every swing of an Extra Attack action. Spec line 682 explicitly says "first attack only". A correct gate would check `Turn.AttacksRemaining < maxAttacksForTurn` (already attacked at least once) or track via a per-turn flag. No test covers the negative path.
  - Invalid-flag errors are returned as formatted strings (e.g., "Great Weapon Master requires a heavy melee weapon", attack.go:461) — spec requirement met.

## Cross-cutting concerns

- **Coverage:** the eight files in this batch carry ~6.5k lines of test code, well above the project's 85% per-package target.
- **3D distance everywhere:** `combatantDistance` (attack.go:1230) routes all attack-range and range-rejection paths through `Distance3D`. Both `/distance` and attack-log entries reuse the same path.
- **Cover gating is correctly ordered:** total cover short-circuits BEFORE any resource (attack, bonus action, ammunition) is consumed (attack.go:878, 1045, 1152). Matches the "no resource burned on a rejected swing" intent.
- **OA detection is queue-and-continue:** spec line 1418 is honored — the mover finishes their full path and the hostile is pinged in `#your-turn` after the commit. Reach is data-driven (PCs read equipped melee weapon "reach"; NPCs read `creatures.attacks[*].reach_ft`).
- **Advantage cancellation is dice-layer correct:** `AdvantageAndDisadvantage` is treated as a single d20 in `Roller.RollD20`, not as roll-two-pick-neither — matches RAW.
- **Ammunition tracker is in-memory:** `AmmoSpentTracker` is per-process. If the bot restarts mid-encounter the spent counter is lost (no recovery). Documented in the code comment but worth surfacing.
- **NPC PC reach detection is best-effort:** the `lookupPCReach` map lookup is nil-safe (move_handler.go:702); a missing lookup degrades to 5ft for PCs, which can miss OAs from PC hostiles with reach weapons (mixed-faction PvP).

## Critical items

1. **Reckless first-attack gate is non-functional (P38).** `cmd.Turn.ActionUsed` is never set by `Service.Attack`, so the gate at attack.go:870 always reads false and Reckless can be applied on every swing of a multi-attack action. Fix: either set `ActionUsed=true` on the first attack of a turn, track a dedicated `reckless_declared` boolean on the turn, or check `AttacksRemaining < maxAttacks` at the gate. Add a negative test asserting that Reckless on swing 2 of a Fighter 5's Extra Attack is rejected.
2. **`/fly` fall-damage trigger is incomplete (P31).** Only the prone-while-airborne path fires fall damage; the spec's "loses fly speed" trigger (spell expiration, polymorph, dispel) is not wired. Currently no test covers the loses-fly-speed branch. Add a hook when fly speed is removed (e.g., when the `fly` magical effect ends) that calls `applyFallDamageOnProne`-equivalent logic.
3. **`/bonus offhand` surface mismatch (P36).** Spec exposes off-hand attacks as `/bonus offhand`; the implementation exposes them as `/attack offhand:true`. Either update the spec or add a `/bonus offhand` shim alias for player-facing consistency. Functional behaviour is correct.
