# F group bundle — implementation worklog

Bundle: 9 Group-F tasks (mix of MINOR + MEDIUM). Date: 2026-05-11.
Implementer: opus-4.7.

## Task status

### F-78c-bonus-actions-schema — DONE

- New migration `db/migrations/20260511130000_add_bonus_actions_to_creatures.sql`
  adds the structured `bonus_actions JSONB` column on `creatures`.
- `db/queries/creatures.sql` UpsertCreature now writes the column.
- `sqlc generate` regenerated `internal/refdata/creatures.sql.go` + models.
- Added `combat.ResolveBonusActions(creature, abilities)` which prefers
  the structured column and falls back to `ParseBonusActions` when null /
  empty / malformed.
- Turn builder now calls `ResolveBonusActions` instead of going straight
  to `ParseBonusActions`. Goblin Nimble Escape behaviour is preserved by
  the fallback path so unimported rows keep working.
- Tests: `TestResolveBonusActions_*` (column-preferred, fallback empty,
  fallback null, fallback bad-json) + `TestBuildTurnPlan_UsesStructuredBonusActions`.

### F-81-targeted-check-handler — DONE

- New optional wiring on the `/check` handler:
  - `CheckTargetResolver` returns the caster's and target's combatants for
    a short ID in the active encounter.
  - `CheckTurnProvider` resolves the caster's active turn + persists action
    cost (reuses `combat.UseResource`/`TurnToUpdateParams`).
- Targeted-check pre-flight runs after the contested-check path returns
  ok=false. It (a) validates 5-ft Chebyshev adjacency, rejecting with a
  player-facing message; (b) if in combat with action available, deducts
  an Action via `UpdateTurnActions`; (c) skips deduction out of combat;
  (d) rejects when action already used.
- Tests: `TestCheckHandler_TargetedCheck_RejectsNonAdjacentTarget`,
  `_AdjacentInCombat_DeductsAction`, `_OutOfCombat_NoDeduction`,
  `_ActionAlreadyUsed_Rejects`, `_NoResolverWired_FallsThrough`.

### F-81-group-check-handler — DONE (dashboard route)

- New `internal/dashboard/check_handler.go` exposing
  `POST /api/encounters/{encounterID}/group-check`. Resolves every alive
  PC combatant, computes each character's skill modifier (`character.
  SkillModifier`), and runs `check.GroupCheck`. Returns aggregated
  per-PC roll + pass/fail + group success.
- Tests: happy path (deterministic d20=12 → all pass), no-participants 400,
  invalid encounter 400, missing skill 400, malformed JSON 400.
- Commands.go was kept untouched (per zone restrictions); DM flow is the
  dashboard endpoint per the F-81 task acceptance ("Discord OR dashboard").

### F-81-dm-prompted-checks — DONE

- New migration `20260511130002_create_pending_checks.sql` mirroring the
  `pending_saves` schema (encounter_id, combatant_id, skill, dc, reason,
  status, roll_result, success).
- New queries: `UpsertPendingCheck`, `GetPendingCheck`,
  `ListPendingChecksByCombatant`, `ListPendingChecksByEncounter`,
  `UpdatePendingCheckResult`, `ForfeitPendingCheck`.
- New dashboard route `POST /api/encounters/{encounterID}/prompt-check`
  persists a row so the player can resolve through `/check` after a
  restart (analog to `pending_saves`).
- Tests cover happy path + each 400 branch.

### F-86-item-picker-homebrew-flag — DONE

- `SearchResult` gains a `homebrew bool` field returned for every result.
- New `homebrew=true|false` query param filters server-side. Truthy
  values: true/1/yes; falsy: false/0/no; anything else returns both
  (preserves legacy callers).
- Armor refdata has no `homebrew` column today — armor rows always
  report `homebrew=false`. Weapons + magic_items honour the refdata
  column.
- Tests: surfaces flag, filter=true keeps only homebrew, filter=false
  keeps only official.

### F-86-item-picker-custom-entry — DONE

- New `POST /api/campaigns/{id}/items/custom` route bound to
  `Handler.HandleCustomEntry`. Accepts `{name, description, quantity,
  gold_gp, price_gp, type}` (name required, quantity defaults to 1, type
  defaults to "custom"). Returns a generated `custom-<uuid>` ID, all
  fields echoed, and `custom=true homebrew=true`.
- Tests: success, missing name 400, malformed JSON 400, default quantity
  + type.

### F-86-item-picker-narrative-price — DONE

- Picker contract now carries `description` (narrative) and `price_gp` /
  `gold_gp` (price override) directly on the custom-entry response. The
  search endpoint already echoed magic-item description; verified the
  field is preserved.
- Covered by `TestHandleCustomEntry_ReturnsPayload` (asserts both
  description and price round-trip).

### F-88c-detect-magic-environment — DONE

- New optional `CastNearbyInventoryScanner` interface on the cast
  handler. `ScanNearby(ctx, guildID, userID, radiusFt)` returns groups
  of `NearbyInventory{SourceName, Items}` for combatants / dropped loot
  within radius. `DetectMagicRadiusFt=30` constant matches PHB.
- `dispatchDetectMagic` now aggregates the caster's own inventory PLUS
  scanner results, filters each group through `inventory.DetectMagic
  Items`, and renders them as "On {self}" + "Nearby — {source}"
  sections. Scanner errors degrade silently (caster-only fallback).
- Tests: aggregates self + multiple nearby groups, drops non-magic
  items, no-nearby still reports self, scanner error degrades
  gracefully, totally-empty returns the "no auras" message.

### F-89d-asi-restart-persistence — DONE

- New migration `20260511130001_create_pending_asi.sql` →
  `pending_asi(character_id PK, snapshot_json, created_at, updated_at)`.
- Queries: `UpsertPendingASI`, `GetPendingASI`, `DeletePendingASI`,
  `ListPendingASI`.
- New `discord.ASIPendingStore` interface + `SetPendingStore` /
  `HydratePending` on `ASIHandler`. `storePendingChoice` upserts on
  Save; `removePendingChoice` deletes. Hydrate is called once at boot.
- Production adapter `asiPendingStoreAdapter` in `cmd/dndnd/
  discord_handlers.go` bridges `refdata.Queries` to the store interface.
  Wiring is a 2-line addition next to the existing FeatLister setter.
- Exposed `MarshalPendingASIChoice` / `UnmarshalPendingASIChoice` for
  test + adapter parity.
- Tests: store called on accept, store called on approve, hydrate
  repopulates after a restart, round-trip marshal.

## Side updates (in-zone)

- `internal/testutil/testdb.go` MutableTables: added `pending_checks`
  + `pending_asi` so the schema-vs-truncation guard recognises them.
- `internal/database/integration_test.go` MigrateDown list: prepended
  the three new migrations.

## Test / coverage / build

- `make test` (`go test ./...`): all packages green.
- `make cover-check`: overall + per-package thresholds hold (no
  regression — discord 86.57%, dashboard 91.59%, itempicker 97.87%,
  combat 91.96%, all ≥ 85% gate).
- `make build`: clean.
- `sqlc_drift_check` script test: clean (regeneration committed).
- `go vet ./...`: clean.
