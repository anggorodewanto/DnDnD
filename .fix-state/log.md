# Orchestrator Log

## 2026-05-10 — startup
- Built tasks.md from SUMMARY.md.
- Tier counts: critical=9 (crit-01 split into 1a/1b/1c), high=10, medium=26, low=5, playtest=1. Total=51.
- Note: SUMMARY's "Critical 1" lists 16+ stub commands across three families (combat / spells / items+character). Split into crit-01a/b/c so workers can be parallelized without overlapping discord/router edits.
- Ordering: Critical batch A = crit-01b + crit-01c (different handler files, no overlap); crit-01a is its own batch (touches router + many handlers).
- crit-02 (`/setup`) sequenced first — single-file fix, lowest risk, unblocks playtest verifier later.
- Then critical batch B (pipeline plumbing): crit-03, crit-04, crit-05.
- Then critical batch C (auth/notifier): crit-06, crit-07.

## 2026-05-10 — crit-02 worker notes
- Out-of-scope follow-up surfaced while wiring SetupHandler: `/setup` still errors with "no campaign found for this server" when no campaign row exists for the guild — Phase 11 done-when bullet ("Campaign created on `/setup`") remains unmet. Suggest a small follow-up: have setupCampaignLookup.GetCampaignForSetup auto-create a campaign with default settings + the invoking DM as dmUserID when GetCampaignByGuildID returns sql.ErrNoRows, OR add a /campaign-create command. Either way, the SetupHandler.Handle "Error: no campaign found…" path is currently the next blocker in the playtest flow.

## 2026-05-10 — crit-05 worker notes
- Spotted in scope, deferred per task constraints (med-21, med-24): `internal/discord/move_handler.go:193,210` still hardcodes Medium creature size and 30ft maxSpeed in the prone-stand/crawl path. Halflings (25ft), Tabaxi (40ft via Feline Agility), and Large+ creatures will get wrong path costs. Same for fly_handler — no per-creature fly-speed lookup, just whatever movement remains. These are tracked in chunk3 cross-cutting risk #8.
- Spotted in scope, also deferred: `fly_handler.go` does no MaxAltitude check vs the creature's actual fly speed for the round (only validates against MovementRemainingFt). A 0-fly-speed walker can /fly today.
- Spotted in scope, also deferred: thrown-attack hand desync (chunk3 Phase 37 ⚠️). `Service.Attack` after a thrown weapon does not nil EquippedMainHand. Out of crit-05 scope.
- Untouched but visible: in-flight worker is mid-edit on internal/combat (attack.go, damage.go, attack_fes_test.go references `nearestAllyDistanceFt` which is undefined — likely crit-04 or crit-06). My changes to internal/combat are limited to turnvalidation.go (added "distance" to IsExemptCommand) + turnlock_integration_test.go (one extra True assertion). No surface overlap.

## 2026-05-10 — crit-03 worker notes
- Wired Service.ApplyDamage wrapper into all 6 production direct applyDamageHP call sites (aoe.go, channel_divinity.go Destroy Undead, dm_dashboard_undo.go undo+override, dm_dashboard_handler.go pending-action damage, turn_builder_handler.go enemy-turn). Wrapper now applies R/I/V → temp HP absorption → exhaustion HP-halving (level 4+) → exhaustion-6 instant death before the underlying applyDamageHP write.
- Out-of-scope follow-up (chunk4 cross-cutting "Damage-pipeline shortcut"): Heavy Armor Master, Tiefling fire resistance, and other PC-side R/I/V are still no-ops because `refdata.Combatant` / `refdata.Character` / PlayerCharacter row do not yet carry `damage_resistances/immunities/vulnerabilities` columns. ApplyDamage already plumbs PC-side R/I/V — it just resolves to empty slices today. Once Phase 45 wires `BuildFeatureDefinitions` into Attack() (crit-01a) AND a per-character resistance accumulator is added, the wrapper will surface those modifiers automatically. No code change needed in damage.go at that point — only a new `s.resolveDamageProfile` branch reading the PC's accumulated R/I/V.
- Co-existing in-flight worker (crit-01a) leaves attack.go + attack_fes_test.go in a non-compiling intermediate state (undefined `attackAbilityUsed`, `nearestAllyDistanceFt`, `fesDamageDice`). Verified my wrapper + tests by stashing those two files, running `go test ./internal/combat/... ./internal/dashboard/...` (all green), then restoring. Production `go build ./...` is green; only the combat package's _test_ build fails until crit-01a finishes plumbing those helpers. Not a regression from crit-03.

