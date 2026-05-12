# CMD-WIRE follow-up — reviewer worklog

Reviewer: Claude Opus 4.7 (1M context). READ-ONLY.
Working directory: /home/ab/projects/DnDnD.

## Verdicts

### C-DISCORD-followup-cmd-wire-setters — APPROVE
- `cmd/dndnd/discord_handlers.go:267-269` wires
  `handlers.action.SetStabilizeStore(deps.queries)` and (within
  `attachCombatActionHandlers`) `handlers.attack.SetMapProvider(deps.queries)`.
  `*refdata.Queries` exposes `UpdateCombatantDeathSaves` and `GetMapByID`
  structurally — confirmed.
- Wiring guards (`HasStabilizeStore`, `HasMapProvider`) exist on the
  handlers and the new tests assert them.

### AOE-CAST-followup-cmd-wire-setters — APPROVE
- `cmd/dndnd/discord_handlers.go`: `discordHandlerDeps.reactionPrompts`
  added; `attachCombatActionHandlers` calls `SetMaterialPromptStore(prompts)`
  and `SetAoESaveResolver(discord.NewAoESaveServiceAdapter(...))`.
- `cmd/dndnd/main.go:987-1080`: a single shared `NewReactionPromptStore`
  is constructed, passed into `buildDiscordHandlers`, AND registered on
  the `cmdRouter` via `SetReactionPromptStore` — so `rxprompt:*` button
  clicks route back to the same store the prompts registered with.
- Wiring tests cover both setters via `HasMaterialPromptStore` /
  `HasAoESaveResolver`.

### D-48b-49-51-followup-discord-prompts — APPROVE
- `internal/discord/attack_handler.go`: new
  `AttackClassFeatureService` interface (slice of `*combat.Service`),
  `SetClassFeaturePromptPoster` + `SetClassFeatureService` setters,
  `postClassFeaturePrompts` helper, `resolvePromptChannel`.
- `Handle` and `dispatchOffhand` both invoke `postClassFeaturePrompts`
  with the encounter so offhand hits also surface prompts.
- Reads `PromptStunningStrikeEligible`, `PromptDivineSmiteEligible`/
  `PromptDivineSmiteSlots`, `PromptBardicInspirationEligible`/
  `PromptBardicInspirationDie` per spec.
- Post-hit tests in
  `internal/discord/attack_handler_post_hit_prompts_test.go` cover all
  three prompts plus the negative no-prompt case AND a Use-Ki click that
  drives `HandleComponent` and asserts `StunningStrike` was called with
  the right attacker/target/CurrentRound.

## Verification gates

- `make build` — clean.
- `make test` — clean (no FAIL lines).
- `make cover-check` — OK; overall 92.47%, `internal/discord` 86.51%
  (>85%), every per-package threshold met.
- `go vet ./...` — not re-run; build is the stricter signal.

## Findings

- Red-before-green not directly verifiable from history (test files are
  untracked alongside the wiring), but the `Has*` accessors only flip
  to true with the wiring changes and the post-hit prompt tests rely on
  the new helpers — they would fail without the implementation.
- Out-of-zone: lifecycle adapters (`lifecycle_adapters.go`), magicitem
  publisher, ASI pending-store, B-26b notifiers, COMBAT-MISC zone
  lookups, D-54 speed lookup, C-43 medicine lookup all landed in the
  same diff. Out of scope for this bundle but no regressions observed.
