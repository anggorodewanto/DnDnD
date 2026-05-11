# task crit-01b — Spell slash handlers stubbed (/cast /prepare-spells /action ready)

## Finding (verbatim from chunk5_spells_reactions.md)

> Phase 58 — `combat.Service.Cast` has zero non-test callers. No `cast_handler.go` in `internal/discord/`.
> Phase 59 — `CastAoE` has zero non-test callers; pending-save persistence + ping flow not wired.
> Phase 65 — No paginated Discord select-menu UI for `/prepare`. `FormatPreparationMessage` is text-only.
> Phase 71 — No `/action ready` Discord handler. `ReadyAction` service exists.

> Cross-cutting wiring debt: "every `grep` for production callers of `Cast/CastAoE/PrepareSpells/ReadyAction` returns only `_test.go` files."

This task wires the SLASH HANDLERS only. Out of scope (separate tasks):
- Zone auto-creation in Cast (med-26)
- Silence cast-time rejection (med-25)
- ReadyAction slot deduction + concentration link (med-28)
- Counterspell Discord prompt (med-29)
- Interactive metamagic prompts (med-30)
- /prepare paginated select-menu UX — implement basic ephemeral text UI per `FormatPreparationMessage`; defer the multi-step select menu pagination if it bloats scope

Spec: Phase 58, 59, 65, 71 in `docs/phases.md`; "Spell Casting Details" + "Reactions" in `docs/dnd-async-discord-spec.md`.

## Plan

1. Cast handler (`internal/discord/cast_handler.go`): single-target vs AoE
   dispatched on `spell.AreaOfEffect.Valid`. Single path resolves target via
   `combat.ResolveTarget` and calls `combat.Service.Cast`. AoE path parses the
   target coordinate via `renderer.ParseCoordinate`, loads walls best-effort
   from the encounter map, and calls `combat.Service.CastAoE`. Combat-log
   line posted via `combat.FormatCastLog` / `FormatAoECastLog` and mirrored
   via `postCombatLogChannel` (same shape as /attack).
2. Prepare handler (`internal/discord/prepare_handler.go`): MVP text UX. With
   no `spells` arg, post the ephemeral preview from `FormatPreparationMessage`.
   With a `spells:id1,id2` arg, commit via `PrepareSpells`. Reject when the
   active encounter is `status="active"`. Defer the multi-step paginated
   select-menu UX as out of scope.
3. /action ready: extended the existing `ActionHandler` rather than spawning
   a new handler file. Added a `ready` branch that splits off rawArgs as the
   trigger description and dispatches to `combat.Service.ReadyAction`.
4. Wire production via `cmd/dndnd/discord_handlers.go` — added
   `castLookupAdapter` (bridges combat.Service + queries to the cast
   provider interface, exposing `GetSpell` + `GetMapByID`) and
   `prepareEncounterProviderAdapter`. The existing `actionCombatServiceAdapter`
   gained a `ReadyAction` method. Both new handlers go through
   `attachCombatActionHandlers` (keeping the wiring centralized).
5. Router setters (`SetCastHandler`, `SetPrepareHandler`) added to
   `internal/discord/router.go` mirroring existing setters.
6. Slash command surface (`internal/discord/commands.go`): added the
   `ritual` boolean to `/cast`, and added `spells`/`class`/`subclass`
   string options to `/prepare`. The `/action` command already exposes the
   `args` option used by the `ready` branch as the trigger description.
7. Combat service exposes `GetCasterConcentrationName(ctx, casterID)` so the
   handler can populate `CastCommand.CurrentConcentration` before invoking
   `Cast`/`CastAoE` (the existing `lookupCasterConcentrationID` is unexported
   and returns the spell-ID, not the human-readable name needed for log
   parity).

## Files touched

- `internal/discord/cast_handler.go` (new)
- `internal/discord/cast_handler_test.go` (new)
- `internal/discord/prepare_handler.go` (new)
- `internal/discord/prepare_handler_test.go` (new)
- `internal/discord/action_handler.go` (added `ready` subcommand branch +
  `performReadyAction` helper; extended `ActionCombatService` interface
  with `ReadyAction`)
- `internal/discord/action_handler_test.go` (added `ReadyAction` mock and
  4 new tests for the ready path)
- `internal/discord/router.go` (`SetCastHandler`, `SetPrepareHandler`)
- `internal/discord/commands.go` (added `ritual` flag on `/cast`; added
  `spells`/`class`/`subclass` options on `/prepare`)
