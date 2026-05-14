# Playtest Readiness Report

Date: 2026-05-14T23:27+07:00
Reviewer: playtest_readiness (automated)

---

## 1. Checklist Scenario → Code Path → Test Coverage

| # | Scenario | Key Code Paths | Test / Harness |
|---|----------|---------------|----------------|
| 1 | Combat round + OA | `internal/combat/service.go` (move, OA trigger), `internal/discord/move_handler.go` | `internal/combat/service_lifecycle_test.go`, `internal/discord/sr007_card_update_test.go`, E2E replay harness |
| 2 | Spell with saving throw | `internal/combat/spell.go`, `internal/combat/damage.go` | `internal/combat/spell_test.go`, SR-013 AoE tests |
| 3 | Exploration → initiative | `internal/combat/service.go` (RollInitiative), encounter mode flip | `internal/combat/service_lifecycle_test.go` |
| 4 | Death save sequence | `internal/combat/deathsave.go` | `internal/combat/deathsave_test.go` |
| 5 | Short rest | `internal/discord/rest_handler.go` | `internal/discord/rest_handler_test.go`, card update test |
| 6 | Long rest | `internal/discord/rest_handler.go` (long path) | `internal/discord/rest_handler_test.go` |
| 7 | Loot claim | `internal/discord/loot_handler.go` | `internal/discord/sr007_card_update_test.go` (loot claim assertion) |
| 8 | Item give | `internal/discord/give_handler.go` | `internal/discord/sr007_card_update_test.go` (give assertion, 2 calls) |
| 9 | Attune / unattune | `internal/discord/attune_handler.go`, `unattune_handler.go` | `internal/discord/sr007_card_update_test.go`, `equip_handler_card_update_test.go` |
| 10 | Equip swap | `internal/discord/equip_handler.go`, `internal/combat/equip.go` | `internal/combat/equip_card_update_test.go`, `internal/discord/equip_handler_card_update_test.go` |
| 11 | Dashboard edit during combat | `internal/combat/workspace_handler.go`, advisory locks (Phase 27/103) | `cmd/dndnd/auth_dm_routes_test.go` (403 gating), `main_wiring_test.go` |

All 11 scenarios have unit-level test coverage for their critical code paths. No scenario-level E2E transcripts exist yet (all marked `pending` per H-121.4 deferral).

---

## 2. `make playtest-replay` Target

- **Exists:** Yes (`Makefile` line 56–61).
- **Default transcript:** `internal/playtest/testdata/sample.jsonl` (smoke: `/recap` on empty campaign).
- **Run result:** `PASS` (TestE2E_ReplayFromFile, 2.48s). Container lifecycle clean.
- **No transcripts under `docs/testdata/playtest/`** — transcripts live at `internal/playtest/testdata/`.
- **Scenario transcripts (1–11):** All pending; deferred per H-121.4 (requires live session to capture).

---

## 3. Build & Smoke Test

| Check | Result |
|-------|--------|
| `make build` | ✅ PASS — produces `bin/dndnd` + `bin/playtest-player` |
| `go test ./cmd/dndnd/ -short` | ✅ PASS (27.3s) |
| `make playtest-replay` | ✅ PASS (2.5s) |

Compilation is clean; no errors or warnings beyond known spell-data quality WARNs (cosmetic, non-blocking).

---

## 4. Discord Bot Intents & Handlers

Confirmed in `cmd/dndnd/discord_handlers.go` and `cmd/dndnd/main.go`:

- **HandleGuildCreate** — wired via `adder.AddHandler` (spec line 179, dynamic guild-join command registration). ✅
- **HandleGuildMemberAdd** — wired (spec lines 183–200, welcome DMs). ✅
- **InteractionCreate** — wired as the slash-command router shim dispatching through `CommandRouter`. ✅
- **Intents:** `IntentsGuildMembers` OR'd in per SR-003. ✅

Test: `TestWireBotHandlers_RegistersAllHandlers` (`main_wiring_test.go:1172`) asserts all three handler types are registered.

---

## 5. DM Auth Middleware Gating

- `mountDMOnlyAPIs` (main.go:305) wraps all DM-mutation handler groups in a `router.Group` with `dmAuthMw`.
- `mountCombatDashboardRoutes` is called **inside** that group — all Patch/Delete/Post combat routes inherit the middleware.
- Bare-router mounts: only `encounterHandler` (read-only player data), `assetHandler` (static assets), `open5eHandler` (SRD reference). Intentionally public per SR-001 reviewer notes.
- Test: `TestMountDMOnlyAPIs_NonDMReceives403` walks every DM route and asserts 403 for non-DM callers. ✅

**No unprotected mutation endpoints found.**

---

## 6. `OnCharacterUpdated` Fires from Every Non-Combat Mutator

Confirmed `notifyCardUpdate` (which calls `OnCharacterUpdated`) is invoked from:

| Handler | File | Call sites |
|---------|------|-----------|
| `/equip` | `equip_handler.go:295` | 1 |
| `/use` | `use_handler.go:186, 281` | 2 |
| `/give` | `give_handler.go:182, 183` | 2 (both parties) |
| `/loot` | `loot_handler.go:183` | 1 |
| `/attune` | `attune_handler.go:156` | 1 |
| `/unattune` | `unattune_handler.go:105` | 1 |
| `/rest` (short + long) | `rest_handler.go:334, 359, 597` | 3 |
| `/prepare` | `prepare_handler.go:210` | 1 |
| Level-up | `internal/levelup/service.go:152` | 1 |
| DM add/remove item | `internal/inventory/api_handler.go:88` | 1 |

Test: `sr007_card_update_test.go` asserts exact call counts per handler. ✅

---

## 7. Risks & Notes

1. **No real-session transcripts yet.** The 11 checklist scenarios are all `pending`. The replay harness works (proven by the smoke transcript), but scenario-specific regression coverage depends on the first live playtest session recording transcripts.
2. **Spell data quality warnings** (mass-cure-wounds, banishing-smite, etc.) are cosmetic and do not affect gameplay mechanics — these spells use non-standard resolution paths that the engine handles via fallback logic.
3. **`TestIntegration_MigrateDown`** is a known pre-existing failure unrelated to remediation (per regression report).

---

## VERDICT: GO

All critical code paths compile, are tested at the unit level, and pass. Discord handlers are wired. Auth middleware gates all mutations. `OnCharacterUpdated` fires from every non-combat mutator. The replay harness is functional. The only gap is the absence of captured scenario transcripts, which is expected (they require the live session this playtest will produce).
