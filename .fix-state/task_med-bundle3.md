# task med-bundle3 — Final medium-tier push

You are the final implementer for the DnDnD remediation campaign. Close as much of the remaining ledger as possible. The orchestrator has already closed 39/51 tasks across 14 commits; what remains is interactive Discord features + class-feature prompt-UIs + a few small wiring fixes.

## Findings (in order of likely difficulty — start easiest)

### EASY: med-35-residue — /use and /give combat resource costs
> "/use and /give combat costs explicitly deferred at phases.md:485 but no follow-up phase tracks it."

Fix: in `internal/discord/use_handler.go`, deduct a bonus action from the active turn before completing a /use of a potion. In `internal/discord/give_handler.go`, deduct the free-interaction resource via `combat.UseResource(turn, ResourceFreeInteract)`. Reject when the resource is already spent.

### EASY: med-38-residue — magicitem publisher
> "Phase 104b publisher fan-out partial — `inventory` and `levelup` wired; `rest.Service` and `magicitem.Service` never constructed in `main.go`."

Investigate: `rest.Service` is functional (no state); `magicitem` has no `Service` struct. The chunk's recommendation may be stale.

Fix: confirm whether either needs a publisher hook by checking if their mutation surface (long rest, magic-item activation) currently emits the Phase 104b dashboard event. If not, add the publisher call at the existing emission point (`internal/discord/rest_handler.go` after rest succeeds; `inventory.Service.UseActiveAbility` after charge consumed). If the existing dashboard publisher is wired through other surfaces (combat, levelup) and rest/magicitem are minor, mark as `Verdict: not-applicable` with rationale.

### MEDIUM: med-24 — Phase 55 OAs invoked from /move
> "DetectOpportunityAttacks and OATrigger are only referenced inside `internal/combat/opportunity_attack*.go`. `internal/discord/move_handler.go` never calls them, so OAs do not fire on movement."

Fix: in `internal/discord/move_handler.go` after `ValidateMove` returns the path, call `combat.DetectOpportunityAttacks(...)`. For each trigger, post `combat.FormatOAPrompt` to the hostile player's `#your-turn` channel via the existing channel-id provider (the same one used for combat-log). Use the equipped weapon's `reach` property for PCs (default 5ft).

### MEDIUM: med-27 — Phase 68 FoW: explored history + RenderMap from production
> "Explored (dim) state never set. RenderMap is never called from production."

Fix: smallest viable. Add an in-memory `exploredCells map[uuid.UUID]map[Coord]bool` to `mapRegeneratorAdapter` (in `cmd/dndnd/discord_adapters.go`); after each `RenderMap` call, union the currently-visible cells into the map and pass them as `Explored` on the next render. The `mapRegeneratorAdapter` already calls renderer.RenderMap per high-10; just wire the explored-state plumbing.

