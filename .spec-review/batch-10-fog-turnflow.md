# Batch 10: Fog of war, obscurement, turn flow (Phases 68–80)

## Summary

The five mechanical pillars — turn timeout escalation (76a/76b), player /done flow (77),
enemy/NPC dashboard turn builder + bonus-action parsing + legendary/lair (78a/78b/78c),
summons (79), and /recap (80) — are largely implemented in the Go service layer with
solid unit/integration coverage. Obscurement mechanics (69) are wired into the
advantage/disadvantage engine and check pipeline.

The **fog-of-war pipeline (68) is the one large gap**: the underlying shadowcasting
algorithm, magical-darkness demotion, light sources, explored-history bookkeeping,
and DM-sees-all flag all exist in `internal/gamemap/renderer`, but the production
map-regeneration adapter at `cmd/dndnd/discord_adapters.go` never populates
`md.VisionSources`, `md.LightSources`, `md.MagicalDarknessTiles`, or `md.DMSeesAll`.
Consequently the fog overlay is never drawn in live combat. Static lighting tiles
painted by the Svelte map editor's lighting brush are also never read by the Go
renderer or surfaced to combat-side `CombatantObscurement` (zone-based obscurement
works for spell zones in `encounter_zones`, but not for the per-tile static lighting
gids 1–5 in the map JSON).

Recap is correctly an on-demand player command (not a per-round auto-emit), matching
the spec at lines 2057–2083 — the "Round Recap" mention in the batch prompt was not
in the spec.

## Per-phase findings

### Phase 68 — Dynamic Fog of War
- Status: **Partial (algorithm complete, production wiring missing)**
- Key files:
  - `internal/gamemap/renderer/fow.go` — symmetric raycast shadowcasting
  - `internal/gamemap/renderer/fog_types.go` — `VisionSource`, `FogOfWar`,
    `ComputeVisibilityWithZones`, Devil's-Sight & darkvision in magical darkness
  - `internal/gamemap/renderer/fog.go` — three-state overlay rendering + token filter
  - `internal/gamemap/renderer/renderer.go` — auto-compute path + DMSeesAll propagation
  - `cmd/dndnd/discord_adapters.go:417–514` — `RegenerateMap` + in-memory explored history
