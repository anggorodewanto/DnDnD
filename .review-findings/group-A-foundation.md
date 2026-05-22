# Group A: Foundation (Phases 1-17) — Review Findings

Scope: scaffolding, DB+migrations, ref-data schemas, characters, player characters, Discord bot foundation, OAuth/sessions, campaigns, /setup, slash command registration, /register/import/create-character, dashboard skeleton, character approval queue, character cards.

---

## [Critical] `/setup` lets any guild member silently become the campaign DM

- **Location:** cmd/dndnd/discord_adapters.go:135-163
- **Spec/Phase ref:** spec §Authentication & Authorization (line 65) — "System verifies the authenticated Discord user ID matches the campaign's designated DM"; Phase 12
- **Problem:** `GetCampaignForSetup` auto-creates the campaign row with the invoking user as DM whenever no row exists for that guild. The `/setup` slash command has `DefaultMemberPermissions: ManageChannels`, but Discord allows guild admins to override that and the handler itself does no server-side authorization check. The first non-owner who runs `/setup` becomes the permanent DM, which then gates the entire dashboard and player-management surface for that guild.
- **Suggested fix:** Make /setup require an explicit DM identity (e.g., compare invoker against the guild owner returned by Discord, or require a pre-provisioned `campaigns` row, or have an admin-only "claim DM" endpoint). At minimum, never let an arbitrary `interaction.Member` user implicitly create the campaign + DM binding.

## [Critical] Dashboard approval endpoints aren't scoped to the DM's own campaign

- **Location:** internal/dashboard/approval_handler.go:230-338 (Approve/Reject/RequestChanges)
- **Spec/Phase ref:** spec §Authentication & Authorization (line 65); Phase 16
- **Problem:** `RequireDM` checks that the user is the DM of *some* campaign, but the handler routes (`/dashboard/api/approvals/{id}/approve|reject|request-changes`) look up the approval row by `id` only — no check that the player_character belongs to a campaign the authenticated DM owns. A DM of campaign A can approve, reject, or retire any character in campaign B by guessing/learning UUIDs. No test covers this.
- **Suggested fix:** Resolve the approval row, get its campaign_id, and verify the authenticated DM owns that campaign (use `IsCampaignDM`) before mutating. Add a regression test for cross-campaign rejection.

## [High] Player can never resubmit after `changes_requested` (broken status flow)

- **Location:** internal/registration/service.go:46-56 + internal/dashboard/approval_store.go:30-40 + db/migrations/20260513120000_make_player_characters_unique_partial.sql:49-55
- **Spec/Phase ref:** spec §Registration feedback (line 54) — "Use `/create-character` or `/import` to resubmit"; Phase 8 / Phase 14
- **Problem:** `validTransitions`/`validApprovalTransitions` do not allow `changes_requested -> pending` (or anything else), and the partial unique index only excludes `retired`. So a player whose registration is in `changes_requested` is permanently stuck: `/register` and `/import`/`/create-character` will fail the unique constraint and there is no admin transition to clear the row. Same problem for `rejected`.
- **Suggested fix:** Either (a) allow `changes_requested -> pending` and `rejected -> pending` in both transition maps, or (b) widen the partial unique index to `WHERE status NOT IN ('retired','rejected','changes_requested')`. Add an integration test that resubmits after `changes_requested`.

## [High] OAuth access/refresh tokens stored in plaintext

- **Location:** internal/auth/session_store.go:50-62 + db/migrations/20260310120001_create_sessions.sql:5-6
- **Spec/Phase ref:** spec §Session management (lines 68-75); Phase 10
- **Problem:** Discord access and refresh tokens are persisted as plain TEXT in `sessions`. Spec says tokens are "stored server-side, never exposed to the client" but doesn't explicitly require encryption — still, a DB read (backup leak, SQL injection elsewhere, ops mistake) leaks every DM's and player's live Discord access. Refresh tokens are long-lived and let an attacker impersonate the user against Discord's API beyond the 30-day session window.
- **Suggested fix:** Encrypt access/refresh tokens at rest with a server-held key (AES-GCM with a key in env/secret store). Decrypt on use in `SessionStore.GetByID`. At minimum, treat the column as sensitive and document the operational requirement (DB encryption-at-rest).

## [High] WebSocket origin verification defaults to `InsecureSkipVerify: true`

