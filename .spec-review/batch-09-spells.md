# Batch 09: Spell casting (Phases 58–67)

## Summary

The spell-casting pipeline is broadly implemented and exhaustively unit-tested at the service layer. `Cast`/`CastAoE` cover slot selection (regular + pact), upcast scaling, cantrip scaling, ritual validation, save DC + spell attack rolls, concentration replacement, material components with gold fallback, teleportation, and zone creation. The Phase 118 concentration-cleanup pipeline (`BreakConcentrationFully`) is wired from every trigger (damage CON-save fail, incapacitation, Silence entry, voluntary drop, replacement) and consolidates condition/zone/summons cleanup.

However, several **integration gaps** prevent the player-facing flow from matching the spec:

1. **Cylinder AoE shape unsupported** in `GetAffectedTiles` — Moonbeam, Flame Strike, Ice Storm, Sleet Storm, Call Lightning all fail at cast time.
2. **AoE pipeline ignores Metamagic entirely** — Careful/Heightened/Empowered/Twinned never reach `CastAoE`; only single-target `Cast` consumes them.
3. **Encounter zone overlays never reach the rendered map** — `MapData.ZoneOverlays` is never populated by any production map renderer caller.
4. **Metamagic interactive prompts (`PromptEmpowered`, `PromptCareful`, `PromptHeightened`) are dead code** — built and tested but never invoked from `cast_handler.go`.
5. **Twinned target ID is never set** from the Discord layer (`CastCommand.TwinTargetID` always uuid.Nil).
6. **DM-queue routing is a string flag, not a row insertion** — `dm_required` spells & higher-level teleports log "Routed to DM" but never create a `dm_queue_items` row.
7. **Teleport `requires_sight` field is parsed but never enforced.**
8. **/prepare paginated select-menu UX is explicitly deferred** — only a text-based MVP exists.
9. **Wall-of-Fire / line-shaped zones render as a single tile** — `ZoneAffectedTilesFromShape` has no `line` case.

## Per-phase findings

### Phase 58 — `/cast` Basic
- **Status:** Matches (with minor gap)
- **Key files:** `internal/combat/spellcasting.go` (Cast 330–712), `internal/discord/cast_handler.go`
- **Findings:** Bonus-action detection (`IsBonusActionSpell`), bonus-action restriction in both directions (`ValidateBonusActionSpellRestriction`), range validation (touch/ranged/self/unlimited), spell save DC (`SpellSaveDC`), spell attack rolls (`SpellAttackModifier`), slot deduction with persistence, concentration replacement, raging-barbarian / wild-shaped Druid blocks all present. Combat-log formatted via `FormatCastLog`. `#roll-history` posting wired through `postCombatLog`. **Gap:** "see-the-target" check for invisible single-target targets present; bonus-action vs leveled-action persistence on turn state correct.

### Phase 59 — AoE & Saves
- **Status:** Partial
- **Key files:** `internal/combat/aoe.go`, `internal/combat/cover.go`
- **Findings:** Sphere, cone, line, square shapes implemented (`SphereAffectedTiles`, `ConeAffectedTiles` w/ 53° cone width = projFt/2, `LineAffectedTiles`, `SquareAffectedTiles`). `GetAffectedTiles` dispatches by shape. Pending save rows persisted (`pending_saves` table with `aoe:<spell-id>` source). `ResolveAoEPendingSaves` triggers damage once all rolls return; forfeited rows treated as failures. Cover DEX bonus applied via `CalculateAoECover`. `ApplySaveResult` correctly handles `half_damage` / `no_effect` / `special`. Damage funnels through `ApplyDamage` so R/I/V + concentration save + 0-HP unconscious all fire.
- **Critical gaps:**
  - **Cylinder shape not handled** — falls into the `default` error branch in `GetAffectedTiles` (aoe.go:236). Affects Moonbeam, Flame Strike, Ice Storm, Sleet Storm, Spike Growth-style spells.
  - **CastAoE ignores Metamagic** — no `Metamagic` field on `AoECastCommand` and no validation/application path, so Careful, Heightened, Empowered, Twinned cannot affect AoE casts even though spec specifically calls them out for AoE.

