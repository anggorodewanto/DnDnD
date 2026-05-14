# Batch 14: WS, crash recovery, simultaneous encounters (Phases 103–105c)

## Summary

All six phases land with substantive implementations:

- **103 (WS)**: hub + per-encounter snapshot publisher + JS client with exponential backoff are in place. Snapshot-always protocol matches spec (no deltas, no sequence numbers). WS upgrade is auth-gated.
- **104 (Crash recovery)**: startup sequence in `cmd/dndnd/main.go` matches spec lines 116–121 exactly (connect DB → migrate → seed → stale-turn scan → open gateway → register commands → start ticker). Narration poster, DM messenger, dashboard publisher, story poster, and DM-correction poster are all wired post-recovery.
- **104b (Fan-out)**: combat, rest, inventory, levelup, magicitem all expose `SetPublisher` and are wired in `main.go`. Shared `combat.NewStoreAdapter` consolidates the per-test adapter that the original phase note flagged.
- **104c (Levelup mount)**: DB-backed `CharacterStoreAdapter` + `ClassStoreAdapter` + `NotifierAdapterWithStory`; handler mounted on router; publisher fires before route registration.
- **105 (Simultaneous encounters)**: per-encounter advisory locks (keyed on `turn_id`), per-tab WS subscriptions in CombatManager.svelte, label "⚔️ <display_name> — Round N" via `combat.EncounterLabel`. Integration test seeds two active encounters and exercises constraint that a character can only be in one.
- **105b (Handler wiring)**: 32 of 32 router `Set*Handler` setters attached via `attachPhase105Handlers`. The two unmatched setters (`SetErrorRecorder`, `SetReactionPromptStore`) are wired separately in `main.go` lines 1159/1164. `wireEnemyTurnNotifier` injects `DiscordEnemyTurnNotifier` (with encounter-lookup SET) so the label populates in production.
- **105c (Display-name editor)**: pure-JS helpers (`displayNameEditor.js`), Svelte `DisplayNameEditor.svelte` mounted from both `CombatManager.svelte` (line 913) and `EncounterBuilder.svelte` (line 440); `updateEncounterDisplayName(id, name)` in `api.js`; PATCH `/api/combat/{encounterID}/display-name` mounted in `combat.Handler.RegisterRoutes`.

## Per-phase findings

### Phase 103 — WebSocket State Sync
- **Status**: MATCH
- **Key files**:
  - `internal/dashboard/ws.go` — Hub + per-client `EncounterID` subscription, ServeWebSocket auth gate on `auth.DiscordUserIDFromContext`.
  - `internal/dashboard/snapshot.go` — `SnapshotBuilder` + `Publisher.PublishEncounterSnapshot`.
  - `dashboard/svelte/src/lib/wsClient.js` — 1s/2s/4s/8s/16s/30s capped backoff (matches spec lines 105).
  - `dashboard/svelte/src/lib/optimisticMerge.js` — preserves dirty fields, surfaces `_pendingFromSnapshot.<field>` for the indicator the spec describes ("HP updated to 3 by player action").
- **Findings**:
  - Snapshot includes encounter + combatants + current turn + ServerTime — full state per push, no deltas. Matches spec line 104.
  - WS auth: `ServeWebSocket` rejects with 401 if no `DiscordUserIDFromContext`. Route mounted behind `dmAuthMw` in main.go line 781 (DM-only).
  - `BroadcastEncounter("", msg)` is a no-op safeguard so a bug never broadcasts to everyone; nice touch.
  - Reader loop discards client messages (push-only protocol confirmed).
  - Slow-client drop policy: drop + close `Send` channel. Reasonable.
  - Minor: `websocket.AcceptOptions{InsecureSkipVerify: true}` is permissive; comment says "dev"; might want a prod CORS check eventually.

### Phase 104 — Bot Crash Recovery
- **Status**: MATCH
- **Key files**:
  - `cmd/dndnd/main.go` lines 458–996 — startup ordering.
  - `internal/combat/timer.go` — `PollOnce`, `TurnTimer.Start/Stop`.
  - `internal/combat/timer_stale_integration_test.go` — verifies "stale turns processed in deadline order".
  - `internal/discord/asi_handler.go` — `HydratePending` rehydrates ASI prompts (the only durable in-flight state besides combat turns).
