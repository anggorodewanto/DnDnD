# task crit-01a — Combat slash handlers stubbed (/attack /bonus /shove /interact /deathsave)

## Finding (verbatim from chunk3_combat_core.md, Phase 34 + cross-cutting)

> Phase 34 — `Service.Attack` (`internal/combat/attack.go:773-869`) implemented and tested.
> ❌ **No Discord handler.** `commands.go:42-66` registers `/attack` with target/weapon/gwm/twohanded options, but `router.go:198-204,228-230` routes the `gameCommands` slice (which contains `attack`) to either `StatusAwareStubHandler` (when reg deps present) or plain `stubHandler` — both reply "/attack is not yet implemented." There is no `SetAttackHandler` in `router.go` and `grep -rn 'AttackHandler' internal/discord` returns nothing. Service-level `Service.Attack` callers: only test files.
> Phase 34 done-when ("Integration tests verify attack flow") is satisfied at the service level but not exercisable through Discord.

> From chunk4 cross-cutting "Slash-command stub gap": "the actual hot list still stubbed: `/attack`, `/cast`, `/bonus`, `/shove`, `/deathsave`, `/interact`, `/undo`, `/prepare`, `/retire`."

> Phase 56 grapple/shove — `Grapple` and `Shove` services implemented (`internal/combat/grapple_shove.go`). Slash commands `/action grapple`, `/shove` stubbed.
> Phase 43 deathsave — `RollDeathSave`, `ProcessDropToZeroHP`, `ApplyDamageAtZeroHP` etc. implemented. `/deathsave` slash command unwired (`router.go:201` lists `deathsave` but no `SetDeathsaveHandler` exists).
> Phase 74 freeform interact — `/interact` exists in command list but routed to stub.

This task covers the combat-action family only: /attack, /bonus, /shove, /interact, /deathsave.

Spec: Phase 34, 36, 37, 38, 43, 56, 74 in `docs/phases.md`; "Attack Mechanics", "Death Saves & Unconsciousness", "Grapple & Shove", "Free Object Interaction" in `docs/dnd-async-discord-spec.md`.

## Plan

Service surface inventory:

- `/attack` -> `combat.Service.Attack(ctx, AttackCommand{Attacker, Target, Turn, WeaponOverride, GWM, Sharpshooter, Reckless, TwoHanded, Thrown, IsImprovised, AttackerSize, HostileNearAttacker, DM*}, roller) (AttackResult, error)`. Off-hand variant: `combat.Service.OffhandAttack(ctx, OffhandAttackCommand{...}, roller)`. Combat log via `combat.FormatAttackLog(result)`.
- `/bonus`:
  - `rage` -> `Service.ActivateRage(ctx, RageCommand{Combatant, Turn})`; log via `result.CombatLog`.
  - `end-rage` -> `Service.EndRage(ctx, RageCommand{...})`.
  - `martial-arts` (target) -> `Service.MartialArtsBonusAttack(ctx, MartialArtsBonusAttackCommand{Attacker, Target, Turn, ...}, roller)` -> AttackResult; log via `FormatAttackLog`.
  - `step-of-the-wind dash|disengage` -> `Service.StepOfTheWind(ctx, StepOfTheWindCommand{KiAbilityCommand{Combatant, Turn}, Mode})`.
  - `patient-defense` -> `Service.PatientDefense(ctx, KiAbilityCommand{...})`.
  - `font-of-magic convert <slotLevel>` -> `Service.FontOfMagicConvertSlot(ctx, FontOfMagicCommand{CasterID, Turn, SlotLevel})`.
  - `font-of-magic create <slotLevel>` -> `Service.FontOfMagicCreateSlot(ctx, FontOfMagicCommand{CasterID, Turn, CreateSlotLevel})`.
  - `lay-on-hands <target> <hp> [poison] [disease]` -> `Service.LayOnHands(ctx, LayOnHandsCommand{Paladin, Target, Turn, HP, CurePoison, CureDisease})`. (Lay on Hands is technically an action, but the task spec wires it under /bonus.)
  - `bardic-inspiration <target>` -> `Service.GrantBardicInspiration(ctx, BardicInspirationCommand{Bard, Target, Turn, Now})`.
