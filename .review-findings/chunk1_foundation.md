# Chunk 1 Review — Phases 1–10 (Foundations)

## Summary

Phases 1–10 are largely solidly implemented and well tested. The migration framework, reference data seeders, character math, registration logic, OAuth2 flow, and Discord bot core all match the spec. SRD reference counts meet or exceed the spec targets (37 weapons, 13 armor, 16 conditions including `surprised`, 12 classes, 9 races, 41 feats, 358 spells, 327 creatures, 70 magic items). The most material gap is in **Phase 9b**: the per-channel `MessageQueue` with 429 backoff is implemented and unit-tested, but it is not wired into the live bot — production sends bypass the queue entirely. **Phase 5** has a similar "implemented but not wired" gap: `LogSpellValidationWarnings` exists with full tests but is never called at startup. A handful of smaller drifts are noted below (spec says "+1 Longsword" spot-check, code uses generic "weapon-plus-1"; phase doc mentions 15 conditions but seeder ships 16, etc.).

## Per-phase findings

### Phase 1 — Project scaffold, Chi, slog, /health, Dockerfile, fly.toml
- Confirmed: Chi router skeleton at `internal/server/router.go:11-20` with `PanicRecovery` middleware (`internal/server/middleware.go:13`) and `/health` (`internal/server/health.go:39`).
- Confirmed: Structured slog JSON logger at `internal/server/logging.go:11-19`, level toggled by `debug` flag.
- Confirmed: Dockerfile multi-stage at `Dockerfile:1-19` (CGO disabled, alpine base, `/data/assets` for volume mount), exposes 8080.
- Confirmed: `fly.toml:1-26` defines app, primary_region, http_service with `/health` check and `dndnd_data` volume mount at `/data`.
- Confirmed: `Makefile` has `build`, `test`, `cover`, `cover-check`, `docker-build`, `run`. Single binary built to `bin/dndnd`.
- Confirmed: `cmd/dndnd/main.go:299` wires `server.NewRouter`, registers health subsystem checks.

### Phase 2 — PostgreSQL pool, goose migrations, sessions table, testcontainers
- Confirmed: pgx pool via stdlib at `internal/database/database.go:7-28` (DSN-based, ping check).
- Confirmed: goose `MigrateUp` / `MigrateDown` wrappers at `internal/database/migrate.go:11-48`.
- Confirmed: `campaigns` migration `db/migrations/20260310120000_create_campaigns.sql` and `sessions` migration `db/migrations/20260310120001_create_sessions.sql` (HTTP-only-ready columns, expires_at default 30 days, indexes on `discord_user_id`/`expires_at`).
- Confirmed: Testcontainers helper at `internal/testutil/testdb.go:20-65` with shared-container optimization (`SharedTestDB`, `AcquireDB`) and FK-safe `TruncateUserTables` (line 193).
- Confirmed: Round-trip integration test at `internal/database/integration_test.go:11` (MigrateUp → insert → verify) and `MigrateDown` test starting at line 59.

### Phase 3 — Weapons, armor, conditions_ref + SRD seeder
- Confirmed: Migration `db/migrations/20260310120002_create_reference_tables.sql` creates the three tables with weapon_type / armor_type CHECKs and `mechanical_effects` JSONB.
- Confirmed: 37 weapons seeded across all four weapon_type categories at `internal/refdata/seeder.go:86-132`. Verified against `WeaponCount = 37` (`seeder.go:13`) and `TestIntegration_SeedAll_WeaponCount` (`seeder_test.go:21`).
- Confirmed: 13 armor pieces (light/medium/heavy/shield) at `seeder.go:134-156`.
- Concern (minor): `seeder.go:168` declares `MechanicalEffect` JSON struct, then 16 conditions seeded at `seeder.go:175-350`. Phase 3 doc says "15 standard conditions"; seeder includes the extra `surprised` condition (line 322) plus tests for it (`seeder_surprised_test.go`). Consistent with spec line 1292+ surprise mechanics, but Phase 3 doc text is now stale (`docs/phases.md:16`).
- Confirmed: Snapshot tests `TestIntegration_SeedAll_WeaponCount` (37), `TestIntegration_SeedAll_ArmorCount` (13), `TestIntegration_SeedAll_ConditionCount` (16) and spot checks (`TestIntegration_SeedAll_LongswordHasVersatile` at `seeder_test.go:51`, `_PlateArmor` at line 90, `_StunnedCondition` at line 121).
- Confirmed: sqlc-generated queries compile (`internal/refdata/weapons.sql.go`, `armor.sql.go`, `conditions.sql.go`).

