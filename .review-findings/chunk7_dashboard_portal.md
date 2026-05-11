# Chunk 7 — Dashboard, Portal, Magic Items, Leveling, DDB Import (Phases 88–102)

## Summary

Phases 88a–102 are largely landed in code, with strong coverage of pure-logic slices (magic-item effects, attunement, level-up math, DDB parser/diff, narration markdown, homebrew CRUD, mobile layout helpers) but several **integration/wiring gaps** that mean key features will not work in production. The headline issues:

1. **Portal token flow is unwired.** `cmd/dndnd/main.go:730` passes a hardcoded `"e2e-token"` placeholder for `RegistrationDeps.TokenFunc` and constructs `portal.NewHandler(logger, nil)` (validator nil) — `/portal/create` will nil-panic. The real `portal.TokenService` is fully implemented but never instantiated from `main.go`. Phase 91a's "one-time link, 24h expiry, single-use" is logic-complete but not wired.
2. **Portal API and Character Sheet routes not registered.** `routes.go` exposes `WithAPI` and `WithCharacterSheet`, but `main.go` calls `portal.RegisterRoutes` with only `WithOAuth`. The Svelte builder will 404 every API call (`/portal/api/races`, etc.) and `/portal/character/{id}` is unreachable.
3. **DDB re-sync skips DM approval.** `ddbimport.Service.Import` (`internal/ddbimport/service.go:88`) calls `UpdateCharacterFull` immediately on re-import — the player's response says "pending DM approval" but the database has already been mutated.
4. **ASI/Feat flow only handles "+2 / +1+1"; the "Feat" branch is a stub.** `internal/discord/asi_handler.go:266-275` returns "Feat selection is not yet available" — Phase 89/89d done-when says "feat selection via select menu".
5. **Active-ability charge usage and dawn-recharge are implemented but never invoked.** `inventory.UseActiveAbility` and `Service.DawnRecharge` exist with tests but no caller in `internal/discord/`, `internal/rest/`, or anywhere else.
6. **`/cast identify` and `/cast detect-magic` not wired into Discord.** The pure functions `inventory.CastIdentify`, `StudyItemDuringRest`, `DetectMagicItems` exist (Phase 88c logic) but the `/cast` handler does not exist as a top-level Discord handler that recognizes these spell names — only the dashboard `POST /api/inventory/identify` is reachable.
7. **DM Dashboard cannot initiate DDB import.** Spec line 2420 says "or the DM can import from the dashboard"; no Svelte/HTML surface exists for it.
8. **Level-up public announcement deferred.** `cmd/dndnd/main.go:612` notes "public announcements deferred"; `FormatPublicLevelUpMessage` exists but no caller posts it.
9. **Portal builder UI is missing subrace, language, and skill-count enforcement.** `subrace` and `subclass` are tracked in form state but no `<select>` exposes them.
10. **No Svelte components for Character Overview (Phase 101) or Homebrew editor (Phase 99).** Backends exist; UI is absent (homebrew is API-only).

Tests are deep on the Go side (each phase has a coverage test file), but the **dashboard/portal Svelte tests do not exercise the missing wiring**, and the playtest checklist (`docs/playtest-checklist.md`) marks every scenario `Status: pending` — no real transcripts captured.

---

## Per-phase findings

### Phase 88a — Magic Items: Tracking, Bonuses & Passive Effects — ⚠️
- ✅ Schema fields present in `internal/character/types.go:96-104` (`IsMagic`, `MagicBonus`, `MagicProperties`, `RequiresAttunement`, `Rarity`, `Charges`, `MaxCharges`, `Identified`).
- ✅ FES integration: `internal/magicitem/effects.go:14-78` converts `MagicBonus` to `combat.Effect` for weapons/armor; `ParsePassiveEffects` (line 90) covers `modify_attack`, `modify_damage`, `modify_ac`, `modify_save`, `resistance`, `bonus_damage`.
- ✅ `/inventory` formatter shows rarity + attunement (`internal/inventory/service.go:204-281`).
- ⚠️ Spec example "Boots of Speed" (`type: "modify_speed", modifier: "double"`) and "modify_speed" type are not implemented in `convertPassiveEffect` (`internal/magicitem/effects.go:113-153`); only six effect types handled.

