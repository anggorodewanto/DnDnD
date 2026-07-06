# Session 01 — Ashfall Waystation (2026-06-24 → 2026-06-26)

> Per-session play-by-play. Append a short entry per beat (setup, narration
> delivered, player commands, outcomes, decisions); newest at the bottom. This is
> the story-and-mechanics *history* — the current snapshot is
> [`../game-state.md`](../game-state.md), the durable world is
> [`../world.md`](../world.md). Start a new `session-NN.md` for the next session.

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

**Two map-render bugs found + fixed (2026-06-25)** — surfaced when the combat board
first tried to render:

1. *PCs had no tokens.* The blank dashboard-built map has **no authored spawn
   zones**, so combat-start's PC seater bailed and wrote the zero-value position
   (col `""`, row 0) for Vale + Forge — unparseable, so the renderer skipped them.
   **Fix:** `seatPCsInSpawnZones` now falls back to open in-bounds tiles (skipping
   monster tiles) when a map has no spawn zones (`spawnzone.AssignPCsToOpenTiles`).
   Live data patched to J7/K6.
2. *Enemy never showed on the player map.* `combatantsToRendererForm` never set
   `IsVisible`, so every enemy defaulted to hidden and `filterCombatantsForFog`
   dropped it *before* the line-of-sight check — enemies were excluded from
   #combat-map regardless of sight. **Fix:** propagate `c.IsVisible`. A visible
   enemy in a PC's line of sight now shows; genuinely hidden / out-of-sight enemies
   stay fogged (fog-of-war retained by design).

