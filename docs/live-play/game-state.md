# Game State — live save file

> **Update this file as play advances.** It is the single source of truth for
> "where are we right now." Timestamps in the campaign's local fiction are loose;
> real-world dates are absolute.

_Last updated: 2026-06-26 (session 1 — **COMBAT OVER. Victory.** The wretch (Ghoul G1)
is dead; the DM **narrated the kill** to #the-story (read-aloud, 2026-06-26 13:45 UTC,
Discord msg `1520062389649670288`) and **ended the encounter** via the new dashboard
**End Combat** button — encounter `status=completed`, Vale's hold-person concentration
auto-cleared, both PCs full HP (Forge 32/32, Vale 24/24, 1 pact slot left). The party
stands over the corpse in the waystation common room; the cellar still gapes, clawed from
the inside. **Next scene = the cellar descent** (reserve wretch lives down there).
Shipped this beat: an **End Combat button** on the Combat Manager (it didn't exist — only
End Turn), wired to `POST /api/combat/{id}/end`, TDD'd + redeployed; and a README
"keep narration + docs in lockstep" diligence rule (this session's whole drift was the
failure mode it prevents).)_

## Stack status

- **UP** via `make local-up` (docker compose). App `localhost:8080`, DB
  `localhost:5432`. Bot `DnDnD` (id `1507904367301496862`) connected to guild
  `DnDnD`.
