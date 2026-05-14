# Batch 02: Player commands, dashboard, cards, dice (Phases 13–18)

## Summary

All six phases are implemented and broadly faithful to the spec, but two
gaps stand out:

1. **Character cards do not auto-update outside combat.** `OnCharacterUpdated`
   is only fired from `internal/combat/service.go` (the `CardUpdater` hook).
   `/equip`, `/use`, `/give`, `/loot`, `/inventory` (DM mutations),
   `/prepare`, `/rest`, `/attune`, and the level-up flow do not invoke it.
   Spec line 216 says cards "auto-edit on state change (level up,
   equipment change, condition applied/removed, **rest completed**, etc.)."
   Out of combat, the `#character-cards` message goes stale until the
   next combat-side write touches the same character.
2. **Equip line on the card never changes.** `/equip` writes only the
   `inventory` JSONB (`UpdateCharacterInventory`). The card reads
   `characters.equipped_main_hand` / `equipped_off_hand`, which the
   handler never touches. Even if the OnCharacterUpdated hook were
   wired, the "Equipped:" line would still show the pre-/equip values.
3. **Dashboard WebSocket disables origin verification** (`InsecureSkipVerify:
   true` in `internal/dashboard/ws.go:119`). The comment says "Allow
   connections from any origin in dev" but the code path is the same in
   production. Session cookie is still required, but a malicious origin
   that the DM visits could open a cross-origin websocket using their
   browser session.

Phases 13/14/16/18 are otherwise complete and well-tested. Phase 15
shipped a Go-template shell + WebSocket reconnect — the spec also calls
for a "Svelte SPA stub embedded via embed.FS"; the FS is wired but the
SPA itself is just an `index.html` placeholder (acceptable for a stub).

## Per-phase findings

### Phase 13: Slash Command Registration — Player Commands
- Status: **matches**
- Key files:
  - `/home/ab/projects/DnDnD/internal/discord/commands.go` (full
    `CommandDefinitions` slice; per-guild bulk overwrite + stale-command
    cleanup at lines 633–664)
  - `/home/ab/projects/DnDnD/internal/discord/router.go` (StatusAware
    stubs for game commands)
  - `/home/ab/projects/DnDnD/internal/discord/bot.go` (GuildCreate +
    startup sweep handlers)
- Findings:
  - All 34 player commands from `phases.md` line 71 are registered
    (verified by name in `commands.go`).
  - `/setup` carries `DefaultMemberPermissions = ManageChannels` (good).
  - Idempotent bulk-overwrite + stale-command-delete on every startup
    matches spec lines 178–181.
  - Minor divergence: spec example `/attack G2 --gwm` is a flag-style
    syntax, but Discord slash commands don't support free-form flags —
    the implementation models each modifier as its own boolean option
    (`gwm`, `sharpshooter`, `reckless`, `twohanded`, `offhand`,
    `thrown`, `improvised`). This is the only sane Discord-native
    rendering; the spec text just shows the conceptual model.
  - `/cast` has the same flag-as-boolean shape and exposes all 8 SRD
    metamagic options as booleans.

### Phase 14: Player Registration Commands
- Status: **matches**
- Key files:
  - `/home/ab/projects/DnDnD/internal/registration/service.go`
  - `/home/ab/projects/DnDnD/internal/registration/fuzzy.go`
    (Levenshtein, max-dist = `max(len/2, 3)`, top-3)
  - `/home/ab/projects/DnDnD/internal/discord/registration_handler.go`
    (`RegisterHandler`, `ImportHandler`, `CreateCharacterHandler`,
    `StatusAwareStubHandler`, `GameCommandStatusCheck`)
  - `/home/ab/projects/DnDnD/internal/discord/welcome.go` + `bot.go`
    (GuildMemberAdd welcome DM per spec lines 185–200)
