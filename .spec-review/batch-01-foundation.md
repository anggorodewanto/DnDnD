# Batch 01: Foundation (Phases 1–12)

## Summary

The foundation is broadly in place — all 12 phases have substantive implementation, migrations exist for every spec'd table, SRD seeders run, OAuth2 + session storage + sliding TTL + transparent refresh work, slash commands are registered per-guild with stale-command cleanup, message splitting and per-channel rate-limit queue are wired into production sends, and the startup recovery sequence respects spec lines 116–121 (DB → migrations → stale-state scan → gateway open → command re-register → ticker start). Several real bugs and divergences exist, most notably (1) the Discord `GuildCreate` and `GuildMemberAdd` handlers are defined but never registered with the gateway, so welcome DMs and runtime guild-join command registration never fire; (2) campaign pause/resume HTTP endpoints are mounted on the unauthenticated router; and (3) the `UNIQUE(campaign_id, discord_user_id)` constraint plus retire-only-sets-status-to-retired flow contradicts the spec line 40 promise that retired players can `/register` a new character.

## Per-phase findings

### Phase 1: Project Scaffolding & Build System
- Status: matches
- Key files: `cmd/dndnd/main.go`, `internal/server/router.go`, `internal/server/health.go`, `internal/server/middleware.go` (PanicRecovery), `Dockerfile`, `fly.toml`, `Makefile`, `go.mod`
- Findings:
  - Chi router + slog JSON logging + panic recovery + `GET /health` all present. Health subsystem checkers (db, discord) added later in Phase 112.
  - Single-binary build, Docker image, Fly deploy config. fly.toml correctly mounts `/data` for asset volume.
  - Minor: `internal/database/database.Connect` does not set `MaxOpenConns`/`MaxIdleConns` — DSN-level tuning is the only knob. Probably fine but no explicit pool sizing.

### Phase 2: Database Connection & Migration Framework
- Status: matches
- Key files: `internal/database/database.go`, `internal/database/migrate.go`, `db/migrations.go` (embed.FS), `internal/testutil/testdb.go`
- Findings:
  - goose v3 used with embedded FS, `MigrateUp`/`MigrateDown` symmetric. Initial migration creates both `campaigns` and `sessions`.
  - Integration test (`internal/database/integration_test.go`) uses testcontainers Postgres via `internal/testutil.NewTestDB`. Matches spec.
  - `sessions.created_at` / `updated_at` / `expires_at` defaults all OK; sliding TTL implemented via `UPDATE … SET expires_at = now() + INTERVAL '30 days'`.

### Phase 3: Reference Data Schema — Weapons, Armor, Conditions
- Status: matches (with one minor count divergence)
- Key files: `db/migrations/20260310120002_create_reference_tables.sql`, `internal/refdata/seeder.go`, `internal/refdata/weapons.sql.go`, `internal/refdata/armor.sql.go`, `internal/refdata/conditions.sql.go`
- Findings:
  - 37 weapons, 13 armor (incl. shield), 16 conditions seeded. Spec lists 15 SRD conditions; seeder includes a 16th "surprised" condition (commented as Phase 114 dependency for auto-skip path). Not a divergence from spec intent, but worth knowing.
  - `mechanical_effects` JSONB schema is consistent. `ac_dex_bonus`/`ac_dex_max`/`strength_req`/`stealth_disadv` all modeled. Versatile damage present.

### Phase 4: Reference Data Schema — Classes, Races, Feats
- Status: matches
- Key files: `db/migrations/20260310120003_create_classes_races_feats.sql`, `internal/refdata/seed_classes.go`, `internal/refdata/seed_races.go`, `internal/refdata/seed_feats.go`
- Findings:
  - `classes.features_by_level`, `hit_die`, `save_proficiencies`, `spellcasting` JSONB all present (spec line 21 callouts).
  - Counts: 12 classes, 9 races, 41 feats. Acceptable for SRD breadth.

### Phase 5: Reference Data Schema — Spells
- Status: matches
- Key files: `db/migrations/20260310120004_create_spells.sql`, `internal/refdata/seed_spells*.go`, `internal/refdata/spell_classify.go`, `internal/refdata/validate_spells.go`
- Findings:
  - 358 spells seeded; auto-classification of `resolution_mode` ('auto' vs 'dm_required') is present (`spell_classify.go`) with a validate step that logs warnings. F-21 follow-up (algorithmic invariant + audit) recently merged per recent commit log.
  - `area_of_effect`, `damage`, `healing`, `conditions_applied`, `teleport`, `save_ability`, `save_effect`, `attack_type`, `material_*`, `ritual`, `concentration`, `higher_levels`, `classes` columns all present.