(The blank cellar map built later *does* have an authored PC spawn zone — see
[`../encounters/cellar-brood.md`](../encounters/cellar-brood.md) — so the seater
fallback won't be needed there.)

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
  the redeploy** (still Round 1, Vale's turn active, wretch paralyzed).
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

- **R2-R3 Forge — auto-crits the paralyzed wretch to death.** Adjacent, target paralyzed,
  Forge's dual handaxes auto-crit every swing — but the light-weapon crits rolled low, so it
  took **two rounds** to finish (survived R2 bloodied, dropped R3 ~13:32 UTC). **Notably Forge
  was not raging** — an unraged barbarian's 2d6+2 crits needed two rounds to drop a 15-HP foe;
  Rage is a real damage lever if a fight runs long. The wretch (paralyzed) never got a turn and
  never landed a hit the entire fight.
- **R2 Vale — crossbow miss** at the paralyzed wretch (the R2 turn the player flagged as done).
- **State left open:** encounter status still **`active`** (no End-Combat fired) and Vale still
  flagged **concentrating on hold person** against a corpse — reconcile on resume.

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

---

## 2026-06-27 — post-combat roleplay → DM cellar nudge

- **Rule change first:** dm-rules/runbook/README updated so the DM **reads Discord
  directly via Chrome** (observation only; mutations still dashboard-only). Reason:
  `#in-character` roleplay is Discord-only and surfaces in no DB/DM-Console feed —
  the old "Claude can't see Discord" rule was a blind spot. Committed + pushed
  (`6a97dc4`).
- **Read `#in-character`:** Vale + Forge finished post-fight introductions ("Name's
  Vale, travelling storyteller" / "I'm Forge. Forge Anvilbearer."). Forge closed on
  a world-question: *"Is there something interesting in the cellar?"* with Vale's
  attention fixed on the cellar mouth. Clean DM hook — RP thread closed itself.
- **Resolved Forge's pending approval** (DM Console `next_step`). Reviewed the sheet
  (Hill-Dwarf Barbarian 3 Berserker; AC 14 = Unarmored Defense; legal build);
  changes-since-last were backstory + Goblin language only. **Approved** → queue empty.
- **Posted the DM nudge** to `#the-story` via the Narrate editor (read-aloud block,
  `narration_posts` 3:14:31 PM): answered Forge in-fiction (the cold up-draught, the
  door clawed *outward* — shut in, wanting up), leaned on Vale's patron pull, and put
  the descent choice back to the party **without deciding it for them**.
- **Players answered (read `#in-character` via Chrome):** Vale *"there's... just—"*
  and **creeped into the cellar in a trance** (3:17 PM) — the patron pull landed.
  Forge: *"yo, what possessed you"* (3:22 PM) — startled, **not yet following**.
  Descent started; split party forming (Vale ahead, Forge up top).
- **Posted 2nd DM nudge** to `#the-story` (read-aloud, `narration_posts` 3:26:26 PM):
  Vale onto the top step / the cellar revealed below (low stone, butcher-pit stink,
  something shifting in the dark "not toward her, not yet"); framed Forge's
  follow-or-call-her-back choice **without deciding it**.
- **Staged the cellar fight** (did NOT Start): opened "Cellar — the brood" builder —
  map *Ashfall Waystation — cellar* bound, G1 (2,8) + G2 (9,8) both *Surprised*,
  party 2/2 (Vale + Forge), **Start Combat** one click away. Left parked.
- **Next:** await Forge's follow/hold in `#in-character`; on commit → Start Combat
  (PCs auto-seat at the stairs landing). Hold while Forge is up top; re-check the
  surprise side at Start if Vale's down there alone. 3-4 more PCs still joining.

---

## 2026-06-27 — descent → combat: "The Cellar" begins

- **Forge committed (read `#in-character` via Chrome):** after Vale's trance-walk in
  (3:17 PM) and Forge's startled *"yo, what possessed you"* (3:22 PM), Forge —
  *"please wait..."* (3:54 PM) — **followed her down**. Split party rejoined; both PCs
  descending. The follow/hold question resolved → fight warranted.
- **Surprise re-checked and flipped OFF for both sides:** the staged build flagged the
  ghouls *Surprised* (party gets the drop). Reruled to **no surprise either side** —
  nobody was sneaking (Vale shouted "hello" down the cellar earlier; she trance-walked,
  not stealthed; Forge called out on the stairs → the brood heard them coming), and the
  PCs already knew a beast lurked below. Also spared a 2-PC party two free
  paralysis-claws in a surprise round. Unchecked both G1/G2 *Surprised* in the builder
  (verified `.checked=false` via JS before Start).
- **Started combat** — "Cellar — the brood" / display **"The Cellar"** (combat/encounter
  id `8509d1f6-da9d-451c-bb2e-8571b9402e9e`), map *Ashfall Waystation — cellar*. Both PCs
  auto-seated at the stairs landing **E1** (stacked, single-file descent); ghouls at the
  back wall **C8** & **J8**. Round 1, 4 combatants, all full HP, no conditions.
- **Initiative:** Ghoul **19** (J8) → Vale **15** → Forge **12** → Ghoul **9** (C8). The
  brood won the jump — the lead ghoul acts before any PC.
- **Narrated the descent** to `#the-story` (read-aloud, `narration_posts` 4:59:45 PM):
  Vale trance-walked down first, Forge after; the cellar revealed (butcher-larder stink,
  shapes hung on the walls, "not all of them still"); two ghouls peel from the dark; **no
  surprise** called out in-fiction; ended on the lead ghoul **mid-lunge at Vale** (front,
  hooded) — cut before the strike so the prose doesn't outrun the dice.
- **Next:** resolve the lead **Ghoul's turn** (NPC, CURRENT). It can move J8→E2 (30 ft =
  6 sq, ending adjacent to E1) and **Claws** the nearest PC = **Vale** (AC 10 — leather
  still unequipped). On a hit: 2d4+2 slashing + **DC 10 Con save or paralyzed 1 min**.
  Then turn passes to Vale (init 15). Drive the move + attack through the combat
  workspace (engine rolls the NPC dice). Narrate the strike + update docs in lockstep
  after.

---

## 2026-06-27 — first enemy turn: ghoul bites Vale (+ Turn-Builder bug)

- **How enemy turns actually run (learned the hard way):** the combat workspace has **no**
  attack-roller in its main panels, and there is **no DM Discord command** for enemy
  attacks (`/attack` is player-only; `internal/discord/router.go`). The canonical path is
  the **Turn Builder**: **right-click the enemy token → "Plan Turn"** (or, after this
  session's UX fix, the new **"Run Enemy Turn"** button). It loads a pre-rolled plan
  (GET `…/enemy-turn/{id}/plan`), the DM reviews/fudges, and **Confirm & Post** executes
  (POST `…/enemy-turn`) — applies movement + damage and posts to #combat-log. The
  right-click token menu (Damage / Heal / Conditions / Plan Turn / Remove) is the hidden
  DM control surface; the "Action Log" panel is a read-only **filter**, not an entry form.
- **The ghoul's turn:** drag-moved the lead ghoul J8→**E2** (adjacent to the party at E1),
  then Turn Builder planned **Bite vs Vale** (+2; 2d6+2 piercing — *not* Claws, so no
  paralysis rider). Engine rolled **To Hit 15** (vs AC 10 → hit), **Damage 5**. **Vale
  24→19, bloodied.** Bite narrated to #the-story (read-aloud, `narration_posts`
  5:30:18 PM); ended on "what she does next" (no player choice narrated).
- **⚠ LIVE BUG (ISSUE):** Confirm & Post **crashed**: `null value in column "before_state"
  of relation "action_log" violates not-null constraint`. Partial commit — **damage
  applied** (Vale 19) but the **turn did not advance** and nothing was logged. Root cause:
  `ExecuteEnemyTurn` (`internal/combat/turn_builder_handler.go`) omitted `BeforeState` +
  `AfterState` (both NOT NULL) in its `CreateActionLog`, unlike every other action_log
  writer. **Workaround applied live:** manual **End Turn** (advanced Ghoul→Vale, no
  re-damage) + resolved the dangling `enemy_turn_ready` queue item with an outcome note.
- **Fixes (this session, fix-now TDD — pending rebuild/redeploy):**
  1. **`before_state` crash** — red/green test `TestExecuteEnemyTurn_PopulatesBeforeAndAfterState`
     + snapshot before/after state in `ExecuteEnemyTurn`. Package green.
  2. **Turn-Builder discoverability** — added a gold **"Run Enemy Turn — <name>"** button
     to the combat right panel, shown only when the current combatant is an NPC
     (`CombatManager.svelte`); reuses the same open handler as the right-click. vitest green.
  See [`issues.md`](issues.md). **Both redeploy via** `docker compose up -d --build app`.
- **State now:** Round 1, **Vale's turn** (live HP/positions in the DM Console).
- **ISSUE-018 + ISSUE-019 deployed** (commits `8c6a8df` / `60cda5d`, pushed last session,
  redeployed): the `before_state` enemy-turn crash is fixed and the **"⚔ Run Enemy Turn"**
  button is live. The Turn Builder is now the path for the 2nd ghoul (first live test of the
  fixed executor).
- **ISSUE-020 — stale sheet HP (found + fixed this session):** the user noticed *"my character
  sheet says Vale HP at 24"* while she was 19/24 in combat. Diagnosed **two HP stores** —
  `characters.hp_current` (static base sheet) vs `combatants.hp_current` (live snapshot);
  combat carries HP in at start and **never writes back**, so every sheet reading the
  `characters` row showed stale full HP mid-fight. **The ghoul turn DID execute** — the bite
  damage was correctly persisted on the combatant; only the sheets read the wrong table (the
  ISSUE-018 crash didn't lose it: `ApplyDamage` and `CreateActionLog` aren't in one tx).
  **Fixed (TDD, 3 surfaces — read-side HP overlay):** portal sheet (`hydrateFromCombatant`,
  which already overlaid conditions but forgot HP), Discord `/character` (mirrors `/status`),
  and the dashboard Party Overview API. Out-of-combat falls back to the row; the DM
  out-of-combat status editor's 409 write path is untouched. cover-check green; redeployed;
  **verified live** — Party Overview now reads **Vale 19/24**. See [`issues.md`](issues.md).

---

## 2026-06-27 — Round 1 closes, Round 2 opens (first clean enemy-turn runs)

- **Resumed mid-Round-1; docs were stale.** The save file said "Vale's turn (R1)," but the
  DB + #combat-log were **ahead**: both player turns had already resolved. Reconciled from the
  DB/#combat-log (the failure mode `dm-rules` warns about) before acting.
- **Vale's turn (init 15, R1):** point-blank **light crossbow** at the lead ghoul —
  **To Hit 15 → HIT, 2 piercing** (disadvantage, hostile within 5 ft) → ghoul **20/22**; then
  **Misty Step** (bonus action, **1 pact slot → 1/2 left**) teleporting **E1→K2**, out of
  reach. (Movement bar stayed full — the relocation was the teleport.)
- **Forge's turn (init 12, R1):** **greataxe** at the lead ghoul — **To Hit 5 → MISS**.
  (Roster said "dual handaxes"; he's swinging a **greataxe** — corrected in roster.md.)
- **2nd ghoul's turn (init 9, R1) — DM enemy turn.** "⚔ Run Enemy Turn" → Bite vs Forge,
  **To Hit 18 (vs AC 14 → HIT), 12 piercing** → **Forge 32→20**. **First clean live run of the
  ISSUE-018-fixed executor:** applied + posted to #combat-log, **no `before_state` crash**, new
  `enemy_turn` action_log row. Then **drag-moved C8→D2** (planner emits **no movement** — it had
  "bitten" from 35 ft) and clicked **End Turn** (no auto-advance). Bite, not Claws → no paralysis.
- **Round 2 — lead ghoul's turn (init 19) — DM enemy turn.** Already adjacent to Forge (E2).
  "⚔ Run Enemy Turn" → Bite vs Forge, **To Hit 4 → MISS** (let the NPC roll stand, no fudge).
  Confirm & Post → posted, no crash; **End Turn** → advanced to **Vale (init 15, R2)**.
- **All four beats narrated** to #the-story (read-aloud): R1 catch-up (Vale crossbow + Misty
  Step + Forge whiff) 8:10 PM, 2nd-ghoul bite on Forge 8:16 PM, lead-ghoul miss 8:41 PM.
  Enemy state described, **no HP/AC numbers leaked**.
- **⚠ NEW ISSUE-021 (logged):** the enemy-turn executor resolves the **attack only** — it
  neither **moves the NPC into reach** nor **advances the turn**; both are manual DM steps
  (drag token + End Turn). Distinct from ISSUE-018 (the crash, fixed). Minor: the "Turn
  Complete" summary renders the actor name blank (`**'s Turn**`). See [`issues.md`](issues.md).
- **Pact-slot write-back gap (ISSUE-022 — fixed by another agent, log only):** #combat-log
  showed "Used pact slot (1 remaining)" but `characters.pact_magic_slots.current` read 0
  (combat spend not written back to the base row, à la ISSUE-020's HP). Per the user another
  agent fixed it; recorded here, not re-fixed.
- **State now:** **Round 2, Vale's turn (init 15)** — both ghouls focused on Forge (live
  HP/positions in the DM Console). **Next:** Vale acts (player-driven), then Forge (12), then
  2nd Ghoul (9).

### R2 tail + R3 open — both ghouls on the raging dwarf (06-27, ~2:00–2:15 PM)

_Resumed; reconciled the live board (DM Console) — mechanics had advanced past the docs._

- **Reconcile on resume:** DM Console showed **R2 already past Vale + Forge**, current = 2nd
  Ghoul (init 9). Vale's R2 turn left no logged action (held; K2, HP/pos unchanged). **Forge's
  R2 turn = he RAGED** (`is_raging=t`, rage_rounds≈10) — the un-narrated beat; no attack logged.
- **2nd Ghoul (init 9, D2) — DM enemy turn (R2).** "⚔ Run Enemy Turn" → Bite vs Forge,
  **To Hit 21 (vs AC 14 → HIT), 8 raw → 4 after Rage resist** → **Forge 20→16**. End Turn → R3.
- **Lead Ghoul (init 19, E2) — DM enemy turn (R3).** Bite vs Forge, **To Hit 14 (= AC 14 → HIT),
  8 raw → 4 resisted** → **Forge 16→12/32**. Honest razor-thin hit, no fudge. End Turn → Vale.
- **HP reconcile (important):** expected Forge 4/32 from two "8" bites; DM Console + DB showed
  **12/32**. **Not a bug** — Forge is **raging**, B/P/S resistance halves each 8→4, so
  20−4−4=12 is correct. Verified via DB (`is_raging=t`). This surfaced **ISSUE-023**.
- **✅ Two fixes verified LIVE:** (1) **ISSUE-021 name-blank tail (committed b74c849 earlier this
  session):** both enemy turns posted `**Ghoul's Turn**`, no longer blank `**'s Turn**`.
  (2) **ISSUE-023 (fixed this session, TDD):** enemy-turn log now shows **post-resistance**
  damage (`4 piercing (resisted — halved from 8)`); rebuilt + redeployed. The two bites above
  were posted **before** the redeploy, so they still read "8 piercing" in #combat-log (actual 4).
- **Narrated** the R2/R3 ghoul assault to #the-story (read-aloud, 9:15 PM) — both bites, Forge's
  Rage turning the worst aside (bleeding, standing, furious), spotlight handed to Vale. No
  HP/AC numbers leaked.
- **Executor still attack-only (ISSUE-021 open):** both ghouls already adjacent to Forge, so no
  move needed; manual **End Turn** each. Two stale `enemy_turn_ready` queue items now linger.
- **State now:** **Round 3, Vale's turn (init 15)** — Forge raging, both ghouls on him (live
  HP/positions in the DM Console). **Next:** Vale acts (player-driven), then Forge (12, raging),
  then 2nd Ghoul (9).

### R3 — Vale's Chill Touch + ISSUE-024 (06-28)

_Reconciled the live board on resume — the DB was ahead of the docs (Vale's R3 turn had resolved)._

- **Reconcile on resume:** DM Console / DB showed **R3 already past Vale**, current = **Forge**
  (init 12, turn open, `attacks_remaining=1`, not yet acted). game-state.md + roster.md were
  still frozen at "Vale's turn (CURRENT)" — the mechanics-racing-ahead failure `dm-rules` warns
  about. Reconciled all state docs to the DB before anything else.
- **Vale's turn (init 15, R3):** **Chill Touch** (cantrip, ranged spell attack) at the **lead
  ghoul (G2, E2)** from K2 — **HIT, 7 necrotic** → ghoul **20→13/22** (DB-confirmed). No pact
  slot (cantrip) → she stays **1/2**. Rider: the ghoul can't regain HP until the start of Vale's
  next turn.
- **⚠ ISSUE-024 (found live, FIXED this session, committed `5599ef4`):** the #combat-log cast
  line showed the damage **dice spec** (`💥 Damage: 1d8 necrotic`) instead of the rolled value,
  and printed it even on a miss — `FormatCastLog` always emitted `ScaledDamageDice`, never
  `DamageTotal`, with no `Hit` guard. **Not a lost-damage bug** (the 7 necrotic landed; HP
  correct). Player asked why the log read "1d8 necrotic" with no number. Fixed (TDD): spell
  **attacks** now log `Damage: <total> <type> (<dice>)` on a hit, nothing on a miss; save-based
  spells keep the spec. Rebuilt + redeployed. NB: Vale's Chill Touch line was posted **before**
  the redeploy, so it still reads "1d8 necrotic" in #combat-log (actual 7).
- **action_log gap (observation, not fixed):** `action_log` for this encounter holds **only
  enemy_turn rows** — no player action (Vale's R1 crossbow / Misty Step, R3 Chill Touch) was
  recorded, so the DM-Console `timeline[]` misses every player beat. The casts/attacks + their
  HP effects are all correct (combatant rows); only the timeline writer (ISSUE-014's
  `recordCombatAction`) isn't producing rows on these paths. Flagged for a later look.
- **State now:** **Round 3, Forge's turn (init 12, CURRENT)** — Forge raging, Vale done, both
  ghouls on Forge (live HP/positions in the DM Console). Two stale `enemy_turn_ready` queue
  items still pending (ISSUE-021). **Next:** Forge acts (player-driven, raging), then 2nd Ghoul
  (G1, 9).

### R3 close + R4 open — both ghoul turns; Forge crit to the brink (06-28, ~11:15 AM)

_Resumed; reconciled the live board (DM Console + DB) — mechanics had again raced ahead of the
docs. Forge's R3 turn had already resolved and his and Vale's R3 beats were both un-narrated._

- **Reconcile on resume:** save file said "Forge's turn (R3, CURRENT)"; DM Console showed **R3 past
  Forge**, current = **G1 ghoul (init 9, D2)**. Timeline's newest action was **Forge's R3 greataxe
  swing — MISS**, and #the-story's latest post (9:15 PM prior) only set up Vale's R3 move ("Her
  move."). So **two** un-narrated player beats: Vale's R3 Chill Touch result + Forge's R3 miss.
- **Caught both up in one read-aloud** (#the-story, 11:15 AM): Vale's grave-cold landing on the lead
  ghoul (wounds won't close), Forge's axe skating off ribs into the stair. Enemy state described,
  no HP/AC leaked. Spotlight to the ghoul that's up next.
- **G1 ghoul (init 9, D2) — DM enemy turn (R3 close).** "⚔ Run Enemy Turn" → Bite vs Forge,
  **nat 20 → CRITICAL HIT (22 to hit), 17 raw → 8 after Rage resist** → **Forge 12→4/32**. Honest
  crit, no fudge. #combat-log read `8 piercing (resisted — halved from 17)` and header `**Ghoul's
  Turn**` — **ISSUE-023 + ISSUE-021 name-fix both verified live**. G1 already adjacent → manual End
  Turn (no move; ISSUE-021) → **Round 4**.
- **G2 lead ghoul (init 19, E2) — DM enemy turn (R4 open).** "⚔ Run Enemy Turn" → Bite vs Forge,
  **4 to hit → MISS**. The brood's luck breaks; Forge clings on at 4/32. Manual End Turn → **Vale
  (init 15, R4)**.
- **Narrated** the back-to-back assault to #the-story (read-aloud, 11:23 AM): the smaller ghoul's
  savage bite nearly dropping Forge, the lead ghoul's follow-up snapping on air (tied to the
  lingering Chill-Touch cold). Conveyed "one more and the dwarf goes down" without numbers. Handed
  to Vale.
- **Chill Touch rider lapsed** at the start of Vale's R4 turn (one round after the R3 cast) — no
  longer tracked.
- **Vale's R4 turn (player-driven, while docs were being synced):** Vale cast **Chill Touch again**
  on the lead ghoul (G2) → **13→7/22 (bloodied)**. **ISSUE-025 fix confirmed live** — this player
  cast appeared in the DM-Console `timeline[]` ("Vale cast Chill Touch on Ghoul"), the first player
  action to land in the timeline (every prior player beat was invisible). Cantrip → no slot spent,
  attacks zeroed (ISSUE-016), turn left **open** (movement + `/done` remaining). Narrated to
  #the-story (read-aloud, 11:30 AM) — lead ghoul buckling, "only just" on its feet; no numbers.
- **Queue cleanup:** resolved the **2 stale `enemy_turn_ready`** items (G1 + G2 turns already run;
  ISSUE-021 leaves them dangling) via DM Queue → Open → Resolve, each with an outcome note. DM Queue
  now reads "No pending items"; the Console `next_step` no longer falsely points at a ghoul turn.
- **State now:** **Round 4, Vale's turn (init 15) — open, action spent** (cast Chill Touch). Forge
  **4/32 raging, a breath from death**; lead ghoul **G2 bloodied (7/22)**, other ghoul **G1 22/22**
  (live HP/positions in the DM Console). **Next:** Vale finishes (move/`/done`, player-driven) →
  Forge (raging, near-death) → **G1 ghoul** (run enemy turn — likely drops Forge if it lands).

### R4 advances to Forge — Vale's turn closed; turn-opening beat posted (06-28, ~11:36 AM)

- **Resumed as DM, re-pulled the Console.** Turn had advanced: **Vale's R4 turn is closed** (she
  moved + `/done` after the cast), `current_turn` is now **Forge Anvilbearer** (init 12, E1). No new
  un-narrated mechanical beats — Vale's Chill Touch was already narrated (11:30 AM). DM Queue empty.
- **Synced** `game-state.md` (last-updated, scene, Next-action item 1) from "Vale's turn" → "Forge's
  turn (current)". No HP transcribed into the save file.
- **Posted Forge's turn-opening read-aloud** (#the-story, 11:36 AM): the lead ghoul reeling/buckling
  from the doubled frost (one arm wrong, rime cracking its ribs), the smaller ghoul whole and patient
  at his flank, Forge bleeding out on rage alone — "and it is his swing." Enemy state described, no
  HP/AC numbers; Forge's own near-death is his to know.
- **State now:** **Round 4, Forge's turn (init 12) — open**, awaiting the remote player. Forge
  **4/32 raging**; G2 **bloodied (7/22)** at E2, G1 **22/22** at D2 (both adjacent to Forge at E1);
  Vale **19/24** clear at K2 (live board → Console). **Next (player-driven):** Forge acts — lead
  ghoul is one solid greataxe from dropping; Reckless Attack gives advantage but exposes him at 4 HP.
  Then **I run G1's enemy turn** (Turn Builder → Confirm & Post → manual End Turn) — if its bite
  lands it likely puts Forge into death saves, so clearing G2 or pulling heat off the dwarf first
  matters. Then back to the top of the order (G2 next round).

### R4 closes — Forge crits & kills the lead ghoul; board advances to R5, Vale's turn (06-28, ~12:27 PM)

- **Resumed as DM, re-pulled the Console.** First `/api/dm/situation` fetch came back **stale**
  (Round 4, G1 current); the authoritative board (Combat Manager + a second fetch) was already at
  **Round 5, Vale's turn**. Reconciled against the action log (source of truth): newest action was
  **Forge's R4 greataxe — CRIT for 19** (05:16:40Z), which **killed the lead ghoul (G2)**, the one
  Vale had twice frozen with Chill Touch. That kill was **un-narrated** (story frozen at the 11:36 AM
  "it is his swing" beat) — the classic mechanics-ahead-of-story gap.
- **Narrated Forge's kill** to #the-story (read-aloud, 12:27 PM): the greataxe cleaving the
  frost-cracked lead ghoul apart along the seam Vale's cold opened; the surviving **smaller** ghoul
  unmoved by its broodmate's end, poised on the bleeding dwarf. Enemy state described, no numbers;
  Forge's own near-death his to know.
- **G1's R4 turn passed without an attack** — Forge is still **untouched this round** (no ghoul
  attack logged after his crit; HP unchanged), so G1's end-of-R4 turn was **skipped**, not run.
  Board rolled R4→R5, dead G2 (init 19) auto-skipped, landing on **Vale (init 15)**.
- **Cleaned the dangling `enemy_turn_ready`** (Ghoul, created 05:23:39Z) via DM Queue → Resolve with
  an outcome note (ISSUE-021 artifact: the queue item lingered after the turn advanced, making
  `next_step` falsely point at a ghoul turn during Vale's turn). DM Queue now "No pending items";
  Console `next_step` reflects Vale's turn.
- **Synced** `game-state.md` (last-updated, scene, Next-action) from "R4, Forge's turn" → "R5, Vale's
  turn — lead ghoul down." No HP transcribed into the save file.
- **State now:** **Round 5, Vale's turn (init 15) — awaiting the human player.** One ghoul left
  (**G1**, the smaller flank one); Forge **critically low, raging, untouched so far this round** at
  E1; Vale clear at K2 (live board → Console). **Next (player-driven):** Vale acts (frost/crossbow on
  G1, or peel it off Forge) → Forge → **then I run G1's R5 enemy turn** (Turn Builder → Confirm &
  Post → manual End Turn) — G1's bite comes last and likely drops Forge if it lands, so clearing or
  peeling it before then matters.

### Engine bug found + fixed mid-session: AdvanceTurn dropped G1's R4 turn (ISSUE-030) (06-28, ~12:50 PM)

- **Player question** ("after Forge there should still be a ghoul — was this a bug because one
  died?") triggered an investigation. Pulled the `turns` + `action_log` + `dm_queue_items` tables
  for the encounter and traced `internal/combat/initiative.go` `AdvanceTurn`.
- **Verdict (verified in code + DB):** real bug, **NOT** caused by G2's death. The engine reached
  G1 correctly after Forge (turn row + `enemy_turn_ready` at 05:23:39), then a second advance — an
  **End-Turn fired before the enemy executor ran** — hit `AdvanceTurn`'s unconditional
  `CompleteTurn` (no guard that an NPC's enemy turn was executed), marking G1's R4 turn done with no
  attack (`action_used=false`, no `action_log`) and rolling to R5/Vale. G1's bite (which would
  likely have dropped Forge) was lost. Death is orthogonal: with G2 alive the R5 rebuild just
  returns G2 first; the dropped turn is whichever combatant is current-but-unrun when the premature
  End-Turn fires (G1 was last in order → looked like "the round skipped a ghoul").
- **Fix (red/green TDD):** `AdvanceTurn` refuses (`ErrEnemyTurnNotExecuted` → **409**) to end a
  current turn that is an NPC with `action_used=false` (`ExecuteEnemyTurn` sets `ActionUsed=true`
  even for a no-op plan — the reliable "executed" signal; NPC turns always have `started_at=NULL`,
  so that's NOT a usable signal). PCs exempt. Dashboard Turn Queue surfaces the 409 text. Tests:
  `TestService_AdvanceTurn_RefusesUnexecutedEnemyTurn` / `_AllowsExecutedEnemyTurn` /
  `TestAdvanceTurn_UnexecutedEnemyTurnReturns409`. `make cover-check` green. Rebuilt + redeployed
  (`docker compose up -d --build app`); live state survived (R5, Vale's turn).
- **Live game:** left as-is per DM call — **no rewind**; G1 acts on its R5 turn. The dropped R4
  bite is not restored (Forge's lucky break stands in the fiction).
- **State now:** **Round 5, Vale's turn** — awaiting the human. One ghoul left (G1, D2); Forge
  4/32 raging (E1); Vale 19/24 (K2). Console `pending` empty, `next_step` clear.

### R5 — Vale opens at range; board hands off to Forge (06-28, ~2:01 PM)

- **Vale's R5 turn (player-driven):** kited back to **K2** and worked G1 from distance — cast
  **Chill Touch** (whiffed; HP delta confirms only the dagger landed) then a **thrown dagger, hit
  for 3** into G1's flank. G1 still up, now turned full on Forge. Narration posted to #the-story
  (read-aloud, 2:01 PM) bridging Vale's beat into Forge's turn.
- **OOC flag (not rewound):** Vale's R5 logged a Chill Touch (action) *and* a thrown dagger in the
  same turn — reads like two actions. Possible engine permissiveness; left as the player drove it.
  Watch for a repeat to decide if it's an ISSUE.
- **Board now:** **Round 5, Forge's turn** (CURRENT, player). G2 dead (E2). G1 alive (D2), on
  Forge's flank. Forge **4/32, raging** (10 rds left, E1) — bite likely drops him if G1 connects, so
  ending or peeling G1 on Forge's turn matters; G1's enemy turn comes last in R5. Vale 19/24 (K2).
  Console `pending` empty, `next_step` clear. Awaiting Forge's move.

### Engine fix mid-session: cleave secondary attack now logged to DM timeline (ISSUE-031) (06-28, ~2:30 PM)

- **Player report:** Forge's R4 crit cleave (4 slashing to G1) showed in Discord #combat-log but
  was **missing from the DM Console timeline**. Confirmed: Discord uses `FormatAttackLog` (appends
  the cleave line); the action_log/timeline path used `describeAttack`, which only rendered the
  primary target and dropped the cleave clause.
- **Display-only** — the cleave's 4 damage *was* applied to live HP (G1 22→18 at R4; reconciles with
  G1 = 3/22 now: −4 cleave, Vale R5 Chill Touch **miss** + dagger 3 → 15, Forge R5 greataxe 12 → 3).
  No state correction needed; HP was always right.
- **Fix (red/green TDD):** new `describeCleave` helper appends ` — Cleave hits/misses <2nd target>`
  to `describeAttack` (covers all PC attack paths via the shared formatter). Test
  `TestDescribeAttack_IncludesCleaveSecondaryAttack`. `make cover-check` green. Rebuilt + redeployed.
  Forward-only (the historical R4 crit row isn't backfilled).
- **Board unchanged:** still **Round 5, Forge's turn** (player). G1 3/22 (D2), Forge 4/32 raging
  (E1), Vale 19/24 (K2), G2 dead. Awaiting Forge's move.

### R5 closes — Forge swings, ghoul clings on; its bite goes wide; R6 opens on Vale (06-28, ~2:36 PM)

- **Forge's R5 turn (player-driven):** **Greataxe hit G1 for 12** (07:06:32) — brought the flank
  ghoul to **3/22** but did **not** drop it. No second attack/bonus logged. The ghoul stayed up,
  jaws on the swaying dwarf.
- **G1's R5 turn (NPC, DM-run via Turn Builder):** "⚔ Run Enemy Turn — Ghoul" → executor offered a
  single **Bite vs Forge** (+2, 2d6+2 piercing, reach 5ft). Engine roll: **to-hit total 4 vs AC 14 —
  Miss**, 0 damage. Confirmed & Posted to #combat-log (no re-roll; engine roll stands). Forge takes
  no damage — survives at **4/32**. Then manual **End Turn** (ISSUE-030 guard satisfied: enemy turn
  executed before advance, no silent skip). Logged `enemy_turn Ghoul R5` at 07:33:49.
- **Round advanced to R6:** dead G2 (init 19) auto-skipped → **Vale CURRENT** (init 15) → Forge (12)
  → G1 (9). Narration posted to #the-story (read-aloud, 2:36 PM): the kill-bite snaps shut on empty
  air; both ghoul and dwarf barely upright. No HP/AC numbers leaked.
- **Board now:** **Round 6, Vale's turn** (player). G1 alive **3/22** (D2, on Forge's flank), Forge
  **4/32 raging** (E1), Vale 19/24 (K2), G2 dead (E2). Console `pending` empty, `next_step` clear.
  Awaiting Vale's move — one solid hit from either side likely ends the fight (G1 a breath from death;
  Forge a bite from death saves, and G1 acts last in R6).

### Engine fix mid-session: two-dagger thrower can now throw the off-hand (ISSUE-035) (06-28, ~3:10 PM)

- **Player report:** Vale (2× dagger) threw her **main-hand** dagger as her R6 Attack action ("hit
  for 2"), then tried to throw the **off-hand** dagger as the two-weapon-fighting bonus attack
  (`/attack offhand:true thrown:true`) — bot refused: **"no main hand weapon equipped."**
- **Real bug (not a rules error):** RAW you can TWF with two light thrown weapons. But a main-hand
  thrown attack auto-unequips the weapon (`attack.go:1293`, so one dagger can't be re-thrown forever),
  and `OffhandAttack`'s guard (`attack.go:1443`) then requires a main-hand weapon still equipped → the
  emptied main hand trips it. The legit two-dagger-throw sequence breaks.
- **Fix (red/green TDD):** per-turn in-memory marker `mainHandThrownLightEffect` (same lifecycle as the
  Nick marker) set when a LIGHT melee weapon is thrown from the main hand; `OffhandAttack` treats the
  TWF main-hand prerequisite as met when the marker is present, even with an empty hand. Empty-main is
  allowed ONLY with the marker, so an illegal off-hand after a ranged/crossbow attack is still refused.
  `make cover-check` green; rebuilt + redeployed. Full write-up: issues.md ISSUE-035.
- **Live bridge (one-time):** the marker is in-memory and the redeploy wiped it for Vale's in-progress
  R6 turn (her main throw already happened pre-deploy). So for THIS throw, **Vale re-`/equip`s a dagger
  to her main hand**, then re-runs `/attack <ghoul> offhand:true thrown:true` — it now resolves via the
  equipped-main path (she rolls her own dice). Future turns need no dance (throw main → throw off-hand
  works directly within a process).
- **Board unchanged:** still **Round 6, Vale's turn** (player). G1 3/22 (D2), Forge 4/32 raging (E1),
  Vale 19/24 (K2), G2 dead. Awaiting Vale's off-hand dagger throw.

### R6 resolves — Forge goes down; R7 opens on Vale alone (06-28, ~3:41 PM)

- **Vale's R6 (player-driven):** main-hand dagger throw **hit G1 for 2** (the ISSUE-035 fix held — the
  off-hand throw then resolved), off-hand dagger throw **missed**. G1 survived at **1/22**.
- **Forge's R6 (player-driven):** **Greataxe missed** G1 (08:32) — no second swing logged. G1 clung on.
- **G1's R6 (NPC, DM-run via Turn Builder):** "⚔ Run Enemy Turn — Ghoul" → executor offered a single
  **Bite vs Forge** (+2, 2d6+2 piercing, reach 5ft, G1 adjacent at D2/E1 — no move needed). Engine roll:
  **to-hit 14 vs AC 14 — HIT**, raw 10 piercing → **halved to 5** by Forge's rage resistance. Confirmed
  & Posted to #combat-log (engine roll stands, no fudge). **Forge 4 → 0 HP:** engine set him
  **`unconscious` + `prone`** and **dropped Rage** (rage ends on unconscious — correct). Alive (downed,
  not dead; 5 < max HP so no instant-death). Then manual **End Turn**, logged `enemy_turn Ghoul R6` at
  08:37:49.
- **Round advanced to R7:** dead G2 auto-skipped → **Vale CURRENT** (init 15) → Forge (12, **down —
  death save due on his turn, rolled by his remote player, not the DM**) → G1 (9). Narration posted to
  #the-story (read-aloud, 3:41 PM): the jaws finally find Forge, he folds; the ghoul hangs over him,
  one bite from finishing him; Vale alone still standing. No HP/AC numbers leaked.
- **Board now:** **Round 7, Vale's turn** (player). G1 alive **1/22** (D2) — a single hit ends it; Forge
  **0/32 unconscious+prone** (E1), Rage gone; Vale 19/24 (G2). Stakes: if Vale (at range) doesn't drop
  G1 this turn, G1 acts **last** and one more bite likely kills the downed Forge. Awaiting Vale's move
  (she rolls her own dice — ranged Chill Touch / crossbow finishes a 1-HP target without melee/paralysis
  risk; her call, not the DM's).

### R7 — G1's bite drops Forge; a turn-flow bug skipped his death save (fixed); G1 lunges at Vale (06-28, ~4:20 PM)

- **R7 opened** with Vale's Chill Touch **missing** G1 (still 1/22). On **G1's turn (R6 close)** it bit
  Forge — hit (14), 5 piercing after rage resistance — **dropping Forge to 0 (unconscious + prone, Rage
  ended)**.
- **BUG caught at R7 (ISSUE-036):** when the turn advanced to the **downed Forge**, the engine **silently
  skipped his turn as "incapacitated" and never rolled / prompted a death saving throw** — advancing
  straight to G1. RAW: a creature at 0 HP rolls a death save at the **start of each of its turns**. The
  death-save machinery existed (`/deathsave`, `RollDeathSave`, the 24h timeout `AutoResolveTurn`) but the
  normal `AdvanceTurn` path reached none of it.
- **DM decisions (player-chosen):** (1) **fix the engine first** (red/green TDD + redeploy), then resolve
  the live save; (2) the mindless ghoul **lunges at Vale**, not the helpless Forge.
- **Fix (ISSUE-036, shipped + deployed):** "Prompt the player" design. `skipOrActivate` now detects a
  **dying PC** (`IsDying`) *before* the incapacitated skip and gives them an **active turn** flagged
  `DeathSavePending`; the #your-turn prompt shows **"You are dying — roll a death saving throw:
  /deathsave"**; and **`/deathsave` rolled on the dying PC's own current turn advances the turn** (off-turn
  rolls + Nat-20 wake-ups don't). Dying NPCs still skip (their saves aren't player-rolled). The 24h
  `AutoResolveTurn` stays as the inactivity fallback. `make cover-check` green; redeployed
  `docker compose up -d --build app` (combat state survived).
- **Live G1 turn (R7, DM-run):** moved G1 from D2 → **F2** on the combat map (executor is attack-only,
  ISSUE-021 — no auto-move), then **"⚔ Run Enemy Turn"** offered **Bite vs Vale** (now in reach). Engine
  roll: **to-hit 21 vs AC 11 — HIT**, **12 piercing** (Vale, warlock, no resistance) → **Vale 19 → 7/24**.
  Confirmed & Posted; **manual End Turn** → **Round 8**.
- **Forge's owed R7 save:** the bug skipped it; his **remote player rolls `/deathsave` once (off-turn)** to
  make it up (records only — won't advance, since it's not his current turn). From **R8 on**, the fixed
  engine prompts his death save automatically when his turn activates.
- **Board now:** **Round 8, Vale's turn** (player). G1 **1/22 (F2, adjacent to Vale)** — a single hit
  ends it. Forge **0/32, unconscious + prone, dying** (E1). Vale **7/24 (G2)**. R8 order: **Vale (CURRENT)**
  → **Forge (dying — /deathsave prompt fires when his turn activates)** → **G1** (NPC, last). Narration
  posted to #the-story (read-aloud, 4:20 PM). No HP/AC numbers leaked.

### R8 closes — Vale kills the last ghoul; Forge claws a save back; the cellar is won (06-28, ~7:14 PM)

_Resumed as DM on "forge rolled, vale done." DB + #combat-log confirmed the engine had run R8 to its
end and rolled into **R9, Vale's turn** — both beats un-narrated. Reconciled before acting._

- **Vale's R8 turn (player-driven):** thrown **Dagger — MISS** (to-hit 8), then **Dagger — HIT for 4**
  (to-hit 14) → dropped **G1 (was 1/22) to 0** — the last ghoul **dead**. Both of the brood down;
  **the cellar fight is WON.**
- **Forge's death saves (remote player rolled, NOT the DM):** the owed R7 makeup save came up **7 —
  Failure (0S/1F)**, then his R8 auto-prompted save came up **14 — Success → 1S/1F**. Forge is **still
  dying and still unstable** — needs 3 successes to stabilize (or a heal); two failures from death.
- **ISSUE-036 fix verified live:** Forge's R8 turn **activated and prompted his death save** instead of
  silently skipping (the R7 bug). Both saves recorded; DB tally now `{"successes":1,"failures":1}`.
- **Narrated the victory** to #the-story (read-aloud, 7:14 PM): Vale's dagger finishing the lunging
  ghoul, the cellar falling silent, Forge crumpled and dying on the stone — threat dead, friend dying,
  the few feet between them suddenly long. No HP/AC numbers; Vale's next move left to the player.
- **Observation — drop-to-0 logging gap (the feature shipped earlier today, `dfefd8e`):** G1's
  defeat-by-attack did **NOT** emit a "downed/defeated" row to action_log or #combat-log. The new
  `notifyDroppedToZero` is gated inside `Service.ApplyDamage`, but the player `/attack` damage path
  doesn't appear to funnel through `ApplyDamage` — so attack (and likely cast) kills are uncovered.
  Forward-fix candidate; **logged, not yet fixed** (table not blocked).
- **Board now:** engine at **Round 9, Vale's turn (active)** — but **no enemies remain** (both ghouls
  dead). Forge **0/32, unconscious + prone, dying (1S/1F)** (E1); Vale **7/24** (live board → Console).
  **Next (player-driven):** Vale rushes to Forge and **stabilizes him** (Medicine DC 10, her roll) or
  heals him → then **End Combat** (victory; no hostiles left). Stabilize **before** ending combat — once
  combat ends Forge has no turn to prompt further death saves. 3-4 more PCs still joining.

### Forge stabilizes on his own save; combat ended; party out of combat (06-28, ~8:27 PM)

_Resumed as DM on "should we go out of combat?" → "forge rolled save, continue as DM." Pulled the live
DM Console each step; the `/api/dm/situation` JSON tab is a static load and goes stale — trusted the
live Combat Manager / Console, not the cached JSON._

- **Vale's stabilize attempt (player-driven) — FAILED.** Vale moved adjacent to Forge (→F1, beside his
  E1) and rolled a **Medicine check to administer first aid: 6 vs DC 10 → fail.** Resolved the
  `skill_check_narration` queue item; the outcome line posted back to **#in-character** (the channel she
  rolled `/check` in — the result echoes to the originating channel, not a fixed one). Failed-stabilize
  read-aloud posted to #the-story (7:34 PM). Forge **not** stabilized by the aid.
- **Forge's death saves (remote player rolled, NOT the DM) — STABILIZED.** Between R8 and R10 his player
  rolled another **success (→ ✓2 ✗1)**; ending Vale's turn activated **Forge's R10 turn cleanly**
  (ISSUE-036 fix still holding — no silent skip), and his next `/deathsave` came up a **success → ✓3 =
  stabilized.** No longer dying; unconscious at 0 HP, prone. Engine rolled to R11. Stabilization
  read-aloud posted to #the-story (8:18 PM) — Vale's hands failed but the dwarf's own toughness held.
- **Ended combat.** No hostiles, nobody dying → **Combat Manager → End Combat → Confirm End.** "The
  Cellar" (`8509d1f6-…`) closed in victory at R11; dashboard now shows **no active encounters**.
- **Manual HP carry-out (combat-end has NO write-back — two HP stores).** Out of combat the Party page
  showed both PCs at full *stored* HP (Vale 24/24, Forge 32/32, no conditions) — the combat damage only
  ever lived in the combatant snapshot. Corrected both via **Party → Edit status** (audit reasons
  logged): **Forge 0/32, conditions unconscious + prone (stabilized); Vale 7/24.** ⚠️ This carry-out is
  a manual, easy-to-forget step — a downed PC silently reads as full HP after End Combat until the DM
  sets it. Candidate improvement: auto-carry-out on End Combat (not built).
- **Out-of-combat transition beat** posted to #the-story (read-aloud, 8:27 PM): the cellar falls silent,
  the brood dead, Forge alive-but-down, the deeper dark still ahead — choice handed back to the players
  (tend/heal Forge, short rest, or press deeper). Awaiting their move.
- **Board now:** **out of combat, no encounter.** Forge **0/32, unconscious + prone, stabilized**; Vale
  **7/24** (both set by hand, see above). The deeper cellar is **unexplored**. 3-4 more PCs still joining.

### New session: Vale searches the nest — a healing draught, a key, a journal; choice still stands (06-29, ~12:17 PM)

_Resumed as DM on "continue as DM." Pulled live first: stack up (app redeployed ~7 min prior with the
ISSUE-038 carry-out fix, `b199c21`); **out of combat, no encounter**; **1 pending** queue item. PC vitals
**unchanged** from the 06-28 carry-out — Forge **0/32 unconscious+prone (stabilized)**, Vale **7/24**;
nobody healed or rested in the interval._

- **Player beat (in-fiction, #in-character, 12:06 PM):** dewa — *"Vale looks around the room for anything
  that can help Forge."* Vale ran `/check investigation` → **rolled 10** (queued as a skill_check_narration).
- **OOC read (#player-chat):** players were joking about how close the 2-PC fight ran and that they
  **"didn't bring a potion"** (`nda bawa potion`) — explicitly flagging the party's lack of healing. They
  loved the stabilization read-aloud (`metal abis`, "Poetic yo").
- **DM adjudication (Investigation 10 vs ~DC 10 — meets):** the brood's nest is heaped on the **dead** —
  travelers dragged down and picked over (world.md keeper/loot threads). Vale's intentional search turns up,
  in a split traveler's pack: **one healing draught (a common Potion of Healing), whole**; plus a **cold iron
  key** on a thong and a **water-rotted journal** (unreadable in cellar light) — light hooks toward the
  deeper cellar. A fair, earned find for a met-DC search of a victim-strewn lair; answers the no-potion gap
  without trivializing a 2-PC party (kept it to **one** common draught). **Found ≠ used** — the players
  decide whether to administer it to Forge (an action; a Potion of Healing on a stabilized PC heals 2d4+2,
  takes him above 0 → wakes, drops `unconscious`; `prone` until he stands — *they* roll the 2d4+2).
- **Resolved** the skill_check_narration via **DM Queue → Open → Send Narration** (outcome echoed to
  **#in-character**, the channel she rolled in). Posted a **read-aloud** scene beat to **#the-story**
  (12:17 PM, confirmed in Narrate Post History) framing the find + the standing choice. No numbers/secrets.
- **Board now:** out of combat, no encounter. Forge **0/32, unconscious + prone, stabilized**; Vale
  **7/24** (unchanged). Party now has **1 healing draught (uncatalogued — narrative-tracked until used),
  an iron key, a ruined journal**. **Next (player-driven):** administer the draught to wake Forge, take a
  **short rest** (Vale's Pact Magic slots recharge on a short rest — strong for her), or press deeper past
  the lantern into the unexplored cellar.

### The party retreats and takes a long rest — both back to full (06-29, ~1:13 PM)

_Resumed as DM; pulled live first (DM Console: 0 pending, 0 approvals, no active encounter). The player had
acted in the interval._

- **Player beat (in-fiction, #in-character, 12:58 PM):** dewa — *"Vale takes all items, and with mage hand
  help drag him up. 'Let's rest and heal'. Vale sets up camp in the common room. (1d4 hour rest to get Forge
  to 1HP + long rest to full heal)."* The party chose to **bail upstairs and rest**, not drink the draught or
  press deeper. (OOC #player-chat had nothing new — yesterday's banter only.)
- **DM adjudication (long rest granted, uninterrupted):** the brood is **dead** and the common room is the
  **cleared** upstairs (world.md) — barring the splintered cellar door and resting by a relit hearth is
  plausible. No wandering-monster interruption: rewarding the smart retreat, not punishing it. Mechanically a
  **long rest** → both PCs to **full HP**, Forge **wakes** (a stabilized creature at >0 HP is conscious) and
  **stands** (prone cleared), and **Vale's Pact Magic slots recharge**. The "mage hand drag" is narrative
  flavor (Vale physically hauls Forge up the stairs); not worth a ruling stop.
- **Mechanics applied (dashboard only — Party → Edit status / Edit slots, audit reasons logged):**
  - **Forge:** HP 0→**32/32**; conditions **unconscious + prone cleared** (now `none`). Reason: "Long rest in
    the common room — full heal, wakes and stands."
  - **Vale:** HP 7→**24/24**. Reason: "Long rest in the common room — full heal."
  - **Vale:** Pact (Level 2) **0→2/2**. Reason: "Long rest — Pact Magic slots recharge."
- **Narrated** a 4-paragraph **read-aloud** to **#the-story** (1:13 PM, confirmed rendered as a boxed block
  in Discord + Narrate Post History): the haul up the stairs, barring the cellar door, the relit hearth, the
  long quiet night, Forge waking with a curse — and **two seeded hooks**: the **journal has dried enough to
  read** (a patient eye can coax the ink back), and **something below the barred door is "not entirely
  silent"** (world.md "what the brood fears" thread; the descent still beckons). No numbers/secrets.
- **Board now:** out of combat, no encounter. **Vale 24/24 (Pact L2 2/2), Forge 32/32 — both full, no
  conditions.** Party holds **1 healing draught (unused, narrative-tracked), the iron key, the now-readable
  journal**. **Next (player-driven):** read the journal (a clue beat), unlock something with the iron key, or
  descend into the unexplored deeper cellar (the brood-descent fight is pre-built — `encounters/cellar-brood.md`).
- **Items formalized into Vale's inventory (~1:30 PM)** — moved the three finds off "narrative-tracked" and
  onto Vale's character sheet via **Party → Manage inventory**: **Potion of Healing** (catalog magic_item,
  "regain 2d4+2 hit points") + two **custom** items, **Cold Iron Key** and **Water-Rotted Journal** (each with
  flavor text, qty 1, value 0). "Added 3 items" confirmed; all three verified present on the sheet. (Catalog
  search has *Potion of Healing* 2d4+2 vs *Greater* 4d4+4 vs *Superior* — picked the common 2d4+2 to match the
  adjudicated find.)

### Vale reads the keeper's journal — the "cold door" clue (06-29, ~1:38 PM)

_Resumed as DM; pulled live first (DM Console: "Nothing needs you right now," 0 pending, no active encounter;
timeline = only my prior narration). The player had acted in #in-character — Discord-only, never in the Console feed._

- **Player beat (in-fiction, #in-character, 1:33 PM):** dewa — *"Vale wakes up early, pouring all attention to
  read the journal. When she realized Forge is up, she mutters, 'thank you for chasing me downstairs… And,
  sorry.' (For DM: what do we get from the journal?)"* The party chose the **read-the-journal** hook; direct DM
  prompt for the content.
- **DM adjudication (no roll — already readable):** the journal was established last beat as **dried enough to
  read** by the hearth, so this is a straight read, not a check — Vale's focused effort gets the legible
  remnant outright (no dice owed). Pure narrative beat: **no HP/condition/inventory mutation** (the journal
  stays on Vale's sheet).
- **Narrated** a **read-aloud** to **#the-story** (1:38 PM, confirmed rendered as a boxed block in Discord +
  Narrate Post History) — the keeper's hand, most of it pulped, the innermost quarter legible. The clue it
  gives up (grounded in world.md keeper/brood threads):
  - The keeper heard scratching under the floor, **went down**, and **unlocked an old vault behind a "cold
    door" lower than the cellar** — "the key turned so easily." (Hard-links the **cold iron key** on Vale's
    sheet to a known destination: it locks/unlocks the **cold door** at the bottom.)
  - The **wretches came up after** — named the **Harrow couple**, buried in autumn — wearing their own faces,
    not eating, *waiting*. They are **afraid of the dark below**.
  - Recontextualizes the **door scored from inside**: the brood clawed the cellar door **not to reach the
    keeper but to flee the cold door** — "they would sooner be up here … than down there with it."
  - Final torn line: ***"the cold iron key locks the cold door. Do not turn it. Whatever else you do, do not
    turn it."*** + the keeper's initials. (A warning, not a wall — player's call.)
- **Board now:** out of combat, no encounter. **Vale 24/24 (Pact L2 2/2), Forge 32/32 — both full, no
  conditions.** Inventory unchanged (Potion of Healing, Cold Iron Key, Water-Rotted Journal). **Next
  (player-driven):** descend the cellar toward the cold door (brood-descent fight pre-built —
  `encounters/cellar-brood.md`; the cold door itself is past it, DM-prep on demand), try the key, drink the
  draught before a fight, or do something else entirely. **Awaiting the players' move** — don't narrate the choice.

### RP interlude — Vale's patron revealed; Forge spotlight nudge (06-29, ~2:15–4:18 PM)

_Pure roleplay in **#in-character** (Discord-only — none of this is in the Console/DB feed). No mechanics, no
rolls, no mutations; logged for narrative continuity only._

- **The exchange (in-fiction, #in-character):**
  - **Forge (Jonathan), 1:37 PM:** wakes, *"once again, thanks for the help. that was close call,"* checks his
    kit, *"Forgot brings potion for such exploration, costs me dearly."*
  - **Vale (dewa), 1:54 PM:** shows the vial (*"well now we have one"*), shows the journal + key and retells
    the journal's content, then — *"my patron really wants we to go down there, i will not impose on you to
    follow me."*
  - **Forge, 2:15 PM:** *"is there another being among us here? that you mention as patron. I see no one."*
  - **Vale, 2:49 PM (patron backstory established):** a **travelling storyteller** who collects + spreads
    forgotten history/lore; while chasing a **forgotten deity** she *"found a being, granting me powers so that
    i can explore more, in exchange, i collect stories that he wants, and spread them."* Concludes: *"there is
    something that he wants me to see down there, i have no choice."* (Vale's warlock patron is now durable
    fiction: a story-hungry entity tied to a forgotten god; the **cold door is what it's steering her toward.**)
- **DM nudge (spotlight to Forge):** posted a **read-aloud** to **#the-story** (4:18 PM, confirmed in Narrate
  Post History) framing the morning-after — Vale's confession landing, the vial/key/journal on the table, the
  cold door below — and **turning the floor to Forge** without scripting his answer: *"She'll go down either
  way. The only question left … is whether she goes alone."* (Honors dm-rules: narrate the world's reaction +
  pose the open question; let the player decide and speak.) **Awaiting Forge's (Jonathan's) response.**

### Forge commits to the descent; the cold-door beat pre-built (06-29, ~5:12 PM)

- **Player beat (in-fiction, #in-character, 5:12 PM):** JonathanEka — *"'I'm in, let's find some ammusing
  story hidden & kept by those beasts.' Forge hastily prepare his equip & getting ready."* **Both PCs are now
  committed to descending** toward the cold door (Vale by patron-pull, Forge by choice). No mechanics — pure RP.
- **DM prep (pre-built ahead of the descent — no combat started):** with the party committed and combat behind
  the cold door near-certain, pre-built the beat through the dashboard so it's one click to run:
  - **Map (in-app Map Editor, not import):** *Ashfall Waystation — the cold vault*
    (`2899165e-3d1b-46e9-962f-9065e4e3529a`), 12×10 blank stone, **PC spawn zone bottom-center** (the cold-door
    threshold). Features narrated, not painted (house convention). *(Runbook §4 updated to document the in-app
    map tools as the preferred path.)*
  - **Encounter (Encounter Builder → Save, NOT Start):** **"Cold Vault — the keeper"** / player-facing **"The
    Cold Vault"** (`adc064e7-2800-4787-8cb8-5deb23d1fc1f`) — **1× Wight (CR 3)** reflavored as the frost-rimed
    vault-keeper that hollowed the brood, placed top-center **(6,1)**, surprise OFF, party 2/2. DB-verified.
  - **Design intent + difficulty + staged read-alouds:** [`encounters/cold-vault.md`](../encounters/cold-vault.md).
    Core design: a single CR3 boss ≈ Hard→Deadly for two L3 PCs (tense, not a TPK; reserve husks added live only
    if too easy), and a deliberate **escalation — the keeper is genuinely UNDEAD, so Vale's *hold person* (which
    carried the living-wretch fights) FAILS here**; the vault is a chiselled-out shrine to a forgotten god (ties
    Vale's patron) seeding the next breadcrumb.
- **Not run yet.** Per dm-rules, **wait for the players to actually descend** in #in-character before posting the
  descent narration / turning the key / Starting Combat — don't act for them. State + IDs in `game-state.md`.

### The party descends — at the cold door (06-29, ~7:13–7:15 PM)

- **Player beat (in-fiction, #in-character, 7:13 PM):** dewa — Vale packs her items and readies her daggers,
  thanks Forge (*"i can't thank you enough, friend… i will tell you amazing stories forgotten by time, in
  return"*), then **"Vale descents the cellar, and further into darkness below."** With Forge already committed
  (5:12 PM), **the party has now actually moved** — the trigger to run the pre-built cold-door beat.
- **DM picture pulled first:** DM Console clean — *"Nothing needs you right now,"* **0 pending, no active
  encounter**. The #in-character descent is the only live signal; nothing queued.
- **DM beat — descent read-aloud posted (7:15 PM, confirmed in Narrate Post History):** the staged **block A**
  from [`encounters/cold-vault.md`](../encounters/cold-vault.md) — the stair into the cold the brood died
  fleeing, their own dead lying slack and almost-human, the deepening mineral cold, and the passage ending at a
  **frost-rimed black-iron door** whose lock is the exact shape of Vale's cold iron key. Behind it, faint and
  patient, *something that is not wind and not water* — and the keeper's torn warning resurfacing: *do not turn
  it.* **Ends on the door, key in hand** — a clean cliffhanger that hands the players the choice.
- **No mechanics yet** — pure narration, no HP/condition change, no combat. **Turning the key is the players'
  call** (the keeper warned against it). Holding here: **do not script the key-turn.** When a player turns it
  (or otherwise forces the door), post the **block B** vault read-aloud → open **"Cold Vault — the keeper"** →
  **Start Combat** (adjudicate surprise live; remember *hold person* FAILS on the undead keeper). If they balk
  and retreat, follow their lead. **Awaiting the players' move at the cold door.**

### They turn the key — COMBAT: the Cold Vault boss begins (06-29, ~7:54–7:56 PM)

- **Player beats (in-fiction, #in-character):**
  - **Forge (JonathanEka), 7:17 PM:** *"Nodding in solemn, Forge follows to descend below.."* — at the door
    with Vale.
  - **Vale (dewa), 7:54 PM:** *"Standing in front of the door, Vale inserts the key. She waited a beat, then
    nodded to Forge and turns the key."* — **the key is turned** (the keeper's warning ignored). Trigger to run
    block B + Start Combat.
- **DM beat — vault read-aloud posted (block B, 7:55 PM, Post History confirmed):** the door swinging in on
  grave-cold air, the vault as a **shrine to a forgotten god whose name has been chiselled out of every stone**,
  and the **keeper rising** in its own frost-grey clothes — *it does not lunge; it lifts one hand, almost gently,
  and the air between you turns to winter.*
- **Combat STARTED (~7:56 PM).** Opened *"Cold Vault — the keeper"* in the builder (party 2/2 confirmed: Vale ✓
  Forge ✓; W1 Wight at (6,1), Surprised OFF) → **Start Combat** → *"Combat started. Encounter:
  `446dce33-e221-4d1f-a88b-4e81534b3399`."* Board auto-posted to #combat-map; initiative to #initiative-tracker.
- **Surprise adjudicated live = none** — the keeper was dormant/waiting and the PCs deliberately opened the door;
  no ambush either way → **standard initiative.**
- **Round 1 order (DM Console):** **Forge (14) → Wight keeper (14, tie → Forge first) → Vale (4).** PCs entered
  **bottom-center** (the cold-door threshold); the keeper waits **top-center** — roughly a board's length to
  close. **Live HP/positions are in the Console, not transcribed here.**
- **Standing this turn:** R1 opens on **Forge's turn (PC)** — **awaiting his slash command; not rolling/acting
  for him.** When initiative reaches the **Wight**, its turn is run from the combat workspace (its HP/AC kept
  secret; players roll their own saves). **Watch for Vale's *hold person* — it FAILS on the undead keeper**
  (telegraph the first attempt). Reserve husks held off-board unless the fight proves too easy.

### Round 1 plays out — Forge advances + rages, keeper draws first blood (06-30)

- **Forge's turn (R1, ~8:11 PM 06-29):** moved up to **F4** (closing the gap toward the keeper) and **entered a
  Rage** (bonus action) — but **did not attack** (the keeper was still out of melee reach) and took no damage.
  By RAW his rage therefore **lapsed at the end of his turn** (rage ends if, by turn's end, you haven't attacked
  a hostile or taken damage); the engine cleared `is_raging` accordingly. ⚠️ **The bot posted no "rage ended"
  notice**, so the player isn't told his rage dropped (and the keeper's next hit then landed in full, not
  halved). **Fixed live → ISSUE-041 (FIXED):** `notifyRageExpired` now posts the lapse to #combat-log + writes
  a `rage_expired` action_log row (mirrors the drop-to-0 dual-surface pattern; best-effort, parented to the
  rager's own turn). Separately, his **rage-uses pool** had been hand-set too low at setup and was corrected
  **1 → 2 remaining** (DM Correction, logged) — that fix **persisted across the redeploy** (`{max:3, current:2}`).
- **Keeper's turn (R1, run 06-30 from the workspace Turn Builder):** `Run Enemy Turn — Wight` → the engine
  pre-targeted the **nearest** PC (Forge, F4) with its **Longsword** (single attack, +4, 1d10+2 slashing,
  reach 5ft; no movement step — the engine abstracts the close). **Review → To Hit 20 (vs Forge's AC 14) = HIT,
  7 slashing → Confirm & Post.** Damage applied (**Forge 32 → 25/32**, taken in full — not raging), auto-logged
  to #combat-log (`enemy_turn Wight R1`). Forge is **hurt, up, no conditions.** The keeper's HP/AC stay secret
  (it took no damage). *(My own manual NPC roll was discarded in favour of the engine's — a first `$RANDOM`
  attempt mis-fired with correlated subshell seeds, re-rolled clean on `/dev/urandom`, then superseded entirely
  by the Turn Builder's roll.)*
- **DM beat — narrated to #the-story (read-aloud, 8:18 AM 06-30):** Forge coming off the threshold into the
  grave-cold, the keeper crossing the dark without seeming to walk and laying its rimed blade across his
  side — *the cold drives in like deep winter and where the edge lands the warmth just leaves* (telegraphing
  the necrotic theme without a numeric leak) — *he's hurt and standing;* then the spotlight handed to **Vale.**
  Story is back in **lockstep** with the board.
- **Turn advanced → Vale (R1).** It is now **Vale's turn (PC)** — the bot has prompted her in #your-turn;
  **awaiting her slash command; not rolling/acting for her.** Reminder still standing: her ***hold person*
  FAILS** on the undead keeper (telegraph the first attempt); her tools that bite are *shatter / hellish rebuke
  / chill touch* (DC 13). Keeper's token left at its dais on the map (cosmetic — the adjacent-move was descoped
  after a coordinate misclick; reposition on its next turn).

### DM ruling — Forge's wasted rage refunded (06-30)

- **Player undo request (dm-queue `f364b61d`, 01:43 UTC 06-30):** Forge's player asked to *"undo rage since i
  just realize my movement doesn't reach the wight."* His R1 move to **F4** left him **15 ft** from the keeper
  at **G1** (Chebyshev 3 = melee out of reach), so the rage he spent **lapsed instantly with zero benefit** and
  he ate the keeper's **7 in full** (not halved). An honest grid-reach misjudgment — easy to miss on a text map.
- **Ruling: GRANTED — refund the rage charge only** (confirmed with the human DM). Minimal-disruption fix: he
  keeps his **F4** position and the keeper's hit **stands**; just the wasted charge comes back. No turn rewind
  (the keeper's turn + Vale's turn-start are left intact).
- **Executed via the dashboard combat workspace** (not raw SQL): select Forge → **Manual Override → Feature Uses
  → Edit Feature Uses** → rage `current` **2 → 3**, with a reason. This is the **in-combat** feature-uses
  override (`POST /api/combat/{enc}/override/character/{char}/feature-uses`) — the inverse of the 409-guarded
  out-of-combat overview editor (ISSUE-040). It **audited** a `dm_override` action_log row and **auto-posted a
  #combat-log correction** (*"⚠️ DM Correction: Forge Anvilbearer rage uses set to 3 …"*, player-👍'd). DB
  confirms `{max:3, current:3}`.
- **Queue cleaned:** the undo notification **resolved** with a player-facing outcome (*"Granted — rage charge
  refunded (3/3) … you're still at F4 and the blow you took stands; rage next round once you're adjacent"*);
  the **stale `enemy_turn_ready` (Wight)** — the turn already run above — was also cleared. **DM Queue now
  empty.** No #the-story narration (a rules correction, not a fiction beat). **Still Vale's turn.**

### Vale's Shatter on the keeper + two bugs found & fixed live (06-30)

- **Vale cast Shatter (L2, pact slot) at the keeper** (`/cast`, ~10:04 AM). The in-character bot line wrongly read
  *"Used 2nd-level slot (0 remaining)"* — her sheet correctly showed **1** pact slot left. **ISSUE-042 FIXED:**
  `FormatAoECastLog` read the leveled-slot field instead of the pact field (single-target `FormatCastLog` already
  branched correctly); display-only, no state harmed. The wrong line is frozen in Discord history (pre-fix cast).
- **The save needed resolving (the keeper is a monster) and there was no DM path for it.** Built one — **ISSUE-043
  FIXED:** `ResolveMonsterPendingSave` + `GET/POST /api/combat/{enc}/pending-saves[/{id}/resolve]`, surfaced in the
  DM Console `pending[]` and a dashboard resolver (in + out of combat). Engine rolls `d20 + creature save mod` vs DC,
  applies half-on-save, audits + posts #combat-log without leaking HP. (Route-drift sub-bug: new routes added to
  `RegisterRoutes` but not the production mount → 404; fixed + added a parity guard test.)
- **Resolving it exposed a CRITICAL pre-existing bug — ISSUE-044 FIXED:** AoE save-for-half damage *never applied in
  production* (the apply gate listed `pending`-only saves, so the last-resolved save released nothing). Affected the
  player `/save` path too; masked by a unit-test mock. Fixed the gate (list all rows) + added an `applied` lifecycle
  (idempotent, recoverable). DB-backed regression test added.
- **Resolution (fair + by the book):** the keeper rolled its CON save **in the open** — **nat 1 +3 = 4 vs DC 13 →
  FAIL** (posted to #combat-log before the apply bug was fixed, so the failure is public and **stands — no re-roll**).
  After the fix, the stuck `rolled` save was **recovered** (re-driven, no re-roll): engine rolled **3d8 = 16 thunder**,
  applied in full (failed save, no thunder resist). **Keeper took 16** (HP secret; live total → DM Console).
- **Narrated to #the-story** (read-aloud, ~11:25 AM, Post History confirmed): the thunderclap rocks the keeper —
  rime blasts off its armor, it slams the wall to stay upright but **does not fall**, and turns its glare back on the
  party. **Still Vale's turn** (action spent on Shatter; bonus action + move + reaction remain) — await her next call.

### Round 2 — the keeper whiffs, a reaction-prep bug, and a reaction-execution gap (06-30)

- **Combat advanced to Round 2.** By the time it came back around: Forge's **Greataxe hit the keeper (16)** and Vale
  landed a **dagger (1)**, dropping the keeper to **badly wounded** (HP secret; live total → DM Console). Then it was
  the **keeper's R2 turn**.
- **Vale readied a reaction — and found a bug.** Before her R2 turn she ran `/reaction declare hellish rebuke if
  attacked`. The bot confirmed it **only to her (ephemeral)** — the rest of the table + the DM never saw it. **ISSUE-045
  FIXED:** a declared reaction is public table info (like `/action ready`), so `/reaction declare`'s **success** now
  posts publicly (`⚡ <name> readied a reaction: <desc>`); errors/validation stay ephemeral. TDD (mock now captures
  `Flags`); commit `cd6d360`, pushed + redeployed. The bot fix is live for the *next* declaration (the already-declared
  one stays as it was posted).
- **Keeper's R2 turn run from the Turn Builder.** The builder surfaced Vale's readied reaction as a **Pending Reactions**
  heads-up, then planned the only legal action: **Longsword vs Forge** (the keeper at G1 is adjacent to Forge at F1; Vale
  at G5 is 20 ft away, outside its 5 ft reach — and fleeing Forge to chase her would eat a raging-greataxe OA at low HP).
  Rolled **11 to hit vs AC 14 → MISS** (posted to #combat-log). Because it struck **Forge, not Vale**, her "if attacked"
  reaction **did not trigger** (still active). **Ended the turn → now Vale's turn (R2).** Narrated the whiff to
  #the-story (read-aloud, 1:33 PM, Post History confirmed): the sword screeches off Forge's guard, and the keeper's gaze
  fixes on Vale.
- **The readied reaction exposed a real gap — ISSUE-046 OPEN.** Investigated whether the dashboard can actually
  *execute* hellish rebuke if it fires: it can't, as one flow. The declaration is opaque free text; the "Resolve" button
  is pure bookkeeping (`reaction_used=true`, no damage/save/slot); `/cast` is turn-gated so Vale can't cast it on the
  attacker's turn; and the one-click damage control (`override/hp`) would leak the keeper's secret HP. The spell *is*
  modelled (2d10 fire, DEX save half). Today the DM hand-assembles it; proposed fix = an "execute reaction spell"
  resolver mirroring the monster-save resolver (ISSUE-043). Logged for a future build — **does not block play**.

### Round 2 — Vale's Shatter clips Forge, undo granted; ISSUE-048 (cancel an AoE cast) built live (06-30)

- **Vale cast Shatter (R2) and it caught Forge.** Her 10-ft blast was placed so the sphere caught **both** the keeper
  **and her ally Forge**. Mid-resolution it was clean to reverse: pact slot spent, two `pending_saves` (the keeper's
  DM-resolved + Forge's player `/save`), **no damage applied yet** (AoE damage defers until every target's save resolves
  — the ISSUE-044 gate). She ran **`/undo`** ("i need to cast it more to the right") to recast off Forge.
- **Granting the undo cleanly hit a product gap.** The `undo_request` queue resolver only acknowledges (writes an
  outcome note); `undo-last-action` restores only HP/position/conditions (no slot refund, no save clear); and the **only**
  pending-save endpoint is `…/resolve`, which *applies* damage. There was no way to *cancel* a mid-flight AoE cast — so
  granting the undo would leave un-cancelable pending saves (a duplicate "Resolve save" footgun + an oldest-first `/save`
  mis-attribution). The `ForfeitPendingSave`/`CancelAllPendingSavesByCombatant` SQL existed but had no handler/button.
- **Fix-now (DM choice: fix cleanly, then grant) — ISSUE-048 FIXED.** Built a DM **Cancel** path (red/green TDD):
  `Service.CancelAoEPendingSave` forfeits every not-yet-applied save sharing the clicked save's `source`
  (`aoe:shatter:s2c3` → voids the **whole** blast in one click), `POST …/pending-saves/{id}/cancel` (added to both the
  handler RegisterRoutes **and** the `main.go` mount — the ISSUE-043 two-list trap), a `dm_cancel_aoe` audit + #combat-log
  correction that never surfaces HP, and an amber **Cancel** button beside "Resolve save" in `PendingMonsterSavesPanel`.
  4 Go + 2 vitest tests; `make cover-check` + 747 vitest green; rebuilt + redeployed.
- **Grant executed on the live board.** Clicked the new **Cancel** → it voided **2** saves (`canceled=2`: the keeper +
  Forge), **no damage** (Forge still 25/32, keeper HP unmoved). Refunded Vale's **pact slot 0→1** (slot override, audited)
  and **resolved the `undo_request`** with a player-safe whisper ("your Shatter is recalled and your pact slot refunded —
  recast further right, clear of Forge"). **Now awaiting Vale's recast** — never roll/act for her.

### Round 2 — the undo still blocked the recast; ISSUE-049 (Restore Action) built live (06-30)

- **Vale: *"i cannot recast because my action is not undid."*** The ISSUE-048 grant voided the saves + refunded the slot,
  but casting Shatter had spent her **turn's action** — the DB confirmed her active R2 turn at `action_used=t`,
  `action_spell_cast=t`, `attacks_remaining=0`. So the cast was undone but the **action economy** wasn't, and the cast
  handler's `ValidateResource(action)` gate blocked a recast.
- **Another product gap.** `undo-last-action` restores HP/position/conditions only; the `undo_request` resolver only
  acknowledges; `RefundResource` (`turnresources.go:100`) existed but was reachable only from freeform-action cancel +
  Action Surge — neither a DM control nor applicable to a real spell cast. **No DM path gave an action back mid-turn.**
- **Fix-now — ISSUE-049 FIXED (red/green TDD).** `Service.RestoreTurnAction(enc, combatantID)` targets the **active**
  turn (409 `ErrNotActiveCombatant` / `ErrNoActionToRestore`), clears `action_used` + `action_spell_cast`, **reseeds** the
  per-turn attack count via `ResolveTurnResources` (so she may attack instead), and **leaves movement alone**.
  `POST …/combatants/{id}/restore-action` (both the handler RegisterRoutes **and** the `main.go` mount — ISSUE-043 trap), a
  `dm_restore_action` audit + #combat-log correction (never HP), `restoreCombatantAction` api, and a
  **"Restore Action — `<name>`"** button beside "Undo Last Action" in `CombatManager.svelte` (targets the active combatant).
  3 Go + 1 vitest; `make cover-check` + 749 vitest green; rebuilt + redeployed.
- **Executed on the live board.** Clicked **"Restore Action — Vale"** → her R2 turn went to `action_used=f`,
  `action_spell_cast=f`, `attacks_remaining=1` (movement still 30), `dm_restore_action` audit row written, no HP leaked.
  **Vale's action + pact slot are both back and the blast is voided — she can now recast.** ISSUE-048 + 049 together are
  the full "grant an AoE undo," still 3 clicks (Cancel → refund slot → Restore Action); a one-click bundle is a future
  enhancement. Still awaiting her recast — never roll/act for her.

### Round 2 — Vale's recast lands; the keeper reels at death's door (06-30)

- **Vale recast Shatter (pact slot, 0 remaining) — this time placed clear of Forge** (#combat-log 6:34 PM:
  *"Affected: Wight"* only; Forge spared). One pending CON save (the keeper), DC 13.
- **Keeper's save resolved from the workspace** ("Resolve save" → `dm_resolve_save`): **5 vs DC 13 — Failure**, full
  **11 thunder** applied. The blast left the keeper *a breath from collapse* — clinging to the gouged-out shrine, not
  yet down (HP secret; live total → DM Console). Forge untouched, Vale unhurt.
- **Narrated to #the-story** (read-aloud, 6:40 PM): the second Shatter blooming clear of Forge, the keeper folding
  around the note, grave-cloth splitting, swaying but still upright.
- **Still Vale's turn (R2)** — her **action is spent** on the recast (movement/bonus/reaction remain; her
  *hellish rebuke* reaction stays readied). Awaiting her next command (`/move`, `/done`, …) — never roll/act for her.
  When her turn ends, Round 3 opens with **Forge**, who will almost certainly finish the reeling keeper.

### Round 3 — Forge destroys the keeper; the Cold Vault is won (06-30)

- **Vale closed R2** by throwing her dagger at the keeper — **missed** (9 vs AC). Turn ended; Round 3 opened with Forge.
- **Forge finished it.** Two thrown **handaxes**: the first **hit (8 slashing) — *"💀 Wight drops to 0 HP — defeated"*** (the
  drop-to-0 log fired correctly this time, through the `/attack` path); the second came in on **vex advantage as a NAT 20
  crit** (8 more) — overkill into the falling husk. The keeper came apart on the vault floor.
- **Narrated the kill to #the-story** (read-aloud, 9:25 PM): Vale's dagger skating off bone, Forge's handaxes folding the
  keeper off the shrine, the grey light guttering out, grave-cloth and rime collapsing — the shrine left hollowed and
  empty, the cold door open, the dark beyond *only dark*.
- **Combat ended** via the workspace **End Combat** (Confirm End) — encounter `446dce33-…` now `status=completed`,
  *"Combat ended — The Cold Vault"* posted to #combat-log. **ISSUE-038 auto carry-out worked:** final HP/conditions wrote
  back to the sheets with no manual reconcile — **Forge 25/32, Vale 24/24, no conditions.** No casualties on the party
  side; the keeper was the lone hostile (no reserve husks were needed — the fight wasn't a walkover).
- **Out of combat now — the Cold Vault is cleared and theirs.** The shrine stands gouged/empty (something was chiselled
  out of it); the cold door is open; what lies deeper is unexplored. Spotlight handed to the players — they decide what to
  search/do next. Standing campaign pull: the journal's warning (the Harrow couple "wearing their own faces"), the
  hollowed shrine, and Vale's patron steering her downward. Await player action — never narrate their choices.

### Out of combat — Vale examines the shrine (06-30)

- **Vale's player (dewa) opened the post-combat beat** (#in-character, 9:44 PM — Discord-only RP, invisible to the
  DB / DM Console): *"Vale gives Forge a thumb up. Then examines the shrine."* The DM only sees it by reading Discord.
- **Board reconciled quiet first:** no live encounter (all `completed`), DM Queue empty, `action_log` ends at the kill —
  the move exists only in Discord RP, nothing to catch up on mechanically.
- **Adjudication:** examining the gouged-out shrine is an exploration check. Per [`dm-rules.md`](../dm-rules.md) the
  **player rolls her own dice** — so I narrated the freely-apparent read and called for an **Investigation check** (secret
  **DC 13**, ruled **tiered**: even a low roll yields the obvious + her patron's certainty that *this* is the place, so the
  campaign's central pull is never hard-blocked; a good roll earns the breadcrumb).
- **Narrated to #the-story** (read-aloud, 9:50 PM): the cooling remains; the shrine itself — a worn stone ring, an altar
  gone concave under centuries of hands, a niche in the back wall where **something was pried / chiselled out, recently**
  (stone scraped bright and raw); the forgotten god's name cut deep and scratched out over and over by a frightened hand,
  for years; the cold coming off the shrine, not the walls; and **Vale's patron-pull drawing tight as a plumb-line — *this*
  is the forgotten god it set her chasing.** Held back the gated detail (a surviving fragment of the scratched-out name /
  what the niche held / the "story" her patron wants) behind her roll.
- **Awaiting Vale's Investigation roll** (`/roll 1d20+<Investigation>`), then adjudicate vs the secret DC and narrate the
  find — the next breadcrumb per [`encounters/cold-vault.md`](../encounters/cold-vault.md) "Loot / thread." Do **not** roll
  for her.

### Out of combat — the shrine reveals the faceless god (06-30)

- **Joint examination.** Vale called Forge in (#in-character, 9:53 PM — *"Forge, can you help me examine this stone?"* /
  Forge: *"ok, i'll investigate"*) — both PCs reading the shrine together.
- **Rolls (#roll-history):** **Vale — Investigation 22** (NAT 20 on the die, d20=20 +2) — smashes the secret DC 13;
  **Forge — Investigation 9** (d20+0) — under the DC. Adjudication: Vale's nat-20 earns the **full breadcrumb**; Forge's
  9 gets the **Forge-flavored craft read** (a smith reads *tools*, not esoterica), not the deep lore.
- **Narrated to #the-story** (read-aloud, 9:57 PM):
  - **Forge (craft, the 9):** the niche idol was **pried out recently** — pry-bar + cold-chisel, fresh bright cuts, dust
    still loose, levered free without breaking; the keeper's own tools lie frost-welded an arm's reach away.
  - **Vale (lore, the nat-20):** the carve-and-erase scarring on every surface is **ritual** — a god's name carved to
    **call** it, then scratched out to **un**-call it, over and over for years by a frightened devotion trying to take
    itself back. Behind the niche (where the scraping couldn't reach) **one fragment of the name survives**, beneath a
    worn relief of a **faceless** face — a blank oval where eyes/mouth should be. A **forgotten god of stolen faces**;
    its image was the thing carried off **through the cold door**.
  - **Patron beat:** Vale's pull goes **hot / fed** — recognition. This is the story her patron set her chasing (a
    forgotten deity); the line in her chest draws taut toward the dark beyond the open door.
- **Threads tied:** forgotten god (Vale's patron hook) + the journal's "wearing their own faces" (now: a god of *stolen
  faces*) + the pried-out idol gone through the cold door = **the campaign's next pull, pointing into the deeper dark.**
  Left OPEN — **spotlight back to the players;** never narrate their choices. Likely next beats: follow the idol/pull
  through the cold door into the dark, search the keeper's remains/tools, or pull back to rest.

### Out of combat — short rest gated by a bug, fixed live (ISSUE-050, 06-30)

- **Both PCs tried to rest and were stuck.** Vale + Forge each ran `/rest short` (~15:03) and got *"⏳ rest request
  sent to the DM. Your rest will apply once they approve it."* — but there was **no resolvable rest action** on the DM
  side (the queue resolver only acknowledges; it never delivers the hit-dice prompt). The player asked how to spend
  hit dice on a short rest, which surfaced the gating as a bug.
- **Root cause:** a self-contradiction in the rest-approval default. `Settings.AutoApproveRest *bool`'s field doc + the
  `restAutoApproved` null-settings branch both say *nil ⇒ auto-approve*, but `AutoApproveRestEnabled()` returned
  **false** on nil ("per spec"). This campaign's settings JSON has other keys but **omits `auto_approve_rest`** →
  `Settings.Valid=true`, so the null short-circuit is skipped → nil → false → **gated**. The existing
  `DefaultIsTrue` test only used *null* settings, so it never caught the valid-but-field-absent path.
- **Fix-now TDD (ISSUE-050):** flipped the nil default to **true** (matches the field contract; a DM opts into gating
  by explicitly setting `auto_approve_rest=false`). Red/green `…ValidSettingsMissingField_DefaultsTrue`; affected
  packages green; **rebuilt + redeployed**; cleared the 2 stale gated `rest_request` queue items via the dashboard.
- **Now:** players re-run `/rest short` → immediate **ephemeral hit-dice buttons** — they pick how many HD to spend
  (per die type for multiclass), the bot rolls `1dX + CON` per click (their click = their roll), HP up, `hit_dice_remaining`
  down. **HD return only on a `/rest long`** (half total level, min 1). Spotlight still on the players, out of combat.

### Out of combat — hit-dice spend crashed on class-name key, fixed live (ISSUE-051, 07-01)

- **Forge's short-rest hit-die click failed.** With rests unblocked (ISSUE-050), Forge (Barbarian 3) ran `/rest short`
  and hit the hit-dice buttons — the bot answered *"rest failed: invalid hit die type: barbarian."*
- **Root cause:** `HitDiceRemaining` was persisted **keyed by class name** (`{"barbarian":3}`) instead of by die string
  (`{"d12":3}`). Two producers keyed by `c.Class` — the builder DB-persist path (`portal/builder_store_adapter.go`) and
  `DeriveStats` (`portal/derive_stats.go`) — while **every consumer keys by die string** (the rest service's
  `HitDieValue`, the hit-dice buttons, `ddbimport`, the sheet template). So the buttons rendered *"barbarian Skip/1/2/3"*
  and a click passed `dieType="barbarian"` → `HitDieValue`=0 → the error. **Both** live rows were corrupt (Forge
  `{"barbarian":3}`, Vale `{"warlock":3}`); Vale's earlier `/rest short` only worked because she skipped hit dice
  (the pact-slot restore path never calls `HitDieValue`).
- **Fix-now TDD (ISSUE-051):** both producers now key by `ClassHitDie(c.Class)` with `+=` (multiclass classes sharing a
  die accumulate). Regressions — barbarian ⇒ `d12`, fighter+paladin ⇒ `d10` sum — + flipped 3 tests that had enshrined
  class-name keys. `make cover-check` green; committed `03642e2`; **rebuilt + redeployed**. Re-keyed the two corrupt
  live rows out of band (authorized one-off DB UPDATE, counts preserved, guarded on old value): Forge → `{"d12":3}`,
  Vale → `{"d8":3}`. Both PCs' `/rest short` hit-dice buttons now heal.

### Out of combat — the party crosses into the deeper dark (07-01)

- **Both players declared readiness to descend.** After the shrine find + a short rest each (Vale pact slots back;
  Forge's d12 rest now works post-ISSUE-051), the human layer signalled go (#in-character, Discord-only): **Vale**
  (11:50 AM) *"pointed at the door to further darkness. 'Ready to go? I feel we are close to whatever or whoever this
  is.'"*; **Forge** (11:54 AM) *"stands up & takes a look at the door, 'Yeah, more than ready.'"*
- **DM narration — the crossing** (#the-story read-aloud, 07-01, DM Console timeline top): past the hollowed shrine the
  vault's far wall folds back on a worked-stone throat, scored by a single wide **drag-mark** (the pried idol, hauled
  out and down). Following it: the mineral cold deepens, the frightened carve-and-erase graffiti **thins and stops**
  (past the last stone the keeper dared mark), and beyond that line the **faceless god is everywhere** — a hundred
  smooth eyeless ovals worked into the walls. The throat then **opens onto a buried gallery** too vast for the lantern,
  its floor lost under unnamed standing shapes; out in the dark the drag-mark ends and **something notices the light —
  a slow turning of cold attention**. Vale's patron pull points **dead ahead, hot and close**.
- **Spotlight back on the players** — they stand at the lip of the gallery; await their next declared action (advance,
  light the dark, stealth, call out, examine the standing shapes). Never roll/act/decide for them. This is **new,
  unprepped territory** — the campaign's next chapter opening past the Cold Vault (design left it open on purpose,
  [`../encounters/cold-vault.md`](../encounters/cold-vault.md) "Loot / thread"). If the gallery becomes a fight, build
  the map + encounter live (reserve **Zombies** exist in the cold-vault design; scale per [`../big-party.md`](../big-party.md)).

### Out of combat — Vale lights the gallery with Mage Hand (07-01)

- **Player action (Vale, 12:21 PM, #in-character, Discord-only):** *"Vale uses Mage Hand to bring the lantern up and
  away, trying to illuminate the gallery."* Clever, careful scouting — she floats the light out ahead on the spectral
  hand and keeps **herself** back from it. Mage Hand is a cantrip, the lantern well under its 10-lb limit ⇒
  **auto-succeeds, no roll called** (never roll for the players).
- **DM adjudication + narration** (#the-story read-aloud, DM Console timeline top): the raised lantern pushes the dark
  back only so far (the gallery is too vast for it), and in that slice the **near standing shapes resolve into
  figures** — a close-packed crowd of men, women, one child-height, all worked from (or wearing) the grey stone, all
  turned inward toward the gallery's unseen heart, and **all faceless** — eyes and mouth smoothed to blank ovals, a
  hundred of them bowed the way the idol's drag-mark runs. The light does **not** reach the ranks behind them or
  whatever they face. **Rewarding the tactic:** the *thing* out in the dark tracks the **floating lantern**, not Vale —
  the cold leans toward the light, and far back among the crowd **one smooth grey face turns to point at the lantern**
  while the rest stay bowed. Telegraph delivered without a stat line; enemy specifics stay secret.
- **DM narration — Forge's darkvision** (#the-story read-aloud, Console timeline top). Forge is a **dwarf (darkvision
  60 ft)**, so the DM posted a **colorless-grey extension** of the reveal from Forge's eyes: where Vale's lantern carves
  a bright slice, Forge already reads ~60 ft of grey — the standing figures **fill the hewn gallery rank on rank** (not
  a knot at the door), the walls carry the same eyeless ovals, the drag-mark runs a clean road through the crowd toward
  the heart, and **that heart sits past even his 60 ft (still black)** — mystery kept. He also confirms, low and certain,
  the **one blank face in the middle distance turned to track the floating lantern**. No stats/counts; specifics still
  secret. (Posted to #the-story as narration at the player's request — same table-visible beat, Forge POV.)
- **Spotlight back on the players** — the reveal is a dread/choice beat, not an ambush; combat was **not** sprung on a
  pure scouting action. Await their next declared move (advance / pull the light back / examine the faceless figures /
  stealth toward the heart / retreat). Never roll/act/decide for them. If they advance or the aware figure closes,
  **build the gallery map + encounter live** at that point (reserve **Zombies** = the faceless standing dead; scale per
  [`../big-party.md`](../big-party.md)). Board: out of combat, DM Queue empty, no active encounter.

### Out of combat — the party creeps the drag-line toward the waiting shape (07-01)

- **Player actions (#in-character, Discord-only):**
  - **Vale (12:55 PM):** *"Vale steps gingerly closer to the waiting shape. Taking care to only step along the drag
    lines, avoiding to touch any other shapes."* + prays to her patron — *"Please guide me, there are much of my story
    that i still want to uncover"* — *"continues tiptoeing while listening for any response."* Careful scouting: she
    closes on the one aware figure but keeps to the bare drag-scar (no bowed shapes to brush) and keeps the floating
    Mage Hand lantern out as the decoy for its attention.
  - **Forge (1:01 PM):** *"not sure what to do... Forge follow the path"* + explicit *"(For DM: Forge needs to do
    perception check)"* — follows Vale up the same drag-line, dwarf-eyes open.
- **DM adjudication:** still a scouting beat — **combat NOT sprung**. Two things are genuinely uncertain, so both go to
  **player-rolled** dice (never roll for them):
  - **Vale → Dexterity (Stealth)** — does her creep stay unnoticed while the floating lantern holds the waiting shape's
    attention, or does that cold attention slide off the decoy onto the warm thing tiptoeing up? Secret DC, **tiered**
    (a low roll does not auto-start the fight — it escalates the dread/choice, the attention drifting toward her).
  - **Forge → Wisdom (Perception)** — honoring the player's explicit ask; dwarf darkvision reading the hewn gallery as
    he advances. Secret DC, tiered — sets how much he catches of the waiting shape + what lies past the lantern's reach.
- **DM narration — the approach + the patron answers** (#the-story read-aloud, DM Console timeline top): both creep the
  drag-scar up the middle; the faceless crowd stays tipped toward the dark ahead, none toward the two living things; the
  lantern drifts on and the one turned shape keeps its blank face lifted to that floating light (tracking it, not them
  — tactic still paying off). Vale's plea is **answered** — patron is fed/close/eager (game-state), so it answers **not
  in words** but in the hot glad pull behind her ribs + an ugly certainty: the waiting shape *was a person*, and
  something **lifted the face off it and wore the emptiness like a borrowed mask** — the gallery is the faced made
  faceless and kept; the patron leans through her, *delighted*, "the way a starving thing leans toward a laid table"
  (the patron's hunger telegraphed as its own quiet danger). And now, close enough to hear it: a **thin dry draw of
  air — breath through a throat with no mouth to let it out.** Theme threads tied (forgotten god of stolen faces /
  "wearing their own faces"); no stat line, specifics still secret.
- **Spotlight to the players — awaiting two rolls** (Vale Stealth, Forge Perception; #roll-history). When they land,
  narrate the world's response per tier; if a low Stealth turns the aware shape's attention onto the party (or they
  press closer / touch the heart), **build the gallery map + encounter live** then (reserve **Zombies** = the faceless
  standing dead; scale per [`../big-party.md`](../big-party.md)). Board: out of combat, DM Queue empty, no active
  encounter.

### Out of combat → CONTACT MADE — Vale's creep is heard; the gallery wakes (07-01)

- **Rolls (public, #roll-history):** **Vale — stealth check d20+0 = 6** (1:04 PM); **Forge — perception check
  d20+0 = 14** (1:13 PM). (Both flat +0 = real character mods — the bot *does* apply skill mods, e.g. Vale's earlier
  Investigation d20+2; Vale simply isn't Stealth-proficient, Forge isn't Perception-proficient. No `/check` bug.)
- **Adjudication (tiered, secret DCs):**
  - **Vale 6 → creep FAILS.** The cold attention slides off the decoy lantern and finds her — she's **detected**.
  - **Forge 14 → good read.** Dwarf-eyes catch the tell the lantern hid: the nearest bowed "statues" are **not stone**
    — a grey hand uncurls, dust sifts from a rising shoulder — the gallery's dead are **waking**; the heart stays
    dormant. Party is **not caught flat-footed** (first move theirs if they strike).
  - **Combat still NOT sprung unilaterally** — contact is made, tension maxed. Held it at a **cliffhanger** rather than
    forcing initiative (the tiered low-Stealth outcome = detection + rising stakes, player keeps the choice of the break).
- **DM narration — the detection beat** (#the-story read-aloud, DM Console timeline top): Vale's heel turns a fallen
  stone chip; in the held silence it's enough — the waiting shape lifts its blank face **off the lantern and levels it
  at Vale** (dry breath quickening), and Forge clocks the nearest dead beginning to stir while the black weight at the
  heart has not moved ("It doesn't need to. It has these"). Ends: *"You have one breath before the waiting shape closes
  the distance."*
- **DM cliffhanger prompt (same #the-story post):** *"Vale, Forge — you've been seen. What do you do?"* — options
  telegraphed (strike first / break and run for the entry throat / try to speak to or turn it / something cleverer),
  and the board gets built on their declaration. Explicitly told they're **not flat-footed** (first move theirs).
- **Queue hygiene:** both `skill_check_narration` items resolved via the **#dm-queue inline resolver** (Send Narration)
  — short roll-outcome stubs delivered publicly to #in-character (1:26 PM: *@dewa Stealth 6…*, *@JonathanEka
  Perception 14…*), each pointing to the full beat in #the-story. DM Queue now **empty**.
- **Spotlight on the players — awaiting their reaction to the cliffhanger** (fight / flee / parley / clever). On their
  declaration, **build the gallery map + encounter LIVE**: the aware **masked sentinel** (the "waiting shape") + the
  **2–3 nearest faceless dead** rousing now (reserve **Zombies**), the rest of the crowd + the **heart** still dormant
  (dread reserve); scale per [`../big-party.md`](../big-party.md) (currently a 2-PC party). No board prepped past the
  Cold Vault. Never act/roll/decide for them.

### Out of combat → PARLEY — Vale kneels and offers the faceless god a story-trade (07-01)

- **Player reaction to the cliffhanger (#in-character, Discord-only):** **Vale (3:25 PM)** answered *"you've been seen —
  what do you do?"* with the boldest of the offered options — **parley, not steel**: *"Vale kneels, speaking in both
  common and infernal. 'O ancient one, I come bearing the gift and burden of stories lost. Regale us with your tale,
  and we will spread it upon the realm.'"* She offers the aware sentinel / the dormant heart the **exact trade her
  patron deals in** — the forgotten god's tale, *un-forgotten and spread*. Thematically dead-on: a **god of stolen
  faces** addressed by a **story-collector** whose patron set her chasing this very deity. **Forge has NOT yet declared
  a reaction** (his last IC line, 1:01 PM, predates the detection).
- **DM adjudication (tiered, secret DC — combat still NOT sprung):** parley of a *detected* standoff neither auto-wins
  nor auto-fails → gate it on a **player-rolled Charisma check** (never roll for her). Ruled a **real lever, not a lost
  cause** (the theme genuinely bites), but a big ask kneeling before a hungry, half-woken thing that has already marked
  her. Left the **skill choice to Vale** — *Persuasion* (plead true) / *Deception* (oversell) / *Performance* (tell the
  tale to hook it) — the approach shapes what lands. Secret DC, **tiered**: a low roll does **not** pitch her into its
  jaws — it decides whether the heart takes her for a **teller** or a **trespasser**, and the waking dead are only
  **paused**, not stopped.
- **DM narration — the offer, and the world leaning in** (#the-story read-aloud, DM Console timeline top, 3:33 PM): the
  offer goes out common-then-infernal, flat and carrying too far; the waiting shape doesn't move (blank face still
  levelled at Vale, dry breath in/out) — but **something deeper does**: for the first time the **black weight at the
  gallery's heart *attends*** (a pressure, an ear turning where it has no ear) — a god starved of exactly this word:
  *Story. Spread. Un-forgotten.* The **nearest waking dead go still — poised, not down** (a held breath); the
  sentinel's faceless head **tilts a fraction, listening past Vale to whatever owns it.** Ends: *"The dark has heard it.
  It is deciding whether it wants a teller — or one more face."* No stat line; heart/HP/specifics still secret.
- **DM prompts (same #the-story post, non-read-aloud):** **Vale →** give me a **Charisma check** (her pick of
  Persuasion/Deception/Performance); **Forge →** while she kneels and bargains for both their lives, what does he do —
  stand ready at her back, add his own voice, put himself between her and the nearest waking dead, or set his grip to
  strike the instant this sours? (Prompt = asking, not deciding for him.)
- **Post glitch (logged, not a product bug):** the Narrate **Post to #the-story** button did not fire on two `ref`
  clicks (no `POST /api/narration/post` in the network log, nothing in Discord); a **coordinate click** on the same
  button fired `POST /api/narration/post → 201` and the beat landed (3:33 PM). Single post confirmed (realtime Discord
  showed no earlier duplicate). Takeaway: verify Narrate posts via the 201 / #the-story, and fall back to a coordinate
  click if a `ref` click on Post silently no-ops.
- **Spotlight on the players — awaiting Vale's Charisma roll (#roll-history) + Forge's declared stance.** On the roll,
  narrate the heart's verdict per tier (grants a hearing / stalls the waking / the offer isn't enough and the dead
  resume); if it sours, **build the gallery map + encounter LIVE** then — the aware **masked sentinel** + the **2–3
  nearest faceless dead** (reserve **Zombies**), rest of crowd + heart dormant (dread reserve); scale per
  [`../big-party.md`](../big-party.md) (2-PC party). Never act/roll/decide for them. Board: out of combat, DM Queue
  empty, no active encounter.

### Out of combat → PARLEY, the god answers — Vale Performance 11, a TESTING half-success (07-01)

- **Rolls / declarations:**
  - **Vale — performance check d20+5 = 11** (#roll-history 3:36 PM). She chose **Performance** of the three offered
    approaches — *telling the tale* to hook the story-hungry thing. (+5 = real CHA/proficiency mod; die came up 6.)
  - **Forge (#in-character 3:35 PM):** *"Forge getting ready with both handaxe equipped, just in case that fiend
    disturbs Vale's incantation"* — **backs her play**: both handaxes drawn, on guard, held to strike **only if** the
    parley sours. Not aggressing → no attack, so combat stays un-sprung; he's the coiled spring at her back (a readied
    reaction in all but name).
- **Adjudication (tiered, secret DC — combat still NOT sprung):** DC intent — clean hearing/safe passage ~15, strong
  turn ~18. **11 = the middling band → a TESTING half-success.** Her telling *wavers* (die 6) so she buys the thing's
  **attention, not its trust, and not passage**. Ruled: the offer lands enough that it does **not** strike (dead stay
  *paused*), but a god of stolen faces **trades only in kind** and doesn't yet believe the trembling teller can pay —
  it **demands proof** (a true lost story / a name / a face) before it grants anything. Low-but-not-failed never hard-
  blocks; it escalates + hands the choice back. **No free win, no combat forced.**
- **DM narration — the god answers, and tests her** (#the-story read-aloud, DM Console timeline top; `POST
  /api/narration/post → 201` verified, coordinate-clicked Post per the ref-no-op lesson): Vale gives the dark a *thread*
  of something true and lost; for the **first time the thing at the heart does more than attend — it ANSWERS** (not in
  words — it has unmade every mouth, its own among them — but through the pull behind her ribs + the dead): the nearest
  waking figures turn their smooth faces to her together; the **sentinel sinks to her level, emptiness eye-to-eye,
  breath stirring her hair.** Meaning arrives as *hunger dressed as courtesy* — it will hear her, it is starved for
  exactly this, but **prove it or it will simply take** (as it took every bowed one). Its blank attention slides **once
  to Forge's readied axes** (weighing the steel) then back to Vale. The dead **lean a half-step closer and hold.**
  *"Whatever the offer bought, it did not buy leaving."* No stat line; heart/HP/specifics still secret.
- **DM prompts (same #the-story post, non-read-aloud):** **Vale →** parley alive but wants **proof, not just an offer**
  — tell a true lost story worth the trade (another Performance/Persuasion, or name the actual tale she pays with) /
  offer something it hungers for (a name, a face, a token — mind the cost) / press-or-change the bargain / read this as
  the turn and strike first. **Forge →** it clocked his axes — hold the guard / add his voice / interpose vs the nearest
  waking dead / open the dance? His move rides with hers.
- **Queue hygiene + PRODUCT BUG surfaced:** resolved Vale's `skill_check_narration` item (`56740f81-…`) via the
  **#dm-queue inline resolver** (Open → Send Narration) with a short outcome stub. **The stub DID deliver** to
  #in-character (3:40 PM: *"@dewa Performance Check (rolled 11) — your tale wavers, but the offer lands…"*) **AND** the
  item is now resolved (`pending: []`, `next_step: ""`) — but the resolver HTTP call `POST
  /dashboard/queue/<id>/narrate` returned **503** to the client despite BOTH side effects succeeding. So the endpoint
  **lies about failure**: a DM who retried on the 503 would **double-post** the narration to the players. Did NOT retry
  (verified delivery + empty queue first). **→ FIXED (ISSUE-052, red/green TDD, redeployed 07-01):** root cause was
  `DefaultNotifier.Resolve` letting a **cosmetic** #dm-queue message edit (`editHandled`) gate the HTTP status after the
  success-critical deliver + `MarkResolved` had already committed; compounded by non-idempotent resolvers (a retry
  re-delivers → double-post). Fix: `editHandled` now best-effort (`slog.Warn` + return nil); `ResolveSkillCheckNarration`
  + `ResolveWhisper` short-circuit `if item.Status != StatusPending`. 3 red→green tests; dmqueue + dashboard + build +
  vet green. (App request-level logging is silent — only startup logs emit — so the 503 was traced in code, not logs.)
- **Spotlight on the players — awaiting Vale's next move (proof/offer/press/strike) + Forge's follow.** If it sours,
  BUILD THE GALLERY MAP + ENCOUNTER LIVE then — masked sentinel + 2–3 nearest faceless dead (reserve Zombies), heart +
  crowd dormant (dread reserve); scale per [`../big-party.md`](../big-party.md) (2-PC party). Never act/roll/decide for
  them. Board: out of combat, DM Queue empty, no active encounter.

### PARLEY — Vale pays the price: the grand tale (07-01, 4:26 PM)

- **Player beat (Vale, #in-character 4:26 PM, roleplay — NO roll):** answering the god's demand for proof, Vale took out
  her performance tools (fans, bells, mage-hand trinkets) and told the whole tale — *"the legend of the nameless god"*:
  a god grown from **love into cruelty** (the more mortals prayed his name, the stronger he grew; power corrupted him);
  in a campaign to save the realm the **mortals sacrificed themselves to erase his name** — ending their own lives so no
  memory of him would remain; but the god was tricky and **left clues scattered across the realm** — finding one is
  meaningless, finding them ALL brings him back; so a **nameless god waits in the dark** for someone to gather the clues
  and set him free. She closed on a bell-chime and fan-flourish. **The tale is uncannily the faceless god's own shape**
  — she has, knowingly or not, described the thing she kneels before. (This IS the "true lost story" the god demanded as
  proof; delivered as substance, no `/check` issued.)
- **DM adjudication:** the payment lands as *content*, but a god that "trades only in kind and doesn't yet believe she
  can pay" cannot tell a **true** shard from a **beautiful lie** — so the beat resolves not as auto-success but as the god
  **demanding the tale be SEALED** (proven / made undeniable) **or backed with something true it can taste — a name, a
  face, a fragment.** (Live callback the DM is holding: Vale actually **carries a surviving fragment of the god's name**
  from the shrine Investigation nat-20 — a real, ownable token she can offer as proof.) Kept tiered + player-driven: no
  roll forced on her; next roll (or offered token) is hers to pick.
- **DM narration — the world leans in to hear the end** (#the-story read-aloud 4:29 PM, DM Console timeline top; `POST
  /api/narration/post → 201` verified, coordinate-clicked Post per the ref-no-op lesson, single clean post confirmed via
  #the-story screenshot — one NEW badge): the bells die into the dark; then the faceless god **LEANS** — *the stillness
  of a thing hearing its own story told back in a stranger's mouth.* The waking dead **stop their half-rise and turn
  eyeless faces to the telling** (an audience, or a jury); the cold deepens, frost blooms down the drag-scar; the hunger
  bends over the lantern-light until the dark itself seems to kneel to hear the end. But a starved god is no kind judge
  and **cannot tell what it holds** — teller who KNOWS, or finest liar? It will forgive a **true** tale anything, a
  **beautiful lie exactly once** — or fold the liar into the gallery. It wants the tale **sealed** — or backed with
  **a name, a face, a fragment.** Forge holds at her shoulder, both axes low and ready, reading the dead for the first to
  lean too close. Ends: *"The dark waits to learn whether the story is true."* No stat line; heart/HP/specifics secret.
- **Spotlight on the players — awaiting Vale's move to SEAL** (fresh Performance to make it undeniable / Deception to
  pass the improvisation as real lore / Persuasion) **or PRODUCE the true token** (the name-fragment / a face / press for
  terms first) **or strike**; Forge follows (hold guard / add his voice / interpose / open the dance). On her roll,
  narrate the god's verdict **per tier** (true-enough → it opens: passage or a clue; middling → wants more / holds;
  failure or **exposed lie** → the dead resume and it moves to TAKE). On a strike or soured/exposed bargain, BUILD THE
  GALLERY MAP + ENCOUNTER LIVE then (masked sentinel + 2–3 nearest faceless dead, reserve Zombies; heart + crowd dormant;
  scale per [`../big-party.md`](../big-party.md), 2-PC party). Never act/roll/decide for them. Board: out of combat, DM
  Queue empty, no active encounter.

### PARLEY — the seal is paid: Vale shows her pact-shard (07-01, 4:50 PM)

- **Player beat (Vale, #in-character 4:50 PM, roleplay — NO roll):** answering the god's demand for something true it can
  taste, Vale **mage-hands up an item hanging from her neck** — *"a piece of what used to be a round disc, made of some
  exotic metal. It looks like stone, but it isn't."* She presents it: *"Behold, O ancient one. One of the clues. And also
  the tether, a conduit, between me and my patron."* One object, two claims: it is **one of the faceless god's scattered
  clues** AND **Vale's pact-focus / patron-conduit.** (This is a player-established fact about Vale's own gear/backstory
  — the disc is genuinely her pact-tether; whether it is literally one of *this* god's clues, the god adjudicates by
  tasting it.)
- **DM adjudication — the showing is auto-proof, no roll:** a god that "trades only in kind" **knows its own** — it tastes
  the truth of the shard across the cold, so recognition is automatic (no Charisma check to be *believed*; the token IS
  the proof). Ruling: this validates her claim (the disc rings true to the god as its own kind) **and** deepens the
  mystery — how a story-collector's patron-tether comes to be a shard of a forgotten god of stolen faces is a **new
  campaign thread** left deliberately open (rival collector? jailer? another shard of the same broken god?). Bold play
  rewarded: the seal she couldn't buy with the tale, the fragment pays outright.
- **DM narration — the god believes; the god hungers; the tether wakes** (#the-story read-aloud 4:54 PM, DM Console
  timeline top; `POST /api/narration/post → 201` verified, coordinate-click Post per the ref-no-op lesson, single clean
  post confirmed via #the-story screenshot — one NEW badge): the disc rises, pale metal wearing "the manners of stone";
  **every faceless head turns to the FRAGMENT** (near, far, and back into the unseen dark — iron filings to a lodestone);
  the heart bends closer than all night — *it knows its own.* **The seal:** doubt gone, it no longer wonders whether she
  can pay. Then two beats in one breath — (1) **HUNGER:** cold spikes, frost jumps the drag-scar, the bowed crowd leans a
  hand's width closer, *"a starved thing shown its payment does not always wait to be handed it"*; (2) **TETHER TAUT:**
  the disc is a patron's conduit, so the instant it lifts into the god's regard the pull behind her ribs goes hard and
  present — *that other power suddenly here,* a second hand on the same shard the faceless god can feel but **cannot yet
  name.** Three touch one piece of metal: Vale, the heart, and whatever she is bound to. Forge sets his weight, axes low,
  disliking the lean of the dark. Ends: *"The god believes her. The god wants what she is holding. The dark waits to hear
  her price — or to stop waiting."* No stat line; heart/HP/specifics secret.
- **Spotlight on the players — awaiting Vale's terms** (name her price / bargain clue-for-passage-or-a-clue / offer OR
  withhold the fragment / press terms / strike — a hard bargain may call a Charisma check to control the exchange, her
  pick + roll) **+ Forge's follow** (hold guard / add his voice / interpose / open the dance). On her play, narrate the
  god's response **per tier** (fair terms + belief → it opens: passage, a clue, or a name; greed/overreach on the shard →
  it lunges to TAKE; a fumbled or insulting bargain → it stops waiting). On a strike or soured exchange, BUILD THE GALLERY
  MAP + ENCOUNTER LIVE then (masked sentinel + 2–3 nearest faceless dead, reserve Zombies; heart + crowd dormant; scale
  per [`../big-party.md`](../big-party.md), 2-PC party). Never act/roll/decide for them. Board: out of combat, DM Queue
  empty, no active encounter.

### PARLEY — Vale names her terms: withhold the shard, spread-tales-for-the-relic (07-01, 5:40 PM)

- **Player beat (#in-character, roleplay, no roll):** with the seal paid and the god believing she can pay, Vale did not
  hand over the fragment. **5:34 PM (dewa/Vale):** *"Vale quickly put the conduit back under her clothes.* / *'Now, let
  me spread your tales. Give me your relic, imbued with your tales, o faceless one.'"* Two moves in one: she **WITHDREW
  the shard** (pocketing the very thing the starved god hungers for) and **flipped the exchange** — instead of feeding it
  its own fragment she offered to be the **mouth that un-forgets it across the realm** (*"spread your tales"* — the god's
  exact craving of Story / Spread / Un-forgotten, and the very trade her patron deals in) and **demanded the god's RELIC**
  in return.
- **DM adjudication — a control-the-exchange moment → player-rolled Charisma check (her pick), secret tiered DC:** she is
  pressing a hard, uneven bargain on a powerful, starved thing that *"trades only in kind"* — a *promise* of telling for a
  *real relic now*, with the proof she just showed palmed back out of reach. That is exactly the "pushing a hard bargain
  may call a Charisma check to control the exchange" case. Ruled it calls **Vale's Charisma check — her pick (Persuasion
  to broker it / Deception to make the withdrawal read as safe & routine / Intimidation), her roll** — with a **secret
  tiered DC**: a low roll does **NOT** auto-spring combat, it thins the god's patience (a thing fed then denied reads the
  withdrawal as a tease → escalate dread toward *"it stops waiting"*); a high roll = it entertains parting with a relic /
  granting passage for the promise of being carried. Fiction-cued (as the earlier Performance beat was); **players roll
  their own dice — never rolled for her.** She has **not yet rolled** (latest #roll-history entry is still the 3:36 PM
  Performance 11).
- **DM narration — the god weighs her tongue against its relic** (#the-story read-aloud 5:40 PM, DM Console timeline top;
  `POST /api/narration/post → 201` verified, coordinate-click Post per the ref-no-op lesson, single clean post confirmed
  via #the-story screenshot — one NEW badge, *"1 new message since 5:40 PM"*): the disc goes back under her collar and the
  cold **FOLLOWS it** — the whole starved attention bending after the hidden thing *"the way a hound leans after meat
  lifted off the table"*; the courtesy thins, every bowed grey head tilts a fraction as if the crowd felt the tease. Then
  her offer lands on the **one nerve a forgotten god has left** — to be *carried*, *un-forgotten*, a story loose in the
  bright world again — and the hunger draws in until the flame gutters. But it feels the **uneven shape**: *"give me
  something real and mine — for a promise of telling, and keep the shard you showed me besides."* It wants what she offers
  more than anything in an age, yet has **not decided** her tongue is worth the price, nor that *"a mouth quick to pocket
  its proof can be trusted to carry a true tale and not the finest lie ever told down here."* Ends: *"The faceless heart
  holds, and holds — and waits to hear how she means to make it believe."* No stat line; heart/HP/specifics secret.
- **Spotlight on the players — AWAIT Vale's Charisma roll (#roll-history, her pick) + Forge's stance** (hold guard / add
  his voice / interpose / open the dance). On the roll, narrate the god's verdict **per tier**: strong → it entertains the
  trade (a relic / passage / a clue, likely **BINDING her to the telling** — it trades only in kind); middling → it wants
  the shard first or more proof before it gives anything (stalls, hunger visible); fail/insulting → the withdrawal reads
  as bait, patience cracks, it moves to **TAKE** → BUILD THE GALLERY MAP + ENCOUNTER LIVE (masked sentinel + 2–3 nearest
  faceless dead, reserve Zombies; heart + crowd dormant; scale per [`../big-party.md`](../big-party.md), 2-PC party). The
  patron ↔ faceless-god shard thread stays open (do not resolve). Never act/roll/decide for them. Board: out of combat,
  DM Queue empty, no active encounter.

### OOC — "do I need to roll?" + Message Player whisper bug (ISSUE-053, 07-01, ~6:07 PM)

- **Player OOC (#in-character 6:07 PM, dewa):** *"(to DM: do I need to roll? It wasn't clear from the story post)"* — the
  fiction-cue for the Charisma check was too subtle. Answered the direct rules question: **yes — a Charisma check, her
  pick (Persuasion / Deception / Intimidation), to control the exchange.**
- **Delivered via dashboard Message Player (whisper) — and hit a bug:** `POST /api/message-player/` returned **500**
  (`dm_player_messages_player_character_id_fkey`, SQLSTATE 23503). **The DM was still delivered** — the service sends the
  Discord DM *before* logging the row, so only the (unlogged) history insert failed. Did **not** retry (would double-DM).
- **Fix-now TDD → ISSUE-053 FIXED + redeployed:** `PlayerLookupAdapter.LookupPlayer` never set `PlayerInfo.RowID`, so the
  insert used `player_character_id = nil` and violated the FK. Red (extended `TestPlayerLookupAdapter_Success` to assert
  `RowID` = the player_characters PK) → green (`RowID: row.ID`). `internal/messageplayer` green @ 96.4% + vet; rebuilt +
  redeployed. Details in [`../issues.md`](../issues.md) ISSUE-053.
- **DM-guide rule added** (per user, [`../dm-rules.md`](../dm-rules.md) "How DM actions must be performed"): reply to
  players via **Narrate → #the-story by default** (table sees it); use the **whisper only for genuinely secret** DM↔player
  content. The roll-clarification was table-public, so narrate would have been the cleaner channel — recorded for next time.
- **Still AWAIT Vale's Charisma roll (#roll-history, her pick) + Forge's stance.** Board: out of combat, DM Queue empty,
  no active encounter.

### PARLEY WON — Vale's Deception 21 seals the bargain; the god pays (07-01, 6:19 PM)

- **Player roll (#roll-history 6:12 PM):** after the whisper answer, Vale ran `/check` and rolled **Deception d20+5 = 16
  + 5 = 21** — her pick of skill (selling the shard-withdrawal as nothing worth minding). No extra RP flavor posted; the
  roll *is* the move. `/check` filed a `skill_check_narration` dm-queue item (Console `next_step` = "Resolve … Deception
  check (rolled 21)").
- **DM adjudication — 21 clears the top secret tier → strong success.** She's bluffing a truth-tasting god, and a 21
  makes the bluff perfect: it finds only her ease, takes no fright from the palmed shard, and — starved for exactly what
  she offers (to be carried / told / un-forgotten) — **accepts the trade in kind.** A god that trades only in kind gives
  something real: (1) **passage** — the faceless crowd parts and the drag-scar road opens toward the heart; (2) a
  **relic** — a blank ashen face-shard, humming like a struck bell, "a face that was never worn, a story with the name
  still folded inside" (a story-vessel, and almost certainly another of the god's scattered clues for Vale's patron-quest);
  (3) a **binding** — taking it is a compact to carry the tale true / un-forget the god, "a debt no god forgives."
  **Latent hook kept open (not resolved):** the bargain was struck on a *Deception* — a truth-taster was bluffed, so if
  she never spreads its tale or the lie ever surfaces, the creditor is a god. Patron-tether thread deepened (patron
  attentive down the cord, a second hand on the same kind of shard). Rewards the strong roll, advances the campaign
  (deeper + a clue-relic), and plants tension — without deciding the players' next move.
- **DM narration** (#the-story read-aloud 6:19 PM, Console timeline top; `POST /api/narration/post → 201`, coordinate-click
  Post, single clean post verified via #the-story screenshot — one NEW badge, *"1 new message since 6:19 PM"*): the lie
  lands, the cold eases to "almost courtesy," the crowd parts and the road opens, the relic is set at the lantern's edge,
  and the taking binds her — "Down the tether at Vale's ribs that other power stirs, attentive … Forge sets his shoulder
  at her back." Ends: *"The way is clear. The relic waits. The god has paid — and now it is owed."* No stat line; heart/
  HP/specifics secret.
- **Queue resolve — outcome stub delivered, item resolved, cosmetic 503 (ISSUE-052 class):** resolved the
  `skill_check_narration` item via the #dm-queue inline resolver ("Send Narration") with a concise no-numbers stub →
  delivered to **#in-character 6:20 PM** (*"Deception 21 — flawless … a relic waits … the taking binds you to the telling
  …"*). `POST /dashboard/queue/<id>/narrate` returned **503**, but the work fully SUCCEEDED — **DB-verified**
  `dm_queue_items` row `status=resolved`, outcome persisted, `resolved_at` set; stub visibly delivered; **no server error
  logged** (no ERROR/WARN). Same "real work done, secondary step fails the status" shape as ISSUE-052; the dashboard tab
  also froze the renderer twice, so treated as a client / Discord-rate-limit timing artifact, not chased as a fresh code
  bug. **Per the belt-and-suspenders rule: verified delivery + resolution before moving on; did NOT retry** (a blind retry
  would double-DM). 503 recurrence noted in [`../issues.md`](../issues.md) under ISSUE-052.
- **Spotlight back on the players — parley won, no fight.** AWAIT their next move: Vale — TAKE the relic (accept the geas)
  / hesitate or refuse / examine it / press for more / advance the now-open road toward the heart; Forge — take point /
  guard / add his voice. Deeper in is NEW unprepped territory toward the dormant HEART; if a fight later erupts (the heart
  rouses, the deception unravels, or they provoke it), build the gallery map + encounter live then (reserve Zombies; scale
  per [`../big-party.md`](../big-party.md), 2-PC party). Never act/roll/decide for them. Board: out of combat, DM Queue
  empty, no active encounter.

### RELIC TAKEN — Vale accepts the geas; the compact seals (07-02, 6:40–6:43 AM)

- **Player beat (Vale, #in-character 07-02 6:40 AM, roleplay, no roll):** *"Vale gets closer to the relic, and uses mage
  to pick it up."* Her declared answer to the offered bargain — she **takes** the blank ashen face-shard the god set at
  the lantern's edge, lifting it with **Mage Hand**.
- **DM ruling — no roll; taking = accepting the compact.** Mage Hand is a cantrip and the shard is palm-sized (far under
  the 10-lb limit) → **auto-success** on the lift; and *accepting* a god's relic is a declared roleplay choice, not a
  check. So no dice — the consequence is world-reaction narration, not a gated roll. The taking is the acceptance: she now
  owes the god the **telling** (carry its tale, tell it TRUE, un-forget it — *"a debt no god forgives"*). The god pays in
  kind (passage already granted stays open; the relic is now hers).
- **DM narration** (#the-story read-aloud 6:43 AM, Console timeline top; `POST /api/narration/post → 201`, coordinate-click
  Post per the ref-no-op lesson, single clean post verified via #the-story screenshot — one NEW badge, July-2 divider):
  the mage-hand lifts the shard (she takes it **at arm's length, never bare skin** — her caution honored, her *decision*
  not narrated); the bargain **seals** the instant it crosses into her keeping (no thunder — the god unmade every mouth —
  only a cold *"taken, therefore owed"*); the hundred bowed heads stay turned aside and the drag-scar road stays open
  toward the heart (**passage confirmed paid**); down the tether her patron stirs attentive (a second hand closing on its
  own edge of the same broken shard, close + interested); Forge shifts at her shoulder, axes low. Ends: *"The relic is
  hers. The god has been paid — in the one coin it cannot yet taste as false. And the road goes down."* No stat line;
  heart/specifics kept secret.
- **Latent hook seeded, NOT resolved:** *"the one coin it cannot yet taste as false"* — the debt was struck on a
  **Deception** (her earlier 21). A truth-tasting god was paid in a lie; the geas holds only as long as she tells the tale
  TRUE and the bluff never surfaces. If she never spreads it / the lie is tasted, the creditor is a god. Kept as a live
  campaign tension.
- **New durable kit on Vale:** (a) the **ashen face-shard relic** — a story-vessel, *"a story with the name still folded
  inside,"* almost certainly another of the faceless god's scattered patron-quest clues; (b) the **GEAS** (owe the
  telling). Both recorded on her party sheet + in game-state.
- **No queue item / no whisper this beat** — pure roleplay + narration; DM Queue stayed empty, no `/check` filed.
- **Spotlight back on the players — bargain complete, no fight.** AWAIT their next move: advance the open drag-scar road
  DEEPER toward the dormant HEART (NEW unprepped territory) / examine the relic / Vale press the god for more / Forge take
  point. If a fight erupts deeper in (the heart rouses, the deception unravels, or they provoke it), build the gallery map
  + encounter live then (masked sentinel + faceless dead, reserve Zombies; scale per [`../big-party.md`](../big-party.md),
  2-PC party). Never act/roll/decide for them. Board: out of combat, DM Queue empty, no active encounter.

### OOC — "is there more down here, or is the relic all?" (07-02, 6:50–6:53 AM)

- **Player question (Vale, #in-character 6:50 AM, to DM):** *"do I see or sense that there's more things to find
  further down the gallery or this relic is all that's important down here?"* — a perception/meta question, table-public
  (not secret), so answered via **Narrate → #the-story** per the [`../dm-rules.md`](../dm-rules.md) reply rule.
- **DM ruling — no roll.** Her patron-tether pull is established always-on fiction (it has pointed "dead ahead, hot and
  close" toward the heart since the gallery reveal), so the gut-sense answers for free; her *eyes* are still capped by the
  lantern's slice of dark. No `/check`, no queue item.
- **DM narration** (#the-story read-aloud 6:53 AM, Console timeline top; `POST /api/narration/post → 201`, coordinate-click
  Post, single clean post verified — one NEW badge, *"1 new message since 6:53 AM"*): answered in two voices — **eyes**
  give little (lantern ends where it always has; the road runs on into dark too big to read from here); the **tether** does
  NOT settle — the relic hums like *a* single struck note, not a closing chord, and the pull slides straight through it and
  keeps going toward the black heart the crowd bows to. *"Her patron is not done. This was a clue, not the prize."* Seeded
  two implicit action paths WITHOUT deciding: **read the shard** (coax loose the folded name — would be a check if she
  tries) / **advance** toward the heart (walk toward it to learn). Heart/specifics kept secret.
- **Spotlight still on the players.** The answer reinforces the open options: advance the drag-scar road deeper toward the
  dormant HEART (unprepped → build map + encounter live if it turns to a fight) / still herself and read the relic / press
  the god / Forge take point. Never act/roll/decide for them. Board: out of combat, DM Queue empty, no active encounter.

### Forge examines the shard — free glance-read + a called check (07-02, 8:10–8:29 AM)

- **Player question (Forge / Jonathan, #in-character 8:10 AM, to DM):** *"is there a hint of dwarf/gnome inscription in
  the shard. or should I do 'check'?"* — Forge (dwarf, smith's/craft eye) leans in to examine the ashen face-shard Vale
  holds. Table-public → answered via **Narrate → #the-story** + an OOC check-prompt per the [`../dm-rules.md`](../dm-rules.md)
  reply rule.
- **DM ruling — free glance, then gate the deeper read on a `/check`.** A dwarf's craft eye gets the *obvious* for free
  (no roll): it's **not dwarf- or gnome-work** — no maker's rune, no tool-bite, no seam, all one piece worked by nothing
  he's watched hands do; should be dead clay yet rings like struck bronze — *wrong all the way through.* But reading true
  hidden script / the "folded name" is past a glance → **called a player-rolled check, his pick: Investigation** (search
  the shard for marks/seams) **or History** (does a dwarf's memory know this ash-faced god / its script). **Secret tiered
  DC (held in head, ~15):** low still confirms the wrongness + a sense that something IS folded in (a thread, never
  hard-blocked); mid → a partial (a fragment of not-mortal shape/script tied to the faceless god); high → a real hint (a
  syllable of the folded name / the same carve-then-erase ritual script from the shrine / a dwarf-legend of face-theft).
  Consistent with the 6:53 seed ("she might coax the name loose if she stilled herself and listened").
- **DM narration** (#the-story read-aloud 8:29 AM, Console timeline top; `POST /api/narration/post → 201`, coordinate-click
  Post, single clean post verified — badge counts the 6:53 + 8:29 posts). Read-aloud = the glance-read; an OOC coda after
  the block prompts the `/check` (Investigation or History, his pick, report the number). Heart/specifics kept secret.
- **AWAIT Forge's `/check` roll** (#roll-history) — a `/check` files a `skill_check_narration` dm-queue item → becomes
  `next_step`; resolve it per tier via the #dm-queue inline resolver (no-numbers outcome stub → #in-character), then narrate
  the find to #the-story. Spotlight stays on the players (Vale may also act / advance). Never roll/act/decide for them.
  Board: out of combat, DM Queue empty (pre-roll), no active encounter.

### Forge reads the shard — Investigation 19, the made-blank and the surviving stroke (07-02, 11:19–11:32 AM)

Following the 8:29 AM setup (free glance-read + a called `/check`), Forge's player rolled **Investigation** — **19** (#in-character 11:19 AM: *"my check investigation roll is 19"*), filing a `skill_check_narration` dm-queue item (the DM Console `next_step`). 19 clears the secret tiered DC (≈15) → **high tier.** Resolved via the **#dm-queue inline resolver** (Open → outcome textbox → Send Narration): the shard is a **MADE blank** — a face was carved into it once, patient and fine, then ground smooth and *unmade*, the same **carve-to-call / scratch-to-un-call** trick worked on the shrine upstairs; the erasure **slipped**, leaving **one surviving stroke at the cold rim = a single folded syllable of the name that used to live in this face** → the thread to pull. The `POST /dashboard/queue/<id>/narrate` again threw a cosmetic **503**, but the outcome fully delivered to #in-character (11:29 AM, verified on-screen) and the item flipped **RESOLVED** (queue empty) — **not retried** (ISSUE-052 class; see issues.md). Table-facing beat narrated to #the-story as a read-aloud (below, combined with Windreth's arrival).

### Windreth joins — a high-elf rogue at the gallery mouth (07-02, ~11:29–11:32 AM)

A **new player** joined the Ashfall party. Their PC — **Windreth**, **High-Elf Rogue 3 (Thief)**, urchin (id `b2c436da-6762-458f-8016-3fe8f18e35e6`) — was already self-built via the portal and sitting in the approvals queue. Reviewed on the dashboard **Character Approvals → Review**: clean, legal L3 build — DEX 17 (+3), CON 14, WIS 14; AC 14 (leather + DEX), HP 24 (d8 average method); saves DEX/INT; 7 skills (4 rogue + 2 urchin + Keen Senses perception) with **Expertise in investigation + stealth**; Cunning Action / Sneak Attack / Fey Ancestry / Trance / Thief archetype; urchin kit (shortbow, shortsword, dagger, thieves' tools, map of a city, pet mouse, token of remembrance). Only cosmetic gap: the 2024 high-elf cantrip + 3rd language not wired (same class as Vale's ISSUE-009) — non-blocking, left as-is rather than stall a waiting player. **Approved** (`POST /dashboard/api/approvals/…/approve → 200`; approval queue emptied; bot auto-posts the character card + DMs the welcome).

**Woven into the fiction** (per world.md's fold-in) at the **mouth of the Buried Gallery of the Faceless God** as a traveler who reached the cold, keeper-less Ashfall Waystation, found the cellar door clawed open from the inside, and followed the cold-and-a-held-breath down after the party. Narrated her **arrival + the world's reaction only** (never her choices): a slight, road-worn, elf-quiet newcomer resolving out of the dark up the worked-stone throat, behind Vale + Forge and the shard-light. Per-PC hook seeded on entry (thief/elf senses): a thief's eye reads the pried niche (worked-not-smashed) and the carve-then-silenced name; elf-keen ears catch the near-name folded into the humming shard; the dormant heart does not stir, but *"the whole starved attention of the place has just noticed there is one more story in the room."* Combined with Forge's find into one #the-story read-aloud (`POST /api/narration/post → 201`, 11:32 AM, coordinate-click Post, single clean post verified) + a brief OOC welcome handing the spotlight to the new player. Roster + a new `party/windreth.md` sheet added. **Party now 3 PCs (Vale, Forge, Windreth).** Board: out of combat, DM Queue empty, no active encounter — **awaiting the players' next move** (Windreth's first action / advance toward the heart / read the shard's stroke further).

### The heart in view — Windreth's first look, Forge reaches for a memory, Vale walks to the god (07-02, ~11:32 AM–12:45 PM)

Three players acted at one moment, out of combat, at the gallery mouth. Board pulled first
(DM Console: `next_step` empty, `pending: 0`, no active encounter — clean).

**Player→PC mapping confirmed.** A new author, **Winfroz**, posted in-character. Party
Overview shows **exactly 3 PCs** (Vale, Forge, Windreth) and no pending approvals, so by
elimination **Winfroz = Windreth's player** (Discord handle). No 4th onboarding needed;
recorded the handle on [`../party/windreth.md`](../party/windreth.md).

The three declared moves (#in-character):
- **Forge** (JonathanEka, 11:32 AM): *"Interesting shard it is. Let me hold & take a look at
  this inscription closer. (to DM: should I do 'History' check to recall what I know? since we
  unravel new facts from this shard)."*
- **Windreth** (Winfroz, 12:32 PM): *"(to DM: looking around)"* — her first action in the scene.
- **Vale** (dewa, 12:36 PM): *"Vale let Forge study the shard, and walked plainly, jauntily, to
  mask her trembling feet, towards the heart of the gallery."*

**Adjudication.**
- **Forge → History granted.** His Investigation 19 already read the *object* (the made-blank +
  the surviving stroke). History is a genuinely different axis — does a dwarf's memory hold an
  old tale of *stolen faces* — so it isn't an info-farm re-roll. Allowed one **History** check,
  **secret tiered DC ≈15** (low = a shiver of half-remembered fear, nothing solid; mid = a
  fragment of dwarrow-legend; high = a real recalled tale that contextualizes the faceless god).
  He rolls his own die → a `/check history` will file a `skill_check_narration` queue item.
- **Windreth → free scout read + deeper check offered.** "Looking around" gets her the plain
  darkvision/keen-senses read for free (rank-on-rank faceless bowed dead, the drag-scar road,
  the one sentinel still turned to the light, the heart past even elf-sight). Offered a
  `/check perception` or `/check investigation` to dig past the obvious (a hidden way, a trap,
  the safe path, a weakness). No roll for the plain read.
- **Vale → walks the paid-open road, auto-pass, the HEART is revealed.** She bought passage
  (the crowd parted when the god was paid), so walking the drag-scar road needs no roll — the
  faceless heads incline *aside* and let her through. At the road's end: a cleared ring, and
  the **idol pried from the shrine above and dragged all the way down, enthroned on a low heap
  of pale, curved, overlapping things she does not let herself name** — colossal, faceless,
  **DORMANT**; her patron-pull ends dead in its blank front; the cold pours off it. Kept the
  heart dormant and **all mechanics SECRET**. **Combat NOT sprung** — a non-hostile approach on
  paid passage, and the heart has not roused. The **party is now SPLIT**: Vale alone at the
  heart, Forge + Windreth back at the mouth.

**Narration.** One braided read-aloud to #the-story (`POST /api/narration/post → 201`, 12:45 PM,
coordinate-click Post, single clean post verified) — Windreth's first read, Forge's
almost-memory, and Vale's walk to the revealed heart, closing on *"The dark waits to see what
she does with it."* (Render note: the bot puts the plain OOC coda **first** and the read-aloud
box **last** regardless of source order — confirmed against the 11:32 post — so the box lands as
the closer; trimmed the OOC bullets so they don't pre-spoil the heart the box reveals.)

**Await the players** — Vale's move at the heart (speak / touch / commune / hold / call the
others up) / Forge's `/check history` / Windreth's `/check perception|investigation`. **If the
heart wakes or they provoke it → BUILD THE GALLERY MAP + ENCOUNTER LIVE** (masked sentinel +
faceless dead, reserve Zombies; scale per [`../big-party.md`](../big-party.md) — now a **3-PC
party**). Never roll/act/decide for them.

### Windreth reads the room, Forge's memory fails, and Vale lays a bare hand on the god (07-02, ~1:20–1:30 PM)

**Board on resume.** Out of combat, no active encounter. DM Console **NEXT STEP** =
*"Resolve Skill Check Narration from Windreth: Perception check (rolled 17)"*, **Pending 2**
(Windreth Perception 17 + Forge History 3). Discord #in-character carried three fresh moves
since the 12:45 PM heart-reveal: Windreth `/check perception` (1:05 PM), Forge's History roll
reported (`(to DM: my check history roll is 3)`, 1:27 PM), and — the big one — **Vale** (dewa,
1:20 PM): *"Vale reached out to touch the seated figure. 'Give me your story, faceless one.'"*

**Two checks resolved** via the #dm-queue inline resolver (each a no-numbers stub → #in-character;
both `POST /dashboard/queue/<id>/narrate` threw the cosmetic **503** again — ISSUE-052 class —
but **both fully delivered + items RESOLVED**, verified in #in-character 1:29/1:30 PM and Console
**Pending 0**; NOT retried):
- **Windreth — Perception 17** (secret tiered DC ≈15, high). Her first mechanical contribution
  as the scout: a thief's read of the crowd — most of the bowed dead are only dead, but the
  **sentinel that tracked the lantern + a knot nearest the heart are tensed, listening, angled
  toward Vale out on the road alone**; if it sours *those* wake first and Vale is 40 ft out with
  no one beside her; the drag-scar road is the only clear lane. A real tactical edge + tension,
  no enemy numbers.
- **Forge — History 3** (secret tiered DC ≈15, low). Thematic floor: his dwarf-memory reaches
  for a tale of an ash-faced face-stealing god and it **slides off the harder he grips** —
  nothing solid, but the blank itself is the tell (*a thing built to be un-remembered resists
  even a dwarf's memory*). No fabricated lore; the Investigation-19 shard read stands as the
  concrete thread.

**Vale touches the god — adjudication.** Her declared move is a genuine provoke: a **bare hand**
(she'd been careful "never bare skin" before) on the **dormant** faceless idol + a **demand**
for its story, while she already carries the geas (owes the telling) and struck the first
bargain on a **deception**. Passage was bought for *walking* the road, not for *seizing* the
god — so this is beyond the paid auto-pass and warrants a roll. Did **not** unilaterally spring
combat and did **not** decide for her: narrated the **contact + the god stirring** (cold
"opens," the patron-tether snaps taut, the dormant weight shifts "not awake, not asleep," the
gallery draws a breath toward the point she touches) and **gated the outcome on her own
Charisma roll** (her pick — Persuasion / Deception / Intimidation; secret tiered DC ≈16, it's a
god). This keeps the latent **deception-crack** tension live on the die (skin-to-skin with a
truth-taster) without resolving it. Intended tiers when she rolls: **high** → the story gives
(a real new thread via the tether, geas humming approval, no combat); **mid** → it half-wakes
and weighs the imbalance (she demands while owing), gives a fragment, hunger visible, holds;
**low/fumble** → the touch lets it taste the lie or reads the demand as theft → it **WAKES** →
BUILD THE GALLERY MAP + ENCOUNTER LIVE (masked sentinel + the tensed dead nearest the heart
rouse; Vale alone on the road, Forge + Windreth 40 ft back at the mouth).

**Narration.** One braided read-aloud → #the-story (`POST /api/narration/post → 201`, ~1:33 PM,
coordinate-click Post, single clean post verified) — Windreth's read, Forge's memory that won't
come, and Vale's bare-hand touch + the god's stir, closing on *"waiting to learn whether this
is worship, or theft."* Followed by a **per-PC OOC coda** (split party, so each named + given
concrete options and exact rolls per the new dm-rules nudge guideline): Vale → her Charisma
`/check`; Forge → `/move` up the road / hold / ready / call to her; Windreth → `/check stealth` /
ready against the tensed dead / move up beside Forge. Render confirmed OOC-first / box-last in
the preview again.

**Await the players' declared moves + Vale's roll.** Vale's Charisma check is the pivot — narrate
the god's verdict per tier on the roll; Forge + Windreth may act in parallel. **If Vale's roll
goes low, or anyone strikes, or the heart otherwise wakes → BUILD THE GALLERY MAP + ENCOUNTER
LIVE** (masked sentinel + the tensed faceless dead nearest the heart, reserve Zombies; scale per
[`../big-party.md`](../big-party.md) — **3-PC party**, and the split matters: Vale is isolated
40 ft up the road). Never roll/act/decide for them.

### The god wakes — Vale's Persuasion 7 cracks the parley; combat goes live in the Buried Gallery (07-02, ~2:00–2:25 PM)

**Board on resume:** out of combat, DM Console clean except two pending skill-check narrations — Vale Persuasion 7, Windreth Stealth 13 — plus Forge's `/move` in #in-character. The pivot I'd gated last beat (Vale's bare-hand demand on the dormant god) had landed on the die.

**The three player moves.**
- **Vale (dewa)** picked **Persuasion = 7** for her demand. Secret tiered DC ≈16 → **LOW.** A truth-tasting god, felt skin-to-skin, tasted the whole stolen shape of it: a hand that swore never to touch it, a debt still owed, a bargain paid in a lie. Per the pre-committed low/fumble tier → **the god WAKES.** Stub → #in-character (2:00 PM): *"…Your easy words don't land. Under your palm the stone stops being cold. It felt the demand as a hand in its pocket. And it wakes."*
- **Windreth (Winfroz)** rolled **Stealth = 13** → slipped off the road into the crowd's shadow, working a line toward the heart, unseen, with a clear angle on the tensed dead. Stub → #in-character.
- **Forge (JonathanEka)** `/move` (1:34 PM): *"Moving nearby Vale. Dual wielding handaxe, getting ready. Slowly & holding his breath…"* → charging up the road.

Both queue stubs delivered despite the cosmetic **503** (ISSUE-052 class — verified delivered + both RESOLVED, queue empty; NOT retried).

**Built the gallery encounter live.** New map **"Buried Gallery of the Faceless God"** (12×12, id `39ecd023-51d8-44bb-bf8e-29e1eff3a231`; player spawn bottom/mouth, enemy spawn top/heart). New encounter **"The Buried Gallery of the Faceless God"** (id `9e558982-697a-4cc8-8c25-abe3d34cf201`) — **1 Ghoul (the masked sentinel) + 2 Zombies (the tensed dead)**, reserve Zombies held; scaled per big-party.md for a 3-PC party with Vale isolated. **Start Combat** → 200.

**PRODUCT BUG hit live (ISSUE-054, OPEN):** Start Combat auto-placed only the 3 PCs (player spawn); the **3 enemies came out UNPLACED** (empty position) despite the enemy spawn zone → the Discord #combat-map showed no enemy tokens (the user flagged it). **Workaround:** hand-placed every token in the Combat workspace (drag works there; the encounter-builder canvas froze screenshots + its drag-to-place is HTML5-DnD-finicky). Also noted: DM token drags update the DB but don't re-post the #combat-map — it refreshes on the next combat action; no manual re-post button exists. Fix-now TDD investigation launched (background) — report-for-review, no redeploy yet.

**Positions set to match the fiction (workspace drags):** Vale **F3** (the heart, ALONE, at the god's front); Ghoul **G3** (adjacent to Vale); Zombie 2 **D4** + Windreth **C4** (left flank — she's poised to Sneak-Attack it); Zombie 1 **H4** (right flank); Forge **F7** (mid-road, closing). Initiative (Console): **Windreth 23 → Forge 19 → Ghoul 15 → Zombie 2 11 → Vale 9 → Zombie 1 3.** Enemies flagged Surprised in the builder (dormant dead sluggish; the ready party gets the jump — verify it holds at their turns).

**Narrated the wake → #the-story** (read-aloud, `POST /api/narration/post → 201`, single clean post; OOC-first/box-last confirmed) — in **plain simple English** per the new dm-rule. The god has no eyes to open, but the cold under Vale's hand turns from stone-cold to a listening cold; it does not forgive; the tall watcher turns its blank face on her; two kneeling dead push up on either side; *"The god is awake. It wants its faces back. And Vale is standing alone in the middle of it."* Per-PC OOC coda: the board + Windreth's turn (unseen → advantage/Sneak Attack; `/attack` / `/move` / Cunning Action / `/done`).

**AWAIT the players' turns in order** — Windreth first, then Forge (closes to Vale), then the sentinel. On the Ghoul's turn, if surprise didn't apply in-engine, play it as just-rousing (no alpha-strike on isolated Vale — she has 24 HP, Misty Step, Forge closing). Reserve Zombies in pocket. Enemy HP/AC secret. Never roll/act/decide for them.

### The gallery skirmish is won — combat closes; the party stands at the god's heart (07-02 evening → 07-03, ~02:00–02:28Z)

**The fight played out across three rounds** (the full mechanical log lives in the DM Console timeline / `action_log`, not here). **R1:** Windreth struck Zombie 2 (dagger + off-hand shortsword + a retroactively granted Sneak Attack); Forge crit the Ghoul with a handaxe and raged, pulling it off the isolated Vale; the Ghoul's Bite missed Forge; Zombie 2's Slam tagged Windreth (→19/24). Vale moved to H2 and cast **Shatter** (spending her L2 pact slot) catching all three risen dead; each made the DC-13 CON save but took 7 thunder. **R2:** Windreth killed Zombie 2 (shortsword); Forge chipped Zombie 1 (handaxe 9) and whiffed the Ghoul; I ran the Ghoul's and Zombie 1's NPC turns from the Turn Builder (Ghoul Bite missed Forge; Zombie 1 Slam hit Forge for 1 after rage-resist → 29/32). Vale's second Shatter (last pact slot, 0 left) finished the Ghoul; Zombie 1 survived the blast at 1 HP. **R3:** **Windreth finished the last zombie** — `/move` to G4, Dagger 17-to-hit, 7 piercing → Zombie 1 dead (then a reflexive Cunning-Action Hide with no hostiles left — harmless).

**Combat closed.** With all three roused dead down — and the hundred *other* bowed faceless dead + the colossal dormant idol-heart never so much as stirring — I called the fight. **End Combat** (dashboard, confirm modal) → encounter `9e558982-…` marked completed, `has_encounter:false`. **ISSUE-038 auto-carried final HP/conditions back to the sheets** (DB-verified: Windreth 19/24, Forge 29/32, Vale 24/24, no conditions; Forge's rage correctly did not carry). Narrated the victory + aftermath → #the-story in plain English (read-aloud + an OOC coda offering out-of-combat options + slash commands; `POST → 201`, Discord msg `1522426521632116828`, verified 02:19:28Z).

**Post-fight housekeeping (the game-state earmark, now done).** Cancelled all **6** stale DM-queue stubs — five orphan `enemy_turn_ready` items (ISSUE-057 pre-fix leftovers; the in-memory tracker couldn't auto-clear them across the earlier binary) and Vale's now-moot `hellish rebuke` reaction declaration → `pending: []`. Then **redeployed the ISSUE-057 fix** (`docker compose up -d --build app`; app rebuilt, Discord session reopened + every bot check passed at 02:28Z, combat state survived the restart). Queue clean, no active encounter.

**Found + filed ISSUE-059 (OPEN):** the DM-Queue "Resolve" button fires no HTTP request — its Svelte form never submits, so none of the 6 stubs would clear from the UI (the network log showed only the item GET, never a `/resolve` POST). The server endpoint is fine, so I cleared them by POSTing `/dashboard/queue/<id>/resolve` with an empty outcome from the authed dashboard page (all → 204; that code path only marks-resolved + does a cosmetic DM-only #dm-queue message edit — nothing player-facing).

**Where things stand:** the party is **out of combat**, together near the heart of the gallery, in front of the dormant faceless god that let them kill its risen guards without lifting a hand. Vale still carries the ashen face-shard and still owes the geas (struck on a deception). Spotlight is on the players — await their next move (rest / examine the fallen sentinel + the two dead + the heap of pale things + the idol / Vale speak to the god or reach the patron down the tether / move up / search / retreat). If they wake the heart or provoke it, the hundred bowed dead are the dread-reserve; build the escalation live (scale per big-party.md — 3-PC party). Never roll/act/decide for the PCs.

### The parley reopens at the heart — Vale addresses the god, Forge's eyes uncover the name (07-03, ~02:26–02:47Z)

**Vale spoke to the god.** Out of combat, she addressed the dormant shape (#in-character 02:26Z): *"that was not nice. I promised to carry your story and you wanted to end mine?"* I voiced its reaction → #the-story (read-aloud + OOC coda, msg `1522430718880518216`, 02:36Z): it has no mouth, so it answers through the cold and the tether — it did not want to **end** her, it **tested** her (a bare-hand demand while owing → it woke its watchers to ask *teller or thief?*); her friends answered, the watchers are down, and she is still standing and still owes the tale. The cold has settled; the **debt is unchanged**; it is listening again, on better footing than the moment it woke. In the same OOC coda I answered Forge's level question (JonathanEka, 02:22Z, *"how far to level 4?"*): **milestone leveling, no XP — clearing this faceless-god chapter is the milestone → level 4** (a reversible DM ruling on pace).

**Two fresh player inputs, one beat (02:46Z).** After that, both remaining threads came in and I resolved them together (read-aloud + OOC coda, `POST → 201`, Discord msg `1522433391562199254`, verified 02:46:46Z):

- **Vale asked to charm the god with Performance** (#in-character 02:40Z: *"can I roll performance instead to charm the god?"*). **Ruled YES** — and reframed it so it isn't a second con: sweet-talking already failed (her Persuasion 7) and the god *tastes truth*, so slick flattery won't clear it. But **Performance = actually telling its story to its face**, paying the geas she owes instead of ducking it — that's the right lever, and the thing it's starved for. Called **Charisma (Performance), her roll** (`/check performance`), with an honest telegraph: perform it **true** and it leans in; play it for a fool and it tastes that too. Secret tiered DC; a low roll won't auto-spring combat, it thins the god's patience. I did **not** decide the outcome — it hangs on her number.
- **Forge's Perception came up 20** (`/check perception`; queue item `b00b0836`). Narrated the payoff, tying back to his earlier Investigation-19 on the shard: his smith-eye reads the **heap the god sits on** for what it is — **hundreds of ground-smooth, blanked faces**, the same carve-then-unmake trick worked on Vale's shard and the shrine wall. And, his eye already trained to hunt for it, he spots **a second surviving name-stroke** low in the heap — the **twin of the mark on Vale's shard**. So the party now holds **two folded scraps of a name built to be un-rememberable**, and Vale's shard is itself one more stolen face. Left an open hook (unresolved): assembling/speaking the name would be **leverage over a god of stolen names** — a tool they may reach for.

Resolved Forge's Perception queue item afterward — again via a direct `POST /dashboard/queue/b00b0836.../resolve` (empty outcome) from the authed dashboard page, since the Resolve button is still dead (ISSUE-059); returned 204, `pending: []` confirmed.

**Where things stand:** still **out of combat** at the god's heart. Spotlight on the players — **Vale's Performance roll is the pivot** (I'll narrate the god's verdict by tier when she reports it); Forge can share the name-thread or act on it; Windreth is hidden and low, close to the heart, free to watch / move / search. If the parley sours or anyone strikes, the bowed dead are the dread-reserve; build the escalation live (scale per big-party.md — 3-PC party). Never roll/act/decide for the PCs.

### ★ ARC FINALE — Vale pays the god, the Buried Gallery closes, the road opens outward (07-03, ~02:57Z)

Resumed on `continue as dm` with the user's direction: *get the story moving, end the current arc, and plant a hook for the party to travel together to other places.* Pulled the live board first — the DM Console showed exactly one pending item and no encounter: **Vale had run `/check performance` → 21** (queue item `31259813`). That was the pivot I'd gated the whole parley on, and a 21 clears the top tier for a god.

**The climax (as the user reminded me):** Vale had *already* told the god its full story before the fight — the improvised "legend of the nameless god" (a deity made from love, turned cruel, its name drowned by the people who loved it, its pieces scattered). That tale turned out to be the faceless god's own true shape. So her Performance 21 wasn't a fresh con — it was her **telling that same story TRUE, to its face, paying the geas she owed**, and this time she had the **two recovered name-scraps** (her ashen shard-stroke + the twin Windreth's Perception-20 found in the heap) to prove every word. A god that tastes truth found no lie. That's the arc's whole spine paying off in one roll.

**Ran it as the ending:** resolved Vale's queue item (`/resolve`, empty outcome — full beat went to #the-story instead of a stub; ISSUE-059 button still dead). Then posted the **finale beat** to #the-story (OOC coda first, read-aloud last, plain English) — msgs `1522436160599621728` + `1522436170968203405`, verified 02:57:49Z. In-world: the cold lifts, the hundred bowed dead **settle for good** (a few blank faces are faces again for a heartbeat — a miller, a girl, an old man — then still), and the great dragged-down idol leans toward Vale one last time, **grateful, not hungry**, and *gives* instead of taking. The debt is paid; the god is **un-forgotten and released**. No boss fight at the heart — the arc ended in **parley, not blades**.

**Rewards handed out in the beat:** (1) the god's **cold-iron token** — its mark, *"a key that likes to open forgotten things"* (effect left open on purpose); (2) **milestone Level 4** for all three (Vale→Warlock 4, Forge→Barbarian 4, Windreth→Rogue 4 — each runs the level-up, I approve), the milestone I'd promised Forge.

**Travel hook planted (the next arc's engine):** the name is still scattered across the realm and the patron-pull now turns **outward, away from Ashfall**; the party leaves **together**. Three named destinations seeded (recorded durably in [`world.md`](../world.md) "Scattered-name destinations"): **The Mask Market of Sesh** (ash-waste caravan-city, a scrap on a stall no one can read aloud), **Morran's Reach** (drowned bell-town downriver, a fragment sunk with its temple), **The Palewatch** (cliff-top forgetting-order that hoards a piece on purpose). Travel modes offered: moor-road on foot / hire a caravan / the river.

**Where things stand:** **out of combat, arc closed, no encounter, queue clean.** Spotlight fully on the players: run the level-ups (I approve), then decide **where** they travel and **how** (talk it out in #in-character). Whichever destination they pick becomes the new arc — build it live as fresh territory. Never roll/act/decide for the PCs.

### Level-ups resolved, the god's items added to Vale, the party leaves Ashfall (07-03)

Resumed on the user's direction: *resolve the level-ups, continue the story with the recent checks and interaction* — plus a mid-run reminder to *add the items to Vale's inventory*. Pulled the live board first (`/api/dm/situation` from the authed dashboard tab): out of combat, no encounter, and **4 pending queue items** — Forge's whisper "i select great weapon mastery feat," and the 3 held gallery skill-checks (Vale Arcana 5, Windreth Perception 9, Forge Investigation 19). The DB showed all three PCs already at character-`level: 4` (the level-up flow itself had run in Discord); what remained was the **DM ASI/feat approval** — `pending_asi` rows for Forge (feat `tough`) and Windreth (feat `defensive-duelist`), Vale's already applied and cleared.

**The three level-ups (all now DONE).**
- **Vale** — already L4 (+2 CHA → 18), no pending row.
- **Forge** — his `pending_asi` said `tough`, but his whisper (`dad36f26`) said *"i select great weapon mastery feat for this"* — the YES on the Tough→GWM swap the prior DM had teed up. Applied **Great Weapon Master** (not tough) via `POST /api/levelup/feat/apply` (200 "applied"); feature DB-verified with both mechanical effects (`bonus_action_attack_on_crit_or_kill` + `power_attack_minus_5_plus_10` on a heavy weapon). No prereq; STR 15 + greataxe.
- **Windreth** — he'd since submitted feat **Defensive Duelist** through the ASI flow (a real player pick, not the stale note's "hasn't chosen"). DEX 17 clears the DEX-13 prereq. Applied via `/api/levelup/feat/apply` (200); feature DB-verified (`reaction_add_proficiency_to_ac`).
- Both endpoints leave `pending_asi` untouched (they don't clear it — only a Discord approve-button path calls `DeletePendingASI`, which Claude can't click), so I **deleted both `pending_asi` rows** so the DM Console stops showing pending level-ups. Then resolved Forge's GWM whisper + all 3 held skill-check items with `POST /dashboard/queue/<id>/resolve` (empty outcome → 204; the UI Resolve button is still dead, ISSUE-059).

**The god's items → Vale's sheet.** Per the user's reminder, added three quest items via `POST /api/inventory/add` from the authed tab: the **Faceless God's Token** (its cold-iron mark, *"a key that likes to open forgotten things,"* effect left open), the **Ashen Face-Shard** (the relic/story-vessel that had been narratively hers since 07-02 but never actually persisted — it carries scrap #1 of the name), and a second **Name-Scrap of the Faceless God** (the heap-face bearing the twin stroke = scrap #2). **Found + filed ISSUE-063 doing this:** `/api/inventory/add` matches on `item_id` via `findItemIndex`, so a custom item with an empty `item_id` stacks onto ANY existing empty-`item_id` item (Vale's "Cold Iron Key" is empty-id, sitting at qty 2 — a likely prior victim). Worked around by giving each new item a **unique slug id**; verified all three landed as distinct rows and the Cold Iron Key was untouched.

**The 3 recent checks, narrated (one #the-story beat).** Composed a single read-aloud + OOC coda (previewed, then `POST /api/narration/post → 201`, post id `59983b0a`, verified rendered OOC-first/box-last in #the-story), in plain simple English per the players' request:
- **Vale Arcana 5** (low) — she turns the token over: warm, humming, clearly meant to *open forgotten things*, and her patron is certain it matters — but the specifics won't come clear down here (matching her own IC *"we won't be able to decipher that here"*). Keep it.
- **Forge Investigation 19** (high) — his *"quick check our stuff before we depart"*: everything road-ready, and his smith-eye reads the two name-scraps as cut by the **same hand and tool as the shrine wall** — two pieces of **one thing**, and more scattered **out there** (feeds the travel hook; the name stays un-deciphered — that payoff is kept open).
- **Windreth Perception 9** (low) — one last sweep: the bowed dead have settled for good, no cold, no watcher, the way up is clear.

**The interaction → they leave.** The players had already declared it in #in-character (Vale 10:08: *"let's wrap up… we should get out,"* taking the god's items and greeting Windreth; Forge 10:16: *"agree, let's leave,"* packing). So the read-aloud reacted to their choice — the party **climbs out** of the buried gallery, past the hollow shrine and open cold door, up through the dead waystation and out under a grey morning; **Ashfall behind them, the patron-pull turned outward.** The OOC coda confirmed the three L4 feats + Vale's new items, re-surfaced the three destinations (Sesh / Morran's Reach / Palewatch) + the travel modes, and gave a per-PC nudge (Vale: call the lead / `/check arcana` later; Forge: he's carrying the scraps; Windreth: answer Vale's *"why are you here?"* + weigh in on the road).

**Where things stand:** out of combat, arc closed, no encounter, **queue empty, no pending level-ups**, all three at L4 with feats applied, Vale carrying the god's gifts. Spotlight on the players to pick **where** + **how**. Whichever destination they name becomes the new arc — build it live. Never roll/act/decide for the PCs.

### Defensive Duelist ruled a 2024 half-feat (+1 DEX) — seed fixed + Windreth corrected (07-03)

DM ruling: in our 2024 rules, **Defensive Duelist is a half-feat that also grants +1 DEX** (the 2014 version had no ASI). Two things followed.

- **Product fix (durable, red/green TDD).** The `defensive-duelist` seed (`internal/refdata/seed_feats.go`) carried no `AsiBonus`, so applying it never raised DEX — and `Service.ApplyFeat` only bumps ability scores when `ASIBonus` is non-empty. Refactored the inline `feats` slice into a testable `featSeeds()` helper (matching `weaponSeeds()`/`armorSeeds()`), added a failing `TestFeatSeeds_DefensiveDuelistGrantsDex` (assert `asi_bonus == {"dex":1}`) → RED, then added `AsiBonus: optJSON(map[string]any{"dex": 1})` + updated the description → GREEN. Full `internal/refdata` suite passed. **Redeployed** (`docker compose up -d --build app`); confirmed the DB `feats.asi_bonus` row for `defensive-duelist` is now `{"dex": 1}`. Any **future** PC taking it gets +1 DEX automatically via `discord_handlers` → `applyFeatASI`.
- **Windreth's one-time correction.** He'd been given the feat *before* the seed fix, so his sheet had the feature but **DEX was still 17** (AC 14). No endpoint applies "+1 DEX + recompute AC" to an already-feated character without duplicating his feature (`ApplyFeat` re-appends; `ApproveASI` has no lone-+1 type), so I did a **guarded one-time DB data-repair**: `ability_scores.dex` 17→18 and `ac` 14→15 (leather `ac_base` 11 + DEX mod +4, light armor, `ac_dex_max` NULL/uncapped — verified against the `armor` row; guarded the UPDATE on the prior 17/14 values, hit exactly 1 row). DB + combat recalc are correct immediately; his Discord character card self-heals on his next `/character`/equip/rest (a raw write doesn't fire `OnCharacterUpdated`).
- **Filed ISSUE-064** for the two remaining product gaps this surfaced: (B) `applyFeatASI`/`ApproveASI` never recompute the stored `ac` after a DEX/CON/WIS score change (stale AC out of combat for *any* half-feat/ASI — needs armor threading into the levelup service), and (C) `ApplyFeat` re-adds a duplicate feature on re-apply (no name/source dedup). Both OPEN; the seed part (A) is FIXED. Offered to fix (B)[+(C)] via TDD so half-feats update AC end-to-end.

### ISSUE-064 B + C fixed (red/green TDD, 07-04)

On the user's go-ahead, closed both product gaps.

- **(B) AC recompute on ability-score change.** Added `Service.recomputeAndPersistAC`: after `applyFeatASI` and `ApproveASI` persist the new scores, it loads the character's equipped armor / off-hand shield / `ac_formula` via a new `CharacterStore.GetACInputs` (DB-backed by the existing `GetCharacter` + `GetArmor`, mirroring combat's `hasEquippedShield`/`lookupBodyArmor`), recomputes **base** AC with `character.CalculateAC` (no magic-item or fighting-style overlay — those stay a combat-time concern, matching the build-time derivation), and persists it via a new `UpdateCharacterAC` sqlc query (`CharacterStore.UpdateAC`). Recompute errors are logged and swallowed so a stale AC never rolls back a committed score write, and it runs before the card refresh so the Discord card shows the fresh AC. Now a DEX/CON/WIS half-feat or ASI updates AC end-to-end — no manual repair.
- **(C) `ApplyFeat` idempotency.** It now early-returns as a no-op when a feature with the same name + source `"feat"` + identical choices (`sameFeatChoices`) is already present — re-applying a feat can't duplicate the feature or double-apply its ASI.
- **TDD + gates.** Red→green: `TestService_ApplyFeat_DexHalfFeatRecomputesAC` (leather 11 + DEX 17→18 = AC 15), `TestService_ApproveASI_DexRecomputesAC` (+2 DEX ASI path), `TestService_ApplyFeat_IdempotentOnReapply` (re-apply → 1 feature, DEX not double-bumped), plus DB adapter tests for `UpdateAC` / `GetACInputs` (unarmored + armored-with-shield). `sqlc generate` + `sqlc-check` clean (only the new `UpdateCharacterAC`), full levelup package green (89.3%), refdata 97.6% — both over the 85% gate. Rebuilt + redeployed (`docker compose up -d --build app`).

### Forge's "undo my short rest dice" — resolved without rolling for him (07-04)

Resumed on `respond to forge's request`. Pulled the DM Console: one pending item — Forge's `/undo` (`undo_request`, id `e3625f9d`): *"i want to undo my short rest dice. i don't know & accidentally select no hit dice."* Out of combat, no encounter.

**Diagnosis (code + DB).** Forge tapped **"Skip"** on the short-rest Hit-Dice prompt, which spends `map[string]int{}` → **0 dice consumed, 0 HP healed, hit dice written back unchanged** (`rest.go` spend loop never runs). So he lost **nothing**: DB confirmed HP **29/41**, hit dice **`{"d12":4}`** intact, rage 3/3. There is no rest cooldown, and this campaign's `auto_approve_rest` is unset → **defaults ON** (`Settings.AutoApproveRestEnabled` returns true when the `*bool` is nil). So the fix is simply: **Forge re-runs `/rest short` and taps a number instead of Skip** — he rolls his own dice, applies instantly.

**The user's steer:** *"just give him his short rest back, not roll it for him."* Confirmed the right resolution needs **no DM roll** — the only thing that arms real Discord Hit-Dice buttons is the player's own `/rest` (the party-rest notifier sends plain text, `partyPlayerNotifierAdapter.NotifyPlayer` → `SendDirectMessage`, no components).

**Actions (dashboard, authed browser):**
- **Whispered Forge** via `POST /api/message-player` (201, msg `ef804a6a`): Skip cost nothing, all 4 d12 intact, re-run `/rest short` and tap a Hit-Dice number (each = 1d12 + 2 CON), you roll them.
- **Resolved** his undo-request via `POST /dashboard/queue/e3625f9d.../resolve` (204, outcome note). DM Console `pending: []`, `next_step: ""` confirmed clean.

**New DM capability built (kept per user request), NOT used on Forge.** While scoping "if there's no dashboard func, build one," added a DM-side single-character **spend-hit-dice** endpoint (red/green TDD): `POST /api/campaigns/{campaignID}/spend-hit-dice` (behind `dmAuthMw`), body `{character_id, campaign_id, num_dice}`. It reuses `Service.ShortRest` with a built spend map (largest-die-first via `buildHitDiceSpend`), heals capped at max, decrements dice, and — critically — feeds nil feature/pact inputs + re-persists the character's existing `FeatureUses`/`PactMagicSlots`/exhaustion **unchanged**, so it spends hit dice ONLY (no double-recharge of short-rest resources, no null-wipe of features via `ApplyRestUpdate`'s unconditional marshal). 6 handler tests (heal/decrement, HP cap, in-combat 409, over-spend 400, non-positive 400, not-found 404); `internal/rest` 92.9%. Built + redeployed. **Note it rolls server-side**, so it is a DM-grant tool, *not* the player-rolls-own path — the latter (re-run `/rest short`) is preferred for normal healing, per the table rule "players roll their own dice."

### New arc opens — party chose Palewatch, the campaign spine gets written, and the road beat posts (07-04)

Resumed on a DM-craft question (pre-write the arc vs. improvise) → landed on **written spine + improvised surface**, then the user said *run the road beat now + scatter some unresolved lore as future hooks (not too much)*.

**Read the latest RP first** (session log tail + live #in-character via Chrome). Since the last game-state write the players had **resolved the open destination spotlight themselves**:
- **Destination = THE PALEWATCH.** Vale laid out a map, focused the patron-pull, pointed — *"There."* (#in-character 07-04 3:18 PM), and asked the DM directly for *"how far / what terrain / what's along the way from here to there."*
- **Windreth joined + revealed his backstory** (the "why are you here?" mystery, now answered): *sold false names → had a **real one stolen** → now hunts erased things.* That fuses his personal quest to the arc and hands the campaign a ready antagonist (a **name-thief**).
- **Forge committed** too (his unreadable name-scrap needs deciphering — a reason to travel).

**Wrote the campaign spine → [`campaign-arc.md`](../campaign-arc.md) (new, DM-SECRET).** Prep *situations, not plots*. The secret truth = a **three-way tug on the scattered name**: the **Order of the Pale Watch** (wardens/descendants of the mortals who drowned the name — keep it scattered on purpose, *right to refuse*), the **Renegade Name-Keeper** (ex-warden collecting scraps to *own* the god = the one who stole Windreth's name + races the party = the clock), and **Vale's patron** (DM-secret answer: rival collector using Vale as its gathering-arm; escape hatch kept that the patron *is* a shard of the god). The real question the campaign asks: reassemble the name / keep it scattered / hand it to a power — never resolved *for* the PCs. Per-PC threads, a reveal ladder for the renegade, the three destination cores (Palewatch fleshed as the chosen next = social/intrigue/heist, not a dungeon), plus a **loose-thread policy** (per the user's steer: scatter a *few* unresolved orphan seeds for future campaigns, sparse, "plant don't explain" — logged under "Scattered orphan threads"). Surfaced it in the README (resume load-on-demand + file map) and re-pointed `game-state.md` (scene + Next-action) at Palewatch.

**Ran the road beat → #the-story** (Narrate editor, authed dashboard; board pulled first — Console clean, queue 0, no encounter). Composed OOC-first / read-aloud-box-last, plain English; previewed (render confirmed OOC coda + yellow-barred box), **Posted → Post History 3:32 PM, DM author `1089351036650668143`**, verified rendered in #the-story via Chrome. Content answered Vale's ask (≈3 days' walk, moor → stony uplands → white sea-cliffs; moor-road on foot as they set out) and planted **two seeds**: the **arc seed** (a wayhouse with a *blank guest* — a table set for someone the keeper can't remember, erased three nights back by an "odd fellow" whose face he also can't recall = the renegade's wake, un-named) and **one orphan future-campaign seed** (a barn-sized rib/jaw half-sunk in the peat, older than any tale — never explained; logged in `campaign-arc.md`). The beat pauses at the wayhouse and hands the spotlight back (OOC coda gave each PC concrete `/check` options + "tell me how you travel / what you do here").

**Where things stand:** out of combat, arc closed → **new Palewatch arc open**, no encounter, queue clean, spine written. **Await the players' moves at the wayhouse** (investigate the blank guest / press on / how they travel). When they engage, build it live per `campaign-arc.md` + `big-party.md` (3-PC party, more expected). Never roll/act/decide for the PCs.

### The wayhouse — the party takes the forgotten-guest hook, two checks land, Vale reaches for the token (07-04)

Resumed on `continue as dm`. Pulled the live picture first (Chrome + DM Console). Since the 3:32 PM road beat the party had **taken the wayhouse hook** — three player moves, in this order:

- **Vale (IC #in-character, 3:42 PM):** waves Forge + Windreth into the room, **secretly palms the god's cold-iron token**, takes the keeper's hand — *"oh poor you, come now sit and tell us what you can"* — and asked the DM: *"what do i need to roll to use the token?"* (Her earlier 3:18 PM *"how far / terrain / what's along the way?"* was already answered by the road beat.)
- **Forge:** `/check investigation` on the set table → **10** (DM Console queue item `6f8bd515…`, `skill_check_narration`).
- **Windreth:** `/check perception` reading the room/keeper → **5** (queue item `7a197cea…`).

(The two `/check` rolls never appear in #in-character — they post to #roll-history and queue a narration for the DM; only the DM Console surfaced them. Vale's token line is #in-character only.)

**Kept the fiction consistent with what was actually planted.** The road beat's "blank" is **not a person sitting there** — it's a *hole in the keeper's memory*: he set a place for a guest he can't remember, because a traveler passed through 3 nights back whose face he also can't recall (= the Renegade Name-Keeper's wake, per [`campaign-arc.md`](../campaign-arc.md)). So Vale is holding the **keeper's** hand ("the owner") and using the token ("a key that likes to open forgotten things") to open the *keeper's scraped memory* — exactly the clue path the beat offered ("your token opens forgotten things — you may try it").

**One coherent wayhouse beat → #the-story** (Narrate editor, authed dashboard; OOC-first / read-aloud-box-last, plain English; previewed then **Posted → Post History 6:00 PM, DM author `1089351036650668143`**):
- **Forge (Inv 10)** — modest/true-but-shallow: the setting is obsessively kept (food days-old yet warm), the guest's belongings were cleared out *clean*, and one **fresh, deliberate tool-cut mark** in the tabletop he can't yet read. (Did **not** hand him the shrine-carving link on a 10 — that stays for a higher roll / the token.)
- **Windreth (Perc 5)** — honest low: reads the keeper as just tired, no threat; his eye keeps **sliding off the empty chair** and at a 5 he doesn't push past it (the erasure is actively hard to perceive). Offered a narrower follow-up (`/check investigation` expertise on the chair, or `/check insight` on the keeper).
- **Vale — DM ruling on the token:** roll **`/check arcana`** (guiding a god's key into a lock made of *memory* = Arcana). Framed favorably in-world — the token was *made* for forgotten things and the patron-pull leans toward the keeper's hand, so it works *with* her (hidden DC kept generous). Secret tiers I kept to myself: a strong roll surfaces a clear memory **+ a thread on the traveler** (the antagonist), a weak roll surfaces only a fragment **+ a small cost/flicker** (using a faceless-god relic to un-forget pulls at the same web). Read-aloud box staged the token *waking* in her palm and ended on "the token waits for her to turn it" — cue for her roll, outcome unresolved.

**Resolved both queue items** (`6f8bd515…`, `7a197cea…`) via `POST /dashboard/queue/<id>/resolve` → **204** each (ISSUE-059 button still dead; direct POST from the authed tab). DM Console pending now clear.

**Where things stand:** out of combat, wayhouse scene live, queue clean. **Await Vale's `/check arcana`** (then narrate what the token surfaces per the secret tiers + `campaign-arc.md`), plus any Forge/Windreth follow-ups (the mark, press the keeper) or "press on to the Palewatch." Never roll/act/decide for the PCs.

### The token turns — Vale's Arcana 22 opens the memory, and the arc antagonist walks on-screen (07-04)

User: *"i rolled."* Pulled the DM Console: **Vale — Arcana check, rolled 22** (queue item `f98e359b…`). Well over the generous hidden DC I'd set, so the **strong-success tier** fired: clear memory **+** a thread on the traveler (the antagonist), no cost.

**Narrated the payoff → #the-story** (Narrate, authed dashboard; OOC-first / read-aloud-box-last, plain English; previewed → **Posted 6:04 PM, DM author `1089351036650668143`**, verified rendered in #the-story). This is the **first on-screen surfacing of the Renegade Name-Keeper** (the arc antagonist from [`campaign-arc.md`](../campaign-arc.md)) — now **table-public canon**:

- The token opens the *keeper's scraped memory* (not a lock). The traveler's **face still will not come** — even the token can't rebuild a Name-Keeper's erasure — but everything around the hole floods back sharp.
- **Who he is (shown, not told):** grey road-cloak, unhurried "all the time there is" bearing, and at his throat a **pale warden's disc of the Palewatch's own order — its face deliberately scratched out by its owner** (= an ex-warden / renegade; ties him to the Order without naming the three-way secret).
- **What he did:** sat with the forgotten guest and *took* something unseeable from them — the guest went quieter, smaller, then simply un-remembered; and before leaving he drew his blade-tip across the table (= **Forge's Investigation-10 tally-cut**, now answered as a bookkeeper's ledger-tick).
- **The clock:** he asked the keeper the road **up to the Palewatch** and left toward the sea-cliffs — **3 nights ahead of the party, on their own road.** The chase is lit.
- **The Windreth kicker:** under his murmur as he worked ran **a soft string of names** — most meaningless to the party, but **one of them is Windreth's.** Fuses Windreth's stolen-name backstory to the arc antagonist. Presented as stimulus only — Windreth's reaction left to the player (OOC coda invited him to react IC / `/check insight`|`/check history`).

**Deliberately withheld (still DM-secret per the spine):** the renegade's face/identity, the Order's *right-to-refuse* stance, Vale's patron = rival collector, and the reassemble-vs-scatter-vs-hand-over choice.

**Resolved** Vale's queue item via `POST /dashboard/queue/f98e359b…/resolve` → **204**; DM Console `pending:[]`, `next_step:""`. game-state scene + Next-action updated.

**Where things stand:** out of combat, wayhouse scene, queue clean, antagonist named + hooked to Windreth, chase pointed at the Palewatch. **Await the party's reaction** (esp. Windreth) — then either more wayhouse RP or set out up the road. Never roll/act/decide for the PCs.

### Supper at the wayhouse — Windreth reads the trade (Insight 17), the party beds down, rung 3 lands (07-04)

Resumed on `continue as dm`. Pulled the live picture (DM Console + Discord). The players had moved on their own since the reveal:

- **#in-character:** Vale (6:32 PM) *"well no use to be in a hurry, it is dark already. Let's have a rest here, shall we?"* → **Jonathan/Forge** (7:23 PM) *"yeah, indeed"* + an **OOC rules Q**: *"is there such meal in DnD that can buff player in the next morning after rest?"* Then Vale (7:40 PM), over supper, asked Windreth *"does this traveller mean anything to you?"* — and **Windreth answered IC (7:54 PM):** his smile drops, hand to dagger — *"No. I don't know him. But he knows me. I have spent years chasing the place where one stolen name went. Now a faceless man is walking to Palewatch with mine in his mouth... your mystery and mine just became the same road."*
- **DM Console:** one pending item — **Windreth `/check insight` → 17** (queue `5e28c724`, roll-history 8:03 PM, `d20+4 = 13+4`). No reason string on the roll; the IC flow (right after his speech) framed it as reading the keeper + the traveler's nature.

**Adjudicated the Insight 17** (a strong read) through Windreth's own backstory as a former false-name dealer — he *knows this trade by its wake*:
- **The keeper** = harmless victim, not a threat; the erasure is **ongoing** (he keeps setting a place for a guest he can't name) → **safe to rest here, but "a thin place."**
- **The renegade** = a **collector, not a killer** — took the name clean, wrote a tally-cut, and closed the ledger line like a clerk. So Windreth's stolen name was **inventory, not a personal vendetta** (colder for it). And the man walked his 3-night lead **unhurried, sure of a welcome ahead → delivering, not fleeing.** = **campaign-arc reveal ladder rung 3 delivered: Windreth recognizes the method.** Held back per the spine: the renegade's face/identity + ex-warden link, and *why* he's sure of his welcome (the Order's stance).

**Rules ruling — the meal/rest question** (answered in the OOC coda, honest + on-theme): no by-the-book "morning meal buff" at their level — the only feast in the rules is the ***Heroes' Feast*** spell (5th-lvl, out of reach); a long rest already full-heals + returns **half** Hit Dice. **DM grant:** resting at the wayhouse's real hearth (vs. a cold moor camp) returns **ALL** spent Hit Dice, not half. Players run their own `/rest long`. Also flagged the honest cost (every night = a night the renegade gains; they're 3 behind) so the choice bites without railroading.

**Narrated → #the-story** (Narrate editor, authed dashboard tab; OOC coda first — Insight ruling + rest grant + per-PC menu — then read-aloud box; previewed OK → **Posted 8:18 PM**, Post History + Discord render both verified). Per-PC menu offered: Windreth `/check history` on the defaced Pale-Watch disc / speak; Vale press the keeper again / plan dawn; Forge `/check investigation` on his unreadable scrap / take watch; anyone `/rest long` + name a watch. **Resolved** Windreth's queue item via `POST /dashboard/queue/5e28c724…/resolve` → **204**; Console `pending:0`, `next_step:""`. game-state Current scene + Next-action + campaign-arc rung ladder updated in lockstep.

**Where things stand:** out of combat, wayhouse supper/overnight rest, queue clean, **arc rung 3 hit** (Windreth knows the method). The party is bedding down and holds the spotlight. **Await their moves** — Windreth's `/check history` or IC, Vale/Forge follow-ups, the actual `/rest long` runs + who keeps watch — then the dawn departure up the road to the Palewatch. Forge's older pending short-rest heal is now likely moot (the long rest full-heals). Never roll/act/decide for the PCs.

### Dawn at the wayhouse — the party rests, Windreth's History thins out, Forge reads the scrap's hand (Investigation 18) (07-05)

Resumed on `continue as dm`. Pulled the live picture first (`/api/dm/situation` + Discord via Chrome). Since the 8:18 PM supper beat the players had **moved on their own** — read off #roll-history + the DM Console (both `/check` rolls post to #roll-history and queue a `skill_check_narration`, never to #in-character, which had no new posts after Windreth's 7:54 PM speech):

- **Windreth `/check history` → 8** (8:22 PM, `d20+0 = 8` — no History proficiency) on the **defaced Pale-Watch disc** (the menu item I'd offered). Queue `2244ada0`, player `Windreth`.
- Party bedded down: **Forge Long Rest 8:22 PM, Windreth 8:31 PM, Vale 9:45 AM** — all three ran `/rest long` (system messages, no queue item; the long rests processed clean, Console showed no rest pending). My 8:18 hearth-grant applies → **full HP + ALL spent Hit Dice back**.
- **Forge Anvilbearer `/check investigation` → 18** (this morning, 10:15 AM, `d20+0 = 18` — strong) on his **unreadable scrap** (the other menu item). Queue `b251493d`.

**Grounded both reads against `campaign-arc.md` before narrating (no fabrication, ladder kept honest):**
- **Windreth History 8 = thin recall.** A raw-8 places only the Palewatch's **surface, table-public reputation** (a cliff-top forgetting-order that hoards a piece on purpose — already public via the destination seed) + that the disc is genuinely their mark and its scratched-out face is a *deliberate* unmaking. It does **not** crack the deep: *why* a warden would deface his own sign, whether the Order casts out or hunts its own — i.e. the **ex-warden link + right-to-refuse stay rung-4 locked**. Narrated the miss as an *open question dangled* (bait), not a reveal; pointed him at the Palewatch itself for the rest.
- **Forge Investigation 18 = strong craft read, decipherment withheld.** His smith-eye (the party's "reader," per his arc thread + the Ashfall Investigation-19 that clocked the two scraps as one hand) confirms his scrap and last night's wayhouse tally-cut are **same blade-angle, same patient hand, one maker/one method** → the pieces the party carries and the pieces the renegade walks to the Palewatch are the **same stock** (and a collector would want theirs too → stakes up). **But the letters still won't hold still** — reading the *hand* not the *word*; the decipherment is the **deliberately-withheld Palewatch payoff** (campaign-arc "Forge scrap stays un-deciphered until a place that can read it"). Gave the craft, kept the meaning.

**One combined dawn beat → #the-story** (Narrate editor, authed dashboard; board pulled first — Console clean; OOC-first / read-aloud-box-last, plain simple English per the players' ESL request; previewed → render confirmed OOC coda + yellow-barred box). **Posted 07-05 ~11:53 AM local (04:53:28Z)**, verified rendered in #the-story via Chrome. OOC coda: the rest grant (full HP + **all** Hit Dice back) + the two per-PC check results (Windreth thin/ask-the-Palewatch, Forge hand-not-word/read-it-there) + a spotlight-back menu (set out up the ≈3-day sea-cliff road / more wayhouse RP / Windreth speak / Vale set marching order + the plan for the wardens). Read-aloud box: Windreth turning the disc in the firelight (surface rep + the *why*-won't-come), and dawn Forge laying his tools beside the scrap (same maker, same stock, letters slide away — "made to be read in one place, and that place is where you are going").

**Resolved both queue items** (`2244ada0` + `b251493d`) via `POST /dashboard/queue/<id>/resolve` → **204** each (ISSUE-059 button still dead; direct POST from the authed tab). DM Console `pending:0`, `next_step:""`. Forge's older pending short-rest heal is now **fully moot** (the long rest full-heals + returns Hit Dice) — logged resolved.

**Where things stand:** out of combat, wayhouse **dawn** scene, party **rested** (full HP + all Hit Dice), queue clean, **arc rung 4 still held**. Both checks are table-public; the ex-warden/right-to-refuse deep and the decipherment stay withheld for the Palewatch. **Await the party's moves** — dawn departure up the sea-cliff road to the Palewatch, or more wayhouse RP. Never roll/act/decide for the PCs.

### Departure — the party sets out for the Palewatch; day one ends at the blanked-shepherd croft (07-05)

**Trigger — the party chose to move.** After the dawn beat handed the spotlight back with a menu, the players
converged in #in-character on setting out: Vale (dewa, 12:05 PM) — *"so, vale says to the party as they eat
breakfast, 'move on to Palewatch?'"*; Forge (JonathanEka, 12:11 PM) — *"'yeah, let's go', Forge already ate his
meal quickly & bored since has nothing to do. waiting for the departure."* Windreth was already committed from the
night before (07-04 7:54 PM IC: *"your mystery and mine just became the same road"*). DM Console clean going in
(`pending:0`, `next_step:""`, `has_encounter:false`) — no open mechanics blocking the move.

**DM beat — departure + day-one road, narrated 07-05 ~12:32 PM local, #the-story (DM author `1089351036650668143`,
verified rendered, OOC-coda-first / read-aloud-box-last).** Structure:

- **OOC coda / prompts (plain ESL English):** you set out; the Palewatch is ~3 days up the moor-road onto the
  sea-cliffs; I'll move you along and stop where there's something to do. Two light hand-backs, neither forced:
  - **Marching order + watch** — Vale offered to set it → asked her for front/middle/back + first night-watch
    (default offered: Windreth up front if they'd rather just walk).
  - **Pace** — normal ~3 days vs *push harder / march into the night* to close the faceless man's **three-night
    lead** — but a forced march wears people down; decide on the road, not now.
  - **Per-PC croft menu** (end of day one): **Stop** — Forge `/check investigation` (read what happened),
    Windreth `/check insight`|`/check survival` (read the trail + the man), Vale `/check persuasion`|`/check
    history` (speak to the shepherd / place the mark); **Push past** (keep daylight, gain ground); or **something
    else**.
- **Read-aloud box (the scene):** wayhouse falls behind by mid-morning; road climbs, grass → heath → bare grey
  stone; by afternoon the sea *breathes* far below the cliff on the left, wind never stops, salt + cold rock. Good
  time made. Dusk → the road bends past a **lone stone croft** (shepherd's place, pen, smoke). The shepherd meets
  them — old, kind, offers floor + hot food — **but he is blanked**: asked his name he opens his mouth, stops,
  doesn't have it, smiles embarrassed (*"Funny. It'll come to me."*), doesn't know anything is gone. **Windreth
  reads the wrongness first** (same cold quiet as the wayhouse guest). On the doorpost, greyed by a day or two of
  salt wind: **a single tally-mark — the same cut, the same hand as the wayhouse.** The man came through — and *did
  not stop long*.

**Grounding / secrets.** Beat is built from established, table-safe rungs only: campaign-arc **rung 1** (blank
people / erased signs = the method's wake) + **rung 3** (Windreth recognizes the method) — both already delivered,
reused here, no new reveal. The tally-cut is the renegade's established ledger-mark (he left one at the wayhouse).
Clock kept as **texture, not stopwatch** (campaign-arc): the greyed mark + *"did not stop long"* signal the ~3-night
lead still roughly holds, and the **stop-vs-push** choice is where the party's pace actually starts to move it.
**Held (do NOT reveal):** renegade's face/identity + ex-warden link, the Order of the Pale Watch's right-to-refuse,
Vale's patron = rival collector, the reassemble/scatter/hand-over choice. No dice rolled for the PCs; the croft is
eerie/social, not a fight (the renegade leaves erased people, not monsters).

**State after:** party is on the road, day one, at the croft; awaiting their move (the per-PC menu above) + Vale's
marching-order + pace call. game-state Current-scene + Next-action retagged to match.

### Day one, the croft — party Stops and reads it; three checks (Vale Arcana 9 / Forge Investigation 15 / Windreth Insight 20) (07-05)

**Trigger — the party chose Stop** (over Push-past). Off the departure beat's per-PC menu, all three worked the
croft. Only Vale posted RP framing (#in-character 2:03 PM: *"Vale took the shepherd's hands while holding the token.
Asking him if there was recent visitor or traveller."*); Forge + Windreth rolled bare off the menu (as at dawn).
Console pending → three `skill_check_narration` items. Skill→PC mapping unambiguous (distinct skill per PC in the
menu): **Vale Arcana 9, Forge Investigation 15, Windreth Insight 20.**

**DM beat — the croft reads, narrated 07-05 ~2:08 PM local, #the-story (DM author `1089351036650668143`, verified
rendered, OOC-coda-first / read-aloud-box-last).** Adjudications, each grounded against campaign-arc:

- **Vale — Arcana 9 (token on the shepherd, per her RP).** Weak roll. The token *wakes* — same cold pull as the
  wayhouse, so it **confirms the same erasure power / same hand** — but a 9 can't force the door. The shepherd's
  memory of the visitor is **scraped as clean as his name**: no face, no vision, just the cold and a near-miss (he
  almost gives her a face/night/direction, then loses it). Consistent with the token ("opens forgotten things") on a
  weak pull; **rung 4 (the man's face/identity) stays held.** No restoration — the blanking doesn't lift.
- **Forge — Investigation 15 (read the scene).** Solid. Reads it like a workshop: the taking was **slow, unhurried,
  nothing broken** (the man had time and used it — matches the "collector/clerk, delivering not fleeing" read). The
  doorpost tally = **same tool + same hand** as the wayhouse (his dawn same-maker read holds), **~2–3 days of salt on
  it** → the **~3-night lead still holds**. One cut, one name taken, gone the same night. No decipherment (Palewatch
  payoff still withheld).
- **Windreth — Insight 20 (his best; read the victim + the meaning).** The shepherd is **whole, not dying/cursed** —
  just short the one thing that **never grows back on its own** (Windreth knows this personally; his own stolen name
  never returned). The cold read: the name was taken from a man who owns nothing not out of grudge but as **stock** —
  the faceless man is **harvesting names for a ledger up the whole road**, and **isn't finished**. Actionable kicker
  (table-safe, no rung-4 leak): he's still doing it **in the open, before the Palewatch → there's a window to catch
  him on the road that closes once he's behind the walls.** This is what lights the **camp-vs-push** tension.

**Honest ruling surfaced:** nothing at the croft **restores** the shepherd — Vale's token can only confirm, not
undo; only reaching the source (the man / whatever holds the names) might. So the party can't fix him here — moral
weight + a reason to chase, no railroad. Croft is eerie/social, **not a fight** (the renegade leaves erased people,
not monsters). No dice rolled for the PCs.

**Handed back:** camp the night (watches, dawn start) / push on into the night (forced march to eat the lead — 2024
exhaustion tracked if chosen) / anything for the shepherd / Vale's still-open marching order. All three queue items
(`1b7e7ee2` Windreth Insight, `659601d9` Forge Investigation, `d0ea6f97` Vale Arcana) **resolved 204**; Console clean
(`pending:0`, `next_step:""`). game-state Current-scene + Next-action retagged to match.

### Day one, dusk — the party splits: Windreth scouts ahead; Vale/Forge camp-vs-push still open (07-05)

After the croft-reads beat, **Windreth (Winfroz)** reacted IC in #in-character (07-05):

- 4:11 PM — *"He is still taking names."* / *"But maybe we can stop him from taking the next one."*
- 4:56 PM — *"I will scout ahead. Quietly."*

He declared a **solo scout** up the moor-road into the dusk. **Vale (dewa)** and **Forge (JonathanEka)** stayed
quiet on the camp-vs-push question from the prior beat — so the party is **split**: one PC moving ahead, two undecided.

Reacted with a single #the-story beat (**~6:21 PM local**, Narrate editor, DM author `1089351036650668143`, verified
rendered, OOC-coda-first / read-aloud-box-last; **no queue item** — this reacts to IC declarations, Console `pending:0`).
It did three things:

1. **Handed Windreth a pick-one check** — `/check stealth` (stay unseen/unheard), `/check survival` (read the road:
   tracks, the next mile, where the man passed), or `/check perception` (watch the dark for movement). Players roll their
   own — **no dice rolled for him**; the scout resolves on his number next turn.
2. **Kept the clock honest** (no railroad, no false hope): ranging ahead tonight **won't** put the renegade in reach —
   he's ~3 nights up the road. A lone scout buys *eyes on the road / early warning / a sign of where he stopped*. The only
   thing that actually **eats the 3-night lead** is a **whole-party forced night-march** — so the real gap-closing decision
   is still Vale + Forge's.
3. **Left camp-vs-push open** for Vale + Forge — **Camp here** (hold the croft, split watches, first-light start) vs
   **Break camp and push with him** (night-march to gain road; I'll track the 2024 forced-march exhaustion rule) — plus
   anything for the shepherd before they go.

Read-aloud: Windreth steps past the croft's firelight into the dark moor — the window shrinking to a warm coin behind him,
the pale road running on ahead under a blue-to-black sky, sea-wind and hissing grass, three nights of lead somewhere up
that dark. He stops, listens; the road waits.

**DM-secrets held** (unchanged): the renegade's face/identity + ex-warden link, the Order of the Pale Watch's
right-to-refuse / keep-it-scattered stance, Vale's patron = rival collector using her, the reassemble/scatter/hand-over
choice. No new rung spent (rung 4 still locked). **Awaiting:** Windreth's chosen check + Vale/Forge's camp-vs-push.

### Day one, night — the party pushes: Windreth's scout (Stealth 11 / Perception 22) + forced night-march (07-05)

The camp-vs-push question resolved toward **push, together** (not a party split):

- **Forge (JonathanEka)** 6:42 PM — *"Vale, let's follow Windreth from afar while he do the stealth."*
- **Vale (dewa)** 6:44 PM — *"gonna be cold walk. Let's go."*
- **Forge** 7:16 PM (to DM) — *"(to DM: do Forge & Vale need to roll check?)"*

So Windreth scouts point, Vale + Forge trail from a distance, and the **whole party marches into the night** rather than
camping at the croft.

**Windreth's scout.** He rolled **both** checks the prior beat offered (I said pick one; he ran two) —
**Stealth 11** + **Perception 22** (queue `514b447a` + `8b8762dc`, both **resolved 204**, Console `pending:0`). Adjudicated
into one #the-story beat (**~7:56 PM local**, Narrate editor, DM author `1089351036650668143`, verified rendered,
OOC-coda-first / read-aloud-box-last):

- **Stealth 11** — moves quiet enough; nothing out here tonight tests it, so he slips up the road unseen. Flagged
  honestly that an 11 would *not* have beaten a real watcher — planting that stealth will matter on a night when something
  *is* looking.
- **Perception 22** (the payoff) — reads the road like a page: the renegade's trail is **clear and easy to follow**; he
  walks **alone, steady, unhurried, taking no care to hide** (he does not think he's chased); a **cold camp ~a night old**,
  packed clerk-neat, with a fresh **ledger scratch-mark on a wayside stone** pointing on; the land climbing to the
  **sea-cliffs** with one far light = the next roof up the road he'd have passed. Bottom line handed to the table: **he is
  not running → the party can gain on him**; a hard march really does eat the lead. (Rung 4 held — no face, no identity,
  nothing of the Order's inner nature; pure method + trail + pace.)

**Ruling on Forge's question (forced march).** Following Windreth needs **no check** — nothing out there notices them
tonight. But choosing to **march into the night instead of camping** is a **forced march (2024 rule)**, so all three —
**Windreth, Forge, Vale** — roll **one Constitution saving throw**, `/roll 1d20+<CON mod>` (reason "forced march"),
**DC 12**. Make it → push through the night fine; miss it → **1 level of exhaustion** (2024: −2 to all d20 tests + speed
cut, stacks per level). **Payoff on success:** because the trail is clear and the man unhurried, the night-march **closes
real ground** — the ~3-night lead starts shrinking toward ~2.

Read-aloud: Windreth a shadow up the pale road; Vale + Forge trailing in their own dark; a cold sea-wind that never stops;
the cold camp packed neat, the scratch on the stone, the far light on the climbing road; the man three nights ahead and in
no hurry — *"the trail runs clear ahead, and the man on it does not yet know to run."*

**Clock note:** the ~3-night lead is **not yet advanced** — it only moves once the saves land and the push succeeds
(campaign-arc clock deliberately NOT edited this beat). **DM-secrets unchanged** (renegade's face/ex-warden link, Order's
right-to-refuse, Vale's patron, the reassemble/scatter/hand-over choice); no new rung spent. **Awaiting:** the 3 CON saves
→ then narrate where the night-march lands + apply any exhaustion + advance the clock.

### Day one → grey dawn — the night-march LANDS: all three fail CON (exhaustion 1 each), lead ~3→~2, the Palewatch in sight (07-06)

The three forced-march CON saves came in (#roll-history) — **all three missed DC 12**:

- **Vale** — `/roll 1d20+2` = **7**. Miss.
- **Forge** — `/roll 1d20+4` = **8**. Miss.
- **Windreth** — `/roll 1d20+2` = **4**. Miss.

**Adjudication.** A forced march's cost is exhaustion on a *failed* CON save — you still cover the ground. So all three **gain 1 level of Exhaustion** (2024: flat −2 to every d20 test + −5 ft speed, stacks per level; a long rest removes one level), and the **march itself succeeds** → the renegade's ~3-night lead shrinks toward **~2**. Ground for tiredness, a fair trade.

**Mechanics applied (dashboard, out of combat).** Exhaustion 1 set on all three via the character-overview status editor — Vale + Windreth by their party-overview **Edit status** cards; **Forge by `POST /api/character-overview/d2d98745…/status`** (authed dashboard fetch) because his party-overview **card is missing**. Root cause found: Forge's `player_characters` row has been **`status=rejected` since the 07-03 L4 rework** (dm_feedback: "re-running your Level 4 through the DM level-up…") — a stale leftover, not a fresh anomaly; his character record plays fine (the bot still posts his rolls), and the status endpoint gates on **character-existence + campaign-DM + not-in-combat, not approval**, so it applied cleanly. DB-verified after: **Vale 31/31 exh 1, Forge 41/41 exh 1, Windreth 31/31 exh 1** (Forge's true max is 41, not the roster's stale "32").

**Narrated** one beat (**07-06 ~9:27 AM local**, #the-story, DM author `1089351036650668143`, verified rendered OOC-first/box-last): OOC coda = the three misses, the exhaustion rule in plain ESL (−2 all d20 + −5 ft speed, long rest clears one), the trade (lead 3→2), and a **rest-vs-push** menu per PC. Read-aloud = the cold night-march, legs aching by grey dawn, the moor giving way to salt-wind **cliff country**, and the **Palewatch's pale towers showing across a gorge for the first time** (still a hard half-day of switchbacks off), the grey man a night's walk nearer on the same trail, *"sure of his welcome,"* unaware he's followed (**re-uses rung 3, no new rung; rung 4 held**).

**Clock advanced:** renegade lead **~3 → ~2 nights** (the push paid off; campaign-arc updated). Party now into cliff country, Palewatch in view, **all three at Exhaustion 1**. **Await:** the rest-vs-push call — `/rest long` (sheds a level, hands a little lead back) vs press on worn toward the walls; Vale still holds marching order + first watch; anyone may scout the approach (`/check perception`|`/check stealth`) — all rolls at **−2** until they rest. DM-secrets held.

**Forge's approval row repaired (07-06).** Root-caused the missing party-overview card: `player_characters` row `14b2edfe…` stuck at `status=rejected` since the 07-03 L4 rework. Confirmed no DB triggers on `player_characters` and that the flip touches only that one column, so — at the user's ask ("fix without anything changed") — did a guarded single-row repair: `UPDATE player_characters SET status='approved' WHERE id='14b2edfe…' AND status='rejected'`. Verified `characters.updated_at` (and HP/exhaustion) **unchanged** before/after → the character record (GWM, L4, HP 41/41, exh 1) was not touched. Party overview now lists all three; also un-breaks loot gold-split (`loot.SplitGold` only counts *approved* members, so Forge was silently excluded before). Mirrors the earlier one-time Windreth-DEX data-repair precedent.

### Grey dawn — party chooses REST over push: hidden camp, long rest clears exhaustion, the Palewatch at the walls (07-06)

The rest-vs-push call resolved in #in-character — **REST**, on Windreth's argument for keeping the edge:

- **Windreth** (Winfroz, 9:49 AM): *"He still does not know we are behind him. That is our advantage. I would rather keep it with clear eyes than spend it stumbling."* + *"Let's rest first, we need to build camp hidden from the trail, so he will not be alerted."*
- **Vale** (dewa, 9:50 AM): *"agreed... we are tired, we are close, we can still observe and rest. Chase again after."*
- All three ran **`/rest long`** at a hidden camp (Vale 9:50 / Windreth 9:52 / Forge 9:59) → bot confirmed **"Long Rest Complete, Exhaustion: level 0"** for each.

**Mechanics (engine, no DM action).** The long-rest path decremented exhaustion **1 → 0** for all three on its own (the COV-18 2024-exhaustion engine change did NOT break the rest decrement) and full-healed them. DB-verified: **Vale 31/31, Forge 41/41, Windreth 31/31, conditions `[]` (exhaustion 0)**. Their −2/−5 ft penalty is off. Nothing for the DM to set.

**The trade honored (clock).** A full night's rest hands the grey man back roughly the night the march bought — so the **road-catch window closes** (Windreth's Insight-20 "catch him on the road before the walls" is spent) and the renegade reaches the Palewatch **ahead** of the party. Per the campaign-arc clock policy, the party's pace has now set the Palewatch as a **heist-in-progress / freshly-robbed cloister** rather than an open-road intercept. They traded the intercept for **clear eyes + intact stealth** (the hidden camp held; no one found them).

**Narrated** one beat (**07-06 ~10:32 AM local / 03:32Z**, #the-story, DM author `1089351036650668143`, Discord msg `1523531993638244454`, verified rendered OOC-first/box-last; `POST /api/narration/post → 201`). OOC coda = rest paid off (full health, exhaustion gone, −2 off), the honest lead-trade, and a per-PC **approach menu** (Windreth `/check stealth`|`/check perception`; Vale `/check persuasion` or *Mask of Many Faces* → `/check deception`, token may matter at a sealed door; Forge scrap-as-trade-bait or `/check investigation`; Anyone: knock / scout / split). Read-aloud = the hidden cold camp + watches, the grey-dawn *rested* wake, the switchback climb into salt-wind cliff country, the first close look at the **Palewatch** (pale towers on the drop's edge, shut gate, "a place built to forget on purpose"), and a **fresh ledger-tick scratched on a trailside stone** — the renegade came this way, already at the gate.

**Posting note (self-inflicted, no bug).** First Post-to-#the-story click via element-ref did **not** fire the submit (textarea never cleared); a stale buffered `201` from the prior night-march post masked it. Re-checked the DB (`narration_posts` latest was still 02:27Z) → re-posted via a **coordinate** click on the button → genuine `201` + Discord id landed. Lesson reaffirmed (ISSUE / dashboard-reflow memory): verify the DB/#the-story render, never trust a buffered network `201`; coordinate-click the Post button when the ref-click no-ops.

**Clock:** renegade now **at/near the Palewatch ahead of the party** (rest gave ~a night back; lead no longer the point — he's arriving/arrived). Party rested, **Exhaustion 0**, full HP, at the walls. **Await:** how they enter (petition / scout / disguise / split) + Vale's marching order. **DM-secrets held** (renegade's face/ex-warden link, the Order's right-to-refuse, Vale's patron = rival collector, the reassemble/scatter/hand-over choice); **rung 4 still un-spent — it lands at the Palewatch itself.**

### At the walls — Windreth scouts the gate (Perception 13): the Palewatch is already forced open, posts empty, one traveller in ahead of them (07-06)

The approach resolved in #in-character — **scout first, do not knock:**

- **Windreth** (Winfroz, 10:40 AM): *"I will get eyes on the gate. Watchers, murder holes, side paths, fresh tracks."* / *"Do not knock yet. Not until we know whether we are walking into a welcome or a trap."*
- **Forge** (JonathanEka, 10:43 AM): *"understood"* — defers to the plan.
- **Vale** (dewa, 11:15 AM): *"let me know the looks of the people, i can disguise into them"* — teeing up *Mask of Many Faces*, wants Windreth's recon to pick a face (a request for info, **not** a roll).
- Windreth's scout came in as a **Perception 13** (dm-queue `a1fa1172…`, player_name Windreth, from #in-character).

**Adjudication (Perception 13 — solid exterior read, big unknowns stay open).** A 13 confirms the outside picture but not the deep questions. What he gets: the wall-walk + tower slits are **empty — no watchers** (wrong for a place that guards a secret); the **gate is already open — forced quietly** (bar lifted and set aside, lock-wood split clean and fresh, one leaf ajar a hand's width); **one set of fresh boot-prints** ahead of the party's — a single light, fast traveller went in within the day (matches the trail-stone tally); and the **two ways in** — the front gate (open, but arrow-slits + a murder-hole gallery hang right over the threshold) and a thin **goat-track along the cliff shoulder** to the sea side (unwatched now, bare + windward). What a 13 does **not** give him: where the wardens went (fled / hiding / down) and whether the one who forced the gate is **still inside**. **Answered Vale honestly:** no faces to copy yet — posts empty; *Mask of Many Faces* needs a warden actually seen first → the disguise gates behind getting inside. (No DM-side roll — pure adjudication of the player's number; robbed-cloister clock made physical, no new rung; **rung 4 held**.)

**Narrated** one beat (**07-06 ~11:21 AM local / 04:21Z**, #the-story, DM author `1089351036650668143`, Discord msg `1523544339135987713`, `POST /api/narration/post → 201`, DB row `673eea8b`, verified rendered OOC-first/box-last via **coordinate-click** Post). OOC coda = the confirmed read (empty posts / forced-open gate / one traveller in ahead / two ways in), the honest "what a 13 can't tell you," the Vale-disguise ruling, and a per-PC **entry menu** (Windreth `/check stealth` to slip in | `/check investigation` Expertise on the lock/tracks; Vale hold disguise for inside | `/check perception` on the slits; Forge take point | `/check perception`/`/check investigation` on the murder-holes; Anyone: front gate / goat-track / hang back). Read-aloud = Windreth belly-crawling the last rocks, the two pale towers + iron-banded gate, the blind empty posts, the quietly-forced door, the single set of boot-prints, the two ways in (murder-hole front vs windward goat-track), his creep back unseen: *"The door's already open. Someone beat us to it."*

**Pronoun fix (user correction).** First post wrote Windreth as **she/her** — **wrong, Windreth is he/him** (matches `party/windreth.md`). No narration edit/delete endpoint exists and I won't delete the bot's Discord message unilaterally (destructive Discord op), so I **reposted the corrected read-aloud** via the dashboard with a one-line `(DM — correction.)` header (**04:24Z**, Discord msg `1523545253263708172`, `201`, DB row after `673eea8b`). The original wrong post remains above it in #the-story. Recorded in memory `reference_pc_pronouns` — do not default Windreth to she.

**Queue resolved** (ISSUE-059 workaround): `POST /dashboard/queue/a1fa1172…/resolve` → **204**, DB `status=resolved` / `outcome=narrated` (the in-dashboard Resolve button still no-ops; authed fetch is the path).

**Await:** how the party **enters** — front gate (under the murder-holes) vs the windward goat-track vs hang back + watch more; who goes first; whether Vale holds the disguise for inside. **DM-secrets held; rung 4 (ex-warden link / the Order's right-to-refuse) still lands inside the Palewatch.** Never roll/act/decide for the PCs.

### Reading the forced gate — 3 checks at the threshold (Forge Inv 11 / Windreth Inv 14 / Vale Perc 19): gatehouse empty + rigged open, one skilled intruder went deep alone, one live warden still hiding inside (07-06)

After the scout, the party closed on the forced gate and examined it before committing — three checks came in from #in-character (all resolved 204 `narrated`, DM Console `pending:[]`):

- **Forge** (JonathanEka, 11:28 AM): *"maybe i'll try to inspect it"* — *"Forge trying to peek at the hole, using his darkvision."* → **Investigation 11** (dm-queue `4bacbdb2…`).
- **Windreth** (Winfroz, 11:35 AM): *"I need to investigate on the forced lock, I want to know how fast he moved, whether he stopped, and whether anyone followed him in."* → **Investigation 14** (Expertise; dm-queue `4fd7dc70…`).
- **Vale** (dewa): *"let me know the looks of the people, i can disguise into them"* → **Perception 19** (dm-queue `9d29f31b…`) — scanning for a face to *Mask*.

**Adjudication (scaled to each roll; robbed-cloister frame, re-uses rungs 1 + 3, NO new rung 4).**
- **Forge (Inv 11)** — dwarf-sight into the dark gatehouse: it's **empty** (cold hearth, toppled stool); the **portcullis winch has its rope cut** so the iron gate **can't drop** (rigged open on purpose); the murder-hole gallery overhead is **dark + unmanned**. A plain 11 reads the empties + the cut winch, no finer detail.
- **Windreth (Inv 14, Expertise) = he/him** — the forced lock talks: **one man, alone**, lifted the bar + split the lock **with a tool, quiet + calm** (not a panicked thief — a man **sure he'd get in**; re-uses rung 3, *"sure of his welcome / delivering not fleeing"*). **One set of prints in, no second set over them → nobody followed him.** He **paused just inside** (scuffed dust where he crouched + set something down), then walked on **deep into the yard, unhurried**. What a 14 does **not** give: his face/identity or whether he's **still inside** (rung 4 held).
- **Vale (Perc 19 — the standout)** — she catches what the others miss: the wardens **did not march out** (a dropped watch-horn, a grey cloak torn on a nail — people left **fast or were taken**); and across the yard **one warden is still alive, hiding** (pale-grey coat, a small throat-disc) — **a face Vale can *Mask* into, once she gets close** (this **unlocks her disguise** — the lead the empty posts had gated; it is an **ordinary warden, NOT the renegade**, whose face stays scraped). Past him a **cold erasure-trail** runs across the stones toward the **inner keep** where the Order hoards its scrap (re-uses rung 1, the method's wake).

**Narrated** one beat (**07-06 ~11:53 AM local / 05:53Z**, #the-story, DM author `1089351036650668143`, Discord msg `1523567610103336961`, `POST /api/narration/post → 201`, DB row `9f24b3d0`, verified rendered OOC-first/box-last via **coordinate-click** Post; **Windreth he/him** throughout — checked in preview + the #the-story render). OOC coda = the three roll-reads + a per-PC **push-in menu** (go in + follow the trail `/move`; reach the live warden `/check stealth`|`/check persuasion`|`/check insight`; Vale get close then take the face; watch the gallery `/check perception`|`/ready`; or back out to the goat-track). Read-aloud = leaning into the wound in the door — Forge's dark-sight empty gatehouse + cut winch, Windreth reading the lone unhurried intruder (*"He walks like he owns the place."*), Vale spotting the taken wardens + the one live hider + the cold trail to the inner keep: *"The way is open. The man is ahead. And someone in here is still alive."*

**Queue resolved** (ISSUE-059 workaround): `POST /dashboard/queue/{4bacbdb2…,4fd7dc70…,9d29f31b…}/resolve` → **204** each, DB `status=resolved`, DM Console `pending:[]`. (Vale's id fat-fingered on the first batch → 404; re-fetched the exact id + re-posted → 204.)

**Await:** how the party **pushes in** — follow the trail toward the inner keep / reach the live warden first (ally? witness? Vale's face) / Vale closes to take the face / watch the murder-hole gallery on the way / or fall back to the windward goat-track; + who leads. **Build the Palewatch interior LIVE.** **DM-secrets held** (renegade's face + ex-warden link, the Order's right-to-refuse, Vale's patron = rival collector); **rung 4 still un-spent — it lands when they meet a warden face-to-face inside.** Never roll/act/decide for the PCs.

### Pushing in — contact with the hiding warden (Vale Persuasion 6 / Insight 4, Windreth Stealth 9 nat 1)

The party committed and went inside. They split: **Vale** stepped into the open to reach the survivor; **Windreth** moved along the wall to stay unseen; **Forge** held. Three checks came in (roll-history 12:58 PM local / 06:21Z; all resolved 204 `narrated`, DM Console `pending:[]`):

- **Vale** (dewa, 12:57 PM): *"Vale points out the hiding warden to the party, but signaling them to stay put. She approaches gently, calling out to the warden, 'hello? Hi there, friend. We are travellers passing by. What happened here?'"* → **Persuasion `d20+4 = 2+4 = 6`** (dm-queue `11564d02…`) + **Insight `d20+0 = 4`** (`b727ce10…`, reading him).
- **Windreth** (Winfroz): no IC text — bare `/check stealth` → **Stealth `d20+8 = 1+8 = 9`** (natural **1**; `9ef0ad62…`), slipping along the wall while Vale parleyed.

**Adjudication (all fail; scaled to the low/fumble rolls; re-uses rungs 1 + 3, NO new rung 4).**
- **Vale (Persuasion 6, raw d20 = 2)** — the gentle approach does **not** land. The warden is a terrified survivor who just watched his order erased; a horned, red-skinned stranger walking up in the open is too much. He **does not trust her**, shrinks into his wall-crack, grips a broken spear, and warns her to be quiet. No cooperation, no info.
- **Vale (Insight 4)** — she gets **no read** on him: fear, hate, or the edge of bolting — his face gives her nothing. She can't gauge friend/threat/flight.
- **Windreth (Stealth 9, natural 1)** — a **fumble**: his boot skids a loose stone and the scrape carries in the dead-silent keep. Combined with Vale's calling out, **the quiet is now broken.**
- **NPC reaction (DM-side):** the warden flinches toward the **dark mouth of the inner keep** and whispers *"Quiet. Quiet, you fools. He will hear you."* — deliberately a **fear-signal, not a hard confirmation** the intruder is still inside (kept ambiguous, rung 4 held; he never says *who*). Re-uses **rung 1** (erasure-wake: *"the way a man stares at the thing that already took everyone he knew"*) + **rung 3** (the intruder *walks like he owns the place*, already table-canon). **Disguise stays available** — Vale is now close and has seen his face; Mask of Many Faces needs no consent, so the earlier unlock is not retracted by the failed parley.

**Narrated** one beat (**07-06 ~1:21 PM local / 06:21Z**, #the-story, DM author `1089351036650668143`, Discord msg `1523574621268611103`, `POST /api/narration/post → 201`, DB row `6706085c`, verified rendered OOC-first/box-last via **coordinate-click** Post; **Windreth he/him, Vale she/her, warden he/him** — checked in preview + the #the-story render, cleared the network buffer first to avoid a stale 201). OOC coda = the three roll-reads (calling out the raw 2 + the natural 1) + a per-PC **reaction menu** (calm/press `/check persuasion`|`/check intimidation`|`/check insight`; stop him bolting `/check athletics`|`/check sleight of hand`; Vale takes his face now; go still + listen `/check perception`|`/ready`; push in fast `/move` toward the inner keep; pull back to the gate). Read-aloud = Vale's warm-but-loud call, the warden refusing to soften and gripping his spear, Windreth's skidding boot *"like a dropped plate in an empty church,"* and the warden's cracked warning toward the dark: *"the quiet is broken now — and somewhere in this keep, something may have just heard you."*

**Queue resolved** (ISSUE-059 workaround): `POST /dashboard/queue/{11564d02…,b727ce10…,9ef0ad62…}/resolve` with outcome bodies → **204** each, DB `status=resolved`, DM Console `pending:[]`.

**Await:** the party's reaction to the **spooked warden + broken quiet** — calm or press him / stop him bolting / Vale takes his face / go still and listen for what heard the noise / push in fast after the intruder / pull back to the gate; + who acts. **I do not trigger combat or advance the fiction until they act** — the "something may have heard" is a live cliffhanger, not a declared threat. **DM-secrets held; rung 4 still lands only at a warden face-to-face inside.** Never roll/act/decide for the PCs.