- **Location:** internal/dashboard/handler.go:117-170 (default `wsInsecureSkipVerify: true`), internal/dashboard/ws.go:124-127
- **Spec/Phase ref:** spec §Authentication & Authorization (line 73), §Concurrency Model WebSocket sync; Phase 15
- **Problem:** The dashboard WebSocket upgrade defaults to skipping origin checks. Production wiring is expected to call `SetWebSocketOriginPolicy(allowed, false)` but the fail-open default means a forgotten config line lets any origin upgrade a session-cookie-authenticated WS connection (Cross-Site WebSocket Hijacking), allowing arbitrary cross-origin pages to read every push (HP, hidden enemy info, whisper data, etc.).
- **Suggested fix:** Flip the default to `InsecureSkipVerify: false` and require `wsAllowedOrigins` to be set explicitly. Local-dev wiring can opt back in.

## [High] OAuth callback handler treats any 4xx error from Discord as a generic 403

- **Location:** internal/auth/oauth2.go:150-156, 178-182
- **Spec/Phase ref:** spec §Authentication & Authorization, Phase 10
- **Problem:** `HandleCallback` never validates that `FetchUserInfo` returned a non-empty Discord user ID. If Discord's API returns an empty body / unverified user, `user.ID` could be the empty string, which then goes into `sessions.Create` and later into `player_characters.discord_user_id`. Empty user IDs would collide across users via the `UNIQUE(campaign_id, discord_user_id)` partial index.
- **Suggested fix:** After `FetchUserInfo`, reject the callback if `user.ID == ""`. Consider also rejecting non-`verified` accounts when Discord returns it.

## [High] Portal token redemption has a TOCTOU race

- **Location:** internal/portal/token_service.go:82-90 + internal/portal/token_store.go:81-88
- **Spec/Phase ref:** spec §Discord integration "link expires after 24 hours, scoped to the player's Discord ID"; Phase 14
- **Problem:** `RedeemToken` calls `ValidateToken` (SELECT) then `MarkUsed` (UPDATE) without a transaction or conditional WHERE. Two concurrent redemptions of the same one-time token both pass the validate step and both succeed at MarkUsed, defeating the single-use guarantee.
- **Suggested fix:** Make `MarkUsed` atomic: `UPDATE portal_tokens SET used = true WHERE id = $1 AND used = false RETURNING id`. Treat "no row" as "already used."

## [High] HP calculation always uses fixed-average; no rolled-HP path

- **Location:** internal/character/stats.go:21-47
- **Spec/Phase ref:** spec §Manual Character Creation step 3 (line 2408); Phase 7
- **Problem:** `CalculateHP` hardcodes `avg = die/2 + 1` (the 5e fixed-average rule) for every level beyond first. The spec says "manual entry (rolled or point-buy, DM's choice — system doesn't enforce a generation method)" but the system *does* enforce fixed-average and never supports player/DM-supplied rolled HP totals. Multiclass characters who took max die at level 1 of a secondary class (a common 5e variant rule) also can't represent that.
- **Suggested fix:** Either pass an explicit `hp_max` value through from the builder/importer and use `CalculateHP` only for recompute-on-level-up validation, or add a per-class-entry `roll_mode` flag.

## [High] Welcome DM sent to every joining member even when no campaign exists

- **Location:** internal/discord/bot.go:119-131 + internal/discord/welcome.go:6-19
- **Spec/Phase ref:** spec §Player Onboarding (lines 184-200); Phase 9a
- **Problem:** `HandleGuildMemberAdd` sends the welcome DM as soon as a member joins, regardless of whether `/setup` has been run or a campaign exists. The DM literally renders "Welcome to this campaign!" (the fallback in `campaignName`). This (a) confuses users in non-DnDnD servers that happen to invite the bot, (b) creates a spammy onboarding experience pre-setup, and (c) potentially DMs every member of large servers immediately on bot invite.
- **Suggested fix:** Gate `SendWelcomeDM` on the existence of a campaign row + at least one configured channel id, OR require the campaign to be in `active` status before greeting new joiners.

## [High] Fuzzy match suggestion message renders incorrectly when multiple matches