### Phase 4 — Classes, races, feats + SRD seeder
- Confirmed: Migration `db/migrations/20260310120003_create_classes_races_feats.sql` includes `features_by_level` JSONB, `hit_die`, `save_proficiencies`, `spellcasting`, `subclasses` JSONB, and multiclass JSONB columns.
- Confirmed: All 12 SRD classes seeded (`internal/refdata/seed_classes.go` lines 16,49,86,120,154,187,220,258,292,327,362,394 — barbarian … wizard).
- Confirmed: 9 races, 41 feats per `RaceCount`/`FeatCount` constants (`seeder.go:17`) and integration tests at `seeder_test.go:186` (RaceCount), 201 (FeatCount), 216 (`TestIntegration_SeedAll_FighterClass`), 264 (Wizard spellcasting), 291 (Elf race), 337 (Great Weapon Master feat).
- Confirmed: Spec spot checks present.

### Phase 5 — Spells (~300 SRD with resolution_mode)
- Confirmed: Migration `db/migrations/20260310120004_create_spells.sql` covers full schema (level, school, components, ranges, AoE JSONB, save_ability/effect, attack_type, conditions_applied, teleport JSONB, **resolution_mode** with default `dm_required`).
- Confirmed: 358 spells across `seed_spells_cantrips.go` … `seed_spells_9.go` (`grep -E '^\s*\{ID:'` count = 358) — exceeds spec's "~300" target.
- Confirmed: `resolution_mode` tagged on every entry (61 in `seed_spells_1.go` etc.); spot-check tests assert `auto` for Fireball/Cure Wounds/Misty Step (`seeder_test.go:428,518,545`) and `dm_required` for Wish/Polymorph (`seeder_test.go:572,588`).
- **Concern (gap)**: `LogSpellValidationWarnings` defined in `internal/refdata/validate_spells.go:95` is **never called** in production wiring (no hits in `cmd/dndnd/main.go`). The done-when criterion "validation warnings logged for data quality issues" is satisfied only at test-time via `TestValidateSpells_SRDDataQuality` (`validate_spells_test.go:211`). At minimum this should run once during seed/startup so warnings surface in operational logs.
- Confirmed: Snapshot test `TestIntegration_SeedAll_SpellCount` (`seeder_test.go:413`) and per-class/level/school/resolution_mode listings (lines 619-696).

### Phase 6 — Creatures (~325) + magic items
- Confirmed: Migration `db/migrations/20260310120005_create_creatures_magic_items.sql` (verified by integration test that inserts test rows at `refdata_integration_test.go:90,97`).
- Confirmed: 327 creatures total (179 low-CR + 55 CR3-5 + 17 CR6-7 + 76 high-CR), assembled by `seed_creatures.go:15-23` (`slices.Concat`). Matches `CreatureCount = 327` (`seeder.go:19`).
- Confirmed: 70 magic items in `seed_magic_items.go` (`MagicItemCount = 70`).
- Confirmed: Spec's spot-checks present — Goblin AC/HP/CR at `refdata_integration_test.go:127-140` (15/7/"1/4"), Adult Red Dragon at line 142, Bag of Holding at line 175.
- ⚠ Concern (drift): Spec done-when says "+1 Longsword magic_bonus", but the seeded entry uses generic `weapon-plus-1` (`seed_magic_items.go:15`) and the test spot-check uses that ID (`refdata_integration_test.go:163-173`). Functionally equivalent — `MagicBonus = 1`, rarity uncommon — but does not match the spec verbatim. Consider adding a named "+1 Longsword" entry or updating the phase note.

### Phase 7 — Characters table & derived stat math
- Confirmed: Schema `db/migrations/20260310120006_create_characters.sql` covers ability_scores, classes JSONB, spell_slots, pact_magic_slots, hit_dice_remaining JSONB, feature_uses JSONB, proficiencies JSONB, equipped_main_hand/off_hand/armor, ac_formula, gold, attunement_slots, languages array, inventory JSONB, ddb_url, homebrew flag.
- Confirmed: `internal/character/stats.go` — `TotalLevel` (line 9), `CalculateHP` with first-class-max-die rule (line 21), `CalculateAC` with armor + Unarmored Defense + shield + magic AC bonuses (line 56), `evaluateACFormula` for "10 + DEX + WIS"-style strings (line 92).
- Confirmed: `spellslots.go` — full multiclass spell-slot table 1–20 (line 8), `MulticastSpellSlots` (line 34), `CalculateCasterLevel` with full/half/third weighting (line 47).
- Confirmed: `modifiers.go` covers proficiency bonus, ability mods, skill mods (incl. expertise/Jack-of-all-Trades) — verified by `modifiers_test.go` (84 lines) and `stats_test.go` (348 lines).
- Confirmed: JSONB round-trip integration test `TestIntegration_CharacterJSONBRoundTrip` (`integration_test.go:38-214`) writes and re-reads classes, ability_scores, proficiencies, feature_uses, inventory, spell_slots, attunement_slots, features.

