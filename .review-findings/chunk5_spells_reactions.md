# Chunk 5 Review — Phases 58–72 (Spells, Zones, FoW, Reactions)

Generated: 2026-05-10. Source: phases.md lines 321–399, spec sections "Spell Casting Details" (891–1071), "Reactions" (1073–1118), "Dynamic Fog of War" (2192+), "Encounter Zones" (3210+).

## Summary

The **service layer is broadly complete** for every phase 58–72. Validators, formulas, formatters, persistence, and reverse-direction restrictions are implemented and TDD-tested at unit and integration level. However, **wiring is the dominant gap**: the `/cast`, `/cast` (AoE), `/prepare`, `/bonus font-of-magic`, and `/action ready` Discord commands are registered but routed to *stub* handlers — none of `combat.Service.Cast`, `CastAoE`, `PrepareSpells`, `FontOfMagicConvertSlot`, `FontOfMagicCreateSlot`, or `ReadyAction` are invoked from production code. Several support paths (zone enter/leave triggers, zone anchor follow, zone round-reset, expired-zone cleanup, expire-readied-actions on turn start, ZoneDefinition→CreateZone on /cast, fog-of-war composition into rendered maps, ObscurementCheckEffect in /check) exist as service methods but have no callers outside tests. Concentration is the cleanest area: the Phase 118 break pipeline is fully wired through the timer's pending-save resolver.

Non-wiring functional gaps:
- Phase 61: `ValidateSilenceZone` is implemented but *not* invoked by `Cast`/`CastAoE`, so Silence does not block casting V/S spells (only post-hoc breaks via `CheckSilenceBreaksConcentration` work).
- Phase 65: `/prepare` UI is text-only via `FormatPreparationMessage`; no paginated select-menu implementation.
- Phase 67: `ZoneDefinition` lacks `AnchorMode`, so combatant-anchored zones (Spirit Guardians, Aura of Protection) cannot be auto-created from the lookup.
- Phase 67: `Cast` / `CastAoE` never call `CreateZone` for persistent-AoE spells (Fog Cloud, Spirit Guardians, Wall of Fire, Darkness, Silence).
- Phase 67: enter/leave damage triggers (Spirit Guardians once-per-turn) are never evaluated — `CheckZoneTriggers` and `ResetZoneTriggersForRound` have zero non-test callers.
- Phase 68: `Explored` visibility state is defined in the renderer but never produced by `ComputeVisibility` (no historical tracking). "Albert Ford symmetric shadowcasting" is actually a per-tile center-to-center raycast (still symmetric, but not octant-recursive).
- Phase 68: `LightSource` lacks the bright/dim two-range model called out in the spec (torch = 20ft bright + 20ft dim).
- Phase 68: `RenderMap` is not invoked from any production code path; no `/map` slash command exists.
- Phase 69: `ObscurementCheckEffect` (perception disadvantage) is wired into attack rolls but not into `/check perception`.
- Phase 71: `ReadyAction` records the spell name + slot level but does *not* deduct the slot or set a concentration flag — both required by spec.
- Phase 72: no Discord-side player prompt UI (slot-level buttons + Pass) — the two-step counterspell flow is service+HTTP only; no `#your-turn` interaction wired.

## Per-phase findings