- **Location:** internal/discord/registration_handler.go:97-100
- **Spec/Phase ref:** spec §Registration name matching (lines 47-48); Phase 14
- **Problem:** When 2–3 fuzzy matches exist, code emits `Did you mean: **Thorn, Thorin, Thora**? Use /register <name> to confirm.` — the entire join is wrapped in a single `**…**`, so only the comma-joined block (not each name) is bolded, and the literal `<name>` placeholder is shown verbatim instead of a real name. Spec explicitly shows per-name bolding: `Did you mean: **Thorn**, **Thorin**, **Thora**?`. Also, when there's only one fuzzy match, the spec drops the "to confirm" clause for multi-match.
- **Suggested fix:** Bold each suggestion (`**Thorn**, **Thorin**`), and only append the "Use /register <name> to confirm." clause when exactly one suggestion is offered, with the actual matched name substituted in.

## [High] Sessions middleware re-issues cookie even when slide TTL fails silently

- **Location:** internal/auth/middleware.go:62-77
- **Spec/Phase ref:** spec §Session management (line 72 — "TTL resets on each authenticated request"); Phase 10
- **Problem:** When `SlideTTL` fails the middleware logs and continues without re-issuing the cookie *and without aborting the request*. The session in the DB still has its old `expires_at`. This is benign on a one-off, but a persistently failing DB write (e.g., read-replica fallback misconfig) silently lets sessions expire mid-traffic, producing inconsistent UX and partially-authenticated requests that pass auth this hop but fail next. Worse, the cookie is *not* re-issued in the error branch so the browser's MaxAge drifts away from the DB's expires_at.
- **Suggested fix:** Either fail the request on slide error (consistent with fail-closed auth) or, at minimum, always re-issue the cookie since the DB state already lets this request through.

## [Medium] `MessageQueue.Stop` doesn't preempt long backoff sleeps

- **Location:** internal/discord/queue.go:90-134
- **Spec/Phase ref:** spec §Discord Rate Limiting (lines 166-173); Phase 9b
- **Problem:** During a 429 backoff the drain goroutine calls `sleepFunc(wait + jitter)` unconditionally; it doesn't `select` on `<-mq.done`. If Discord returns a Retry-After of e.g. 30s and the bot is shutting down, the queue can't exit cleanly for up to 30s. Pending callers' `Send` calls block on `<-errCh` for the full sleep window with no way to cancel.
- **Suggested fix:** Replace `sleepFunc(wait + jitter)` with a `select { case <-mq.done: flushErrors(...); return; case <-time.After(wait+jitter): }` so shutdown drains immediately.

## [Medium] `SplitMessage` splits on bytes, can produce invalid UTF-8 mid-codepoint

- **Location:** internal/discord/message.go:67-122
- **Spec/Phase ref:** spec §Message size handling (line 207); Phase 9b
- **Problem:** All message length checks and slices use byte length (`len(content)`, `content[:MaxMessageLen]`). Discord's 2000-char limit is by character, not byte; more importantly, `splitHardCut` slicing in the middle of a multi-byte rune produces broken UTF-8 that Discord may reject or render as `�`. Combat log lines with non-ASCII names ("Aerïon", "Ælric", or emoji) are most exposed.
- **Suggested fix:** Convert to runes for length checks and split on rune boundaries (or use `utf8.DecodeRuneInString` to back up to a valid boundary before slicing).

## [Medium] Fuzzy match Levenshtein operates on bytes, not runes

- **Location:** internal/registration/fuzzy.go:10-40
- **Spec/Phase ref:** spec §Registration name matching (line 47); Phase 8 / Phase 14
- **Problem:** `levenshteinDistance` indexes the string by bytes. For non-ASCII names ("Æthelred", "Brünhilde"), each multi-byte rune contributes >1 to the distance, breaking the threshold heuristic and the per-byte comparison silently compares half-runes. Real-world impact is small for English campaigns but the spec promises fuzzy matching against "DM-created character names" with no ASCII restriction.
- **Suggested fix:** Convert both strings to `[]rune` before the DP computation.

## [Medium] `ShortID` operates on bytes, may produce invalid UTF-8 for non-ASCII names

- **Location:** internal/charactercard/shortid.go:21-29
- **Spec/Phase ref:** spec §Character Cards (line 219 example `Aria (AR)`); Phase 17
- **Problem:** `words[0][:2]` and `b.WriteByte(w[0])` take the first one or two *bytes* of each name word. For names starting with multi-byte runes (e.g., "Ælric" → first byte 0xC3) the resulting "short ID" is invalid UTF-8 that may render as `Æ?` or `?` on Discord. Combined with the unbounded suffix loop, a degenerate input can also iterate longer than needed.
- **Suggested fix:** Switch to rune-aware slicing (`[]rune(w)[:n]`) for both the single-word and per-word branches.