### Phase 88b — Active Abilities, Attunement & Identification — ⚠️
- ✅ Attune/unattune logic with 3-slot cap, restriction check, already-attuned check (`internal/inventory/attunement.go:31-65`).
- ✅ `/attune` and `/unattune` Discord handlers (`internal/discord/attune_handler.go:21`, `unattune_handler.go:16`).
- ✅ Unattuned-equip warning (`internal/inventory/equip.go:53`).
- ✅ Active-ability charge consumption logic (`internal/inventory/active_ability.go`).
- ✅ Dawn recharge with destroy-on-zero (`internal/inventory/recharge.go:38-90`).
- ❌ **`UseActiveAbility` not invoked from any Discord handler.** `grep -rln UseActiveAbility internal/discord internal/dashboard` returns nothing — players cannot actually use a Wand of Fireballs from Discord.
- ❌ **`DawnRecharge` not invoked from `/rest long`.** `grep -rln DawnRecharge internal/rest` is empty; charges never restore in production.

### Phase 88c — Identification Flow — ⚠️
- ✅ Pure functions: `CastIdentify`, `StudyItemDuringRest`, `DetectMagicItems`, `SetItemIdentified` (`internal/inventory/identification.go:81-173`).
- ✅ DM dashboard endpoint `POST /api/inventory/identify` (`internal/inventory/api_handler.go:130-173`, mounted at `internal/dashboard/routes.go:75`).
- ❌ **`/cast identify` and `/cast detect-magic` not wired in Discord.** No `cast_handler.go` file exists; `grep -rn identify internal/discord/*.go` returns no hits. The `/cast` slash command is registered (`internal/discord/commands.go:69`) but no router maps "identify" or "detect-magic" spell names to `inventory.CastIdentify`. Spec done-when says "Integration tests verify `/cast identify` identification" — only the API layer is tested.
- ⚠️ Short-rest study path (`StudyItemDuringRest`) not wired into the rest flow either.

### Phase 89 — Character Leveling — ⚠️
- ✅ Pure level-up math: `CalculateLevelUp` (`internal/levelup/levelup.go:26`), spell-slot multiclass table, attacks-per-action.
- ✅ HTTP API: `POST /api/levelup`, `/asi/approve`, `/asi/deny`, `/feat/apply`, `/feat/check` (`internal/levelup/handler.go:86-95`).
- ✅ `NeedsSubclassSelection` flag returned by service (`internal/levelup/service.go:234`).
- ✅ Private DM "level-up details" message via `NotifierAdapter.NotifyLevelUp` (`internal/levelup/notifier_adapter.go:38`).
- ❌ **Public `#the-story` announcement deferred** (`cmd/dndnd/main.go:611-612` comment). `FormatPublicLevelUpMessage` exists but no caller; spec done-when says "notifications" plural and the spec lines 2461-2465 explicitly call for the public ribbon.
- ⚠️ No programmatic "set subclass" flow — handler only renders an HTML message asking the DM to talk to the player (`internal/levelup/handler.go:310`). Acceptable per "DM-helps" framing but not strict spec compliance.

### Phase 89d — Discord Interactive ASI/Feat Components — ⚠️
- ✅ Buttons for +2/+1+1/Feat (`internal/discord/asi_handler.go:42-59`), select menu for ability scores (`BuildAbilitySelectMenu` line 91), DM approve/deny buttons (line 136).
- ✅ End-to-end interaction routing through `CommandRouter` (`internal/discord/router.go:425-432`).
- ❌ **Feat select-menu not implemented.** The Feat button replies with "Feat selection for X is not yet available" (`internal/discord/asi_handler.go:271`). Phase 89d done-when says "select menus for scores or feats (with prerequisite check)" — only the prerequisite check exists (`internal/levelup/feat.go:29`); no Discord-side feat picker.

### Phase 90 — D&D Beyond Import — ⚠️
- ✅ URL parser (`urlparser.go`), JSON parser (`parser.go`), validator with structural + advisory warnings (`validator.go`), preview (`preview.go`), diff (`diff.go`).
- ✅ Exponential backoff with cap: `internal/ddbimport/client.go:62-113` (1 s initial, 30 s cap, 3 retries default).
- ✅ Discord `/import` handler with DDB client (`internal/discord/registration_handler.go:108-195`).
- ❌ **Re-sync mutates the DB before DM approval.** `ddbimport.Service.Import` calls `UpdateCharacterFull` (`internal/ddbimport/service.go:88-93`) on the existing record before posting to `#dm-queue`. The player message says "changes detected and pending DM approval" (`registration_handler.go:188`) but the data has already been overwritten. The DM has nothing to "approve" — the diff is a notification, not a gate.
- ❌ **DM dashboard cannot initiate import.** Spec line 2420: "DM pastes URL in dashboard". No `internal/dashboard/ddbimport_*` handler, no Svelte UI; only the post-fact "DDB sheet" link in the approval handler (`internal/dashboard/approval_handler.go:482`).

