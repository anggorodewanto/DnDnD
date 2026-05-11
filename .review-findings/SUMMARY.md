# DnDnD Implementation Review — Aggregate Summary

Generated: 2026-05-10. Covers all 156 phases (1–121) in `docs/phases.md` against `docs/dnd-async-discord-spec.md`.

## Headline

The codebase has **strong domain implementations and weak production wiring**. Per-package services and unit tests faithfully cover the spec, but `cmd/dndnd/main.go` and the Discord router (`internal/discord/router.go`) ship with a sprawl of nil dependencies and stub handlers that prevent end users from reaching most features. Every phase from 23 onward is checked off in `phases.md`, but real users following the playtest quickstart cannot exercise core combat (`/attack`, `/cast`, `/bonus`, `/shove`, `/interact`, `/deathsave`, `/prepare`, `/help`, `/inventory`, `/give`, `/attune`, `/unattune`, `/equip`, `/character`) because their Discord handlers route to `StatusAwareStubHandler`.

The damage and Feature Effect System pipelines have a similar pattern: rich service code with table-driven tests, never reached by the production attack/cast paths because callers don't populate `input.Features` and write raw HP without going through `ApplyDamageResistances` / `AbsorbTempHP`.

Phase 121 (interactive playtest) is the only `[ ]` phase. 121.4 transcripts are deferred until first live playtest. All other phases checked, but ~30% have material gaps below.

## Severity tiers

### 🔴 Critical (block end-to-end gameplay)

1. **Slash command stubs in production** (chunk 3, 4, 5, 6, 7) — `/attack`, `/cast`, `/bonus`, `/shove`, `/deathsave`, `/interact`, `/prepare`, `/undo`, `/retire`, `/help`, `/inventory`, `/give`, `/attune`, `/unattune`, `/equip`, `/character` route to `internal/discord/registration_handler.go:418` "not yet implemented". Services exist; no `Set*Handler` call in `cmd/dndnd/main.go` or `cmd/dndnd/discord_handlers.go`.
2. **`/setup` broken** (chunk 2) — `cmd/dndnd/main.go:741` passes `nil` for setupHandler; not in fallback list. `/setup` returns "Unknown command", blocking the quickstart entirely.
3. **Damage pipeline bypasses Phase 42** (chunk 4) — every `applyDamageHP` call writes raw damage. Resistance, immunity, vulnerability, temp HP absorption, exhaustion HP-halving never apply in real combat.
4. **FES never reaches `Attack()`** (chunk 4) — `attack.go:456` reads `input.Features`; no production caller populates it. Sneak Attack, Rage damage, all fighting styles, Pack Tactics, Sacred Weapon, Vow of Enmity silently no-op.
5. **Turn lock not enforced** (chunk 3) — `internal/combat/turnlock_integration_test.go` validates the lock, but `move_handler.go:155` has a literal TODO; `/move`, `/fly`, `/distance` all bypass it. Out-of-turn rejection unreachable.
6. **Portal token validator nil** (chunk 7) — `cmd/dndnd/main.go:594` constructs `portal.NewHandler(logger, nil)` → nil-deref at `internal/portal/handler.go:94` on first portal hit. `RegistrationDeps.TokenFunc` is hardcoded to `"e2e-token"` (`main.go:730`).
7. **Player notifier nil for approvals** (chunk 2) — `cmd/dndnd/main.go:587` passes `nil` PlayerNotifier; players never receive Discord DM on approve/reject.

### 🟠 High (visible regressions, clear spec drift)