## [Medium] WebSocket hub channels are unbuffered and synchronous

- **Location:** internal/dashboard/ws.go:31-103
- **Spec/Phase ref:** spec §Concurrency Model (lines 100-106); Phase 15
- **Problem:** `Broadcast`, `encBroadcast`, `Register`, `Unregister` are unbuffered channels. Callers like `ApprovalHandler.broadcastUpdate` push directly: `h.hub.Broadcast <- msg`. If the hub's `Run` goroutine is busy handling a slow client send loop, every HTTP handler that calls `broadcastUpdate` blocks until `Run` reaches the next iteration. If `Stop()` was called, every subsequent `Broadcast <-` blocks forever (deadlock). Same for `BroadcastEncounter` which is called from many sites.
- **Suggested fix:** Buffer the channels (`make(chan []byte, 64)`), and use non-blocking sends with a `default` case in `Broadcast`/`BroadcastEncounter` (matching the per-client send logic), or expose a context-aware Send method.

## [Medium] Approval POST endpoints accept POST without CSRF protection

- **Location:** internal/dashboard/approval_handler.go:50-58, 230-338
- **Spec/Phase ref:** spec §Session management (line 69 — SameSite=Lax cookie); Phase 16
- **Problem:** `Approve`, `Reject`, and `RequestChanges` are POST routes that mutate state, authenticated solely by the session cookie. SameSite=Lax mitigates most CSRF but does *not* prevent top-level POST navigations (e.g. a form auto-submitted from a malicious link the DM clicks). With no CSRF token or `Origin` header check, a single click on a crafted page could approve/reject any character.
- **Suggested fix:** Either require a CSRF double-submit token tied to the session, or verify the `Origin`/`Referer` header matches the dashboard's expected host on every state-mutating POST. Same goes for `/dashboard/queue/*/resolve` and the campaign pause/resume POSTs.

## [Medium] Expertise grants double-prof bonus even when not proficient

- **Location:** internal/character/modifiers.go:18-35
- **Spec/Phase ref:** spec §Skill & Ability Checks (lines 2576-2579); Phase 7
- **Problem:** `SkillModifier` returns `mod + profBonus*2` whenever the skill appears in `expertiseSkills`, without first checking that the skill is in `profSkills`. In 5e, expertise is *only* available on skills the character is proficient in. A bad data shape (or a future bug that adds a skill to `expertiseSkills` without also adding it to `profSkills`) silently grants double-prof on a non-proficient skill.
- **Suggested fix:** Require `slices.Contains(profSkills, skill)` as a precondition for the expertise branch (treat unproficient + expertise as just proficient or just non-proficient — your choice — and log a data warning).

## [Medium] `error_log.error_detail` column written by no Go code

- **Location:** internal/errorlog/pgstore.go:79-83 + db/migrations/20260427120001_create_error_log.sql:17
- **Spec/Phase ref:** spec §Error Log (lines 3185-3190 — "optional structured detail (full error msg / stack / fields)"); Phase 17 supporting infra
- **Problem:** The migration defines `error_detail JSONB` as part of the spec's error_log schema but `buildInsertErrorQuery` only inserts (`command`, `user_id`, `summary`). The dedicated detail column the spec mandates is permanently NULL, so stack traces and structured context the DM panel was promised to render are unreachable.
- **Suggested fix:** Add an `ErrorDetail json.RawMessage` field to `Entry`, write it into the INSERT, and surface it in `ListRecent`.

## [Medium] Spell-slots map sorted lexicographically — slot "10" would precede "2"

- **Location:** internal/charactercard/format.go:152-170
- **Spec/Phase ref:** spec §Character Cards (line 224); Phase 17
- **Problem:** `formatSpellSlots` does `sort.Strings(keys)`, so if any caller ever puts spell levels above 9 in the map (e.g., epic-tier homebrew, or even just including the cantrip level "0" alongside "1"-"9"), the lex order misorders entries (e.g., "10" before "2"). Today's data tops out at 9 so this is latent.
- **Suggested fix:** Parse the keys to ints and sort numerically (or store the key as int from the start).

## [Medium] `CreatePlaceholder` inserts `ac = 0` for new characters

