# Open Questions — DM UX & Technical Implementation

Gaps and ambiguities identified from a review of `dnd-async-discord-spec.md`, focused on DM user experience and technical implementation.

---

## DM Dashboard UX

- [x] **1. Dashboard layout & navigation** — No wireframes, mockups, or information architecture described. How are the panels (Combat Manager, Action Resolver, Reactions Panel, Character Overview, Map Editor, etc.) organized? Tabs, sidebar nav, collapsible panels? What's the default view when the DM opens the dashboard?

- [x] **2. Dashboard mobile/responsive support** — DMs may need to resolve actions from a phone (e.g., approve a rest request while away from their desk). Is the dashboard responsive? Are any views mobile-optimized, or is it desktop-only?

- [x] **3. Multi-encounter dashboard UX** — The spec says the DM manages simultaneous encounters from "its own panel." Are these side-by-side panels, tabs, or a switcher? How does the DM see cross-encounter context (e.g., total party HP across both encounters)?

- [x] **4. Encounter setup workflow** — How does the DM create and start an encounter? The spec mentions "DM clicks Start Combat" but doesn't detail the setup flow: selecting creatures from the stat block library, placing tokens on spawn zones, setting surprise flags, naming the encounter. Is there an encounter preparation mode where DMs can pre-build encounters before starting them?

- [x] **5. Enemy turn smart defaults — complexity ceiling** — Smart defaults suggest "shortest path toward nearest hostile" and "primary attack." What about creatures with multiattack, legendary actions, lair actions, or complex abilities (e.g., a dragon with breath weapon recharge, tail attack, and legendary actions)? How are legendary/lair actions surfaced in the DM's turn flow?

- [x] **6. DM queue overload management** — In a busy round with multiple freeform actions, reaction declarations, whispers, and enemy turns queued, how does the DM prioritize? Is `#dm-queue` sorted chronologically or by urgency? Can items be filtered, pinned, or bulk-resolved?

- [x] **7. DM queue resolution UX from dashboard** — Each `#dm-queue` notification has a "Resolve →" link. What does the resolution interface look like? Is it a modal, inline expansion, or a separate page? Can the DM apply mechanical effects (damage, conditions, movement) directly from the resolution view?

- [x] **8. Lighting/obscurement zone management** — DM places zones from the dashboard, but how? A painting tool on the map? A form with coordinates? Can zones be resized, moved, or removed mid-combat? How does the DM visualize zone boundaries while editing vs. what players see?

- [x] **9. Shop/merchant workflow** — The spec mentions "DM creates a shop via the dashboard, posts available items to `#the-story`, and transfers purchased items/deducts gold through the dashboard." No details on the shop creation UI. Is it a reusable merchant template? Can players browse items interactively, or does the DM manually handle each purchase?

- [x] **10. DM-to-player communication** — The DM can narrate in `#the-story` and resolve whispers, but how does the DM send ad-hoc private messages to individual players from the dashboard (e.g., "you notice something the others don't")? Is there a direct message feature beyond whisper replies?

- [x] **11. Loot pool management** — After combat, the DM populates a loot pool. Can the DM drag items from defeated creature inventories, or must they manually select each item? How is gold from multiple creatures aggregated? Can the DM add narrative descriptions to loot items?

- [x] **12. Dashboard keyboard shortcuts & power-user features** — For DMs running complex encounters, are there keyboard shortcuts for common actions (advance turn, apply damage, open the map)? Searchable command palette?

- [x] **13. Action log depth & undo limits** — "Undo Last Action" walks back mutations using `before_state`. How far back can the DM undo? Is there a cap on action log retention? Can the DM undo across turn boundaries (e.g., undo an action from a previous turn)?

- [x] **14. Encounter end workflow** — What happens when the DM ends combat? Is there a confirmation step? Does the system auto-detect "all hostiles defeated" and prompt, or must the DM always manually end? What cleanup occurs (conditions removed? initiative tracker cleared? map state preserved?)?