- **Findings**:
  - Spec sequence (lines 116–121) matched exactly: connect Postgres → migrate → seed → `timer.PollOnce(ctx)` (sync, before gateway) → `rawDG.Open` → `RegisterAllGuilds` → `timer.Start`.
  - Per spec lines 122–123, in-flight commands rely on Postgres rolling back uncommitted transactions and releasing advisory locks — no custom "pending interactions" table required, and the implementation respects that.
  - Turn timers derived from DB fields, not in-memory: confirmed in `timer.go` (every poll re-reads from store).
  - Panic recovery: `CommandRouter.SetErrorRecorder` plus `errorlog` integration so handler panics become friendly ephemeral + error_log row.
  - Discord session built BEFORE recovery but Opened AFTER — keeps the bot "dark" during stale scan so no live interactions race the recovery. Excellent.

### Phase 104b — Publisher Fan-out & Store Adapter Cleanup
- **Status**: MATCH
- **Key files**:
  - `internal/combat/service.go` — 6 publish call-sites inside the service (HP change, conditions, status, position override, summon, etc.).
  - `internal/rest/rest.go`, `internal/inventory/api_handler.go`, `internal/levelup/service.go`, `internal/magicitem/service.go` — each exposes `SetPublisher(publisher, encLookup)`.
  - `internal/combat/store_adapter.go` — exported `combat.NewStoreAdapter(*refdata.Queries)`, used by both main.go and the integration test.
- **Findings**:
  - Each service uses an `EncounterLookup` to skip publishing when the mutated character isn't currently in combat — avoids spurious WS noise for out-of-combat HP changes (rest, inventory, levelup).
  - `encounterLookupAdapter` in main.go handles `sql.ErrNoRows` cleanly (treats as "not in combat" rather than error).
  - Publisher hooks tolerant of nil publisher and swallow publish errors (logged via `s.publish` in combat.Service).
  - DM-correction poster, hostiles-defeated notifier, loot-pool creator, turn-start notifier, and initiative tracker also wired through the same combat service — broader fan-out than the spec strictly requires.