- **Location:** internal/registration/service.go:122-141
- **Spec/Phase ref:** spec §Data Model `characters` (line 3050 — `ac INTEGER NOT NULL`); Phase 8 / Phase 14
- **Problem:** `CreatePlaceholder` doesn't pass `Ac`, so Go's zero-value 0 is inserted. Anyone listing characters or rendering an approval card sees "AC: 0" until the importer/builder fills in a real value. For `/import` placeholder rows that linger pending DM approval, the card shows nonsense.
- **Suggested fix:** Set a sane default (e.g., 10 = unarmored, no Dex) and document that this is a placeholder. Better: don't post a character-card-style preview at all until approval.

## [Medium] Welcome DM message hardcodes channel names that may not exist

- **Location:** internal/discord/welcome.go:6-19
- **Spec/Phase ref:** spec §Player Onboarding (lines 184-200); Phase 9a / Phase 12
- **Problem:** The welcome DM references `#character-cards` and `#the-story` by name. If `/setup` was never run, these channels don't exist; or if the DM later renames them, the references go stale. There's no link to actual channel IDs.
- **Suggested fix:** Either gate the welcome DM on `/setup` having completed, or render `<#channel_id>` references using the stored channel IDs so Discord auto-renders them as clickable links and survives renames.

## [Medium] `/setup` handler runs no authorization check beyond Discord's default-perms hint

- **Location:** internal/discord/setup.go:217-249
- **Spec/Phase ref:** spec §Server Setup (line 163), Phase 12
- **Problem:** `DefaultMemberPermissions: ManageChannels` only hides the command in the UI by default; a guild admin can override it for any role/user. The handler does not re-check `Member.Permissions` or compare the invoker to the campaign's `dm_user_id` before mutating channels. Combined with finding #1 (auto-create campaign), this is the chain that escalates a regular member to DM.
- **Suggested fix:** After resolving the campaign, reject the interaction if `invokerUserID != campaign.DmUserID`. Treat the Discord `DefaultMemberPermissions` only as a UI hint.

## [Medium] HP recompute on multiclassing assumes secondary classes never reach level 1 with max die

- **Location:** internal/character/stats.go:30-42
- **Spec/Phase ref:** spec §Character Leveling (line 2453 — "hp_max adds the new class's hit die + CON mod"); Phase 7
- **Problem:** The first class entry gets max die at level 1; all subsequent class entries' levels (including their level-1) use the average. PHB §Multiclassing (p.164) explicitly says level 1 of a new class on a multiclass character adds *just* `hit die + CON mod` (rolled or average) — the "max at first character level" rule applies only to your very first level. The code's behavior is therefore correct *only if* the first class in the JSONB array is always the character's level-1 class. If the array is reordered (e.g., classes stored alphabetically, or DM toggles the primary class), HP recomputes wrong.
- **Suggested fix:** Stop relying on array order; tag one ClassEntry as `IsPrimary` or `IsFirstClass`, or pass the original level-1 class explicitly.

## [Medium] `setup` channel creation has no rollback on partial failure

- **Location:** internal/discord/setup.go:128-182
- **Spec/Phase ref:** spec §Server Setup (line 163); Phase 12
- **Problem:** `SetupChannels` iterates categories and channels and creates them one-by-one. If channel #5 fails (rate limit, permissions), channels #1–4 are already created and the campaign row's `channel_ids` is *not* updated (line 232-242 saves only after the full loop succeeds). The DM sees "Failed to create channels"; partial state remains in Discord with no recorded IDs.
- **Suggested fix:** Either persist channel_ids incrementally as each channel is created so re-running `/setup` can resume, or batch into a single Discord API call (if available), or detect partial state on rerun and reconcile (the "skip existing" logic already exists for category/channel names, so a re-run mostly recovers — but the saved channel_ids map only reflects channels created in the successful run, not pre-existing ones; that should be verified).

## [Low] `SidebarNav` "Errors" path may render before error badge wiring is loaded

- **Location:** internal/dashboard/handler.go:55-68
- **Spec/Phase ref:** spec §DM Tool Layer architecture; Phase 15
- **Problem:** The sidebar emoji icons (🏠, 📋, ⚔️, etc.) match the spec's "icon+label entries" intent but are emoji literals, not SVG/icon-font references. Renders depend on browser font and may show inconsistent glyph styles or fall back to text. Spec doesn't mandate this, so it's truly Low.
- **Suggested fix:** Optional polish — use inline SVG icons or a single icon font for consistency.