## 2026-05-10 — crit-04 worker notes
- Wired the Feature Effect System into `Service.Attack()` and `Service.attackImprovised()`. Both now call a new `s.populateAttackFES` helper that builds `BuildFeatureDefinitions(classes, feats)` and populates the chunk-4-listed EffectContext flags: IsRaging (combatant), AbilityUsed (str/dex via new attackAbilityUsed helper mirroring abilityModForWeapon), AllyWithinFt (new nearestAllyDistanceFt helper, same-side-only, dead-excluded), WearingArmor, OneHandedMeleeOnly. ResolveAttack now also rolls FES `ExtraDice` (Sneak Attack 3d6 etc.) into DamageTotal, doubling on crit. Order changed: DetectAdvantage now runs BEFORE the FES so HasAdvantage is the post-cancellation roll mode.
- Out-of-scope per task constraints — left as orchestrator follow-ups:
  - Magic-item passive defs (`magicitem.CollectItemFeatures`) are not threaded into BuildFeatureDefinitions. The combat package can't import magicitem (cycle: magicitem already imports combat). Resolution requires either moving the feature-def builder to a higher layer (e.g. cmd/dndnd/discord_handlers) or moving InventoryItem/AttunementSlot types into combat. Defer to a magic-item integration task.
  - Sneak Attack `OncePerTurn` filter is permissive: I pass an empty `UsedThisTurn` map. There is no per-turn feature-usage column on `turns` today, so multi-attack actions could trigger sneak attack twice in the same turn. Adding a sneak-attack-used flag to turns is a separate small migration.
  - `ApplyEvasion`, `ApplyUncannyDodge`, `ApplyGreatWeaponFighting` still have no production callers. Reaction-prompt wiring + damage-roll reroll hook are tracked as separate tasks per the chunk recommendations.
  - Rage end-of-turn auto-end (`ShouldRageEndOnTurnEnd`) and Wild Shape spellblock (`CanWildShapeSpellcast`) are NOT wired here.
  - Defense fighting style EffectModifyAC is collected into `ProcessorResult.ACModifier` but the AttackResult/DM-side AC pipeline ignores it. Out of scope.

## 2026-05-10 — crit-01c worker notes
- /retire posts a `dmqueue.KindRetireRequest` event to #dm-queue but does NOT flip the player_character row. The dashboard already supports a `created_via="retire"` flow in `internal/dashboard/approval_handler.go:248` (calls `RetireCharacter` which transitions to status="retired"), but that flow is tied to a separate "retire" submission card — it does NOT consume the new dm-queue retire-request item. Wiring the dm-queue resolve action → `registration.Service.Retire(playerCharacterID)` is a separate small task: needs the dashboard's queue-resolve handler to recognise `KindRetireRequest` and call `regSvc.Retire(pc.ID)`. Same gap exists for `KindUndoRequest` (Phase 97b dashboard undo handles state rollback but is invoked via its own UI panel, not via dm-queue resolve).

## 2026-05-10 — Critical tier closed (5 batches, 7 commits: 79a6edf, 49ab856, 23112a8, e671444, 456cb33)
- Integration verifier: `make test`, `make e2e`, `make playtest-replay TRANSCRIPT=$PWD/internal/playtest/testdata/sample.jsonl` all PASS.
- /setup, damage pipeline, FES, turn-lock, portal-token, player-notifier, /attack /bonus /shove /interact /deathsave, /cast /prepare /action ready, /undo /retire, /help /equip /inventory /give /attune /unattune /character /asi all wired.
- high-15 absorbed into crit-01c (the SetHelpHandler wiring landed alongside the inventory family).
- high-11 (spell handler family) is largely closed by crit-01b + crit-01a font-of-magic; the residue is med-25 (silence) + med-26 (zone CRUD wiring) which remain as separate medium tasks.

## High tier sequencing
- Batch H-main (sequential): high-09, high-10, high-13, high-14, high-17 — all main.go wiring; one implementer.
- Batch H-parallel (concurrent): high-08, high-12, high-16 — independent file scopes.
- high-11 marked done (spell handlers wired; silence/zone-CRUD live in med-25/26).
- high-15 already marked done in crit-01c.

## 2026-05-10 — high-main worker notes
- Cascading effects from high-main:
  - Added `discord.NewDefaultCampaignSettingsProvider(getCampaign)` constructor in `internal/discord/done_handler.go` (was struct-only with unexported field) so main.go can build the provider cleanly.
  - Added `HasRollLogger()` on /check, /save, /rest handlers and `HasMapRegenerator()` on /done handler — single-line introspection helpers matching the existing `MoveHandler.HasCharacterLookup()` pattern. Lets production-wiring tests detect silent-no-op gaps without poking at unexported fields.
  - Added `cmd/dndnd/dashboard_apis.go` to the Makefile `COVER_EXCLUDE` regex — same category as `discord_adapters.go` (wiring code, exercised end-to-end via the e2e suite, not unit-coverable).
- Out-of-scope follow-ups surfaced while wiring:
  - `loot.APIHandler.SetCombatLogFunc` and `shops.Handler.SetPostFunc` are still unwired in production — the dashboard buttons exist but no Discord post fires. Both need a campaign-scoped channel resolver + the queueing session injected. Out of high-13's "construct + mount" scope; tracked as a separate small follow-up.
  - The /done handler's `SetTurnNotifier`, `SetTurnAdvancer`, `SetCampaignProvider`, `SetPlayerLookup`, `SetImpactSummaryProvider` are still unwired in production; only mapRegenerator + campaignSettings landed (per high-10's narrow scope). PostCombatMap will fire, but auto-skip and turn-start prompts will not. Suggest a follow-up "high-10b: wire remaining done_handler setters" task.
  - rollHistoryLogger by-roller resolver does an O(C × N) scan per roll because there's no `GetCampaignByCharacterName` sqlc query. Realistic scale (≤5 campaigns × ≤6 PCs) makes this a non-issue, but a dedicated query would be the obvious optimization.
  - PartyRest service-time costs deferred per phase 84 already; my implementation respects the existing semantics (action/bonus-action cost in combat is still skipped).
- `make cover-check` ✅, `go test ./... -count=1` ✅, `make e2e` ✅, `make playtest-replay TRANSCRIPT=…/sample.jsonl` ✅.