- Findings:
  - `/register`: case-insensitive exact match → pending PC; fuzzy
    suggestions when no exact; "no match" when none close.
  - `/import`: falls back to placeholder-character mode when DDB
    importer isn't wired; otherwise routes through `ddbimport.Service`
    with a real preview + DM-queue notice. Re-sync path is non-mutating
    until DM approval (spec lines 2418–2445 honoured).
  - `/create-character`: portal token issued via injected `tokenFunc`;
    URL is `{portalBaseURL}/create?token=...` with a 24h-expiry note in
    the ephemeral copy.
  - Status-aware game-command stubs (`StatusAwareStubHandler`) cleanly
    distinguish pending / changes_requested / rejected / approved per
    spec line 202.
  - **Minor:** `StatusCheckResponse` returns the empty string for the
    `approved` branch — callers must treat empty as "fall through".
    Used correctly by `GameCommandStatusCheck` but worth a comment.
  - **Minor (dead-ish):** `CharacterCreator` interface and
    `CreatePlaceholder` on the service are only exercised in the
    placeholder-import path and `/create-character`; the DDB-import path
    now produces real characters via `ddbimport.Service`, so the
    placeholder fallback in `handlePlaceholderImport` is mostly a
    test-only / DDB-disabled compatibility shim.

### Phase 15: DM Dashboard — Skeleton & Campaign Home
- Status: **matches** (with caveats)
- Key files:
  - `/home/ab/projects/DnDnD/internal/dashboard/handler.go` (sidebar
    `SidebarNav`, `CampaignHomeData`, inline template + JS for WS
    reconnect with exponential backoff: lines 304–333)
  - `/home/ab/projects/DnDnD/internal/dashboard/ws.go` (Hub + reader
    goroutine; `nhooyr.io/websocket`)
  - `/home/ab/projects/DnDnD/internal/dashboard/dm_middleware.go`
    (RequireDM; F-2)
  - `/home/ab/projects/DnDnD/internal/dashboard/embed.go` +
    `dashboard/assets/`
- Findings:
  - Sidebar covers Campaign Home, Character Approval, Encounter
    Builder, Stat Block Library, Asset Library, Map Editor, Exploration,
    Character Overview, Create Character, Errors — broader than the
    phase-15 scope (later phases stacked entries here). Acceptable.
  - WS reconnect: starts at 1 s, doubles to 30 s cap (line 305–306) —
    matches "exponential backoff" requirement.
  - **Security concern (med):** `websocket.AcceptOptions{
    InsecureSkipVerify: true }` (line 119) disables Origin checking
    in production. Should be gated on a dev flag or check
    `Origin == BASE_URL`. Session middleware does run upstream, but
    skipping origin verification still allows a CSRF-style WS upgrade
    if the DM visits a malicious page.
  - Counters (`SetCounters`) are wired via `approvalsCounter` /
    `dmQueueCounter` adapters in `cmd/dndnd/main.go:832`; failures
    degrade to 0 with a warn log — good.
  - Svelte SPA "stub" is just an `index.html` placeholder under
    `internal/dashboard/assets/`. Spec said "Svelte SPA stub embedded
    via embed.FS"; the embed wiring is real, the SPA content is
    minimal. Acceptable as a phase-15 stub.

### Phase 16: Character Approval Queue (Dashboard)
- Status: **matches**
- Key files:
  - `/home/ab/projects/DnDnD/internal/dashboard/approval.go`
    (`ApprovalStore`, `CharacterCardPoster`, `PlayerNotifier`
    interfaces)
  - `/home/ab/projects/DnDnD/internal/dashboard/approval_handler.go`
    (ServeApprovalPage, ListApprovals, GetApproval, Approve,
    RequestChangesHandler, Reject; WS `approval_updated` broadcast)
  - `/home/ab/projects/DnDnD/internal/dashboard/approval_store.go`