### Phase 6: Reference Data Schema — Creatures & Magic Items
- Status: matches
- Key files: `db/migrations/20260310120005_create_creatures_magic_items.sql`, `internal/refdata/seed_creatures*.go`, `internal/refdata/seed_magic_items.go`
- Findings:
  - 327 creatures, 70 magic items. Snapshot spot-check tests exist (`seeder_test.go` covers Goblin, Longsword versatile, etc.).
  - One observation: `seed_creatures_*.go` is split into 5 files (low/mid/high CR) — purely cosmetic, fine.

### Phase 7: Characters Table & Core Character Logic
- Status: matches
- Key files: `db/migrations/20260310120006_create_characters.sql`, `internal/character/types.go`, `stats.go`, `spellslots.go`, `modifiers.go`
- Findings:
  - All spec'd columns present (ability_scores, classes JSONB, spell_slots, pact_magic_slots, hit_dice_remaining, feature_uses, proficiencies, inventory, equipped_*, ac_formula, gold, attunement_slots, languages).
  - Derived stat calculation: proficiency bonus (level-based), HP (per-class hit dice + CON), AC (armor + dex cap + shield + formula), spell slots (multiclass table + caster level rollup), saving throws + skills (incl. expertise + Jack of All Trades) all implemented.
  - `CalculateHP` correctly gives max hit die at level 1 only for the FIRST class entry. This is a defensible interpretation but D&D 5e PHB says max hit die at level 1 attaches to the FIRST class taken at character creation, regardless of slot order in the `classes` array. If the caller's ordering doesn't reflect "first class taken", HP is wrong. Worth a comment.
  - JSONB round-tripping tested in `internal/character/integration_test.go`.

### Phase 8: Player Characters Table & Registration Logic
- Status: divergent (constraint + retire conflict)
- Key files: `db/migrations/20260310120007_create_player_characters.sql`, `db/migrations/20260511120000_extend_player_characters_created_via_retire.sql`, `internal/registration/service.go`, `internal/registration/fuzzy.go`
- Findings:
  - Levenshtein fuzzy match with case-insensitive normalization and dynamic distance cap (`max(len(query)/2, 3)`). Returns up to 3 sorted by distance. Matches spec lines 47–49.
  - Status transition table is well-defined; valid transitions enforce pending→{approved,changes_requested,rejected,retired} and approved→retired only.
  - **DIVERGENCE / spec contradiction**: spec line 40 says "On approval [of retirement]: ... the player is unlinked — they can now /create-character, /import, or /register a new character." The schema has `UNIQUE(campaign_id, discord_user_id)`, and the retire path only flips `status` to `retired`; the row is never deleted nor `discord_user_id`-nulled. So `INSERT INTO player_characters` for a new character will fail the unique constraint, blocking the spec'd "register again after retire" flow. Either the unique constraint needs to be partial (e.g., `WHERE status NOT IN ('retired','rejected')`) or the retire path needs to free the slot.
  - `Register()` returns sql error from the unique constraint as an opaque "creating player character: …" — re-registering the same player will surface a confusing error instead of a status-aware response.

### Phase 9a: Discord Bot Foundation — Core
- Status: partial (handlers defined but never wired)
- Key files: `internal/discord/bot.go`, `internal/discord/commands.go`, `internal/discord/welcome.go`, `internal/discord/permissions.go`, `cmd/dndnd/main.go:1197-1211`
- Findings:
  - `RegisterCommands` does a `ApplicationCommandBulkOverwrite` then loops over previously-existing commands deleting any not in the current set — but bulk overwrite already replaces the full command set, so the subsequent delete pass is dead code (Discord no-ops removed commands as part of bulk overwrite). Harmless but misleading.
  - **BUG**: `Bot.HandleGuildCreate` and `Bot.HandleGuildMemberAdd` are defined but **never** registered with `rawDG.AddHandler(...)`. Only `InteractionCreate` is wired (main.go:1198). Consequences:
    - Guilds the bot joins **after** startup do not get commands registered until the bot restarts. Spec line 179: "On guild join, the bot registers commands for the new guild via the GuildCreate event handler" — not implemented.
    - **Welcome DMs never fire** on member join (spec lines 183–200). The handler is implemented and unit-tested in isolation but unreachable in production.
  - **BUG / production concern**: no `Identify.Intents` configured on the `discordgo.Session`. discordgo's default is `IntentsAllWithoutPrivileged`, which excludes `GuildMembers` (privileged). Even if `HandleGuildMemberAdd` were wired, member-join events would not arrive without enabling the privileged GuildMembers intent in the Discord Developer Portal AND in code.
  - `ValidatePermissions` / `RequiredPermissions` are defined and unit-tested but never **called** in production code. Phase 9a done-when says "permission validation works" — only the helper works; nothing actually validates the bot's perms on startup or on GuildCreate.
  - `setupPermission` uses `PermissionManageChannels` — sensible gate for `/setup`.