- `internal/discord/commands_test.go` (removed `prepare` from the
  no-options list now that it has options)
- `internal/combat/concentration.go` (new public `GetCasterConcentrationName`
  method)
- `cmd/dndnd/discord_handlers.go` (constructed `cast` + `prepare` handlers,
  added `castLookupAdapter` + `prepareEncounterProviderAdapter`, extended
  `actionCombatServiceAdapter` with `ReadyAction`, wired both handlers via
  `attachPhase105Handlers`)

## Tests added

- `TestCastHandler_DispatchesSingleTargetCast`
- `TestCastHandler_DispatchesAoECastForAreaSpell`
- `TestCastHandler_PassesSlotLevelAndMetamagic`
- `TestCastHandler_NoSpell`
- `TestCastHandler_NoActiveEncounter`
- `TestCastHandler_TurnGate_RejectsWrongOwner`
- `TestCastHandler_PostsToCombatLog`
- `TestCastHandler_TargetNotFound_SingleTarget`
- `TestCastHandler_AoENoTarget`
- `TestCastHandler_ServiceError`
- `TestCastHandler_AoEServiceError`
- `TestCastHandler_RitualFlag`
- `TestPrepareHandler_PreviewListsSpellsWhenSpellsArgEmpty`
- `TestPrepareHandler_CommitsSpellsWhenSpellsArgProvided`
- `TestPrepareHandler_RejectsActiveCombat`
- `TestPrepareHandler_NoEncounter_StillUsesCharacterFromCampaign`
- `TestPrepareHandler_NoPreparedCasterClass`
- `TestPrepareHandler_CharacterLookupError`
- `TestPrepareHandler_PrepareServiceError`
- `TestPrepareHandler_ExplicitClassOverride`
- `TestActionHandler_Ready_CallsReadyAction`
- `TestActionHandler_Ready_RequiresDescription`
- `TestActionHandler_Ready_RejectsNonOwner`
- `TestActionHandler_Ready_ServiceError`

`make cover-check` passes (overall 93.92%, per-package ≥85% on every
covered package).

## Implementation notes

- /cast vs /cast AoE dispatch is decided in the handler by inspecting
  `spell.AreaOfEffect.Valid` — no separate `--aoe` flag required. If the
  spell has area_of_effect data, AoE; otherwise single-target. This mirrors
  how the seed data already partitions spells (e.g. `fireball`, `cone-of-cold`
  vs `fire-bolt`, `cure-wounds`).
- Wall lookup for the AoE cover calculation is best-effort: missing map,
  parser failure, or DB error all degrade to `nil` walls (no cover bonus)
  rather than failing the cast. Keeps a misconfigured map from blocking
  combat.
- /prepare uses a flat text UX (preview ↔ `spells:id1,id2` commit). The
  paginated multi-step select-menu UX called for in spec lines 1018–1026 is
  explicitly deferred per task scope.
- The /prepare gate only triggers when the user passes `spells` (commit
  intent). Preview is allowed during active combat so a player can read
  the list without DM intervention.
- /action ready takes the trigger description from `rawArgs` directly so
  the existing `/action <action> <args>` shape keeps working — `/action ready
  args:"shoot if a goblin opens the door"`.
- Slot deduction + concentration linkage on ReadyAction (med-28),
  Counterspell prompt (med-29), interactive metamagic prompts (med-30),
  zone auto-creation in Cast (med-26), Silence cast-time rejection (med-25),
  and the multi-step /prepare select-menu UX are all out of scope; they live
  in their own task entries.
- /prepare takes the player's first prepared-caster class entry by default
  but accepts a `class:` override for multiclass characters (e.g. Cleric/Wizard).
- Added `combat.Service.GetCasterConcentrationName` rather than exporting
  the existing `lookupCasterConcentrationID` because the handler needs the
  human-readable spell name for log parity, not the spell ID. Keeps the
  ID-based lookup private.
- TurnGate is wired for /cast (it costs an action). /prepare is intentionally
  not gated since it runs out-of-combat — the in-handler `status="active"`
  rejection covers the active-combat case.

## Review (reviewer fills) — Verdict: PASS | REVISIT

STATUS: READY_FOR_REVIEW

## Review

Verdict: PASS

Reviewer: reviewer-C, rev=1, 2026-05-10.

All eight in-scope checks confirmed against the diff:

1. **/cast dispatch correct.** `cast_handler.go:138` branches on `spell.AreaOfEffect.Valid && len(spell.AreaOfEffect.RawMessage) > 0`. Single path calls `Service.Cast` (`:190`) with `combat.ResolveTarget` for the named target; AoE path calls `Service.CastAoE` (`:240`) with `renderer.ParseCoordinate`-derived `TargetCol`/`TargetRow`. TurnGate enforced at `:113` (`combat.IsExemptCommand("cast")` is false per `turnlock_integration_test.go:420`). Combat-log line posted via `postCombatLogChannel` (`:271`).
2. **/cast dice rolling.** Real `*dice.Roller` is plumbed top-to-bottom: `main.go:706` constructs `dice.NewRoller(nil)`, threaded through `deps.roller` → `NewCastHandler` (`discord_handlers.go:254`) → `Service.Cast(..., h.roller)`. `attachCombatActionHandlers` already guards `deps.roller == nil` at `:226`. Tests use a deterministic `dice.NewRoller(func(_ int) int { return 10 })`.
3. **AoE wall lookup graceful-degrades.** `loadWalls` (`cast_handler.go:255`) returns `nil` for any of: `!encounter.MapID.Valid`, `GetMapByID` error, `ParseTiledJSON` error. Misconfigured map cannot block a cast.
4. **/prepare preview-vs-commit gate correct.** `prepare_handler.go:91` reads `spells` arg; with empty arg it routes to `preview` (read-only `GetPreparationInfo` + `FormatPreparationMessage`). Active-encounter rejection at `:100` is correctly conditioned on `spellsArg != ""` — preview is allowed during active combat. Commit path (`:184` `commit`) is the only branch invoking `PrepareSpells`.
5. **/action ready uses turn-ownership gate.** `action_handler.go:168` `combatantBelongsToUser` rejects ("It's not your turn.") when the invoker's character does not match the current turn's combatant — same guard `/action freeform` and `/action cancel` use. ReadyAction can only fire on the caster's own turn (RAW). Empty trigger description rejected at `:210`. `TestActionHandler_Ready_RejectsNonOwner` covers the wrong-turn case.
6. **Router setters present.** `SetCastHandler` (`router.go:206`) and `SetPrepareHandler` (`:212`) mirror the existing `SetDeathSaveHandler` pattern. Both are invoked from `attachPhase105Handlers` (`discord_handlers.go:323/326`).
7. **Production wiring constructs both handlers with real deps.** `attachCombatActionHandlers` builds `castLookupAdapter` (combat.Service + queries + resolver, exposing `GetSpell` + `GetMapByID`) and `prepareEncounterProviderAdapter`. `actionCombatServiceAdapter.ReadyAction` added at `discord_handlers.go:642`. `cast.SetChannelIDProvider`, `cast.SetTurnGate` wired conditionally; `/prepare` deliberately not gated (handler-level status check covers it — note explains).
8. **Tests assert correct service dispatch + log.** `cast_handler_test.go` covers single-target-vs-AoE branching, slot-level/metamagic propagation, ritual flag, target-not-found, AoE-no-target, service errors, turn-gate rejection, and combat-log post-through. `prepare_handler_test.go` covers preview vs commit, active-combat rejection on commit, no-encounter fallback, non-prepared-caster, lookup error, service error, multiclass class override. `action_handler_test.go` adds 4 tests for the ready branch (calls service, requires desc, rejects non-owner, surfaces service error).
9. **Scope deferrals correct and documented.** `cast_handler.go:44–46` enumerates med-25 / med-26 / med-29 / med-30. `action_handler.go:200` enumerates med-28. `prepare_handler.go:48–49` defers paginated select-menu UX.
10. **`GetCasterConcentrationName` minimal.** Public method (`concentration.go:213`) only returns the human-readable spell name string from `ConcentrationSpellName`; underlying ID-based `lookupCasterConcentrationID` stays unexported.

Minor observations (non-blocking):
- `indexToCol` (`cast_handler.go:321`) only handles A–Z (single letter). For >26-column maps it produces garbage chars. Out of scope for crit-01b but worth tracking — the AoE path then hands `TargetCol="["`/junk to the service. Today's seed maps are well under 26 cols so this is latent.
- The `/action ready` description trim happens twice (in `Handle` at `:103` then `performReadyAction:210`). Harmless redundancy.
- Coverage already verified by orchestrator (overall 93.92%).