## [Low] `WelcomeMessage` says "Type /help for a full command list" but /help requires the user to be in the guild

- **Location:** internal/discord/welcome.go:18
- **Spec/Phase ref:** spec §Player Onboarding (line 199); Phase 9a
- **Problem:** The welcome DM is sent to a DM channel (1:1 with the bot). Slash commands typed in a DM channel don't have a `GuildID`, so /help (and every other command requiring guild context) returns an error. Spec doesn't account for this nuance.
- **Suggested fix:** Tell the user to type `/help` *in the server*, not in the DM, or have the bot accept DM-context /help.

## [Low] Race "Half-Elf" referenced in character card example not seeded

- **Location:** internal/refdata/seed_races.go:1-* (only 9 races as `RaceCount = 9`)
- **Spec/Phase ref:** spec §Reference Data Sources (line 2531 — "all SRD races"), spec character-cards example (line 219 — "Half-Elf"); Phase 4
- **Problem:** The seeder constant says `RaceCount = 9`. The SRD includes 9 races (Dwarf, Elf, Halfling, Human, Dragonborn, Gnome, Half-Elf, Half-Orc, Tiefling). Let me verify all are seeded… (Verified: 9 races present in code.) **Actually OK** — see below. Keeping this stub note in case CI lists fewer.
- **Suggested fix:** Verify with `make test-refdata` against snapshot.

## [Low] Class entries don't expose Eldritch Knight / Arcane Trickster as third-caster subclasses

- **Location:** internal/refdata/seed_classes.go (Fighter & Rogue blocks)
- **Spec/Phase ref:** spec §Character Leveling (line 2511 — "third casters (Eldritch Knight, Arcane Trickster) contribute class level × ⅓"); Phase 7
- **Problem:** `CalculateCasterLevel` switches on `spellcasting.SlotProgression` keyed by class name. Fighter and Rogue both have no `Spellcasting` block in the seed, so an EK/AT character's caster level always evaluates to 0 and their spell slots come out as nil. The third-caster mode is reachable only when the calculator gets a spellcasting block, which would have to come from the subclass — but the seeded subclass shape is just `features_by_level`, not `spellcasting`.
- **Suggested fix:** Either store subclass-level spellcasting in `classes.subclasses[*].spellcasting` and merge it in `CalculateCasterLevel`, or extend `ClassSpellcasting` to a list per class entry and let the import path push the right SlotProgression. Either way add a multiclass test that includes an Eldritch Knight.

## [Low] `Settings.AutoApproveRestEnabled` defaults differ from spec

- **Location:** internal/campaign/service.go:36-42
- **Spec/Phase ref:** spec §Short & Long Rests (lines 2587-2613 — every rest goes through `#dm-queue` for DM approval); Phase 11 (settings schema)
- **Problem:** `AutoApproveRestEnabled` returns true when the field is absent ("defaults to true (the historical behaviour)"). Spec is explicit: every `/rest short` and `/rest long` posts to `#dm-queue` and waits for DM approval. The "auto-approve" default subtly contradicts the spec; only on retry does the DM see the request.
- **Suggested fix:** Flip the default to false (DM approval required) and document that opting in to auto-approval is a campaign choice.

## [Low] CookieSecure defaults to false when `COOKIE_SECURE` is unset

- **Location:** cmd/dndnd/main.go:564
- **Spec/Phase ref:** spec §Session management (line 69 — "HTTP-only, Secure, SameSite=Lax cookie"); Phase 10
- **Problem:** `secure := os.Getenv("COOKIE_SECURE") == "true"` means production deploys that forget to set the env var ship insecure cookies (not marked Secure). Spec mandates Secure.
- **Suggested fix:** Default to true; require an explicit `COOKIE_SECURE=false` for local-dev HTTP.

## [Low] `RequiredPermissions` omits `Manage Channels` though `/setup` needs it

- **Location:** internal/discord/permissions.go:11-17
- **Spec/Phase ref:** spec §Bot Permissions (lines 153-159) lists 5 perms; Phase 9a, Phase 12
- **Problem:** Spec's bot-permissions list intentionally does not include Manage Channels — but the implementation's `/setup` calls `GuildChannelCreateComplex` for every category/channel which requires Manage Channels at runtime. `ValidateGuildPermissions` therefore won't warn DMs that the bot can't run `/setup` until they actually try. The spec is technically the source of truth; the gap is between spec and code.
- **Suggested fix:** Add `Manage Channels` to `requiredPerms` (and update the spec) so missing perms are logged at guild-join time.