- Findings:
  - All five spec verbs (review, approve, request-changes, reject,
    retire) implemented. Retire is folded into `Approve` when
    `detail.CreatedVia == "retire"` (handler line 248) and routes
    through `cardPoster.UpdateCardRetired`.
  - F-2 DM-only mount is wired in main.go:853 via
    `approvalHandler.RegisterApprovalRoutes(router.With(dmAuthMw))`.
  - `parseFeedbackRequest` does auth + ID-parse + body-parse + detail
    lookup in one helper, but the handler then re-parses ID in two
    paths (`Reject`, `RequestChangesHandler`) — only because
    `parseFeedbackRequest` already returns the detail. Reads cleanly.
  - **Minor:** WebSocket broadcast payload is just `{type, id}` — the
    client refetches `/dashboard/api/approvals` (handler.go line 540).
    Snapshot-based per spec line 106. Good.
  - **Minor:** `ApprovalHandler` constructor still takes a fixed
    `campaignID` arg even though production always overrides via
    `SetCampaignLookup`. Acceptable for tests but the constructor
    signature is now slightly misleading.

### Phase 17: Character Cards (`#character-cards`)
- Status: **partial**
- Key files:
  - `/home/ab/projects/DnDnD/internal/charactercard/service.go`
    (PostCharacterCard, UpdateCard, OnCharacterUpdated, retire badge)
  - `/home/ab/projects/DnDnD/internal/charactercard/format.go`
    (FormatCard; condition / spell-slot / concentration / exhaustion /
    gold / languages rendering)
  - `/home/ab/projects/DnDnD/internal/charactercard/shortid.go`
    (deterministic short-ID assignment by `(campaign_id, name)`)
  - `/home/ab/projects/DnDnD/internal/combat/service.go:230-516`
    (`CardUpdater` hook firing on combatant mutations)
- Findings:
  - Format follows spec line 218–227 closely: name, short ID, level,
    race, multiclass `Fighter 5 (Champion) / Rogue 3 (Thief)` via
    `character.FormatClassSummary`, HP (+ temp HP), AC, speed, ability
    scores, equipped, spell slots, spells count, conditions,
    concentration, exhaustion, gold, languages.
  - Combat-side state (conditions, concentration, exhaustion) is
    merged in from the active combatant row via
    `GetActiveCombatantByCharacterID` (service.go line 263) — addresses
    the "deferred fields" MEMORY note.
  - **Gap (high):** Auto-update triggers are mostly missing outside of
    combat:
    - `OnCharacterUpdated` is fired only from
      `internal/combat/service.go` (notifyCardUpdate*).
    - `/equip` (internal/discord/equip_handler.go) writes inventory
      and returns — no `OnCharacterUpdated` call.
    - `/use`, `/give`, `/loot` (loot/inventory packages) — none fire
      the hook.
    - `/attune`, `/unattune` — no hook.
    - `/rest short` / `/rest long` (rest_handler.go) — no hook;
      `#character-cards` HP is wrong until next combat write.
    - Level-up flow (`internal/levelup`) — no hook; the
      `#the-story` announcement fires, but the persistent card is
      not re-rendered.
    - `/prepare` — no hook; prepared-spell count on the card stays
      stale.
    - DM dashboard inventory API (`internal/inventory/api_handler.go`)
      — no hook.
    Spec line 216 explicitly lists "level up, equipment change,
    condition applied/removed, **rest completed**" as triggers. Combat
    is covered; everything else is not.
  - **Gap (high):** `/equip` only writes the `inventory` JSONB. The
    card's "Equipped:" line reads `characters.equipped_main_hand` /
    `equipped_off_hand`, which `UpdateCharacterInventory` does not
    touch (see db/queries/characters.sql line 88). So even if the
    auto-update hook were called, the equipped weapon would not
    change on the card.
  - Card-post side effects on retire approval go through
    `UpdateCardRetired` which writes a "🏴 RETIRED — " prefix; matches
    the spec's retire-badge intent.

### Phase 18: Dice Rolling Engine
- Status: **matches** (for phase scope)
- Key files:
  - `/home/ab/projects/DnDnD/internal/dice/dice.go` (Expression parser:
    NdM[+K], multiple groups, signed modifier)
  - `/home/ab/projects/DnDnD/internal/dice/roller.go` (Roll,
    RollDamage with crit-doubles-dice, GroupResult breakdown)
  - `/home/ab/projects/DnDnD/internal/dice/d20.go` (RollMode, RollD20,
    advantage/disadvantage/cancellation, crit hit/fail flags,
    CombineRollModes)
  - `/home/ab/projects/DnDnD/internal/dice/log.go` (RollLogEntry +
    RollHistoryLogger interface for the `#roll-history` posting hook)
