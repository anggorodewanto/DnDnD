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

**Combat — Round 1 (2026-06-25)**

- **Initiative:** Forge **22** → the wretch/Ghoul **19** → Vale **19**. Combat id
  `6f317490-c43e-44a0-a1d0-b6ed51e58a3e`. Wretch AC 12 / HP 22 at the cellar mouth;
  party by the door. (Living-wretch ruling — see `game-state.md`.)
- **Forge (init 22) — handaxe throw.** Freeform *throw* action: **HIT** (roll 15 vs
  AC 12) for **7 damage**. Wretch **22→15 HP, bloodied**. Turn done. (Narrated as the
  axe biting deep before the thing even fully cleared the steps.)
- **The wretch (init 19) — closes + whiffs.** Moved off the cellar mouth into melee,
  now at **D7** (adjacent to Forge at **E7**), and used **Multiattack** — **bite (8)
  and claws (10) BOTH MISSED** Forge's AC 14. No damage. Turn done. (Narrated as a
  frantic, grasping lunge that Forge turns aside.)
- **Vale (init 19) — hold person LANDS.** Vale cast **hold person** on the wretch
  (action spent; she is now **concentrating**; pact slots **2→1**). DM rolled the
  wretch's **WIS save: 6 vs DC 13 → FAIL** → the wretch is **PARALYZED** (engine
  condition, source_spell hold person; hidden from players — narrated as the thing
  locking up "bloodied and rigid"). Vale still has her **movement + bonus action**;
  her turn is **not yet ended** (player's call).
- **Set-up for Round 2:** once Vale ends her turn, `/done` advances to Forge first —
  and Forge is within 5 ft of a **paralyzed** target, so his melee attacks get
  **advantage + auto-crit on hit**. Big swing for the party incoming.

**ISSUE-014 fix shipped mid-session + Hold Person narration posted (2026-06-25)**

- **ISSUE-014 fixed, committed, pushed, deployed.** The DM Console wasn't tracking
  player combat actions (player casts/freeform/attacks never wrote `action_log`, so
  `/api/dm/situation` `timeline[]` was blind to them). Fixed on `main` (`f1e3aeb`,
  pushed `f29edd4..f1e3aeb`): a best-effort `recordCombatAction` helper
  (`internal/combat/action_log_record.go`) writes an `action_log` row at the success
  tail of every player combat path (`Cast`, `CastAoE`, `FreeformAction`, `Attack`,
  `attackImprovised`, `OffhandAttack`). `make cover-check` green; review ship-ready.
  **Redeployed** `docker compose up -d --build app` ~13:45 UTC — clean boot (db
  connected + migrated, no new migration; discord session opened; all checks passed;
  `:8080`; no error). **DM-side only:** #combat-log output is unchanged (players never
  see the Console), and save adjudication stays a manual DM roll. **Live combat state
  preserved across the redeploy** (still Round 1, Vale's turn active, wretch paralyzed).
- **Hold Person narration POSTED to #the-story.** Drove the beat through the
  dashboard **Narrate** editor (#narrate tab, authenticated as DM) → **Post to
  #the-story** → the bot relayed it. Recorded as a `narration_posts` row at
  **13:51:18 UTC**, Discord message id **`1519701526946386084`**. (Text: Vale's
  Infernal incantation snaring the wretch; it locks rigid, paralyzed — "a puppet on a
  severed string" — Forge within reach.)

**Next:** Vale finishes her turn (movement/bonus action — player decides), then End
Turn / `/done` opens Round 2 with Forge auto-critting the paralyzed wretch. Keep Vale's
concentration intact (CON save on any damage to her, or the paralysis drops).

**Two live-play bugs fixed + redeployed (2026-06-25, ~22:50 UTC)**

- Two more bugs surfaced **during this live combat** and were fixed, shipped, and
  redeployed in one commit (`main` `b108bf2`, pushed `0dfa1ec..b108bf2`). Both found
  while watching Vale's Hold Person beat play out:
  - **ISSUE-016 (NEW) — `/done` phantom "1 attack" after a spell cast.** When Vale
    (Warlock 3, no Extra Attack) cast Hold Person with her **action**, `/done` warned
    "you still have 1 attack" — an attack she never had. Casting a spell is the
    Cast-a-Spell action, not the Attack action, but `Service.Cast`/`CastAoE` left the
    seeded `attacks_remaining=1` in place, so the `/done` unused-resource check (and the
    "Remaining" summary) reported a phantom attack. **Fixed:** zero
    `turn.AttacksRemaining` when a spell consumes the action (cantrip or leveled);
    bonus-action casts left untouched. Red/green `cast_attacks_remaining_test.go`,
    `make cover-check` green. Severity medium (misleading UX).
  - **ISSUE-015 DISPLAY half — paralysis showing as "[object Object]".** The Combat
    Manager rendered the wretch's *hold person* paralysis as **"[object Object]"** — the
    engine stores conditions as objects (`{condition:"paralyzed",…}`) but the Svelte UI
    interpolated each entry as a string. **Fixed:** new `conditionName()` helper
    (`dashboard/svelte/src/lib/combat.js`) Title-Cases either an object's `.condition` or
    a bare string; `CombatManager.svelte` renders `conditionName(cond)`. vitest 64/64,
    svelte build clean, embedded assets regenerated. **Display half only** — the
    **WRITE half of ISSUE-015 stays OPEN** (the dashboard "add condition" PATCH still
    writes a bare string array the engine ignores, so a button-added condition renders
    but no-ops mechanically; the correct-shape writer remains the DM-Override POST). The
    live paralysis renders correctly now because it was written in the object shape.
- **Redeployed** `docker compose up -d --build app` ~22:50 UTC — clean boot (db connected
  + migrated, no new migration; discord session opened; all checks passed for guild
  `1507910398886543532`; server `:8080`; no error). **Live combat state preserved across
  the redeploy** (still Round 1, Vale's turn active, wretch paralyzed at D7 15/22, Forge
  E7 32/32, Vale K6 24/24 concentrating, 1 pact slot left).
- **Cosmetic caveat:** Vale's *current* turn still carries the pre-fix
  `attacks_remaining=1` (the ISSUE-016 fix only affects casts made on the new binary), so
  `/done` will still warn **once** for this turn — she just confirms past it; her next
  cast is clean.

**Next (unchanged):** Vale finishes her turn (movement/bonus action — player decides),
then `/done` opens Round 2 with Forge auto-critting the paralyzed wretch. Keep Vale's
concentration intact.

**Rounds 2–3 — Forge auto-crits the wretch to death (2026-06-26, reconstructed)**

> Reconstructed 2026-06-26 from live DB (`turns` / `combatants`) + Discord #combat-log —
> these beats had played out in Discord but were never logged here, and `game-state.md`
> had drifted to a stale "Round 1." **No DM narration was posted for any of this** (last
> #the-story post is still the R1 Hold Person beat, 2026-06-25 13:51 UTC).

- **R2 Forge — auto-crit, wretch survives.** Forge (adjacent E7↔D7, target paralyzed) hit
  with dual handaxes, both **auto-crit**: main **10** (2d6+2) + bonus-action off-hand vex
  **2** (2d6) = **12 dmg**. Wretch **15→3** — it **did not drop** (the light-weapon crits
  rolled low). **Wretch's R2 turn auto-skipped** (paralyzed, can't act).
- **R2 Vale — crossbow miss.** Vale fired her **light crossbow** at the paralyzed wretch
  (advantage) → **roll 10, MISS**. Turn completed (~13:31 UTC). *(This is the R2 turn the
  player flagged as already done.)*
- **R3 Forge — the kill.** Forge auto-crit handaxe again: main **12** (2d6+2) — already
  lethal against 3 HP — off-hand **6** overkill. **Wretch → `0/22`, DEAD** at **13:32 UTC**
  (`is_alive=f`). Notably Forge was **not raging**; an unraged barbarian's 2d6+2 crits took
  **two rounds** to finish a 15-HP target. The wretch never landed a hit the entire fight.
- **R3 Vale — turn currently ACTIVE but moot.** The turn queue advanced to Vale (action
  unused), but the only enemy is already dead.
- **State left open:** encounter status is still **`active`** (no End-Combat fired), and
  Vale is still flagged **concentrating on hold person** against a corpse.

**Next:** (1) **narrate the kill** to #the-story (2 rounds behind — wrap in `:::read-aloud:::`;
never say "paralyzed"); (2) **resolve the encounter** — End Combat for victory, or send the
reserve 2nd wretch up the pit if one kill is too light; (3) drop Vale's stale concentration.
See `game-state.md` "Next action."

**Kill narrated + combat ended + End Combat button shipped (2026-06-26)**

- **Kill narrated.** Posted the wretch's death beat to #the-story via the dashboard Narrate
  editor (read-aloud block) — `narration_posts` 2026-06-26 **13:45:15 UTC**, Discord msg
  **`1520062389649670288`**. The bot rendered it as a read-aloud box. Masked throughout:
  "locks rigid / seized," the wretch "comes apart at the shoulder," no HP/AC numbers, the
  word "paralyzed" never used. Aftermath teed up the cellar (the thing had a face once; the
  pit still gapes, door clawed from the inside).
- **End Combat button added to the dashboard (it didn't exist).** The Combat Manager only had
  **End Turn** — no way to end the whole encounter from the UI (the `POST /api/combat/{id}/end`
  endpoint existed but was unwired on the frontend). Added it TDD:
  - `endCombat(encounterId)` in `dashboard/svelte/src/lib/api.js` (red/green `api.test.js`,
    70/70 then full suite **591/591** vitest green).
  - **End Combat** button in `CombatManager.svelte`'s encounter header with an **inline
    two-step confirm** (End Combat → "End this combat? Confirm/Cancel") — deliberately *not*
    a native `window.confirm` (those block claude-in-chrome automation and are worse UX). On
    confirm it calls `endCombat`, then `loadWorkspace()` so the ended encounter drops out.
  - Rebuilt embedded assets (`npm run build` → `internal/dashboard/assets`) + redeployed
    (`docker compose up -d --build app`). Clean boot; combat state preserved through the redeploy.
- **Combat ended via the new button.** Clicked End Combat → Confirm End on the live encounter.
  Result (verified in DB): encounter `status=completed`, **Vale's `concentration_spell_id`
  cleared** by the EndCombat service, ghoul `0/22 is_alive=f`, PCs full. Combat Manager now
  reads "No active encounters." Victory banked.
- **README diligence rule added** (`docs/live-play/README.md`, Hard constraints): keep
  narration + state docs in lockstep with the engine; on resume treat DB + #combat-log as the
  source of truth and reconcile before acting. This whole session's "Round 1 vs reality Round 3"
  drift is exactly what it guards against.

**Next:** players decide in `#in-character` — most likely the cellar descent. Start a fresh
encounter (reserve wretch) if they go down and want a fight. See `game-state.md` "Next action."

**Cellar descent encounter pre-built + encounter-builder bug fixed (2026-06-26)**

- **Pre-built the next fight** (the cellar descent) so it's one click to run when the party
  goes down:
  - **Map:** built "Ashfall Waystation — cellar" (`d2fe03c6-…`), 12×10 blank stone grid via
    Maps → New Map, with a **PC spawn zone** at the top-center stairs landing. Features narrated.
  - **Encounter:** "Cellar — the brood" / player-facing "The Cellar" (`0a54efd4-…`) on that
    map — **2× Ghoul wretches** placed in the back corners, **G1 (2,8)** + **G2 (9,8)**, party
    Vale + Forge. DM design call (delegated): two wretches = a real fight for two L3s.
- **Bug found + fixed mid-build: encounter builder couldn't save edits to an existing
  encounter.** While placing the 2nd wretch, the builder Save kept no-op'ing; the page
  surfaced **`campaign_id query parameter required`**. Root cause: the frontend
  `getEncounter` / `updateEncounter` / `deleteEncounter` / `duplicateEncounter`
  (`dashboard/svelte/src/lib/api.js`) never appended the backend-required `?campaign_id=`
  (only `createEncounter`/`listEncounters` did) — so **Edit, Save-after-create, Delete, and
  Duplicate of any existing encounter all 400'd.** G1 only persisted because it was placed
  before the first *create*-save (which sends campaign_id in the body).
  - **Fixed TDD:** added 4 red→green `api.test.js` cases asserting `campaign_id` is in each
    URL; added the param to all four api.js fns + their call sites (`EncounterBuilder.svelte`
    ×2, `EncounterList.svelte` ×2). Full vitest **595/595**. Rebuilt embedded assets +
    redeployed (`docker compose up -d --build app`, clean boot). Re-opened the encounter via
    **Edit** (now works), placed G2, Saved → "Encounter saved." → DB confirms G1 (2,8) + G2
    (9,8). Severity: real — you couldn't edit/save/delete/duplicate any saved encounter.

**Next:** players decide in `#in-character`; on descent, open "Cellar — the brood" → Start
Combat (PCs auto-seat at the stairs spawn zone; G1/G2 lurk in the back). See `game-state.md`.
</content>