### Phase 104c — levelup.Handler Mounted with DB Store Adapter
- **Status**: MATCH
- **Key files**:
  - `internal/levelup/store_adapter.go` — `NewCharacterStoreAdapter`, `NewClassStoreAdapter`.
  - `internal/levelup/notifier_adapter.go` — `NewNotifierAdapterWithStory` (DM + #the-story story poster).
  - `cmd/dndnd/main.go` lines 931–951 — full wiring path: SetPublisher BEFORE RegisterRoutes.
  - `cmd/dndnd/levelup_story_adapter_test.go` — coverage.
- **Findings**:
  - `levelup.Handler` constructed with hub for any WS broadcast (route mounted via `RegisterRoutes`).
  - `levelup.Notifier` carries both DM messenger and StoryPoster; both are nil-safe when Discord is offline.
  - One nit: levelup.Handler keeps an internal `tmpl` rendering an embedded HTML page; F-16 note in the template warns "A Svelte equivalent of this widget now lives at /dashboard/app/#levelup" — acceptable, but the legacy page is still mounted at `/dashboard/levelup` without auth middleware. Worth confirming this is intentional.

### Phase 105 — Simultaneous Encounters
- **Status**: MATCH
- **Key files**:
  - `internal/combat/turnlock.go` — `AcquireTurnLock` keyed on `pg_advisory_xact_lock(UUIDToInt64(turn_id))`.
  - `internal/combat/turnlock_test.go` — proves distinct turn_ids produce distinct lock keys.
  - `internal/combat/phase105_integration_test.go` — two simultaneous active encounters; "character can only be in one active encounter" constraint enforced.
  - `internal/combat/domain.go` — `EncounterDisplayName`, `EncounterLabel`, `FormatEncounterLabel`.
  - `dashboard/svelte/src/lib/encounterTabsWs.js` — per-tab wsClient manager.
  - `dashboard/svelte/src/CombatManager.svelte` lines 36/109/279 — wires `createEncounterTabsWs` into the tabbed Combat Workspace exactly as the phase note prescribed.
- **Findings**:
  - Per-turn lock is keyed on `turn_id` not `encounter_id`, but the integration test proves any two active encounters have distinct `current_turn_id`s, so locks remain disjoint — matches spec line 84.
  - UUID→int64 truncation is a documented 2^-64 collision risk (F-17). Acceptable, called out in comments.
  - Command routing per-user: `discordUserEncounterResolver` walks `guild_id → campaign → (campaign, discord_user_id) → character → active encounter`. Used by every Phase 105 handler.
  - Label formatting: `combat.EncounterLabel(enc)` returns `"⚔️ <display_or_internal> — Round N"`, falls back to internal name when display_name is NULL (matches spec line 1694).

### Phase 105b — Discord Handler Wiring in main.go
- **Status**: MATCH
- **Key files**:
  - `cmd/dndnd/discord_handlers.go` — `buildDiscordHandlers` constructs every Phase 105 handler; `attachPhase105Handlers` registers them on the CommandRouter.
  - `cmd/dndnd/discord_adapters.go` — `discordUserEncounterResolver` is the concrete `ActiveEncounterForUser` impl.
  - `cmd/dndnd/discord_handlers_wiring_test.go` — broad wiring assertions.
- **Findings**:
  - 32/32 `r.Set*Handler` setters in `attachPhase105Handlers` match the 32 router setters in `discord/router.go` (the other two router setters — `SetErrorRecorder`, `SetReactionPromptStore` — are not handler setters and are wired separately).
  - `wireEnemyTurnNotifier(combatHandler, discordHandlerSet.enemyTurnNotifier)` injects the Discord-backed notifier with `SetEncounterLookup` already called in `buildDiscordHandlers` line 314 — guarantees the "⚔️ <name> — Round N" label populates in production. Phase 105b done condition satisfied.
  - Status, recap, reaction, action, use, whisper handlers all share the same `userEncounterResolver` so per-user routing is consistent.
  - Notifier wires: rest, check, reaction, use, whisper, action all `SetNotifier(dmQueueNotifier)` so the DM queue receives the right event types.
  - `set.loot` is guarded for nil (loot service may be absent in test deploys); same pattern for attack/bonus/shove/etc. Defensive.

### Phase 105c — DM Display-Name Editor
- **Status**: MATCH
- **Key files**:
  - `internal/combat/handler.go` line 35 — `r.Patch("/{encounterID}/display-name", h.UpdateEncounterDisplayName)`.
  - `dashboard/svelte/src/lib/api.js` line 611 — `updateEncounterDisplayName(encounterId, name)`.
  - `dashboard/svelte/src/lib/displayNameEditor.js` — `normalizeDisplayName` + `commitDisplayName` (pure, testable).
  - `dashboard/svelte/src/DisplayNameEditor.svelte` — shared component.
  - `dashboard/svelte/src/EncounterBuilder.svelte` line 440 + `CombatManager.svelte` line 913 — both mount the editor.
- **Findings**:
  - Empty input clears to NULL via `nullString` path in encounter service — fallback to internal name preserved (spec line 1694).
  - `commitDisplayName` returns structured `{status: 'unchanged' | 'saved' | 'error', value, error?}` so the UI can show a transient indicator.
  - `combat.service.UpdateEncounterDisplayName` covered by tests at `internal/combat/service_test.go:2397+`.

## Cross-cutting concerns

- **WS auth model**: `/dashboard/ws` sits behind `dmAuthMw`, so only DMs can subscribe to encounter snapshots. Spec doesn't explicitly say "DM-only WS", but the dashboard itself is described as "DM dashboard" throughout, so this is consistent.
- **Optimistic UI dirty-field handling** is split correctly: `optimisticMerge.mergeSnapshot` is the pure helper, `encounterTabsWs.markDirty/clearDirty/subscribe` is the per-tab state manager. Both fully unit-tested.
- **Publisher symmetry**: every mutation surface that affects an active encounter's combatant state now fires `PublishEncounterSnapshot` — except (intentionally per Phase 104b) shops, loot post-encounter, character overview, narration, template, messageplayer. Per the phase doc this is correct.
- **Crash-recovery test coverage**: integration test seeds two stale turns and confirms deadline-ordered nudges, which is the only piece spec-mandated; in-flight transaction rollback is delegated to Postgres (no test needed — Postgres guarantees it).
- **Truncated advisory-lock key (UUID → first 8 bytes)** is documented in code and ADR (F-17). Acceptable.

## Critical items

None blocking. Soft items worth a follow-up but not required by the spec:

1. `/dashboard/levelup` legacy HTML page (Phase 104c) is mounted without authMiddleware. The Svelte SPA at `/dashboard/app/#levelup` is the canonical surface and is behind auth, so this is more a clean-up than a security gap — but a curious player who knew the URL could probe POST `/api/levelup` (which IS unauthenticated based on `RegisterRoutes`). Worth gating behind `dmAuthMw` to be consistent with every other DM-only handler.
2. `nhooyr/websocket` `InsecureSkipVerify: true` is fine for local dev but should be re-evaluated when fronted by a non-`fly.io` proxy in case origin spoofing becomes a concern.
3. The 32 router setters / 33 attach calls mismatch is an artifact of `r.SetLootHandler(set.loot)` being inside a nil-guard so the attach happens conditionally — there's no missing wiring. Worth a sanity test like the existing `discord_handlers_wiring_test.go` set that asserts every handler-typed field is non-nil when its deps are all present.