### Phase 8 — player_characters table + registration logic
- Confirmed: Migration `db/migrations/20260310120007_create_player_characters.sql` with status CHECK constraint, `created_via` CHECK, two unique constraints (`campaign_id, discord_user_id` + `campaign_id, character_id`).
- Confirmed: `Service.Register` with case-insensitive exact match (`service.go:53`), then Levenshtein fuzzy fallback (`service.go:89` → `fuzzy.go`).
- Confirmed: Status transitions table at `service.go:40` allows `pending → {approved, changes_requested, rejected, retired}`. Tests `TestIntegration_StatusTransitions` (`integration_test.go:146-223`) cover all four.
- Confirmed: Unique-constraint test `TestIntegration_UniqueConstraints` (`integration_test.go:255`).
- ⚠ Concern (intentional but worth flagging): Phase 8's transition table only allows transitions from `pending`. Spec line 33+ describes `/retire` operating on an *approved* character. That `approved → retired` flow is implemented later (Phase 14/16 dashboard layer issues retirement requests as new pending dm_queue items). Phase 8 in isolation cannot retire an already-approved PC — it relies on a higher-layer workflow. Consistent with phase scope wording, but a reader might expect it.
- Confirmed: Fuzzy unit tests `TestFindFuzzyMatches` and `TestLevenshteinDistance` (`fuzzy_test.go:7,61`).

### Phase 9a — Discord bot core
- Confirmed: `Bot` struct at `internal/discord/bot.go:11` tracks guilds in a sync.RWMutex-guarded map; `HandleGuildCreate` (`bot.go:86`) registers commands per guild via `trackAndRegister` (line 72).
- Confirmed: Idempotent registration with stale-command cleanup at `commands.go:522-554` (fetches existing, bulk-overwrites, then deletes any not in current set).
- Confirmed: Welcome DM via `SendWelcomeDM` (`welcome.go:22-35`) creating user channel then sending campaign-aware message; tested by `TestSendWelcomeDM_Success/_ChannelCreateError/_SendError` (`welcome_test.go:11,36,52`) and `TestBot_HandleGuildMemberAdd_*` (`bot_test.go:60+`).
- Confirmed: Permission validation at `permissions.go` — Send Messages, Attach Files, Manage Messages, Use Application Commands, Mention Everyone — matches spec lines 153-159 verbatim.
- Confirmed: `Session` interface at `session.go:6-19` and `discordfake.Fake` (`internal/testutil/discordfake/fake.go:21`) with compile-time `var _ discord.Session = (*Fake)(nil)`.
- Confirmed: Live gateway open in `cmd/dndnd/main.go:659` via `rawDG.Open()` and `discordgo.New("Bot " + token)` at line 222.

### Phase 9b — Message queue, splitting, rate-limit backoff
- Confirmed: Constants `MaxMessageLen = 2000`, `MaxSplitLen = 6000` (`message.go:9-14`) match spec.
- Confirmed: `SplitMessage` at `message.go:67` splits to ≤3 parts at newline boundaries (`splitAtNewlines` line 86), falls back to hard-cut (`splitHardCut` line 111). `SendContent` (`message.go:25`) auto-fallbacks to `.txt` attachment via `ChannelMessageSendComplex` for >6000 (`message.go:36-49`).
- Confirmed: `MessageQueue` (`queue.go:28`) per-channel FIFO with 429 backoff: `defaultSend` (`queue.go:149`) recognizes both `discordgo.RateLimitError` and `RESTError` with HTTP 429 and a `Retry-After` header. Re-enqueues at front and waits backoff + 0–100ms jitter (`queue.go:108-115,121-130`).
- Confirmed: Unit tests cover serialization, parallel channels, backoff, retry, error propagation, stop semantics (`queue_test.go:15,51,87,132,153,167,294`).
- ❌ **Gap (significant)**: `MessageQueue` is **never instantiated outside tests** — `grep NewMessageQueue` returns only the definition itself. The live bot (`cmd/dndnd/main.go`) wires no queue, so production outbound traffic uses `SendContent` / direct `ChannelMessageSend` calls without per-channel serialization or 429 retry. Phase 9b's done-when "rate-limit 429 triggers backoff and retry, per-channel queue serializes sends" is satisfied in principle (code + tests exist) but not in deployment. Either wire `MessageQueue` into the production session adapter, or document this as a deferred follow-up.