8. **`#character-cards` never auto-update** (chunk 2/4) — `OnCharacterUpdated` exists but no production mutation site invokes it. MEMORY note about deferred condition/concentration/exhaustion fields still applies — `buildCardData` in `internal/charactercard/service.go:203` doesn't populate those fields.
9. **`#roll-history` empty** (chunk 2) — every `RollHistoryLogger` argument is `nil` in `discord_handlers.go` ("no production adapter yet"). Phase 18 done-when unmet at runtime.
10. **`#combat-map` PNG never posted** (chunk 2) — `mapRegenerator` field on `discordHandlerDeps` declared, never set; `done_handler.PostCombatMap` returns silently.
11. **Spells: full handler family unwired** (chunk 5) — `Cast`, `CastAoE`, `PrepareSpells`, `FontOfMagicConvertSlot/CreateSlot`, `ReadyAction`, all zone CRUD have zero non-test callers. Concentration silence-on-cast `ValidateSilenceZone` defined but unused.
12. **Magic item active abilities unreachable** (chunk 7) — `inventory.UseActiveAbility`, `Service.DawnRecharge`, `inventory.CastIdentify`, `StudyItemDuringRest`, `DetectMagicItems` coded with tests but no Discord caller.
13. **Loot dashboard, item picker, shops, party rest unwired** (chunk 6) — `loot.NewAPIHandler`, `itempicker`, `shops`, `rest.PartyRestHandler` not constructed in `main.go`. Svelte UIs that call those endpoints 404.
14. **Phase 9b message queue bypassed** (chunk 1) — `MessageQueue` only instantiated in tests; production sends skip rate-limit retry. Splitting helper still works.
15. **`/help` is a stub in production** (chunk 8) — `NewHelpHandler` / `SetHelpHandler` defined but never called outside tests.
16. **DDB import mutates DB before DM approval** (chunk 7) — `internal/ddbimport/service.go:88-93` calls `UpdateCharacterFull` pre-approval; "diff message" is post-hoc only.
17. **OAuth/portal API surface gap** (chunk 7) — `portal.RegisterRoutes` invoked without `WithAPI` or `WithCharacterSheet`. Portal builder's `/portal/api/*` returns 404 and `/portal/character/{id}` unreachable.

### 🟡 Medium (partial spec, missing edge cases)

18. **Phase 25 initiative tracker** never posted to or updated in `#initiative-tracker`; only returned to dashboard.
19. **Phase 26b end-combat** missing concentration end, ammunition recovery, timer cancellation in cleanup.
20. **Phase 26a** no first-combatant ping on StartCombat.
21. **Phase 30 `/move`** hardcodes Medium size and 30ft maxSpeed (`move_handler.go:193,210`).
22. **Phase 37** thrown weapon not removed from hand.
23. **Phase 38** Reckless first-attack-only not enforced.
24. **Phase 55 OAs** detection logic exists; `move_handler.go` never calls `DetectOpportunityAttacks`. PC reach weapons unsupported (always 5ft).
25. **Phase 61 silence zone** post-hoc only via movement/zone-creation hooks; `Cast` doesn't pre-validate.
26. **Phase 67** `Cast` never invokes zone creation; `ZoneDefinition` lacks `AnchorMode` field.
27. **Phase 68 FoW** no historical "explored" state, no two-range light source model, comment claims shadowcasting but is per-tile raycast, `RenderMap` never called.
28. **Phase 71 readied actions** spell-name + slot-level recorded but slot not deducted, concentration not set.
29. **Phase 72 counterspell** HTTP routes exist; no Discord-side player prompt UI, no auto-timeout, Subtle Spell bypass not honored.
30. **Phase 66b metamagic** Empowered/Careful/Heightened set display flags only — no interactive prompts.
31. **Phase 75b** `stealth_disadv` not honored by `/check stealth`; heavy-armor speed penalty logged not applied.
32. **Phase 81** `target` option unused in `check_handler.go`; group/contested/passive checks unwired.
33. **Phase 82** `FeatureEffects` never populated in `save_handler.go:83-96`. Aura of Protection, Bless, magic-item save bonuses, dodge-on-DEX silently dropped.
34. **Phase 83a** rest applies unconditionally (`rest_handler.go:30` TODO for DM approval gate).
35. **Phase 84 `/use` and `/give` combat costs** explicitly deferred at `phases.md:485` but no follow-up phase tracks it. Dead `PotionBonusAction` field at `campaign/service.go:27`.
36. **Phase 89 ASI/feat** feat select-menu stub at `internal/discord/asi_handler.go:271` ("not yet available").
37. **Phase 99/101** Homebrew Content + Character Overview have backend, no Svelte UI.
38. **Phase 104b publisher fan-out partial** — `inventory` and `levelup` wired; `rest.Service` and `magicitem.Service` never constructed in `main.go`.
39. **Phase 21a** Svelte campaign UUID still hardcoded `00000000-…0001` (already noted at `phases.md:114`).
40. **Phase 15 Campaign Home** hardcodes `DMQueueCount` / `PendingApprovals` to 0 (`handler.go:149-152`) despite Phase 16 shipping a working `ListPendingApprovals`.
41. **Phase 11** no production code path calls `Service.CreateCampaign` — `/setup` errors when no campaign row exists.
42. **Phase 20 asset persistence** `ASSET_DATA_DIR` defaults to relative `data/assets`; fly volume mounts `/data` — undocumented override needed.
43. **Class features** Stunning Strike, Divine Smite, Uncanny Dodge auto-prompts missing entirely. Bardic Inspiration 30s/10min timeouts unwired. Rage no-attack auto-end, Wild Shape spellcasting block + auto-revert defined but unused.