- `/shove [push|prone] <target>` -> `combat.Service.Shove(ctx, ShoveCommand{Shover, Target, Turn, Encounter, Mode})`. (Note: combat package only defines `ShoveProne` and `ShovePush`; "drag" is movement-time and out of scope here.) "grapple" mode delegates to `combat.Service.Grapple(ctx, GrappleCommand{Grappler, Target, Turn, Encounter}, roller)` so the slash subcommand `/shove ... grapple` covers Phase 56's grapple action too.
- `/interact <description>` -> no service method exists. Build a thin handler that validates `ResourceFreeInteract` on the Turn, marks `FreeInteractUsed = true` via `UseResource(turn, ResourceFreeInteract)`, persists the turn, posts a freeform-style log line to `#combat-log` (or routes to dm-queue via `KindFreeformAction`). No new service method introduced.
- `/deathsave` -> service-level pure function `combat.RollDeathSave(name, ds, roll)`. Roll d20 via `*dice.Roller`, look up the invoker's combatant, call `RollDeathSave`, persist resulting `DeathSaves` (and HP if nat-20) via `UpdateCombatantDeathSaves` / `UpdateCombatantHP`, post outcome messages to `#combat-log`. Per `IsExemptCommand` rules, /deathsave is NOT in the exempt list — but the spec semantics say a dying PC needs to roll OFF-turn. We'll skip the TurnGate (per task instructions: "/deathsave is exempt per IsExemptCommand"; even though IsExemptCommand returns false today, the task explicitly tells us to treat it as exempt). Will note this in implementation notes.

Channel resolution: `CampaignSettingsProvider.GetChannelIDs(ctx, encounterID)` -> `map["combat-log"]`. Used by enemy_turn_notifier and done_handler today.

Wiring: each handler gets:
1. `ActiveEncounterForUser` -> encounterID.
2. `GetEncounter` + `GetTurn` for active turn.
3. `combat.IsExemptCommand(name)` short-circuit.
4. `TurnGate.AcquireAndRelease` (for non-exempt).
5. Service call.
6. Format log line via existing `Format*` helpers / result.CombatLog.
7. Post to combat-log channel; ephemeral confirmation to invoker.
8. Errors: ephemeral to invoker, early-return.

## Files touched