### Phase 10 — OAuth2 + 30-day sliding sessions
- Confirmed: `OAuthService` at `internal/auth/oauth2.go:97`. `HandleLogin` (line 124) generates 32-byte CSRF state cookie (5 min TTL) and redirects to Discord. `HandleCallback` (line 137) verifies state, exchanges code, fetches user, persists session, sets HTTP-only `dndnd_session` cookie.
- Confirmed: Cookie attrs at `oauth2.go:197-207` — `HttpOnly: true`, `Secure: s.secure`, `SameSite: http.SameSiteLaxMode`. Secure flag is configurable for dev.
- Confirmed: `SessionStore` SQL-backed at `session_store.go:26`. `GetByID` filters `expires_at > now()` (line 73), `SlideTTL` resets to `now() + 30 days` (line 95). `UpdateTokens` (line 105) used by transparent refresh.
- Confirmed: `SessionMiddleware` at `middleware.go:31` — checks cookie, fetches session, refreshes token if expired (line 53 → `refreshToken` line 74), slides TTL (line 64), injects Discord user ID into request context (line 68).
- Confirmed: 30-day default TTL constant `SessionTTL = 30 * 24 * time.Hour` (`oauth2.go:24`) and DB-side default `expires_at NOT NULL DEFAULT now() + INTERVAL '30 days'` (`db/migrations/20260310120001_create_sessions.sql:10`).
- Confirmed: Transparent refresh flow uses `oauth2.TokenSource` (`middleware.go:81-84`) which automatically swaps in the new token; UpdateTokens persists. On refresh failure the session is deleted (line 57). Tested by `TestSessionMiddleware_TokenRefreshSuccess/_TokenRefreshFails_DeletesSession` (`middleware_test.go:100,134`).
- Confirmed: Wired in `cmd/dndnd/main.go:192-208` (NewSessionStore, NewOAuthService, SessionMiddleware in `authBundle`).

## Cross-cutting risks

1. **"Implemented but not wired" anti-pattern.** Both Phase 5 (`LogSpellValidationWarnings`) and Phase 9b (`MessageQueue`) ship full code + tests, but the production binary never instantiates them. Easy to overlook because tests pass and the phase is checked off. Recommend a short audit in `cmd/dndnd/main.go` for any other `internal/...` constructor that is only referenced from `_test.go`.
2. **Phase 8/16 retirement boundary.** The pending → retired transition table (`registration/service.go:40`, `dashboard/approval_store.go:22`) is consistent within itself, but the spec's player-driven `/retire` flow on an already-approved PC depends on a higher-layer workflow (creating a new dm_queue item or similar). A future regression could leave approved-character retirement broken without either layer's tests catching it. Add a single end-to-end test that walks register → approve → /retire → DM-approve-retirement → unlinked.
3. **"+1 Longsword" spec drift.** Magic item `weapon-plus-1` is generic, not the specifically-named "+1 Longsword" the spec calls out. Low impact (DM can rename), but the spot-check spec text and the data don't quite match.
4. **Phase doc count drift.** `docs/phases.md:16` says "15 standard conditions"; seeder ships 16 (includes `surprised`). Tests assert 16. Update the phase text or split out `surprised` as a "phase 3 + bonus" note.
5. **Coverage exemption list.** `Makefile:7` excludes `internal/refdata/.*\.sql\.go`, `cmd/dndnd/main.go`, `cmd/dndnd/discord_handlers.go`, `cmd/dndnd/discord_adapters.go`, `cmd/dndnd/notifier.go`, `internal/discord/adapter.go`, `internal/testutil/.*\.go` from the 90/85 coverage gate. Reasonable for thin delegations and generated code, but the wiring layer is exactly where the "implemented but not wired" gaps surface — coverage won't catch them.

## Recommended follow-ups

1. **Wire `MessageQueue` into the production Discord adapter.** Either wrap `discord.SessionAdapter.ChannelMessageSend` to push through a single `*MessageQueue` per process, or expose a `BotSender` shim that all command handlers call. Add a smoke test in `cmd/dndnd/discord_adapters_test.go` that asserts the path traverses the queue.
2. **Call `LogSpellValidationWarnings(logger)` once in `main.go` after `SeedAll` succeeds** so any spell-data regressions show up in the deployment log instead of only in test runs.
3. **Add an end-to-end retirement test** that exercises the approved → retirement-request → retired path, since Phase 8 alone can't reach `retired` from `approved`.
4. **Reconcile spec spot-check vs. seed:** either rename `weapon-plus-1`/etc. to "+1 Longsword" SKUs, or update the spec / phase doc to refer to generic `weapon-plus-N` magic items.
5. **Update `docs/phases.md:16`** to say "16 standard conditions (incl. `surprised`)" so the doc matches the seeder + tests.
6. **Consider running `LogSpellValidationWarnings` in CI as a soft check** (fail-on-critical, warn-on-cosmetic) so future SRD additions can't silently regress.
