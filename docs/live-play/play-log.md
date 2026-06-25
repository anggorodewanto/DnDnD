# Play Log — chronological

Append a short entry per beat (setup steps, narration delivered, player commands,
outcomes, decisions). Newest at the bottom. This is the story-and-mechanics
history; `game-state.md` is the current snapshot.

---

## Session 1 — 2026-06-24

**Setup**

- Stood up the stack with `make local-up` (docker compose; app built from current
  source so combat fixes are present). Verified: app `:8080` → 307, Postgres
  healthy, `discord session opened`, `channel-bindings passed` for guild `DnDnD`.
- Inspected the DB: a campaign **already existed** from a prior playtest
  (`/setup` run on 2026-05-24) — DM = the user, all 10 channels created, one 10×10
  map imported. **0 characters, 0 encounters** → clean slate for fresh play.
- Decided the play shape (via interview): **sandbox**, Claude drives the dashboard
  in the browser, user **builds a fresh character**, Claude picks the setting.
- Created this `docs/live-play/` doc set (README / runbook / game-state / play-log)
  so any fresh agent can resume as DM.
- DM picked an opening frame: **"Ashfall Waystation"** (see `game-state.md`), to be
  tailored to the character the player builds.

- **Clean-slate decision (user):** keep the campaign shell + working Discord
  channels, delete leftovers. Deleted the old map, the resolved test-whisper, and
  3 stale portal tokens. DB now: 0 characters / 0 encounters / 0 maps; campaign +
  channels + DM ownership intact.

**Character build + first bug**

- Claude took over the authenticated DM dashboard (claude-in-chrome). Confirmed
  clean campaign home (0 approvals / encounters).
- User started building a **level-3 warlock** in the portal and hit a wall: only
  cantrips selectable, no leveled spells. → Investigated (subagent + hand-verified
  code/data): **ISSUE-001** — Pact Magic never wired into the builder's
  max-spell-level derivation (`derive_stats.go`) nor persisted at creation
  (`builder_store_adapter.go`); `PactMagicSlotsForLevel` existed but had zero
  callers. Data was fine (warlock spells seeded).
- User chose **fix now (TDD)**. Delegated a bounded TDD fix; reviewed the diff,
  confirmed build/vet/tests + `make cover-check` green, rebuilt + restarted the
  app so the fix is live. Logged the fix + a surfaced latent gap (**ISSUE-002**:
  standard-caster `spell_slots` may not persist at creation — unconfirmed).

**Class-creation audit (probe for more warlock-style gaps)**

- User asked to probe whether other classes have similar "data/consumer exists
  but builder never wires it" gaps. Ran 3 parallel read-only investigators
  (persistence / spell-level gating / non-spell resources). Found + logged
  ISSUE-002..ISSUE-007: full/half-caster `spell_slots` dropped at creation
  (confirmed, major); EK/AT not recognized as casters by the frontend (spell step
  skipped); Unarmored Defense AC never wired for Barbarian/Monk; Expertise never
  wired for Rogue/Bard; L1 paladin/ranger phantom slot; multiclass UI spell-budget
  uses primary class only. None block the warlock. See `issues.md`.

**Submit 500 → ISSUE-008 fix**

- User submitted the warlock for DM approval → **HTTP 500**. Traced to
  `characters.languages TEXT[] NOT NULL` + the builder sending no languages
  (`pq.Array(nil)` → SQL NULL). Guaranteed blocker for *all* portal builds, first
  triggered now (campaign's first portal character). User chose **fix now (TDD)**.
  Coerced nil→`[]` in `CreateCharacterRecord` (2 red→green tests), `cover-check`
  green, app rebuilt + restarted. Logged ISSUE-008 + the deeper gap (builder
  collects no concrete languages at all).

**Vale resubmitted + approved (2026-06-25)**

- Player resubmitted the warlock as **Vale** (Tiefling Fiend-pact Warlock 3,
  entertainer). Submit succeeded — **no 500** (ISSUE-008 fix is live). Inspected the
  stored record before approving: pact magic `{2 × slot-level-2}` (ISSUE-001 fix
  worked end-to-end), 2 cantrips + 4 known spells incl. three L2 (hold person,
  shatter, misty step) + hellish rebuke, HP 24, AC 10 (DEX +0), languages `'{}'`.
  Build sound → **approved** on the dashboard (queue cleared). Details in
  `game-state.md`.
- In parallel (per player request): two background subagents fixing **ISSUE-005**
  (Expertise wiring) and **ISSUE-006** (L1 half-caster phantom slot), TDD, each on
  its own worktree branch for clean integration.

**Scene opened (2026-06-25)**

- Opening "Ashfall Waystation" narration posted to `#the-story` via the dashboard
  Narrate tool (player-confirmed), tailored to Vale (Tiefling Fiend-pact warlock,
  entertainer): cold hearth, missing keeper, cellar door gouged from the inside,
  patron's pull. Awaiting Vale's first action.

**Equip + card bugs found & fixed (2026-06-25)**

- Player flagged: card shows "Equipped: —" + "Spell Slots: —", and `/character`
  shows nothing equipped despite equipping in the builder. Read-only investigator
  traced both. **ISSUE-011** (equipped gear dropped) = frontend async-load ordering
  bug; **ISSUE-012** (warlock pact slots never shown on Discord cards) = display
  gap. Both fixed TDD on worktree branches, cherry-picked to `main`, cover-check
  green. Also improved `/equip` help (slot model + `armor:true`) and shipped the
  ISSUE-002..006 builder fixes.
- **Live unblock meanwhile:** Vale can `/equip item:leather armor:true` +
  `/equip item:dagger` (or `light-crossbow`) — items are already in inventory.
- App **redeployed** (`docker compose up -d --build app`) — 002..006 + `/equip`
  help live; a second redeploy folds in 011/012.

**First action — Vale opens the cellar (2026-06-25)**

- Player (as Vale) posted in `#in-character`: *"i cast mage hand to open the cellar
  door."* In-world roleplay, not a slash command. Ruled the door unlocked/unbarred
  (the clawing was from the *inside*, trying to get out) → mage hand opens it, no
  roll. DM narration posted to `#the-story` (player-confirmed): spectral hand lifts
  the latch, door groans open; inner face gouged + planks bowed **outward**; cold
  air, wet-stone/sweet-wrong smell, stone steps into full dark, a slow dragging
  *scrape* below, then silence. Hand hovers at the threshold. Awaiting Vale's next
  move (descend / cast light / listen / retreat).
- **Stack hiccup:** both containers were found `Exited (0)` (graceful stop ~3 min
  before the post — first Narrate click failed "Failed to fetch"). Restarted with
  `docker compose up -d` (no rebuild; running image preserved). App healthy in 1s:
  HTTP 307, `discord session opened`, `channel-bindings passed`. Retried the post
  → landed once (no dup; the failed attempt never reached the server).

**Next:** Vale acts on the open cellar; DM responds. Import the 10×10 waystation map
if/when the cellar fight starts.
</content>
