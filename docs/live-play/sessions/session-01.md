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