### Phase 60 — Upcasting, Ritual, Cantrip Scaling
- **Status:** Matches
- **Key files:** `internal/combat/spellcasting.go` (1067–1170)
- **Findings:** `ScaleSpellDice` handles upcast (`higher_level_dice` from spell JSON, added per slot above base), supports both "NdX" and ray "NxMdX" formats. `ScaleHealingDice` mirrors for healing. `CantripDiceMultiplier` returns 1/2/3/4 at levels 1–4/5–10/11–16/17+. `SelectSpellSlot` defaults to lowest available when `--slot` omitted. `ValidateRitual` requires `spell.ritual = true`, `encounter.status != "active"`, and class with Ritual Casting feature (Wizard/Cleric/Druid/Bard). Ritual cast skips slot deduction (Cast §15 only runs when `!cmd.IsRitual`).
- **Gap:** Spec mentions ritual adds 10 min to casting time — no in-game timer/clock reflected, but encounter status gate is sufficient.

### Phase 61 — Concentration Checks & Breaking
- **Status:** Matches (Phase 118 reinforcement)
- **Key files:** `internal/combat/concentration.go`
- **Findings:** `ConcentrationCheckDC = max(10, damage/2)`. Damage trigger via `MaybeCreateConcentrationSaveOnDamage` enqueues a `pending_saves` row with `source="concentration"`. Failure routed via `ResolveConcentrationSave` -> `BreakConcentrationFully`. Incapacitation auto-break in `ApplyCondition` (condition.go:191–202) — all five spec conditions tracked (`incapacitated`, `stunned`, `paralyzed`, `unconscious`, `petrified`). Silence-zone break: zone-creation hook (`breakSilenceCaughtConcentrators`) + movement hook (`UpdateCombatantPosition` calls `CheckSilenceBreaksConcentration`) + cast-time pre-validation (`combatantInSilenceZone` rejects V/S spells before slot deduction). `BreakConcentrationFully` strips spell-sourced conditions, deletes concentration-tagged zones, dismisses summons, clears columns, and emits consolidated 💨 log line.

### Phase 62 — Teleportation
- **Status:** Partial
- **Key files:** `internal/combat/teleport.go`, `internal/combat/spellcasting.go` (1203–1269)
- **Findings:** `ParseTeleportInfo` parses target/range/sight/companion. `IsDMQueueTeleport` flags `portal`/`party`/`creatures`/`group`. `ValidateTeleportDestination` checks (a) destination unoccupied, (b) within range, (c) companion within `companion_range_ft`. Caster + companion positions updated atomically. Path validation correctly bypassed.
- **Critical gaps:**
  - **`info.RequiresSight` parsed but never enforced** — no line-of-sight check in `ValidateTeleportDestination` (the spec explicitly requires this for Misty Step, Thunder Step, Far Step).
  - **DM queue "routing" for narrative teleports is a string flag only** — `TeleportResult.DMQueueRouted = true` sets `result.ResolutionMode = "dm_required"` but no `dm_queue_items` row is created; the player just sees "Routed to DM" in the log. Spec requires actual DM-queue entry for Teleport / Word of Recall.
  - **Companion-willing flag not modelled** — spec requires "companion is willing", implementation only checks distance.

### Phase 63 — Material Components
- **Status:** Matches
- **Key files:** `internal/combat/spellcasting.go` (1271–1390), `internal/discord/cast_handler.go` (promptMaterialFallback)
- **Findings:** Free components auto-satisfied (only spells with `MaterialCostGp.Valid` are checked). `ValidateMaterialComponent` returns `Proceed`/`NeedsGoldConfirmation`/`Rejected`. Inventory match is case-insensitive by description string. Consumed materials removed via `RemoveInventoryItem` after gold deduction. Discord layer surfaces a "Buy & Cast" / "Cancel" prompt through `materialPrompts` store; rejection path posts `FormatMaterialRejection`. Slot is NOT deducted on the gold-confirmation early return.