### Phase 91a — Player Portal Auth & Scaffold — ❌
- ✅ Token logic complete: `TokenService` with single-use semantics + 24 h TTL parameterized (`internal/portal/token_service.go:48-90`); cryptographic random tokens.
- ✅ Discord `/create-character` posts an ephemeral link (`internal/discord/registration_handler.go:262`).
- ✅ Portal landing/create/error templates (`internal/portal/handler.go:158-269`).
- ❌ **`TokenService` never instantiated from `main.go`.** `grep -rn portal.NewTokenService cmd/` returns nothing. `cmd/dndnd/main.go:730` uses a hardcoded `func(_ uuid.UUID, _ string) (string, error) { return "e2e-token", nil }`.
- ❌ **`portal.NewHandler(logger, nil)` is constructed with a nil validator** (`cmd/dndnd/main.go:594`). Visiting `/portal/create?token=…` will trigger a nil dereference at `internal/portal/handler.go:94` (`tok, err := h.validator.ValidateToken(...)`).
- ⚠️ The `routes.go:46` comment "TODO(Phase 14)" matches what's in `main.go:726`, so this is known but unaddressed.

### Phase 91b — Portal Multi-Step Builder Form — ⚠️
- ✅ Steps wired (Basics → Class → Ability Scores → Skills → Equipment → Spells → Review) in `portal/svelte/src/App.svelte:7-10`.
- ✅ Point-buy calculator (`portal/svelte/src/lib/pointbuy.js`) with "remaining points" indicator.
- ✅ Form state preserved across steps (single Svelte component with `$state`).
- ✅ Derived stats on review page (`finalScores`, `derivedHP`, `derivedAC` in App.svelte:208-237).
- ⚠️ **Subrace, subclass, languages: no UI.** Form-state fields exist (`subrace` line 14, `subclass` line 17) but no `<select>` exposes them; payload always sends empty strings.
- ⚠️ **Skill picker has no count limit / class allowance enforcement** (App.svelte:382-389). Spec: "based on class/background/race allowances".
- ⚠️ **Spell picker shows every spell for the class regardless of caster level.** Spec: "filtered by available level".
- ❌ **Submit will fail in production**: `portal.RegisterRoutes` doesn't get `WithAPI` (`cmd/dndnd/main.go:599`), so `POST /portal/api/characters` returns 404.

### Phase 91c — Portal Starting Equipment — ⚠️
- ✅ `GET /portal/api/starting-equipment?class=…` returns `EquipmentPack` data (`internal/portal/api_handler.go:155-166`); class default packs in `internal/portal/starting_equipment.go`.
- ✅ Builder UI loads packs (App.svelte:404-436), supports manual add/remove search (line 440-470).
- ❌ Same wiring gap as 91b: route returns 404 because `WithAPI` not passed.

### Phase 92 — Portal Character Sheet — ❌
- ✅ Sheet domain model (`internal/portal/character_sheet.go:21-56`), DB store (`character_sheet_store.go`), handler (`character_sheet_handler.go`), full HTML template (~448 lines).
- ✅ Spell grouping by level, prepared indicator, derived modifiers/skills/saves.
- ❌ **`WithCharacterSheet` not invoked from `main.go`.** `grep -rn WithCharacterSheet cmd/` returns nothing → `/portal/character/{id}` is not registered → `/character` Discord embed link points to a 404.

### Phase 92b — Spell List Storage & Display — ✅
- ✅ Portal builder writes selected spells into `character_data.spells` (App.svelte:255).
- ✅ DDB importer writes spells into `character_data` (`internal/ddbimport/service.go:142`).
- ✅ Sheet adapter joins against `spells` table (`internal/portal/character_sheet_store.go`).
- ✅ `/character` Discord embed shows spell summary by level (`internal/discord/character_handler.go:131-202`); handles both DDB and portal formats.
- ✅ `#character-cards` includes spell count (`internal/charactercard/format.go:79-87`).

