# STEP-007 — Initiative seeding (DM starts combat → real `RollInitiative`)

> Lifecycle: **EXPLORE → AUTHOR → CRYSTALLIZE → AUTOMATED**. See
> [`../README.md`](../README.md) for how we work. This note is the per-step
> authoring record.

## Picked (QnA, 2026-06-24)

- **Step:** Initiative seeding — turn the "DM starts combat" moment into a real
  driven flow instead of the harness's hardcoded fake.
- **Scope:** **Full `Service.StartCombat`** (closest to the real
  `POST /api/combat/start` dashboard call) — build encounter-template seeding infra.
- **Determinism:** **Distinct DEX** so the asserted turn order *proves the sort
  reorders*, not just preserves insertion order.

## EXPLORE findings (code-confirmed)

**Initiative is NOT a slash command.** It is the dashboard REST flow
`POST /api/combat/start` (`internal/combat/handler.go:202`) →
`combat.Service.StartCombat` (`internal/combat/service.go:1038`). So the
crystallized artifact is a **Go `TestE2E_InitiativeScenario`** driving the real
service directly (the established pattern for DM/dashboard actions — see
STEP-003/004), **not** a `.jsonl` player transcript.

`StartCombat(ctx, input, roller)` orchestration (`service.go:1038-1110`):
1. `CreateEncounterFromTemplate` (`service.go:872`) — reads `encounter_templates`
   (`creatures` JSONB = `[]TemplateCreature`), creates the encounter (status
   `preparing`, round 0) + one NPC combatant per template creature
   (`GetCreature` stat block → `CombatantFromCreature`). Validates creature
   positions vs map bounds **only if** `template.map_id` is set.
2. Add PC combatants — `seatPCsInSpawnZones` (`service.go:998`) honors explicit
   `CharacterPositions`; `CombatantFromCharacter` pulls HP/AC from the character.
3. `markSurprisedByShortIDs` — no-op when `SurprisedShortIDs` empty.
4. `RollInitiative` (`initiative.go:274`) — `d20 + DexMod` per combatant
   (`getDexModifier`: char/creature `ability_scores`; NPC w/o creature ⇒ 0),
   `SortByInitiative` (**roll desc → dexMod desc → name asc → id asc**), writes
   `initiative_roll` + `initiative_order` (1..N), sets encounter `active` /
   round 1.
5. `AdvanceTurn` (`initiative.go:392`) — creates the first turn for the order-1
   combatant, sets `encounter.current_turn_id`.
6. Fires `turnStartNotifier.NotifyFirstTurn` (#your-turn ping) +
   `initiativeTrackerNotifier.PostTracker` (#initiative-tracker), returns
   `StartCombatResult{Encounter, Combatants, InitiativeTracker, FirstTurn}`.

**Deterministic dice:** harness roller = always-max (`e2eDefaultRoll`), so every
d20 = 20 ⇒ `Roll = 20 + DexMod`. Order therefore = **DEX desc, then name** (both
tiebreakers agree). To prove the sort reorders, seed combatants with distinct
DEX so a later-inserted combatant lands at order 1.

**Harness today fakes all this** via `PromoteEncounterToActive`
(`e2e_harness_test.go:330`): hardcoded `initiative_roll`=10/5, order 1/2, manual
turn holder. This step replaces the fake with the real flow (new
`TestE2E_InitiativeScenario`; existing attack/move/bonus scenarios keep the fast
fake — they don't test initiative).

## Infra to build (red → green)

1. **`main.go` seam** — `withCombatServiceReady(cb func(*combat.Service))`
   runOption + `runConfig.onCombatServiceReady`, fired next to `onRouterReady`
   (~`main.go:1838`, combatSvc fully wired by then). `main.go` is
   coverage-excluded; mirrors the existing `withCommandRouterReady`/`withRoller`
   seams. No-op in production.
2. **`e2eHarness.combatService`** field captured in `startE2EHarness`.
3. **`SeedEncounterTemplate(name, mapID, creatures...)`** → `CreateEncounterTemplate`.
4. **`SeedApprovedPlayerWithDex(userID, name, dex)`** — `NewTestCharacter` +
   raw-SQL `ability_scores` patch (the `SeedApprovedMonk` idiom).

## Expected outcome / assertions (to confirm with user before crystallizing)

Setup: 1 template goblin (NPC) + 2 PCs with distinct DEX; explicit PC positions
(no spawn-zone infra needed). Drive `StartCombat`.

- **Bram** DEX 20 (+5) ⇒ roll **25** ⇒ `initiative_order` **1** (turn holder)
- **Goblin** SRD DEX (+2) ⇒ roll 20+mod (read via `GetCreature`) ⇒ order **2**
- **Alice** DEX 6 (−2) ⇒ roll **18** ⇒ order **3**

(Bracket holds for any sane goblin DEX: 25 > goblin > 18.)

Assert (via `h.queries`, real DB read-back):
- `ListCombatantsByEncounterID` (orders by `initiative_order`) ⇒ `[Bram, Goblin, Alice]`
  with the exact rolls/orders above; `is_npc` correct; **proves the sort reordered**
  (Bram added *after* Goblin yet lands at order 1).
- Encounter: `status="active"`, `round_number=1`, `current_turn_id` non-nil.
- `GetTurn(current_turn_id).CombatantID == Bram` (== `result.FirstTurn.CombatantID`).
- `result.InitiativeTracker` contains `Round 1` + `🔔 @Bram — it's your turn!`.

## Observed + crystallized (2026-06-24) — AUTOMATED ✅

Drove the real `h.combatService.StartCombat` with the always-max roller. Observed
order (user-confirmed correct standard D&D):

| order | combatant | DEX | roll | is_npc |
| --- | --- | --- | --- | --- |
| 1 (turn holder) | Bram (PC) | 20 (+5) | 25 | false |
| 2 | Goblin (template NPC) | 14 (+2, SRD) | 22 | true |
| 3 | Alice (PC) | 6 (−2) | 18 | false |

Bram is added *after* the goblin yet sorts to order 1 → proves `SortByInitiative`
reorders. Encounter `active`/round 1, `current_turn_id`→Bram, tracker pings
`🔔 @Bram — it's your turn!`. **No bug found** — confirms current behavior.

- **Artifact:** `cmd/dndnd/e2e_scenarios_test.go` `TestE2E_InitiativeScenario`
  (Go scenario, no `.jsonl` — initiative is a dashboard/service action, not a
  player slash command).
- **Infra added:** `withCombatServiceReady` runOption + `runConfig.onCombatServiceReady`
  (`main.go`, fired next to `onRouterReady`); `e2eHarness.combatService`;
  `SeedEncounterTemplate`; `SeedApprovedPlayerWithDex`.
- **Verified:** new scenario PASS (first run); full e2e suite green (15 scenarios,
  ~39s); `make cover-check` green; gofmt/vet clean.

## Status

- 2026-06-24: AUTOMATED. Real StartCombat/RollInitiative locked end-to-end.