### Phase 64 — Pact Magic (Warlock)
- **Status:** Matches
- **Key files:** `internal/combat/spellcasting.go` (943–1010), `internal/character/spellslots.go`
- **Findings:** `pact_magic_slots` JSONB column. `PactMagicSlots{SlotLevel, Current, Max}`. Cast prefers pact slot when (a) caster has pact slot, (b) spell fits ≤ pact slot level, (c) `--spell-slot` flag is false. Upcast validated by `SelectSpellSlot` and pact slot level cap. `RechargePactMagicSlots` restores to Max; wired into rest pipeline via `result.PactSlotsRestored` flag on short rest. `PactMagicSlotsForLevel` table covers levels 1–20 per Warlock spec.

### Phase 65 — Spell Preparation (/prepare)
- **Status:** Partial (UX deferred)
- **Key files:** `internal/combat/preparation.go`, `internal/discord/prepare_handler.go`
- **Findings:** `MaxPreparedSpells = abilityMod + classLevel (min 1)`. `IsPreparedCaster` covers Cleric/Druid/Paladin. `AlwaysPreparedSpells` provides subclass-static lists for Life Cleric, Devotion Paladin, Land Druid (others stubbed). `ValidateSpellPreparation` enforces class spell list membership, slot-level availability, and max count (always-prepared excluded from count). `LongRestPrepareReminder` is wired into the rest handler.
- **Critical gap:** The paginated **Discord select-menu UX (per spec §1018–1026) is explicitly deferred** — `prepare_handler.go:48–49` says the MVP UX is text-only via `--spells` arg. Only a few subclasses have always-prepared seed data; Tempest, Trickery, Knowledge clerics, Vengeance Paladin, Coast/Mountain/etc. Druids missing.

### Phase 66a — Sorcery Points & Metamagic Framework
- **Status:** Matches
- **Key files:** `internal/combat/sorcery.go`, `internal/discord/bonus_handler.go`
- **Findings:** `feature_uses["sorcery-points"]` tracked. `metamagicCosts` table matches spec exactly (careful/distant/empowered/extended/subtle = 1, quickened = 2, heightened = 3, twinned = spell level / 1 for cantrips). `slotCreationCosts` matches spec (1st=2, 2nd=3, 3rd=5, 4th=6, 5th=7), rejects ≥6th level. `FontOfMagicConvertSlot` caps points at Sorcerer level; `FontOfMagicCreateSlot` deducts points and adds slot. Both consume a bonus action via `useBonusActionAndPersist`. `ValidateMetamagic` enforces one-per-spell + Empowered combo rule. Wired into `/bonus font-of-magic` via `bonus_handler.go:137`.

### Phase 66b — Metamagic Individual Options
- **Status:** Partial (single-target only; prompts dead)
- **Key files:** `internal/combat/metamagic.go`, `internal/discord/metamagic_prompt.go`, `internal/discord/cast_handler.go` (523–550)
- **Findings:** All 8 metamagic options have validators (`validateSingleMetamagicOption`). Costs correct. `applyMetamagicEffects` populates `CastResult` fields. `ApplyDistantSpell` doubles range or sets 30ft for touch. `ApplyExtendedSpell` doubles duration and caps at 24h. Discord cast handler picks up boolean flags via `collectMetamagic` and normalizes aliases.
- **Critical gaps:**
  - **`CastCommand.TwinTargetID` is never set** in `cast_handler.go` — there's no `twin-target` Discord option being read, so `Cast` always sees `uuid.Nil` and the twin-target lookup path (spellcasting.go:586–598) is dead. Twinned metamagic effectively does nothing today.
  - **`MetamagicPromptPoster.PromptEmpowered/PromptCareful/PromptHeightened` are implemented, tested, but NEVER invoked** from the cast handler. The interactive flows (pick dice to reroll, pick allies to protect, pick target to heighten) are dead code. Empowered just adds "may reroll up to N dice" to the log; no actual reroll happens.
  - **Metamagic does not flow through `CastAoE`** — Careful & Heightened, which the spec explicitly defines on AoE spells, never apply to AoE casts. `CarefulSpellCreatures` is reported in log but never wired into the AoE save resolution to mark allies as auto-success.
  - **Quickened bonus-action restriction interaction is handled correctly** (`isBonusAction` flips after quickened detection at spellcasting.go:339–342, then ValidateBonusActionSpellRestriction runs against the effective resource type).