### Phase 58 — `/cast` basic [WARN]
- ✅ Slot validation/deduction: `internal/combat/spellcasting.go:46` `ValidateSpellSlot`, `:59` `DeductSpellSlot`, `:727` `deductAndPersistSlot`, full Cast at `:329`.
- ✅ Range enforcement (touch/self/ranged): `internal/combat/spellcasting.go:75` `ValidateSpellRange`.
- ✅ Spell save DC = 8 + prof + ability mod: `internal/combat/spellcasting.go` (uses `SpellSaveDC` helper) referenced at line 580; ability resolved via `resolveSpellcastingAbilityScore` `:1199`.
- ✅ Spell attack rolls: `internal/combat/spellcasting.go:96` `SpellAttackModifier`, used at `:586`.
- ✅ Bonus action auto-detect from casting_time: `internal/combat/spellcasting.go:21` `IsBonusActionSpell`.
- ✅ Bonus action restriction (both directions): `internal/combat/spellcasting.go:28` `ValidateBonusActionSpellRestriction`, with `BonusActionSpellCast` / `ActionSpellCast` flags persisted on `turns`.
- ✅ Concentration auto-drop on new concentration spell: `internal/combat/spellcasting.go:110` `ResolveConcentration`, plus `applyConcentrationOnCast` at `internal/combat/concentration.go:488`.
- ❌ NOT wired into Discord. `/cast` command is registered at `internal/discord/commands.go:69` (with all metamagic flags) but routed through `NewStatusAwareStubHandler` — `internal/discord/router.go:198–217` lists `cast` in `gameCommands` whose handlers are stubs. `combat.Service.Cast` has zero non-test callers (`grep "Cast(ctx" → only test files and `applyConcentrationOnCast`). No `cast_handler.go` in `internal/discord/`.
- ⚠️ Cast does not validate that the spell is in the caster's prepared/known list; any spell ID with a slot of the right level can be cast.
- Tests: `internal/combat/spellcasting_test.go` (covers slot deduction, save DC, bonus action detection, concentration tracking, attack rolls).

### Phase 59 — AoE & Saves [WARN]
- ✅ Sphere/cone/line/square shapes: `internal/combat/aoe.go:47` `SphereAffectedTiles`, `:110` `ConeAffectedTiles`, `:119` `LineAffectedTiles`, `:128` `SquareAffectedTiles`. 5e cone width = projection distance: `:111`.
- ✅ Affected creature overlap: `internal/combat/aoe.go:144` `FindAffectedCombatants`.
- ✅ DEX cover bonus on saves: `internal/combat/aoe.go:183` `CalculateAoECover` calls `CalculateCoverFromOrigin` and `cover.DEXSaveBonus()`.
- ✅ Half damage on save: `internal/combat/aoe.go:203` `ApplySaveResult` (returns 0.5 / 1.0 / 0.0 / -1.0 special).
- ✅ ResolveAoESaves rolls damage once and applies multiplier per target: `internal/combat/aoe.go:516`. Routes through `applyDamageHP` for the Phase 118 concentration-on-damage hook.
- ❌ Pending-save persistence + ping flow not wired. `CastAoE` returns a slice of `PendingSave` structs (`internal/combat/aoe.go:249,477`) but no caller persists them via `CreatePendingSave` or pings affected players. `CastAoE` also has zero non-test callers.
- ⚠️ NPC saves: spec calls for DM dashboard rolls; no DM AoE-save handler found in `internal/combat/handler.go` or dashboard routes.
- Tests: `internal/combat/aoe_test.go` covers each shape and cover interaction.

### Phase 60 — Upcasting, Ritual, Cantrip Scaling [PASS]
- ✅ Upcasting `--slot N`: `internal/combat/spellcasting.go:983` `SelectSpellSlot` validates `slotLevel >= spellLevel`, rejects empty slot.
- ✅ Default lowest available: `internal/combat/spellcasting.go:1001` (auto-walk from spellLevel up to 9).
- ✅ Higher-level dice scaling: `internal/combat/spellcasting.go:878` `ScaleSpellDice`, `:890` `ScaleHealingDice`, `:908` `addDice` (handles `NdX` and ray `NxMdX`).
- ✅ Ritual flag: `internal/combat/spellcasting.go:951` `ValidateRitual` (spell.ritual + class has Ritual Casting + encounter !=active). `HasRitualCasting` covers Wizard/Cleric/Druid/Bard at `:941`.
- ✅ Cantrip scaling at level 5/11/17: `internal/combat/spellcasting.go:966` `CantripDiceMultiplier` returns 1/2/3/4. Applied via `scaleCantripDice` at `:900`.
- Tests: `internal/combat/spellcasting_test.go` covers each breakpoint and ritual class gate.

### Phase 61 — Concentration Checks & Breaking [WARN]
- ✅ Damage CON DC = max(10, half damage): `internal/combat/concentration.go:17` `ConcentrationCheckDC`.
- ✅ Pending CON save enqueued automatically on damage: `internal/combat/concentration.go:323` `MaybeCreateConcentrationSaveOnDamage`. Damage paths funnel through `applyDamageHP` (`:281`).
- ✅ Failed-save break pipeline: `internal/combat/concentration.go:357` `ResolveConcentrationSave` → `BreakConcentrationFully` (`:428`).
- ✅ Timer wires this end to end: `internal/combat/timer.go:46` `SetConcentrationResolver`, called at `cmd/dndnd/main.go:644`. Resolution invoked at `internal/combat/timer_resolution.go:231`.
- ✅ Incapacitation auto-break: `internal/combat/concentration.go:55` `CheckConcentrationOnIncapacitation`. Hooked from `ApplyCondition` (incapacitation hook noted in concentration.go:276 and integration tested in `concentration_integration_test.go`).
- ✅ Silence-zone break on entry/zone creation: `internal/combat/concentration.go:218` `CheckSilenceBreaksConcentration`. Wired into `Service.UpdateCombatantPosition` (`internal/combat/service.go:441`) and `CreateZone` (`internal/combat/zone.go:149` for new Silence zones via `breakSilenceCaughtConcentrators` `:160`).
- ✅ Cleanup on break: `BreakConcentrationFully` strips spell-sourced conditions across encounter, deletes concentration zones, dismisses summons, clears caster columns, returns 💨 line. Tested in `concentration_integration_test.go`.
- ❌ **Cast-time Silence block is NOT wired.** `ValidateSilenceZone` (`internal/combat/concentration.go:92`) is defined but `Cast`/`CastAoE` never call it — a player inside Silence could still successfully cast a V/S spell because the call site is missing. Spec line 902 explicitly requires the cast itself to be rejected.
- Tests: `internal/combat/concentration_test.go`, `concentration_integration_test.go` (covers incapacitation hook, Silence break on zone-creation, failed-CON-save flow, voluntary drop cleanup).

### Phase 62 — Teleportation Spells [PASS]
- ✅ Bypass pathfinding: `Cast` calls `resolveTeleport` (`internal/combat/spellcasting.go:599`) before any movement; teleport path uses direct `UpdateCombatantPosition` skipping movement validation.
- ✅ Destination unoccupied / in range / companion in range: `internal/combat/teleport.go:69` `ValidateTeleportDestination`.
- ✅ `requires_sight`: parsed at `internal/combat/teleport.go:22` but no actual line-of-sight check is performed (LoS plumbing absent — spec line 1045 expects "(3) line of sight to destination if `requires_sight = true`").
- ✅ DM-queue routing: `internal/combat/teleport.go:48` `IsDMQueueTeleport` covers `portal`, `party`, `creatures`, `group`. Set on result via `result.ResolutionMode = "dm_required"` at `spellcasting.go:606`.
- ✅ `additional_effects` propagated: e.g. Thunder Step departure damage is described in TeleportInfo and surfaced in log. Damage is *not* auto-applied — it's a narrative note.
- Tests: `internal/combat/teleport_test.go` covers all targets, occupant collision, range, companion range, DM-queue routing.
- ⚠️ Without `requires_sight` line-of-sight enforcement, Misty Step / Far Step lose part of their constraint. Acceptable as DM oversight, but not RAW.

### Phase 63 — Material Components [PASS]
- ✅ Free for non-costly: `Cast` only enters the material path when `spell.MaterialCostGp.Valid` (`internal/combat/spellcasting.go:416`).
- ✅ Inventory check: `internal/combat/spellcasting.go:1111` `ValidateMaterialComponent` matches by description case-insensitively.
- ✅ Gold fallback prompt: `MaterialCheckNeedsGoldConfirmation` returns; if `cmd.GoldFallback=false`, Cast returns early with `MaterialComponent.NeedsGoldConfirmation=true` (`spellcasting.go:430`).
- ✅ Consumed removal: `internal/combat/spellcasting.go:1160` `RemoveInventoryItem` (decrements quantity, drops at 0).
- ✅ Gold deduction + non-consumed item add-on-buy: `spellcasting.go:444–453`.
- ✅ Rejection message: `spellcasting.go:1148` `FormatMaterialRejection`.
- Tests: `internal/combat/spellcasting_test.go` (proceed / gold-confirm / rejected / consumed paths).

### Phase 64 — Pact Magic (Warlock) [PASS service / WARN handler]
- ✅ Separate pact pool: `internal/combat/spellcasting.go:751` `PactMagicSlotState` alias of `character.PactMagicSlots`. Parsed at `:755`.
- ✅ Pact slots used first by default: `spellcasting.go:404` (`!cmd.UseSpellSlot && pactSlots.Current > 0 && spellLevel <= pactSlots.SlotLevel`).
- ✅ `--spell-slot` override: `CastCommand.UseSpellSlot` flag at `spellcasting.go:311`; Discord option registered at `internal/discord/commands.go:125`.
- ✅ Upcast ≤ pact level: enforced because `effectiveSlotLevel = pactSlots.SlotLevel` only when `spellLevel <= pactSlots.SlotLevel`; otherwise falls back to regular slots.
- ✅ Short-rest recharge: `internal/combat/spellcasting.go:783` `RechargePactMagicSlots`. Wired through rest service: `internal/rest/rest.go:126–132` restores PactMagicSlots in `ShortRest`, applied via `internal/rest/party_handler.go:223–237`.
- ✅ Both pools displayed separately: `CastResult.UsedPactSlot/PactSlotsRemaining` and `SlotUsed/SlotsRemaining` are distinct fields; format log differentiates at `spellcasting.go:207–211`.
- ⚠️ `Cast` itself isn't wired (see Phase 58), but the pact pipeline is correct internally.
- Tests: `internal/combat/spellcasting_test.go` (pact slot use, recharge fully and partially, non-warlock no-op).

### Phase 65 — `/prepare` [WARN]
- ✅ Service-level prep flow: `internal/combat/preparation.go:375` `PrepareSpells` validates count, class spell list, slot-level availability.
- ✅ Always-prepared subclass spells: `internal/combat/preparation.go:69` `alwaysPreparedBySubclass` (life cleric, devotion paladin, land druid). Excluded from count via `countNonAlwaysPrepared` (`:148`).
- ✅ Max prepared = ability mod + class level (min 1): `internal/combat/preparation.go:119` `MaxPreparedSpells`.
- ✅ Long-rest reminder: `internal/combat/preparation.go:295` `LongRestPrepareReminder`. Wired via `internal/rest/rest.go:263` into `LongRestResult.PreparedCasterReminder`, formatted at `internal/rest/format.go:68`.
- ❌ No paginated Discord select-menu UI. `FormatPreparationMessage` (`preparation.go:305`) outputs static text. No `/prepare` Discord handler in `internal/discord/`. Spec lines 1018–1026 explicitly call for "ephemeral message", "select/deselect via Discord select menus (paginated by spell level)", "Confirm/Cancel buttons" — none implemented.
- ⚠️ Encounter-status guard ("only available out of combat") not implemented in service (spec line 1018) — Cast's ritual path checks `EncounterStatus="active"`, but `PrepareSpells` doesn't.
- Tests: `internal/combat/preparation_test.go` (count validation, always-prepared exclusion, slot-level filtering, full prep flow).

### Phase 66a — Sorcery Points & Framework [WARN handler]
- ✅ Sorcery point tracking via `feature_uses["sorcery-points"]` with `recharge: long` — `internal/combat/sorcery.go:172` `ParseFeatureUses`.
- ✅ Font of Magic slot↔point conversion: `internal/combat/sorcery.go:200` `FontOfMagicConvertSlot`, `:243` `FontOfMagicCreateSlot`. Costs `2/3/5/6/7` (slotCreationCosts at `:28`).
- ✅ Sorcerer level 2+ gate, max-cap, bonus-action cost: `sorcery.go:151` `validateFontOfMagic`.
- ✅ Metamagic-per-spell rule with Empowered exception: `internal/combat/sorcery.go:78–88`.
- ✅ Per-option cost validation + total deduction: `internal/combat/spellcasting.go:467–486`, deducted at `:646–653`.
- ❌ `/bonus font-of-magic` not wired — `bonus` command is in the stub list at `internal/discord/router.go:199`; no Discord handler invokes `FontOfMagicConvertSlot`/`FontOfMagicCreateSlot`.
- Tests: `internal/combat/sorcery_test.go` (cost calculation, validation, conversion both directions, max-cap, level gate).

### Phase 66b — Individual Metamagic Options [PASS service]
- ✅ All 8 SRD options validated and applied: `internal/combat/metamagic.go:14` `ValidateMetamagicOptions` dispatches to per-option validators (`:23`).
  - careful: AoE+save check (`:60`). Effect: `result.CarefulSpellCreatures = chaModMin1` (`:200`).
  - distant: range>0 or touch (`:70`). Effect: `ApplyDistantSpell` doubles range, touch→30ft (`:136`).
  - empowered: damage check (`:80`). Effect: `IsEmpowered + EmpoweredRerolls = chaModMin1` (`:204`).
  - extended: duration ≥1 minute (`:87`). Effect: `ApplyExtendedSpell` doubles duration, capped at 24h (`:148`).
  - heightened: save check (`:94`). Effect: `IsHeightened=true` (`:209`).
  - quickened: 1-action check (`:101`). Effect: `isBonusAction=true` at `internal/combat/spellcasting.go:339` — bonus-action restriction still applies (correct per spec line 947).
  - subtle: no validation, sets `IsSubtle=true` (`:211`).
  - twinned: not self / not AoE (`:108`). Cost = spell level (1 for cantrip) at `sorcery.go:38`.
- ✅ Twinned target validation: `internal/combat/spellcasting.go:555–567` validates range to second target.
- ⚠️ **Empowered rerolls are not actually performed** — `EmpoweredRerolls` is recorded for display but no interactive reroll button or rerolled dice are surfaced. Spec line 944 expects "Bot shows rolled dice and prompts: 'Reroll which dice? [4] [2] [1] [6] [3]'". Currently it's display-only.
- ⚠️ **Careful target selection** — spec expects "Bot prompts: 'Pick allies to protect: [AR] [TH] [KL]' via buttons" (line 942). Currently `CarefulSpellCreatures` is just an integer in the result.
- ⚠️ **Heightened target selection** — spec expects "If multiple targets, bot prompts which target to heighten" (line 946). Currently `IsHeightened=true` is global; no per-target tracking.
- ⚠️ **Subtle bypassing Counterspell** — implementation does not surface `IsSubtle` to the Counterspell flow; Counterspell can still be triggered against a Subtle spell. Spec line 948 says it "cannot be Counterspelled".
- Tests: `internal/combat/metamagic_test.go`, `sorcery_test.go` (each option's validation; combos with empowered).

### Phase 67 — Spell Effect Zones (Encounter Zones) [FAIL]
- ✅ Schema match: `encounter_zones` migration matches spec lines 3210–3232 (encounter_id, source_combatant_id, source_spell, shape, origin_col/row, dimensions JSONB, anchor_mode, anchor_combatant_id, zone_type, overlay_color, marker_icon, requires_concentration, expires_at_round, zone_triggers JSONB, triggered_this_round JSONB).
- ✅ CRUD service: `internal/combat/zone.go:111` `CreateZone`, `:174` `DeleteZone`, `:180` `CleanupConcentrationZones`, `:185` `CleanupExpiredZones`, `:193` `CleanupEncounterZones`, `:198` `UpdateZoneAnchor`, `:225` `CheckZoneTriggers`, `:286` `ResetZoneTriggersForRound`, `:291` `ListZonesForEncounter`.
- ✅ ZoneDefinition catalog: `internal/combat/zone_definitions.go:24` `KnownZoneDefinitions` covers Fog Cloud, Spirit Guardians, Wall of Fire, Darkness, Cloud of Daggers, Moonbeam, Silence, Stinking Cloud (8 entries).
- ✅ Concentration zones cleaned on break: `BreakConcentrationFully` calls `DeleteConcentrationZonesByCombatant` (`concentration.go:445`).
- ✅ Map overlay rendering: `internal/gamemap/renderer/zone.go:9` `DrawZoneOverlays`. Workspace dashboard exposes zones at `internal/combat/workspace_handler.go:202–219`.
- ❌ **Cast does NOT auto-create zones.** `Cast` and `CastAoE` never call `LookupZoneDefinition` or `CreateZone`. `CreateZone` has zero non-test callers in the codebase. Spec line 2221: "when `/cast` resolves a spell with a persistent area effect ..., the backend inserts a row into `encounter_zones`."
- ❌ **Enter/leave triggers are never evaluated.** `CheckZoneTriggers` and `ResetZoneTriggersForRound` have no callers. Movement (`MoveHandler` → `Service.UpdateCombatantPosition`) does NOT consult zones. Spec line 2233: trigger fires once per creature per turn for damage zones — currently inert.
- ❌ **Combatant-anchored zones do not follow.** `UpdateZoneAnchor` is never called from `UpdateCombatantPosition`. Spec line 2225: "On any movement by the anchor combatant, the system updates `origin_col/origin_row`."
- ❌ **Round-start cleanup not wired.** `CleanupExpiredZones` and `ResetZoneTriggersForRound` should fire at round start; `initiative.go` round transitions don't call them.
- ❌ **Encounter-end cleanup not wired.** `CleanupEncounterZones` has no callers.
- ❌ **`ZoneDefinition` lacks `AnchorMode`.** Even if `Cast` called `LookupZoneDefinition` → `CreateZone`, every zone would default to `static`. Spirit Guardians (combatant-anchored) cannot be created correctly.
- ⚠️ Map legend integration: `legend.go` exists but no zone-source-spell + caster-name + duration-rounds-remaining text per spec line 906.
- Tests: `internal/combat/zone_test.go`, `concentration_integration_test.go` (CreateZone, anchor update, trigger gating, concentration cleanup) — all bypass the cast-side wiring.

### Phase 68 — Dynamic Fog of War [WARN]
- ✅ Symmetric line-of-sight: `internal/gamemap/renderer/fow.go:16` `shadowcast` uses center-to-center raycasting with wall-segment intersection, inclusive bounds on the wall (`u`) and exclusive on the ray (`t`) to ensure A↔B symmetry.
- ✅ Vision union across multiple sources: `internal/gamemap/renderer/fog_types.go:48` `ComputeVisibilityWithLights` unions visible cells from each `VisionSource` and `LightSource`.
- ✅ Vision modifier ranges: `fog_types.go:55–65` selects max of base/darkvision/blindsight/truesight as effective range.
- ⚠️ **Algorithm is not Albert Ford octant-recursive shadowcasting.** Comment at `fow.go:10` advertises the spec's named algorithm; actual implementation is per-tile raycasting (O(W·H·walls) per source, not O(visible cells)). Symmetric and correct, but slower on large maps and not what the spec says.
- ❌ **`Explored` (dim) state never set.** `ComputeVisibility` only assigns `Visible`; `Explored` constant exists at `fog_types.go:9` and is honored by `DrawFogOfWar` (`fog.go:33`), but no production path persists "previously seen" cells across renders. Spec line 2200: "Previously seen but currently out-of-range cells rendered as dim/greyed out".
- ❌ **`LightSource` lacks bright/dim two-range model.** Spec line 2206: "torches (20ft bright + 20ft dim)". Implementation has a single `RangeTiles` and treats all illuminated tiles as fully visible (`fog_types.go:185–189`).
- ❌ **Devil's Sight not modelled in renderer** (only in `combat/obscurement.go`). FoW does not let a Devil's-Sight creature see through magical_darkness zones at the renderer layer — the renderer doesn't know about magical darkness zones at all. Spec line 2208: "Devil's Sight — sees through magical darkness".
- ❌ **`RenderMap` is never called from production.** `grep "RenderMap" /home/ab/projects/DnDnD/ --include="*.go"` returns only the function definition and tests. There is no `/map` slash command. The dashboard's combat manager exposes raw zone data but no rendered PNG with fog. Spec section "Dynamic Fog of War" assumes a rendered map per update.
- ❌ **`VisionSources` is never populated from production.** `MapData.VisionSources` is constructed only in tests (`fow_test.go`); `ParseTiledJSON` (`parse.go:46`) never sets it.
- Tests: `internal/gamemap/renderer/fow_test.go` covers symmetry, range limits, darkvision, blindsight, truesight, light sources, three-state rendering.

### Phase 69 — Obscurement & Lighting Zones [WARN]
- ✅ Zone type → obscurement level: `internal/combat/obscurement.go:200` `ZoneObscurement` maps `heavy_obscurement`, `magical_darkness`, `darkness` → HeavilyObscured; `dim_light`, `light_obscurement` → LightlyObscured.
- ✅ Effective level vs vision: `internal/combat/obscurement.go:53` `EffectiveObscurement`. Darkvision downgrades `darkness`/`dim_light` (heavy→light, light→none); not other zone types. Devil's Sight covers all darkness within 120ft. Magical darkness ignores darkvision.
- ✅ Per-tile worst-zone lookup: `obscurement.go:154` `CombatantObscurement`.
- ✅ Attack integration: `internal/combat/attack.go:1091` computes attacker/target obscurement and applies to attack mode (advantage/disadvantage). Reasoning string surfaced via `ObscurementReasonString` (`obscurement.go:116`).
- ✅ Hide availability: `obscurement.go:109` `ObscurementAllowsHide` (lightly or heavily obscured — both grant hide).
- ❌ **`/check perception` does not auto-apply obscurement disadvantage.** `ObscurementCheckEffect` (`obscurement.go:93`) has zero non-test callers. Spec line 2254: "/check perception in lightly obscured zones: disadvantage (unless Darkvision negates)".
- ⚠️ Spec line 2240: darkvision treats Darkness as dim light. Implementation treats darkvision as full-vision in darkness zones (heavy→light, NOT heavy→dim with disadvantage). This may give too much benefit — the spec wants "Perception disadvantage only, not Blinded", which matches LightlyObscured, so this is correct.
- ⚠️ Obscurement is currently driven only by `encounter_zones` rows; there's no integration with map-baked lighting (the per-tile lighting brush from spec line 2284). Static lighting zones are not modelled in the codebase.
- Tests: `internal/combat/obscurement_test.go`, `obscurement_integration_test.go` (every zone type, vision interactions, attack roll modifier).

### Phase 70 — Reactions [PASS]
- ✅ `reaction_declarations` table CRUD: `internal/combat/reaction.go:27` `DeclareReaction`, `:105` `CancelReaction`, `:115` `CancelReactionByDescription`, `:135` `CancelAllReactions`, `:144` `ResolveReaction`.
- ✅ Multiple active declarations: backed by `ListActiveReactionDeclarationsByCombatant`.
- ✅ Persist until used/cancelled/encounter-end: `internal/combat/reaction.go:211` `CleanupReactionsOnEncounterEnd` (also has zero non-test callers — see Phase 67 failure mode for encounter-end).
- ✅ One-per-round enforcement on `turns.reaction_used`: `reaction.go:71–101` `CanDeclareReaction` enumerates current-round turns. `ResolveReaction` sets `turn.ReactionUsed=true` at `:181`.
- ✅ Reset at creature's turn start: each new turn row is created via `CreateTurn` at `internal/combat/initiative.go:650` with default `ReactionUsed=false` — implicit per-turn reset is correct.
- ✅ Surprised guard: `reaction.go:33–39` rejects declaration when surprised, matching spec.
- ✅ DM Active Reactions Panel: `internal/combat/reactions_panel.go:34` `ListReactionsForPanel` enriches with display name + reaction-used status. HTTP route exposed at `internal/combat/handler.go` (verified the panel handler exists at `reactions_panel_handler_test.go`).
- ✅ Discord wiring: `/reaction declare`, `/reaction cancel`, `/reaction cancel-all` — all handled by `internal/discord/reaction_handler.go:97` (`Handle` method routes to `handleDeclare`/`handleCancel`/`handleCancelAll`). Wired in `cmd/dndnd/discord_handlers.go:136`.
- ✅ DM-queue `KindReactionDeclaration` event posted on declaration; cancel emits strikethrough edit (`reaction_handler.go:154–175`, `cancelDMQueueItem` at `:200`).
- Tests: `internal/combat/reaction_test.go`, `reaction_integration_test.go`, `reactions_panel_test.go`, `reactions_panel_handler_test.go`, `internal/discord/reaction_handler_test.go`.

### Phase 71 — Readied Actions [WARN]
- ✅ `/action ready` cost & declaration: `internal/combat/readied_action.go:32` `ReadyAction` deducts the action and creates a reaction declaration with `is_readied_action=true` and optional `spell_name`/`spell_slot_level`.
- ✅ Fires using reaction (DM resolves): same pipeline as `/reaction` — `ResolveReaction` sets reaction_used.
- ✅ Expiry notice text: `readied_action.go:99–104` produces `⏳ Your readied action expired unused: "<desc>"` plus the spell-slot-lost line for spell-readied actions.
- ✅ `/status` integration: `FormatReadiedActionsStatus` (`readied_action.go:134`) lists active readied actions; status_handler wires this via `ReactionLookupAdapter` (`cmd/dndnd/discord_handlers.go:144`).
- ❌ **Slot is NOT actually expended on ready.** Spec line 1103: "the spell slot is expended when readying (not when releasing)". `ReadyAction` only writes `SpellName`/`SpellSlotLevel` to the declaration row at `readied_action.go:50–63`; no call to `deductAndPersistSlot` or `deductAndPersistPactSlot`. The expiry notice claims "slot lost" but the slot was never deducted.
- ❌ **Concentration is NOT held.** Spec line 1103: "the caster must hold concentration on the readied spell until the trigger fires". `ReadyAction` does not call `applyConcentrationOnCast` or set `concentration_spell_id`. If concentration breaks externally, no link from break → readied-spell expiry exists.
- ❌ **`ExpireReadiedActions` is never called from production.** Should fire at start of the creature's next turn; `initiative.go` round transitions don't call it. Spec lines 1103, 1105–1113.
- ❌ No `/action ready` Discord handler — `action` command is in the stub list at `internal/discord/router.go:199`. The freeform-action path (`internal/discord/action_handler.go`) handles `/action <text>` and `/action cancel` but not `/action ready`.
- Tests: `internal/combat/readied_action_test.go`, `readied_action_integration_test.go` cover declaration, action cost, spell-info storage, expiry notice — but do not test slot deduction (because there is none) or concentration linkage.

### Phase 72 — Counterspell Two-Step [WARN]
- ✅ DM trigger from panel: `internal/combat/counterspell.go:48` `TriggerCounterspell` validates declaration, hides enemy cast level (`CounterspellPrompt` lacks `EnemyCastLevel`).
- ✅ Available slots filter: `counterspell.go:308` `AvailableCounterspellSlots` returns sorted slots ≥ 3, plus pact slot if level ≥ 3.
- ✅ Slot ≥ enemy level → auto-counter: `counterspell.go:158`. Slot < enemy → DC 10 + enemy level → `needs_check` outcome at `:162–166`.
- ✅ Slot deducted on resolve: `counterspell.go:128–137` (pact-first, regular fallback). `ResolveReaction` consumes the reaction at `:139`.
- ✅ Ability check: `counterspell.go:178` `ResolveCounterspellCheck` — checkTotal vs DC. Failure means slot expended (already deducted), enemy spell resolves.
- ✅ Pass: `counterspell.go:223` — slot NOT spent, reaction NOT consumed (declaration stays active per spec line 1085 wording).
- ✅ Forfeit: `counterspell.go:254` — slot NOT spent, reaction CONSUMED (`ResolveReaction` at `:268`).
- ✅ HTTP routes: `internal/combat/handler.go:60–65` exposes trigger/resolve/check/pass/forfeit POST endpoints.
- ❌ **No Discord-side player prompt UI.** Spec lines 1095–1097: bot pings player in `#your-turn` with "Enemy is casting **[Spell Name]**. Use Counterspell? Pick a slot level:" + slot buttons + `[Pass]`. No production code in `internal/discord/` references counterspell. The DM dashboard can call the HTTP routes, but the player has no Discord-button interaction surface.
- ❌ **No retroactive removal of enemy spell effects.** Spec line 1101: "If the Counterspell succeeds, the DM retroactively removes the spell's effects." `CounterspellCountered` outcome doesn't undo applied damage/conditions. Acceptable as DM-side responsibility, but no helper exists.
- ⚠️ **`AvailableCounterspellSlots` requires slot level ≥ 3** (`counterspell.go:311`) — Counterspell itself is a 3rd-level spell so a 1st/2nd-level slot can never cast it. Correct.
- ⚠️ **Subtle Spell bypass not honored**: `IsSubtle` from the casting flow is not consulted by `TriggerCounterspell`. Spec line 948: subtle "cannot be Counterspelled". DM would have to manually skip the trigger.
- ⚠️ Auto-timeout to forfeit: spec line 1101 "If the player does not respond within the turn timeout, the Counterspell is forfeited." `ForfeitCounterspell` is exposed but no timer auto-fires it; DM or some scheduler must invoke.
- Tests: `internal/combat/counterspell_test.go` covers all five outcomes (auto-counter, needs-check pass/fail, pass, forfeit, slot-deduction, available-slot filter).

## Cross-cutting risks

### Wiring debt (highest priority)
The combat service implements every Phase 58–72 feature, but the chain Discord-command → handler → service → DB is broken at the handler layer for `/cast`, `/cast` AoE, `/prepare`, `/bonus font-of-magic`, and `/action ready`. End-to-end this means the playtest can declare reactions and break concentration via damage but cannot actually cast a spell that creates a zone, deducts a slot, applies an effect, or expends sorcery points. Visible symptom: every `grep` for production callers of `Cast/CastAoE/PrepareSpells/FontOfMagic*/ReadyAction/CreateZone/CheckZoneTriggers/UpdateZoneAnchor/ExpireReadiedActions/CleanupEncounterZones` returns only `_test.go` files. The phases.md "done when integration tests verify..." gates were satisfied by service-level integration tests, not end-to-end Discord flows.

### Concentration cleanup correctness
Phase 118 retrofit (line 488 of `concentration.go`) made the cleanup pipeline correct and fully tested when triggered. Triggers wired:
- New concentration spell replaces old via Cast → `applyConcentrationOnCast` (works in tests).
- Damage → `MaybeCreateConcentrationSaveOnDamage` → timer-resolved → `ResolveConcentrationSave` → `BreakConcentrationFully`. Wired through `cmd/dndnd/main.go:644`.
- Incapacitation → `ApplyCondition` hook → `BreakConcentrationFully`.
- Silence on entry → `Service.UpdateCombatantPosition:441` → `CheckSilenceBreaksConcentration`.
- Silence on zone create → `CreateZone:149` → `breakSilenceCaughtConcentrators`.
- Voluntary drop → not surfaced in any handler, but service path exists.
Trigger NOT wired:
- Cast-time Silence rejection (`ValidateSilenceZone` is dead code from a caller perspective).
- Readied-spell concentration link (Phase 71).
- Counterspell-success retroactive condition cleanup.

### Fog of War correctness
Beyond the wiring gap, the renderer has algorithmic deviations:
- Symmetric raycast vs. octant-recursive shadowcast — correct but slower; spec doc would be more accurate as "symmetric raycasting".
- Three visibility states are partially supported: `Visible` and `Unexplored`. `Explored` is rendered correctly but never produced (no historical state).
- Bright/dim distinction missing for light sources.
- DM mode (no fog) works (`renderer.go:33` only computes fog when sources are non-empty).

### Metamagic correctness
Service-side: 8 options, validation, costs, one-per-spell rule. Two failure modes:
- Subtle bypass of Counterspell not surfaced to `TriggerCounterspell`.
- Empowered/Careful/Heightened all expect interactive button prompts (reroll dice / pick allies / pick target). Currently only display flags are set; the DM (or player) cannot make those choices through any UI.
Stacking: the one-per-spell rule (with Empowered exception) is enforced in `ValidateMetamagic` at `sorcery.go:78–88`. Edge case: the rule only allows ≤1 non-empowered metamagic option, but does not enforce that two empowereds cannot be stacked (an empty-non-empowered count won't exceed 1). Empowered+Empowered would pass — a minor RAW deviation; spec line 944 phrasing implies a single Empowered Spell instance per cast.

### Zone enter/leave triggers
Spec calls for tracking triggers per-creature-per-turn via `triggered_this_round` JSONB. Service supports this (CheckZoneTriggers respects it; ResetZoneTriggersForRound clears it). But movement code path doesn't call CheckZoneTriggers, so Spirit Guardians, Cloud of Daggers, Moonbeam, and Wall of Fire never deal damage on entry. The data structure is fully ready; an integration call site is missing.

### Round and encounter lifecycle
- Round start: spec line 2229 ("at the start of each round, the system checks `expires_at_round`"). Implementation ready (`CleanupExpiredZones`); not called.
- Round start: `triggered_this_round` reset (`ResetZoneTriggersForRound`); not called.
- Encounter end: `CleanupEncounterZones`, `CleanupReactionsOnEncounterEnd` — both have zero non-test callers. Lingering rows from past encounters could pollute new ones.

## Recommended follow-ups

1. **Wire `/cast` end-to-end.** Build `internal/discord/cast_handler.go` calling `combat.Service.Cast` and `CastAoE`. Plumb spell+target+slot+ritual+metamagic flags from `commands.go:69`. After Cast, inspect `result.Concentration` and `cleanup.ConsolidatedMessage` for the 💨 line. Ping affected players via `CreatePendingSave` for AoE.
2. **Wire zone creation in `Cast`/`CastAoE`.** After successful cast, `LookupZoneDefinition(spell.Name)` and `CreateZone` if defined and the spell is a persistent AoE. Add `AnchorMode` and (optionally) `AnchorCombatantID` to `ZoneDefinition` so Spirit Guardians and Aura of Protection are combatant-anchored.
3. **Wire zone enter/leave triggers in movement.** In `Service.UpdateCombatantPosition`, after position changes, call `CheckZoneTriggers(... "enter")` and surface results. At turn start, call `CheckZoneTriggers(... "start_of_turn")` for the active combatant.
4. **Wire zone anchor follow.** In `Service.UpdateCombatantPosition`, call `UpdateZoneAnchor(combatantID, newCol, newRow)` so combatant-anchored zones move with the caster.
5. **Wire round-start and encounter-end cleanup.** Add `CleanupExpiredZones` + `ResetZoneTriggersForRound` to round-advance code. Add `CleanupEncounterZones` + `CleanupReactionsOnEncounterEnd` to encounter-end code.
6. **Wire silence-rejects-cast.** In `Cast` and `CastAoE`, look up zones at the caster's tile, call `ValidateSilenceZone(inSilence, spell)` before slot deduction.
7. **Wire `/prepare` UI.** Build `internal/discord/prepare_handler.go` using `GetPreparationInfo` + `PrepareSpells`. Use Discord paginated select menus by spell level. Gate by `encounter.status != 'active'`. Tests exist at the service layer; only the UI layer is missing.
8. **Wire `/bonus font-of-magic`.** Discord handler that calls `FontOfMagicConvertSlot` / `FontOfMagicCreateSlot`.
9. **Wire `/action ready`.** Discord handler that calls `ReadyAction` AND inside `ReadyAction` deduct the slot via `deductAndPersistSlot` and persist concentration via `applyConcentrationOnCast` (so concentration breaks remove the readied spell). Wire `ExpireReadiedActions` at the next turn-start of the readying combatant.
10. **Wire counterspell player prompt.** When `TriggerCounterspell` returns a `CounterspellPrompt`, post a Discord interaction in `#your-turn` with slot-level buttons and `[Pass]`. On button press, call the appropriate `ResolveCounterspell`/`PassCounterspell` route. Add a timeout watcher invoking `ForfeitCounterspell`.
11. **Honor Subtle Spell.** Pass `IsSubtle` through to `TriggerCounterspell` so the DM panel suppresses the counterspell prompt for subtle-cast spells (spec line 948).
12. **Add interactive metamagic prompts.** For Empowered (reroll dice), Careful (pick allies), Heightened (pick target). Mirror the counterspell two-step pattern.
13. **Persist `Explored` fog state.** Either store a per-encounter "explored cells" bitmap (or hash by tile) on the encounter row, or compute it at render time as the union of all cells that have been visible in any prior render. Set state to `Explored` for cells in the historical set but not in the current frame.
14. **Model torch bright/dim in `LightSource`.** Add `BrightTiles` / `DimTiles` fields. Bright tiles get `Visible`; dim tiles get `Visible` for darkvision creatures, otherwise contribute to a "dimly lit" zone.
15. **Wire perception-check obscurement.** In `internal/discord/check_handler.go`, when checkType=="perception", look up `CombatantObscurement` and apply `ObscurementCheckEffect` to set the roll mode.
16. **Add `requires_sight` LoS check for teleports.** When `info.RequiresSight=true`, run shadowcast or ray test from caster to destination through walls and reject if blocked.
17. **Tighten metamagic stacking.** Reject multiple `--empowered` flags (currently parsed as duplicates). Reject non-Empowered combinations explicitly with a clearer error than the current count check.
18. **Document algorithm.** Update `internal/gamemap/renderer/fow.go` comment to accurately describe symmetric raycast + wall-segment intersection rather than referencing Albert Ford's recursive algorithm. (Or implement the recursive version for performance on larger maps.)

## File index (relevant code paths)

- /home/ab/projects/DnDnD/internal/combat/spellcasting.go (1212 lines) — Cast, slots, materials, ritual, scaling
- /home/ab/projects/DnDnD/internal/combat/aoe.go (575) — AoE shapes, cover, save resolution
- /home/ab/projects/DnDnD/internal/combat/concentration.go (526) — DC, breaks, Silence, cleanup pipeline
- /home/ab/projects/DnDnD/internal/combat/teleport.go (97) — TeleportInfo, validation, DM-queue routing
- /home/ab/projects/DnDnD/internal/combat/preparation.go (409) — /prepare service
- /home/ab/projects/DnDnD/internal/combat/sorcery.go (316) — Sorcery points, Font of Magic, metamagic costs
- /home/ab/projects/DnDnD/internal/combat/metamagic.go (235) — Per-option validation + effects
- /home/ab/projects/DnDnD/internal/combat/zone.go (337) — Zone CRUD, triggers, anchor
- /home/ab/projects/DnDnD/internal/combat/zone_definitions.go (114) — KnownZoneDefinitions catalog (no AnchorMode)
- /home/ab/projects/DnDnD/internal/combat/obscurement.go (209) — Obscurement levels, vision interactions
- /home/ab/projects/DnDnD/internal/combat/reaction.go (213) — /reaction service
- /home/ab/projects/DnDnD/internal/combat/reactions_panel.go (104) — DM Active Reactions Panel
- /home/ab/projects/DnDnD/internal/combat/readied_action.go (162) — /action ready service
- /home/ab/projects/DnDnD/internal/combat/counterspell.go (320) — Two-step counterspell flow
- /home/ab/projects/DnDnD/internal/gamemap/renderer/fow.go (87) — Shadowcasting raycast
- /home/ab/projects/DnDnD/internal/gamemap/renderer/fog_types.go (87) — VisibilityState, ComputeVisibility
- /home/ab/projects/DnDnD/internal/gamemap/renderer/fog.go — Fog overlay drawing
- /home/ab/projects/DnDnD/internal/gamemap/renderer/zone.go — Zone overlay drawing
- /home/ab/projects/DnDnD/internal/discord/reaction_handler.go — /reaction wiring (only spell-related Discord handler that actually invokes combat service)
- /home/ab/projects/DnDnD/internal/discord/router.go (lines 198–217) — Game commands routed to stubs
- /home/ab/projects/DnDnD/internal/discord/commands.go (line 69) — /cast registration with metamagic flags
- /home/ab/projects/DnDnD/cmd/dndnd/discord_handlers.go (lines 101–158) — Wired handlers (move/fly/distance/done/check/save/rest/summon/recap/reaction/use/status/whisper/action) — note: cast/equip/inventory/prepare/bonus/give/loot/attune/unattune/retire/character/recap absent from constructor or stubbed.
- /home/ab/projects/DnDnD/cmd/dndnd/main.go:644 — `SetConcentrationResolver` wires the timer-driven CON-save cleanup.