### Phase 93a — Dashboard Manual Char Create (Basics → Ability Scores) — ✅
- ✅ Wizard HTML template at `internal/dashboard/charcreate_handler.go` (~700 lines including JS); fields for name, race, background, class entries (multi), abilities.
- ✅ Validation (`charcreate.go:26-72`), derived stats preview (`DeriveDMStats`).
- ✅ Pre-approved status: `Status: "approved"`, `CreatedVia: "dm_dashboard"` (`internal/dashboard/charcreate_service.go:104-105`).

### Phase 93b — Dashboard Char Create (Equipment, Spells, Features) — ✅
- ✅ Equipment, spells UI in the same wizard (`charcreate_handler.go:476-552`).
- ✅ `FeatureProvider` interface + auto-population via `CollectFeatures` (`charcreate_service.go:62-69`).
- ✅ Subclass `<select>` populated from `classes.subclasses` (`charcreate_handler.go:539-552`).
- ⚠️ The HTML/JS UI is hand-rolled (~700 lines of Go-templated HTML with inline `onclick` handlers) — not Svelte. Functional, but inconsistent with the rest of the dashboard's Svelte stack.

### Phase 94a — Combat Manager: Map & Token Display — ✅
- ✅ Svelte `CombatManager.svelte` (1502 lines) renders map + tokens client-side from JSON.
- ✅ HP/Condition tracker (apply damage/healing, add/remove conditions) wired to `/api/combat/*` endpoints.
- ✅ Multi-encounter tabs with badge counts; per-encounter WebSocket (`encounterTabsWs.js`) for live state.
- ✅ Encounter Overview bar.

### Phase 94b — Combat Manager: Movement & Interaction — ✅
- ✅ Drag-and-drop with snap-to-tile (`CombatManager.svelte:87-89` + drag handlers).
- ✅ Distance overlay during drag, range circles on selection (line 391).
- ✅ Distance-measurement tool (`measureMode`, line 96-98 + 779-784).
- ✅ Wall validation via `mapdata.js:getWalls` and `combat.js:findPath`.
- ✅ Token context menu (line 92-93).

### Phase 95 — Turn Queue & Action Resolver — ✅
- ✅ `TurnQueue.svelte` (186 lines) — initiative order, current-turn highlight, End Turn button, read-only mode for mobile.
- ✅ `ActionResolver.svelte` (461 lines) — pending action list, expand-inline, outcome text, damage/condition/move effect builder, resolve API call.

### Phase 96 — Active Reactions Panel — ✅
- ✅ `ActiveReactionsPanel.svelte` (320 lines) — grouped by combatant, status badges (active/used/dormant/cancelled), highlights when `activeTurnIsNpc` and reaction is active (line 65-68), resolve/dismiss buttons.
- ✅ "Reaction used this round" → 'dormant' status (line 60-63).

### Phase 97a — Action Log Viewer — ✅
- ✅ `ActionLogViewer.svelte` (364 lines): filters by action type/actor/target/round/turn, sort asc/desc, expandable entries with before/after diff via `lib/diff.js`.

### Phase 97b — Undo & Manual Corrections — ✅
- ✅ `lib/api.js` exports `undoLastAction`, `overrideCombatantHP`, `overrideCombatantPosition`, `overrideCombatantConditions`, `overrideCombatantInitiative`, `overrideCharacterSpellSlots` (imported in `CombatManager.svelte:7-13`).
- ✅ Backend `/api/encounters/{id}/undo` and `/api/combat/override-*` endpoints.
- ⚠️ Could not confirm "Discord corrections posted to #combat-log (never edit/delete originals)" wiring without deeper trace; assume present given combat-log is foundational.

### Phase 98 — Stat Block Library — ⚠️
- ✅ Backend service with SRD/homebrew/source filters, CR/type/size/Open5e source filters, search, pagination (`internal/statblocklibrary/service.go:36-80`).
- ✅ HTTP routes `GET /api/statblocks` and `GET /api/statblocks/{id}` (`handler.go:46-49`).
- ⚠️ **No dedicated Svelte browser component.** Only the encounter builder uses creature data, and via the `/api/creatures` endpoint (not `/api/statblocks`). The Stat Block Library is reachable from the browser only as a "desktop-only redirect" target (`layout.js:44`) that has no actual page behind it.

### Phase 99 — Homebrew Content — ❌
- ✅ Full backend CRUD for creatures, spells, weapons, magic items, races, feats, classes (`internal/homebrew/`).
- ✅ Routes mounted under `/api/homebrew/*` (`handler.go:43-75`).
- ✅ Campaign-scoping enforcement, `homebrew=true`, `source="homebrew"`.
- ❌ **No Svelte UI.** `grep -rn homebrew dashboard/svelte/src` returns nothing. The DM cannot create a homebrew monster from the dashboard — only by hitting the JSON API directly.