### Phase 67 — Spell Effect Zones (Encounter Zones)
- **Status:** Partial
- **Key files:** `internal/combat/zone.go`, `internal/combat/zone_definitions.go`, `db/migrations/20260314120001_create_encounter_zones.sql`
- **Findings:** Migration creates `encounter_zones` with all expected columns (shape, origin, anchor_mode, anchor_combatant_id, zone_type, overlay_color, marker_icon, requires_concentration, expires_at_round, zone_triggers, triggered_this_round). `KnownZoneDefinitions` covers Fog Cloud, Spirit Guardians, Wall of Fire, Darkness, Cloud of Daggers, Moonbeam, Silence, Stinking Cloud. `Cast.maybeCreateSpellZone` (single-target Spirit Guardians anchored to caster) and `CastAoE` (anchor at target tile) both insert zones. Duration parser (`SpellDurationRounds`) converts "1 minute" → 10 rounds, "10 minutes" → 100, "1 hour" → 600, sets `ExpiresAtRound`. Concentration break removes concentration-tagged zones. `UpdateCombatantPosition` moves combatant-anchored zones with caster. `CheckZoneTriggers` enforces once-per-creature-per-round via `triggered_this_round` map and is invoked from initiative round advance and movement. `ResetZoneTriggersForRound` resets at round advance. `CleanupExpiredZones` and `CleanupEncounterZones` provided.
- **Critical gaps:**
  - **Zone overlays are never rendered on the map.** `MapData.ZoneOverlays` field exists and `DrawZoneOverlays` is called, but no production code (cast_handler, move_handler, attack_handler) populates the slice — all `ParseTiledJSON(..., nil, nil)` calls pass nil effects. So persistent spell zones do not appear visually. Map legend integration also missing.
  - **`ZoneAffectedTilesFromShape` has no `"line"` case** (zone.go:323–333) — Wall of Fire (defined as Shape="line") falls into the `default` branch returning a single origin tile, so its overlay/coverage is wrong.
  - **Spirit Guardians shape is `circle` in `KnownZoneDefinitions`** even though it's anchored to the moving caster, which is fine, but the **damage trigger flow** is `enter`/`start_of_turn` -> `Effect: "damage"` -> no automatic damage application is wired into `CheckZoneTriggers` results. Callers receive `ZoneTriggerResult{Effect: "damage"}` but no integration point rolls and applies the spell's damage dice.
  - **Anchor mode constant inconsistency** — Silence zone has no `AnchorMode` set, defaults to `"static"` via `zoneAnchorOrDefault`; this is correct.
  - **Migration timestamp `20260314120001` precedes Phase 67 implementation phase order** — chronologically OK but worth noting.

## Cross-cutting concerns

