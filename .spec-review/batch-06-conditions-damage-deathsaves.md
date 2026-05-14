# Batch 06: Conditions, damage, death saves, standard actions (Phases 39–43, 54)

## Summary

All six phases land in the codebase and are wired into the live combat flow
(slash handlers, damage pipeline, death-save state machine, character cards).
Coverage is largely faithful to the spec — every RAW condition is recognized,
the damage pipeline routes through a single seam (`Service.ApplyDamage`) that
honours R/I/V, temp HP, exhaustion HP-halving and exhaustion-6 death, and the
death-save state machine covers drop-to-0, damage-at-0, nat-1 / nat-20,
instant-death overflow, stabilization, and heal-from-0.

That said, several real defects exist:

1. `/action help` writes a `help_advantage` condition that no read site ever
   consumes — Help action grants no actual advantage today.
2. `AutoResolveTurn`'s Dodge condition uses `ExpiresOn: "start_of_next_turn"`,
   which doesn't match `isExpired`'s `start_of_turn` / `end_of_turn` check and
   never sets `SourceCombatantID` — the auto-skip Dodge never expires.
3. Phase 42 exhaustion has no application path: nothing in the dashboard,
   `/rest`, or any handler increments or decrements `exhaustion_level`, so
   levels 1–6 are unreachable in practice (only the read-side effects work).
4. `/cast spare-the-dying` does not actually stabilize the target — the spell
   seed exists with `ResolutionMode: "auto"`, but no `StabilizeTarget` branch
   in the cast pipeline wires it up. Medicine-check stabilization (Phase 43)
   *is* wired (`action_handler.go:1121`).
5. `DropProne` bypasses `Service.ApplyCondition`, so the prone-while-airborne
   fall-damage hook (`applyFallDamageOnProne`) never fires when a player
   voluntarily drops prone in flight; concentration immunity check via
   ApplyCondition is also skipped (minor — prone is not incapacitating).

## Per-phase findings