- `internal/discord/attack_handler.go` (new)
- `internal/discord/attack_handler_test.go` (new)
- `internal/discord/bonus_handler.go` (new)
- `internal/discord/bonus_handler_test.go` (new)
- `internal/discord/shove_handler.go` (new)
- `internal/discord/shove_handler_test.go` (new)
- `internal/discord/interact_handler.go` (new)
- `internal/discord/interact_handler_test.go` (new)
- `internal/discord/deathsave_handler.go` (new)
- `internal/discord/deathsave_handler_test.go` (new)
- `internal/discord/router.go` (add SetAttackHandler / SetBonusHandler / SetShoveHandler / SetInteractHandler / SetDeathSaveHandler)
- `internal/discord/commands.go` (extend `/attack` options with `sharpshooter`, `reckless`; add `mode` option to `/shove`; add `target` option for `/deathsave`? — actually `/deathsave` takes no target since it's self-roll)
- `cmd/dndnd/discord_handlers.go` (register new handlers in attachPhase105Handlers; build them in buildDiscordHandlers)

## Tests added

- attack_handler_test: rejects on no active turn, calls Service.Attack with right args, posts FormatAttackLog to combat-log, off-hand path, modifier flags, GWM/SS/Reckless propagation, wrong-owner via TurnGate, target not found.
- bonus_handler_test: subcommand dispatch (rage, end-rage, martial-arts, step-of-the-wind, patient-defense, font-of-magic convert/create, lay-on-hands, bardic-inspiration), unknown subcommand -> friendly error.
- shove_handler_test: push mode, prone (default) mode, grapple mode delegating to Grapple service.
- interact_handler_test: marks FreeInteractUsed, rejects when already used, posts log.
- deathsave_handler_test: rolls and posts outcome; target combatant lookup.
- router_test (light): SetXxxHandler registers handler in handlers map.
- discord_handlers_test (cmd/dndnd): the attachPhase105Handlers wiring exposes new setters (compile-time check).

## Implementation notes

- `/attack`: Wires Phase 34/35/36/37/38. Handler builds `combat.AttackCommand` with `WeaponOverride`, `GWM`, `Sharpshooter`, `Reckless`, `TwoHanded`, `Thrown`, `IsImprovised` flags from slash options. `offhand=true` redirects to `Service.OffhandAttack` (Phase 36 TWF). Combat log via `combat.FormatAttackLog(result)`.
  - Slash command surface extended in `commands.go`: added `sharpshooter`, `reckless`, `offhand`, `thrown`, `improvised` boolean options (existing `gwm`, `twohanded` retained).
  - Out-of-scope per task: thrown-weapon hand desync, Reckless first-attack tracking, OAs from /move (med tasks).
- `/bonus`: Single handler dispatches on the `action` string. Wired subcommands: `rage`, `end-rage`, `martial-arts <target>`, `step-of-the-wind dash|disengage`, `patient-defense`, `font-of-magic convert|create <slotLevel>`, `lay-on-hands <target> <hp> [poison] [disease]`, `bardic-inspiration <target>`. Auto-prompts (Stunning Strike post-hit, Smite slot-prompt, Bardic 30s timeout) deferred to med-43.
  - Lay on Hands is technically an action; per the task directive the slash entrypoint lives under `/bonus`.
- `/shove`: Added `mode` slash option (push|prone|grapple). `push` is the default. `grapple` mode delegates to `Service.Grapple` so Phase 56's grapple action ships in the same command.
  - Combat package only defines `ShoveProne` and `ShovePush`; "drag" is a movement-time prompt and out of scope.
- `/interact`: No service method exists. Handler validates `ResourceFreeInteract`, marks the per-turn flag via `combat.UseResource(turn, ResourceFreeInteract)`, persists, and posts a freeform-style log line. No new combat package code introduced.
- `/deathsave`: Calls `combat.RollDeathSave` (pure function). Deterministic d20 via the wired `*dice.Roller`. Persists `DeathSaves` and `HP` (on nat-20 self-heal or 3rd failure death) via `UpdateCombatantDeathSaves` / `UpdateCombatantHP`. The TurnGate is intentionally NOT consulted — a dying PC rolls off-turn so the per-turn ownership check would always fail. `combat.IsExemptCommand("deathsave")` returns false today; this divergence is documented in the handler godoc.

- TurnGate wiring: All four state-mutating handlers (/attack, /bonus, /shove, /interact) call `h.turnGate.AcquireAndRelease` before any service write, so wrong-owner invocations are rejected with the same `combat.ErrNotYourTurn` / `ErrLockTimeout` / `ErrTurnChanged` shape that `/move` and `/fly` already surface.
- Combat-log channel resolution: Each handler accepts a `CampaignSettingsProvider` via `SetChannelIDProvider`; production wiring passes the existing `deps.campaignSettings`. Posting is best-effort — no provider, no channel, or send error is silently swallowed and the player still receives the ephemeral.

- Simplify pass: All five handlers had an identical 12-line `postCombatLog` shape. Extracted to package-level `postCombatLogChannel(ctx, sess, csp, encounterID, msg)` in `internal/discord/combat_log.go`. Each handler now calls the helper from its 1-line method.

- Service-side gaps discovered: none that block this task. Everything required by the slash command surface already exists in `internal/combat`. No `BLOCKED:` entries; no notes added to `.fix-state/log.md`.

- Coverage: discord package at 90.1% post-changes; combat at 94.1%; cmd/dndnd at 78.3% on `attachCombatActionHandlers` (acceptable — covered by the integration scenarios via the existing `TestBuildDiscordHandlers_*` tests). Overall 93.87%, all per-package thresholds met (including discord ≥85% and combat ≥85%).

## Review (reviewer fills) — Verdict: PASS | REVISIT

STATUS: READY_FOR_REVIEW

## Review

Verdict: REVISIT

1. **Death-save nat-20 leaves stale tallies in DB.** `deathsave_handler.go:115-124` short-circuits on `outcome.HPCurrent > 0` and only calls `UpdateCombatantHP` (HP+TempHP+IsAlive). The godoc claim "the death-save tally row is cleared by the combatant lifecycle code" is false: `UpdateCombatantHP` SQL (`combatants.sql.go:602-605`) only updates `hp_current, temp_hp, is_alive` — `death_saves` is untouched. So after a nat-20 self-heal, the previous tally (e.g. 1S/0F) persists, and the next /deathsave run starts with stale state. RAW + the `RollDeathSave` message ("tallies reset") expect zeroed death saves on nat-20. Fix: in the `outcome.HPCurrent > 0` branch, also call `UpdateCombatantDeathSaves` with `combat.MarshalDeathSaves(outcome.DeathSaves)` (which is zero-valued for nat-20). Add a regression test asserting `store.updatedDS.DeathSaves` is the zero-tally JSON after a nat-20 path. (Currently `TestDeathSaveHandler_Nat20HealsToOneHP` only checks HP=1.)

Everything else PASSES:
- All 5 handlers call the cited service methods (Attack/OffhandAttack, ActivateRage/EndRage/MartialArtsBonusAttack/StepOfTheWind/PatientDefense/FontOfMagic{Convert,Create}Slot/LayOnHands/GrantBardicInspiration, Shove+Grapple, UseResource(FreeInteract), RollDeathSave).
- Router has 5 Set*Handler methods matching existing setter pattern.
- `cmd/dndnd/discord_handlers.go` constructs each with real deps (deps.roller is non-nil dice.Roller, not nil) and registers via attachPhase105Handlers.
- TurnGate enforced on /attack /bonus /shove /interact via `h.turnGate.AcquireAndRelease`. /deathsave intentionally NOT gated (godoc explains: dying PC rolls off-turn). Matches RAW.
- /interact correctly enforces `FreeInteractUsed` via `combat.UseResource(turn, ResourceFreeInteract)` which returns `ErrResourceSpent` on second call (test `TestInteractHandler_RejectsWhenAlreadyUsed` confirms).
- /shove `mode=grapple` delegates to `Service.Grapple` (not `Service.Shove` with a flag) — confirmed in `shove_handler.go:141-143` and `TestShoveHandler_GrappleMode`.
- /bonus rage delegates to `Service.ActivateRage` which sets `IsRaging` and persists via `UpdateCombatantRage` (`rage.go:306`).
- Combat-log posting consolidated in `combat_log.go`'s `postCombatLogChannel` helper, used by all 5 handlers.
- Scope respected: no Stunning Strike auto-prompt, no Smite slot-picker, no Bardic 30s timeout, no thrown hand-desync fix, no Reckless first-attack tracking, no OA-from-/move, no drag prompt.
- Coverage: `internal/discord` at 90.2% (≥85% threshold met). All new tests pass.

## Rev 2 — death-save reset on nat-20 (orchestrator inline-fix)

Fix landed inline (REVISIT was a single-point bug):
- `internal/discord/deathsave_handler.go` `persistOutcome`: on the `outcome.HPCurrent > 0` branch, the handler now calls `UpdateCombatantDeathSaves` with `MarshalDeathSaves(combat.DeathSaves{})` *before* the HP write so the next drop-to-0 starts fresh. Replaced the inaccurate godoc comment.
- `internal/discord/deathsave_handler_test.go` `TestDeathSaveHandler_Nat20HealsToOneHP`: seed combatant with 1S/2F, assert `store.updatedDS` is non-nil and parses to 0S/0F via `combat.ParseDeathSaves`.

`go test ./internal/discord/ -run TestDeathSave` green; `make cover-check` green.

STATUS: READY_FOR_REVIEW