- Findings:
  - 38 unit tests cover normal / advantage / disadvantage / cancel /
    crit-hit / crit-fail / damage-crit / breakdown formatting / invalid
    expression / spaces / negative modifier. Solid coverage.
  - `RollDamage(expression, critical=true)` doubles dice counts and
    applies modifier once per spec line 692. Multi-group damage
    (`1d8+2d6`) doubles each group. Good.
  - **Acceptable divergence from prompt wording:** the spec/phases do
    NOT actually require "keep-highest" or "exploding" — those are 5e
    house-rule extras. Spec spells out:
    - parse NdM+K (✓)
    - advantage/disadvantage (✓)
    - critical detection (✓ — nat 20 and nat 1)
    - modifier stacking (✓ via multi-group + modifier)
    - roll logging with full breakdown (✓ — `Breakdown` field +
      `RollLogEntry`)
    The Great Weapon Fighting style ("reroll 1s and 2s on damage") is
    listed in phases.md Phase 36 (Fighting Styles), not Phase 18 —
    deferred.
  - **Minor (dead code):** `GroupResult.Purpose` (roller.go line 21)
    is declared but never set by the roller; only the higher-level
    callers tag it. Acceptable.
  - **Minor:** `crypto/rand` panics if entropy is unavailable. For a
    server process this is acceptable; some prefer a logged fatal.

## Cross-cutting concerns

- **DM auth on dashboard mutation routes** — `RequireDM` is wired in
  `cmd/dndnd/main.go:762` (`dmAuthMw`) and applied to:
  - `/dashboard/queue/*` (RegisterDMQueueRoutes)
  - `/dashboard/exploration/*` (RegisterExplorationRoutes)
  - `/dashboard/approvals*` (approvalHandler.RegisterApprovalRoutes)
  - `/dashboard/errors*` (MountErrorsRoutes)
  - `/api/open5e/campaigns/{id}/sources` (GET + PUT)
  - `/api/inventory/*` (RegisterInventoryAPI)
  - DM char-create form
  Read-only `/dashboard/` (Campaign Home) sits on `authMw` only — the
  page degrades to 0/empty when the user is not a DM rather than 403.
  That's intentional (see `dm_middleware.go` comment); but
  `/api/me` does leak the "is or is not a DM" bit without 403.
- **Status-aware command gating** — Every game-command stub flows
  through `GameCommandStatusCheck`. Once real handlers replace the
  stubs (later phases), each handler must remember to re-run this
  check or the spec line 202 "❌ No character found." behaviour will
  regress. Worth a lint rule or a wrapping middleware in the router.
- **`OnCharacterUpdated` reach** — see Phase 17 above. The only place
  the hook fires is combat. Every other persistent-state mutation
  path needs to call it (or the character card needs to be re-rendered
  on a schedule).
- **WS Origin verification** — `InsecureSkipVerify: true` should be
  gated to non-prod.

## Critical items (must fix)

1. **Wire `OnCharacterUpdated` into all non-combat state mutators**
   (Phase 17): `/equip`, `/use`, `/give`, `/loot`, `/attune`,
   `/unattune`, `/rest` (post-rest), `/prepare`, level-up
   (`internal/levelup`), and DM inventory API
   (`internal/inventory/api_handler.go`). Without these, the
   persistent `#character-cards` message drifts out of sync any time
   the character changes outside of combat — directly contradicts
   spec line 216.
2. **Fix `/equip` to update `characters.equipped_main_hand` /
   `equipped_off_hand`** in addition to the inventory JSONB (or
   change the card formatter to read equipped weapons from inventory).
   Right now `/equip longsword` never changes the card's "Equipped:"
   line because the writer and reader look at different columns.
3. **Tighten WebSocket origin verification**
   (`internal/dashboard/ws.go:118-120`). Replace `InsecureSkipVerify:
   true` with an explicit Origin allowlist sourced from the configured
   BASE_URL; keep the loose mode behind an env flag for local dev.
