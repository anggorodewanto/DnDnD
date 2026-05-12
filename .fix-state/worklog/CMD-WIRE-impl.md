# CMD-WIRE follow-up — implementer worklog

Implementer: Claude Opus 4.7 (1M context).
Working directory: /home/ab/projects/DnDnD.
Date: 2026-05-11.

Bundle: production-handler wiring follow-ups from C-DISCORD,
AOE-CAST, and D-48b/49/51 (3 task IDs).

## Per-task status

### C-DISCORD-followup-cmd-wire-setters — DONE
- `cmd/dndnd/discord_handlers.go`:
  - `buildDiscordHandlers` now calls
    `handlers.attack.SetMapProvider(deps.queries)` — `*refdata.Queries`
    already exposes `GetMapByID` so it structurally satisfies the
    `discord.AttackMapProvider` interface.
  - `buildDiscordHandlers` now calls
    `handlers.action.SetStabilizeStore(deps.queries)` — `*refdata.Queries`
    already exposes `UpdateCombatantDeathSaves` and structurally satisfies
    the `discord.ActionStabilizeStore` interface.
- Tests (new): `cmd/dndnd/discord_handlers_wiring_test.go::
  TestBuildDiscordHandlers_AttackHandlerHasMapProvider` and
  `_ActionHandlerHasStabilizeStore`. Each spins a real Postgres-backed
  refdata.Queries, constructs the handler set, and asserts the optional
  setter was invoked via newly-added `Has*` accessors on the handlers.

### AOE-CAST-followup-cmd-wire-setters — DONE
- `cmd/dndnd/discord_handlers.go`:
  - `discordHandlerDeps` gained a `reactionPrompts *discord.ReactionPromptStore`
    field. The constructor builds a fresh per-call store when the deps field
    is nil so unit-test wiring still exercises the production setters.
  - `handlers.cast.SetMaterialPromptStore(prompts)` (E-63 follow-up) and
    `handlers.save.SetAoESaveResolver(discord.NewAoESaveServiceAdapter(
    deps.combatService, deps.roller))` (E-59 follow-up) are now wired
    unconditionally inside `attachCombatActionHandlers`.
- `cmd/dndnd/main.go`:
  - A single shared `discord.NewReactionPromptStore(discordSession)` is now
    created in `runWithOptions`, passed into `buildDiscordHandlers` via
    `discordHandlerDeps.reactionPrompts`, AND registered on the
    `cmdRouter` via `SetReactionPromptStore` so button clicks with the
    `rxprompt:*` CustomID prefix route back to the registered OnChoice
    closures. The store is shared between the gold-fallback prompt and the
    class-feature prompts so there's exactly one routing destination.
- Tests (new): `TestBuildDiscordHandlers_CastHandlerHasMaterialPromptStore`,
  `TestBuildDiscordHandlers_SaveHandlerHasAoESaveResolver`.

### D-48b-49-51-followup-discord-prompts — DONE
- `internal/discord/attack_handler.go`:
  - New `AttackClassFeatureService` interface exposing
    `StunningStrike` / `DivineSmite` / `UseBardicInspiration`. `*combat.Service`
    satisfies it structurally.
  - New `SetClassFeaturePromptPoster(*ClassFeaturePromptPoster)` and
    `SetClassFeatureService(AttackClassFeatureService)` setters, plus the
    `HasClassFeaturePromptPoster()` / `HasMapProvider()` accessors used by
    production-wiring tests.
  - `Handle` now invokes a new `postClassFeaturePrompts` helper after the
    main-hand attack returns and the offhand-dispatch path threads the
    encounter so it can fire the same helper. The helper reads
    `AttackResult.PromptStunningStrikeEligible`,
    `PromptDivineSmiteEligible` (+ `PromptDivineSmiteSlots`), and
    `PromptBardicInspirationEligible` (+ `Die`), then posts the
    corresponding ReactionPromptStore button rows. Each OnChoice closure
    invokes the wired class-feature service (skip / forfeit consumes no
    resources) and mirrors the resulting combat-log line to #combat-log.
  - New `resolvePromptChannel` mirrors `cast_handler.resolvePromptChannel`
    so the prompt lands in #combat-log when wired, falling back to the
    interaction's channel.