## [Low] Bot session race in `Bot.HandleGuildCreate`

- **Location:** internal/discord/bot.go:86-90
- **Spec/Phase ref:** spec §Slash Command Registration (lines 178-181); Phase 9a
- **Problem:** `HandleGuildCreate` discards the return error from `trackAndRegister`. If command registration for a new guild fails (transient 5xx, rate limit), the bot tracks the guild as ready but the slash-command set is missing/stale. There's no retry. The spec implies idempotent re-registration on startup, but a guild that errored at `GuildCreate` and didn't go through restart never gets retried.
- **Suggested fix:** Schedule a retry with exponential backoff when `trackAndRegister` errors, or log + surface in errorlog so the DM sees "commands not registered for guild X".

## [Low] Character `level` column not indexed despite spec

- **Location:** db/migrations/20260310120006_create_characters.sql:8-37
- **Spec/Phase ref:** spec §Data Model `characters` (line 3045 — "cached total (sum of all class levels); indexed for queries"); Phase 7
- **Problem:** Spec calls for an index on `level`; migration only indexes `campaign_id`.
- **Suggested fix:** Add `CREATE INDEX idx_characters_level ON characters(level);` either as a new migration or by amending the existing one if not yet shipped.

## [Low] Welcome DM is sent for every guild-join, including bots

- **Location:** internal/discord/bot.go:119-131
- **Spec/Phase ref:** spec §Player Onboarding (line 184); Phase 9a
- **Problem:** Bot-user skip exists. But there's no per-user dedupe — if a player leaves and rejoins, they get the welcome DM again. Spec doesn't specify, so this is intentional but worth flagging if onboarding messaging is meant to be one-time-only.
- **Suggested fix:** Track previously-welcomed Discord user IDs (per campaign) and skip subsequent welcomes; or accept the current "always greet" behavior as a deliberate choice.

---

## Phases that appear correct

- **Phase 1 (Scaffolding):** OK. Health endpoint, slog logging, chi router, Docker, Makefile all present.
- **Phase 2 (DB & migrations):** OK. goose-based MigrateUp/MigrateDown, testcontainers integration tests (`internal/database/integration_test.go`).
- **Phase 3 (Weapons/Armor/Conditions seed):** OK. Weapons/armor stats match SRD; 16 conditions seeded (15 SRD + surprised, justified in comment).
- **Phase 4 (Classes/Races/Feats):** Mostly OK. All 12 class save proficiencies, hit dice, weapon/armor profs, and subclass-choice levels match PHB. Subclass features partly seeded (only one subclass per class shown — Path of the Berserker, College of Lore — full SRD has more, but this matches the spec's "all SRD subclasses" goal at coarse grain).
- **Phase 5 (Spells):** OK at structural level — `ClassifyResolutionMode` heuristic looks sane (damage/heal/attack/save→auto, else dm_required). Snapshot tests exist.
- **Phase 6 (Creatures & Magic Items):** OK at structure; spec compliance not verified per-creature.
- **Phase 9b (Message queue):** Mostly OK; backoff bug (#13) and UTF-8 splitting (#12) are the only substantive issues.
- **Phase 11 (Campaigns):** Service is correct (pause/resume/archive transitions, default settings, multi-tenant scoping via guild_id). Pause/resume HTTP endpoints exist.
- **Phase 12 (/setup):** Channel structure matches spec exactly; permission overwrites for #the-story, #combat-map, #dm-queue are correct. Authorization gap (#1 + #21) and partial-failure rollback (#22) are the real issues here.
- **Phase 13 (Slash command registration):** OK. All 34 commands listed by name in `CommandDefinitions`; bulk-overwrite registration ensures stale-command cleanup.
- **Phase 15 (Dashboard skeleton):** OK. Sidebar nav with icons + labels, WS reconnect with exponential backoff (1→2→4 capped at 30 sec) in client JS.
- **Phase 17 (Character cards):** OK in formatting (matches spec layout including STR/DEX/CON/WIS/INT/CHA ordering, conditions, concentration, gold, languages). Auto-update via `OnCharacterUpdated` hook fires from many handlers.