- Findings:
  - Algorithm is symmetric (raycasting on wall segments) and tested for the symmetry
    invariant; matches Albert Ford's intent. Three states (Unexplored/Explored/Visible)
    rendered correctly.
  - `VisionSource` carries `RangeTiles`, `DarkvisionTiles`, `BlindsightTiles`,
    `TruesightTiles`, `HasDevilsSight` — matches spec catalog of vision modifiers.
  - **Critical gap**: `discord_adapters.go:454` calls
    `ComputeVisibilityWithLights(md.VisionSources, md.LightSources, …)` but
    `md.VisionSources` is never populated — `ParseTiledJSON` does not build vision
    sources from combatants, and the adapter does not project the PC combatants'
    race darkvision/feat darkvision/Devil's Sight into VisionSources before calling
    `ComputeVisibility…`. Net effect: the `if len(md.VisionSources) > 0 ||
    len(md.LightSources) > 0` guard is false in production, no FoW is computed, and
    the renderer draws the full uncovered map every time. The unit tests construct
    VisionSources manually so they pass.
  - **Critical gap**: `md.DMSeesAll` is also never set in production. There is no
    "DM view vs player view" branching in the regeneration path — both would get
    the same image even once fog is wired.
  - Explored history is **in-memory only** (`exploredCells map[uuid.UUID]map[int]bool`
    on the adapter struct). A bot restart loses every party's explored tiles.
    Spec language ("previously seen but currently out-of-range cells rendered as
    dim/greyed") implies persistence across the campaign lifetime.
  - `ComputeVisibilityWithZones` accepts a magical-darkness tile set, but the
    adapter calls `ComputeVisibilityWithLights` (line 454) — so magical darkness
    never demotes darkvision in production even if VisionSources were filled.

### Phase 69 — Obscurement & Lighting Zones
- Status: **Matches for spell zones, partial for static lighting**
- Key files:
  - `internal/combat/obscurement.go` — `ObscurementLevel`,
    `EffectiveObscurement`, vision/darkness/devil-sight interactions,
    `CombatantObscurement`, `ZoneObscurement` for all 5 zone types
  - `internal/combat/advantage.go:54–60` — heavily obscured ⇒ disadv to attack /
    adv to be attacked (the "Blinded-like" effect)
  - `internal/combat/zone.go`, `zone_definitions.go` — Darkness/Fog Cloud/Silence
    insert `encounter_zones` rows with zone_type populated
  - `internal/combat/obscurement_integration_test.go` — covers darkvision vs
    darkness, magical darkness ignoring darkvision, Devil's Sight, fog/heavy
    obscurement
  - `dashboard/svelte/src/lib/mapdata.js:17–24` — `LIGHTING_TYPES` painter (dim_light,
    darkness, magical_darkness, fog, light_obscurement) — UI side
- Findings:
  - Combat effects are correct: heavily obscured ⇒ effectively blinded
    (disadvantage attacking + advantage attackers), lightly obscured ⇒ Perception
    disadvantage, hide allowed.
  - Darkvision: darkness → dim → none mapping in `EffectiveObscurement` ✓.
    Magical darkness explicitly bypasses darkvision (line 81–88) ✓.
    Fog/heavy_obscurement marked as non-darkness so darkvision does not help ✓.
  - Devil's Sight: 120ft cap, penetrates both natural and magical darkness ✓.
  - `ObscurementCheckEffect` adds disadvantage on Perception checks in obscured
    zones — wired into `/check` via the check package (verifiable in obscurement
    integration tests).
  - **Gap — static lighting brush not consumed by the server**: The map editor
    paints lighting GIDs 1–5 into the tiled JSON's "Lighting" layer, but no Go
    code parses that layer (grep on `internal/gamemap/renderer/parse.go`,
    `internal/combat/zone.go` returns nothing for "lighting"). So an unlit
    dungeon room painted by the DM has no mechanical effect on combatants
    standing in it — `CombatantObscurement` only looks at `encounter_zones`.
    Per spec lines 2216–2217, static lighting "is baked into the map data like
    terrain" and should drive auto-applied modifiers, which is not happening.
  - Combat log shows obscurement reasons via `ObscurementReasonString` — text
    closely mirrors the spec example lines 2259–2260.

### Phase 76a — Turn Timeout: Timer Infrastructure & Nudges
- Status: **Matches**
- Key files:
  - `internal/combat/timer.go` — `TurnTimer`, polling loop (30s default),
    `processNudges`, `processWarnings`, adjacent-enemy detection
  - `internal/combat/timer_messages.go` (referenced via `FormatNudgeMessage`,
    `FormatTacticalSummary`)
  - `internal/combat/timer_overrides.go` — `PauseCombatTimers`, extend, manual skip
  - `cmd/dndnd/main.go:985` — 30-second polling interval wired at startup
- Findings:
  - 50% nudge and 75% tactical summary (HP, AC, conditions, resources, adjacent
    enemies) implemented and tested. Adjacent detection uses Chebyshev distance 1.
  - DM overrides exist: `PauseCombatTimers`, extend timer, skip-now route the
    turn into the 100% decision prompt path.
  - Default 24h timeout configurable per campaign at turn creation (handled in
    `resolveTimerForTurn`).

### Phase 76b — Turn Timeout: 100% Resolution & Prolonged Absence
- Status: **Matches**
- Key files:
  - `internal/combat/timer_resolution.go` — `FormatDMDecisionPromptWithSaves`,
    `AutoResolveTurn`, `WaitExtendTurn`, `ScanStaleTurns`
  - `internal/combat/timer_stale_integration_test.go`
  - `cmd/dndnd/main.go:992–996` — startup `PollOnce` runs as the stale-state scan
- Findings:
  - DM prompt format includes pending move/attacks/bonus and pending saves (lines
    32–58) — spec format at 2022–2027.
  - Auto-resolve applies Dodge (with `StartedRound`/`ExpiresOn` set so the
    condition expires at start of next turn), auto-rolls pending saves with the
    Phase 118 concentration resolver hook, forfeits pending actions and reactions,
    increments `ConsecutiveAutoResolves`, flags `IsAbsent` at 3.
  - Wait extends by 50% of original duration and is gated by `WaitExtended` flag
    (one-shot per timeout cycle) ✓.
  - DM 1-hour fallback: `processDMAutoResolves` reads `dm_decision_deadline` —
    list query / column in place.
  - Stale state scan on startup: `ScanStaleTurns` exists; production path uses
    `PollOnce(ctx)` once before opening the Discord gateway — equivalent.
  - Minor: Bardic Inspiration / On-hit decision (Divine Smite) decline behavior
    relies on the absence of player input rather than an explicit decline path;
    spec lines 2036–2037 describe these as explicit actions. Not a correctness
    issue (forfeit by no-response is consistent), just worth noting.

### Phase 77 — Player Turn Flow: Turn Start & Done
- Status: **Matches**
- Key files:
  - `internal/combat/initiative.go:647–700` — `createActiveTurn` (resources reset,
    timer set for PCs only, ProcessTurnStartWithLog, expire readied actions, reset
    summoned creature resources)
  - `internal/combat/impact_summary.go` — personal impact summary
  - `internal/combat/unused_resources.go` — `/done` warning composer
  - `internal/discord/done_handler.go` — slash /done flow with confirmation
  - `internal/combat/auto_skip.go` — incapacitated auto-skip text
- Findings:
  - Turn-start: condition expiration runs via `ProcessTurnStartWithLog`; movement
    and attacks restored via `ResolveTurnResources`; PC ping in #your-turn channel
    with personal impact summary if logs reference target=combatant since last
    completed turn.
  - `/done` warns via `CheckUnusedResources` when action/attacks/bonus action remain;
    spec lines 1891–1894 example match `FormatUnusedResourcesWarning` output.
  - Auto-skip for incapacitated combatants integrated into `AdvanceTurn` via
    `skipOrActivate`; surprised round-1 skip removes the condition.
  - Map regeneration on turn end is wired through `RegenerateMap` adapter (called
    by the done handler / advance flow).

### Phase 78a — Enemy/NPC Turns: Dashboard Turn Builder
- Status: **Matches**
- Key files:
  - `internal/combat/turn_builder.go` — `BuildTurnPlan` (movement via A*,
    single-attack vs multiattack, recharge abilities, bonus actions)
  - `internal/combat/turn_builder_handler.go` — GET/POST endpoints
  - `dashboard/svelte/src/TurnBuilder.svelte` — multi-step UI with review/adjust
  - `internal/combat/turn_builder_test.go`
- Findings:
  - Movement step uses A* via `pathfinding.FindPath`, trimmed to speed budget
    and stops within best-reach of target ✓.
  - Multiattack: parses "two with its scimitar and one with its dagger" patterns
    plus digit forms.
  - Recharge abilities surfaced from ability name regex `Recharge (\d+)` with
    default min 6.
  - Reactions injected into `TurnPlan.Reactions` via
    `ListActiveReactionDeclarationsByEncounter` at handler entry.
  - DM "fudge any roll" supported by Svelte UI (`updateRoll`) before
    `executeEnemyTurn` posts to combat log.
  - Combat log output via `FormatCombatLog` posted by `EnemyTurnNotifier`.

### Phase 78c — Enemy/NPC Turns: Bonus Action Parsing
- Status: **Matches**
- Key files:
  - `internal/combat/turn_builder.go:466–497` — `ResolveBonusActions`,
    `ParseBonusActions`
  - structured `creatures.bonus_actions` JSONB column
- Findings:
  - Structured column preferred; falls back to scanning ability descriptions for
    the literal "bonus action" substring (excludes Multiattack and Recharge).
  - Verified against Goblin Nimble Escape pattern in tests.

### Phase 78b — Enemy/NPC Turns: Legendary & Lair Actions
- Status: **Matches**
- Key files:
  - `internal/combat/legendary.go` — `LegendaryInfo`/`LairInfo` parsing,
    budget tracking, no-repeat lair tracker, turn-queue builder, plan builders
  - `internal/combat/legendary_handler.go`, `legendary_handler_test.go`
  - `dashboard/svelte/src/LegendaryActionPanel.svelte`,
    `dashboard/svelte/src/LairActionPanel.svelte`
- Findings:
  - Budget parsed from "can take N legendary actions" with default 3.
  - Per-action cost parsed from "(Costs N Actions)" suffix with default 1.
  - `LegendaryActionBudget.Spend`/`Reset` — reset documented to fire at the
    creature's own turn start.
  - Lair: `LairActionTracker.LastUsedName` enforces no-consecutive-repeat; queue
    entry inserted at initiative 20 (`BuildTurnQueueEntries`).
  - Pending reactions surfaced before legendary/lair via the same panel flow.

### Phase 79 — Summoned Creatures & Companions
- Status: **Matches**
- Key files:
  - `internal/combat/summon.go` — Summon/DismissSummon/CommandCreature,
    SummonedTurnResources thread-safe tracker, FormatSummonTurnNotification,
    ValidateCommandOwnership
  - `internal/discord/summon_command_handler*.go` — /command slash handler
  - `internal/combat/summon_integration_test.go`
- Findings:
  - `summoner_id` column on combatants links creature to PC combatant.
  - `/command [creature-id] [action] [target?]` parsed via `ParseCommandArgs`;
    rejects with `ErrNotSummoner` if caller != summoner.
  - Per-creature turn resources (action/movement/bonus/done) tracked via
    `SummonedTurnResources` keyed by creature combatant ID; reset at the
    summoner's turn start (`resetSummonedCreatureResources`).
  - `HandleSummonDeath` removes summons immediately on 0 HP (no death saves).
  - `DismissSummonsByConcentration` ties dismissal to the concentration
    cleanup pipeline.
  - Initiative placement: `IsSummonedCreature(c)` filter in `AdvanceTurn`
    excludes summons from getting their own initiative slot — they share the
    caster's turn. Spec lines 1956–1958 say creatures *with their own turns*
    (familiars, conjured animals, animated dead) should roll initiative —
    **divergence**: the current code uniformly treats every summon as
    caster-turn-only, with no flag to opt into independent initiative.
    `FormatSummonTurnNotification` exists ("your Wolf #1's turn") but is
    apparently never reached for summons that should have their own turn.

### Phase 80 — Combat Recap (`/recap`)
- Status: **Matches**
- Key files:
  - `internal/combat/recap.go` — formatting, filtering, truncation
  - `internal/discord/recap_handler.go` — /recap slash handler with
    active-vs-completed encounter fallback
  - `internal/refdata/ListActionLogWithRounds` query
- Findings:
  - `/recap` with no args during active combat filters to logs after the player's
    last completed turn timestamp (`filterSinceLastTurn`); ephemeral response.
  - `/recap N` uses `FilterLogsLastNRounds`.
  - Post-combat fallback to most recent completed encounter via
    `GetMostRecentCompletedEncounter` ✓.
  - Output grouped by round; truncated to 2000-char Discord limit.
  - Phase 105 multi-encounter routing — `ActiveEncounterForUser` ensures the
    caller's encounter is picked.

## Cross-cutting concerns

1. **Fog of war is dead code in production.** The full algorithm + types +
   explored history scaffolding exists in `internal/gamemap/renderer` and
   `cmd/dndnd/discord_adapters.go`, but no caller builds `VisionSource` slices
   from live combatants, no caller builds `LightSource` slices from torch/Light
   cantrip entries, no caller projects magical-darkness zones into
   `MagicalDarknessTiles`, and `DMSeesAll` is never toggled. This is the single
   largest user-facing divergence in this batch. The unit tests pass because they
   construct these manually.

2. **Static lighting (map-editor lighting brush) is rendered in the Svelte
   editor but never reaches the Go renderer or combat resolver.** No Go code
   parses the lighting layer's GIDs into terrain-level obscurement. Only
   spell-created `encounter_zones` flow through `CombatantObscurement`.

3. **Explored cells are in-memory.** `mapRegeneratorAdapter.exploredCells` is
   a `map[uuid.UUID]map[int]bool` on the struct — no DB table. A `fly.toml`
   redeploy or crash loses every party's exploration history mid-campaign.

4. **Summon initiative dichotomy not represented.** Spec distinguishes
   "creatures with their own turns" (Conjure Animals wolves, Animate Dead
   skeletons, familiars) from "spell effects that act on the caster's turn"
   (Spiritual Weapon, Flaming Sphere). The codebase treats every summon as the
   latter. `FormatSummonTurnNotification` is unreachable.

5. **Combat log format mostly matches the reference.** Spot-checks against
   spec lines 1756–1898 show emoji glyphs and structure align (movement, attack
   with adv/disadv, condition application, death saves, summons, legendary,
   lair). The auto-skip message has been Phase 114-adjusted for "surprised"
   text.

## Critical items

1. **Wire VisionSources/LightSources/MagicalDarknessTiles/DMSeesAll into the
   production map regenerator.** Build a translation from
   `refdata.Combatant` + race/feat data + active concentration light effects →
   `renderer.VisionSource` / `renderer.LightSource`; project zones of type
   `magical_darkness` into `MagicalDarknessTiles`; set `DMSeesAll=true` when
   rendering for the DM dashboard view. Without this, **Phase 68 is effectively
   not delivered** even though the algorithm is correct and well-tested.

2. **Wire static lighting brush GIDs into the obscurement pipeline.** Parse
   the lighting layer in `ParseTiledJSON`, expose tile-level lighting in
   `MapData`, and have `CombatantObscurement` (or a new helper) consult that
   per-tile data in addition to `encounter_zones`. Otherwise an unlit dungeon
   room is purely cosmetic.

3. **Persist explored cells to the database.** Add an
   `encounter_explored_cells` table or store packed bitmaps on the encounter
   row; load on regen, save after each regen. Required for true async play
   where the bot may be restarted between turns.

4. **Decide and implement summon-initiative semantics.** Either add an
   `initiative_mode` column (per-summon `own_turn` vs `caster_turn`) and route
   `AdvanceTurn` accordingly, or document in the spec that all summons share
   the caster's turn (which simplifies things but is a divergence from D&D 5e
   for conjured animals/animated dead).

5. (Minor) **Action-economy decline paths in 100% auto-resolve are implicit.**
   Adding explicit "declined Divine Smite (auto-resolved)" / "declined Bardic
   Inspiration (auto-resolved)" log lines would match spec lines 2036–2037 and
   give the DM a clearer record of forfeited choices.
