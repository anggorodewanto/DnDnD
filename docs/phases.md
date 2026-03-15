# DnDnD Implementation Phases

## Phases

- [x] **Phase 1: Project Scaffolding & Build System**
  - Scope: Initialize Go module, directory structure, Dockerfile, fly.toml, Makefile/taskfile, embed.FS stubs for dashboard, Chi router skeleton, slog structured logging, panic recovery middleware, `GET /health` endpoint.
  - Depends on: None
  - Done when: `go build` produces a single binary, health endpoint returns 200, structured JSON logs to stdout, Docker image builds and runs.

- [x] **Phase 2: Database Connection & Migration Framework**
  - Scope: PostgreSQL connection pool setup, goose migration framework integration, initial migration creating the `campaigns` table, session storage table for OAuth. Testcontainers-based test helper for integration tests.
  - Depends on: Phase 1
  - Done when: `goose up` runs migrations, `goose down` rolls back, integration test connects to a throwaway Postgres via testcontainers.

- [x] **Phase 3: Reference Data Schema — Weapons, Armor, Conditions**
  - Scope: Migrations for `weapons`, `armor`, `conditions_ref` tables. Go structs and sqlc queries for CRUD. SRD seeder for weapons (all SRD weapons with properties), armor (all SRD armor with AC formulas), and conditions (15 standard conditions with `mechanical_effects` JSONB). Snapshot tests verifying seeded row counts.
  - Depends on: Phase 2
  - Done when: Seeder populates all SRD weapons, armor, and conditions; snapshot tests pass; sqlc-generated queries compile.

- [x] **Phase 4: Reference Data Schema — Classes, Races, Feats**
  - Scope: Migrations for `classes` (with `features_by_level` JSONB, `hit_die`, `saving_throws`, `spell_ability`), `races`, `feats` tables. SRD seeder. Go structs and sqlc queries. Snapshot tests.
  - Depends on: Phase 2
  - Done when: All SRD classes (with features), races, and feats seeded; snapshot tests pass.

- [x] **Phase 5: Reference Data Schema — Spells**
  - Scope: Migrations for `spells` table (full schema: level, school, components, casting_time, range, area_of_effect, duration, concentration, ritual, higher_levels, material_cost, material_consumed, damage, healing, conditions_applied, teleport, resolution_mode, save_ability, save_effect). SRD seeder (~300 spells) with auto-tagging `resolution_mode` (auto vs dm_required). Go structs and sqlc queries. Snapshot tests.
  - Depends on: Phase 2
  - Done when: All SRD spells seeded with correct `resolution_mode` tags; validation warnings logged for data quality issues; snapshot tests pass.

- [x] **Phase 6: Reference Data Schema — Creatures & Magic Items**
  - Scope: Migrations for `creatures` and `magic_items` tables. SRD seeder for ~325 creature stat blocks and SRD magic items. Go structs and sqlc queries.
  - Depends on: Phase 3
  - Done when: All SRD creatures and magic items seeded; snapshot tests verify counts and spot-check entries (e.g., Goblin AC/HP, +1 Longsword magic_bonus).

- [x] **Phase 7: Characters Table & Core Character Logic**
  - Scope: Migration for `characters` table (full schema: ability_scores, classes JSONB, spell_slots, pact_magic_slots, hit_dice_remaining, feature_uses, proficiencies, inventory, equipped items, ac_formula, gold, attunement_slots, languages). Go domain types. Derived stat calculation: proficiency bonus from total level, HP from class hit dice + CON, AC from equipped armor/formula, spell slots from class/multiclass table, saving throw modifiers, skill modifiers (including expertise, Jack of All Trades).
  - Depends on: Phase 4, Phase 5
  - Done when: Unit tests verify derived stat calculations for single-class and multiclass characters, multiclass spell slot table, AC formulas (standard, unarmored defense), proficiency bonus progression. Integration tests verify JSONB round-tripping (write and re-read classes, proficiencies, inventory, feature_uses JSONB columns with no data loss).

- [x] **Phase 8: Player Characters Table & Registration Logic**
  - Scope: Migration for `player_characters` table (status, dm_feedback, created_via, unique constraints). Service layer for registration: case-insensitive name matching, fuzzy/Levenshtein suggestions, status transitions (pending -> approved/changes_requested/rejected/retired). Character retirement logic.
  - Depends on: Phase 7
  - Done when: Integration tests verify name matching, status transitions, retirement, unique constraints.

- [x] **Phase 9a: Discord Bot Foundation — Core**
  - Scope: Discordgo bot setup: gateway connection, guild tracking, `GuildCreate` handler. Per-guild slash command registration (idempotent, with stale command cleanup). Welcome DM on member join. Bot permission validation. Discord session interface + mock for testing.
  - Depends on: Phase 1
  - Done when: Bot connects to Discord, registers commands per guild, sends welcome DM to new members, permission validation works, mock session available for tests.

- [x] **Phase 9b: Discord Bot Foundation — Message Queue & Rate Limiting**
  - Scope: Message size handling (split at 2000/6000 chars, .txt attachment fallback). Per-channel outbound message queue with rate-limit backoff (429 handling). Queue draining with jitter. Integration tests for message splitting and rate-limit retry.
  - Depends on: Phase 9a
  - Done when: Messages split correctly at size boundaries, .txt fallback works, rate-limit 429 triggers backoff and retry, per-channel queue serializes sends.

- [x] **Phase 10: Discord OAuth2 & Session Management**
  - Scope: Discord OAuth2 flow for DM dashboard and player portal authentication. Server-side sessions in PostgreSQL (HTTP-only cookie, SameSite=Lax). Transparent token refresh. 30-day sliding TTL. Middleware to validate session and extract Discord user ID.
  - Depends on: Phase 2, Phase 9a
  - Done when: OAuth2 login/callback works, session persists across requests, expired tokens auto-refresh, session TTL slides on activity.

- [x] **Phase 11: Campaign CRUD & Multi-Tenant Scoping**
  - Scope: Campaign creation (one per guild), settings JSONB (turn_timeout_hours, etc.), status management (active/paused/archived). All queries scoped by guild_id/campaign_id. Campaign pause/resume with Discord announcements.
  - Depends on: Phase 2, Phase 9a
  - Done when: Campaign created on `/setup`, all queries properly scoped, pause/resume posts to `#the-story`.

- [x] **Phase 12: Discord Channel Structure (`/setup`)**
  - Scope: `/setup` slash command that auto-creates the full channel structure (SYSTEM, NARRATION, COMBAT, REFERENCE categories with channels). Permission overrides (e.g., `#the-story` DM-write-only, `#combat-map` bot-write-only). Skip existing channels. Store channel IDs in campaign settings.
  - Depends on: Phase 9a, Phase 11
  - Done when: `/setup` creates all channels with correct permissions, skips duplicates, stores channel references.

- [x] **Phase 13: Slash Command Registration — Player Commands**
  - Scope: Register all player-facing slash commands with Discord: `/move`, `/fly`, `/attack`, `/cast`, `/bonus`, `/action`, `/shove`, `/interact`, `/done`, `/deathsave`, `/command`, `/reaction`, `/check`, `/save`, `/rest`, `/whisper`, `/status`, `/equip`, `/undo`, `/inventory`, `/use`, `/give`, `/loot`, `/attune`, `/unattune`, `/prepare`, `/retire`, `/register`, `/import`, `/create-character`, `/character`, `/recap`, `/distance`, `/help`. Command routing stubs that return "not yet implemented".
  - Depends on: Phase 9a, Phase 12
  - Done when: All commands appear in Discord's slash command UI with parameter hints; stubs respond with placeholder messages.

- [x] **Phase 14: Player Registration Commands (`/register`, `/import`, `/create-character`)**
  - Scope: Implement `/register` (name matching + fuzzy suggestions), `/import` (stub — accepts URL, creates pending record), `/create-character` (returns one-time portal link). Ephemeral confirmation messages. Status-aware responses for pre-approved commands. DM approval queue notifications to `#dm-queue`.
  - Depends on: Phase 8, Phase 13
  - Done when: `/register` finds characters with fuzzy matching, `/import` creates pending record, all post to `#dm-queue`, status messages shown correctly.

- [x] **Phase 15: DM Dashboard — Skeleton & Campaign Home**
  - Scope: Go templates for dashboard shell (sidebar nav with icon+label entries). Campaign Home view: pending `#dm-queue` items count, Character Approval Queue, active encounters, saved encounters list, quick-action buttons. WebSocket connection setup (nhooyr/websocket) with reconnect + exponential backoff. Svelte SPA stub embedded via embed.FS.
  - Depends on: Phase 10, Phase 11
  - Done when: Dashboard loads with sidebar, Campaign Home shows placeholder data, WebSocket connects and reconnects.

- [x] **Phase 16: Character Approval Queue (Dashboard)**
  - Scope: Dashboard panel showing pending characters from `/import`, `/create-character`, `/register`, and `/retire`. DM can review full sheet, approve, request changes (with message), or reject. On approve: character linked, `#character-cards` entry created, player pinged via Discord DM. On retire: character unlinked, card updated with "Retired" badge.
  - Depends on: Phase 14, Phase 15
  - Done when: Full approval workflow works end-to-end: submission -> DM review -> approve/reject -> player notification -> character card posted.

- [x] **Phase 17: Character Cards (`#character-cards`)**
  - Scope: Bot maintains one auto-updated message per character in `#character-cards`. Format per spec (name, short ID, level, race, class/subclass, HP, AC, speed, ability scores, equipped weapons, spell slots, conditions, concentration, gold, languages). Auto-edits on state change.
  - Depends on: Phase 7, Phase 12
  - Done when: Character card created on approval, auto-updates on HP/equipment/condition/level changes.

- [x] **Phase 18: Dice Rolling Engine**
  - Scope: Core dice rolling: parse dice expressions (NdM+K), roll with advantage/disadvantage, critical hit detection (nat 20), critical failure (nat 1). Modifier stacking. Roll logging with full breakdown. All rolls posted to `#roll-history`.
  - Depends on: Phase 1
  - Done when: Unit tests cover all dice expressions, advantage/disadvantage, crits; roll results include full breakdown for combat log.

- [x] **Phase 19: Maps Table & Map Storage**
  - Scope: Migration for `maps` table (id, campaign_id, name, width/height_squares, tiled_json, background_image_id, tileset_refs). Map size validation (soft limit 100x100, hard limit 200x200). Tiled-compatible JSON storage format. Go structs and sqlc queries.
  - Depends on: Phase 2
  - Done when: Maps can be created/read/updated, size validation enforced, Tiled JSON stored and retrieved.