- [x] **15. DM override audit trail** — Manual state overrides post corrections to `#combat-log`. Does the dashboard also maintain an internal audit log the DM can review? Can the DM see a history of all overrides for a given encounter or character?

- [x] **16. Map editor usability** — Undo/redo in the map editor? Copy/paste regions? Can the DM duplicate an existing map as a starting point for a new one? What about map templates (tavern, dungeon room, forest clearing)?

- [x] **17. Pre-combat preparation** — Can the DM pre-configure encounters (creature selection, placement, lighting zones) and save them as drafts before starting combat? Or must everything happen in real-time?

---

## Player-Facing DM Experience

- [x] **18. Group rest coordination** — `/rest short` and `/rest long` are per-player. Do all players need to individually request a rest? Is there a DM-initiated "party rest" that applies to everyone simultaneously?

- [x] **19. DM narration tools** — The DM narrates in `#the-story` but from Discord, not the dashboard. Should the dashboard have a "Narrate" panel that posts to `#the-story` (rich text, attached images, formatted boxed text)? Or is Discord sufficient?

- [x] **20. Encounter name visibility to players** — Simultaneous encounter messages are labeled with the encounter name. Can the DM control what name players see (to avoid spoilers like "Boss Fight" or "Ambush")?

- [x] **21. DM view of player `/whisper` history** — Whispers post to `#dm-queue`. Can the DM see the full whisper conversation history with a player, or only individual messages? Is there a thread/conversation view?

- [ ] **22. Rest interruption UX** — When the DM cancels a rest mid-progress, how does the DM specify "how much time has passed" to determine whether short rest benefits apply for interrupted long rests? Is it a simple toggle or does the DM enter elapsed time?

---

## Technical Implementation

- [ ] **23. WebSocket reconnection & state sync** — What happens when the dashboard loses its WebSocket connection? Auto-reconnect with exponential backoff? Does the dashboard request a full state snapshot on reconnect, or does it rely on catching up from missed events?

- [ ] **24. Map rendering performance** — PNG generation for every state change could be expensive on large maps (e.g., 50x50 grid with fog of war, spell overlays, 20+ tokens). What's the expected rendering time? Is there a caching layer (e.g., only re-render changed tiles)? Are map images generated synchronously (blocking the command response) or asynchronously?

- [ ] **25. Shadowcasting algorithm** — The spec says fog of war uses "shadowcasting" but doesn't specify which algorithm. Recursive shadowcasting? Symmetric shadowcasting (Milazzo)? Performance characteristics matter for large maps with complex geometry.

- [ ] **26. Pathfinding algorithm** — Movement validation checks path cost including difficult terrain and obstacles. What pathfinding algorithm is used? A* on the grid? Dijkstra? Is the path shown to the player before confirmation, or just the cost? What about diagonal movement through wall corners (wall-hugging)?

- [ ] **27. Discord API rate limiting** — The bot posts to multiple channels on every action (combat-log, roll-history, initiative-tracker, combat-map). Discord rate limits are per-channel (5 messages/5 seconds). How does the bot handle bursts (e.g., AoE spell affecting 6 creatures, each requiring a save prompt)? Message queuing? Batching?

- [ ] **28. Discord message size limits** — Combat log entries and turn prompts could exceed Discord's 2000-character limit (e.g., a Fireball hitting 8 creatures with saves, damage, and conditions). What's the truncation/splitting strategy? The spec mentions text file attachments for "very large output" — what's the threshold?

- [ ] **29. Slash command registration** — Are commands registered globally (takes up to 1 hour to propagate) or per-guild (instant)? How are command updates deployed without downtime?

- [ ] **30. Advisory lock timeout & deadlock handling** — Per-turn advisory locks serialize mutations. What happens if a lock is held for too long (e.g., a slow database query)? Is there a lock timeout? What about DM dashboard + player command hitting different locks that could deadlock?

- [ ] **31. Bot crash recovery** — If the bot process crashes and restarts, how does it recover state? Reconnect to Discord gateway, re-register commands, pick up where it left off? What about in-flight commands that were processing when the crash occurred? Are turn timers persisted in the database or in-memory?