### Phase 39 — Condition System: Application, Tracking, Auto-Expiration
- Status: **Matches** (with one bug — see cross-cutting #2)
- Key files:
  - `internal/combat/condition.go` (AddCondition / HasCondition / GetCondition / ListConditions / RemoveCondition; `isExpired`; `CheckExpiredConditions`; `Service.ApplyCondition` + immunity check; `ApplyConditionWithLog`; `RemoveConditionFromCombatant`; `ProcessTurnStart` / `ProcessTurnEnd` + `*WithLog` variants)
  - `internal/refdata/seeder.go:175` (16 conditions seeded — 15 SRD + "surprised")
  - `internal/refdata/conditions.sql.go`
- Findings:
  - JSONB condition CRUD is implemented and persists via `UpdateCombatantConditions`.
  - Duration fields (`duration_rounds`, `started_round`, `source_combatant_id`, `expires_on`) match spec; indefinite = `duration_rounds <= 0`.
  - Turn-start / turn-end expiration sweeps fire across all combatants whose conditions have matching `source_combatant_id`, with optional action-log persistence (`ProcessTurnStartWithLog` / `ProcessTurnEndWithLog`).
  - Combat-log messages emitted: `🔴 applied`, `⏱️ expired (placed by X)`, `🟢 removed`, `🛡️ immune`.
  - **Bug**: `timer_resolution.go:194-199` (auto-skip Dodge) applies a `dodge` condition with `ExpiresOn: "start_of_next_turn"` *and* no `SourceCombatantID`. `isExpired` in `condition.go:72-91` only matches `"start_of_turn"` or `"end_of_turn"` *and* requires `SourceCombatantID == triggerCombatantID`. The auto-skip Dodge will therefore never expire automatically. Manual /action dodge in `standard_actions.go:168-174` does it correctly.

### Phase 40 — Condition Effects: Saves, Checks, Attacks, Speed, Action Blocking
- Status: **Matches** (with one functional gap — Help advantage not consumed)
- Key files:
  - `internal/combat/condition_effects.go` (CheckSaveConditionEffects, CheckAbilityCheckEffects, EffectiveSpeed, IsIncapacitated, CanAct, IsCharmedBy, ValidateFrightenedMovement, *WithExhaustion variants)
  - `internal/combat/advantage.go` (attacker/target condition adv/disadv)
  - `internal/combat/attack.go:1237-1296` (charmed attack restriction, auto-crit within 5ft for paralyzed/unconscious)
  - `internal/discord/commands.go:24` (`incapacitatedRejection` shared across handlers)
  - `internal/discord/action_handler.go:349,611` / `attack_handler.go:213` / `bonus_handler.go:197` / `move_handler.go:308` (incapacitation gating on /action /attack /bonus /move)
- Findings:
  - Save effects: paralyzed/stunned/unconscious/petrified auto-fail STR/DEX (`condition_effects.go:12`), restrained disadv on DEX, dodge adv on DEX — all present and unit-tested.
  - Ability check effects: blinded/deafened auto-fail with sight/hearing context flags, frightened (only when source visible) and poisoned disadvantage — present.
  - Attack adv/disadv table: all eight conditions implemented in `advantage.go` (blinded, invisible, poisoned, prone, restrained both sides; stunned/paralyzed/unconscious/petrified grant adv to attackers; prone within 5ft adv vs beyond 5ft disadv).
  - Paralyzed/unconscious auto-crit within 5ft handled in `attack.go:716-722`.
  - Charmed attack restriction (both `/attack` and `/cast`) wired via `validateCharmedAttack` + `IsCharmedBy`.
  - Speed: grappled / restrained → 0 (`EffectiveSpeed`); frightened can't move closer (`ValidateFrightenedMovement` + `move_handler.go:957-964`).
  - Action blocking: incapacitated / stunned / paralyzed / unconscious / petrified rejected at the slash-handler layer.
  - Note: the spec table at line 1240 says frightened has "No direct attack effect" (simplified from RAW). Implementation matches the spec, not RAW. Acceptable.
  - **Bug**: `/action help` writes a `help_advantage` condition on the ally (`standard_actions.go:250-257`) but no read site in `advantage.go` or `attack.go` ever consumes it. The Help action grants no mechanical advantage today.

### Phase 41 — Moving While Prone
- Status: **Matches**
- Key files:
  - `internal/discord/move_handler.go:372-400` (Stand & Move vs Crawl prompt buttons)
  - `internal/discord/move_handler.go:1065-1226` (HandleProneStandAndMove / HandleProneCrawl, mode confirmation, prone removal, HasStoodThisTurn)
  - `internal/discord/move_handler.go:1290+` (ParseProneMoveData)
  - `internal/combat/condition_effects.go:254-258` (StandFromProneCost = max/2)
- Findings:
  - Prone-and-not-yet-stood combatants get the [🧍 Stand & Move] / [🐛 Crawl] prompt before standard movement.
  - Stand & Move deducts half-max-speed + path cost, sets `HasStoodThisTurn`, removes prone.
  - Crawl doubles per-tile cost; stacks with difficult terrain.
  - Subsequent /move skips the prompt once `HasStoodThisTurn` is set.

### Phase 42 — Damage Processing
- Status: **Partial** (read-side complete; no write path for exhaustion)
- Key files:
  - `internal/combat/damage.go` (ApplyDamageResistances, AbsorbTempHP, GrantTempHP, ExhaustionEffectiveSpeed/MaxHP, ExhaustionRollEffect, IsExhaustedToDeath, CheckConditionImmunity, ApplyDamageInput/Result, Service.ApplyDamage, routePhase43DeathSave, resolveDamageProfile)
  - `internal/combat/condition_effects.go:200-240` (EffectiveSpeedWithExhaustion, CheckSaveWithExhaustion, CheckAbilityCheckWithExhaustion)
  - Call-site funnels: `aoe.go:762`, `channel_divinity.go:272`, `dm_dashboard_handler.go:330`, `dm_dashboard_undo.go:194/385`, `turn_builder_handler.go:302`, `altitude.go:158`
- Findings:
  - R/I/V resolution: immunity zeroes (precedence honoured), resistance halves (`rawDamage / 2`, integer-truncated == rounded down), vulnerability doubles, R+V cancel — all in `ApplyDamageResistances`.
  - Petrified resistance to all damage types auto-applied (`damage.go:31` `hasCondition(conditions, "petrified")`).
  - Temp HP: `AbsorbTempHP` deducts temp first, `GrantTempHP` keeps higher of current vs new (no stacking). Temp HP cannot be healed — heal paths don't touch `temp_hp` (verified by call-site inspection, not explicitly enforced by code).
  - Exhaustion read-side effects all present: speed (lvl 2 half, lvl 5 zero), HP-max half (lvl 4), check disadv (lvl 1+), attack/save disadv (lvl 3+), instant death (lvl 6).
  - Condition immunity check fires inside `Service.ApplyCondition` (`condition.go:144-153`).
  - Concentration save on damage: `MaybeCreateConcentrationSaveOnDamage` is wired (`concentration.go:343,403`); incapacitation auto-breaks concentration (`condition.go:191-201`). This is the Phase 118 cleanup.
  - **Divergence**: no code path increments `exhaustion_level`. There is no `/exhaustion` slash command, no dashboard mutation, no `/rest` decrement, no forced-march / starvation hook. The spec (line 1365) says "applied by DM from dashboard" + "Decreases by 1 level per long rest". Read-side effects exist but are unreachable in production. (Tests stub `ExhaustionLevel` directly.)
  - **Divergence**: temp HP duration tracking — `damage.go` doesn't expire temp HP at duration end; the spec says "expires at end of duration ... or on long rest". No long-rest hook clears `temp_hp`.

### Phase 43 — Death Saves & Unconsciousness
- Status: **Matches** (with one wiring gap — Spare the Dying)
- Key files:
  - `internal/combat/deathsave.go` (TokenState, ParseDeathSaves / MarshalDeathSaves, CheckInstantDeath, ProcessDropToZeroHP, RollDeathSave, ApplyDamageAtZeroHP, HealFromZeroHP, StabilizeTarget, GetTokenState, IsDying, ConditionsForDying, MaybeResetDeathSavesOnHeal, resetDyingState)
  - `internal/combat/damage.go:305-367` (routePhase43DeathSave inside ApplyDamage)
  - `internal/discord/deathsave_handler.go` (/deathsave slash command)
  - `internal/discord/action_handler.go:1056-1121` (Medicine-check stabilization)
  - `internal/discord/commands.go:24` (incapacitatedRejection blocks all commands except /deathsave at 0 HP)
  - `internal/combat/timer_resolution.go:143-189` (auto-resolve death save on player timeout)
- Findings:
  - Drop-to-0 path: applies unconscious + prone (`ConditionsForDying`), routes through `applyDamageHP` which auto-applies the dying-condition bundle.
  - Instant-death rule: overflow ≥ max HP → `TokenDead`, no death saves.
  - Death save rolls: `RollDeathSave` handles nat-20 (regain 1 HP + reset tallies), nat-1 (2 failures), ≥10 success, <10 failure, 3S → stable, 3F → dead.
  - Damage at 0 HP: `ApplyDamageAtZeroHP` adds 1 failure (or 2 on crit, via `IsCritical` flag plumbed from attack call sites). Instant-death overflow short-circuits before tallying.
  - Concentration broken on drop-to-0: handled inside `applyDamageHP` via `ApplyConditionsForDying`.
  - Heal-from-0: `MaybeResetDeathSavesOnHeal` resets tallies + removes unconscious/prone; remains prone is **not** enforced (the dying-condition bundle includes prone which gets removed too — spec says "still prone after waking up"). Minor divergence.
  - Timeout auto-resolution: `AutoResolveTurn` calls `RollDeathSave` for dying combatants on timeout (`timer_resolution.go:145-189`).
  - Token states (`alive` / `dying` / `dead` / `stable`) defined and used by `GetTokenState`.
  - **Wiring gap**: `/cast spare-the-dying` is in the spell seed with `ResolutionMode: "auto"` but no code path calls `StabilizeTarget` when this spell is cast. Medicine check stabilization (DC 10) IS wired (`action_handler.go:1121`).
  - **Minor divergence**: heal-from-0 removes the prone condition too, contrary to spec "Status → conscious, still prone".

### Phase 54 — Standard Actions
- Status: **Matches** (with one functional gap — Help advantage)
- Key files:
  - `internal/combat/standard_actions.go` (Dash, Disengage, Dodge, Help, Hide, Stand, DropProne, Escape, CunningAction)
  - `internal/discord/action_handler.go:621-869` (dispatch table — surge, dash, disengage, dodge, help, hide, stand, drop-prone, escape)
  - `internal/discord/bonus_handler.go` (cunning-action dash/disengage/hide; step-of-the-wind)
- Findings:
  - All eight standard actions land and persist turn-resource state through `UseResource` + `UpdateTurnActions`.
  - Dash: +speed to MovementRemainingFt; resolves base speed via `resolveBaseSpeed` (defaults to 30 for NPCs).
  - Disengage: sets `HasDisengaged = true` on the turn (consumed by OA detection in Phase 55).
  - Dodge: applies a 1-round `dodge` condition (`start_of_turn` expiry, source set to self). Wired into `CheckSaveConditionEffects` for DEX-save advantage and into `advantage.go` (target-side dodge is *not* directly checked in advantage.go — checked via the condition lookup elsewhere). Standard-actions Dodge applies `start_of_turn` correctly; auto-skip Dodge in timer_resolution.go does **not** — see cross-cutting #2.
  - Help: adjacency check (5ft) on helper-to-target; sets `help_advantage` condition on ally. **No read site consumes this** — Help grants no actual advantage. (Bug.)
  - Hide: full Stealth (skill mod + armor stealth disadv + Medium Armor Master) vs highest passive Perception across hostiles, sets `IsVisible=false` on success. Obscurement gating in handler (`action_handler.go:769-782`).
  - Stand: cost = half max speed (rounded down), rejects if insufficient movement, removes prone, sets `HasStoodThisTurn`. Speed resolved per-character via `ActionSpeedLookup` (Halfling 25 → 13, Tabaxi 35 → 18).
  - DropProne: applies `prone` (indefinite) — bypasses `Service.ApplyCondition`, so the prone-while-airborne fall-damage hook (`applyFallDamageOnProne` in `altitude.go:135`) does not fire. Spec lists no exception, so this is a divergence (rare, but a player could /action drop-prone while flying and avoid fall damage).
  - Escape: contested d20 — Athletics (STR) or Acrobatics (DEX) vs grappler's Athletics. Default takes whichever ability is higher *per spec* (line 1145) — the implementation defaults to Athletics and only uses Acrobatics on `--acrobatics`; it does NOT auto-pick the higher mod (`standard_actions.go:716-720`). Minor divergence vs spec.
  - Cunning Action (Rogue): dash / disengage / hide as bonus action; Rogue lvl 2 validation (`CunningAction`); hide reuses `resolveHide` so stealth check and IsVisible mutation are identical to action-cost hide.
  - All actions invoke `CanActRaw` (incapacitation gate), except `Stand` and `DropProne` (correctly, since these are not "actions" per RAW).

## Cross-cutting concerns

1. **`help_advantage` never consumed.** `standard_actions.go:250-257` applies the condition; nothing in `advantage.go` / `attack.go` looks for it. Fix is one switch arm in `DetectAdvantage`'s target loop and a follow-up `RemoveCondition` after the consumed roll.
2. **Auto-skip Dodge never expires.** `timer_resolution.go:198` uses `ExpiresOn: "start_of_next_turn"` (unrecognized) and omits `SourceCombatantID`. Either change the value to `"start_of_turn"` and set source to `combatant.ID.String()`, or teach `isExpired` to honour `start_of_next_turn`.
3. **Exhaustion has no write path.** No /exhaustion command, no /rest decrement, no forced-march hook. Phase 42 spec language ("applied by DM from dashboard ... Decreases by 1 level per long rest") is not implemented. Read-side effects are wired correctly but are unreachable.
4. **Temp HP not auto-expired.** Spec line 1346 says temp HP expires at duration end or long rest; neither hook exists.
5. **Spare the Dying isn't wired.** Spell exists in seed (`seed_spells_cantrips.go:27`, ResolutionMode "auto") but cast pipeline doesn't trigger `StabilizeTarget`.
6. **Heal-from-0 removes prone.** Spec says "still prone" after revive; `resetDyingState` removes both unconscious and prone.
7. **DropProne airborne loophole.** `DropProne` doesn't route through `Service.ApplyCondition`, so the `applyFallDamageOnProne` hook is skipped. A flying combatant can /action drop-prone with no fall consequences.
8. **Escape default ability.** Spec says default uses the higher of STR/DEX mods; implementation defaults to STR (Athletics).

## Critical items

- Help action grants no advantage (silently broken — players will think it works).
- Exhaustion is read-only — no way for a DM to apply or remove it in-game.
- /cast spare-the-dying is a no-op stabilizer.
- Auto-skipped Dodge condition lingers indefinitely (state leak; will manifest as "I keep having disadvantage on me / advantage on DEX saves rounds after the auto-skip").