- [x] **Phase 20: Assets Table & AssetStore Interface**
  - Scope: Migration for `assets` table. Add FK constraint from `maps.background_image_id` to `assets.id`. `AssetStore` Go interface (`Put`, `Get`, `Delete`, `URL`). `LocalAssetStore` implementation (local filesystem at `data/assets/{campaign_id}/{type}/`). UUID filenames. `/api/assets/{id}` endpoint for serving files to dashboard. Fly Volume mount configuration.
  - Depends on: Phase 2
  - Done when: Files can be uploaded, stored, retrieved, and deleted through the interface; assets served via API endpoint.

- [x] **Phase 21a: Map Editor — Grid, Terrain, Walls, Save/Load (Dashboard)**
  - Scope: Svelte map editor: specify grid dimensions, blank grid with default terrain, terrain brush (open ground, difficult terrain, water, lava, pit), wall tool (draw along tile edges). Map save/load via API.
  - Depends on: Phase 15, Phase 19
  - Done when: DM can create maps with terrain and walls, save and reload them.
  - Note: Campaign ID is currently a placeholder in the Svelte frontend. Needs wiring via the dashboard session (OAuth user → campaign lookup). Should be addressed when the first dashboard phase that requires live campaign context is implemented (e.g., Phase 23 Encounter Builder).

- [x] **Phase 21b: Map Editor — Image Import & Opacity (Dashboard)**
  - Scope: Image import as background layer with adjustable opacity slider. Image stored via AssetStore. Background renders beneath terrain layer.
  - Depends on: Phase 21a, Phase 20
  - Done when: DM can import a background image, adjust opacity, and see it beneath the grid.

- [x] **Phase 21c: Map Editor — Lighting, Elevation, Spawn Zones (Dashboard)**
  - Scope: Lighting brush (dim, darkness, magical darkness, fog/obscurement). Elevation painting per tile. Spawn zone marking (player/enemy regions).
  - Depends on: Phase 21a
  - Done when: DM can paint lighting zones, set tile elevations, mark spawn zones; all data persists in map JSON.

- [x] **Phase 21d: Map Editor — Undo/Redo, Region Select, Copy/Paste, Duplicate (Dashboard)**
  - Scope: Undo/redo stack for all editor operations. Rectangular region select tool. Copy/paste selected region. Duplicate entire map.
  - Depends on: Phase 21a
  - Done when: Undo/redo works for all tools, region select+copy+paste works, duplicate map creates independent copy.

- [x] **Phase 22: Map Rendering Engine (Server-Side PNG)**
  - Scope: Go `image/draw` + `gg` map renderer. Tile rendering at 48px (32px for >100x100). Terrain types with colors/patterns. Wall rendering. Grid lines and coordinate labels (A-Z, AA-AZ, etc.). Token rendering with short ID labels. Token health tier indicators (dual-channel: color + icon, colorblind-accessible). Stacked token rendering for altitude. Unified map legend (terrain key + active effects). Per-encounter render queue with debouncing.
  - Depends on: Phase 19
  - Done when: PNG generated from map JSON + combatant positions; tokens show health tiers with accessible indicators; legend renders when needed; render queue debounces rapid updates.

- [x] **Phase 23: Encounter Templates & Encounter Builder (Dashboard)**
  - Scope: Migration for `encounter_templates` table. Dashboard Encounter Builder: name (internal + display), map selection, add creatures from Stat Block Library, set quantities, auto-generate short IDs, drag-drop creature token placement on map. Save/edit/duplicate/delete templates. Saved Encounters list on Campaign Home.
  - Depends on: Phase 6, Phase 15, Phase 21a
  - Done when: DM can create encounter templates with creatures placed on maps, edit/duplicate/delete them, see them listed on Campaign Home.

- [x] **Phase 24: Encounters & Combatants Tables**
  - Scope: Migrations for `encounters`, `combatants`, `turns`, `action_log` tables (full schema from spec). Go domain types for all combat entities. sqlc queries for CRUD. Combatant creation from character + creature reference data.
  - Depends on: Phase 7, Phase 6, Phase 19
  - Done when: Encounters can be created from templates, combatants instantiated with correct stats, turns table ready for use.

- [x] **Phase 25: Initiative System**
  - Scope: Initiative rolling for all combatants. Tiebreaking (higher DEX, then alphabetical). Initiative order assignment. Initiative tracker message in `#initiative-tracker` (auto-updated). Surprise marking (condition-based, auto-skip in round 1). Round counter advancement.
  - Depends on: Phase 24, Phase 18
  - Done when: Unit tests verify tiebreaking, surprise skipping, round advancement; initiative tracker message posted and updated in Discord.

- [x] **Phase 26a: Combat Lifecycle — Start Combat**
  - Scope: "Start Combat" flow: create encounter instance from template, assign PCs (via DM selection or spawn zones), place PC tokens on map, roll initiative, post initiative tracker + map image to Discord, ping first combatant. Encounter status transition: preparing -> active.
  - Depends on: Phase 25, Phase 22, Phase 23
  - Done when: Full start-combat flow works: template -> instantiate -> initiative -> Discord messages -> first player pinged.

- [x] **Phase 26b: Combat Lifecycle — End Combat & Cleanup**
  - Scope: Auto-detect all hostiles at 0 HP -> DM prompt to end. Manual end option from dashboard. Cleanup: clear combat conditions, end concentration, freeze initiative tracker, cancel timers, ammunition recovery prompt, loot pool availability, bot announcement in #combat-log. Clear reaction declarations. Encounter status transition: active -> completed.
  - Depends on: Phase 26a
  - Done when: End-combat detection works, cleanup removes all transient state, Discord messages posted, encounter marked completed.