- [ ] **32. Database migrations** — No migration strategy mentioned. How are schema changes applied? goose, atlas, golang-migrate? How are JSONB schema changes handled (e.g., adding a new field to `conditions`)?

- [ ] **33. Image/asset storage** — Map background images, token images, and generated PNGs are referenced but storage isn't specified. Local filesystem? Object storage (S3/MinIO)? Embedded in the database? How are generated map PNGs served to Discord (upload every time vs. URL)?

- [ ] **34. SRD data seeding validation** — The 5e-database JSON is seeded into PostgreSQL. How is the mapping validated? Are there automated tests that the seeded data matches expected schema? What about data quality issues in the source dataset?

- [ ] **35. Discord OAuth2 session management** — DM dashboard uses Discord OAuth2. How are sessions managed? JWT? Server-side sessions? Token refresh strategy? Session expiry during long DM sessions?

- [ ] **36. Spell effect zone lifecycle** — Spell overlays are rendered on the map. How are zones created in the database when a spell is cast? How are moving zones (Spirit Guardians anchored to a creature) updated on each movement? How are zones cleaned up when concentration breaks or duration expires?

- [ ] **37. Concurrent map image generation** — If multiple commands in different encounters trigger map regeneration simultaneously, can the image renderer handle concurrency? Is there a queue, or are renders truly parallel?

- [ ] **38. Character import validation depth** — D&D Beyond imports go through a parser. How deeply is the imported data validated against 5e rules? (e.g., illegal multiclass combos, ability scores exceeding 20 without magic items, invalid spell selections). Is validation the DM's responsibility, or does the system flag issues?

- [ ] **39. Horizontal scaling** — The spec describes a single Go binary. Can multiple instances run behind a load balancer? WebSocket affinity? Shared database state is fine, but what about in-memory state (lock coordination, active timers)?

- [ ] **40. Monitoring & observability** — No mention of logging, metrics, alerting, or health checks. What's the observability strategy? Structured logging? Prometheus metrics? How does the DM know if the bot is healthy? How are errors surfaced (silent failure vs. user-visible error)?

- [ ] **41. Testing strategy** — No testing approach mentioned. Unit tests for combat resolution? Integration tests for the full command → state change → Discord output pipeline? How are 5e rule interactions tested (there are hundreds of edge cases)?

- [ ] **42. Tiled JSON import compatibility** — The map format is "Tiled-compatible." What subset of Tiled features is supported? What happens when a DM imports a `.tmj` file with unsupported features (tile animations, infinite maps, image layers, parallax)? Silent ignore, warning, or rejection?

- [ ] **43. Multiclass spellcasting table** — Is the multiclass spell slot table hardcoded or data-driven? The table has specific slot counts per caster level — is this stored as reference data or computed?

- [ ] **44. Turn timer persistence** — Turn timeouts use `started_at` + campaign timeout setting. Are timer checks done via polling (cron job) or event-driven (scheduled task)? What's the granularity — will the 50%/75%/100% reminders fire within minutes of their target, or could they drift?

- [ ] **45. Feature Effect System extensibility** — The effect type vocabulary has 14 types. What happens when a 5e feature doesn't fit any type (e.g., Portent dice replacing rolls, Lucky feat, Divination Wizard's third eye)? Is there a generic "custom" effect type that routes to DM queue, or must new types be added to the engine?

- [ ] **46. Spell data completeness** — The `spells` table has `damage`, `healing`, `effects`, and `teleport` fields. Many spells have complex conditional effects not easily captured in structured data (e.g., Polymorph, Banishment, Wish). How are these handled — via `effects` text field with DM resolution, or structured data?

- [ ] **47. JSONB query performance** — Several hot-path queries filter on JSONB fields (conditions, features, spell_slots). Are GIN indexes used? What's the query pattern for "find all combatants with condition X" or "check if feature Y has uses remaining"?

- [ ] **48. Map size limits** — Maximum grid dimensions aren't specified. What's the practical upper bound before rendering or fog-of-war computation becomes too slow? Is there a hard limit enforced by the system?