- **DM-queue integration for spells with `resolution_mode = "dm_required"` is missing.** A flag is set on the response but no `dm_queue_items` row is created — DM has nothing to act on in the dashboard. Spells affected: Polymorph, Banishment, Wish, Wall of Force, Animate Dead, Reverse Gravity, Sleet Storm, Magic Circle, narrative teleports (Teleport, Word of Recall). The dashboard never sees these casts.
- **AoE pipeline divergence:** `Cast` and `CastAoE` duplicate ~60% of the cast-time validation logic (silence, concentration, slot, ritual gating). The duplication means concentration cleanup is symmetric but metamagic / material components / invisibility / wild-shape gating live only in `Cast`. A unified `castPreflight` helper would close several gaps at once.
- **Material components** only check the description string match — a "diamond worth 50 gp" item won't match "diamond worth 300 gp" required by Revivify. Inventory item naming is fuzzy.
- **Spell zone damage application** (per-creature start-of-turn / enter damage like Spirit Guardians, Wall of Fire, Moonbeam, Cloud of Daggers) — `CheckZoneTriggers` returns `Effect: "damage"` results but there is no caller-side machinery that rolls the spell's damage and applies it via `ApplyDamage`. Confirmed by grepping for callers of `CheckZoneTriggers` results.
- **Empowered combo unreachable from Discord** — even though `ValidateMetamagic` correctly allows Empowered + one other, the Discord cast handler enumerates boolean flags `subtle, twin, careful, heightened, distant, quickened, empowered, extended`; all can be true simultaneously, so `ValidateMetamagic` rejection of `nonEmpowered > 1` is the only gate. Acceptable, but the per-flag set has no UI hint about the rule.

## Critical items

1. **`GetAffectedTiles` missing `cylinder`** — silently breaks 6+ seeded SRD spells (Moonbeam, Flame Strike, Ice Storm, Sleet Storm, Call Lightning, Reverse Gravity). Trivial fix: add a `cylinder` case treating it as a 2D circle on the grid (height_ft is decorative for a flat map). `internal/combat/aoe.go:226–238`.
2. **AoE Metamagic missing** — `AoECastCommand` has no `Metamagic` field; `CastAoE` never validates nor applies metamagic. The spec explicitly describes Careful + Heightened acting on AoE saves and Twinned rejecting AoE. Patch: add `Metamagic []string` to `AoECastCommand`, run `ValidateMetamagic` + `ValidateMetamagicOptions`, plumb `CarefulSpellCreatures` into the per-target save resolution to flip those allies to auto-success.
3. **Zone overlays not rendered on the map** — `MapData.ZoneOverlays` is never populated by any caller (`cast_handler.go:496`, `move_handler.go`, `attack_handler.go`). Discord renderer pipeline must `ListZonesForEncounter` + convert each `ZoneInfo` to a `ZoneOverlay` with tile coverage from `ZoneAffectedTilesFromShape` before calling `RenderMap`. Map legend also missing.
4. **Metamagic interactive prompts dead** — `PromptEmpowered`, `PromptCareful`, `PromptHeightened` constructed but never invoked. Empowered Spell currently announces "may reroll up to N dice" but no reroll occurs server-side.
5. **Twinned target Discord wiring missing** — no `twin-target` option read in `cast_handler.go`; `CastCommand.TwinTargetID` is always uuid.Nil. The whole twin-target path in `Cast` (spellcasting.go:586–598) is dead code from the player perspective.
6. **DM-queue rows not created for `dm_required` spells & high-level teleports** — only a flag is set; no `dm_queue_items` row, so DM dashboard never receives these.
7. **`requires_sight` teleport flag unenforced** — Misty Step / Thunder Step / Far Step can teleport to invisible destinations without restriction.
8. **`ZoneAffectedTilesFromShape` missing `line` case** — Wall of Fire renders as a single tile.
9. **/prepare paginated select-menu UX deferred** — only text-based `--spells "a,b,c"` arg supported; spec describes a full ephemeral UI with checkboxes per level.
10. **Zone damage triggers not auto-applied** — `CheckZoneTriggers` returns `Effect: "damage"` but caller never rolls and applies the damage; Spirit Guardians / Wall of Fire / Moonbeam / Cloud of Daggers all rely on this hook firing damage and it currently does not (or if it does, the integration is buried in callers I could not find).