- [x] **Phase 27: Concurrency — Advisory Locks & Turn Validation**
  - Scope: Per-turn pessimistic lock using PostgreSQL advisory locks keyed on `turn_id`. Lock timeout (5s). Out-of-turn prevention (validate Discord user ID matches active turn's character owner). DM dashboard mutations through same lock. Exceptions for `/reaction`, `/check`, `/save`, `/rest`.
  - Depends on: Phase 24
  - Done when: Integration tests verify lock serialization, rapid command queueing, lock timeout behavior, out-of-turn rejection, DM concurrent access.

- [x] **Phase 28: Turn Resource Tracking**
  - Scope: Turn resource management: movement remaining, action used, bonus action used, free object interaction used, attacks remaining (by class/level), reaction used (per round, resets at creature's turn start). Resource validation on every command. Turn status prompt (at turn start + after every command) showing remaining resources. Spent resources omitted from display.
  - Depends on: Phase 24, Phase 27
  - Done when: Unit tests verify resource tracking, deduction, and display; commands rejected when resources spent.

- [x] **Phase 29: Pathfinding (A*)**
  - Scope: A* pathfinding on tile grid. Edge weights for difficult terrain (x2), prone crawling (x2, stacks to x3 with difficult terrain). Tile occupancy rules (move through allies, not enemies unless size diff >= 2). Wall edge blocking. Diagonal movement at 5ft (no alternating). Diagonal corner-cutting allowed. Path cost calculation.
  - Depends on: Phase 19
  - Done when: Unit tests cover normal paths, difficult terrain, wall blocking, occupancy rules, diagonals, corner-cutting, prone crawling costs.

- [x] **Phase 30: Movement (`/move`)**
  - Scope: `/move` command: destination coordinate parsing (A1 through AA99+), path validation via A*, movement cost deduction, split movement support, ephemeral confirmation prompt with path cost and remaining movement. Moving through occupied tiles (ally pass-through, enemy blocking, size exception). Cannot end turn in another creature's space (block `/done`).
  - Depends on: Phase 28, Phase 29
  - Done when: Integration tests verify valid/invalid moves, split movement, confirmation flow, tile occupancy rules.

- [x] **Phase 31: Altitude & Flying (`/fly`)**
  - Scope: `/fly` command: set altitude (costs movement 1:1). Altitude display on tokens (`AR^30`). 3D Euclidean distance for range checks (rounded to nearest 5ft). Flying tokens don't block ground tiles. Fall damage on prone/lose fly speed (1d6 per 10ft). Stacked token rendering offset for same-tile different-altitude.
  - Depends on: Phase 30, Phase 22
  - Done when: Unit tests verify altitude movement cost, 3D distance calculation, fall damage; map shows stacked tokens correctly.

- [x] **Phase 32: Distance Awareness (`/distance`)**
  - Scope: `/distance` command (ephemeral): distance from self to target, or between two combatants. 3D Euclidean calculation. Passive distance in action feedback (attack/cast log entries include distance). Range rejection messages include actual distance and allowed range.
  - Depends on: Phase 31
  - Done when: `/distance` returns correct distances; attack/cast feedback includes distance; range rejections show both distances.

- [x] **Phase 33: Cover Calculation**
  - Scope: Dynamic cover computation from map geometry (walls, obstacles, creatures). DMG grid variant: corner-of-attacker to corners-of-target line tracing. Half (+2 AC, +2 DEX save), three-quarters (+5, +5), full cover (block targeting). Creature-granted half cover. Integration with attacks (AC bonus) and saves (DEX save bonus for AoE).
  - Depends on: Phase 19, Phase 29
  - Done when: Unit tests verify cover from walls, obstacles, creatures; half/three-quarters/full correctly determined; integration with AC and saves.

- [x] **Phase 34: Basic Attack Resolution (`/attack`)**
  - Scope: `/attack` command: weapon selection (equipped or specified), attack roll (d20 + modifiers), AC comparison, hit/miss determination, damage roll. Critical hits (nat 20, double dice). Auto-crit (paralyzed/unconscious within 5ft). Finesse auto-select (higher of STR/DEX). Range validation (melee reach, ranged normal/long). Cover AC bonus integration. Distance in combat log. Attacks remaining tracking.
  - Depends on: Phase 28, Phase 18, Phase 33
  - Done when: Integration tests verify attack flow: weapon selection, to-hit calculation, damage on hit, crits, range validation, cover, finesse auto-select.

- [x] **Phase 35: Advantage/Disadvantage Auto-Detection**
  - Scope: Auto-detect advantage/disadvantage from: conditions (blinded, invisible, poisoned, prone, restrained, stunned, paralyzed, unconscious, petrified per tables in spec), combat context (reckless attack, ranged with hostile within 5ft, beyond normal range, heavy weapon + small creature). Multiple sources cancel per 5e. Combat log shows reason. DM override from dashboard.
  - Depends on: Phase 34
  - Done when: Unit tests cover all condition-based and context-based adv/disadv sources; cancellation logic; combat log output includes reason.

- [x] **Phase 36: Extra Attack & Two-Weapon Fighting**
  - Scope: Extra Attack: track attacks per action by class/level (Fighter 5=2, 11=3, 20=4). One `/attack` per swing. Report remaining attacks. Unused attacks forfeited on `/done`. Multiclass: use highest `attacks_per_action`. Two-Weapon Fighting: `/bonus offhand` with light weapon validation. Off-hand damage without ability mod (unless fighting style).
  - Depends on: Phase 34
  - Done when: Unit tests verify extra attack counts by class/level, multiclass highest-wins, TWF light weapon validation, off-hand damage modifier rules.

- [x] **Phase 37: Weapon Properties — Versatile, Reach, Heavy, Loading, Thrown, Ammunition, Improvised**
  - Scope: `--twohanded` flag for versatile weapons (off-hand must be free). Reach weapons extend melee to 10ft. Heavy weapons: disadvantage for Small/Tiny. Loading: one attack per action with loading weapons (Crossbow Expert override). Thrown weapons: range validation, weapon removed from hand after throw. Ammunition: auto-deduct from inventory, reject when empty, post-combat half recovery. Improvised weapons: 1d4 bludgeoning, no proficiency (Tavern Brawler override), `--thrown` range 20/60.
  - Depends on: Phase 34
  - Done when: Unit tests verify each weapon property mechanic; integration tests cover ammunition tracking and thrown weapon hand management.

- [x] **Phase 38: Attack Modifier Flags (GWM, Sharpshooter, Reckless)**
  - Scope: `--gwm` (-5 hit, +10 damage, requires heavy melee). `--sharpshooter` (-5 hit, +10 damage, requires ranged). `--reckless` (advantage on melee STR attacks, enemies get advantage, Barbarian only, first attack). Invalid flag errors with explanation.
  - Depends on: Phase 34, Phase 35
  - Done when: Unit tests verify each flag's modifiers, validation (correct weapon type, correct class), and interaction with advantage system.

- [x] **Phase 39: Condition System — Application, Tracking, Auto-Expiration**
  - Scope: Condition CRUD on combatants (JSONB array). Duration tracking (duration_rounds, started_round, source_combatant_id, expires_on). Auto-expiration at start/end of source creature's turn. Indefinite conditions (grappled, prone) removed by specific actions only. Turn-start sequence: check expired -> apply start-of-turn effects -> ping player. Combat log messages for application/removal/expiration.
  - Depends on: Phase 24, Phase 3
  - Done when: Integration tests verify condition application, duration countdown, auto-expiration timing (start vs end of turn), indefinite condition persistence, combat log output.

- [x] **Phase 40: Condition Effects — Saves, Checks, Attacks, Speed, Action Blocking**
  - Scope: Implement all condition effect tables from spec. Saves: paralyzed/stunned/unconscious/petrified auto-fail STR/DEX; restrained disadv on DEX; dodge adv on DEX. Checks: frightened/poisoned disadv; blinded auto-fail sight checks; deafened auto-fail hearing checks. Attacks: all attacker/target modifiers per table. Speed: grappled/restrained = 0; prone stand cost; frightened can't approach source. Action blocking: incapacitated/stunned/paralyzed/unconscious/petrified block actions/reactions, auto-skip turn. Charmed attack restriction.
  - Depends on: Phase 39, Phase 35
  - Done when: Unit tests for every condition effect on saves, checks, attacks, speed, and action blocking; auto-skip of incapacitated turns.

- [x] **Phase 41: Moving While Prone**
  - Scope: When prone combatant uses `/move`: prompt for Stand & Move vs Crawl. Stand: deduct half max speed then normal movement. Crawl: x2 movement cost (stacks with difficult terrain for x3). Confirmation prompt reflects chosen mode. Skip prompt if already stood this turn via `/action stand`.
  - Depends on: Phase 30, Phase 40
  - Done when: Integration tests verify stand-and-move cost, crawl cost, crawl+difficult terrain stacking, prompt skip after prior stand.

- [x] **Phase 42: Damage Processing**
  - Scope: Damage pipeline: resistance (half), immunity (zero), vulnerability (double). Resistance+vulnerability cancel. Immunity trumps all. Petrified resistance to all damage. Temp HP absorption (damage temp HP first, remainder to real HP). Temp HP doesn't stack (keep higher). Temp HP can't be healed. Exhaustion (progressive, levels 1-6, cumulative effects, auto-apply speed/disadv/HP halving/death). Condition immunity check on condition application.
  - Depends on: Phase 39
  - Done when: Unit tests cover all damage type interactions (R, I, V, R+V, I+V), temp HP, exhaustion levels 1-6, condition immunity.

- [x] **Phase 43: Death Saves & Unconsciousness**
  - Scope: Drop to 0 HP: unconscious, prone, break concentration, block commands except `/deathsave`. Instant death check (overflow >= max HP). Death saves: d20, >=10 success, <10 failure. Nat 20 = regain 1 HP. Nat 1 = 2 failures. 3 successes = stabilized. 3 failures = dead. Damage at 0 HP: 1 failure per hit, 2 for crit. Stabilization: Medicine DC 10, Spare the Dying. Healing from 0 HP: reset tallies, still prone. Token states (dying, dead, stable).
  - Depends on: Phase 42, Phase 18
  - Done when: Unit tests cover full death save state machine, instant death, damage at 0 HP, stabilization paths, healing from 0, nat 1/nat 20 edge cases.

- [x] **Phase 44: Feature Effect System — Core Engine**
  - Scope: Data-driven effect processor. Effect types: modify_attack_roll, modify_damage_roll, extra_damage_dice, modify_ac, modify_save, modify_check, modify_speed, grant_resistance, grant_immunity, extra_attack, modify_hp, conditional_advantage, resource_on_hit, reaction_trigger, aura, replace_roll, grant_proficiency, modify_range, dm_resolution. Trigger points: on_attack_roll, on_damage_roll, on_take_damage, on_save, on_check, on_turn_start, on_turn_end, on_rest. Condition filters. Resolution priority (immunities -> R/V -> flat mods -> dice mods -> adv/disadv). Single-pass processor: collect active effects -> filter by conditions -> apply in priority order.
  - Depends on: Phase 7, Phase 39
  - Done when: Unit tests verify each effect type, trigger point matching, condition filtering, priority ordering, single-pass processing with multiple simultaneous effects.

- [x] **Phase 45: Feature Effect System — Class Feature Integration**
  - Scope: Wire Feature Effect System declarations from `classes.features_by_level` and `characters.features` into the combat engine. Sneak Attack (auto-detect finesse/ranged + advantage or ally within 5ft, once per turn, level-scaled dice). Evasion (Rogue 7+: DEX save success = no damage, fail = half). Uncanny Dodge (reaction: halve damage from visible attack). Archery fighting style (+2 ranged). Defense fighting style (+1 AC in armor). Dueling (+2 damage one-handed). Great Weapon Fighting (reroll 1s and 2s on damage). Pack Tactics (creature feature, advantage when ally within 5ft).
  - Depends on: Phase 44, Phase 34
  - Done when: Integration tests verify Sneak Attack auto-detection and damage, Evasion half/zero damage, Uncanny Dodge halving, all fighting styles, Pack Tactics.

- [x] **Phase 46: Rage (Barbarian)**
  - Scope: `/bonus rage`: activate rage, costs bonus action, deduct from feature_uses. Rage effects via Feature Effect System: +2/+3/+4 damage on melee STR attacks, resistance to B/P/S, advantage on STR checks/saves. Duration: 10 rounds, auto-end if no attack and no damage taken (track per-round). End on unconscious, voluntary `/bonus end-rage`. Block `/cast` and drop concentration while raging. Heavy armor restriction. Combat log output.
  - Depends on: Phase 44, Phase 28
  - Done when: Integration tests verify rage activation, damage bonus, resistance, duration tracking, auto-end conditions, spellcasting block, armor restriction.

- [x] **Phase 47: Wild Shape (Druid)**
  - Scope: `/bonus wild-shape [beast]`: validate beast exists in `creatures` table with `type='beast'`, CR limit by Druid level (base and Moon), swim/fly speed level restrictions. Stat swap: snapshot original, overwrite HP/AC/STR/DEX/CON/speed/attacks from beast. Retained: INT/WIS/CHA, proficiencies, features. Spellcasting blocked (except Beast Spells 18+). Concentration maintained. HP in beast form, overflow damage on revert. `/bonus revert`: voluntary revert. Auto-revert at 0 HP. Token change. Combat log output.
  - Depends on: Phase 6, Phase 44, Phase 28
  - Done when: Integration tests verify transformation, stat swap, CR validation, HP overflow, auto-revert, spellcasting block, concentration maintenance.

- [x] **Phase 48a: Monk — Martial Arts & Unarmored Defense/Movement**
  - Scope: Martial Arts: DEX/STR auto-select for monk weapons and unarmed strikes, martial arts die scaling (1d4/1d6/1d8/1d10 by level), `/bonus martial-arts` free unarmed strike after Attack action. Unarmored Defense (ac_formula: 10 + DEX + WIS when no armor). Unarmored Movement (+speed scaling when no armor/shield).
  - Depends on: Phase 44, Phase 36
  - Done when: Integration tests verify martial arts die scaling, DEX/STR auto-select, bonus unarmed strike, unarmored defense AC, unarmored movement speed bonus.

- [x] **Phase 48b: Monk — Ki Abilities**
  - Scope: Ki point tracking (feature_uses, recharge short rest). `/bonus flurry-of-blows` (1 ki, 2 unarmed strikes, replaces martial-arts bonus). `/bonus patient-defense` (1 ki, dodge as bonus action). `/bonus step-of-the-wind` (1 ki, dash or disengage as bonus action). Stunning Strike (auto-prompt on melee hit, 1 ki, CON save or stunned for 1 round). Ki validation (sufficient points, correct class).
  - Depends on: Phase 48a
  - Done when: Integration tests verify each ki ability's cost and effect, stunning strike prompt and save, ki deduction and validation, short rest recharge.

- [x] **Phase 49: Bardic Inspiration**
  - Scope: `/bonus bardic-inspiration [target]`: grant die to ally. Uses tracked in feature_uses (CHA mod uses, recharge long/short by level). Die scaling (d6/d8/d10/d12 by Bard level). Usage prompt on attack/check/save (ephemeral, player sees roll before deciding). 30s timeout on prompt. 10-minute real-time expiration. Turn status visibility. Combat log output.
  - Depends on: Phase 44, Phase 28
  - Done when: Integration tests verify granting, die scaling, usage prompt flow, expiration, turn status display.

- [x] **Phase 50: Channel Divinity (Cleric/Paladin)**
  - Scope: `/action channel-divinity [option]`: costs action, tracked in feature_uses (recharge short). Turn Undead: WIS save for undead within 30ft, Turned condition. Destroy Undead (Cleric 5+): instant destroy below CR threshold. Subclass options: auto-resolved (Preserve Life, Sacred Weapon, Vow of Enmity) or DM-resolved (route to #dm-queue). Usage tracking and validation.
  - Depends on: Phase 44, Phase 28
  - Done when: Integration tests verify Turn Undead saves, Destroy Undead CR threshold, Preserve Life HP distribution, DM-queue routing for narrative options.

- [x] **Phase 51: Divine Smite (Paladin)**
  - Scope: After melee weapon hit: ephemeral prompt with available slot levels. Smite damage (2d8 + 1d8 per slot above 1st, max 5d8). +1d8 vs undead/fiend. Crit doubles smite dice. 30s timeout. Spell slot deduction. Combat log output. Driven by `resource_on_hit` effect type.
  - Depends on: Phase 44, Phase 34
  - Done when: Integration tests verify prompt appearance, slot selection, damage calculation, undead bonus, crit doubling, timeout behavior.

- [x] **Phase 52: Lay on Hands (Paladin)**
  - Scope: `/action lay-on-hands [target] [hp]`: costs action. Healing pool (5 x Paladin level, recharge long rest). Adjacency validation. Undead/construct rejection. Self-targeting shorthand. Cure disease/poison (5 HP per cure, `--cure-poison`/`--cure-disease` flags). Combat log.
  - Depends on: Phase 44, Phase 28
  - Done when: Integration tests verify healing, pool tracking, adjacency check, disease/poison cure, self-targeting.

- [x] **Phase 53: Action Surge (Fighter)**
  - Scope: `/action surge`: grants additional action. Tracked in feature_uses (1 use short rest, 2 at level 17). Resets action_used and attacks_remaining. Prevents double surge per turn (action_surged flag). Not an extra bonus action or reaction. Combat log.
  - Depends on: Phase 44, Phase 28
  - Done when: Integration tests verify surge grants action, extra attack sequence, double-surge prevention, recharge on short rest.

- [x] **Phase 54: Standard Actions — Dash, Disengage, Dodge, Help, Hide, Stand, Drop Prone, Escape**
  - Scope: `/action dash` (add speed to remaining movement). `/action disengage` (suppress OA). `/action dodge` (disadv on attacks against, adv on DEX saves, 1-round condition). `/action help [ally] [target]` (grant advantage, adjacency check). `/action hide` (Stealth vs passive Perception, set is_visible). `/action stand` (half movement cost, remove prone). `/action drop-prone` (no cost, apply prone). `/action escape` (contested check vs grappler, remove grappled). Rogue Cunning Action: `/bonus cunning-action dash` and `/bonus cunning-action disengage` (costs bonus action instead of action, Rogue class validation). All standard actions cost action except stand/drop-prone.
  - Depends on: Phase 28, Phase 40, Phase 18
  - Done when: Integration tests verify each standard action's resource cost, mechanical effect, and combat log output. Rogue cunning-action dash and disengage verified as bonus actions with class check.

- [x] **Phase 55: Opportunity Attacks**
  - Scope: Auto-detect when creature leaves hostile's melee reach during `/move`. Check Disengage suppression (has_disengaged). Queue-and-continue: movement completes, OA trigger recorded at exit tile. Prompt hostile (player: ping in #your-turn; DM: dashboard prompt). Uses reaction. End-of-round forfeiture. If OA kills target, DM handles retroactive correction. Interaction with reach weapons (10ft reach).
  - Depends on: Phase 30, Phase 34, Phase 54
  - Done when: Integration tests verify OA trigger detection, disengage suppression, prompt delivery, reaction consumption, reach weapon interaction, forfeiture.

- [x] **Phase 56: Grapple, Shove & Dragging**
  - Scope: `/action grapple [target]`: requires free hand, target no more than 1 size larger. Contested Athletics vs Athletics/Acrobatics. On success: grappled condition, speed 0. `/shove [target] --prone/--push`: contested check, knock prone or push 5ft. Push destination validation (unoccupied tile). Size restriction. Dragging: when grappler uses `/move`, detect grappled targets, prompt "Drag" vs "Release & Move". Drag: movement costs double (half speed), all grappled creatures move to grappler's destination. Release: remove grapple condition, move at normal speed. Multiple grappled creatures do not further multiply cost (always x2). Combat log output.
  - Depends on: Phase 54, Phase 40
  - Done when: Integration tests verify grapple (free hand, size, contested check, condition), shove (prone/push, destination check), dragging (prompt, x2 cost, release option, multiple targets), combat log.

- [x] **Phase 57: Stealth & Hiding**
  - Scope: `/action hide`: Stealth check vs all hostiles' passive Perception. Success: is_visible=false, token hidden from player map. Failure: remains visible. While hidden: attacks against have disadvantage, first attack from hidden has advantage, attacking reveals. Rogues: `/bonus cunning-action hide`. Passive Perception calculation (10 + Perception mod + proficiency). Equipment stealth disadvantage (armor with stealth_disadv).
  - Depends on: Phase 54, Phase 35
  - Done when: Integration tests verify hide success/failure, hidden attack advantage, auto-reveal on attack, Rogue bonus action hide, armor stealth penalty.

- [x] **Phase 58: Spell Casting — Basic (`/cast`)**
  - Scope: `/cast` command: parse spell + target (coordinate or combatant ID). Spell slot validation and deduction. Range enforcement (touch=5ft, self, ranged). Spell save DC calculation (8 + prof + ability mod). Spell attack rolls. Bonus action spell auto-detection from casting_time. Bonus action spell restriction (both directions). Concentration tracking (one at a time, new drops old). Combat log output. `#roll-history` posting.
  - Depends on: Phase 5, Phase 28, Phase 18, Phase 33
  - Done when: Integration tests verify slot deduction, range check, save DC, spell attacks, bonus action detection, bonus action restriction, concentration tracking.

- [x] **Phase 59: Spell Casting — AoE & Saves**
  - Scope: AoE targeting: sphere (radius from point), cone (from caster toward target), line (length + width from caster). Calculate affected creatures by shape overlap. Spell saves: ping affected players to roll `/save`. Enemy saves rolled by DM from dashboard. Apply damage/effects once all saves resolved. Half damage on save (if applicable). AoE + cover interaction (DEX save bonus).
  - Depends on: Phase 58, Phase 33
  - Done when: Integration tests verify each AoE shape, creature overlap calculation, save flow (player ping, DM roll), half-damage-on-save, cover DEX bonus.

- [x] **Phase 60: Spell Casting — Upcasting, Ritual, Cantrip Scaling**
  - Scope: `--slot N` for upcasting: validate slot level >= spell level, parse `higher_levels` for scaled damage/healing. Default to lowest available slot. `--ritual` for ritual spells: no slot cost, only out of combat, class feature check. Cantrip damage scaling by character level (2 dice at 5, 3 at 11, 4 at 17).
  - Depends on: Phase 58
  - Done when: Unit tests verify upcast damage scaling, ritual validation, cantrip scaling at each breakpoint.

- [x] **Phase 61: Spell Casting — Concentration Checks & Breaking**
  - Scope: Damage triggers concentration check: ping caster for `/save con` (DC = max(10, half damage)). Failure breaks concentration. Incapacitation (stunned, paralyzed, unconscious, petrified) auto-breaks concentration. Silence zone breaks concentration on V/S spells. `/cast` blocked in Silence zone for V/S spells. Active spell effect removal on concentration break.
  - Depends on: Phase 58, Phase 39
  - Done when: Integration tests verify concentration save trigger, auto-break on incapacitation, silence zone interaction, spell effect cleanup.

- [x] **Phase 62: Spell Casting — Teleportation Spells**
  - Scope: Spells with `teleport` JSONB: bypass path validation, no movement cost/difficult terrain/OA. Validate: destination unoccupied, within range, line of sight if required, companion within range. SRD teleportation spells (Misty Step, Thunder Step, Dimension Door, Far Step, etc.). Higher-level teleports route to #dm-queue.
  - Depends on: Phase 58
  - Done when: Integration tests verify teleportation bypasses pathfinding, destination validation, companion range check, DM queue routing for narrative teleports.

- [x] **Phase 63: Spell Casting — Material Components**
  - Scope: Focus/component pouch assumed for non-costly components. Costly components: check inventory for required item. If missing but can afford: gold fallback prompt ("Buy & Cast" or "Cancel"). If `material_consumed=true`: remove item. Neither component nor gold: reject with message.
  - Depends on: Phase 58
  - Done when: Integration tests verify free component bypass, costly component inventory check, gold fallback, consumed component removal, rejection message.

- [x] **Phase 64: Pact Magic (Warlock)**
  - Scope: Separate `pact_magic_slots` pool. Fewer slots, all same level, recharge on short rest. `/cast` draws from pact slots first. Multiclass: `--spell-slot` flag to draw from regular pool. Upcast must be <= pact slot level. Both pools displayed separately.
  - Depends on: Phase 58
  - Done when: Integration tests verify pact slot usage, short rest recharge, multiclass pool selection, upcast validation.

- [x] **Phase 65: Spell Preparation (`/prepare`)**
  - Scope: `/prepare` for prepared casters (Cleric, Druid, Paladin): ephemeral message with current prepared spells, full class spell list, remaining slots. Select/deselect via Discord select menus (paginated by level). Confirm/cancel. Validate count <= max prepared. Domain/Oath/Circle spells always prepared, shown separately. Post long-rest reminder.
  - Depends on: Phase 58
  - Done when: Integration tests verify prepare flow, count validation, always-prepared spells excluded from count, long-rest reminder.

- [x] **Phase 66a: Metamagic — Sorcery Points & Framework (Sorcerer)**
  - Scope: Sorcery points tracking (feature_uses, recharge long rest). `/bonus font-of-magic` for slot/point conversion (both directions, level-based point costs). Metamagic framework: validate `/cast` metamagic flags, enforce one metamagic per spell (except Empowered combos), deduct sorcery points.
  - Depends on: Phase 58, Phase 44
  - Done when: Integration tests verify sorcery point tracking, Font of Magic conversion, metamagic flag validation, one-per-spell enforcement, point deduction.

- [x] **Phase 66b: Metamagic — Individual Options (Sorcerer)**
  - Scope: Implement all 8 SRD Metamagic options as `/cast` flags: `--careful` (chosen creatures auto-succeed on AoE save), `--distant` (double range or touch->30ft), `--empowered` (reroll damage dice up to CHA mod), `--extended` (double duration), `--heightened` (disadvantage on first save), `--quickened` (cast as bonus action), `--subtle` (no V/S components), `--twinned [target]` (single-target spell hits second target, costs spell level in points). Per-option validation rules.
  - Depends on: Phase 66a
  - Done when: Integration tests verify each metamagic option's effect, cost, and validation; Empowered+other combo works; Twinned target validation.

- [x] **Phase 67: Spell Effect Zones (Encounter Zones)**
  - Scope: Migration for `encounter_zones` table. Create zones on persistent AoE spell cast (Fog Cloud, Spirit Guardians, Wall of Fire, Darkness, etc.). Anchor modes (static vs combatant). Auto-removal on concentration break, duration expiry, encounter end, DM manual. Enter/leave triggers (damage zones: once per creature per turn). Zone rendering on map (colored overlays, origin markers). Map legend integration.
  - Depends on: Phase 58, Phase 22, Phase 24
  - Done when: Integration tests verify zone creation, movement triggers, concentration cleanup, duration expiry, map rendering with overlays.

- [x] **Phase 68: Dynamic Fog of War**
  - Scope: Symmetric shadowcasting (Albert Ford's algorithm) from each player token against walls/obstacles. Union of all party tokens' visible cells. Three visibility states: visible, explored (dim), unexplored (black). Enemy tokens hidden in fog, greyed in dim. Vision sources: base vision, darkvision, blindsight, truesight, Devil's Sight. Light sources (torches, Light cantrip). Rendering layers (base map, fog overlay, tokens, grid). DM sees all.
  - Depends on: Phase 22, Phase 19
  - Done when: Unit tests verify shadowcasting correctness (symmetry), vision union, darkvision interaction with darkness. Map renders correctly with fog layers.

- [x] **Phase 69: Obscurement & Lighting Zones**
  - Scope: Zone types: dim light, darkness, magical darkness, fog/heavy obscurement, light obscurement. Combat effects: heavily obscured = effectively blinded; lightly obscured = perception disadvantage + hide available. Darkvision interaction (darkness->dim, dim->bright). Magical darkness ignores darkvision. Auto-applied combat modifiers on attacks and checks. Combat log shows lighting modifier.
  - Depends on: Phase 67, Phase 68, Phase 35
  - Done when: Integration tests verify each zone type's combat effects, darkvision interactions, auto-applied modifiers, combat log output.

- [x] **Phase 70: Reactions System**
  - Scope: `/reaction` command: freeform declaration, stored in `reaction_declarations` table. Multiple active declarations. Persist until used/cancelled/encounter ends. One reaction per round (tracked on turns.reaction_used, resets at creature's turn start). `/reaction cancel [desc]` and `/reaction cancel-all`. DM resolution flow: Active Reactions Panel shows all declarations, DM triggers, resolves, marks spent.
  - Depends on: Phase 24, Phase 27
  - Done when: Integration tests verify declaration CRUD, one-per-round enforcement, DM resolution flow, cancellation, encounter-end cleanup.

- [x] **Phase 71: Readied Actions**
  - Scope: `/action ready [description]`: costs action. Fires using reaction when trigger occurs (DM resolves). Expires at start of creature's next turn (with expiry notice). Readied spells: slot expended on ready, concentration held, lost if concentration breaks. Expiry notice in turn-start prompt. `/status` shows active readied actions.
  - Depends on: Phase 70, Phase 54
  - Done when: Integration tests verify action cost, expiry timing, readied spell slot/concentration, expiry notice, status display.

- [x] **Phase 72: Counterspell Resolution**
  - Scope: Two-step Counterspell flow: DM triggers from Active Reactions Panel -> player prompted with slot level buttons + Pass (spell name revealed, cast level hidden). If slot >= enemy level: auto-counter. If slot < enemy level: spellcasting ability check (DC 10 + enemy spell level). Async timing: enemy turn continues, success removes effects retroactively. Timeout = forfeited.
  - Depends on: Phase 70, Phase 58
  - Done when: Integration tests verify full Counterspell flow: prompt, auto-counter, ability check, timeout forfeiture, retroactive removal.

- [x] **Phase 73: Freeform Actions & `/action cancel`**
  - Scope: `/action [freeform text]`: post to `#dm-queue`, DM resolves from dashboard. `/action cancel`: withdraw pending action (if DM hasn't resolved), strikethrough in #dm-queue, ephemeral confirmation. Rejection if already resolved or nothing pending.
  - Depends on: Phase 13, Phase 15
  - Done when: Integration tests verify freeform posting, cancellation flow, already-resolved rejection, no-pending rejection.

- [x] **Phase 74: Free Object Interaction (`/interact`)**
  - Scope: First `/interact` per turn is free (sets free_interact_used). Second costs action (rejected if action spent). Auto-resolvable interactions (draw/sheathe weapon, open unlocked door) resolve immediately. Others route to #dm-queue.
  - Depends on: Phase 28
  - Done when: Integration tests verify free first interaction, action cost for second, auto-resolve vs DM queue routing.

- [x] **Phase 75a: Equipment Management — `/equip` Command & Hand Management**
  - Scope: `/equip [weapon]` (main hand), `--offhand`, `--armor`. `/equip none` to unequip. In-combat weapon equip costs free object interaction. Shield: donning/doffing costs action in combat, instant out of combat. Armor: blocked in combat. Two-handed weapon validation (off-hand must be free). Grapple free-hand check. Somatic component free-hand check.
  - Depends on: Phase 7, Phase 74
  - Done when: Integration tests verify all equip flows, combat restrictions, hand management, shield action cost, armor combat block.

- [x] **Phase 75b: Equipment Management — AC Recalculation & Enforcement**
  - Scope: AC recalculation engine triggered on any equipment change: armor-based AC, unarmored defense formulas (Barbarian 10+DEX+CON, Monk 10+DEX+WIS), shield bonus, modify_ac effects layered on top. Equipment enforcement: heavy armor STR requirement (speed -10ft penalty), stealth disadvantage from armor with stealth_disadv flag.
  - Depends on: Phase 75a, Phase 44
  - Done when: Integration tests verify AC recalculation for all armor types and unarmored formulas, STR penalty application, stealth disadvantage enforcement.

- [x] **Phase 76a: Turn Timeout — Timer Infrastructure & Nudges**
  - Scope: Turn timeout (24h default, DM-configurable 1-72h). Polling goroutine (30s interval) checks deadlines from DB. Escalation: 50% nudge message, 75% tactical summary (HP, AC, conditions, resources, adjacent enemies). DM manual overrides: skip now, extend, pause combat.
  - Depends on: Phase 28, Phase 43
  - Done when: Integration tests verify timer polling, 50% nudge delivery, 75% tactical summary content, DM override commands.

- [x] **Phase 76b: Turn Timeout — 100% Resolution & Prolonged Absence**
  - Scope: 100% timeout: DM decision prompt (Wait/Roll for Player/Auto-Resolve). Wait: extend 50%, once per timeout. Auto-Resolve: Dodge + no movement (normal turn), auto-roll pending saves/death saves, forfeit unanswered prompts, decline on-hit decisions. DM 1-hour auto-resolve fallback. Prolonged absence (3 consecutive auto-resolves -> flagged absent). Stale state scan on startup.
  - Depends on: Phase 76a
  - Done when: Integration tests verify DM decision prompt, Wait extension, auto-resolve behavior (Dodge applied, saves rolled), prolonged absence flagging, startup stale state recovery.

- [x] **Phase 77: Player Turn Flow — Turn Start & Done**
  - Scope: Turn start: expire conditions, apply start-of-turn effects, ping player with turn status (personal impact summary of events since last turn). `/done`: warn if unused resources (ephemeral confirmation prompt with buttons), end turn, advance initiative, ping next player. Map regeneration on turn end. DM can end any turn. Auto-skip for incapacitated combatants.
  - Depends on: Phase 28, Phase 39, Phase 22
  - Done when: Integration tests verify turn start sequence, impact summary, `/done` confirmation prompt, initiative advancement, map regen, incapacitated skip.

- [x] **Phase 78a: Enemy/NPC Turns — Dashboard Turn Builder**
  - Scope: DM dashboard structured multi-step turn builder: movement step (shortest path suggestion via A*), action step (pre-filled from stat block: single attack, multiattack, special abilities with recharge), bonus action step (if available), review & adjust (roll fudging, reorder, remove), confirm & post. Pending reactions surfaced during enemy turn. Combat log output.
  - Depends on: Phase 26a, Phase 29, Phase 6, Phase 70
  - Done when: DM can run full enemy turns from dashboard with smart defaults; reactions surfaced; combat log posted to Discord.

- [x] **Phase 78c: Enemy/NPC Turns — Bonus Action Parsing**
  - Scope: Parse bonus actions from creature stat block abilities text. Add structured `bonus_actions` field to creature data model. Turn builder generates bonus action steps from parsed data. Dashboard UI shows bonus action step in turn builder flow.
  - Depends on: Phase 78a, Phase 6
  - Done when: Creatures with bonus actions (e.g., Goblin's Nimble Escape) generate bonus action steps in the turn builder; tests verify parsing and step generation.

- [x] **Phase 78b: Enemy/NPC Turns — Legendary & Lair Actions**
  - Scope: Legendary actions: mini-turns between other combatants' turns, budget tracking (3/round typical, configurable per creature), cost per action (1-3), reset at creature's turn start, DM picks from stat block legendary action list. Lair actions: fire at initiative count 20 (losing ties), no consecutive repeats of same lair action, DM selects from lair action list. Both integrated into turn queue display.
  - Depends on: Phase 78a
  - Done when: Integration tests verify legendary action budget, cost deduction, reset timing, lair action initiative 20 trigger, no-repeat enforcement, turn queue display.

- [x] **Phase 79: Summoned Creatures & Companions**
  - Scope: Summoning flow: spell creates combatant entries from reference stat blocks, assigns short IDs, adds to initiative/map. `/command [creature-id] [action] [target?]`: validate summoner ownership, stat block actions, per-creature turn resources. Initiative placement (own turn vs caster's turn). Ping summoning player for creature turns. `/command [id] done`. `/command [id] dismiss`. Death: remove from encounter. Concentration-based dismissal.
  - Depends on: Phase 26a, Phase 58, Phase 34
  - Done when: Integration tests verify summon creation, `/command` routing, turn flow, dismissal, concentration link, death removal.

- [x] **Phase 80: Combat Recap (`/recap`)**
  - Scope: `/recap` (no args): all combat log entries since player's last turn (ephemeral). `/recap N`: last N rounds. Grouped by round and turn. Direct replay of log entries. Usable during or after combat (until encounter archived). If no active combat, show final rounds of most recent completed encounter.
  - Depends on: Phase 24
  - Done when: Integration tests verify recap content, round grouping, turn filtering, post-combat access.

- [x] **Phase 81: Skill & Ability Checks (`/check`)**
  - Scope: `/check [skill/ability]`: d20 + ability mod + proficiency (if proficient). Expertise (double proficiency). Jack of All Trades (half proficiency on non-proficient). `--adv`/`--disadv` flags. Targeted checks (`/check medicine AR`): adjacency validation, action cost in combat. DM-prompted checks. Passive checks (10 + mod). Group checks (DM triggers, half must succeed). Contested checks. Auto-applied condition modifiers. Post to `#roll-history`, DM narrates outcome.
  - Depends on: Phase 7, Phase 18, Phase 40
  - Done when: Integration tests verify check calculations, expertise, JoAT, targeted checks, group checks, contested checks, condition modifiers.

- [x] **Phase 82: Saving Throws (`/save`)**
  - Scope: `/save [ability]`: d20 + ability mod + proficiency (if proficient). Auto-include all modifiers: Paladin Aura of Protection, Bless, condition effects (exhaustion disadv, dodge adv on DEX), magic item bonuses. Full breakdown in combat log. DM-prompted saves. Auto-fail for paralyzed/stunned/unconscious/petrified on STR/DEX. Condition effects on saves (restrained disadv DEX, dodge adv DEX).
  - Depends on: Phase 18, Phase 44, Phase 40
  - Done when: Integration tests verify save calculations, all modifier sources, auto-fail conditions, full breakdown output.

- [ ] **Phase 83a: Short & Long Rests — Individual Flow**
  - Scope: `/rest short`: DM approval, hit dice spending prompt (single-class simple buttons, multiclass grouped by die type). Feature recharge (short rest features). Warlock pact slot restore. `/rest long`: DM approval, full HP restore, all slots restore, all features reset, hit dice restore (half total level), death save tally reset, prepared caster reminder.
  - Depends on: Phase 7, Phase 15
  - Done when: Integration tests verify hit dice spending, feature recharge, full long rest benefits, prepared caster reminder.

- [ ] **Phase 83b: Short & Long Rests — Party Rest & Interruption**
  - Scope: DM-initiated Party Rest from dashboard: select characters, choose short/long, batch processing. Rest interruption: DM cancels mid-rest, 1-hour threshold for long rest (if >= 1 hour elapsed, long rest benefits still apply per 5e rules). Interrupted rest notifications to players.
  - Depends on: Phase 83a
  - Done when: Integration tests verify party rest batch flow, interruption logic, 1-hour threshold, player notifications.

- [ ] **Phase 84: Inventory Management**
  - Scope: `/inventory` (ephemeral: items grouped by type, quantities, equipped status, attunement, gold). `/use [item]` (consume + apply effect: healing potions auto-resolved, others to #dm-queue; costs action in combat). `/give [item] [target]` (adjacency check, both inventories updated, free interaction cost in combat). Gold tracking (integer gold field). DM inventory management from dashboard.
  - Depends on: Phase 7, Phase 28
  - Done when: Integration tests verify inventory display, consumable use, give flow, gold tracking, DM management.

- [ ] **Phase 85: Looting System**
  - Scope: Post-combat loot pool: DM populates via dashboard (auto-populate from defeated creatures + Item Picker). Gold auto-summed. Narrative descriptions on items. `/loot`: players claim items via Discord buttons. Single-claim enforcement. "Split Gold" from dashboard. Unclaimed items persist. Posted to `#combat-log`.
  - Depends on: Phase 26b, Phase 84
  - Done when: Integration tests verify loot pool creation, player claiming, single-claim enforcement, gold splitting.

- [ ] **Phase 86: Item Picker (Dashboard Component)**
  - Scope: Shared dashboard component for item selection: search across SRD + homebrew, category filters, creature inventory source (for loot context), narrative description field, custom entry (freeform name/desc/quantity/gold), price override.
  - Depends on: Phase 15, Phase 3, Phase 6
  - Done when: Item Picker renders in dashboard, search/filter works, custom entries added, used in loot pool and shop contexts.

- [ ] **Phase 87: Shops & Merchants**
  - Scope: DM creates named shop templates from dashboard using Item Picker. Set prices. Save as reusable templates. "Post to #the-story" sends formatted item list. Impromptu shopping handled through narration + dashboard gold/inventory adjustments.
  - Depends on: Phase 86
  - Done when: DM can create shop, post to Discord, player purchases handled via DM dashboard adjustments.

- [ ] **Phase 88a: Magic Items — Tracking, Bonuses & Passive Effects**
  - Scope: Magic item tracking in inventory (is_magic, magic_bonus, magic_properties, requires_attunement, rarity). Bonus weapons/armor: auto-apply +N to attack/damage/AC via Feature Effect System. Passive effects (Cloak of Protection, Ring of Resistance, etc.) via Feature Effect System vocabulary. `/inventory` display with rarity and attunement status.
  - Depends on: Phase 44, Phase 84
  - Done when: Integration tests verify magic bonus application to attacks/damage/AC, passive effect registration, inventory display with rarity.

- [ ] **Phase 88b: Magic Items — Active Abilities, Attunement & Identification**
  - Scope: Active abilities (Wand of Fireballs: charges, spell casting from item). Recharge at dawn (roll recharge_dice, destroy_on_zero). Charge tracking and deduction. `/attune` (short rest, max 3, class/alignment restriction check). `/unattune`. Unattuned item warning (effects suppressed). Identifying (identified flag, `/cast identify` or short rest study). Unidentified items show generic description.
  - Depends on: Phase 88a
  - Done when: Integration tests verify active ability charges, recharge rolls, attunement limits and restrictions, identification flow, unattuned suppression.

- [ ] **Phase 89: Character Leveling**
  - Scope: Milestone only (no XP). DM edits class level in dashboard. Auto-recalculate: total level, HP, proficiency bonus, spell slots (multiclass table), attacks per action. Subclass selection at threshold. ASI/Feat selection: interactive prompt in Discord (buttons for +2/+1+1/Feat), select menu for scores or feats (with prerequisite check), DM approval via #dm-queue. Feat effects applied via Feature Effect System. Level-up notifications (public + private detail).
  - Depends on: Phase 7, Phase 4, Phase 44
  - Done when: Integration tests verify level-up stat recalculation, ASI/feat prompt flow, DM approval, feat effect application, notifications.

- [ ] **Phase 90: D&D Beyond Import**
  - Scope: `/import <ddb-url>`: fetch from DDB character API, parse DDB JSON into internal format (ability scores, features, equipment, spells, HP, AC). Ephemeral preview. DM approval queue. Structural + advisory validation (warnings for rule issues). Re-sync for updates (diff + DM review). Exponential backoff for rate limiting.
  - Depends on: Phase 7, Phase 14
  - Done when: Integration tests verify import parsing, validation warnings, preview, approval flow, re-sync diff.

- [ ] **Phase 91a: Player Portal — Auth & Scaffold**
  - Scope: Player portal web app scaffold: Discord OAuth2 authentication (reuse Phase 10 OAuth flow), session validation, one-time link generation from `/create-character` (24h expiry, single-use token), link validation and landing page. Portal shell layout (header, navigation, responsive).
  - Depends on: Phase 10
  - Done when: OAuth2 login works for player portal, one-time links generated and validated, expired/used links rejected, portal shell renders.

- [ ] **Phase 91b: Player Portal — Character Builder Form & Submission**
  - Scope: Multi-step character builder form: basics (name, race, background), class/subclass selection, ability scores (point-buy calculator with remaining points display), skills/proficiencies, equipment (from class defaults + background), spells (for casters, filtered by class), review page with all derived stats auto-calculated. Submit to DM approval queue. Form state preserved across steps.
  - Depends on: Phase 91a, Phase 7, Phase 4, Phase 5
  - Done when: Player can create a character through the full multi-step form, derived stats calculated correctly, submission appears in DM approval queue.

- [ ] **Phase 92: Player Portal — Character Sheet View**
  - Scope: Read-only web character sheet: ability scores/modifiers, skills, languages, features with descriptions, spell list, inventory, all mechanical state. Accessible via `/character` command (ephemeral embed + link). Same data as character cards but full detail.
  - Depends on: Phase 91b
  - Done when: Character sheet renders with full detail, accessible via link from `/character`.

- [ ] **Phase 93a: DM Manual Character Creation — Basics Through Ability Scores (Dashboard)**
  - Scope: Dashboard character creation wizard first half: name, race, background, class/subclass selection, ability score entry (manual input or point-buy). DM-created characters are pre-approved (skip approval queue). Derived stats preview (HP, AC, proficiency bonus, saves, skills).
  - Depends on: Phase 15, Phase 7
  - Done when: DM can create character through ability scores step, derived stats preview correct, character saved as pre-approved.

- [ ] **Phase 93b: DM Manual Character Creation — Equipment, Spells & Features (Dashboard)**
  - Scope: Dashboard character creation wizard second half: equipment selection (from class/background defaults + manual add), spell selection (for casters, filtered by class/level), features auto-populated from SRD class data by level. Class/subclass/feat interactions mechanically applied. Final review and save.
  - Depends on: Phase 93a
  - Done when: DM can complete full character creation with equipment, spells, and auto-populated features; all interactions correct.

- [ ] **Phase 94a: DM Dashboard — Combat Manager: Map & Token Display**
  - Scope: Combat Workspace left panel (~60%): map + token rendering from map JSON (separate from server-side PNG — client-side Svelte rendering). Click token -> HP & Condition Tracker panel (apply damage, healing, add/remove conditions). Multi-encounter tabs with badges for pending #dm-queue items. Encounter Overview bar (round, active turn, combatant count).
  - Depends on: Phase 15, Phase 26a
  - Done when: DM can view map with tokens rendered from JSON, click tokens to modify HP/conditions, switch between encounter tabs.

- [ ] **Phase 94b: DM Dashboard — Combat Manager: Movement & Interaction**
  - Scope: Drag-and-drop token movement on grid with snap-to-tile. Auto-calculate distance on drag (shown as overlay). Range circles on token select. Movement validation against walls/obstacles. Distance measurement tool (click two points). Token context menu (damage, heal, conditions, remove from encounter).
  - Depends on: Phase 94a
  - Done when: DM can drag tokens with distance display, range circles shown, movement validated, distance measurement works.

- [ ] **Phase 95: DM Dashboard — Turn Queue & Action Resolver**
  - Scope: Turn Queue: shows initiative order, current turn highlighted, "End Turn" button. Action Resolver: chronological list of #dm-queue items, expand inline for resolution controls (text field for outcome, buttons for damage/conditions/movement). Items marked resolved with outcome summary.
  - Depends on: Phase 94a
  - Done when: DM can view initiative order, end turns, resolve queue items with mechanical effects applied.

- [ ] **Phase 96: DM Dashboard — Active Reactions Panel**
  - Scope: Always-visible panel showing all active `/reaction` declarations grouped by combatant. Status: active/used/dormant. Highlight matching declarations during enemy turn resolution. Click to resolve or dismiss. Consumed reactions greyed until turn reset.
  - Depends on: Phase 70, Phase 94a
  - Done when: Panel shows all declarations, highlights during enemy turns, DM can resolve/dismiss, reaction status tracked.

- [ ] **Phase 97a: DM Dashboard — Action Log Viewer**
  - Scope: Action Log viewer panel: display action_log entries with filter by type/character/turn/round, sort chronologically, before/after state diff rendering for each entry. Expandable entries showing full detail.
  - Depends on: Phase 24, Phase 94a
  - Done when: Action log viewer renders entries, filters work correctly, diff view shows before/after state changes.

- [ ] **Phase 97b: DM Dashboard — Undo & Manual Corrections**
  - Scope: Undo Last Action: revert most recent mutation from action_log before_state (current turn only). Manual State Override: edit HP, position, conditions, spell slots, initiative order directly. All overrides go through per-turn lock. Discord corrections posted to #combat-log (never edit/delete originals).
  - Depends on: Phase 97a, Phase 27
  - Done when: DM can undo last action, manually override any value, corrections posted to Discord, overrides respect turn lock.

- [ ] **Phase 98: DM Dashboard — Stat Block Library**
  - Scope: Browseable library of all SRD creature stat blocks + homebrew. Search and filter. Reusable across encounters. Used in Encounter Builder for creature selection.
  - Depends on: Phase 6, Phase 15
  - Done when: DM can browse, search, and select stat blocks; homebrew entries visible alongside SRD.

- [ ] **Phase 99: DM Dashboard — Homebrew Content**
  - Scope: DM creates custom: monsters (full stat block editor), spells, weapons, items, races, class features. Campaign-scoped, stored with `homebrew=true`. Used alongside SRD data in all contexts.
  - Depends on: Phase 98
  - Done when: DM can create/edit homebrew entries for all reference types, entries appear in search/selection alongside SRD.

- [ ] **Phase 100a: DM Dashboard — Narration Editor**
  - Scope: Rich narration editor: Discord-flavored markdown preview, "read-aloud" boxed-text format, image attachments (stored in Asset Library). Preview panel. Post history. "Post to #the-story" button.
  - Depends on: Phase 15, Phase 20
  - Done when: DM can compose narration with markdown, attach images, preview, and post to #the-story.

- [ ] **Phase 100b: DM Dashboard — Narration Template System**
  - Scope: Narration templates: save narrations as reusable templates with placeholder tokens (e.g., `{player_name}`, `{location}`), template categories. Template library: browse, search, edit, duplicate, delete. Apply template to narration editor with placeholder substitution.
  - Depends on: Phase 100a
  - Done when: DM can save/load/edit/delete templates, placeholders substituted on apply, template library searchable.

- [ ] **Phase 101: DM Dashboard — Character Overview & Message Player**
  - Scope: Read-only view of all player character sheets. Party Languages summary. Message Player: DM-initiated Discord DMs via bot, logged in dashboard per-player. Accessible from Character Overview or sidebar.
  - Depends on: Phase 15, Phase 7
  - Done when: DM can view all characters, see party languages, send private messages to players via bot.

- [ ] **Phase 102: DM Dashboard — Responsive Mobile-Lite View**
  - Scope: Simplified mobile view (sidebar collapses to bottom tab bar). Mobile features: DM Queue, Initiative/Turn Queue (read-only), Narrate, Character Approval Queue, Message Player, Quick Actions (end turn, pause/resume). Desktop-only features show redirect message on mobile.
  - Depends on: Phase 94a, Phase 95
  - Done when: Mobile view renders correctly, all mobile features functional, desktop-only features show redirect.

- [ ] **Phase 103: WebSocket State Sync**
  - Scope: Server-authoritative push-only WebSocket. Full state snapshot per encounter on every push. Client auto-reconnect with exponential backoff (1s-30s cap). Optimistic UI: read-only areas update immediately, active form inputs preserved with indicator. No delta/event-log protocol.
  - Depends on: Phase 15, Phase 24
  - Done when: Dashboard receives live state updates, reconnects automatically, form inputs not clobbered, full snapshot on every push.

- [ ] **Phase 104: Bot Crash Recovery**
  - Scope: Startup recovery sequence: connect PostgreSQL, scan for stale state (overdue turns with no escalation sent), connect Discord, re-register commands, resume timer polling. In-flight commands: PostgreSQL auto-rolls back uncommitted transactions. Turn timers derived from DB fields, not in-memory.
  - Depends on: Phase 76a
  - Done when: Bot restarts cleanly, stale turns processed in deadline order, no timer state lost.

- [ ] **Phase 105: Simultaneous Encounters**
  - Scope: Multiple active encounters sharing Discord channels. Encounter display name vs internal name (DM can set vague player-facing name). Message labeling with encounter name + round. Commands routed by combatant membership. Per-turn locks scoped per encounter. DM manages via tabbed Combat Workspace. Character limited to one active encounter.
  - Depends on: Phase 26a, Phase 94a
  - Done when: Two encounters run simultaneously with correct message labeling, independent turn orders, command routing, and DM tab switching.

- [ ] **Phase 106a: DM Notification System — Core Infrastructure & Initial Events (`#dm-queue`)**
  - Scope: `#dm-queue` structured message framework: player name, context summary, "Resolve ->" link to dashboard. DM-only channel visibility. Resolved items show checkmark + outcome. Initial event types: freeform actions (+ cancel), reaction declarations, rest requests, skill check narration, consumable without effect. Spec lines 2825-2870.
  - Depends on: Phase 12, Phase 15
  - Done when: Framework posts structured messages for initial event types, resolve links open dashboard, resolved items marked with checkmark.

- [ ] **Phase 106b: DM Notification System — Remaining Events & Whisper Replies**
  - Scope: Remaining event types: enemy turn ready, narrative teleport, player whispers. Whisper replies: DM replies from dashboard, delivered as Discord DMs to the player. Each whisper is standalone (no threaded view). Spec lines 2870-2905.
  - Depends on: Phase 106a
  - Done when: All remaining event types post correct structured messages, whisper replies delivered as DMs, all event types covered.

- [ ] **Phase 107: `/help` Command System**
  - Scope: `/help` (general command list). `/help [command]` (detailed usage with examples, flags, tips). Class-specific help: `/help rogue`, `/help cleric`, `/help paladin`, `/help ki`, `/help metamagic`, `/help attack`, `/help action`. Context-specific tips (remaining attacks, available slots). All ephemeral. Spec lines 2907-2940.
  - Depends on: Phase 13
  - Done when: `/help` returns command list, per-command help shows full usage with examples, class-specific help renders correctly.

- [ ] **Phase 108: `/status` Command**
  - Scope: `/status` (ephemeral): active conditions with remaining duration, concentration spell, temp HP, exhaustion level, active reaction declarations, Bardic Inspiration (if held), readied actions, ki points (Monk), sorcery points (Sorcerer), rage state (Barbarian), wild shape state (Druid).
  - Depends on: Phase 39, Phase 70
  - Done when: `/status` shows all active state for the character, formatted per spec.

- [ ] **Phase 109: `/whisper` Command**
  - Scope: `/whisper [message]`: ephemeral to player, posted to #dm-queue as "Whisper" event. DM resolves from dashboard, reply sent as Discord DM. Each whisper is standalone (no threaded view).
  - Depends on: Phase 106a
  - Done when: Whisper posts to #dm-queue, DM can reply, reply delivered as DM, standalone queue items.

- [ ] **Phase 110: Exploration Mode (Map-Based & Theater-of-Mind)**
  - Scope: Map-based exploration: DM loads map without encounter/initiative. Players use `/move`, `/check`, `/action` without turn order or action economy. All players act freely. Walls enforced via pathfinding, but no speed limit on movement. If combat breaks out, DM starts encounter on current map. Theater-of-mind: DM narrates in #the-story, players respond in #in-character, checks as needed.
  - Depends on: Phase 30, Phase 81
  - Done when: Exploration mode works without initiative, commands function without turn tracking, wall validation enforced but movement unlimited, transition to combat preserves map state.

- [ ] **Phase 111: Open5e Integration (Extended Content)**
  - Scope: Fetch from Open5e API (monsters, spells). On-demand fetch + local cache. DM enables/disables third-party sources per campaign (settings JSONB). Merged with SRD data in queries.
  - Depends on: Phase 6, Phase 5, Phase 11
  - Done when: DM can enable Open5e sources, creatures/spells fetched and cached, appear in stat block library and spell lists.

- [ ] **Phase 112: Error Handling & Observability**
  - Scope: Player error messages: friendly ephemeral "something went wrong" on internal errors. DM dashboard error notification badge (24h count), error log panel (timestamp, command, player, error summary). Errors stored in action_log with type='error'. Structured logging with contextual fields. Health endpoint subsystem checks.
  - Depends on: Phase 1, Phase 15
  - Done when: Players see friendly errors, DM sees error count badge, error log panel works, health endpoint reports degraded state.

- [ ] **Phase 113: Invisible Condition**
  - Scope: Invisible condition (from spells like Invisibility, Greater Invisibility). Advantage on own attacks, disadvantage on attacks against. Blocks "see the target" spells. AoE still affects. Distinction from is_visible (stealth). Interaction: invisible but not hidden (enemies know square). Both active simultaneously. Breaking Invisibility on attack/cast (non-Greater only, auto-remove from source tracking). Greater Invisibility persists. Spec lines 1565-1600.
  - Depends on: Phase 39, Phase 57
  - Done when: Integration tests verify invisible mechanics, breaking conditions, Greater vs standard, interaction with stealth.

- [ ] **Phase 114: Surprise**
  - Scope: DM marks combatants as surprised during encounter setup. Surprised condition (1 round, auto-skip, no Dodge). Remove surprised at end of skipped turn (reactions available after). Interaction with initiative (surprised still roll, position matters for reaction timing). Combat log output. Spec lines 1175-1200.
  - Depends on: Phase 25, Phase 39
  - Done when: Integration tests verify surprise skip, condition removal timing, reaction availability after turn ends.

- [ ] **Phase 115: Campaign Pause**
  - Scope: DM sets campaign to `paused` from dashboard. Announcement in #the-story. Commands remain functional. Timers continue. Resume: announcement + re-ping current turn player.
  - Depends on: Phase 11
  - Done when: Pause posts announcement to #the-story, resume posts announcement to #the-story and re-pings the current turn player, commands not blocked during pause, timers unaffected.

- [ ] **Phase 116: Tiled Import (Map Import)**
  - Scope: Import .tmj files. Three-tier validation: hard rejection (infinite maps, non-orthogonal, too large), supported (tile layers, object layers, custom properties), skipped with warnings (animations, image layers, parallax, group layers flattened, etc.). Import summary shows stripped features.
  - Depends on: Phase 19
  - Done when: Valid Tiled maps import successfully, rejected maps show clear errors, skipped features listed in summary.

- [ ] **Phase 117: Testing Infrastructure & Coverage**
  - Scope: Three-tier test pyramid setup: unit tests (pure logic, table-driven), integration tests (testcontainers PostgreSQL, command pipelines, multi-step flows), Discord interaction tests (mock session, channel routing, message formatting, rate-limit batching, message splitting). Test fixtures (seeded campaigns, characters, encounters). Target 90% coverage. CI pipeline running full seed + tests.
  - Depends on: Phase 2
  - Done when: Test infrastructure in place, CI green, coverage report generated, fixture helpers available.

## Coverage Map

| Spec Section | Covered by Phase(s) |
|---|---|
| Overview | Skipped |
| Core Architecture > Mental Model | Skipped |
| Core Architecture > Guiding Rule | Skipped |
| Authentication & Authorization | Phase 8, 10, 14, 16 |
| Concurrency Model | Phase 27, 103, 104 |
| Discord Server Structure | Phase 12 |
| Bot Permissions | Phase 9a |
| Server Setup | Phase 12 |
| Discord Rate Limiting | Phase 9b |
| Slash Command Registration | Phase 9a, 13 |
| Player Onboarding | Phase 9a, 14 |
| Character Cards | Phase 17 |
| Combatant Targeting | Phase 24, 25, 22 |
| Grid Movement | Phase 30 |
| Altitude & Elevation | Phase 31 |
| Multiple Floors & Z-Levels | Narrative only (DM uses object placement or narration — no special tile type); no dedicated phase needed |
| Structured Commands | Phase 13 |
| Distance Awareness | Phase 32 |
| Attack Mechanics | Phase 34, 35, 36, 37, 38 |
| Channel Divinity | Phase 50 |
| Lay on Hands | Phase 52 |
| Equipment Management | Phase 75a, 75b |
| Spell Casting Details | Phase 58, 59, 60, 61, 62, 63, 64, 65, 66a, 66b |
| Reactions | Phase 70, 71, 72 |
| Freeform Actions | Phase 73 |
| Standard Actions | Phase 54, 53 |
| Cunning Action (Dash & Disengage) | Phase 54 |
| Free Object Interaction | Phase 74 |
| Condition Effects | Phase 40 |
| Duration Tracking & Auto-Expiration | Phase 39 |
| Damage Processing | Phase 42 |
| Cover | Phase 33 |
| Pathfinding | Phase 29 |
| Difficult Terrain | Phase 29 |
| Opportunity Attacks | Phase 55 |
| Grapple & Shove | Phase 56 |
| Dragging | Phase 56 |
| Stealth & Hiding | Phase 57 |
| Equipment Enforcement | Phase 75b |
| Feature Effect System | Phase 44, 45 |
| Combat Turn Flow > Overview | Phase 26a, 26b, 77 |
| Surprise | Phase 114 |
| Initiative Tiebreaking | Phase 25 |
| Simultaneous Encounters | Phase 105 |
| Player Turns | Phase 28, 77 |
| Combat Log Output Reference | Phase 34, 35, 40, 43, 54, 56, 57 |
| Enemy / NPC Turns | Phase 78a, 78b |
| Legendary Actions | Phase 78b |
| Lair Actions | Phase 78b |
| Summoned Creatures & Companions | Phase 79 |
| Turn Timeout & AFK Handling | Phase 76a, 76b |
| Combat Recap | Phase 80 |
| Death Saves & Unconsciousness | Phase 43 |
| Example: A Full Player Turn | Phase 77 |
| Map Rendering | Phase 22 |
| Dynamic Fog of War | Phase 68 |
| Map Creation & Authoring | Phase 21a, 21b, 21c, 21d |
| Map Size Limits | Phase 19 |
| Asset Storage | Phase 20 |
| Internal Character Format | Phase 7 |
| Player Portal (Web) | Phase 91a, 91b, 92 |
| Manual Character Creation | Phase 93a, 93b |
| D&D Beyond Import | Phase 90 |
| Character Leveling | Phase 89 |
| SRD Content | Phase 3, 4, 5, 6 |
| Extended Content (Open5e) | Phase 111 |
| Homebrew Content | Phase 99 |
| Skill & Ability Checks | Phase 81 |
| Short & Long Rests | Phase 83a, 83b |
| Inventory Management | Phase 84 |
| Item Picker | Phase 86 |
| Shops & Merchants | Phase 87 |
| Magic Items | Phase 88a, 88b |
| Exploration, Social & Travel | Phase 110 |
| DM Notification System | Phase 106a, 106b |
| DM Dashboard > Layout & Navigation | Phase 15, 94a |
| Responsive & Mobile | Phase 102 |
| DM Dashboard > Features | Phase 94a, 94b, 95, 96, 97a, 97b, 98, 100a, 100b, 101 |
| Encounter Builder | Phase 23 |
| Undo & Corrections | Phase 97b |
| Action Log | Phase 97a |
| Encounter End | Phase 26b |
| Campaign Pause | Phase 115 |
| Tech Stack | Phase 1, 2, 9a, 15, 22 |
| Deployment Target | Phase 1 |
| Monitoring & Observability | Phase 112 |
| Testing Strategy | Phase 117 |
| Data Model > Campaign & Player Tables | Phase 2, 7, 8 |
| Data Model > Encounter & Combat Tables | Phase 24 |
| Data Model > Reaction Declarations | Phase 70 |
| Data Model > Encounter Zones | Phase 67 |
| Data Model > Maps | Phase 19 |
| Data Model > Assets | Phase 20 |
| Data Model > Reference Data | Phase 3, 4, 5, 6 |
| Data Model > Key Design Decisions | Phase 7, 24, 44 |
| Risks & Mitigations | Phase 9a, 9b, 27, 76a, 76b |
| MVP Scope | All phases cover MVP items |

## Skipped Sections

- **Overview** (lines 3-7): Purely informational summary, no implementable work.
- **Core Architecture > Mental Model** (lines 11-17): Architectural philosophy, no implementable work (informs all phases).
- **Core Architecture > Guiding Rule** (lines 19-20): Design principle, no implementable work (informs all phases).
- **Example: A Full Player Turn** (lines 2128-2172): Illustrative example, no additional implementation beyond referenced phases.
- **Risks & Mitigations** (lines 3407-3419): Informational risk register; mitigations are covered by their respective implementation phases.
- **Key Design Decisions** (lines 3387-3404): Design rationale, no standalone implementation (informs schema and architecture decisions in other phases).

## User Clarifications

1. **Floor transitions (Q1):** Purely narrative. No special tile type or marker tool. DM uses object placement with custom properties or describes transitions in narration.
2. **Exploration mode movement (Q2):** Walls enforced, no speed limit. Pathfinding validates walls/obstacles but movement is unlimited per command.
3. **Wild Shape beast filtering (Q3):** Filter by `type='beast'` is sufficient. No explicit tagging needed.
4. **Testing infrastructure (Q4):** One scaffolding phase. Each implementation phase adds its own tests.

## Notes

- Phases 1-12 form the foundational infrastructure (DB, bot, auth, channels) that nearly everything depends on. These should be implemented first in sequence.
- Phases 3-6 (reference data) can be parallelized since they are independent tables (Phase 4 now depends only on Phase 2, not Phase 3).
- Phases 18 (dice), 29 (pathfinding), 33 (cover), 44 (feature effect system) are pure-logic phases with no I/O -- ideal for TDD and can be developed in parallel with infrastructure.
- The spec explicitly notes "Phase 1 (MVP)" for the map editor (line 2312) -- Phase 21a covers this MVP scope; tileset support is deferred to Phase 116.
- Class-specific features (Phases 46-53) can largely be parallelized since each is independent once the Feature Effect System (Phase 44) is in place.
- Dashboard phases (94a-102) depend on the combat lifecycle being functional but can be incrementally built.
- Phase 117 (testing infrastructure) should be started alongside Phase 2 and maintained throughout.
- Split phases use letter suffixes (e.g., 9a/9b, 21a/21b/21c/21d) to avoid renumbering all 117+ phases while maintaining clear ordering.