(Skip the bright/dim two-range model unless trivial — that's a deeper renderer change.)

### MEDIUM: med-43-residue — Stunning Strike + Smite + Uncanny Dodge prompts + Bardic 30s timeout
> "Stunning Strike, Divine Smite, Uncanny Dodge auto-prompts missing entirely. Bardic Inspiration 30s/10min timeouts unwired."

Fix: the spec-mandated post-melee-hit prompts are large. Pick the cleanest minimal version:

1. After `Service.Attack` returns a successful melee hit, check the result's `ProcessorResult.ResourceTriggers` (already collected in effect.go:374) for "ki", "smite-slot", or similar. If present, post a Discord ephemeral prompt to the attacker's user with [Use] / [Skip] buttons. On click, dispatch to the corresponding service method (`StunningStrike`, `DivineSmite`).
2. Uncanny Dodge: in `Service.applyDamageHP` (or the wrapper `ApplyDamage`), if target is a Rogue with reaction available and incoming damage is non-AoE, post a prompt; on click, halve the damage.
3. Bardic Inspiration 30s/10min: the Bardic sweep is already wired (med-43-B done). The 30-second usage prompt is a separate UI feature — defer if time-constrained.

If the Discord prompt mechanism doesn't exist yet, build a minimal helper that:
- Posts an ephemeral interaction with buttons
- Stores a pending action keyed by (encounterID, combatantID, action-name) with a TTL
- Routes the button-click back to the service method
- Times out after the configured window

If this exceeds scope, mark as `BLOCKED: requires reaction-prompt UI helper not yet built` and skip.

### HARD: med-29 prompt UI + auto-timeout
> "No Discord-side player prompt UI for counterspell. No auto-timeout."

Fix: build the Discord-side prompt for counterspell (slot-level buttons + [Pass] in `#your-turn`). Same minimal helper pattern as med-43 above. Auto-timeout via `time.AfterFunc` calling `ForfeitCounterspell` after the turn-timer window.

### HARD: med-30 metamagic prompts
> "Empowered, Careful, Heightened all expect interactive button prompts."

Fix: use the same prompt helper (if built for med-43/med-29). Empowered: button menu listing each die value to reroll. Careful: button menu listing AoE creatures to protect. Heightened: button menu listing AoE targets to heighten. Each returns to the cast pipeline.

If the prompt helper isn't built yet, mark as BLOCKED behind med-29.

## Workflow

1. Process EASY findings first (med-35, med-38).
2. Then MEDIUM (med-24, med-27).
3. Then HARD (med-43-residue → med-29 prompt UI → med-30) — close as many as fit.
4. Per finding: TDD red→fix→green; run targeted package tests.
5. After all findings: `make cover-check`.
6. Append per-finding plan/files/tests/notes to this task file.

## Constraints

- NO git commits.
- Match existing patterns. Don't introduce new abstractions for hypothetical futures.
- Early-return style.
- BLOCKED: <reason> if a finding needs major missing infrastructure.
- If you hit org usage limit mid-run, append a STATUS line and stop gracefully.

When done (or when you can't continue), append `STATUS: READY_FOR_REVIEW` as the final line of the task file.

## Plan / Files / Tests / Notes (per finding, worker fills below)

### med-35-residue — Plan / Files / Tests / Notes

**Plan**: Wire combat-time resource costs into `/use` and `/give`. Both
handlers gain an optional turn provider (`UseCombatProvider` extended with
`UpdateTurnActions`; new `GiveTurnProvider` interface) plus a
`useGiveTurnAdapter` in cmd/dndnd that joins (campaign → character → active
encounter → current turn) via the existing
`GetActiveEncounterIDByCharacterID` query. `inventory.IsPotion(itemID)` is a
new exported helper that classifies the auto-resolve healing items so /use
deducts a bonus action for potions and an action for everything else
(magic-item charges + DM-adjudicated consumables). Out-of-combat /use and
/give carry no cost. When a resource is already spent the handler rejects
the command before any inventory mutation. Persistence is best-effort: a
turn-update failure does not undo the committed inventory change.

**Files**:
- `/home/ab/projects/DnDnD/internal/inventory/service.go` — `IsPotion` exported helper.
- `/home/ab/projects/DnDnD/internal/discord/use_handler.go` — `UseCombatProvider` extended with `UpdateTurnActions`; `lookupActiveTurn` + `spendTurnResource` helpers; potion → ResourceBonusAction, else → ResourceAction; magic-item charge path also deducts ResourceAction.
- `/home/ab/projects/DnDnD/internal/discord/use_handler_test.go` — five new tests (potion-in-combat, potion-already-spent, potion-out-of-combat, magic-item-in-combat, magic-item-already-spent).
- `/home/ab/projects/DnDnD/internal/discord/give_handler.go` — `GiveTurnProvider` interface + `SetTurnProvider`; `lookupActiveTurn` + `spendFreeInteract`; ResourceFreeInteract validation/deduction.
- `/home/ab/projects/DnDnD/internal/discord/give_handler_test.go` — three new tests (in-combat-deducts, already-spent-rejected, out-of-combat-no-cost).
- `/home/ab/projects/DnDnD/cmd/dndnd/discord_handlers.go` — `useGiveTurnAdapter` (sql/errors imports) wired into `NewUseHandler` constructor and `handlers.give.SetTurnProvider`.

**Tests**: `TestUseHandler_PotionInCombat_DeductsBonusAction`,
`TestUseHandler_PotionInCombat_BonusActionAlreadySpent_Rejected`,
`TestUseHandler_PotionOutOfCombat_NoCost`,
`TestUseHandler_MagicItem_InCombat_DeductsAction`,
`TestUseHandler_MagicItem_InCombat_ActionAlreadySpent_Rejected`,
`TestGiveHandler_InCombat_DeductsFreeInteraction`,
`TestGiveHandler_InCombat_FreeInteractionAlreadySpent_Rejected`,
`TestGiveHandler_OutOfCombat_NoCost`.

**Notes**: `potion_bonus_action` campaign-setting plumbing is still
deferred — the spec mentions it as an alternative cost mode but the field
at `internal/campaign/service.go:27` has no read site yet. Adding the
campaign-settings read into the use/give handlers is a follow-up. /give
adjacency check (target within 5 ft) is also still deferred per
chunk-6 review item #3 — the bundle3 task scoped only the resource cost.

### med-38-residue — Verdict: not-applicable

Per investigation in bundle2 + this bundle: `rest.Service` is purely
functional (no store dependency, no SetPublisher hook), and `magicitem`
exposes only standalone helper functions (no Service struct, no mutating
surface). The Phase 104b dashboard publisher fan-out hooks for both surfaces
already exist in adjacent paths:
- Long-rest character mutations flow through `internal/discord/rest_handler.go`
  which directly calls character-update queries; the dashboard publisher
  attaches to `inventory.APIHandler` (DM-side mutations) and
  `levelup.Service.SetPublisher` (level-ups). Wiring a publisher into the
  player-rest discord handler would require injecting an
  EncounterPublisher + EncounterLookup into rest_handler.go — see bundle2
  med-38 BLOCKED rationale for the multi-PR refactor footprint.
- Magic-item active-ability charge consumption flows through
  `inventory.UseCharges` from `/use` (this bundle's med-35 wired the
  combat-cost path). The charge-deduction publish path is the same
  refactor problem — `discord.UseHandler` writes the inventory directly
  via UseCharacterStore, bypassing inventory.APIHandler.

Verdict: **not-applicable** — the publisher fan-out exists where it
naturally hangs (inventory APIHandler, levelup Service); rest+magicitem
mutations happen in handler code that doesn't own a publisher hook today
and adding one is a multi-PR refactor (separate plumbing for the
player-rest path AND the discord /use magic-item path). Recommend tracking
under a dedicated phase rather than continuing to fan out from the
package-level service stubs that don't exist.

### med-24 — Plan / Files / Tests / Notes

**Plan**: Wire `combat.DetectOpportunityAttacks` into the `/move` confirm
flow so OAs fire after a player commits a move that exits a hostile's
reach. Four nil-safe collaborators are bolted onto `MoveHandler` via a
single setter (`SetOpportunityAttackHooks`):
1. `MoveOATurnsLookup` — fetches the current round's turns keyed by
   combatant_id (so DetectOpportunityAttacks's reaction-used filter works).
2. `MoveOACreatureLookup` — resolves NPC creature attacks JSON for the
   max-reach lookup that `resolveHostileReach` already uses.
3. `MoveOAPCWeaponReach` — returns 10ft when the PC's equipped main-hand
   weapon has the "reach" property, 5ft otherwise. Wired through a new
   `combat.DetectOpportunityAttacksWithReach` variant that takes a
   per-hostile reach override map (PC-side, since the existing
   creatureAttacks map is keyed by creature_ref_id).
4. `MoveChannelIDProvider` — the existing CampaignSettingsProvider used by
   `done_handler` for #your-turn and #combat-log resolution.

`HandleMoveConfirm` calls `fireOpportunityAttacks` after a successful
position update. The helper re-pathfinds (best-effort, falling back to a
2-point straight segment), runs `DetectOpportunityAttacksWithReach` against
the live combatants, and posts each `FormatOAPrompt` line to the
encounter's #your-turn channel. Best-effort throughout: any wiring failure
silently degrades.

**Files**:
- `/home/ab/projects/DnDnD/internal/combat/opportunity_attack.go` — new `DetectOpportunityAttacksWithReach(...pcReachByID)` variant; existing `DetectOpportunityAttacks` delegates to it for backward-compat.
- `/home/ab/projects/DnDnD/internal/combat/opportunity_attack_test.go` — `TestDetectOpportunityAttacksWithReach_PCReachWeaponOverride` covering both with-and-without-override branches.
- `/home/ab/projects/DnDnD/internal/discord/move_handler.go` — four interfaces (`MoveOATurnsLookup`, `MoveOACreatureLookup`, `MoveOAPCWeaponReach`, `MoveChannelIDProvider`); `SetOpportunityAttackHooks` setter; `fireOpportunityAttacks`, `buildOAPath`, `lookupHostileTurns`, `lookupCreatureAttacks`, `lookupPCReach` helpers; called from `HandleMoveConfirm` after the position update.
- `/home/ab/projects/DnDnD/internal/discord/move_handler_test.go` — extended `mockMoveSession.ChannelMessageSend` to capture sends; `moveOATurnsStub` / `moveOACreatureStub` / `moveOAPCReachStub` / `moveOAChannelStub` mocks; `TestMoveHandler_HandleMoveConfirm_OAFiresToYourTurnChannel` and `TestMoveHandler_HandleMoveConfirm_OASilentWhenChannelsUnset`. Combat package import added.
- `/home/ab/projects/DnDnD/cmd/dndnd/discord_handlers.go` — `moveOATurnsAdapter` (re-keys `ListTurnsByEncounterAndRound` by combatant_id), `moveOACreatureAdapter`, `moveOAPCReachAdapter` (reads `EquippedMainHand` → GetWeapon → checks `reach` property); single `SetOpportunityAttackHooks` call wires all four when both `deps.queries` and `deps.campaignSettings` are non-nil.

**Tests**: `TestDetectOpportunityAttacksWithReach_PCReachWeaponOverride`,
`TestMoveHandler_HandleMoveConfirm_OAFiresToYourTurnChannel`,
`TestMoveHandler_HandleMoveConfirm_OASilentWhenChannelsUnset`.

**Notes**: Reach-weapon resolution treats ranged weapons as 5ft (since they
don't trigger melee OAs at all). The OA detector currently faction-filters
by `IsNpc == mover.IsNpc`, so an OA can never fire against a same-faction
combatant — `lookupPCReach` short-circuits in that case to avoid wasted DB
calls. Hostile-PCs without an equipped main-hand weapon default to 5ft.

### med-27 — Plan / Files / Tests / Notes

**Plan**: Add explored-history to `mapRegeneratorAdapter` per chunk-5 + the
bundle3 task. A per-encounter set of seen tile indexes (row*width+col)
accumulates across renders. Before each render, any previously-seen tile
that's not currently Visible is upgraded from Unexplored → Explored so the
renderer paints it with the dim overlay. After each render, all currently-
Visible tiles are unioned into the set. Skipped med-27's
`BrightTiles + DimTiles` two-range light model — that requires a deeper
renderer change (per the bundle3 task: "Skip the bright/dim two-range model
unless trivial"). RenderMap from production: `mapRegeneratorAdapter` already
calls `renderer.RenderMap` per high-10. Vision-source plumbing into
`MapData.VisionSources` is still missing in production (chunk-5 separate
gap), so the explored-history path is dormant until a future phase wires
vision sources from combatant tokens — the helpers are unit-tested
end-to-end and ready.

**Files**:
- `/home/ab/projects/DnDnD/cmd/dndnd/discord_adapters.go` — `mapRegeneratorAdapter` gains `exploredMu sync.Mutex` + `exploredCells map[uuid.UUID]map[int]bool`; `RegenerateMap` pre-computes FoW + applies explored history before `renderer.RenderMap` and records visible tiles after; `applyExploredHistory` + `recordVisibleTiles` helpers.
- `/home/ab/projects/DnDnD/cmd/dndnd/main_wiring_test.go` — `TestMapRegeneratorAdapter_ExploredHistory_UnionsAcrossRenders` directly exercises both helpers end-to-end with synthetic FogOfWar grids; renderer import added.

**Tests**: `TestMapRegeneratorAdapter_ExploredHistory_UnionsAcrossRenders`.

**Notes**: The two-range light model (BrightTiles vs DimTiles on
LightSource) is genuinely a bigger renderer change touching
`ComputeVisibilityWithLights`, the LightSource struct itself, and the fog
state machine — DEFERRED as out-of-scope per bundle3's "Skip the
bright/dim two-range model unless trivial" guidance.

### med-43-residue — BLOCKED

Stunning Strike + Divine Smite + Uncanny Dodge auto-prompts and Bardic
Inspiration 30-second usage timeout require a Discord-side reaction-prompt
UI helper that does not yet exist. The minimum helper would need:
- Ephemeral interaction post with [Use] / [Skip] buttons keyed by
  (encounterID, combatantID, action-name).
- A pending-action store with TTL.
- A button-click router dispatching back to the corresponding service
  method (`StunningStrike`, `DivineSmite`, `ApplyUncannyDodge`, etc.).
- A `time.AfterFunc` per pending prompt that auto-times-out.
- Per-feature integration: post-Service.Attack hook for melee Monk hits
  (Stunning Strike) and melee Paladin hits (Divine Smite); a new
  applyDamageHP hook for Rogue reaction (Uncanny Dodge); a Bardic
  Inspiration grant hook for the 30-second usage prompt.

Bundle2 already shipped Wild Shape spellblock + auto-revert + Rage
auto-end + Bardic 10-min sweep (Priority A + part of B). The remaining
features (Priority B Stunning Strike + Priority C Divine Smite + Uncanny
Dodge) all share this infrastructure gap. Building the helper plus
integrating into 4+ pipelines + tests is ~6-8 hours, exceeding bundle3's
remaining time budget.

BLOCKED: requires Discord-side reaction-prompt UI helper not yet built.
Recommend tracking under a dedicated reaction-prompt-helper phase (e.g.
"reaction-prompt-helper-01") that builds the helper once and then a
follow-up phase per feature (Stunning Strike, Divine Smite, Uncanny Dodge,
Bardic 30s prompt) wires each into the helper.

### med-29 prompt UI — BLOCKED

Discord counterspell prompt UI + auto-timeout. Same infrastructure gap as
med-43-residue. The service surface (`TriggerCounterspell` returning
`CounterspellPrompt`, `ResolveCounterspell`, `ForfeitCounterspell`) is
ready and Subtle-aware (bundle2 wired med-29-Subtle). The missing piece is
the Discord-side prompt: posting the prompt to #your-turn with slot-level
buttons + [Pass], routing button clicks back to ResolveCounterspell, and
firing `time.AfterFunc(turnTimeout, ForfeitCounterspell)`. The current
counterspell flow is HTTP-only (DM dashboard at
`internal/combat/handler.go:61` triggers it).

BLOCKED: requires Discord-side reaction-prompt UI helper not yet built.
Same recommendation as med-43-residue.

### med-30 — BLOCKED

Empowered / Careful / Heightened metamagic interactive prompts. Same
infrastructure gap. Each metamagic option needs a per-target button menu
(Empowered: per-die rerolls; Careful: per-creature protection toggles;
Heightened: per-target heighten selection). Service-level data flows already
exist (`EmpoweredRerolls`, `CarefulSpellCreatures`, `IsHeightened` plumbed
through the cast pipeline per bundle2's med-30 deferral note).

BLOCKED: requires Discord-side reaction-prompt UI helper not yet built.
Same recommendation as med-43-residue + med-29.

STATUS: READY_FOR_REVIEW