- `cmd/dndnd/discord_handlers.go`:
  - `attachCombatActionHandlers` now calls
    `handlers.attack.SetClassFeaturePromptPoster(discord.NewClassFeaturePromptPoster(prompts))`
    and `handlers.attack.SetClassFeatureService(deps.combatService)`.
- Tests (new):
  - `internal/discord/attack_handler_post_hit_prompts_test.go::
    TestAttackHandler_MonkHit_PostsStunningStrikePrompt`
  - `_NotEligibleForStunningStrike_NoPrompt`
  - `_PaladinHit_PostsDivineSmitePrompt`
  - `_BardicInspirationHolder_PostsBardicPrompt`
  - `_StunningStrike_UseKi_InvokesService` (clicks the Use Ki button
    through `h.classFeaturePrompts.prompts.HandleComponent` and asserts
    the mock `StunningStrike` was called with `Attacker.DisplayName`,
    `Target.DisplayName`, and the correct `CurrentRound`).
  - `cmd/dndnd/discord_handlers_wiring_test.go::
    TestBuildDiscordHandlers_AttackHandlerHasClassFeaturePromptPoster`.

## Files touched
- `internal/discord/attack_handler.go` — new interface,
  `SetClassFeaturePromptPoster`, `SetClassFeatureService`,
  `postClassFeaturePrompts`, `resolvePromptChannel`, `HasMapProvider`,
  `HasClassFeaturePromptPoster`. dispatchOffhand signature extended with
  `encounter` so offhand hits also fire the post-hit prompts.
- `internal/discord/action_handler.go` — `HasStabilizeStore` accessor.
- `internal/discord/cast_handler.go` — `HasMaterialPromptStore` accessor.
- `internal/discord/save_handler.go` — `HasAoESaveResolver` accessor.
- `internal/discord/attack_handler_post_hit_prompts_test.go` (NEW).
- `cmd/dndnd/discord_handlers.go` — `discordHandlerDeps.reactionPrompts`
  field, five new setter calls (map provider, stabilize store,
  material prompt store, AoE save resolver, class-feature prompt poster +
  service).
- `cmd/dndnd/discord_handlers_wiring_test.go` (NEW) — five wiring
  follow-up tests + a shared `buildDiscordHandlersForWiring(t)` helper.
- `cmd/dndnd/main.go` — shared ReactionPromptStore construction and
  `cmdRouter.SetReactionPromptStore(reactionPrompts)`.

## Gates

- `make test` — green.
- `make cover-check` — 92.47% overall (threshold 90%); every per-package
  threshold met. internal/discord at 86.58% (threshold 85%); combat
  unchanged.
- `make build` — clean.
- `go vet ./...` — clean.

## Risks / notes

- The new `AttackClassFeatureService` interface is non-intrusive: it
  only widens the AttackHandler's surface, not the existing
  `AttackCombatService` interface. `*combat.Service` satisfies it
  structurally so the wiring uses `deps.combatService` directly.
- `discord_handlers.go::attachCombatActionHandlers` builds a fresh
  `ReactionPromptStore` when `deps.reactionPrompts` is nil so the
  existing unit-test deps (which do NOT pass a store) still exercise the
  setters and the `Has*` assertions return true. Production main.go
  threads a single shared store through both the cast handler and the
  router so button clicks route correctly.
- `dispatchOffhand` signature gained an `encounter` parameter so both
  attack paths fire the post-hit prompts. Internal-only — no external
  caller exists.
- The offhand path tests already cover the basic dispatch; the new
  post-hit prompt tests exercise the main-hand path only because the
  offhand attack uses the same helper.

## Out-of-zone / not touched

- `internal/combat/*` — closed surface; the AttackClassFeatureService
  interface mirrors existing `*combat.Service` methods exactly.
- `internal/dashboard/*` — no DM-side wiring landed in this bundle.
  AoE NPC saves still need the future dashboard PATCH endpoint to call
  `ResolveAoEPendingSaves` (see AOE-CAST worklog for the contract).