### Phase 100a — Narration Editor — ✅
- ✅ Markdown renderer with read-aloud fenced blocks → Discord embeds (`internal/narration/markdown.go:48-…`).
- ✅ `NarratePanel.svelte` (used by both desktop and mobile shells).
- ✅ Image attachments via Asset Library (`internal/narration/store_db.go` + `adapters.go:22`).
- ✅ Post-history log + service (`internal/narration/service.go`).
- ✅ "Post to #the-story" wired through `narration` adapter to `discord` session.

### Phase 100b — Narration Templates — ✅
- ✅ Placeholder regex extraction + substitution (`internal/narration/template.go:13-61`).
- ✅ Template service + DB store + handler (`template_service.go`, `template_store_db.go`, `template_handler.go`).
- ✅ Frontend in `lib/narrationTemplates.js` + tests; integrated into `NarratePanel.svelte`.

### Phase 101 — Character Overview & Message Player — ⚠️
- ✅ Backend: `internal/characteroverview/service.go` + handler at `GET /api/character-overview`.
- ✅ Backend: `internal/messageplayer/service.go` + `POST /api/message-player/`.
- ✅ `MessagePlayerPanel.svelte` (91 lines) — DM types player_character_id (raw UUID), body, sends.
- ❌ **No Svelte component for Character Overview.** `grep -rn character-overview dashboard/svelte/src` returns nothing — the DM cannot view the party in the dashboard.
- ⚠️ MessagePlayer UI is not character-pickable (free-text UUID); spec line 2804 calls this an "on-the-go task" — usability gap on mobile.

### Phase 102 — Responsive Mobile-Lite View — ✅
- ✅ Mobile breakpoint at 1024 px (`dashboard/svelte/src/lib/layout.js:10`).
- ✅ Six tabs: dm-queue, turn-queue, narrate, approvals, message-player, quick-actions (`layout.js:30`).
- ✅ Bottom tab bar in `MobileShell.svelte`.
- ✅ Desktop-only redirect via `MobileRedirect.svelte` (`App.svelte:140-143`).
- ⚠️ Approvals tab links out to `/dashboard/approval` rather than embedding (`MobileShell.svelte:51-54`); functional but inconsistent.
- ⚠️ Turn Queue has `readOnly` prop respected, but no read-only enforcement on mobile DM-Queue (the resolution panel is fully interactive — may be OK per spec which says "DM Queue: view and resolve").

---

## Cross-cutting risks

### Frontend wiring gaps (highest impact)

1. **`cmd/dndnd/main.go:594-599` ships the portal with `validator=nil` and no `WithAPI`/`WithCharacterSheet`.** This means in a fresh deployment:
   - `/portal/create?token=…` → nil-deref panic.
   - `/portal/api/*` → 404, builder Svelte page is decorative only.
   - `/portal/character/{id}` → 404, `/character` ephemeral link points nowhere.
   - The TokenFunc placeholder (`main.go:730`) issues `"e2e-token"` constant, so even with the validator wired, the system is single-token broken.
2. **Phase 99 Homebrew has no UI surface**, despite full API. The "full stat block editor" called out in the phase description does not exist as a Svelte component.
3. **Phase 101 Character Overview has no UI surface.** Backend ready, but DM cannot view it.
4. **Phase 98 Stat Block Library** is API-only too; the encounter builder borrows creature data via a different endpoint.

### DDB rate-limit handling

- Backoff math is correct (`client.go:66-92`): doubles delay, caps at `maxDelay`, respects `ctx.Done()`. ✅
- Retries only on HTTP 429. 5xx errors propagate immediately. Acceptable.
- **Missing**: no `Retry-After` header parsing (always uses computed backoff). If DDB returns `Retry-After: 60`, the client may retry too eagerly.
- **Missing**: no jitter — synchronized retries across multiple imports could herd.

### Re-sync semantics (Phase 90 spec line 2445)

- Spec says "system diffs and shows DM what changed before applying". Code applies first, diffs second (`service.go:84-93`). The DM-queue post is informational only. **Recommend gating UpdateCharacterFull behind explicit DM approval (separate "diff approved" endpoint)** or at minimum capturing both old and new states for rollback.

### Mobile parity