### Phase 9b: Discord Bot Foundation — Message Queue & Rate Limiting
- Status: matches
- Key files: `internal/discord/message.go`, `internal/discord/queue.go`, `cmd/dndnd/main.go:496-501`
- Findings:
  - `SplitMessage` correctly handles ≤2000, 2001–6000 split-at-newlines (up to 3 parts), and >6000 via .txt attachment. Falls back to hard cut when newline splitting can't fit 3 parts. Matches spec lines 206–212.
  - `MessageQueue` per-channel FIFO with 429 backoff (parses `Retry-After` header, also handles `discordgo.RateLimitError`). Jitter 0-100ms added on resume. Goroutine-per-channel drain.
  - Production wrapping done via `newQueueingSession(...)` in main.go so every `ChannelMessageSend` goes through the queue.
  - Minor: `flushErrors` on Stop() drains pending items with `ErrQueueStopped` per-channel only when `drain` notices the close — drains in flight before stop may still send. Acceptable.

### Phase 10: Discord OAuth2 & Session Management
- Status: matches
- Key files: `internal/auth/oauth2.go`, `internal/auth/middleware.go`, `internal/auth/session_store.go`, `cmd/dndnd/main.go:327-363`
- Findings:
  - OAuth2 login → callback → session create → cookie set; logout deletes session and clears cookie. State cookie + CSRF state check on callback present.
  - Server-side session storage in `sessions` table (HTTP-only, SameSite=Lax, Secure tied to `COOKIE_SECURE=true`). 30-day TTL with sliding via `SlideTTL`.
  - Transparent token refresh: when `TokenExpiresAt.Before(now)`, calls `TokenSource(...).Token()`, updates stored tokens; on refresh failure deletes session.
  - Spec says scopes "Discord OAuth2" — implementation uses only `["identify"]`. That's the minimum needed; matches spec since spec never enumerates required scopes.
  - **Concern (low)**: `HandleCallback` uses `http.StatusTemporaryRedirect` (307) which preserves method on redirect — for a GET callback flow this is fine, but `StatusSeeOther` (303) is more idiomatic. Cosmetic.
  - **Concern (low)**: session cookie `MaxAge` is set to 30 days at creation but `SlideTTL` only updates `expires_at` in DB — it does NOT re-issue the cookie. The cookie will expire client-side at 30 days from issuance even if the DB row is fresh. After cookie expiry the user re-authenticates. Not a security issue, but the cookie+DB TTLs drift apart over time.
  - Middleware fall back to passthrough when `DISCORD_CLIENT_ID`/`SECRET` are unset is documented and reasonable for local dev.

### Phase 11: Campaign CRUD & Multi-Tenant Scoping
- Status: divergent (pause/resume unauthenticated)
- Key files: `internal/campaign/service.go`, `internal/campaign/handler.go`, `db/queries/campaigns.sql`, `cmd/dndnd/main.go:633-635`
- Findings:
  - `Settings` struct carries `turn_timeout_hours`, `diagonal_rule`, `open5e_sources`, `channel_ids`, `auto_approve_rest`. Status transitions guard against re-entering same state and against transitioning from archived.
  - Campaign creation requires non-empty guild_id, dm_user_id, name. `UNIQUE(guild_id)` enforces one-campaign-per-guild.
  - **BUG / security**: `campaignHandler.RegisterRoutes(router)` mounts `POST /api/campaigns/{id}/pause` and `/resume` on the **public** router. Other DM-only endpoints (approvals, exploration, inventory mutations, Open5e source toggle) go through `dmAuthMw`. Any unauthenticated caller with a campaign UUID can pause/resume a live campaign. Should be mounted on `router.With(dmAuthMw)` like the other DM-only routes.
  - All queries are scoped by `guild_id` or `campaign_id` at the entry points (`GetCampaignByGuildID`, `ListPlayerCharactersByCampaign`, etc.). Note that `characters.sql` lookups by primary key are not campaign-scoped — callers must derive `campaign_id` from elsewhere to avoid cross-tenant lookup, which is the case throughout the codebase but worth being conscious of.