- **Rebuilt + redeployed ~13:45 UTC (2026-06-25)** via
  `docker compose up -d --build app` to ship the **ISSUE-014** fix (`main` `f1e3aeb`,
  pushed) — the DM Console now records player combat actions to `action_log`. Clean
  boot: "database connected and migrated" (no new migration), "discord session
  opened", all discord checks passed for guild `1507910398886543532`, server on
  `:8080`, no panic/error. **Combat state preserved across the redeploy** (still
  Round 1, Vale's turn active, wretch paralyzed — see Encounter / combat below).
- **Redeployed AGAIN ~22:50 UTC (2026-06-25)** via `docker compose up -d --build app`
  to ship two more live-play fixes in one commit (`main` `b108bf2`, pushed
  `0dfa1ec..b108bf2`): **ISSUE-016** (`/done` no longer warns of a phantom attack after
  a player casts a spell with their action) + the **ISSUE-015 DISPLAY half** (the Combat
  Manager now renders conditions via `conditionName()` instead of "[object Object]";
  the WRITE half of ISSUE-015 stays OPEN). Clean boot: "database connected and migrated"
  (no new migration), "discord session opened", all discord checks passed for guild
  `1507910398886543532`, server on `:8080`, no error. **Combat state preserved again**
  (still Round 1, Vale's turn active, wretch paralyzed D7 15/22, Forge E7 32/32, Vale
  K6 24/24 concentrating with 1 pact slot left).
  - **Cosmetic caveat:** Vale's *current* turn still carries the pre-fix
    `attacks_remaining=1` (the ISSUE-016 fix only affects casts made on the new binary),
    so `/done` will still warn **once** for this turn — just confirm past it; her next
    cast is clean.

### Remote-player access (cloudflared tunnel) — 2026-06-25

- A second player (the DM's friend) is joining remotely. The local app is exposed
  via a **cloudflared quick tunnel** so he can reach the web character builder +
  Discord OAuth from his own location.
- **Managed by `make tunnel-up` / `make tunnel-down` / `make tunnel-status`**
  (`scripts/tunnel.sh`). `tunnel-up` auto-installs cloudflared to `bin/`, opens the
  tunnel, repoints `.env`, restarts the app, and prints the OAuth callback to
  register. `tunnel-down` stops it and restores `.env`. State lives in `.tunnel/`
  (gitignored). The current live tunnel was started manually this session and
  adopted into that state, so the make targets manage it.
- **Public URL (EPHEMERAL):** `https://coupon-affiliates-foto-employees.trycloudflare.com`
  (updated 2026-06-26; was `pillow-reproduction-centers-feel…`) — changes every time
  the tunnel restarts. On change, `make tunnel-up` redoes the `.env` repoint + app
  restart automatically; you still must re-register the new `…/portal/auth/callback`
  in the Discord dev portal.
- `.env` changed: `BASE_URL` + `OAUTH_REDIRECT_URL` now point at the tunnel
  (backup: `.env.bak.preTunnel`). App restarted; OAuth `redirect_uri` confirmed =
  `<tunnel>/portal/auth/callback`.
- **Manual step still owed by the DM:** register
  `https://coupon-affiliates-foto-employees.trycloudflare.com/portal/auth/callback`
  in Discord Developer Portal → app (`DISCORD_CLIENT_ID` 1507…) → OAuth2 →
  Redirects (Discord rejects unlisted redirect URIs → login fails without it).
  Verified 2026-06-26 via Chrome: tunnel reachable + app 302s to Discord OAuth, but
  Discord returns **Invalid OAuth2 redirect_uri** until this callback is registered.
- **Teardown after the test:** `make tunnel-down` (stops cloudflared, restores
  `.env` from `.env.bak.preTunnel`, restarts the app). App is publicly reachable
  while the tunnel is up (login gated by OAuth; build gated by a minted token;
  dashboard gated by DM auth).

## Campaign

| Field | Value |
| --- | --- |
| Campaign ID | `532b4774-47ff-4f83-b591-632ce3509e40` |
| Name | "Campaign for guild 1507910398886543532" (unrenamed) |
| Guild ID | `1507910398886543532` |
| DM user ID | `1089351036650668143` (the user — already DM) |
| Status | `active` |
| Diagonal rule | standard · Sources: `wotc-srd` · Turn timeout: 24h |

### Discord channel IDs (from `campaigns.settings.channel_ids`)

| Channel | ID |
| --- | --- |
| #the-story | `1507958843769098280` |
| #in-character | `1507958845547217017` |
| #player-chat | `1507958847137120267` |
| #your-turn | `1507958852086399037` |
| #combat-log | `1507958838442070057` |
| #combat-map | `1507958850505019462` |
| #initiative-tracker | `1507958836898693310` |
| #roll-history | `1507958840241684611` |
| #character-cards | `1507958855185862801` |
| #dm-queue | `1507958856930557994` |

## Map

- **Ashfall Waystation — common room** (`1ad14481-f938-462d-be75-25764463ff5b`),
  **12×10** blank grid built fresh via dashboard Maps → New Map (not the sample
  import). A 2×2 **Pit** in the bottom-left corner marks the **cellar mouth** the
  wretch climbed out of; the rest is open ground (hearth / front door / tables are
  narrated, not painted).

## Character(s)

- **Vale** — Tiefling **Warlock 3** (patron: **the Fiend**), entertainer
  background. **APPROVED** 2026-06-25 (was first portal character; built clean
  after the ISSUE-001/008 fixes went live).
  - character id `b6ca7f49-c173-4290-8c80-6fb785fbe733`
  - HP **24/24**, AC **10** (DEX +0, no armor equipped — leather sits in the pack;
    equip → AC 11), speed 30, prof bonus +2.
  - Abilities: STR 10 · DEX 10 · CON 15 (+2) · INT 14 · **CHA 16 (+3)** · WIS 10.
  - Saves: WIS, CHA. Skills: acrobatics, performance, deception, history.
  - **Pact Magic:** 2 slots @ **slot level 2** (`pact_magic_slots {current:2,max:2,
    slot_level:2}`). Spell save DC 13, spell atk +5 (CHA).
  - Cantrips: **chill touch, mage hand** (+ thaumaturgy from Infernal Legacy).
  - Known spells: **hellish rebuke** (L1) · **hold person, shatter, misty step** (L2).
    Infernal Legacy also grants 1/day hellish rebuke @ L2 (free, CHA).
  - Tiefling: **resistance to fire** (Hellish Resistance), darkvision.
  - Kit: dagger, light crossbow + bolts, arcane focus, dungeoneer's pack,
    entertainer's pack (instrument, costume), leather armor (now equipped → AC 11).
    Languages empty (ISSUE-009 cosmetic gap).

- **Forge Anvilbearer** — Hill-Dwarf **Barbarian 3** (Path of the Berserker),
  guild-artisan background. **APPROVED** 2026-06-25 (the DM's friend; the remote
  2nd player, in via the cloudflared tunnel after the ISSUE-013 slug-drift fix went
  live). Character card auto-posted to #character-cards on approval.
  - HP **32/32**, AC **14**, speed 25 (dwarf).
  - Skills: athletics, intimidation (class) + insight, persuasion (guild-artisan bg).
  - Player-controlled — narrate his *arrival/world*, not his choices/dialogue.

## Encounter / combat

- **OVER — encounter `completed` (DM ended combat in Round 3). Victory.** Encounter
  "Waystation — the cellar wretch" (combat id `6f317490-c43e-44a0-a1d0-b6ed51e58a3e`),
  ended 2026-06-26 ~13:58 UTC via the dashboard **End Combat** button (which auto-cleared
  Vale's hold-person concentration). The wretch (Ghoul G1) is DEAD; both PCs untouched.
  - **Initiative:** Forge **22** → the wretch **19** → Vale **19** (Forge up first).
  - **Threat (DEAD):** 1× **Ghoul** stat block (G1) — **AC 12, HP `0/22`, `is_alive=f`**,
    killed by Forge's auto-crit handaxe in **Round 3** (2026-06-26 13:32 UTC). It climbed
    out of the cellar mouth (2,7) into melee at D7 and never landed a hit. **DM RULING: it
    was a LIVING wretch (Humanoid), not undead** — reflavored so Vale's *hold person* was a
    valid target (engine labels the stat block "Ghoul"; ignore the type tag). Corpse still
    carries the stale `paralyzed` tag — cosmetic, it's dead.
  - **PARALYSIS was the kill enabler (now moot).** *Hold person* paralyzed it from R1 (auto-fail
    STR/DEX saves, attackers have advantage, any melee hit within 5 ft is an **auto-crit**),
    so Forge auto-crit it across R2 and R3. **Vale is still flagged concentrating on hold
    person against a dead target** (`concentration_spell_id=hold-person`) — stale; should drop.
    If players are told anything, the wretch "seized up, then came apart" — never "paralyzed".
  - **Full combat chronology (R1→R3, from DB turns + Discord #combat-log):**
    - **R1 Forge (22):** freeform *throw* — **handaxe HIT** (15 vs AC 12) **7 dmg**. Wretch
      22→**15** (bloodied). *(Throw was DM-applied, not auto-logged in #combat-log.)*
    - **R1 wretch (19):** moved cellar-mouth → **D7** (adjacent Forge E7); **Multiattack
      bite (8) + claws (10) BOTH MISSED** AC 14. No damage. *(Also DM-narrated, not auto-logged.)*
    - **R1 Vale (19):** cast **hold person** → wretch **WIS 6 vs DC 13 FAIL → PARALYZED**.
      Pact slot **2→1**. Concentrating. Turn completed.
    - **R2 Forge:** auto-crit dual handaxe — main **10** (2d6+2) + off-hand vex **2** (2d6)
      = **12**. Wretch 15→**3** (**survived** — the light-weapon crits underperformed).
    - **R2 wretch:** turn **auto-skipped** (paralyzed).
    - **R2 Vale:** **light crossbow → MISS** (roll 10). Turn completed.
    - **R3 Forge:** auto-crit handaxe **12** (2d6+2) — already lethal (3−12); off-hand **6**
      overkill. Wretch → **0/22, DEAD** (13:32 UTC 2026-06-26).
    - **R3 Vale — ACTIVE:** turn open, action unused. *(But the only enemy is already dead.)*
  - **Token positions (2026-06-26):** Forge **E7** (HP 32/32, untouched, **not raging**),
    Vale **K6** (HP 24/24, still flagged concentrating — moot), the wretch corpse **D7**
    (**0/22, DEAD**). Monster HP stays hidden from players in #initiative-tracker.
  - **Two map-render bugs found + fixed (2026-06-25):**
    1. *PCs had no tokens.* The blank dashboard-built map has **no authored spawn
       zones**, so combat-start's PC seater bailed and wrote the zero-value
       position (col `""`, row 0) for Vale + Forge — unparseable, so the renderer
       skipped them. **Fix:** `seatPCsInSpawnZones` now falls back to open
       in-bounds tiles (skipping monster tiles) when a map has no spawn zones
       (`spawnzone.AssignPCsToOpenTiles`). Live data patched to J7/K6.
    2. *Enemy never showed on the player map.* `combatantsToRendererForm` never
       set `IsVisible`, so every enemy defaulted to hidden and `filterCombatantsForFog`
       dropped it *before* the line-of-sight check — enemies were excluded from
       #combat-map regardless of sight. **Fix:** propagate `c.IsVisible`. Now a
       visible enemy in a PC's line of sight shows; genuinely hidden / out-of-sight
       enemies stay fogged (fog-of-war retained by design choice).
  - **#combat-map note:** this combat started BEFORE the `7b6c125` deploy, so the
    opening board was NOT auto-posted (that feature only fires on *future*
    StartCombat). The board lands in #combat-map on the first `/done`, on a DM-run
    enemy turn, or when any player runs **`/map`**. Player-view fog is ON: it now
    shows every token a PC can see (the wretch included), and hides the rest.
  - **Reserve:** if 1 wretch proves light for two L3 PCs, a 2nd can claw up from the
    pit mid-fight (the door was scored by something *desperate*).

## Opening scene (DM plan — flex to the character once built)

**Working title: "Ashfall Waystation."** A lone traveler (the PC) reaches a
remote stone waystation on the edge of a grey moor as an ash-coloured dusk
settles. The hearth is cold, the keeper gone, the cellar door scored with deep
fresh scratches from the *inside*. Sandbox: the player can investigate, search,
talk to whatever they find, leave, or force the cellar — which is where a fight
waits if they want one. The 10×10 map = the waystation common-room/cellar for any
combat. Keep it adaptable: tailor the hook to whatever class/race the player
builds (a cleric senses wrongness; a rogue spots the pried lock; etc.).

## Dashboard access

- Claude is driving the DM dashboard via claude-in-chrome (tab on
  `/dashboard/app/#home`). Session already authenticated as the DM (no re-login
  needed). Confirmed clean: 0 pending approvals / 0 encounters / 0 dm-queue.
- **Standing rule:** all DM *mutations* go through this dashboard tab (Chrome),
  never raw SQL/curl; narration to #the-story uses a `:::read-aloud:::` block. See
  README "Hard constraints" + `runbook.md` §8.

## Next action

- **COMBAT IS OVER — victory, encounter `completed`.** The kill was **narrated** to
  #the-story (read-aloud, 2026-06-26 13:45 UTC, Discord msg `1520062389649670288`) and the
  DM **ended the encounter** via the new dashboard **End Combat** button (status→completed,
  Vale's concentration auto-cleared). The party (Forge 32/32, Vale 24/24, 1 pact slot) stands
  over the wretch's corpse in the waystation common room. Nothing is owed on the fight.
- **The scene now:** post-combat lull. The thing is dead; up close it was a *person* once
  (the keeper, maybe), hollowed out. The **cellar mouth still gapes** in the SW corner (the
  2×2 pit), its door clawed to splinters **from the inside**. The dread points downward.
1. **Next beat — the cellar descent. The encounter is PRE-BUILT and ready.** Wait for the
   players (Vale / Forge) to decide in `#in-character` — search the body, loot the room,
   descend, or rest. Narrate what they find; **don't act for them.** When they go down,
   just open the pre-built encounter and **Start Combat** — no setup needed:
   - **Encounter:** "**Cellar — the brood**" (player-facing "**The Cellar**"),
     `encounter_templates` id `0a54efd4-a3a2-47b5-ac7f-0030a9cb22d1`.
   - **Map:** "**Ashfall Waystation — cellar**" (`d2fe03c6-9749-4a24-a6e3-cb9d3a77e3cd`),
     12×10 blank stone grid with a **PC spawn zone at the top-center stairs landing**
     (party seats there on Start Combat). Cellar features (pillars, the deeper shaft, the
     reek) are **narrated, not painted** — same convention as the common room.
   - **Enemies:** **2× Ghoul wretches** (same living-wretch reflavor as upstairs — *hold
     person* etc. valid), placed in the **back corners**: **G1 at (2,8)**, **G2 at (9,8)**,
     away from the PC entry. "Surprised" toggles are OFF — adjudicate surprise live (the
     brood lurking in the dark could surprise the party; the player's light/perception decides).
   - **Difficulty note:** 2 Ghouls for two L3 PCs is a real fight, especially if Vale is
     down to **1 pact slot** (no long rest yet) — she can't hold-person-lock both. Drop one
     wretch (delete G2 in the builder) if you want it lighter.
2. **Loot / aftermath (optional):** the keeper's body / the common room may hold a clue to
   what's below (a key, a journal, claw-scored boards). DM's call whether to seed any.
3. **Bookkeeping done:** concentration cleared, no pending dm-queue from the fight. Vale's
   leather armor still unequipped (AC 10; `/equip item:leather armor:true` → AC 11) if she wants it.
4. **App fixes shipped this session (both TDD + redeployed):** the **End Combat** button on
   the Combat Manager, and an **encounter-builder bug** — `getEncounter`/`updateEncounter`/
   `deleteEncounter`/`duplicateEncounter` never sent the backend-required `campaign_id` query
   param, so **editing/saving any existing encounter 400'd** ("campaign_id query parameter
   required"). Now fixed; that's how G2's placement above could be saved at all.
</content>