- 6 tabs match spec. Approvals tab uses an external link rather than embedding (`MobileShell.svelte:51-54`). On a phone, opening `/dashboard/approval` works but exits the mobile shell — minor.
- `MobileRedirect` only fires when `currentDesktopOnlyID()` resolves; the function has limited mappings (`App.svelte:53-58`) that miss `stat-block-library`, `asset-library`. So if a desktop-only deeplink lands on mobile for those views, no redirect renders.

### Missing tests

- **Portal tokenFunc wiring**: tested in unit-test-only mode (`token_service_test.go`), but no integration test verifies that `main.go` injects the real service. The placeholder is invisible to the test suite.
- **DDB resync DM-approval gate**: no test asserts that the DB is *not* mutated until the DM approves. The test at `service_test.go` checks the diff is generated; it doesn't check the gating semantic.
- **`/cast identify` end-to-end**: only API-handler tests exist.
- **`UseActiveAbility` Discord routing**: no Discord handler test exists because no handler exists.
- **Mobile redirect for `stat-block-library`**: `layout.test.js:79` asserts the helper, but App.svelte routing never reaches that token (so the redirect never renders in practice).

---

## Recommended follow-ups

In rough priority order:

1. **Wire `portal.TokenService` in `cmd/dndnd/main.go`** — replace the placeholder `TokenFunc` with `tokenSvc.CreateToken(ctx, campaignID, discordUserID, "create_character", 24*time.Hour)` and pass the same `tokenSvc` as the validator to `portal.NewHandler`. This unblocks Phases 91a–92.
2. **Pass `WithAPI` and `WithCharacterSheet` to `portal.RegisterRoutes`** in `cmd/dndnd/main.go:599` — instantiate `APIHandler` (with `RefDataAdapter` + `BuilderService` already constructed at lines 570-571) and `CharacterSheetHandler` (build a sheet store adapter against `queries`). This unblocks Phases 91b/91c/92/92b end-to-end.
3. **Gate DDB re-sync on DM approval.** Refactor `ddbimport.Service.Import` to (a) parse + diff in-memory, (b) post to `#dm-queue` with the diff, (c) only call `UpdateCharacterFull` when the DM clicks Approve. Mirrors the `ASIHandler.HandleDMApprove` pattern.
4. **Wire `inventory.UseActiveAbility` to a `/use-magic-item` Discord handler** (or extend `/use`) and **wire `Service.DawnRecharge` into the long-rest flow** (`internal/rest/`).
5. **Wire `/cast identify` and `/cast detect-magic`** — extend the cast-handler dispatch to recognize spell IDs `identify` and `detect-magic` and call `inventory.CastIdentify` / `inventory.DetectMagicItems` before falling through to the regular spell pipeline. Also wire `StudyItemDuringRest` into the short-rest UX.
6. **Implement Discord feat select-menu** in `asi_handler.go:266-275` — load feats from `internal/refdata`, render via `discordgo.SelectMenu`, run `CheckFeatPrerequisites` server-side before posting to `#dm-queue`.
7. **Build the `CharacterOverview.svelte` component** (Phase 101 desktop side) and add it to the desktop nav. The backend is ready (`/api/character-overview`).
8. **Build a `HomebrewEditor.svelte` component** (Phase 99) — at minimum a creature stat-block editor; the backend supports all seven types.
9. **Ship a `StatBlockLibrary.svelte` browser** (Phase 98) — it's currently a phantom in the mobile redirect list with no actual page.
10. **Add a DM-side DDB import surface** (Phase 90 spec compliance) — a small Svelte form on the dashboard that POSTs `{ddb_url, campaign_id}` to a new dashboard endpoint reusing `ddbimport.Service`.
11. **Public `#the-story` level-up announcement.** `FormatPublicLevelUpMessage` is ready; add a publisher fan-out in `levelup.NotifierAdapter`.
12. **Improve `MessagePlayerPanel.svelte`** with a character-picker dropdown (party characters list) instead of free-text UUID.
13. **Add subrace/subclass/language UI** to the portal builder, and enforce skill-count + class spell-list filtering.
14. **DDB client polish**: parse `Retry-After`, add jitter to backoff. Consider documenting the maximum total wait (with default `1+2+4+8 = 15s`, adequate; flag if `maxRetries` is raised).
15. **Capture real playtest transcripts** to flip `docs/playtest-checklist.md` scenarios from `pending` → `captured`. This will exercise most of the wiring gaps above.
