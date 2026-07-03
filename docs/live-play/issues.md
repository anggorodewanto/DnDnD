# Issues Log — live play

Bugs, rough edges, and surprises found while running real games through the app.
One entry per issue. This is a **DM-side field journal**, distinct from the
AI-playtest harness's formal bug ledger — log freely here; promoting an issue to a
fixed + regression-tested item is a separate decision.

**Policy: fix-now TDD.** A bug found in live play gets a red/green TDD fix +
redeploy and an entry here. With a full table waiting, unblock the player first
(fast workaround), then run the fix — ideally delegated/backgrounded so the session
doesn't idle on a red/green cycle. See [`dm-rules.md`](dm-rules.md) +
[`big-party.md`](big-party.md) "Bugs mid-session."

Status: `OPEN` · `WORKAROUND` · `FIXED` · `WONTFIX` · `INFO` (not a bug, just a note).

| # | Date | Area | Severity | Status | Summary |
| --- | --- | --- | --- | --- | --- |
| ISSUE-061 | 2026-07-03 | combat / weapon mastery (off-hand two-weapon fighting) | medium | FIXED | **Off-hand (two-weapon-fighting) swings never applied their weapon-mastery on-hit effect — a known-Vex off-hand hit granted no advantage; Sap/Slow/Topple/Push likewise silently no-op'd off-hand.** Found live: Windreth's **off-hand shortsword** (Vex) hit on Zombie 2 gave no advantage the next round. Root cause: `applyMasteryEffects` (which places `vex_advantage`/`sap_disadvantage`/etc.) is called only by the **main-hand** `Attack` path (`internal/combat/attack.go:1265`); `OffhandAttack` resolved the swing (setting `result.MasteryProperty`) but **never called it** — mastery *detected* off-hand, never *applied*. **Nick unaffected** (handled inline via `nickAbsorbsBonusAction`). Fix (red/green TDD): added the same `applyMasteryEffects` call to `OffhandAttack` after `applyHitDamage`, before the scoped-advantage consume (which reads the stale pre-attack snapshot, so it never clears the fresh grant). Test `TestOffhandAttack_VexHitAppliesVexAdvantageToAttacker`; full combat package green; redeployed 08:01Z. **Loadout note (not a bug):** Windreth swings dagger(main)→shortsword(off-hand), so his dagger's **Nick** is wasted (Nick only helps a Light *off-hand* attack). Flip to **shortsword main / dagger off-hand** to get both main-hand Vex + a free off-hand dagger via Nick. See detail below. |
| ISSUE-060 | 2026-07-03 | builder / class-feature choices (Warlock pact boon + invocations) | medium | OPEN | **The character builder never surfaces Warlock Pact Boon (Chain/Blade/Tome/Talisman) or Eldritch Invocation choices — both stay as unresolved `choose_*` placeholder features and are never picked.** Found live checking Vale (Warlock 4, Fiend): patron is set (subclass `fiend`) but her `Pact Boon` feature is still the raw placeholder `mechanical_effect: choose_pact_boon` and `Eldritch Invocations` is `choose_2_eldritch_invocations` — no concrete picks, and the builder page offers no control for them. **Systemic, not warlock-specific:** the builder is a fixed 7-step wizard (`portal/svelte/src/CharacterBuilder.svelte:52` — Basics→Class→Scores→Skills→Equipment→Spells→Review) with **no class-features-choice step**; the ONLY `choose_*` wired in is **Expertise** (Rogue/Bard, `selectedExpertise`). Fighter Fighting Style (`seed_classes.go` `choose_fighting_style`), Sorcerer Metamagic, etc. are the same unresolved gap. Pact boons + invocations **don't exist as data anywhere** (no seed file, DB table, refdata query, or Svelte catalog); the `character.Feature.Choices` field exists but is never populated. **Scope = LARGE:** author the catalog (4 pact boons + ~18 invocations with prerequisites + level gates), add picker UI (new step or extend Skills, mirroring the Expertise pattern), add `PactBoon`/`Invocations` to `CharacterSubmission` (`internal/portal/builder_service.go:39-63`) + `DRAFT_FIELDS` (`portal/svelte/src/lib/builder-draft.js`), resolve picks into `Feature.Choices` in the derivation, and validate invocation prereqs/pact compatibility. **Workaround (this session):** set Vale's boon + 2 invocations directly on her sheet once the player picks — and add any spells/cantrips they grant (e.g. Pact of the Tome cantrips, Mask of Many Faces → at-will disguise self) so they're castable via slash commands. Chosen path per DM: workaround now, build the feature later. See detailed entry below. |
| ISSUE-059 | 2026-07-03 | dashboard / DM Queue (Resolve button fires no request) | minor | OPEN | **The DM-Queue "Resolve" button submits nothing — a queue item can't be resolved from the UI.** On `#dm-queue`, Open an item → the detail panel shows an optional "Outcome summary" field + a `Resolve` (`type="submit"`) button. Clicking Resolve does nothing: the row stays, the panel stays open, and the Chrome network log shows only the item's `GET /dashboard/queue/<id>` — **no `POST /dashboard/queue/<id>/resolve` is ever sent** (verified; tried ref-click ×3 with an empty summary, and again after confirming the renderer was responsive). Server side is fine: POSTing `/dashboard/queue/<id>/resolve` with `{"outcome":""}` from the authed page returns **204** and clears the item (`HandleResolve`→`notifier.Resolve`→`MarkResolved` + a cosmetic DM-only #dm-queue message edit — it does NOT post to #the-story/#in-character, so an empty outcome is safe). So this is client-only: the Svelte resolve form's submit isn't firing (likely an `on:submit|preventDefault` handler not wired to the button, or a guard that early-returns before `fetch`). **Impact:** the DM can't clear queue items via the dashboard; worked around by POSTing the endpoint directly. Found 07-03 while clearing 6 stale post-combat stubs (5 ISSUE-057 `enemy_turn_ready` orphans + Vale's stale `reaction_declaration`). **Repro:** `#dm-queue` → Open any pending item → click Resolve → nothing happens (Network panel: no POST). **Fix TBD:** inspect the resolve `<form on:submit>` in the dashboard queue Svelte component — ensure the submit button triggers the fetch and that an empty outcome is allowed; check whether the sibling `/reply` (whisper) and `/narrate` (skill-check) resolver forms share the defect. Minor (endpoint works; direct-POST workaround exists). |
| ISSUE-058 | 2026-07-02 | combat / action economy (cast then attack) | medium | OPEN | **A PC cast a leveled action-spell AND made a weapon attack in the same turn** (illegal by RAW — one Action, no Extra Attack). Found live: Vale (Warlock 3) `/cast shatter` (her Action, pact slot spent) at 12:31Z, then `/attack Ghoul dagger` at 12:39Z — the dagger `/attack` was **accepted and rolled** (nat-1 MISS, so **zero impact** this instance). Casting a spell with your Action should leave no Attack action. This is the sibling of **ISSUE-016** (`/done` phantom-attack warning after a cast), whose fix (`b108bf2`) claimed to zero `turn.AttacksRemaining` on **both** `Cast` and `CastAoE` — so either the **AoE path (`CastAoE`) doesn't actually zero `attacks_remaining`** (regression / never covered), or the `/attack` handler doesn't gate on `attacks_remaining > 0`. **Repro:** on your turn `/cast` a leveled AoE (Shatter), then `/attack` — the attack goes through. **Fix TBD (investigate):** confirm `CastAoE` zeroes `AttacksRemaining` like `Cast`, and/or make the weapon-attack path reject when the action was already spent on a spell (`action_spell_cast`/`action_used`). Not chased mid-session (the dagger missed, Vale's turn already ended). **Investigation (07-02, read-only agent + verified by hand):** HEAD source is actually CORRECT — `CastAoE` zeroes `turn.AttacksRemaining` at `internal/combat/aoe.go:649` in the action-cast branch, the pending-save creation (`aoe.go:606-627`) does NOT short-circuit the persist at `aoe.go:656`, and `/attack` gates via `ValidateResource`→`ErrResourceSpent` (`attack.go:1096`, `turnresources.go:59-62`). The b108bf2/ISSUE-016 fix is an ancestor of HEAD (`8809eb6`) and the **deployed** container (`dndnd-app-1`, built 07-02 11:42 UTC) includes it — Vale's cast (12:31Z) + attack (12:39Z) both ran on that binary, so it is **NOT deploy staleness**. No writer reseeds `AttacksRemaining` between the cast and the attack (only `attack.go:1163` post-attack loading + `turn_action_restore.go:72` the DM "Restore Action" button — neither triggered; the monster-save resolve chain never touches the turn). **So the root cause is not yet pinned** — the simple paths are all correct. Needs a focused live-DB repro (cast an AoE save-spell, resolve its monster saves, then `/attack`) to catch what left `attacks_remaining > 0`. **Also note a real test gap:** only `Cast` has a zero-attacks regression test (`TestCast_ActionSpell_ZeroesAttacksRemaining`); add `TestCastAoE_ActionSpell_ZeroesAttacksRemaining` (clone `TestCastAoE_Fireball` @ `aoe_test.go:449`, seed input `AttacksRemaining:1`, capture the persisted arg, assert `==0` + `ActionUsed` + `ActionSpellCast`). Zero-impact bug; fix on the user's word. |
| ISSUE-057 | 2026-07-02 | dm console / queue (enemy_turn_ready not cleared) | minor | RESOLVED 2026-07-02 | **`enemy_turn_ready` queue items are never cleared after the enemy's turn is run, so they pile up and mislead `next_step`.** Running an NPC turn via the Turn Builder (Review → Confirm & Post) + **End Turn** posts the attack, applies damage, and advances the turn correctly — but the `enemy_turn_ready` `dm_queue_items` row for that NPC **stays `pending`**. After Round 1 of the Buried-Gallery fight, all three enemies (Ghoul, Zombie 2, Zombie 1) each left a lingering stub in `GET /api/dm/situation` `pending[]`, so `next_step` kept reading *"Resolve Enemy Turn Ready from Ghoul: is up"* even though `state.current_turn` had already moved to a PC (Windreth, R2). **Mitigation in play:** the combat **workspace** Action Queue correctly shows *"No pending actions"* (it filters to the current turn), so it doesn't misdirect there — but the DM Console `next_step` + `pending[]` do. **Working rule:** trust `state.current_turn`, not `next_step`, when stale `enemy_turn_ready` stubs are present. **Fix TBD:** mark the `enemy_turn_ready` item resolved when its NPC's turn is confirmed/ended (mirror how player-action queue items auto-resolve). **Investigation (07-02, read-only agent) — root cause + fix located:** `postEnemyTurnReady` (`internal/combat/initiative.go:915-933`, called from `startTurn` `:779` gated on `IsNpc`) **discards the returned item ID** at `:926` (`_, _ = s.dmNotifier.Post(...)`), and `ExecuteEnemyTurn` (`turn_builder_handler.go:251-390`) never touches the dm_queue. **Proposed minimal fix (mirror `ForfeitPendingOAs`, `opportunity_attack.go:259-285`):** add a per-encounter `enemyTurnReadyByEncounter map[uuid.UUID]string` (+ mutex) to `Service` (next to the OA map `service.go:374-375`), capture the ID in `postEnemyTurnReady`, and a `resolveEnemyTurnReady(ctx, encID)` helper that pops the ID + calls `s.dmNotifier.Cancel(ctx, id, "enemy turn taken")`; invoke it in `ExecuteEnemyTurn` after the `turn.ActionUsed=true` persist (`turn_builder_handler.go:381`) AND at the top of `startTurn` (before posting the next item) for manual-advance robustness. Test: `TestExecuteEnemyTurn_ResolvesEnemyTurnReadyItem`. Minor/cosmetic (workspace Action Queue already filters correctly); fix on the user's word. **RESOLVED 2026-07-02:** implemented exactly as proposed — `Service.enemyTurnReadyByEncounter map[uuid.UUID]string` (+ `enemyTurnReadyMu`) in `service.go`; `postEnemyTurnReady` now captures the Post item ID via `recordEnemyTurnReady`; `resolveEnemyTurnReady(ctx, encID)` pops + `dmNotifier.Cancel(id, "enemy turn taken")`, called both in `ExecuteEnemyTurn` after the `ActionUsed` persist AND at the top of `createActiveTurn` (safety net for a manual dashboard turn-advance). Red/green TDD in `internal/combat/enemy_turn_ready_test.go` (4 tests, incl. the named integration test); combat suite green, pkg cover 91.6%. **Known limit (documented, unchanged):** the tracker is in-memory, so the **3 pre-fix stale stubs** from Round 1 (created before this binary) are not auto-cancelled by the restart — cancel them once from the dashboard/#dm-queue if they still clutter `pending[]`; all *new* enemy_turn_ready items self-clean going forward. **Deploy + orphan-clear DONE 2026-07-03:** redeployed the fix via `docker compose up -d --build app` (app rebuilt, Discord session reopened + all bot checks passed 02:28Z, combat state survived). The pre-fix stale stubs (by then **5** `enemy_turn_ready` orphans across R1–R2, IDs `e186bc44`/`0373b12c`/`c9df8402`/`f0bbb22f`/`c678f4ed`, plus Vale's stale `reaction_declaration` `a766a7f2`) were cleared post-combat → `pending: []`. Note: the DM-Queue UI "Resolve" button would not clear them (see **ISSUE-059**); they were resolved by POSTing `/dashboard/queue/<id>/resolve` directly (all → 204). |
| ISSUE-056 | 2026-07-02 | Combat — hidden/advantage not carried into combat | major | OPEN | **A hidden PC gets no advantage (so no Sneak Attack) on their first strike — the engine never knows they're hidden.** Windreth passed a pre-combat Stealth 13 and was narrated unseen ("first strike lands with advantage — Sneak Attack"), but `/attack` rolled a plain d20: no advantage, no sneak. Root cause: `CombatantFromCharacter` always sets `IsVisible=true` at combat init (`internal/combat/domain.go` ~L149) — pre-combat hidden status is dropped — **and** no DM/dashboard control exists to mark a combatant hidden or grant advantage on the next attack. The plumbing all exists (`IsVisible`/`AttackerHidden` → `DetectAdvantage`; a `/hide` Cunning Action that sets `IsVisible=false`; a `NextAttackAdvOverride` DM-override field consumed on the next `/attack`) but none is surfaced, and stealth doesn't carry in. **Workarounds:** (a) in combat the rogue spends Cunning Action `/hide` (bonus) → `IsVisible=false` → next `/attack` has advantage → sneak, then auto-reveal; (b) DM grants it. **This session:** Windreth's promised Sneak Attack granted retroactively (he rolls 2d6, DM applies to Zombie 2). Fix TBD: carry pre-combat Stealth into combat init as hidden, and/or add a DM dashboard toggle for hidden / next-attack advantage. |
| ISSUE-055 | 2026-07-02 | Combat — attack-log readability | minor | FIXED | **#combat-log never labeled Sneak Attack** — the rogue's extra dice were folded silently into the damage expression (e.g. `1d8+3+3d6`), so a player couldn't tell whether Sneak Attack had fired. Found in live play (Windreth's strike; the player asked "why no sneak dmg?"). Fix (TDD, display-only): `FormatAttackLog` now appends `— ⚡ Sneak Attack!` to the damage line when a fired once-per-turn effect is named "Sneak Attack" (name-keyed, **not** dice-keyed, so it never mislabels a different once-per-turn effect); fires on normal hits + auto-crits; no enemy HP/AC leaked. Added display-only `AttackResult.OncePerTurnEffectNames` (trigger logic never reads it, so *when* sneak fires is unchanged). Commit `8809eb6`, pushed + redeployed. |
| ISSUE-054 | 2026-07-02 | Combat — Start Combat token placement (spawn zones) | major | FIXED | **Enemies come out UNPLACED at Start Combat despite an enemy spawn zone — only PCs auto-place.** Found live building the Buried-Gallery fight: a 12×12 map with player (bottom) + enemy (top) spawn zones and an encounter (1 Ghoul + 2 Zombies + 3 PCs); `POST /api/combat/start` → 200 placed the 3 PCs at the player spawn but left all 3 enemies with empty `position` (Discord #combat-map showed no enemy tokens — the user flagged it mid-fight). **Workaround:** hand-placed every token in the Combat workspace (drag works there); DB positions now correct. Secondary: DM token drags update the DB but do **not** re-post the #combat-map (it refreshes on the next combat action; no manual re-post button). Also observed: the encounter-**builder** canvas froze `Page.captureScreenshot` repeatedly and its drag-to-place (HTML5 DnD) didn't register via automation, so placement had to happen in the workspace post-start. **Root cause:** `CreateEncounterFromTemplate` (`internal/combat/service.go`) seats PCs into the *player* spawn zone via `seatPCsInSpawnZones` but had **no enemy counterpart** — the authored *enemy* zone was parsed (`ParseSpawnZones`) yet never consumed, so a "Not placed" creature persisted with empty `position_col` (zero-value → renderer skips it / stacks it at A1). Intended behavior per `docs/tiled-maps.md` (spawn zones mark where players **and** enemies start). **Fix (TDD green; committed `0a05a28`, pushed, redeployed 07-02 — combat state survived the restart intact):** extracted `spawnzone.SpawnTilesForType(zones, type)` + added `enemySpawnQueue` — unplaced creatures now seat into the enemy zone (row-major, skipping any hand-placed tile); best-effort (no enemy zone → unchanged legacy). 2 red→green tests; `internal/combat` 91.6% / `internal/spawnzone` 96.6%; full suite pass. **Secondary (by design, not a bug):** `PostCombatMap` fires only from StartCombat + the Discord **`/map`** command — combat `/move` and DM workspace drags do **not** repost, so a stale board after DM placement is refreshed by a player running **`/map`** (no dashboard button; a drag-to-repost hook is a possible small follow-up). |
| ISSUE-053 | 2026-07-01 | Message Player (DM→player whisper) | major | FIXED | **`Send DM` returned HTTP 500 to the DM even though the whisper WAS delivered — the same "real work done, secondary step fails the status" shape as ISSUE-052.** Found live: replying to Vale's *"(to DM: do I need to roll?)"* via the dashboard **Message Player** panel — `POST /api/message-player/` → **500**, body `ERROR: insert or update on table "dm_player_messages" violates foreign key constraint "dm_player_messages_player_character_id_fkey" (SQLSTATE 23503)`. Root cause: `PlayerLookupAdapter.LookupPlayer` (`internal/messageplayer/adapters.go`) resolved the picker's `character_id` → the `player_characters` row correctly but returned `PlayerInfo{DiscordUserID, CampaignID}` **without setting `RowID`**, leaving it `uuid.Nil`. `Service.SendMessage` sends the Discord DM **first**, then `InsertDMMessage(PlayerCharacterID: info.RowID)` — so the DM lands, then the log insert 500s on the FK (no `player_characters` row with a nil id). Net effect: player got the message, DM saw a scary 500, nothing logged to history. (An earlier fix wired lookup-by-`character_id` + switched the insert to `info.RowID` per the FK comment, but never populated `RowID` — and `TestPlayerLookupAdapter_Success` asserted `DiscordUserID`/`CampaignID` only, so the gap slipped through.) **Fixed (TDD):** red — extended `TestPlayerLookupAdapter_Success` to assert `info.RowID == player_characters PK` (failed: `RowID = 000…000`); green — `LookupPlayer` now returns `RowID: row.ID`. `internal/messageplayer` green @ 96.4% + vet; rebuilt + redeployed. **Belt-and-suspenders:** because the DM is sent before the insert, a 500 here does NOT mean the whisper failed — verify delivery before any retry (do not double-DM). |
| ISSUE-052 | 2026-07-01 | dm-queue resolver (skill-check narration / whisper) | major | FIXED | **Resolving a dm-queue item returned an error to the DM (observed as HTTP 503) even though the work SUCCEEDED — the narration/whisper was already delivered to the player AND the item marked resolved; a naive retry then double-posts to the table.** Found live: resolving Vale's Performance-11 `skill_check_narration` via the #dm-queue inline resolver — the outcome stub delivered to #in-character (3:40 PM) and the item dropped out of `pending[]`, yet `POST /dashboard/queue/<id>/narrate` returned **503**. Root cause: `DefaultNotifier.Resolve` (`internal/dmqueue/notifier.go`) runs (1) deliver-to-player + (2) `MarkResolved` (both success-critical, both commit) then (3) `editHandled` — a **cosmetic** #dm-queue message edit (checkmark + button strip) — and **let its error gate the return status**. When the Discord edit flaked (rate-limit sleep/retry → upstream timeout surfaced to the browser as 503; the app itself only maps this route to 500), the resolve returned an error though both real effects were already done. Compounded by **non-idempotency**: `ResolveSkillCheckNarration` / `ResolveWhisper` had no status guard, so a retry re-ran delivery → double-post / double-DM. **Fixed (TDD):** (a) `Resolve` now treats `editHandled` as best-effort — `slog.Warn` + return nil once `MarkResolved` commits (a stray un-stripped button click is already guarded by the resolve handler); (b) `ResolveSkillCheckNarration` + `ResolveWhisper` short-circuit `if item.Status != StatusPending { return nil }` before delivery. Red→green: `TestResolveSkillCheckNarration_EditFailureStillResolves`, `…_AlreadyResolvedNoRedeliver`, `TestResolveWhisper_AlreadyResolvedNoRedeliver`; full `dmqueue` + `dashboard` packages + `go build ./...` + vet green. Rebuilt + redeployed. **Sibling note:** `Cancel` has the identical editHandled-gates-status shape (not yet hit in play) — left as-is; revisit if a cancel ever errors. **Recurrence (07-01, Vale's Deception-21 resolve):** the `…/narrate` resolve again surfaced a **503** to the browser though it fully SUCCEEDED — `dm_queue_items.status=resolved` + `resolved_at` set + outcome persisted (DB-verified), stub delivered to #in-character, and **no server ERROR/WARN logged**; the dashboard tab also froze the renderer twice around the call. Points to a client/upstream **timing** artifact (Discord rate-limit sleep during the deliver step, surfaced to the browser as 503) rather than the app's own error mapping — the `editHandled` best-effort fix holds server-side. Not chased as a fresh code bug (no server-side error to TDD). **Standing rule reaffirmed:** on any resolver non-2xx, verify delivery + the `resolved` state (DB / #in-character) **before any retry** — never retry blind (a blind retry double-delivers). **Recurrence (07-02, Forge's Investigation-19 resolve):** same cosmetic **503** on `…/narrate`, fully delivered (stub → #in-character 11:29 AM, verified on-screen) + item RESOLVED (queue empty); not retried. Three sightings post-fix now (07-01 ×2, 07-02) — a persistent client/Discord-timing artifact, **not** a server error (no ERROR/WARN logged); the server-side `editHandled` best-effort fix holds. Not chased further; the standing verify-before-retry rule is the mitigation. |
| ISSUE-051 | 2026-07-01 | rest / character derivation (hit-dice key) | major | FIXED | **`/rest short` hit-dice click failed: "rest failed: invalid hit die type: barbarian".** Found live: after ISSUE-050 unblocked rests, Forge (Barbarian 3) hit the hit-dice buttons and the bot rejected the spend. Root cause: `HitDiceRemaining` was persisted **keyed by class name** (`{"barbarian":3}`) instead of by die string (`{"d12":3}`). Two producers keyed by `c.Class` — `buildCharacterColumns` (the builder DB-persist path, `portal/builder_store_adapter.go:119`) and `DeriveStats` (`portal/derive_stats.go:99`) — but **every consumer keys by die string**: the rest service (`rest.go` → `character.HitDieValue("barbarian")` returns 0 → the error), the hit-dice buttons (`BuildHitDiceButtons`/`ParseHitDiceCustomID` embed the map key as `dieType`, so they even rendered "barbarian Skip/1/2/3"), `ddbimport`, and the sheet template. Both live rows were corrupt (Forge `{"barbarian":3}`, Vale `{"warlock":3}`); Vale's earlier `/rest short` only worked because she skipped hit dice (pact-slot restore path never calls `HitDieValue`). **Fixed (TDD):** both producers now key by `ClassHitDie(c.Class)` with `+=` so multiclass classes sharing a die accumulate under one key. Regression tests — barbarian ⇒ `d12`, fighter+paladin ⇒ `d10` sum — plus flipped 3 tests that had enshrined class-name keys. `make cover-check` green; committed `03642e2`; rebuilt + redeployed. The two corrupt live rows were re-keyed out of band (authorized one-off DB UPDATE, counts preserved + guarded on old value): Forge `{"d12":3}`, Vale `{"d8":3}`. Both PCs' `/rest short` hit-dice buttons now heal correctly. |
| ISSUE-050 | 2026-06-30 | combat / rest (auto-approve default) | major | FIXED | **`/rest` silently waited for DM approval even though the default is documented as auto-approve — players couldn't rest.** Found live: out of combat post-Cold-Vault, Vale + Forge each ran `/rest short` and got *"⏳ rest request sent to the DM. Your rest will apply once they approve it."* with no resolvable rest action (the dashboard queue resolver only acknowledges; it never delivers the hit-dice prompt) → stuck. Root cause = a **self-contradiction**: `Settings.AutoApproveRest *bool` field doc + `restAutoApproved`'s null-Settings branch (`rest_handler.go:217`) both say *nil ⇒ auto-approve (true)*, but `Settings.AutoApproveRestEnabled()` (`campaign/service.go:37`) returned **`false`** on nil, citing "per spec." This campaign's settings JSON has other keys (channel_ids/diagonal_rule/…) but **omits `auto_approve_rest`** → `Settings.Valid=true` so the null-branch is skipped → falls through to `AutoApproveRestEnabled()` → nil → **false** → gated. The existing `TestRestHandler_AutoApproveRest_DefaultIsTrue` passed only because it used **null** settings (hit the `!Valid` short-circuit), never exercising the valid-but-field-absent path. **Fixed (TDD):** `AutoApproveRestEnabled()` nil ⇒ **`true`** (matches the field contract + the null-Settings branch; a DM opts into gating by setting `auto_approve_rest=false`). Red/green `TestRestHandler_AutoApproveRest_ValidSettingsMissingField_DefaultsTrue` (valid JSON, field absent ⇒ HD prompt, not the wait message); discord+campaign+rest+situation packages green; rebuilt + redeployed. Cleared the 2 stale gated `rest_request` queue items via the dashboard; players re-run `/rest short` for the hit-dice buttons. **Known minor follow-up:** the auto-approve path still posts a `rest_request` notification (`rest_handler.go:191`, before the gate) that nothing auto-resolves, so each rest leaves a lingering `pending` notification — cosmetic queue noise, not blocking. |
| ISSUE-049 | 2026-06-30 | combat / action economy (undo grant) | medium | FIXED | **A granted action-undo left the spender's turn ACTION spent — they could not re-take it.** Sequel to ISSUE-048: after voiding Vale's misplaced Shatter saves + refunding her pact slot, she reported *"i cannot recast because my action is not undid."* Casting a spell with your action persists `action_used=true`, `action_spell_cast=true` (leveled), and **zeroes** `attacks_remaining` (Cast-a-Spell ≠ Attack) on the `turns` row (`spellcasting.go` §13). Nothing reachable from the DM dashboard set those back: `undo-last-action` (`dm_dashboard_undo.go`) restores only HP/position/conditions, the `undo_request` queue resolver only acknowledges, and `RefundResource` (`turnresources.go:100`) had just two callers — freeform-action cancel + Action Surge — neither a DM control nor applicable to a real spell cast. So the action economy gap left the player unable to act again even though the cast itself was fully undone (slot refunded, saves voided, no damage). **Fixed (TDD):** new `Service.RestoreTurnAction(enc, combatantID)` — targets the **active** turn (rejects a non-active combatant `ErrNotActiveCombatant` / an unspent action `ErrNoActionToRestore`, both 409), clears `action_used` + `action_spell_cast` and **reseeds** the fresh per-turn attack count via `ResolveTurnResources` (so the player may attack instead), and **leaves movement untouched** (restoring the action doesn't refund movement already spent). `POST /api/combat/{enc}/combatants/{combatantID}/restore-action` (added to **both** `DMDashboardHandler.RegisterRoutes` and the `main.go` production mount — ISSUE-043 two-list trap), a `dm_restore_action` audit + `#combat-log` correction that never surfaces HP, a `restoreCombatantAction` api, and a **"Restore Action — `<name>`"** button beside "Undo Last Action" in `CombatManager.svelte` (targets `activeTurnCombatant`). 3 Go (service + handler happy/409) + 1 vitest. `make cover-check` green; 749 vitest green; rebuilt + redeployed; **verified live** — clicking it set Vale's turn `action_used=f`, `action_spell_cast=f`, `attacks_remaining=1` (movement still 30), audit row written, no HP leaked. Vale can now recast. (ISSUE-048 + 049 together = the full "grant an AoE undo" flow, still 3 manual clicks — Cancel saves → refund slot → Restore Action; bundling them into one button remains a future enhancement.) |
| ISSUE-048 | 2026-06-30 | combat / AoE undo (DM grant) | medium | FIXED | **No in-app way to cancel a mid-flight AoE cast when granting a player's `/undo`.** Vale cast Shatter (R2) whose 10-ft blast caught her ally Forge; she `/undo`-requested to recast further right. The cast was mid-resolution — pact slot spent, two `pending_saves` (the Wight's DM-resolved + Forge's player `/save`), **zero damage applied** (AoE damage defers until every target's save resolves, ISSUE-044). But there was **no reachable path to void those saves**: the only pending-save endpoint is `…/resolve` (which *applies* damage), and the `undo_request` queue resolver + `undo-last-action` are no help — the queue resolver only acknowledges (writes an outcome note), and `undo-last-action` (`dm_dashboard_undo.go`) restores only HP/position/conditions, never refunds a slot or clears saves. The `ForfeitPendingSave` / `CancelAllPendingSavesByCombatant` SQL existed but had **no service method, handler, or button** (zero Go callers). So a granted AoE undo left un-cancelable pending saves (a duplicate "Resolve save" footgun + an oldest-first `/save` mis-attribution for the other target). **Fixed (TDD):** new `Service.CancelAoEPendingSave` forfeits every not-yet-`applied` save sharing the clicked save's `source` (`aoe:shatter:s2c3` → voids the whole blast in one click), `POST /api/combat/{enc}/pending-saves/{saveID}/cancel` (added to both `DMDashboardHandler.RegisterRoutes` **and** the `main.go` production mount — the ISSUE-043 two-list trap), a `dm_cancel_aoe` audit + `#combat-log` correction that never surfaces HP, and an amber **Cancel** button beside "Resolve save" in `PendingMonsterSavesPanel.svelte` + `cancelMonsterSave` api. 4 Go service/handler tests + 2 vitest (api). `make cover-check` green; 747 vitest green; rebuilt + redeployed; **verified live** — clicking Cancel voided **2** saves (`canceled=2`, the Wight + Forge), HP unchanged (Forge 25/32, Wight unmoved), then Vale's pact slot refunded 0→1 via the slot override + the `undo_request` queue item resolved. The slot refund + queue-resolve are still separate manual steps (this issue only fills the *cancel* gap); a one-click "grant AoE undo" that bundles all three is a possible future enhancement. |
| ISSUE-047 | 2026-06-30 | dashboard / combat board (map render) | medium | FIXED | **Combat Manager tokens rendered one grid row too low** vs the (correct) Discord PNG map — found live comparing the dashboard to the rendered map. Root cause: the Svelte board converted a combatant's **column** to 0-based via `colToIndex` ("A"→0) but read the **row** raw. `position_row` is **1-based** in the DB/API — the Go renderer proves it (`aoe.go:146` "PositionRow (1-based)" + unanimous `int(PositionRow)-1` in aoe/concentration/initiative/teleport/`service.go:1101`; `ParseCoordinate` "D4"→row 3). Zones (`origin_row`) are genuinely 0-based (`renderer/zone.go:32`, no `-1`) and were correctly left alone — which is *why* the asymmetry hid (col-letter needs the conversion, zone-row doesn't, combatant-row was the odd one out). **Latent twin bug:** dashboard drag-to-move wrote `position_row: dragRow` (0-based) → would corrupt the stored row by −1 on save (not yet hit; live moves go via the DM override form / player `/move`). **Fixed (TDD):** new `rowToIndex`/`indexToRow` helpers in `lib/combat.js` mirroring `colToIndex`/`indexToCol`, wired at all 5 combatant sites — token render (`CombatManager.svelte:557`), range circle (`:657`), tile hit-test (`combat.js:378`), stacked-token badge (`combat.js:407`), drag-write (`:868`, now `indexToRow`). Corrected 2 tests that encoded the bug + added `rowToIndex`/`indexToRow`/regression cases; 745 vitest green. No Go touched. Rebuilt bundle + redeployed; **pixel-verified on the live board** — Forge F1/Wight G1 → grid row 0.0, Vale G5 → row 4.05, matching Discord exactly. (EncounterBuilder uses a separate self-consistent all-0-based numeric convention — not affected.) |
| ISSUE-045 | 2026-06-30 | discord / reactions | minor | FIXED | `/reaction declare` confirmed the readied reaction **only to the invoking player** (ephemeral), so the rest of the table + the DM never saw "Vale readied hellish rebuke" live. A declared reaction is public table info — like `/action ready`. Fixed: the declare **success** now goes through `respondPublic` and names the player + reaction (`⚡ <name> readied a reaction: <desc>`); validation/error replies stay ephemeral (router convention). `mockInventorySession` now captures response `Flags` so visibility is testable; added `TestReactionHandler_Declare_RespondsPublicly` (success non-ephemeral) + `TestReactionHandler_Declare_ErrorStaysEphemeral`. Commit `cd6d360`, redeployed. **Surfaced a deeper gap (see ISSUE-046):** declaring a reaction is just an opaque note + the dashboard "Resolve" is bookkeeping — there is no path that actually *executes* a reaction spell (roll save, apply damage, spend slot). |
| ISSUE-046 | 2026-06-30 | combat / reactions (mechanics) | major | OPEN | **No path executes a declared reaction spell.** A `/reaction declare` stores opaque free text ("hellish rebuke if attacked") with no spell/damage/save linkage (`refdata.ReactionDeclaration.Description`; `SpellName`/`SpellSlotLevel` only auto-filled for counterspell). The dashboard "Resolve" button (`ResolveReaction`, `reaction.go:215`) is **pure bookkeeping** — sets `reaction_used=true` + status `used`; **no damage, no save, no slot spent, no dice**. `/cast` can't fire it either (turn-gated to the *current* combatant via `turnGate.AcquireAndRun`, `cast_handler.go:290`), so a warlock can't `/cast hellish-rebuke` during the attacker's turn. The spell **is** fully modelled (`seed_spells_1.go`: 2d10 fire, DEX save half, warlock). Net: when a reaction triggers the DM must hand-assemble it (player rolls the spell in text → DM rolls the NPC save → applies damage → spends the slot → clicks Resolve), and the only one-click "deal damage" control (`override/hp`) **leaks the enemy's secret HP** to #combat-log. Counterspell is the *only* reaction with a real mechanical flow. **Proposed fix:** an "execute reaction spell" resolver mirroring the monster-save resolver (ISSUE-043) — Active-Reaction button → roll NPC save + `ApplyDamage` (no HP leak) + spend caster slot + mark resolved. |
| ISSUE-044 | 2026-06-30 | combat / AoE spellcasting | **critical** | FIXED | AoE save-for-half damage NEVER applied in production. `ResolveAoEPendingSaves` listed saves via `ListPendingSavesByEncounter` (`WHERE status='pending'`), but the apply is driven *after* the last save flips `pending→rolled`, so the gate saw an empty set and returned `(nil,nil)` — no damage. Hit both the player `/save` path and the new DM resolver; masked in unit tests by a mock that returned rows the real SQL filtered out. First live AoE save-spell (Vale's Shatter) exposed it. Fixed: new `ListSavesByEncounter` (all statuses) feeds the gate; added an `applied` status so apply is idempotent + a `rolled` save can be re-driven without re-rolling; DB-backed regression suite (`aoe_save_apply_integration_test.go`). |
| ISSUE-043 | 2026-06-30 | combat / DM dashboard | major | FIXED | No DM-side way to resolve a **monster's** AoE save — only the player `/save` path and a bare-d20 turn-timeout existed, so a monster's `pending_saves` row stuck forever (and blocked the cast's damage via ISSUE-044's gate). Added `ResolveMonsterPendingSave` (engine rolls `d20 + creature save mod` vs DC, applies half-on-save, audits + `#combat-log` without leaking HP) + `GET/POST /api/combat/{enc}/pending-saves[/{id}/resolve]`, surfaced in the DM situation `pending[]` and a dashboard resolver (in + out of combat). **Sub-bug caught in live use:** the new routes were added to `DMDashboardHandler.RegisterRoutes` but not the production mount `mountCombatDashboardRoutes` (two hand-synced `/api/combat` lists) → 404 in the running app though unit tests passed; fixed + added a route-parity guard test (`cmd/dndnd/combat_routes_parity_test.go`). |
| ISSUE-042 | 2026-06-30 | combat / cast log (AoE) | minor | FIXED | Warlock AoE cast (Shatter) reported "Used 2nd-level slot (0 remaining)" in #in-character — wrong store + wrong wording. `FormatAoECastLog` read the untouched leveled-slot field (`SlotsRemaining`=0) instead of `PactSlotsRemaining`, and never branched on `UsedPactSlot`. Deduction was correct (pact slot 2→1, sheet right); cosmetic-only. Fixed: mirror `FormatCastLog`'s pact branch → "Used pact slot (1 remaining)" + pact-path test closing the `aoe_test.go` coverage gap (no `pact` case existed). |
| ISSUE-002 | 2026-06-24 | builder / persistence | major | FIXED | Full/half-caster `spell_slots` dropped at creation — `CreateCharacterRecord` never set it → portal-built wizard/cleric/etc. **could not cast leveled spells**. Fixed: persist standard slots in the canonical string-keyed `{current,max}` shape the `/cast` reader expects. |
| ISSUE-003 | 2026-06-24 | builder / spellcasting (frontend) | major | FIXED | Eldritch Knight (Fighter) & Arcane Trickster (Rogue) not recognized as casters by the frontend → **Spells step skipped entirely**. Fixed: subclass-aware `isSpellcaster`/budgets in JS + Go (INT third-casters from L3, EK/AT cantrip + spells-known + leveled tables); Spells step now shows with correct caps; server validation accepts the selections. |
| ISSUE-004 | 2026-06-24 | builder / AC | major | FIXED | Unarmored Defense never wired: builder never set `ac_formula`, so Barbarian (10+DEX+CON) & Monk (10+DEX+WIS) got **AC = 10+DEX**. Fixed: `unarmoredDefenseFormula` derives `"10 + DEX + CON"`/`"10 + DEX + WIS"` (the form `CalculateAC`/combat `RecalculateAC` parse, not the seed label) for unarmored barb/monk; fed into `DeriveStats` AC + persisted as `ac_formula`. Monk's UD voids shield bonus; armored falls back to armor AC. |
| ISSUE-005 | 2026-06-24 | builder / proficiency | minor→major | FIXED | Expertise (Rogue/Bard) never wired: combat reads an `"expertise"` proficiency key but the builder never collects it and `character.Proficiencies` has no Expertise field → wrong skill modifiers in play. **Fixed (TDD, `main` 6806bde):** added `Expertise []string` + `JackOfAllTrades` to `character.Proficiencies` (the JSONB `expertise` key `standard_actions.go:567` parses; `SkillModifier` `modifiers.go:25` doubles when a skill is in both expertise+proficient sets); builder collects N expert skills from proficient skills (Rogue L1=2, Bard L3=2) and persists them via `CreateCharacterRecord`; dashboard sheet + a latent levelup round-trip drop also closed. No schema change. Svelte bundle rebuilt. 452 vitest + cover-check green. (Out of scope: thieves'-tools expertise, ddbimport.) |
| ISSUE-006 | 2026-06-24 | builder / spellcasting | minor | FIXED | Level-1 Paladin/Ranger get a phantom L1 spell slot — `CalculateSpellSlots` half path uses `(level+1)/2` → 1 at L1 (half-casters get nothing until L2). Masked in the builder UI by an independent leveled-cap of 0, but wrong `spell_slots`/`max_spell_level` is stored and consumed elsewhere. **Fixed (TDD, `main` 558b2d4):** half-caster branch early-returns `nil` below level 2 (L1 Paladin/Ranger → no slots, max spell level 0); L2+ unchanged (L2 2×L1, L3 3×L1, L5 4×L1+2×L2). Downstream derive_stats / levelup verified. cover-check green. |
| ISSUE-007 | 2026-06-24 | builder / spellcasting (frontend+server) | major | FIXED | Multiclass **is** exposed (up to 4 class rows) and the spell *count* budget used the **primary class only** — frontend (`classEntries[0]`) and server (`primaryClassEntry`) — so secondary caster levels were ignored (budget too low) and a **non-caster primary hid the Spells step entirely** (e.g. Fighter 1 / Wizard 3). **Fixed (TDD, both sides, `main`):** `anyCaster` / `multiclassCantripCap` / `multiclassLeveledCap` (`spellcasting.js`) + `multiclassSpellBudget` (`spellbudget.go`) sum each class's own budget over **every** caster entry (5e computes known/prepared/cantrip counts per class; only spell *slots* combine); `CharacterBuilder.svelte` gate + caps now aggregate across `classEntries`. `max_spell_level` was already multiclass-correct (`DeriveStats` passes all classes) and was left untouched. 473 vitest + `make cover-check` green (overall 90.67%, portal 89.23%). Bundle rebuilt. |
| ISSUE-008 | 2026-06-24 | builder / persistence | blocker | FIXED (adapter) | Portal submit 500s — `characters.languages` is `TEXT[] NOT NULL`, builder sends no languages, `pq.Array(nil)` → SQL NULL → constraint violation. Blocked **all** portal builds. Coerced nil→`[]` in `CreateCharacterRecord`. Underlying collection gap tracked as ISSUE-009. |
| ISSUE-009 | 2026-06-24 | builder / language selection | minor | FIXED | Builder collected **no concrete languages** — `backgrounds.js` carried only a *count* of bonus languages, never the strings, so characters persisted with an empty language list. **Fixed (TDD, `main`, frontend-only):** new `portal/svelte/src/lib/languages.js` (standard+exotic master list; `raceBaseLanguages`/`availableLanguageChoices`/`assembleLanguages`/`bonusLanguageCount`); a Languages block in the Skills step shows the race's base languages (locked, from the `/api/races` `languages` already exposed) + exactly *background-bonus-count* picker slots; `gatherSubmission` ships `languages: assembleLanguages(raceBase, chosen)`; draft persistence wired (`builder-draft.js` allow-list + hydrate/snapshot) and a prune `$effect` keeps picks legal. No Go change (persistence path already wired). 494 vitest green; bundle rebuilt. |
| ISSUE-010 | 2026-06-24 | levelup / persistence | major | FIXED | Level-up persisted `spell_slots` as `map[int]int` → `{"1":4}` (`levelup/levelup.go:14`), but the cast reader `ParseSpellSlots` (`combat/divine_smite.go:71`) unmarshals into `map[string]SlotInfo` (`{current,max}`) → `{"1":4}` failed to unmarshal → `/cast` errored after any level-up. **Fixed (TDD, `main`):** `LevelUpResult.NewSpellSlots` is now `map[string]character.SlotInfo`; new `canonicalSpellSlots` helper converts the `CalculateSpellSlots` result to the string-keyed `{current,max}` shape (full on level-up; `nil` for non-casters so the `!= nil` guard skips the column). Regression test round-trips the emitted JSON through `combat.ParseSpellSlots`. cover-check green (overall 90.68%, levelup 90.45%). Slots emitted full (current==max) on level-up — matches the portal convention + long-rest assumption; prior `current` not preserved (the old shape was unparseable, so this is strictly an improvement). |
| ISSUE-011 | 2026-06-25 | builder / equipment (frontend) | major | FIXED | Portal-built characters persist with **nothing equipped** — `equipped_main_hand`/`off_hand`/`armor` empty, all inventory items `equipped:false` — even when the player equips a weapon/armor in the builder. Breaks `/attack` (no weapon), armor AC, and the card "Equipped" row. Go ingest + adapter persist `EquippedWeapon`/`WornArmor` fine; the drop is **frontend**. **Fixed (TDD, `main` 06a0ac5):** real cause was **async-load ordering** — `CharacterBuilder.svelte`'s reset `$effect`s cleared a valid `wornArmor`/`equippedWeapon` pick while the catalog (`allEquipment`) was still `[]` (e.g. right after a draft restore), because the option lists decided armor/weapon purely from the async catalog `category`. New `portal/svelte/src/lib/equip-selection.js` (`reconcileEquipPick` + category-OR-SRD-id fallback mirroring the Go `knownWeapons`/`knownArmor` maps) clears only on a genuine non-option, never on a transient catalog miss. Also wired `EquippedOffHand` (shield via `hasEquipmentItem(equipment,"shield")`). 461 vitest, bundle rebuilt, cover-check green. Workaround pre-deploy: player runs `/equip` in Discord. |
| ISSUE-012 | 2026-06-25 | character card / spellcasting | minor | FIXED | Discord character card + `/character` embed show **"Spell Slots: —" for warlocks** — they read only the `spell_slots` column and never fall back to `pact_magic_slots`. **Fixed (TDD, `main` 5090e02):** both surfaces now pact-aware — parse the canonical `character.PactMagicSlots` ({slot_level,current,max}) and render `Pact Magic: N × Lvl L`; a multiclass caster shows standard + pact joined by ` | `; non-casters keep `—`. `charactercard/format.go`+`service.go` (`CardData.PactMagicSlots`, `formatPactMagicSlots`, `parsePactMagicSlots`), `discord/character_handler.go` (`buildSpellSlotSummary` + a Spell Slots line in `buildCharacterEmbed`). cover-check green. |
| ISSUE-013 | 2026-06-25 | builder / submit (server) | blocker | FIXED | Friend's **barbarian / guild-artisan** submit 400s: `skill "insight" is not selectable for this class`. Root cause = **slug drift** between two hand-maintained Go background maps and the builder's kebab-case slugs. `backgroundSkillProficiencies` (`derive_stats.go`) had **no `guild-artisan`** case and keyed folk-hero as `"folk hero"` (space); both backgrounds therefore resolved to ∅ locked skills, so their PHB grants (insight+persuasion) were treated as off-list class picks and rejected. `backgroundStartingEquipment` (`starting_equipment.go`) had the same space-slug bug → those two backgrounds also silently got no starting-equipment pack. **Fixed (TDD, `main`):** both Go maps re-keyed to the exact 13 builder slugs (kebab-case) + `guild-artisan` added; two contract tests (`TestBackgroundSkillProficiencies_AllBuilderBackgrounds`, `TestBackgroundEquipmentPack_AllBuilderBackgrounds`) lock every builder slug so future drift fails CI; removed a stale test that asserted the old Title-Case `"Folk Hero"` input (never sent by the real builder — why the bug hid). cover-check green. **Deeper fix (SSOT) tracked separately.** |
| ISSUE-014 | 2026-06-25 | dm console / action log | medium | FIXED + DEPLOYED | DM Console didn't track player combat actions — spell casts + freeform actions post to #combat-log but were never written to `action_log`, so `GET /api/dm/situation` `timeline[]` showed nothing for them. **Fixed (`main` f1e3aeb, pushed, redeployed ~13:45 UTC):** a best-effort `recordCombatAction` helper (new `internal/combat/action_log_record.go`) now writes an `action_log` row at the success tail of every player combat path (`Cast`, `CastAoE`, `FreeformAction`, `Attack`, `attackImprovised`, `OffhandAttack`). **DM-side only** — player-facing #combat-log output is unchanged; the Console is behind DM auth. Save adjudication stays a manual DM roll (no auto #dm-queue item, no auto NPC save). |
| ISSUE-015 | 2026-06-26 | combat / ammunition | major | FIXED | Crossbow `/attack` falsely reports **"No bolts remaining"** with bolts in inventory — ammo match required name `"Bolts"` + type `"ammunition"`, but the builder seeds `{item_id:"crossbow-bolt", type:"gear"}` (slug drift, cf. ISSUE-013). **Fixed (TDD):** tolerant whole-word matcher on name/`item_id` (bolts/arrows), lossless full-inventory write (the old narrow re-marshal would have dropped every item's equipped/magic/charges fields once the shot succeeded), and a real empty quiver now routes to `#dm-queue` as a freeform action for lenient DM adjudication (attack resource not spent). Needs rebuild+restart to apply live. |
| ISSUE-015 | 2026-06-25 | dashboard / conditions | high | FIXED | Condition-shape mismatch between the dashboard and the engine, in two halves. **DISPLAY half FIXED** (`b108bf2`): the Combat Manager rendered a condition object as "[object Object]" because the engine stores conditions as objects (`{condition:"paralyzed",…}`) but the Svelte UI interpolated each entry as a string — new `conditionName()` helper now Title-Cases either an object's `.condition` or a bare string. **WRITE half FIXED (2026-06-26):** the workspace PATCH `/api/combat/{id}/combatants/{cid}/conditions` used to persist a bare string array (`["paralyzed"]`) that `parseConditions` can't unmarshal, so a button-added condition rendered but its mechanical effects (auto-crit, advantage-to-attackers, auto-fail STR/DEX saves) never fired. New server-side `reconcileConditionNames` (`workspace_handler.go`) maps the DM-supplied condition *names* into the canonical `[]CombatCondition` object shape — reusing the combatant's existing condition object when the name is already present (so a spell-applied duration/source/timing survives a re-send) and minting an indefinite `{condition: name}` for new ones, lowercased + de-duped. Frontend now works in lowercase canonical keys (`conditionKey` helper). |
| ISSUE-016 | 2026-06-25 | combat / spellcasting | medium | FIXED + DEPLOYED | `/done` falsely warned "you still have 1 attack" after a player cast a spell with their ACTION. Casting a spell is the Cast-a-Spell action, not the Attack action, so no weapon attack remains — but `Service.Cast`/`Service.CastAoE` consumed the action while leaving the seeded `attacks_remaining=1`, so the `/done` unused-resource check (and the "Remaining" summary) reported a phantom attack. **Fixed (`b108bf2`):** zero `turn.AttacksRemaining` when a spell consumes the action (cantrip or leveled); bonus-action casts left untouched (they keep the Attack action + its attacks). Found in live play: Vale (Warlock 3, no Extra Attack) cast Hold Person, then `/done` warned of an attack she never had. |
| ISSUE-017 | 2026-06-26 | refdata / item catalog (SSOT) | major (tech-debt) | FIXED | **Permanent SSOT fix** for the recurring slug/type/quantity drift class (ISSUE-013 background slugs, ISSUE-015 ammo, the builder-ammo follow-up). Delivered on branch `feat/item-catalog-ssot` in 5 phased commits: a canonical seeded **item catalog** (`refdata.ItemCatalog` + `items` table) now backs the builder inventory seeder, combat ammo derivation (via a weapon→ammo `ammunition_id` FK), and `/api/equipment`; the JS classifier is codegen'd from the Go catalog. The 5 fragmented sources + the hand-maintained Go/JS maps are deleted; two contract tests fail CI on re-drift. Full write-up in Details. |
| ISSUE-018 | 2026-06-27 | combat / enemy turn (action_log) | blocker | FIXED | **Turn Builder crashed executing any enemy turn:** `null value in column "before_state" of relation "action_log" violates not-null constraint`. `ExecuteEnemyTurn` (`turn_builder_handler.go`) omitted `BeforeState`+`AfterState` (both NOT NULL) in its `CreateActionLog` — unlike every other action_log writer. **Partial commit:** damage was applied but the turn never advanced and nothing logged → combat stuck on the enemy's turn. Found live (lead ghoul biting Vale). **Fixed (TDD):** snapshot the actor's combatant state before/after via the existing `snapshotCombatantState` helper, populate both columns. Red/green `TestExecuteEnemyTurn_PopulatesBeforeAndAfterState`; package green; assets/binary rebuilt + redeployed. Workaround used live: manual End Turn + resolve the dangling queue item. |
| ISSUE-019 | 2026-06-27 | dashboard / combat UX | minor | FIXED | **Turn Builder was undiscoverable** — the only way to run an NPC turn was to **right-click** the enemy token → "Plan Turn". A DM had no visual cue it existed (cost real table time hunting for it). **Fixed:** added a prominent gold **"⚔ Run Enemy Turn — <name>"** button to the combat right panel (above the Turn Queue), shown only when the current-turn combatant is an NPC (`activeTurnCombatant?.is_npc`); reuses the same `openTurnBuilder` handler as the right-click (no duplicate logic). Right-click menu kept. vitest green; Svelte bundle rebuilt + redeployed. |
| ISSUE-020 | 2026-06-27 | character sheet / HP source | medium | FIXED | **Character sheets showed stale full HP mid-combat.** Two HP stores: `characters.hp_current` (static base sheet) and `combatants.hp_current` (live combat snapshot). Combat carries HP in at start and **never writes back**, so every sheet that reads the `characters` row showed pre-fight HP during a fight (player saw Vale 24/24 while she was 19/24 and bloodied). **NOT a lost-damage bug** — the bite damage was correctly persisted on the combatant; the sheets just read the wrong table. **Fixed (TDD, 3 surfaces):** overlay the live combatant HP (HpCurrent/HpMax/TempHP only) when the character is in an active encounter — portal sheet (`hydrateFromCombatant`, which already overlaid conditions/exhaustion/concentration but forgot HP), Discord `/character` (mirrors the existing `/status` overlay), and the dashboard Character Overview API (`ListApprovedPartyCharacters`). All best-effort read-side; out of combat falls back to the row. The DM out-of-combat status editor's 409-in-combat write path (cf. status-editor feature) is untouched. cover-check green; redeployed; verified live (Party Overview now shows Vale 19/24). #character-cards excluded (static embed — would need a re-post per HP change). |
| ISSUE-021 | 2026-06-27 | combat / enemy turn (executor scope) | medium | OPEN | Enemy-turn executor resolves the **attack only** — it does NOT move the NPC into reach or advance the turn. Confirmed across two clean live runs (after the ISSUE-018 fix): the 2nd ghoul "bit" Forge from 35 ft with **no movement emitted**, and every enemy turn stayed `active` after Confirm & Post. DM must **drag the token into reach + click End Turn** manually. Distinct from ISSUE-018 (the `before_state` crash, fixed) — this runs cleanly but under-does the turn. ~~Minor: the "Turn Complete" summary renders the actor name blank (`**'s Turn**`).~~ **Name-blank tail FIXED 2026-06-27 (TDD):** ordering bug in `ExecuteEnemyTurn` (`turn_builder_handler.go`) — the HTTP handler rebuilds the `TurnPlan` from the POST body (`combatant_id`+`steps` only, no `display_name`), and the service called `FormatCombatLog(plan)` **one line before** backfilling `plan.DisplayName = combatant.DisplayName` → header rendered `**'s Turn**`. Swapped the two lines so DisplayName is set first. Red/green `TestExecuteEnemyTurn_CombatLogNamesActor`. Movement/turn-advance scope **still OPEN**. |
| ISSUE-022 | 2026-06-27 | combat / warlock pact slots (write-back) | medium | FIXED (other agent) | Combat pact-slot expenditure not written back to `characters.pact_magic_slots` — #combat-log showed "1 remaining" after Vale's Misty Step but the base row read `current: 0` (same two-store gap as ISSUE-020's HP). **Fixed by another agent this session**; logged here for the record. |
| ISSUE-024 | 2026-06-28 | combat / spellcasting (cast log) | minor | FIXED | Spell-attack cantrip #combat-log showed the damage **dice spec** (`💥 Damage: 1d8 necrotic`) instead of the **rolled value** — `FormatCastLog` (`spellcasting.go`) always printed `ScaledDamageDice`, never `DamageTotal`, and printed it even on a **miss** (no `Hit` guard). **Not a lost-damage bug** — `Cast` rolls the damage and `ApplyDamage` writes the target HP on a separate, correct path (verified live: Vale's Chill Touch took the lead ghoul G2 20→**13/22**, 7 necrotic, DB-confirmed); only the Discord string dropped the number. **Found live** (player asked why the log read "1d8 necrotic" with no value). **Fixed (TDD):** `FormatCastLog` now mirrors the weapon path — for a spell **attack** it prints `Damage: <DamageTotal> <type> (<dice>)` on a hit and **nothing** on a miss; save-based / no-attack spells keep the dice spec (their per-target total isn't a single value). Red/green `TestFormatCastLog_AttackHitShowsRolledDamage` + `_AttackMissShowsNoDamage`; combat + discord packages green, `make cover-check` green, rebuilt + redeployed. NB: any cast logged **before** this fix still reads the spec in #combat-log. |
| ISSUE-025 | 2026-06-28 | combat / action_log (player actions) | major | FIXED | **DM Console timeline blind to ALL player actions** since 2026-06-25. `recordCombatAction` (the ISSUE-014 writer) called `CreateActionLog` with nil `before_state`/`after_state` — both **NOT NULL** — so every player cast/attack/freeform insert silently violated the constraint and was swallowed (best-effort write). Only enemy-turn rows persisted (they populate state since the ISSUE-018 fix). **Same bug class as ISSUE-018, on the player path** → ISSUE-014 was effectively a no-op in prod. The unit mock accepted the nil columns the real Postgres rejects, so the suite stayed green while prod silently dropped every row. **Found** while syncing live-play state docs (timeline empty of player beats forced manual session-logging). **Fixed (TDD):** coerce nil/empty before/after → `{}` at the `CreateActionLog` choke point (`rawMessageOrEmptyObject`, guards all service-method callers); regression test `TestRecordCombatAction_PopulatesNonNullState` asserts non-null valid JSON state. cover-check green; rebuilt + redeployed. |
| ISSUE-026 | 2026-06-28 | combat / spell riders (effect model) | medium | OPEN (enhancement) | **Spell riders / ongoing effects aren't first-class timed effects**, so the DM hand-tracks them. Chill Touch's "can't regain HP until the caster's next turn", save-each-turn effects (ongoing poison, etc.), and other timed riders live in ad-hoc cast logic, not as a combatant effect carrying duration/started_round/expires_on/source_spell — so `/api/dm/situation` (which reads only `conditions`) can't surface them. Target: a first-class timed-effect model the engine ticks + the Console reads. Removes the residual hand-track in game-state.md ("Next action" Chill-Touch note). |
| ISSUE-027 | 2026-06-28 | dm console / NPC statblock in payload | medium | IMPLEMENTED 2026-06-28 | **DONE:** `/api/dm/situation` now carries a per-NPC `creature_summary` (attacks `{name,to_hit,damage,damage_type,reach_ft,range_ft}` + `recharge_abilities[]` + `has_legendary`/`legendary_budget`/`has_lair`), so an enemy turn can be read straight from the Console without opening the stat block. `combat.BuildCreatureTurnSummary` (reuses the Turn Builder parsers) → adapter maps to `situation.CreatureSummary`; PCs / movesetless NPCs omit the field; per-ref memo avoids refetching shared creatures. Red/green TDD, cover-check green. **ISSUE-021 (executor auto-move/advance) intentionally left OPEN** — DM direction: run NPC turns manually, no auto-advance. |
| ISSUE-028 | 2026-06-28 | dm console / in-character feed (platform) | major | OPEN (enhancement) | **Player #in-character roleplay is Discord-only** — never written to any DB table, absent from `/api/dm/situation` `timeline[]` (which carries only action_log + DM narration). The DM must read Discord directly (the reason Chrome-reading exists — see [`dm-rules.md`](dm-rules.md)). Largest situational gap. Target: ingest #in-character messages (Discord webhook/poll) into a roleplay timeline the Console surfaces. Large (platform integration). |
| ISSUE-029 | 2026-06-28 | dm console / out-of-combat state | medium | OPEN (enhancement) | **`/api/dm/situation` returns an empty `state` out of combat** — exploration progress (`encounters.explored_cells`), party scene/location, and prep readiness are invisible, so between fights the DM falls back to game-state.md notes. Target: surface exploration/scene state (and an exploration-mode view) so the Console isn't combat-only. |
| ISSUE-023 | 2026-06-27 | combat / enemy turn (combat-log damage) | minor | FIXED | Enemy-turn #combat-log reported the **raw rolled damage**, not the amount actually dealt after the target's resistance — a raging Forge took two ghoul bites that each logged "8 piercing damage" while Rage halved each to 4 (20→16→**12/32**), so the log overstated the hit. **Not a lost-damage bug** — HP was correct (resistance applied); only the log text was raw. **Found live** while running both ghoul turns (verified Forge `is_raging=t`, rage_rounds≈10 → 20−4−4=12 is correct). **Fixed (TDD):** `ExecuteEnemyTurn` now threads `ApplyDamage`'s `FinalDamage` back onto each attack step (new `AttackRollResult.FinalDamage`/`DamageResolved`), and `formatAttackLog` (new `attackDamagePhrase` helper) reports the dealt amount with an annotation when R/I/V changed it — `4 piercing damage (resisted — halved from 8)`, `0 … (immune — N negated)`, `N … (vulnerable — doubled from M)`; unchanged + pre-apply (plan preview) read plain as before. Red/green `TestFormatCombatLog_ResistedDamageShowsHalved`/`_ImmuneDamageShowsNegated`/`_ResolvedNoChangeReadsPlain` + `TestExecuteEnemyTurn_LogShowsResistedDamage`; combat package green, rebuilt + redeployed. NB: the two R2/R3 logs posted **before** this fix still read "8 piercing" in #combat-log (actual dealt was 4 each). |
| ISSUE-030 | 2026-06-28 | combat / turn advance (NPC turn dropped) | major | FIXED | **A live NPC's whole turn was silently dropped and the round skipped it.** `AdvanceTurn` (`internal/combat/initiative.go`) unconditionally `CompleteTurn`s the current turn with **no guard** that an NPC's enemy turn was actually executed. When "End Turn" fired on an NPC whose enemy-turn plan hadn't been run, the engine marked the turn completed with no attack and rolled on — and since that NPC was the last in initiative, the round advanced, looking like "the order skipped a ghoul." **Found live:** after Forge's R4 crit killed ghoul G2, the surviving ghoul G1 (init-last, alive 18/22) was reached (turn row + `enemy_turn_ready` created) but its R4 turn was then completed unrun (`action_used=false`, no `action_log`) and the board jumped to R5/Vale — G1's bite (which would likely have dropped Forge) vanished. **NOT caused by G2's death** — verified by tracing `AdvanceTurn`: with G2 alive the R5 rebuild just returns G2 first; the dropped turn is whichever combatant is current-but-unexecuted when a premature End-Turn fires (death is orthogonal). **Fixed (TDD):** `AdvanceTurn` now refuses (`ErrEnemyTurnNotExecuted` → **409** at the dashboard endpoint) to complete a current turn that is an NPC with `action_used=false` — `ExecuteEnemyTurn` sets `ActionUsed=true` even for a no-op plan, so this reliably means "End Turn before Run Enemy Turn." PCs exempt (they legitimately end with the action unused). The dashboard Turn Queue surfaces the 409 text, so the DM is told to run the enemy turn first instead of silently skipping the creature. Red/green `TestService_AdvanceTurn_RefusesUnexecutedEnemyTurn`/`_AllowsExecutedEnemyTurn` + `TestAdvanceTurn_UnexecutedEnemyTurnReturns409`; `make cover-check` green; rebuilt + redeployed. Live game left as-is per DM call (G1 acts normally on its R5 turn; no rewind). Distinct from ISSUE-021 (executor doesn't auto-move/advance) — this is the inverse: the engine *over*-advanced past an unrun NPC. |
| ISSUE-031 | 2026-06-28 | combat / action log (cleave) | minor | FIXED | **A 2024 Cleave-mastery secondary attack never reached the DM Console timeline** — it's in the Discord combat log but not in `action_log`. The Discord public log builds its message with `FormatAttackLog` (appends `→ ⚔️ Cleave hits/misses <2nd target>`), but the DB/timeline path uses `describeAttack` (`internal/combat/action_log_record.go`), which only rendered the **primary** target's outcome and dropped the cleave clause. So `GET /api/dm/situation` `timeline[]` showed `Forge … Greataxe — CRIT for 19` with no sign the cleave also hit the second ghoul. **Display-only** — the cleave's damage **was** applied to the live combatant HP (verified: G1 22→18 from the R4 cleave, consistent with its current 3/22 after later hits); only the timeline record was incomplete. **Found live:** the DM noticed the R4 crit's cleave (4 slashing to G1) was missing from the Console timeline. **Fixed (TDD):** new `describeCleave` helper appends ` — Cleave hits <name> for <n> <type>` / ` — Cleave misses <name>` to `describeAttack` when `result.CleaveAttack != nil`, mirroring `FormatAttackLog`; covers all three PC attack paths (normal/improvised/offhand) since they share the one formatter. Red/green `TestDescribeAttack_IncludesCleaveSecondaryAttack`; `make cover-check` green; rebuilt + redeployed. **Forward-only** — the pre-fix `action_log` row for Forge's R4 crit still lacks the cleave clause (not backfilled; live HP was already correct). |
| ISSUE-032 | 2026-06-28 | combat / weapon mastery (graze) | major | OPEN | **Graze miss-damage is invisible in BOTH the Discord log and the DM timeline** (same class as ISSUE-031 cleave, broader). 2024 Graze mastery (Greatsword/Glaive) deals ability-mod damage on a **miss** (`attack.go:768-775`), applied to target HP by `applyGrazeDamage` (`mastery.go:338`). But `describeAttack` (`action_log_record.go`) hits its default branch → logs **"missed"** and ignores `DamageTotal`; and `FormatAttackLog` (`attack.go:982`) gates its damage line behind `if result.Hit` with no graze branch → Discord shows **"MISS"** with no damage. Net: HP silently drops on a graze with no log line anywhere ("it missed but I lost HP"). **Damage is applied correctly — logging only.** Not live-relevant in "The Cellar" (no graze weapons: Forge=greataxe/cleave, Vale=dagger+crossbow). **Fix idea (TDD, two surfaces):** mirror the cleave fix — a `result.MasteryProperty=="graze" && DamageTotal>0` branch in both `FormatAttackLog` (a `→ Graze deals N <type>` line) and `describeAttack` (` — Graze for N`); keep the "missed" outcome but append the graze clause. |
| ISSUE-033 | 2026-06-28 | dm console / action log (cast outcomes) | medium | OPEN | **Spell damage / attack / save outcomes never reach the DM Console timeline.** `describeCast` (`action_log_record.go`) logs only `"<caster> cast <spell> on <target>"`; the damage roll, spell-attack hit/miss, and save result that Discord shows (e.g. `Chill Touch … Attack d20… Hit … Damage: 6 necrotic`) are absent from `/api/dm/situation` `timeline[]`. Same "Discord richer than Console" pattern as ISSUE-031, on the cast path. **Likely the intended ISSUE-014 scope** (log the *action*, not the outcome — PC attack action_log rows are also stored with empty `before/after/dice_rolls`), so this is a deliberate-vs-gap judgment call, not a clear regression. Impact: a DM scanning the Console can't see how much a spell did or whether a save landed without reading Discord. **Fix idea:** thread the resolved cast's damage/save summary into the `describeCast` description (or populate the action_log `dice_rolls`/`after_state` for casts + surface in the timeline). |
| ISSUE-034 | 2026-06-28 | dm console / action log (attack riders) | minor | OPEN | **On-hit attack riders surfaced in Discord are dropped from the one-line timeline summary.** `describeAttack` renders only hit/CRIT/miss + damage, so `FormatAttackLog`'s `InvisibilityBroken` line (`attack.go:999` — attacker becomes visible again) and the 2024 on-hit masteries Topple/Vex/Sap/Slow/Push never appear in `/api/dm/situation` `timeline[]`. Lower impact than ISSUE-032: Topple's Prone (and any applied condition) still shows in `state.combatants[].conditions`, and Vex/Sap/Slow are transient single-shot markers — so the DM isn't fully blind. **Fix idea:** append a compact rider suffix to `describeAttack` when these fire (mirror the cleave/graze approach), or accept the omission since the state view covers the durable parts. |
| ISSUE-035 | 2026-06-28 | combat / two-weapon fighting (thrown) | major | FIXED | **A two-dagger thrower can't throw the off-hand dagger after throwing the main one** — `/attack offhand:true thrown:true` rejects with "no main hand weapon equipped". RAW two-weapon fighting with two light thrown weapons is legal: throw the main-hand dagger with the Attack action, then throw the off-hand dagger with the bonus action. But a main-hand **thrown** attack auto-unequips the weapon (`attack.go:1293`, by design so one dagger can't be re-thrown forever), and `OffhandAttack`'s guard (`attack.go:1443`) then requires a main-hand weapon to still be equipped → the now-empty main hand trips it. **Found live:** Vale (2× dagger) threw her main dagger (R6, "hit for 2"), equipped the off-hand dagger, and the bot refused the off-hand throw. **Fixed (TDD):** a per-turn in-memory marker `mainHandThrownLightEffect` (same lifecycle as the Nick marker — set when a LIGHT melee weapon is thrown from the main hand, cleared at the combatant's turn start) lets `OffhandAttack` treat the TWF main-hand prerequisite as satisfied even though the weapon has left the hand. The empty-main-hand path is only allowed when that marker is present, so an illegal off-hand after a ranged/crossbow attack is still refused (regression test `TestServiceOffhandAttack_EmptyMainHandNoThrowRejected`); the no-throw message was also clarified to explain TWF needs a weapon in each hand. Red/green `TestServiceOffhandAttack_ThrownMainHandLightSatisfiesTWF`; `make cover-check` green; rebuilt + redeployed. **Live caveat:** the marker is in-memory, so a mid-turn redeploy (like this fix's own deploy) wipes it — Vale must re-`/equip` a dagger to her main hand once and re-run the off-hand throw; all future turns work directly (throw main → throw off-hand) within a process. |
| ISSUE-036 | 2026-06-28 | combat / death saves (turn flow) | major | FIXED | **A downed PC's turn was silently skipped with no death saving throw.** When initiative advanced to a PC at 0 HP (alive, `unconscious`+`prone`), `AdvanceTurn`→`skipOrActivate` (`initiative.go:553`) saw the `unconscious` condition, treated it as `IsIncapacitated`, and **skipped the turn** — never rolling/prompting the death save RAW requires at the **start of each of a dying creature's turns**. The death-save machinery existed (`/deathsave`, `RollDeathSave`, the 24h-timeout `AutoResolveTurn` at `timer_resolution.go:147`) but the normal advance path reached none of it. **Found live:** Forge dropped to 0 (R6 ghoul bite); his R7 turn auto-advanced to the ghoul with **0 death saves recorded** (DM Console + `action_log death_save` filter both empty). **Fixed (TDD, "Prompt the player" — player-chosen design):** `skipOrActivate` now detects a dying **PC** (`IsDying`, PC-only — dying NPCs still skip since their saves aren't player-rolled) *before* the incapacitated check and **activates** the turn flagged `TurnInfo.DeathSavePending`; the #your-turn turn-start prompt (`FormatTurnStartPrompt`/`…WithImpact`) swaps the action-resource list for **"You are dying — roll a death saving throw: /deathsave"**; and the `/deathsave` handler, when rolled on the dying PC's **own** current turn, **advances the turn** (off-turn rolls + Nat-20 wake-ups don't — `DeathSaveTurnAdvancer` wired to `combat.Service`). The 24h `AutoResolveTurn` stays as the inactivity fallback. Red/green: `TestAdvanceTurn_DyingPC_ActivatesTurnForDeathSave`, `…_DyingNPC_StillSkipped`, `TestFormatTurnStartPrompt*_DyingPC_ShowsDeathSavePrompt`, `TestDeathSaveHandler_OnCurrentTurn_AdvancesTurn`/`_OffTurn_DoesNotAdvance`/`_Nat20_DoesNotAdvance`. `make cover-check` green; rebuilt + redeployed. **Live caveat:** the fix is from R8 on — Forge's already-skipped R7 save is made up by his remote player rolling `/deathsave` once off-turn (records only). |
| ISSUE-037 | 2026-06-28 | dashboard / message player (DM whisper) | major | FIXED | **DM "Message Player" whisper failed for every player** with "player character not found", then (after a partial fix) an FK violation. Two stacked id-shape bugs: the picker sends each option's **`character_id`** (`MessagePlayerPanel.svelte` `value={c.character_id}`, from `/api/character-overview`), but (1) the lookup `GetPlayerCharacter` queried `player_characters WHERE id = $1` (the **PK**) → never matched → `ErrPlayerNotFound`; and (2) `dm_player_messages.player_character_id` **FK→`player_characters(id)`**, yet `SendMessage` inserted the raw `character_id` → `SQLSTATE 23503`. **Found live** sending Forge a death-save prompt. **Fixed (TDD, backend, character_id-in / PK-stored):** `PlayerLookupAdapter.LookupPlayer` now resolves character_id → campaign (`GetCharacter`) → row (`GetPlayerCharacterByCharacter`) and returns the player_characters PK as `PlayerInfo.RowID`; `SendMessage` persists `RowID` (FK-valid); `History` translates the incoming character_id → PK before querying (else the log view stays empty). Tests: `TestPlayerLookupAdapter_*` (resolves by character_id, PK in params), `TestService_Send_Success` (stores PK not character_id), `TestService_History_Delegates`/`_PlayerNotFoundReturnsEmpty`. `make cover-check` green; rebuilt + redeployed. **Live caveat:** the Discord DM is sent *before* the log insert, so Forge's death-save whisper **was delivered** on the attempt that hit the FK error (only the dashboard log row was lost) — not re-sent, to avoid double-messaging. |
| ISSUE-038 | 2026-06-28 | combat / end-combat HP carry-out | medium | FIXED | **Ending combat doesn't carry combat HP/conditions back to the sheets — a downed PC silently reads full HP out of combat.** Same two-store root as ISSUE-020 (`combatants.hp_current` snapshot vs `characters.hp_current` base row; combat never writes back), but at the **End Combat** boundary the read-side overlay is gone, so the Party page / sheets show the *undamaged* stored HP and drop combat-applied conditions. **Found live** (2026-06-28, end of "The Cellar"): Forge ended stabilized at 0/32 unconscious+prone and Vale at 7/24, but post-End-Combat both showed full HP / no conditions; the DM had to reconcile each PC **by hand** via Party → Edit status. **Fixed (TDD, 2026-06-29):** `EndCombat` now carries each **PC** combatant's final HP/temp-HP + post-clear conditions + exhaustion back to its `characters` row via `carryOutPCStatus` (`combat/service.go`), reusing the shared `UpdateCharacterVitals` write path the out-of-combat status editor uses ([[project_dm_out_of_combat_status_editor]]) — one writer, no duplication. A downed PC carries out **0 HP / unconscious** (combat-only `prone` is dropped by `ClearCombatConditions`, `unconscious` is preserved; no out-of-combat "dying" state); NPCs are encounter-scoped and skipped; HP is clamped to the sheet max. Red/green `TestEndCombat_CarriesOutPCStatusToCharacterRow` + carry-out error/clamp tests; `make cover-check` green. See Details. |
| ISSUE-039 | 2026-06-29 | dashboard / combat resources (feature uses) | medium | FIXED | **No DM editor for limited-use resources (Barbarian rage, ki, channel divinity, …) — a manually-set character could be stuck with the wrong remaining-uses count mid-fight.** `characters.feature_uses` (JSONB) was unexposed: only a party long rest touched it, and that resets *every* resource to max and 409s during combat. **Found live:** the AI DM had hand-set Forge's values during setup, leaving rage `{current:1, max:3}` — should have been **2** after one rage — with no in-app way to correct it. **Fixed (TDD, 2026-06-29):** audited mid-combat override `POST /api/combat/{enc}/override/character/{id}/feature-uses` `{feature,current,reason}` reusing `Service.SetFeaturePool` (preserves Max+Recharge), logged as `dm_override` + `#combat-log`; read-only `GET /api/character-overview/{id}/feature-uses` for prefill; `FeatureUsesEditor.svelte` in the Combat workspace **Manual Override** panel (PCs only). Rest command audited en route = **correct** (long rest → rage `current=max`, short rest untouched). `make cover-check` green; rebuilt + redeployed; used live to set Forge rage **1→2**. Out-of-combat editing still missing → **ISSUE-040**. See Details. |
| ISSUE-040 | 2026-06-30 | dashboard / character overview (feature uses) | minor | FIXED | **No out-of-combat editor for feature uses (rage / ki / channel divinity / …).** The ISSUE-039 override only mounts in the Combat workspace (gated on an active turn); the **Character Overview / Party** page has HP/conditions + spell/pact-slot editors but **no feature-uses editor**, so a DM can't correct or top up a character's limited-use pool between fights without starting combat or forcing a full long rest (which resets everything). **Fixed (TDD, 2026-06-30):** added `POST /api/character-overview/{id}/feature-uses` mirroring `UpdateSlots` — DM-authorized, **409 during active combat** (defers to the ISSUE-039 in-combat override), validates feature-present + `current ∈ [0,max]` (unlimited when `max<0`), persists via the existing `UpdateCharacterFeatureUses` write path; mounted the already-built `FeatureUsesEditor.svelte` on `CharacterOverview.svelte` (fetch-on-open via the existing read GET, parent picks in/out-of-combat exactly like `SlotEditor`). `make cover-check` green; bundle rebuilt + redeployed. See Details. |
| ISSUE-041 | 2026-06-30 | combat / rage (silent expiry) | medium | FIXED | **Rage lapses silently — the engine drops `is_raging` at end of turn (RAW: raged but didn't attack/take damage) with no #combat-log notice and no DM-timeline row, so the player thinks they're still raging (and resisting at half).** **Found live (Cold Vault R1):** Forge advanced + raged but ended his turn without attacking (keeper out of reach), so by RAW his rage ended at turn's end (`is_raging=f`). The bot posted **only** "enters a Rage!" — nothing when it dropped — and the keeper's next-turn longsword then landed **7 slashing in full** (would have been **3** halved had rage held / been visible). The drop was invisible to player and DM alike. **Fixed (TDD, 2026-06-30):** `maybeEndRageOnTurnEnd` now calls `notifyRageExpired`, mirroring the drop-to-0 (`notifyDroppedToZero`) dual-surface pattern — posts `FormatRageEnd(name, "no attack or damage this round")` to **#combat-log** and writes a **`rage_expired` action_log row** (DM Console timeline), parented to the rager's own (still-current) turn. Best-effort: nil notifier / lookup errors are swallowed so turn flow never blocks. `make cover-check` green; redeployed. See Details. |

---

## Details

### ISSUE-061 — Off-hand weapon-mastery on-hit effects never applied (FIXED)
- **Date:** 2026-07-03
- **Found by:** Windreth's player — off-hand shortsword (Vex) hit gave no advantage next round.
- **Root cause:** The on-hit weapon-mastery effects (Vex → `vex_advantage` on attacker, Sap → `sap_disadvantage` on target, Topple → CON-save/Prone, Slow → `slowed`, Push → forced move) are applied by `s.applyMasteryEffects(...)`. Only the **main-hand** `Attack` path called it (`internal/combat/attack.go:1265`, after `applyHitDamage`). `OffhandAttack` correctly resolved the swing — `ResolveAttack` even set `result.MasteryProperty` for a known off-hand mastery — but the method **never called `applyMasteryEffects`**, so the effect was detected and then dropped. **Nick** is the exception: it's an action-economy change handled inline in the off-hand path (`nickAbsorbsBonusAction`, :1544), so it worked.
- **Why it wasn't caught:** off-hand attacks are a distinct service method sharing `resolveAndPersistAttack` (roll + damage + action_log) but **not** the post-hit mastery/cleave block; the main-hand tests exercised mastery via `Attack`, so the gap sat in `OffhandAttack` only.
- **Fix:** mirror the main-hand path — call `s.applyMasteryEffects(ctx, cmd.Attacker, cmd.Target, &result, roller)` in `OffhandAttack` right after `applyHitDamage` and **before** `consumeHelpAdvantage`. Safe because `consumeHelpAdvantage`/`consumeSapDisadvantage` read the stale passed-in `cmd.Attacker.Conditions` snapshot (pre-attack), not a fresh DB read, so the just-placed grant survives — identical to the working main-hand ordering. (Cleave can't occur off-hand: off-hand weapons must be Light, Cleave weapons are Heavy/two-handed, so no `applyCleaveAttack` call was needed.)
- **Test:** `internal/combat/mastery_test.go` → `TestOffhandAttack_VexHitAppliesVexAdvantageToAttacker` (main-hand dagger + off-hand Vex shortsword the attacker knows → asserts `vex_advantage` written on the attacker, target-scoped). Red before, green after; full `./internal/combat/` suite green. Redeployed 08:01Z (`docker compose up -d --build app`).
- **Loadout implication (not a bug):** Windreth's kit is dagger(main, Nick) + shortsword(off-hand, Vex). Nick only benefits a **Light off-hand** attack, so main-hand Nick is inert; and his shortsword Vex only fired once this fix landed. Optimal: **shortsword main-hand** (Vex advantage, always worked via the main path) + **dagger off-hand** (Nick → the extra Light attack rides the Attack action, freeing the bonus action). Offered to the player; his call.

### ISSUE-001 — Warlock builder shows only cantrips (Pact Magic not derived)
- **Date:** 2026-06-24
- **Area:** portal character builder / spellcasting
- **Severity:** major — a warlock built via the web builder cannot pick any leveled
  spell, only cantrips. Renders the class' core mechanic unusable from the UI.
- **Status:** OPEN
- **Repro:** Build a single-class warlock (level ≥ 1, observed at level 3) in
  `/portal/create`. On the Spells step, cantrips (level 0) are selectable but all
  level 1–2 spells are unselectable/greyed.
- **Expected:** A level-3 warlock selects 2 cantrips **and** 4 known spells of
  level ≤ 2 (Pact Magic slot level at L3 = 2).
- **Actual:** Only cantrips selectable.
- **Root cause (verified):** Pact Magic is not folded into the builder's max
  spell level.
  - `character.CalculateSpellSlots` returns `nil` for a single-class warlock:
    the "half" branch is skipped (warlock is `"pact"`), then
    `CalculateCasterLevel` maps `"pact"` → 0 (`internal/character/spellslots.go:68`,
    `:129-145`).
  - The builder derives `MaxSpellLevel` solely from those (nil) slots →
    stays `0` (`internal/portal/derive_stats.go:97-103`).
  - Frontend: `levelsUpTo(0)` → `[]`, so `SpellPicker.isLevelSelectable` rejects
    every leveled spell while cantrips pass unconditionally
    (`portal/svelte/src/lib/spellcasting.js`, `.../spell-picker.js`).
  - `character.PactMagicSlotsForLevel` (`spellslots.go:112-124`) computes the
    correct pact slot level but is **never called** on this path.
- **Not a data bug:** warlock leveled spells are seeded — `SELECT level, count(*)
  FROM spells WHERE 'warlock' = ANY(classes) GROUP BY level;` → 9 at L1, 12 at L2,
  14 at L3, …
- **Fix idea:** Fold pact slot level into `MaxSpellLevel` for pact casters in
  `derive_stats.go` (consult `PactMagicSlotsForLevel`). Also verify the final
  character-create path actually persists `pact_magic_slots` so the built warlock
  can cast in play (separate from the UI gate). TDD + `make cover-check`.
- **Workaround:** finish the build cantrips-only and inject known spells +
  `pact_magic_slots` directly in the DB, or just fix it.
- **FIX (2026-06-24, TDD, on `main` working tree — not yet committed):** wired
  Pact Magic into the builder.
  - `internal/portal/derive_stats.go`: added `PactMagicSlots` to `DerivedStats` +
    a `pactMagicSlotsForClasses` helper; `DeriveStats` now raises `MaxSpellLevel`
    to the pact slot level (via `character.PactMagicSlotsForLevel`) for pact
    casters, combining with standard slots via max for multiclass.
  - `internal/portal/builder_store_adapter.go`: `CreateCharacterRecord` now
    persists `pact_magic_slots` for pact casters (non-warlocks unaffected).
  - Tests: 6 new red→green cases in `derive_stats_test.go` +
    `builder_store_adapter_test.go` (L3 warlock → MaxSpellLevel 2 + slots
    `{2,2,2}`; warlock/wizard multiclass → 3; non-casters nil; persistence).
  - `make cover-check` green (overall 90.63%, portal 88.61%). App rebuilt +
    restarted so the fix is live.

### ISSUE-002 — Standard-caster spell_slots may not persist at creation (UNCONFIRMED)
- **Date:** 2026-06-24
- **Area:** portal character builder / persistence
- **Severity:** unknown (potentially major if portal-built casters can't cast)
- **Status:** OPEN — **unconfirmed**, surfaced while fixing ISSUE-001.
- **Observation:** `BuilderStoreAdapter.CreateCharacterRecord`
  (`internal/portal/builder_store_adapter.go`) sets `PactMagicSlots` (after the
  ISSUE-001 fix) but never sets the generated `refdata.CreateCharacterParams.
  SpellSlots`, even though `DeriveStats` computes `SpellSlots` for full/half
  casters. Read paths appear to read the stored `spell_slots` column
  (`cmd/dndnd/dashboard_apis.go:324`).
- **To confirm:** build a wizard/cleric via the portal, approve, and check
  whether `/cast` / the sheet shows spell slots. If empty → real bug; fix by
  persisting `DeriveStats.SpellSlots` in the adapter (mirroring the pact fix). If
  slots appear → they're derived on read somewhere; close as INFO.

### ISSUE-004 — Unarmored Defense AC never wired (Barbarian/Monk) (FIXED)
- **Date:** 2026-06-24
- **Area:** portal character builder / AC derivation + persistence
- **Severity:** major — unarmored Barbarian/Monk got AC = 10 + DEX (missing
  CON/WIS), wrong at creation and at every combat AC recompute.
- **Status:** FIXED (TDD, `main`).
- **Root cause:** `DeriveStats` called `CalculateAC(..., "")` with an empty
  formula and `CreateCharacterRecord` never set `ac_formula`; combat
  `RecalculateAC` (`internal/combat/equip.go:387-419`) reads only `char.AcFormula`
  for unarmored defense. Only the Discord REST + DDB paths wrote it before.
- **Contract correction:** the live `ac_formula` value is the token form
  **`"10 + DEX + CON"` / `"10 + DEX + WIS"`** parsed by `evaluateACFormula`
  (`internal/character/stats.go:98`, mirrored in `equip.go:450`) — NOT the seed
  `mechanical_effect` label `ac_10_plus_dex_plus_con` (that label only drives
  feature definitions). A shield adds +2 unless the formula contains `WIS`
  (Monk UD voids it) — identical guard in `stats.go:70` and `equip.go:417`.
- **Fix:** `unarmoredDefenseFormula(classEntries, wornArmor, hasShield)` in
  `derive_stats.go` returns the CON form for an unarmored barbarian (shield ok),
  the WIS form for an unarmored, shieldless monk, else `""` (multiclass barb+monk
  prefers barbarian). `DeriveStats` feeds it to `CalculateAC`; `CreateCharacterRecord`
  persists it as `sql.NullString` (NULL for armored/non-UD). Tests in
  `derive_stats_test.go` + `builder_store_adapter_test.go` (barb 15, monk 15,
  barb+shield 17, armored barb → armor AC, fighter unchanged; persistence cases).
  `make cover-check` green (portal 89.30%). `DeriveAC` left untouched (no live
  callers).

### ISSUE-003 — EK/AT not recognized as casters in the builder (FIXED)
- **Date:** 2026-06-24
- **Area:** portal character builder (frontend gate + Go validation)
- **Severity:** major — an Eldritch Knight (Fighter) or Arcane Trickster (Rogue)
  built via the web builder got **no spell picker** (Spells step skipped). Worse
  than the warlock bug (warlock at least showed cantrips).
- **Status:** FIXED (TDD, `main`).
- **Root cause:** `CASTER_ABILITY` / `isSpellcaster` (`portal/svelte/src/lib/
  spellcasting.js`) keyed only on base class → `isCaster` false for fighter/rogue
  → `builder-steps.js` hid/skipped the Spells step. The Go spell-budget
  (`internal/portal/spellbudget.go`, used by `validateSpellCount`) likewise
  returned 0 for fighter/rogue, so even a shown picker would have been rejected on
  submit. Server `max_spell_level` (via `isThirdCasterSubclass` →
  `CalculateCasterLevel`) was already correct and untouched.
- **Fix:** made both sides subclass-aware. JS: `isThirdCaster(subclass, level)`
  (EK/AT slugs, level ≥ 3 = INT caster), `isSpellcaster`/
  `spellcastingAbilityForClass`/`cantripsKnown`/`leveledSpellCap` fall through to
  third-caster tables (EK 2→3 cantrips, AT 3→4, shared spells-known table);
  threaded subclass + level into `CharacterBuilder.svelte`. Go: mirrored
  `isThirdCaster` + third-caster tables in `spellbudget.go`; `spellCountCap`
  (`builder_service.go`) no longer bails for `SlotProgression=="none"` when EK/AT.
  Tests: Go `spellbudget_test.go` (EK/AT budgets + `validateSpellCount`), JS
  `spellcasting.test.js` (EK/AT casters, plain fighter/EK-L2 not). `npm test`
  441/441, `make cover-check` green (portal 89.12%). **Svelte bundle rebuilt**
  (`vite build`) since `internal/portal/assets/` is git-tracked.

### ISSUE-002 — Full/half-caster spell_slots dropped at creation (FIXED)
- **Date:** 2026-06-24
- **Area:** portal character builder / persistence
- **Severity:** major — portal-built wizard/cleric/sorcerer/druid/bard/paladin/
  ranger stored with `spell_slots = NULL`; `/cast` rejected them (no slots).
- **Status:** FIXED (TDD, `main`).
- **Root cause:** `DeriveStats` computes `SpellSlots` but the adapter
  `CreateCharacterRecord` (`internal/portal/builder_store_adapter.go`) only
  persisted `pact_magic_slots`, never standard `SpellSlots` → SQL NULL. Read paths
  (`/cast` → `parseIntKeyedSlots` → `ParseSpellSlots`) trust the stored column.
- **Fix:** added `spellSlotsForClasses` (`internal/portal/derive_stats.go`) that
  reuses `character.CalculateSpellSlots` and emits the canonical **string-keyed
  `{current,max}`** shape (fresh caster starts full, `current==max`); set
  `SpellSlots` in `CreateCharacterRecord` (NULL for non-casters). 3 red→green
  tests (Wizard L3, Paladin L2, Fighter L3 non-caster). `make cover-check` green
  (portal 89.05%, overall 90.66%). Verified the shape matches `ParseSpellSlots`
  (`combat/divine_smite.go:71`) + the dashboard `map[string]character.SlotInfo`
  reader, not level-up's incompatible `map[int]int` (→ ISSUE-010).

### ISSUE-008 — Portal submit 500s: languages NOT NULL violated (FIXED at write; collection gap OPEN)
- **Date:** 2026-06-24
- **Area:** portal character builder / persistence
- **Severity:** blocker — every portal "submit for DM approval" failed with HTTP 500.
- **Status:** FIXED (write-side) · underlying language-collection gap OPEN.
- **Repro:** Build any character in `/portal/create`, submit. Bot/app log:
  `ERROR creating character error="creating character: ERROR: null value in
  column "languages" of relation "characters" violates not-null constraint
  (SQLSTATE 23502)"`.
- **Root cause:** `db/migrations/20260310120006_create_characters.sql:28` →
  `languages TEXT[] NOT NULL`. Chain: submission `Languages []string`
  (`builder_service.go:48`, json `omitempty`) → `CreateCharacterParams.Languages`
  (`builder_service.go:510`) → adapter `Languages: p.Languages`
  (`builder_store_adapter.go:178`) → `pq.Array(arg.Languages)`
  (`refdata/characters.sql.go:105`). The Svelte builder **never collects concrete
  language strings** — `backgrounds.js` only carries a *count* of bonus languages
  — so the slice is always nil. `pq.GenericArray.Value()` returns SQL NULL for a
  nil slice → constraint violation. Guaranteed 500 for all portal builds; only
  surfaced now because this is the campaign's first portal-built character.
- **Fix (2026-06-24, TDD, `main` working tree, not committed):** in
  `CreateCharacterRecord` coerce `nil` → `[]string{}` before the insert
  (`pq.Array([]string{})` writes `'{}'`, non-null). 2 red→green tests in
  `builder_store_adapter_test.go` (nil → empty array; provided langs pass
  through). `make cover-check` green (portal 88.70%). App rebuilt + restarted.
- **Follow-up:** the builder collects no concrete languages — tracked separately
  as **ISSUE-009**.

### ISSUE-009 — Builder collects no concrete languages (only a count)
- **Date:** 2026-06-24
- **Area:** portal character builder / language selection
- **Severity:** minor — cosmetic today (languages aren't consumed in combat), but
  every portal-built character has an empty language list. Surfaced by ISSUE-008.
- **Status:** OPEN.
- **Detail:** `portal/svelte/src/lib/backgrounds.js` models bonus languages as an
  integer *count* (`languages: 2`, rendered via `formatLanguages`) and the builder
  never turns race base languages or that count into concrete strings.
  `CharacterSubmission.Languages` (`internal/portal/builder_service.go:48`,
  json `omitempty`) is therefore always empty, so `characters.languages` persists
  as `'{}'` (post ISSUE-008 fix; was a 500 before).
- **FIX (2026-06-25, TDD, `main`, frontend-only):** no Go/API change needed — the
  races endpoint already returns each race's base `languages` (Title-Cased, from
  `internal/refdata/seed_races.go` → `RaceInfo.Languages` → `/api/races`), and the
  persistence path already ships `submission.Languages`. New
  `portal/svelte/src/lib/languages.js` holds the standard+exotic master list and
  pure helpers `raceBaseLanguages` / `availableLanguageChoices` (case-insensitive
  exclusion) / `assembleLanguages` (case-insensitive de-dupe, first-seen order) /
  `bonusLanguageCount`. `CharacterBuilder.svelte` gained a Languages block in the
  **Skills step**: the race's base languages render as locked chips, then exactly
  `bonusLanguageCount(background)` `<select>` slots drawn from
  `availableLanguageChoices` let the player pick that many distinct bonus
  languages; `gatherSubmission` sets `languages: assembleLanguages(raceBase,
  chosenLanguages)`. Draft survival wired (`builder-draft.js` `DRAFT_FIELDS`
  allow-list + hydrate/snapshot) and a prune `$effect` drops picks that stop
  being valid when race/background changes. Tests: `languages.test.js` (21 cases).
  494 vitest green; svelte bundle rebuilt. **Remaining gap:** exotic-language
  gating (some are normally DM-granted) and class-bonus languages aren't modeled —
  the picker offers the full list; acceptable for now.

### ISSUE-007 — Multiclass spell count budget used primary class only (FIXED)
- **Date:** 2026-06-24 (fixed 2026-06-25)
- **Area:** portal character builder (frontend gate + budget) + server count cap
- **Severity:** major — confirmed: the builder exposes multiclass (an "add class"
  button, up to 4 class rows, `CharacterBuilder.svelte:882`).
- **Status:** FIXED (TDD, `main`).
- **Root cause:** the spell *count* budget was derived from the primary class
  only on both sides. Frontend: `isCaster` / `cantripCap` / `leveledCap` read
  `classEntries[0]` (`CharacterBuilder.svelte:520-528`). Server:
  `spellCountCap` read `primaryClassEntry` (`builder_service.go`). Two symptoms —
  (a) a multiclass caster (e.g. Wizard 3 / Cleric 1) got a budget too low because
  the secondary's cantrips/known/prepared were never added; (b) worse, a
  non-caster *primary* with a caster *secondary* (Fighter 1 / Wizard 3) made
  `isCaster` false → `builder-steps.js` hid the Spells step entirely.
- **Not the max spell level:** `DeriveStats` already passes **all** classes to
  `character.CalculateSpellSlots` (`derive_stats.go:102`), so `max_spell_level` /
  `spellSelectableLevels` (which spell *levels* are selectable) were already
  multiclass-correct. Left untouched.
- **Fix:** sum each class's own budget across **every** caster entry — 5e computes
  known/prepared/cantrip counts per class (only spell *slots* combine on the
  shared caster-level table). JS: new `anyCaster`, `multiclassCantripCap`,
  `multiclassLeveledCap` (`spellcasting.js`); the component's gate + caps now
  aggregate over `classEntries` and pass a per-ability modifier map so each entry
  uses its own casting ability. Go: new `multiclassSpellBudget`
  (`spellbudget.go`) reusing the exact `SlotProgression=="none" && !isThirdCaster`
  guard; `spellCountCap` delegates to it. Single-class behaviour is the one-term
  sum, unchanged.
- **Tests:** JS `spellcasting.test.js` (`anyCaster`, multiclass cantrip/leveled
  caps incl. non-caster-primary); Go `spellbudget_test.go`
  (`TestMulticlassSpellBudget`, `TestSpellCountCap_Multiclass`,
  `TestValidateSpellCount_Multiclass` — a Fighter1/Wizard3 submission at the
  wizard's budget now passes where the primary-only cap rejected it). 473 vitest +
  `make cover-check` green (overall 90.67%, portal 89.23%). Svelte bundle rebuilt
  (`internal/portal/assets/` is git-tracked).

### ISSUE-010 — Level-up wrote spell_slots in an unparseable shape (FIXED)
- **Date:** 2026-06-24 (fixed 2026-06-25)
- **Area:** level-up persistence vs the `/cast` read path
- **Severity:** major — any leveled caster that leveled up could no longer cast.
- **Status:** FIXED (TDD, `main`).
- **Root cause:** `CalculateLevelUp` built `NewSpellSlots` as `map[int]int`
  (`levelup/levelup.go`) and `service.go` marshaled it raw → `{"1":4,"2":2}`. The
  canonical reader `combat.ParseSpellSlots` (`combat/divine_smite.go:71`)
  unmarshals into `map[string]character.SlotInfo`
  (`{"1":{"current":4,"max":4}}`), so the number-shaped JSON failed with
  `cannot unmarshal number into Go value of type combat.SlotInfo` and `/cast`
  rejected the character.
- **Fix:** changed `LevelUpResult.NewSpellSlots` to
  `map[string]character.SlotInfo`; added `canonicalSpellSlots(map[int]int)` that
  string-keys each slot level and sets `Current == Max == count` (full on
  level-up), returning `nil` for an empty/nil source so `service.go`'s `!= nil`
  guard still skips the column for non-casters. `service.go` unchanged. Confined
  to `internal/levelup`.
- **Tests:** `TestCalculateLevelUp_SpellSlotsParseViaCombat` (RED→GREEN: marshals
  the wizard 2→3 level-up slots and round-trips them through
  `combat.ParseSpellSlots`, asserting `{current,max}` == `MulticastSpellSlots(3)`)
  + `TestCalculateLevelUp_NonCasterSpellSlotsNil`. `make cover-check` green
  (overall 90.68%, levelup 90.45%).
- **Simplification:** slots emitted full (current==max); prior `current` not
  preserved. Acceptable — level-ups conventionally land on a long rest, and the
  old shape was unusable, so any valid shape is a strict improvement.

### ISSUE-014 — DM Console does not track player combat actions (action_log gap)
- **Date:** 2026-06-25
- **Area:** dm console / action log (player-action service vs `/api/dm/situation`)
- **Severity:** medium — DM situational-awareness gap. Combat resolves correctly;
  only the DM Console's after-the-fact timeline was blind to player actions.
- **Status:** FIXED + DEPLOYED (`main` f1e3aeb, pushed `f29edd4..f1e3aeb`,
  redeployed ~13:45 UTC).
- **Detail:** Player spell casts and freeform actions post their results to
  `#combat-log`, but the player-action service paths never wrote to the `action_log`
  table. As a result `GET /api/dm/situation` returned a `timeline[]` with nothing for
  player combat actions — the DM Console looked empty even mid-fight.
- **Root cause:** the player-action service entry points — `Service.Cast`,
  `Service.CastAoE`, `Service.FreeformAction`, `Service.Attack`,
  `Service.OffhandAttack` — never called `CreateActionLog`. Only the DM-side /
  automated flows (enemy turns, legendary actions, the DM dashboard) write to
  `action_log`, so the timeline was populated for those but not for anything a player
  did.
- **FIX (2026-06-25, TDD, `main` f1e3aeb — committed, pushed, deployed):** a
  best-effort `recordCombatAction` helper (new file
  `internal/combat/action_log_record.go`) now writes an `action_log` row at the
  **success tail** of every player combat path — `Service.Cast`, `CastAoE`,
  `FreeformAction`, `Attack`, `attackImprovised`, `OffhandAttack`. That table feeds
  the DM Console `/api/dm/situation` timeline, so player casts/freeform/attacks now
  appear alongside the automated entries. `make cover-check` green (90%/85% gates);
  independent code review = ship-ready. Redeployed via
  `docker compose up -d --build app` ~13:45 UTC — clean boot ("database connected and
  migrated", no new migration; "discord session opened"; all discord checks passed
  for guild `1507910398886543532`; server on `:8080`; no panic/error).
- **Scope note (important):** this is a **DM-SIDE fix only**. Player-facing Discord
  output is **unchanged** — a spell cast already posted the `✨ {caster} casts {spell}`
  line to `#combat-log` and that always worked; the fix only adds the DM Console
  timeline entry, and the Console is behind DM auth (players never see it). The fix
  does **not** auto-create a `#dm-queue` item for save-spells and does **not**
  auto-roll an NPC's saving throw — **save adjudication stays a MANUAL DM roll**.
- **Follow-up (candidate, not yet a numbered issue):** auto-resolving an NPC's
  saving throw (and/or surfacing a `#dm-queue` prompt) for player save-spells is a
  worthwhile future enhancement — today it remains a manual DM roll.

### ISSUE-015 — Condition shape mismatch: dashboard vs the engine (FIXED — both halves)
- **Date:** 2026-06-25 (write half fixed 2026-06-26)
- **Area:** dashboard / combat conditions (Combat Manager render + workspace PATCH +
  Svelte tracker vs engine `parseConditions`)
- **Severity:** high — the WRITE half was a **silent mechanical no-op**: a
  button-added condition showed on the tracker but did nothing in the rules engine.
- **Status:** **FIXED** — DISPLAY half (`b108bf2`, deployed) · WRITE half (2026-06-26).
- **Two halves:**
  - **DISPLAY (the render) — FIXED.** The Combat Manager rendered a combatant's
    condition as **"[object Object]"** because the engine stores conditions as objects
    (`{condition:"paralyzed",...}`) but the Svelte UI interpolated each entry directly
    as a string.
  - **WRITE (the persisted shape) — OPEN.** The workspace PATCH endpoint
    `/api/combat/{id}/combatants/{cid}/conditions` (and the Svelte tracker that drives
    the "add condition" button) still write conditions as a **bare JSON string array**,
    e.g. `["paralyzed"]`. The combat engine reads conditions via `parseConditions`
    as an **array of objects keyed by `.condition`**, e.g.
    `[{"condition":"paralyzed",...}]`.
- **WRITE-half symptom (still live):** a condition added through the normal dashboard
  button now *renders* correctly (post-display-fix), but its mechanical effects —
  auto-crit (melee within 5 ft of a paralyzed target), advantage-to-attackers,
  auto-fail STR/DEX saves — **do NOT fire**, because `.Condition` parses empty out of
  the string-array shape.
- **Only correct WRITE path today:** the DM-Override endpoint POST
  `/api/combat/{id}/override/combatant/{cid}/conditions` is the lone HTTP path that
  writes the correct object shape (which is why the wretch's *hold person* paralysis,
  applied via that override-equivalent path in the object shape, fires correctly — and
  now also renders correctly — while a button-added condition would render but no-op).
- **FIX (DISPLAY half, 2026-06-25, `main` `b108bf2`, pushed `0dfa1ec..b108bf2`,
  deployed ~22:50 UTC):** new `conditionName()` helper
  (`dashboard/svelte/src/lib/combat.js`) Title-Cases either an object's `.condition`
  or a bare string; `CombatManager.svelte` now renders `conditionName(cond)` instead of
  interpolating the raw entry. vitest 64/64, svelte build clean, embedded assets
  regenerated. **Display-only** — the persisted WRITE shape is untouched.
- **FIX (WRITE half, 2026-06-26, TDD, `main` working tree):** aligned the PATCH
  endpoint to the engine object shape, server-side (the canonical-shape boundary), so
  the API and the Svelte tracker stay simple (they speak condition *names*).
  - **Server** (`internal/combat/workspace_handler.go`): new
    `reconcileConditionNames(existing, names)` maps the DM-supplied condition *names*
    (`updateConditionsRequest.Conditions []string`, unchanged) to the canonical
    `[]CombatCondition` object array `parseConditions` reads. It **reconciles** against
    the combatant's existing stored conditions: a name already present keeps its
    existing object (so a spell-/engine-applied condition's `duration_rounds`,
    `started_round`, `source_combatant_id`, `expires_on`, `source_spell` survive a
    re-send of the full set), a new name becomes an indefinite manual condition
    (`{condition: name}`, matching DM-toggle semantics). Names are lowercased to the
    engine's canonical keys, blanks skipped, de-duped first-seen. An unparseable
    existing value (e.g. a legacy bare-string write) is treated as empty, so the next
    PATCH self-heals the row into the object shape. `UpdateCombatantConditions` now
    calls it instead of `json.Marshal(req.Conditions)`.
  - **Frontend** (`dashboard/svelte/src/lib/combat.js` + `CombatManager.svelte`): new
    `conditionKey(c)` returns the engine's lowercase name for a string **or** object
    entry; `currentConditions()` maps stored entries through it, and `handleAddCondition`
    canonicalizes the dropdown value (`conditionKey(conditionToAdd)`), so add/remove/
    dedup compare consistently and the PATCH body is a clean lowercase name array.
  - **Tests (red→green):** Go `workspace_handler_test.go` — `WritesEngineObjectShape`
    (Title-Cased input → object array, lowercase names, `HasCondition` fires),
    `PreservesExistingObjectMetadata` (duration/source/timing survive a re-send),
    `DedupesAndDropsRemoved`, `RecoversFromLegacyStringShape`. JS `combat.test.js` —
    `conditionKey` cases. `make cover-check` green (combat 91.7%); 575 vitest green;
    Svelte bundle rebuilt (`internal/dashboard/assets/` is git-tracked).
  - **Not changed:** the engine's `parseConditions` (kept strict — object shape only)
    and the DM-Override POST path (already correct). Both writers now converge on the
    one canonical shape.

### ISSUE-016 — `/done` phantom "1 attack" warning after casting a spell with the action (FIXED)
- **Date:** 2026-06-25
- **Area:** combat / spellcasting (action economy — `Service.Cast` / `Service.CastAoE`
  vs the `/done` unused-resource check)
- **Severity:** medium — misleading UX; a phantom unused-attack warning could cause a
  player to waste time or a DM to mis-rule the turn.
- **Status:** FIXED + DEPLOYED (`main` `b108bf2`, pushed `0dfa1ec..b108bf2`, redeployed
  ~22:50 UTC).
- **Repro:** A character with **no Extra Attack** (e.g. Warlock 3) casts a spell using
  their **action** (cantrip or leveled), then runs **`/done`**.
- **Expected:** No unused-resource warning for a weapon attack — the action was spent on
  Cast-a-Spell, so there is no Attack action and no weapon attack remaining.
- **Actual:** `/done` warned **"you still have 1 attack"** and the "Remaining" resource
  summary listed a phantom attack.
- **Root cause:** casting a spell is the **Cast-a-Spell action, not the Attack action**,
  so no weapon attack remains — but `Service.Cast` / `Service.CastAoE` consumed the
  action while leaving the seeded `attacks_remaining=1` untouched. The `/done`
  unused-resource check (and the "Remaining" summary) read that stale `attacks_remaining`
  and reported an attack the caster never had.
- **FIX (2026-06-25, TDD, `main` `b108bf2`):** zero `turn.AttacksRemaining` when a spell
  consumes the **action** (cantrip or leveled). **Bonus-action casts are left untouched**
  — those keep the Attack action and its attacks (e.g. a quickened/bonus-action spell
  plus a weapon attack is legal). Red/green test
  `internal/combat/cast_attacks_remaining_test.go`; `make cover-check` passes.
- **Discovered in live play:** Vale (Warlock 3, no Extra Attack) cast **Hold Person**,
  then `/done` warned of an attack she never had.
- **Caveat (live state):** the fix only affects casts made on the **new binary**. Vale's
  *current* in-flight turn still carries the pre-fix `attacks_remaining=1`, so `/done`
  will warn **once more** for this turn — she just confirms past it; her **next** cast is
  clean.

### ISSUE-015 — Crossbow `/attack` falsely reports "No bolts remaining" with a full quiver
- **Date:** 2026-06-26
- **Area:** combat / ammunition
- **Severity:** major
- **Status:** FIXED (TDD; rebuild + redeploy required to take effect live)
- **Repro:** A character whose inventory holds crossbow bolts runs `/attack` with a
  crossbow. The bot rejects the shot with **"No bolts remaining."** despite the bolts
  being present.
- **Expected:** the shot fires and one bolt is deducted.
- **Actual:** every crossbow shot is blocked; the player can never fire.
- **Root cause:** the ammo check matched too strictly. The character builder seeds a
  light crossbow's ammo as `{item_id:"crossbow-bolt", name:"crossbow-bolt", type:"gear"}`,
  but `combat.DeductAmmunition` only matched an item whose **name was exactly "Bolts"**
  **and** whose **type was exactly "ammunition"** — so the seeded slug never matched and
  the deduction reported empty. (Same class of slug-vs-display-name drift as ISSUE-013.)
- **Second bug it would have unmasked:** the ammo write round-tripped the *entire*
  inventory through a narrow 3-field projection (`{name,quantity,type}`), so once the
  match was fixed and the write path was reached it would have **silently dropped every
  other item's `equipped`/magic/charges/`item_id` fields on each shot** (un-equipping
  the player's gear). Fixed at the same time.
- **FIX (2026-06-26, TDD, `internal/combat` + `internal/discord` + `cmd/dndnd`):**
  1. **Tolerant matcher** (`ammoMatches`): a crossbow now matches any non-weapon,
     non-armor, non-consumable item whose name **or** `item_id` contains the whole word
     `bolt` (bows → `arrow`) — so `"crossbow-bolt"`, `"Crossbow Bolts"`, `"Bolts"`,
     `"bolt"` all count, while a `"Lightning Bolt Scroll"` consumable does not. Applied to
     both `DeductAmmunition` and the post-combat `RecoverAmmunition`.
  2. **Lossless write:** the ammo path now parses/marshals through the full
     `character.InventoryItem`, preserving every other item's fields.
  3. **DM-queue fallback:** a genuinely empty quiver now raises a typed
     `combat.NoAmmunitionError`; `/attack` posts a `#dm-queue` **freeform action**
     ("is out of bolts — wants to shoot … anyway (DM may waive ammo)") and tells the
     player the DM was flagged, instead of a dead-end rejection. The attack resource is
     **not** consumed on this path, so the player can re-fire once the DM resolves it.
     DMs commonly hand-wave precise ammo counts — this routes that decision to them.
- **Tests:** `internal/combat/attack_test.go` (seeded-slug deduct, name variants,
  lookalike-consumable guard, typed-error, lossless end-to-end), `internal/discord/
  attack_handler_outofammo_test.go` (dm-queue routing + degraded paths). `go build ./...`,
  `go vet`, combat + discord + cmd wiring suites green.
- **Live caveat:** the running stack must be **rebuilt (`make build`) and restarted** for
  the fix to apply. Existing characters need no data change — the matcher now reads their
  current inventory correctly.
- **Follow-up FIXED (2026-06-26, separate commit):** builder ammo seeding corrected.
  `EquipmentToInventoryWithEquipped` now parses a `:N` quantity suffix (and comma-batched
  options), classifies SRD ammo IDs (`crossbow-bolt`, `arrow`, …) as `type:"ammunition"`,
  and gives them a proper display name (`"Crossbow Bolts"`). The Svelte builder no longer
  strips `:20` on submit (new `lib/equipment-assembly.js` `assembleEquipment` —
  bare-id list still feeds the equipped pickers; a quantity-preserving list goes to the
  backend). So a new crossbow user starts with **20 bolts**, typed ammunition, not one
  `gear` slug. Go + vitest TDD; bundle rebuilt.
- **Still open:** the same narrow-projection field-drop exists on the spell
  material-component path (`spellcasting.go`) — unrelated to ammo, left as-is.

### ISSUE-017 — Permanent SSOT item catalog (kills the slug/type/quantity drift class) — SCOPED for a fresh agent
- **Date:** 2026-06-26
- **Area:** refdata / item catalog (cross-cutting: refdata, portal builder, combat, dashboard JS)
- **Severity:** major (tech-debt; each occurrence has been a player-facing bug)
- **Status:** OPEN — SCOPED (no code yet). This entry is the spec; implement in phases.
- **Why this exists:** three+ separate live-play bugs share ONE root cause — item/equipment
  metadata is fragmented with no single source of truth, so any new item id (or a slug
  rename) silently drifts between layers:
  - **ISSUE-013** — background→skill/equipment slug drift between two hand-maintained Go maps.
  - **ISSUE-015 (ammo)** — combat matcher expected name `"Bolts"`/type `"ammunition"`; the
    builder seeded `{item_id:"crossbow-bolt", type:"gear"}`. Patched with a tolerant matcher.
  - **builder-ammo follow-up** — ammo had no name/type/quantity anywhere; patched with a local
    `knownAmmo` map + `:N` parsing. **Explicitly a stopgap.**
- **The 5 fragmented sources today (grep-verified 2026-06-26):**
  1. `internal/refdata` seeders (`seeder.go`) — **weapons + armor only**. Ammo
     (`crossbow-bolt`, `arrow`, `sling-bullet`, `blowgun-needle`) and adventuring gear
     (packs, tools, torches…) have **no refdata row at all** — they exist only as bare ids
     inside `internal/portal/starting_equipment.go` strings.
  2. `internal/portal/builder_store_adapter.go` — hand-maintained `knownWeapons`,
     `knownArmor`, `knownAmmo` Go maps + `itemDisplayName` + `itemType` + `parseEquipmentEntry`.
  3. `portal/svelte/src/lib/equip-selection.js` — a PARALLEL JS SRD-id fallback set
     (`knownWeapons`/`knownArmor` mirrors) used so pickers work before the async catalog loads.
  4. `internal/combat/attack.go` — `GetAmmunitionName` hardcodes crossbow→`"Bolts"` by
     substring; `ammoMatches` matches by name/`item_id` keyword because **no weapon→ammo-item
     link exists in data**.
  5. `internal/portal/refdata_adapter.go` `ListEquipment` (serves `/api/equipment`) — builds
     its catalog from `ListWeapons`+`ListArmor` only, so ammo/gear never appear in the API.
- **Target design — one canonical seeded item catalog:**
  - A new refdata table (e.g. `items`) or an extension that gives **every** equipment id a row:
    `id, name, category ("weapon"|"armor"|"ammunition"|"gear"|"tool"|"pack"|…), default_quantity,
    stackable bool`, plus category-specific metadata. Weapons/armor can stay in their existing
    tables if the catalog references them, but ammo + gear MUST get rows.
  - A **weapon→ammo link**: add `ammunition_id` (FK to the ammo item) to weapons with the
    `ammunition` property (light/hand/heavy-crossbow → `crossbow-bolt`; shortbow/longbow →
    `arrow`; sling → `sling-bullet`; blowgun → `blowgun-needle`). This replaces the
    `GetAmmunitionName` substring heuristic AND lets the matcher match by **item id**, not a
    name keyword (removes the `"Lightning Bolt Scroll"` false-positive risk entirely).
- **Phased implementation (TDD each phase; keep each independently shippable):**
  1. **Catalog schema + seed.** New migration + refdata seeder rows for SRD ammo + the
     adventuring gear / packs / tools referenced by `starting_equipment.go` and
     `backgrounds_gen.go`. sqlc queries (`ListItems`, `GetItem`). **Migration test hooks:** a
     new migration breaks `internal/testutil/testdb.go` table lists + the database `MigrateDown`
     test unless BOTH are updated (see the `project_new_migration_test_hooks` memory).
  2. **Weapon→ammo FK.** Add `ammunition_id` to the weapon rows; expose via the weapon model.
     Rewrite `combat.GetAmmunitionName` to read the FK (fallback to current heuristic if null),
     and switch `ammoMatches` to prefer item-id equality against the weapon's `ammunition_id`,
     keeping the keyword match only as a legacy fallback. Existing combat ammo tests must stay
     green.
  3. **Builder seeds via catalog.** `EquipmentToInventoryWithEquipped` resolves name / type /
     default_quantity from the catalog instead of `knownWeapons`/`knownArmor`/`knownAmmo` /
     `itemDisplayName`. Keep `:N` override (explicit quantity wins over default). Retire the
     three local Go maps once the catalog covers their ids; add a **contract test** that every
     id in `starting_equipment.go` + `backgrounds_gen.go` resolves to a catalog row (mirrors
     ISSUE-013's `TestBackground*_AllBuilderBackgrounds`, so future drift fails CI).
  4. **API + frontend SSOT.** `ListEquipment` serves the full catalog (ammo + gear, with
     `category` + `default_quantity`). Retire the JS SRD-fallback maps in `equip-selection.js`
     by **generating** the JS catalog/classifier from the Go source — follow the existing
     codegen precedent (`portal/svelte/src/lib/backgrounds.json` ← `backgrounds_gen.go` /
     `generate.go`). One source, both languages, no hand-sync.
  5. **Cleanup.** Delete the now-dead stopgaps (`knownAmmo`, duplicated maps); update the
     `project_item_catalog_ssot_gap` memory to RESOLVED.
- **Acceptance criteria:**
  - A brand-new portal-built crossbow user has `{item_id:"crossbow-bolt", name:"Crossbow Bolts",
    type:"ammunition", quantity:20}` sourced from the catalog (no local map).
  - `combat.GetAmmunitionName`/`ammoMatches` resolve a weapon's ammo via the FK; the substring
    heuristic is gone from the hot path.
  - `/api/equipment` lists ammo + gear; `equip-selection.js` no longer hand-maintains SRD ids.
  - A contract test fails CI if any starting-equipment / background id lacks a catalog row.
  - `make cover-check` (90%/85%), full vitest, `make sqlc-check`, and a Svelte rebuild all green.
- **Effort:** ~M–L (new migration + seeder + sqlc + rewiring 4 call sites + codegen + contract
  tests). Phases 1–3 deliver the bulk of the value (correct seeding + combat); 4–5 remove the
  remaining duplication. Each phase is independently shippable.
- **Pointers:** codegen precedent `internal/portal/backgrounds_gen.go` + `generate.go`; current
  stopgaps `internal/portal/builder_store_adapter.go` (`knownAmmo`/`itemType`/`itemDisplayName`),
  `internal/combat/attack.go` (`GetAmmunitionName`/`ammoMatches`); catalog source
  `internal/portal/refdata_adapter.go` `ListEquipment`. Memory: `project_item_catalog_ssot_gap`.
- **FIX (2026-06-26, TDD, branch `feat/item-catalog-ssot`, 5 phased commits — each independently
  shippable, `make cover-check` 90%/85% green throughout):**
  1. **Catalog schema + seed (`df9f339`).** New `items` table (migration
     `20260626120000_create_items.sql`, sqlc `GetItem`/`ListItems`/`CountItems`/`UpsertItem`) seeded
     from a new canonical `refdata.ItemCatalog()` (`internal/refdata/item_catalog.go`):
     `{id, name, category, default_quantity, stackable}`, one row per id. Weapon/armor rows derive
     their names from the existing seed slices (extracted as `weaponSeeds()`/`armorSeeds()` — names
     live once); ammunition + adventuring gear (which had **no** refdata row) are authored in
     `ammoCatalog`/`gearCatalog`. Migration test hooks updated (testdb `ReferenceTables` +
     `MigrateDown`).
  2. **Weapon→ammo FK (`33c2dae`).** Added a logical `ammunition_id` column to weapons (migration
     `..120100`), seeded on all 7 SRD ammunition weapons (crossbow→`crossbow-bolt`,
     bow→`arrow`, sling→`sling-bullet`, blowgun→`blowgun-needle`). `combat.GetAmmunitionName`
     now reads the FK → catalog name (sling/blowgun get correct names); `ammoMatches` prefers
     item-id equality, keyword scan demoted to a legacy fallback. The `"crossbow"→"Bolts"`
     substring is off the hot path.
  3. **Builder seeds via catalog (`d58e3f2`).** `EquipmentToInventoryWithEquipped` resolves
     name/type/default-quantity from `ItemCatalogByID()`; the hand-maintained `knownWeapons`/
     `knownArmor`/`knownAmmo`/`itemType`/`itemDisplayName` are **deleted**. A bare ammo id now
     seeds its catalog default bundle (lone `crossbow-bolt` → 20); explicit `:N` still wins.
  4. **API + JS SSOT (`29c4bdd`).** `/api/equipment` lists ammo + gear (with category +
     `default_quantity`). `equip-selection.js` classifies weapon/armor from `items-catalog.json`,
     **generated** from the Go catalog by `scripts/gen_items_catalog` + a `go:generate` directive
     (`make items-catalog-check` fails CI on drift). The hand-typed JS `KNOWN_WEAPON_IDS`/
     `KNOWN_ARMOR_IDS` are gone; Svelte bundle rebuilt.
  5. **Cleanup + docs.** No dead stopgaps remain (absorbed into phases 3–4). Memory
     `project_item_catalog_ssot_gap` marked RESOLVED.
- **Acceptance — all met:** a brand-new portal-built crossbow user gets
  `{item_id:"crossbow-bolt", name:"Crossbow Bolts", type:"ammunition", quantity:20}` from the
  catalog; combat resolves ammo via the FK; `/api/equipment` lists ammo + gear; the JS no longer
  hand-maintains SRD ids; **two contract tests** fail CI on re-drift —
  `TestItemCatalog_CoversAllBuilderEquipmentIDs` (every starting-equipment/background id resolves to
  a catalog row) and `TestWeaponSeeds_AmmunitionWeaponsLinkAmmoItem` (every ammo weapon links a valid
  ammunition item). `make cover-check`, full vitest (503), `make sqlc-check`,
  `make items-catalog-check`, `make backgrounds-check`, and a Svelte rebuild all green.
- **Live caveat:** unmerged on a feature branch; a running stack must be rebuilt + restarted (and the
  new migrations applied) to take effect. Existing characters need no data change — the builder reads
  the catalog at create time; combat reads current inventory via the FK + tolerant fallback.

### ISSUE-018 — Enemy-turn execution crashes on action_log NOT NULL (before_state/after_state) (FIXED)
- **Date:** 2026-06-27
- **Area:** combat / enemy turn (Turn Builder → `ExecuteEnemyTurn` → `action_log`)
- **Severity:** blocker — **every** enemy turn run through the Turn Builder crashed; combat
  could not progress past an NPC's turn. Found in live play (Round 1 of "The Cellar": the
  lead ghoul's first attack on Vale).
- **Status:** FIXED (TDD) + REDEPLOYED.
- **Repro:** Start combat with an NPC; right-click the NPC token → **Plan Turn** → **Review**
  → **Confirm & Post**. The Turn Builder shows:
  `creating action log: ERROR: null value in column "before_state" of relation "action_log"
  violates not-null constraint (SQLSTATE 23502)`.
- **Expected:** the enemy turn applies movement + attack damage, logs an `action_log` row,
  and advances initiative to the next combatant.
- **Actual (partial commit — important):** `ApplyDamage` runs **before** the log insert, so
  the **target's HP was reduced** (Vale 24→19) but the failing INSERT aborted the rest —
  `UpdateTurnActions` never ran, so the **turn did not advance** and the `enemy_turn_ready`
  dm-queue item stayed pending. State looked half-done.
- **Root cause:** `Service.ExecuteEnemyTurn` (`internal/combat/turn_builder_handler.go`,
  ~line 339) built `CreateActionLogParams` without `BeforeState`/`AfterState`, leaving them
  nil `json.RawMessage` → SQL NULL. Both columns are **NOT NULL** (`db/migrations/
  20260312120002_create_encounters.sql:91-92`). Every other action_log writer populates them
  (`dm_dashboard_handler.go` resolve/move, `dm_dashboard_undo.go`) — only the enemy path
  didn't, so only it crashed. Postgres names `before_state` first by column order; `after_state`
  was equally null, so the fix had to set both.
- **FIX (2026-06-27, TDD, `internal/combat` only — no `.sql` touched):** before the
  `CreateActionLog` call, capture the actor's pre-turn state from the local `combatant` (never
  reassigned, so it still holds the pre-movement position) via the existing
  `snapshotCombatantState` helper, re-fetch the actor with `GetCombatant` for the after-state,
  and pass both into `CreateActionLogParams`. Marshal errors ignored (matching the move path)
  so the turn never fails on snapshotting. Red/green test
  `TestExecuteEnemyTurn_PopulatesBeforeAndAfterState` (mock store mimics the NOT NULL
  constraint, asserts no error + both states populated + valid JSON + turn advances);
  confirmed it failed with the exact live error first. `go test ./internal/combat/...` green.
  Embedded assets + binary rebuilt and redeployed via `docker compose up -d --build app`.
- **Workaround applied live (before redeploy):** the damage had already landed correctly, so
  I advanced the turn with a manual **End Turn** (no re-damage) and resolved the dangling
  `enemy_turn_ready` queue item with a free-text outcome note. See
  [`sessions/session-01.md`](sessions/session-01.md).

### ISSUE-019 — Turn Builder undiscoverable (right-click only) (FIXED)
- **Date:** 2026-06-27
- **Area:** dashboard / combat UX (Combat Manager)
- **Severity:** minor — no data/mechanics impact, but cost real table time: the DM could not
  find how to run an NPC's turn.
- **Status:** FIXED + REDEPLOYED.
- **Detail:** The combat workspace's visible controls are token drag-to-move, End Turn, Undo,
  End Combat, and a read-only Action Log filter — **none** hint at running an enemy turn. The
  Turn Builder is reached **only** by right-clicking the enemy token → "Plan Turn" (the
  right-click menu also hosts Damage / Heal / Conditions / Remove). With no affordance, the DM
  had no way to know it existed.
- **FIX (2026-06-27, `dashboard/svelte/src/CombatManager.svelte`):** added a prominent gold
  **"⚔ Run Enemy Turn — <name>"** button at the top of the right panel (above the Turn Queue),
  rendered only when the current-turn combatant is an NPC (`activeTurnCombatant?.is_npc`,
  derived from `activeEncounter.active_turn_combatant_id`). Extracted a shared
  `openTurnBuilder(comb)` helper so the new button, the right-click "Plan Turn" item, and the
  no-map list's "Plan Turn" all use one code path (no duplicate open logic). Right-click menu
  left intact. vitest `CombatManager.test.js` 7/7 (added 4 cases); full suite 647 green; Svelte
  bundle rebuilt (`internal/dashboard/assets/` is git-tracked) + redeployed.

### ISSUE-020 — Character sheets show stale base HP mid-combat (two HP stores, no overlay) (FIXED)
- **Date:** 2026-06-27
- **Area:** character sheet / HP source (portal sheet, Discord `/character`, dashboard Character Overview)
- **Severity:** medium — no data loss, but confusing/wrong: a player checking their own sheet
  mid-fight saw full HP and no sign of being bloodied.
- **Status:** FIXED + REDEPLOYED + VERIFIED LIVE.
- **Repro:** during the live "Cellar" fight Vale took a 5-damage ghoul bite → `combatants.hp_current`
  = 19/24 (correct). Open Vale's character sheet (portal, `/character`, or the dashboard Party
  Overview) → it showed **24/24**.
- **Root cause — two HP stores that don't reconcile:**
  - `characters.hp_current` — the static base sheet, set at creation / level-up / out-of-combat DM edit.
  - `combatants.hp_current` — the live per-encounter snapshot. Combat **seeds** a combatant from the
    character at `StartCombat` (`combat/domain.go` `CombatantFromCharacter`, `HPCurrent: char.HpCurrent`)
    and **never writes back** (no write-back at end-of-turn, end-of-combat, or on damage — confirmed:
    `EndCombat` doesn't sync HP; only the out-of-combat editor's `UpdateCharacterVitals` touches the row).
  - So during a fight the `characters` row is frozen at its pre-combat value, and **every sheet that
    reads it shows stale HP**. Only Discord `/status` was already correct (it overlays the combatant).
  - The crash in [ISSUE-018] did **not** lose the damage: `ApplyDamage` and the (then-failing)
    `CreateActionLog` are not in one transaction, so the HP write committed independently.
- **FIX (2026-06-27, TDD, read-side overlay on 3 surfaces — HpCurrent/HpMax/TempHP only):**
  - **Portal sheet** — `internal/portal/character_sheet_store.go` `hydrateFromCombatant`. It already
    overlaid the combatant's conditions/exhaustion/concentration ("the combatant is the live source of
    truth during combat") but **forgot HP**; added the three HP lines. Tests:
    `..._InCombatOverlaysHP`, `..._OutOfCombatKeepsSheetHP`.
  - **Discord `/character`** — `internal/discord/character_handler.go`: new optional `SetCombatProvider`
    wiring (the same `StatusEncounterProvider` + `StatusCombatantLookup` `/status` uses, wired in
    `cmd/dndnd/discord_handlers.go`), `overlayCombatHP` resolves the owner's active encounter and matches
    the combatant by `CharacterID == ch.ID` before building the embed. Tests:
    `..._InCombat_OverlaysLiveCombatantHP`, `..._NotInCombat_KeepsCharacterRowHP`.
  - **Dashboard Character Overview API** — `internal/characteroverview/store_db.go`
    `ListApprovedPartyCharacters` now calls `overlayLiveCombatHP` per sheet (reuses the already-wired
    `GetActiveCombatantByCharacterID` the 409 check uses). Tests: `..._OverlaysLiveCombatHP`,
    `..._NoCombatKeepsRowHP`.
  - All overlays are **best-effort / read-only**: no active combatant, `uuid.Nil`, or lookup error →
    fall back to the character row. The DM out-of-combat status editor's **409-in-combat write path is
    untouched** (its `UpdateStatus`/409 tests still green).
  - `#character-cards` Discord embed **excluded** — it's a static posted message; live HP there would
    require re-posting the card on every damage event (future work if wanted).
  - `make cover-check` green (characteroverview 94.58%, discord 85.93%, portal 89.76%); redeployed;
    **verified live** — DM Party Overview now reads **Vale 19/24**.

### ISSUE-021 — Enemy-turn executor resolves the attack only (no auto-move, no auto-advance)
- **Date:** 2026-06-27
- **Area:** combat / enemy turn (Turn Builder → `ExecuteEnemyTurn`)
- **Severity:** medium — not a crash or lost damage; the turn is *correct but incomplete*, so
  the board and initiative silently drift unless the DM finishes by hand.
- **Status:** OPEN.
- **Context:** first clean live runs of the Turn Builder after the ISSUE-018 `before_state`
  crash fix — the 2nd ghoul (init 9, **C8**, ~35 ft from Forge) and the lead ghoul (init 19,
  **E2**, already adjacent to Forge).
- **Repro:** "⚔ Run Enemy Turn" → the planner generates **only** an ATTACK step (e.g. Bite vs
  Forge, reach 5 ft) with **no MOVE step**, even when the NPC is out of reach. Confirm & Post →
  the attack resolves (damage applied on a hit, `enemy_turn` action_log row written, posted to
  #combat-log) — **but** (a) the token does not move (the 2nd ghoul "bit" from 35 ft, left at
  C8), and (b) the turn stays `status='active'` / the encounter does not advance.
- **Expected:** the executor should path the NPC into reach when out of range (it legally can —
  30 ft move → adjacent) before the attack, and advance the turn on completion.
- **Actual:** attack-only resolution; DM must **drag the token into reach** and click **End
  Turn** manually. Did both live (2nd ghoul: drag C8→D2 + End Turn; lead ghoul: already
  adjacent, miss, End Turn).
- **Minor:** the "Turn Complete" summary prints the actor name blank — `**'s Turn**` (missing
  display name in the post template).
- **Distinct from ISSUE-018:** that was the action_log NOT-NULL crash (fixed + deployed); this
  is executor *scope* — it now runs cleanly but only does the attack.
- **Fix idea:** in `ExecuteEnemyTurn` / the plan builder (`turn_builder_handler.go`), emit a
  move step toward the chosen target when out of reach (reuse the player `/move` pathing) and
  call the turn-advance path after a successful resolve.

### ISSUE-025 — action_log silently dropped every player action (Console timeline blind)
- **Date:** 2026-06-28
- **Area:** combat / `action_log` (player-action observability) vs the DM Console timeline
- **Severity:** major — DM situational-awareness gap. Combat resolved correctly; the
  `/api/dm/situation` `timeline[]` was blind to **every** player cast/attack/freeform for
  ~3 days, which forced manual session-logging to compensate (the very thing the DB should make
  unnecessary). Surfaced while reconciling the live-play state docs.
- **Status:** FIXED (TDD, `main`).
- **Root cause:** `action_log.before_state` and `after_state` are **NOT NULL**.
  `recordCombatAction` (`internal/combat/action_log_record.go`, added for ISSUE-014) builds a
  `CreateActionLogInput` **without** those fields, so `CreateActionLog` passed `nil` straight
  through → every player-action insert violated the constraint. The write is intentionally
  **best-effort** (error swallowed so a logging failure never aborts a resolved cast/attack), so
  the violation vanished without a trace. Only `ExecuteEnemyTurn` rows persisted — it populates
  before/after state since the **ISSUE-018** fix. **This is the same bug class as ISSUE-018, on
  the player path**, which means ISSUE-014 ("FIXED + DEPLOYED + verified") was effectively a
  no-op in production.
- **Why it hid:** the combat unit suite uses a mock store (`captureActionLog`) that happily
  records the nil columns the real Postgres rejects. Every `*_RecordsActionLog` test was green
  while prod silently dropped the row — a mock-vs-DB divergence. Empirically confirmed against
  the live DB: the active encounter's `action_log` held **only** `enemy_turn` rows, none of
  Vale's crossbow / Misty Step / Chill Touch.
- **Fix (2026-06-28, TDD, `main`):** coerce a nil/empty `before_state`/`after_state` to the JSON
  empty object `{}` at the single choke point — new `rawMessageOrEmptyObject`
  (`internal/combat/service.go`) applied in `CreateActionLog`, so **no** service-method caller
  can silently fail the NOT-NULL constraint again (the requested regression guard). Direct
  `store.CreateActionLog` callers (condition/override/undo/legendary/turn-builder) already
  supply real state and are unaffected. `{}` is safe for player-action rows — they are timeline
  observability, not undo targets (undo reads `before_state` only for DM-override action types).
  Red/green `TestRecordCombatAction_PopulatesNonNullState` asserts the recorded params carry
  non-null valid JSON. `make cover-check` green; rebuilt + redeployed.
- **Follow-up (candidate, not done):** the mock store could enforce the NOT-NULL columns so a
  future best-effort writer that forgets state fails the unit suite instead of prod. Logged, not
  implemented — the choke-point coercion already prevents the recurrence.

### ISSUE-026 — Spell riders / ongoing effects aren't first-class timed effects
- **Date:** 2026-06-28
- **Area:** combat / effect model (cast resolution vs the condition/effect store)
- **Severity:** medium (enhancement / removes manual DM tracking)
- **Status:** OPEN — scoped, no code.
- **Problem:** several effects that *should* be tracked by the engine are not modeled as a
  combatant effect with a duration, so the DM tracks them by hand:
  - **Chill Touch** — target can't regain HP until the start of the caster's next turn (and an
    undead target attacks the caster at disadvantage). Lives in ad-hoc `Cast` logic, no effect row.
  - **Save-each-turn / ongoing effects** — e.g. ongoing poison or a spell that repeats a save at
    end of turn; the timing isn't a first-class field.
  - **Timed riders generally** — there's no `{source_spell, duration_rounds, started_round,
    expires_on}` effect the engine ticks down and clears.
- **Why it matters here:** the DM Console (`/api/dm/situation`) only reads `conditions`
  (now with metadata after the Tier-1 work), so any rider not stored as a condition/effect is
  invisible — it's the one residual hand-track left in `game-state.md`'s "Next action".
- **Target:** a first-class timed-effect model the combat engine advances per turn and the
  situation payload surfaces; migrate the ad-hoc riders onto it. TDD.

### ISSUE-027 — NPC quick-statblock in the DM Console payload
- **Date:** 2026-06-28
- **Area:** dm console / situation payload (`internal/situation` + adapter)
- **Severity:** medium (enhancement)
- **Status:** IMPLEMENTED 2026-06-28 (red/green TDD, `make cover-check` green).
- **Problem:** to run an enemy turn a DM needs the creature's moveset — attacks (name, damage
  dice, reach), recharge abilities, legendary/lair actions — but the payload returned combatant
  *state* only, so the DM opened the stat block separately (and the Turn Builder was the only
  place reach/attacks surfaced).
- **Fix:** added a per-NPC `creature_summary` to `CombatantView`:
  - `internal/combat/creature_summary.go` — `BuildCreatureTurnSummary(creature)` reuses the Turn
    Builder's own parsers (`ParseCreatureAttacksWithSource`, `parseCreatureAbilitiesFromCreature`,
    `isRechargeAbility`/`parseRechargeMin`, `HasLegendaryActions`/`ParseLegendaryInfo`,
    `HasLairActions`) → `CreatureTurnSummary{Attacks, RechargeAbilities, HasLegendary,
    LegendaryBudget, HasLair}`; best-effort (malformed/open5e prose → no structured attacks).
    `IsEmpty()` lets the adapter omit movesetless creatures.
  - `internal/situation` — neutral view types `CreatureSummary` / `AttackSummary` /
    `RechargeSummary` (JSON-tagged, `omitempty`); `CreatureSummary *CreatureSummary` field on
    both `CombatantRow` (input) and `CombatantView` (output); `buildState` copies it through. The
    package stays dependency-free (no refdata/combat import).
  - `cmd/dndnd/situation_adapter.go` (coverage-excluded) — `creatureSummary()` fetches the
    creature for NPC combatants with a valid `CreatureRefID`, calls the combat builder, maps to
    the situation view; memoized per ref id so a pack of identical creatures costs one
    `GetCreature`. PCs / no-ref / GetCreature-miss / empty-moveset all yield nil → field omitted.
  - Tests: `internal/combat/creature_summary_test.go` (attacks+recharge, legendary+lair, empty,
    malformed-tolerated, open5e-prose) all 100% covered; `internal/situation` plumbing test
    `TestBuild_StateSurfacesCreatureSummary` (NPC populated, PC nil).
- **Deferred by design:** the **ISSUE-021** executor half (auto-move into reach + auto-advance the
  turn) is intentionally **left OPEN** per DM direction — NPC turns are run manually (Run Enemy
  Turn → Confirm & Post → manual End Turn); no auto-advance wanted. `creature_summary` gives the
  DM the moveset to drive that manual turn from the Console.

### ISSUE-028 — Player in-character roleplay is invisible to the Console (platform gap)
- **Date:** 2026-06-28
- **Area:** dm console / in-character feed (Discord ingestion)
- **Severity:** major (largest situational-awareness gap)
- **Status:** OPEN — scoped, no code. Large (platform integration).
- **Problem:** #in-character roleplay exists only as Discord messages — it is never written to
  any DB table and never appears in `/api/dm/situation` `timeline[]` (which merges only
  `action_log` + DM `narration_posts`). A DM cannot see what a player said in character without
  reading Discord directly; this is exactly why the DM-rules mandate Chrome-reading
  (see [`dm-rules.md`](dm-rules.md)).
- **Target:** ingest #in-character messages (Discord webhook or poll) into a roleplay timeline
  the Console surfaces (a `roleplay_timeline[]` or a new source in `timeline[]`). Needs a new
  table + Discord handler + auth gating. Until then, Chrome-reading #in-character stays required.

### ISSUE-029 — DM Console has no out-of-combat / exploration state
- **Date:** 2026-06-28
- **Area:** dm console / situation payload (out-of-combat)
- **Severity:** medium (enhancement)
- **Status:** OPEN — scoped, no code.
- **Problem:** `buildState` returns an empty `StateView` when there's no active encounter, so
  out of combat the Console shows nothing — exploration progress (`encounters.explored_cells`),
  party scene/location, and prep readiness are invisible, and the DM falls back to game-state.md
  notes (a hand-tracked surface).
- **Target:** surface exploration/scene state in the payload (e.g. an exploration-mode
  `StateView` with explored cells + party position) so the Console is the live view between
  fights too, not only mid-combat.

### ISSUE-030 — AdvanceTurn silently drops an un-executed NPC turn (round skips a live combatant)
- **Date:** 2026-06-28
- **Area:** combat / turn advancement (`internal/combat/initiative.go` `AdvanceTurn`)
- **Severity:** major
- **Status:** FIXED (TDD) + redeployed.
- **Repro (live):** R4 order G2(init-1, NPC) → Vale(2, PC) → Forge(3, PC) → G1(4, NPC). Forge's
  R4 greataxe crit killed G2. Engine advanced Forge→G1 correctly (G1 turn row `status=active`,
  `enemy_turn_ready` posted). Then a second advance fired (an End-Turn before the enemy executor
  ran) → G1's R4 turn marked `completed` with `started_at=NULL`, `action_used=false`,
  `attacks_remaining=1`, **no `action_log` attack**; round rolled to R5/Vale. G1's bite was lost.
- **Expected:** G1 takes its R4 turn (run the enemy turn, resolve its attack) before the round
  advances; ending an unrun NPC turn should be refused, not silently completed.
- **Actual:** `AdvanceTurn` (lines ~399-427) unconditionally `CompleteTurn`s `enc.CurrentTurnID`,
  then — because G1 now appears in `hadTurn` — finds no R4 candidates, advances the round, and
  returns the first R5 combatant (Vale). The NPC's whole turn evaporated.
- **Root cause:** missing guard. No check that an NPC's enemy turn was executed before completing
  it. The `started_at IS NULL` signal the first investigation suggested is **wrong** — NPC turns
  always have `started_at=NULL` even when executed (R3 ghoul attacked with NULL `started_at`). The
  reliable signal is `ExecuteEnemyTurn` setting `turn.ActionUsed=true` (`turn_builder_handler.go:378`,
  unconditional, even for a no-op plan).
- **NOT death-related:** simulated `AdvanceTurn` with G2 alive — the R5 candidate rebuild
  (`initiative.go` ~469-474, filters on `IsAlive` only) returns G2 first; G1's R4 turn is dropped
  either way. The bug drops whichever combatant is current-but-unexecuted when a premature
  End-Turn fires; G1 just happened to be last in order, so it read as "the round skipped a ghoul."
- **Fix:** new sentinel `ErrEnemyTurnNotExecuted`; `AdvanceTurn` returns it (without completing or
  advancing) when the current turn's combatant `IsNpc && !ActionUsed`. `DMDashboardHandler.AdvanceTurn`
  maps it to **409** (`errors.Is`), and the dashboard `apiFetch`/`TurnQueue` already surface the
  body text — so the DM sees "enemy turn must be executed before it can be ended" instead of a
  silent skip. PCs unaffected (guard is NPC-only). Tests: `TestService_AdvanceTurn_RefusesUnexecutedEnemyTurn`,
  `_AllowsExecutedEnemyTurn`, `TestAdvanceTurn_UnexecutedEnemyTurnReturns409`. `make cover-check` green.
- **Live game:** left as-is per DM call — no rewind; G1 acts normally on its R5 turn (the dropped
  bite is not restored).
- **Relationship:** inverse of [ISSUE-021] (executor under-does the turn: no auto-move/advance);
  this was the engine *over*-advancing past an unrun NPC. The dangling `enemy_turn_ready` cleanup
  is the same ISSUE-021 artifact.

### ISSUE-038 — End Combat doesn't carry combat HP/conditions back to the sheets (fresh-agent task)
- **Date:** 2026-06-28
- **Area:** combat / end-combat state reconciliation (two HP stores)
- **Severity:** medium — correctness/safety: a downed PC silently reads **full HP** the moment combat
  ends, erasing the fight's consequences until a DM notices and fixes it by hand.
- **Status:** FIXED (2026-06-29). `EndCombat` carries each PC's final HP/temp-HP + post-clear
  conditions + exhaustion back to the `characters` row via `Service.carryOutPCStatus`
  (`internal/combat/service.go`), run inside the existing condition-clear loop so it reuses the
  just-computed cleared conditions and the original combatant row (CharacterID/HP/exhaustion intact).
  It writes through the **shared `UpdateCharacterVitals`** sqlc query — the same write path the
  out-of-combat status editor uses — plus `rest.CharacterDataWithExhaustion` for the exhaustion merge,
  so there is a single writer for the `characters` row. PCs only (NPCs skipped); a 0-HP PC carries out
  `unconscious` (combat-only `prone` is dropped by `ClearCombatConditions`); HP is clamped to `[0, sheet max]`;
  a write failure bubbles up (consistent with the other post-status-flip DB writes — concentration break,
  timer pause, ammo recovery — rather than the best-effort Discord/loot fan-outs) so a hiccup is visible
  rather than silently re-introducing the stale-full-HP bug. Tests:
  `TestEndCombat_CarriesOutPCStatusToCharacterRow`, `TestEndCombat_CarryOut_GetCharacterError`,
  `TestEndCombat_CarryOut_UpdateVitalsError`, `TestEndCombat_CarryOut_ClampsHPAndDefaultsConditions`.
- **Repro:** Run a combat where a PC takes damage / drops to 0. End the fight (Combat Manager →
  **End Combat → Confirm End**). Open dashboard **Party** (or the portal sheet / Discord `/character`).
- **Expected:** out of combat each PC reflects how they **left** the fight — bloodied PCs at their
  combat HP, a downed-but-stabilized PC at 0 HP / unconscious.
- **Actual:** every PC shows their **pre-combat stored HP** (full, if they started full) with no
  combat conditions. Observed live at the end of "The Cellar": Forge ended **0/32, unconscious + prone,
  stabilized** and Vale **7/24**, but the Party page showed **Forge 32/32** and **Vale 24/24**, no
  conditions. The DM reconciled both by hand via **Party → Edit status**.
- **Root cause:** the same two-store split as **ISSUE-020** — combat damage lives only on
  `combatants.hp_current` (seeded from the character at `StartCombat`, `combat/domain.go`
  `CombatantFromCharacter`), and the engine **never writes back** to `characters.hp_current`. ISSUE-020
  fixed the *in-combat* display with a **read-side overlay** (portal `hydrateFromCombatant`, Discord
  `overlayCombatHP`, dashboard `overlayLiveCombatHP`), but the overlay is keyed on an **active
  encounter** — so the instant End Combat flips the encounter inactive, the overlay disappears and the
  base row (untouched all fight) shows through. No carry-out step bridges the gap.
- **Fix idea (TDD):** on the End-Combat path (the service behind the dashboard "End Combat → Confirm
  End" that flips the encounter status — **locate it**; likely an `EndCombat`/encounter-status method in
  `internal/combat` + its dashboard handler), **carry out** each **PC** combatant's final state to the
  `characters` row before/at deactivation: `hp_current`, `temp_hp`, and combat-applied
  conditions/exhaustion. **Reuse the out-of-combat status-editor write path** ([[project_dm_out_of_combat_status_editor]])
  — it already knows how to persist HP+conditions to `characters` — rather than a second writer.
  Handle the **0-HP cases** explicitly: a PC at 0 HP should carry out **unconscious**, and a
  stabilized/death-save outcome must be preserved (don't resurrect to full, don't re-enter "dying" out
  of combat). **NPCs are not carried out** (they're encounter-scoped). 
- **Design caution:** ISSUE-020 **deliberately** chose read-side overlay over write-back to keep one
  source of truth mid-fight — so confine this write to the **End Combat boundary only** (not
  end-of-turn / on-damage), PCs only, and don't clobber a value a DM has already edited. Decide whether
  pre-existing temp HP / concentration also carry out. Red/green test the carry-out + the 0-HP→
  unconscious + stabilized-preservation cases; `make cover-check`; rebuild + redeploy.
- **Workaround (current):** after every End Combat, the DM reconciles each PC's HP/conditions by hand
  via **Party → Edit status** (audit reason logged). Easy to forget — that's the bug.

### ISSUE-039 — No DM editor for limited-use resources / rage (FIXED)
- **Date:** 2026-06-29
- **Area:** dashboard / combat resources (`characters.feature_uses` JSONB)
- **Severity:** medium — a character whose rage (or ki / channel divinity / sorcery points / …) `current`
  was set wrong had **no** in-app correction mid-fight; the only lever (party long rest) resets *every*
  resource to max and 409s during active combat.
- **Status:** FIXED (2026-06-29).
  - **Backend (override):** `POST /api/combat/{encounterID}/override/character/{characterID}/feature-uses`
    `{feature, current, reason}` → `DMDashboardHandler.OverrideCharacterFeatureUses`
    (`internal/combat/dm_dashboard_undo.go`), reusing `Service.SetFeaturePool` (preserves the row's
    **Max + Recharge**) and the slots-override audit path (`logOverride` → `dm_override` row +
    `#combat-log` ⚠️ DM Correction). Body validated pre-lock (400); unknown feature / `current>max` →
    400 via `errInvalidFeatureOverride`; **unlimited pools** (`Max<0`, e.g. a level-20 rage) skip the
    cap. Routes in `RegisterRoutes` + `main.go`; auth + wiring route-lists updated so the new DM
    mutation is verified behind `dmAuthMw`.
  - **Backend (read/prefill):** `GET /api/character-overview/{id}/feature-uses` → `Handler.GetFeatureUses`
    (`internal/characteroverview/handler.go`); `SlotsContext` now also carries the raw `feature_uses`
    (works in **and** out of combat — it's a read). Empty/NULL renders `{}`, not `null`.
  - **Frontend:** `FeatureUsesEditor.svelte` mounted in the Combat workspace **Manual Override** panel
    (Feature Uses fieldset, **PCs only** — gated on `character_id`, NPCs have none); api helpers
    `getCharacterFeatureUses` / `overrideCharacterFeatureUses`; CombatManager wiring; bundle rebuilt.
- **Found live:** the AI DM manually set Forge's character values during setup, leaving rage
  `{current:1, max:3}` — should have been **2** after one rage this fight. No dashboard path to bump it.
- **Rest command (audited en route):** **correct** — long rest sets every `recharge ∈ {short,long,dawn,daily}`
  feature `current=max` so rage (recharge `"long"`) restores to max (`internal/rest/rest.go`); short rest
  matches only `recharge=="short"`, so rage is untouched (RAW). L3 rage-max formula = 3. The bug was the
  **missing editor**, not the rest logic.
- **Tests:** 15 combat handler cases (`TestOverrideCharacterFeatureUses_*`, incl. the unlimited-pool
  branch) + 7 character-overview GET cases (`TestGetFeatureUses_*`) + a `featureUsesValue` round-trip; 724
  svelte tests pass; `make cover-check` green.
- **Used live:** set Forge rage **1→2** through the new editor (Combat → select FO token → Manual Override
  → Feature Uses → Edit Feature Uses); DB + the `dm_override` before/after audit row both confirm.
- **Known gaps:** (1) out-of-combat editing → **fixed in ISSUE-040** (2026-06-30). (2) rage **Max** is seeded only
  at character creation (`InitFeatureUses`, `internal/portal/init_feature_uses.go`), **not** re-derived on
  level-up — a barbarian leveling into a new rage tier (L3/L6/…) could carry a stale Max; didn't affect
  Forge (his Max was already 3). Unfiled — flag if it bites.

### ISSUE-040 — No out-of-combat editor for feature uses (FIXED)
- **Date:** 2026-06-30
- **Area:** dashboard / character overview (`characters.feature_uses`)
- **Severity:** minor — workaround existed (edit during combat via ISSUE-039, or run a long rest); only
  bit **between** fights.
- **Status:** FIXED (TDD, 2026-06-30).
- **Repro:** out of combat, open dashboard **Party → Character Overview** for a Barbarian (or any
  limited-use class — Monk ki, Cleric/Paladin channel divinity, Sorcerer points, …). HP/conditions +
  spell/pact-slot editors are present; there is **no** feature-uses editor.
- **Expected:** a DM can view + set a character's rage/ki/channel-divinity/… remaining uses **between
  fights** (award a short-rest-recovered resource, or correct a bad value) without starting combat or
  forcing a full long rest.
- **Actual:** the only feature-uses editor (ISSUE-039) is the **in-combat** override in the Combat
  workspace, gated on an active turn. Out of combat the sole lever is a party long rest, which resets
  *all* resources to max.
- **Fix idea (TDD):** add `POST /api/character-overview/{characterID}/feature-uses` mirroring
  `Handler.UpdateSlots` (`internal/characteroverview/handler.go`) — DM-authorized, **409 during active
  combat** (defer to the in-combat override, same split as the slots/HP editors), validate
  feature-present + `current ∈ [0, max]` (unlimited when `max<0`), persist via the
  `characters.feature_uses` write path. **Reuse** the existing read `GET .../feature-uses` and the
  already-built `FeatureUsesEditor.svelte` — mount it on `CharacterOverview.svelte`, wiring `onSave` to
  the new overview endpoint (the parent picks in- vs out-of-combat, exactly like `SlotEditor`). Red/green
  + `make cover-check` + rebuild.
- **Fix (TDD, 2026-06-30):** new `Service.ApplyFeatureUses` + `Store.UpdateCharacterFeatureUses`
  (`internal/characteroverview/feature_uses.go`, `store_db.go`) reusing the already-generated
  `refdata.UpdateCharacterFeatureUses` query — no schema/sqlc change. `Handler.UpdateFeatureUses`
  (`handler.go`) parses id → decodes body → loads `GetSlotsContext` (404/500) → `authorizeDM` (403) →
  **409 if `InActiveCombat`** → `ApplyFeatureUses` (per-feature `current ∈ [0,max]`, `max<0` =
  unlimited, only features already on the row are editable; `ErrInvalidInput` → 400). Auto-mounts via
  `RegisterRoutes` (already inside `dmAuthMw`). Frontend: `saveCharacterFeatureUses` in `lib/api.js`;
  `CharacterOverview.svelte` mounts `FeatureUsesEditor` with `openFeatureEditor` (fetch-on-open via the
  read GET) / `saveFeatures` / `closeFeatureEditor`, mirroring the slot editor. Body is the editor's
  `{changes:[{feature,current}], reason}` shape (batch, applied atomically). Tests: service
  (`feature_uses_test.go`), handler + store (`handler_test.go`, `store_db_test.go`), api
  (`api.test.js`); `make cover-check` green, 726 vitest green.
- **Notes:** the editor component + the read endpoint already existed from ISSUE-039, so this was mostly a
  second handler + a parent mount. The level-up rage-**Max** re-derive gap noted under ISSUE-039 is still
  unfiled and unaffected by this change.

### ISSUE-041 — Rage lapses silently (no #combat-log / no DM timeline) (FIXED)
- **Date:** 2026-06-30
- **Area:** combat / rage end-of-turn auto-expiry (`internal/combat/rage.go`)
- **Severity:** medium — not a state bug (the drop is correct RAW), but an **observability** hole that
  misleads play: a player keeps acting as if raging (resistance + advantage) when the engine no longer
  treats them as raging, and the DM has no timeline signal it happened.
- **Status:** FIXED (TDD, 2026-06-30).
- **Found live (Cold Vault, Round 1):** Forge moved up and **raged** (bonus action) but ended his turn
  **without attacking** (the keeper was still out of reach) and took no damage. By RAW rage ends if, by the
  end of your turn, you haven't attacked a hostile or taken damage — so the engine's `maybeEndRageOnTurnEnd`
  correctly cleared `is_raging` (the schema even tracks `rage_attacked_this_round` / `rage_took_damage_this_round`
  for exactly this). **But the only rage line ever posted was "🔥 Forge enters a Rage!" — nothing when it
  dropped.** Next turn the keeper's longsword hit for **7 slashing, applied in full** (Forge 32→25); had rage
  held it would have been halved to **3**. The 4-HP swing — and the fact that Forge was no longer raging at
  all — was invisible to both player and DM.
- **Root cause:** `maybeEndRageOnTurnEnd` (rage.go) cleared + persisted rage state but emitted nothing. The two
  observability surfaces that exist for other combat events — Discord **#combat-log** (`CombatLogNotifier`) and
  the **action_log** DM-Console timeline — were never touched on expiry. (Rage *entry* posts to #combat-log via
  the Discord bonus handler; rage *exit* had no equivalent.)
- **Fix (TDD, 2026-06-30):** added `Service.notifyRageExpired`, called from `maybeEndRageOnTurnEnd` right after
  the rage clear, **mirroring `notifyDroppedToZero`** (the drop-to-0 dual-surface precedent): it posts
  `FormatRageEnd(name, "no attack or damage this round")` to #combat-log via `postCombatLog`, then writes a
  **`rage_expired`** action_log row (new `actionTypeRageExpired` const) parented to the encounter's current
  turn — which, at the `AdvanceTurn` end-hook (`initiative.go:442`, before `CompleteTurn`), is still the
  rager's own turn, so the timeline row sits under the turn that ended it. Pure observability + best-effort:
  nil notifier / `GetEncounter` failure / missing turn are all swallowed so turn-advance never blocks. Reused
  the existing `FormatRageEnd` formatter (no new message function). Red/green
  `TestMaybeEndRageOnTurnEnd_Lapsed_LogsRageExpired` (asserts the #combat-log post + the `rage_expired` row's
  turn/encounter/actor) + `..._AttackedThisRound_NoLog` (rage holds → no clear, no logs); full `internal/combat`
  suite + `make cover-check` green; redeployed.
- **Not addressed (separate, still open/unfiled):** rage **Max** isn't re-derived on level-up (noted under
  ISSUE-039); and rage end on **unconsciousness** (`maybeEndRageOnUnconscious`) is already implied by the
  drop-to-0 line so it was left as-is.

### ISSUE-060 — Builder never surfaces Warlock pact boon / eldritch invocations (systemic `choose_*` gap)
- **Date:** 2026-07-03
- **Area:** builder / class-feature choices (portal Svelte + `internal/portal` derivation)
- **Severity:** medium (playable via workaround; a real feature gap for every choice-bearing class)
- **Status:** OPEN
- **Repro:** Build/edit a Warlock in the builder → there is no control to pick a Pact Boon (Chain/Blade/Tome/Talisman) or the 2 Eldritch Invocations. On the finished sheet the features remain `mechanical_effect: choose_pact_boon` and `choose_2_eldritch_invocations` (verified live on Vale, char `b6ca7f49-c173-4290-8c80-6fb785fbe733`).
- **Expected:** at levels that grant them, the builder prompts the player to choose their pact boon + invocations (with prereqs enforced), and the sheet stores the concrete picks + any spells/cantrips they grant.
- **Actual:** the placeholders are stored raw; no pick is ever made. Same for **all** other class choices except Expertise.
- **Root cause / scope (read-only investigation 07-03):**
  - Builder is a fixed 7-step wizard — `STEPS = ['Basics','Class','Ability Scores','Skills','Equipment','Spells','Review']` (`portal/svelte/src/CharacterBuilder.svelte:52`). **No class-features-choice step.**
  - **Only precedent** for an interactive class choice is **Expertise** (Rogue L1 / Bard L3): `selectedExpertise` state (`CharacterBuilder.svelte:75`), picker embedded in the Skills step (`:1199-1227`), `reconcileExpertise()` (`portal/svelte/src/lib/skill-selection.js:272-286`), submission field `Expertise []string` (`internal/portal/builder_service.go:51`).
  - Everything else is an unresolved placeholder: Fighter **Fighting Style** (`internal/refdata/seed_classes.go` `choose_fighting_style`), Sorcerer Metamagic, and the Warlock pair (`seed_classes.go:380-383`). Systemic, not warlock-specific.
  - **No catalog data exists** for pact boons or invocations — no seed file, DB table, refdata query, or Svelte data. `character.Feature.Choices map[string][]string` (`internal/character/types.go`) exists to hold resolved picks but is **never populated** — `CollectFeatures` (`internal/portal/derive_stats.go:210-243`) merges seed features raw.
  - Persist path: draft `DRAFT_FIELDS` (`portal/svelte/src/lib/builder-draft.js:28-49`) → `CharacterSubmission` (`internal/portal/builder_service.go:39-63`) → `BuildCharacter`/`CollectFeatures`. A picker would add `selectedPactBoon` / `selectedInvocations` at each layer + resolve them into `Feature.Choices`.
- **Effort: LARGE.** Files: `CharacterBuilder.svelte` (state + picker UI), `builder-draft.js` (draft fields), new `portal/svelte/src/lib/` module (invocation prereqs/level gates), `builder_service.go` (submission fields + resolution), `derive_stats.go` (choice → features), NEW `internal/refdata/seed_pact_boons.go` + `seed_eldritch_invocations.go` (and possibly migrations/tables). Prereq validation (e.g. Agonizing Blast needs Eldritch Blast; some invocations gate on level or Pact of the Blade/Tome) is the hard part. A first cut could scope to Warlock-only + a curated invocation subset, reusing the Expertise UI pattern (→ MEDIUM).
- **Workaround (this session, chosen path):** set Vale's boon + 2 invocations directly on her sheet once the player picks, adding any granted spells/cantrips to her spell list so they're castable via slash commands. Player still chooses; DM never picks for them. Note Vale has **no Eldritch Blast**, so Agonizing Blast is moot for her; her theme (storyteller/deceiver, Fiend) points at Mask of Many Faces / Beguiling Influence / Misty Visions / Devil's Sight and Pact of the Tome or Chain.

<!-- Append a section per issue:

### ISSUE-001 — <short title>
- **Date:** YYYY-MM-DD
- **Area:** setup / auth / dashboard / register / combat / map / narration / …
- **Severity:** blocker / major / minor / cosmetic
- **Status:** OPEN
- **Repro:** exact steps (commands, clicks, IDs).
- **Expected:** what should happen.
- **Actual:** what happened (paste bot/log output verbatim).
- **Workaround:** if any.
- **Notes / fix idea:** code pointer if known.
-->
</content>