### 🟢 Low / cosmetic

44. **Phase 5** `LogSpellValidationWarnings` never invoked at startup.
45. **Phase 3** spec wording says "15 conditions"; seeder ships 16 (includes `surprised`).
46. **Phase 6** spot-check uses generic `weapon-plus-1` ID instead of "+1 Longsword".
47. **TurnTimer.Stop** lacks `sync.Once`; double-stop panics.
48. **FES effect types** `EffectAura`, `EffectDMResolution`, `EffectReplaceRoll`, `EffectGrantProficiency` collected but no consumer.

## Phases with no findings (truly clean)

Foundation Phases 1, 2, 3 (modulo 45), 4, 6 (modulo 46), 7, 8, 9a, 10. Phase 21b/21c/21d map editor. Most pure-logic phases (29 pathfinding, 33 cover, 35 adv/disadv detection, 39 condition state machine, 43 death-save state machine, 44 FES engine internals). Phase 53, 78a/b/c, 80, 103 WS, 104 (modulo 38), 105/105b/105c, 106a–106f, 110/110a, 112, 117, 118/118b/118c, 119, 120, 120a.

## Phase 121 (only `[ ]`)

121.1, 121.2, 121.3, 121.5 done. 121.4 doc merged with all 11 scenarios but every row carries `Status: pending`; only `internal/playtest/testdata/sample.jsonl` checked in. Real transcripts deferred until first live playtest (line 823). This is the only outstanding work item per the phases.md tracker — but the gameplay-blocking gaps above (1–7) need to be fixed before that playtest can be productive.

## Recommended follow-up phases

A reasonable next set:

- **Phase 122: Production handler wiring sweep.** Fix items 1, 2, 7, 9, 10, 14, 15, 16, 17. One commit per package, each lands a `Set*Handler` call + integration test.
- **Phase 123: Damage pipeline integration.** Route every `applyDamageHP` call through `ApplyDamageResistances` + `AbsorbTempHP`. Wire exhaustion HP halving + death at level 6.
- **Phase 124: FES context plumbing.** Populate `input.Features`, `IsRaging`, `HasAdvantage`, `AllyWithinFt`, `WearingArmor` in `BuildAttackEffectContext` from authoritative state. End-to-end test that Sneak Attack + Rage damage actually fire.
- **Phase 125: Spell command wiring.** Land `Cast`/`CastAoE`/`Prepare`/`Ready` Discord handlers. Wire zone creation into `Cast`. Add `ZoneDefinition.AnchorMode`. Fix Phase 71 slot/concentration accounting.
- **Phase 126: Magic item activation.** Land `/use` active-ability, `/cast identify`, `/cast detect-magic`, dawn recharge timer, study-during-rest handlers.
- **Phase 127: Loot/shops/itempicker/party-rest wiring.** Mount API handlers, federate homebrew search, connect Svelte UIs.
- **Phase 128: Character card live updates.** Wire `OnCharacterUpdated` from every mutation site; populate Conditions/Concentration/Exhaustion in `buildCardData`.
- **Phase 129: Class-feature auto-prompts.** Stunning Strike, Divine Smite, Uncanny Dodge prompts. Bardic Inspiration timers.
- **Phase 130: Counterspell + Metamagic interactive UI.** Player slot prompt + Pass button, Empowered reroll prompt, Careful target picker, Heightened target.

## Detail files

- chunk1_foundation.md (106 lines) — Phases 1–10
- chunk2_campaign_maps.md (160 lines) — Phases 11–22
- chunk3_combat_core.md (173 lines) — Phases 23–38
- chunk4_conditions_classes.md (238 lines) — Phases 39–57
- chunk5_spells_reactions.md (289 lines) — Phases 58–72
- chunk6_turn_flow.md (217 lines) — Phases 73–87
- chunk7_dashboard_portal.md (237 lines) — Phases 88–102
- chunk8_polish_e2e.md (429 lines) — Phases 103–121

Total findings: ~1,870 lines, ~50 distinct issues. All findings include file:line references.