### Phase 12: Discord Channel Structure (`/setup`)
- Status: matches (with one quirk)
- Key files: `internal/discord/setup.go`, `cmd/dndnd/main.go:1151-1154`, `cmd/dndnd/discord_adapters.go:131-163,679-706`
- Findings:
  - Creates 4 categories × {3,3,2,2} = 10 channels matching spec lines 131–151. Existing categories/channels are skipped by name match.
  - Permission overwrites: `#the-story` DM-write-only, `#combat-map` bot-write-only, `#dm-queue` DM+bot-only (view+send) and @everyone denied. Matches spec.
  - Channel IDs are persisted into `campaigns.settings.channel_ids` via `UpdateCampaignSettings`, preserving other settings on merge.
  - **Quirk**: `GetCampaignForSetup` auto-creates a campaign if no row exists for the guild, with the invoker as DM and `name = "Campaign for guild <id>"`. Useful for playtest quickstart but means the FIRST user with `ManageChannels` to run `/setup` becomes DM by default. The `DefaultMemberPermissions = ManageChannels` gate provides the intended check. Acceptable but worth documenting.
  - Initial `/setup` response is `DeferredChannelMessageWithSource` which is a public response, not ephemeral — spec doesn't require ephemeral here. Fine.

## Cross-cutting concerns

- **GuildCreate / GuildMemberAdd handlers unreachable** (Phase 9a): These two handlers were implemented and tested, but main.go never wires them to discordgo's `AddHandler`. Combined with missing privileged-intent configuration, the welcome-DM flow (spec lines 183–200) and the dynamic-guild-join command-registration path (spec line 179) are both inert in production.
- **Bot permission validation never called**: `ValidatePermissions` exists but no caller. Phase 9a done-when says it works — only as a helper.
- **Dead code in `RegisterCommands`**: The post-bulk-overwrite delete loop is redundant. `ApplicationCommandBulkOverwrite` already removes stale commands.
- **Unique-constraint + retire-status conflict**: Phase 8 schema enforces `UNIQUE(campaign_id, discord_user_id)`, but Phase 8/16 retire flow only flips status to `retired`. Spec line 40 promises the player "is unlinked" so they can re-register — schema doesn't allow it.
- **`/api/campaigns/{id}/pause`+`/resume` unauthenticated**: Phase 11 handler mount bypasses `dmAuthMw`.
- **Session cookie MaxAge does not slide**: only the DB `expires_at` slides; browser cookie still expires 30 days after issuance regardless of activity. Re-auth required at the cookie boundary even if DB session is fresh.
- **Multi-tenant query scoping is per-entry-point, not enforced at the table level**: `GetCharacter(id)` returns by PK without checking `campaign_id`. Defense-in-depth would scope by `(id, campaign_id)`. Not a current bug because all callers derive campaign from another source.

## Critical items (must fix)

- **Wire `Bot.HandleGuildCreate` and `Bot.HandleGuildMemberAdd` with `rawDG.AddHandler(...)`** (and configure `discordgo.Identify.Intents` to include `IntentsGuildMembers`, plus enable the privileged intent on the Discord Developer Portal). Otherwise welcome DMs and post-startup guild-join command registration are broken — both are explicit Phase 9a done-when items.
- **Move `/api/campaigns/{id}/pause` and `/resume` behind `dmAuthMw`** (or equivalent). Currently any unauthenticated caller who guesses or learns a campaign UUID can pause/resume a live campaign, posting unwanted `#the-story` announcements and stalling player turns.
- **Resolve retire/unique-constraint contradiction (Phase 8 vs spec line 40)**. Either (a) make the unique constraint partial (`WHERE status NOT IN ('retired','rejected')`) so the player can re-register after retirement, or (b) explicitly null out `discord_user_id` on retire approval. Without one of these, the spec'd "retire then make a new character in the same campaign" flow is impossible.
