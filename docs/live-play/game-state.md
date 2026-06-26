# Game State — live save file

> **Update this file as play advances.** It is the single source of truth for
> "where are we right now." Timestamps in the campaign's local fiction are loose;
> real-world dates are absolute.

_Last updated: 2026-06-25 (session 1 — combat LIVE: Round 1 resolved through Forge's
handaxe + the wretch's whiffed Multiattack; Vale's hold person LANDED → wretch
PARALYZED. Vale's turn still active (movement/bonus action pending), then Round 2
opens with Forge auto-critting the paralyzed wretch. App redeployed **again ~22:50 UTC**
with two more live-play fixes live — **ISSUE-016** (`/done` phantom-attack after a spell
cast) + the **ISSUE-015 DISPLAY half** (paralysis no longer renders as "[object Object]")
— on top of the earlier ~13:45 UTC **ISSUE-014** DM-Console fix; combat state preserved.
Cosmetic caveat: Vale's current turn still shows the pre-fix phantom attack — confirm
`/done` past it once. The **Hold Person** narration is posted to #the-story)._

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

- **LIVE — Round 1 (Vale's turn active).** Encounter "Waystation — the cellar wretch"
  (combat id `6f317490-c43e-44a0-a1d0-b6ed51e58a3e`), on the common-room map above.
  - **Initiative:** Forge **22** → the wretch **19** → Vale **19** (Forge up first).
  - **Threat:** 1× **Ghoul** stat block (G1) — **AC 12, HP now 15/22 (bloodied)**,
    climbed out of the cellar mouth (2,7) and into melee. **DM RULING: it is a
    LIVING wretch (Humanoid), not undead** — a person rotted/maddened by whatever's
    in the cellar. Reflavored so Vale's *hold person* is a valid target (the engine
    just labels the stat block "Ghoul"; ignore the type tag). Ghoul claws/bite +
    paralysing touch reflavored as a sickening grip; run RAW numbers.
  - **CONDITION — the wretch is PARALYZED** (source_spell *hold person*, applied in the
    engine, indefinite until Vale drops concentration). **Hidden from players** — describe
    it as "bloodied and rigid / seized", never "paralyzed". Mechanical consequences: it
    auto-fails STR/DEX saves, attackers have advantage against it, and any melee hit from
    within 5 ft is an **auto-crit**. This sets up Forge for a huge Round-2 swing.
  - **Round 1 resolved so far (chronological):**
    1. **Forge (init 22):** freeform *throw* — **handaxe HIT** (roll 15 vs AC 12) for
       **7 damage**. Wretch 22→15 HP, now **bloodied**. Turn done.
    2. **The wretch (init 19):** moved from the cellar mouth into melee (now **D7**,
       adjacent to Forge at E7); **Multiattack — bite (8) and claws (10) BOTH MISSED**
       Forge's AC 14. No damage. Turn done.
    3. **Vale (init 19) — ACTIVE:** cast **hold person** on the wretch (action used,
       now **concentrating**); the wretch's **WIS save 6 vs DC 13 → FAIL → PARALYZED**.
       Vale spent a pact slot (**2→1, one left**). Her **movement (30 ft) + bonus
       action are still available** — the player's call; her turn is NOT yet ended.
  - **Vale's concentration:** on **hold person** — if she takes damage she rolls a CON
    save (DC = max(10, ½ damage)) or the wretch un-paralyzes. Keep the wretch pinned.
  - **Token positions (2026-06-25):** Forge **E7** (HP 32/32), Vale **K6** (HP 24/24,
    concentrating), the wretch **D7** (HP 15/22, PARALYZED — adjacent to Forge). All
    three render on the board. Monster HP hidden from players in #initiative-tracker (good).
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

- **COMBAT IS LIVE — Round 1, Vale's turn (post-hold-person).** All three Round-1
  initiative slots have acted on their *actions*: Forge threw a handaxe (HIT, 7 dmg →
  wretch bloodied 15/22), the wretch closed to melee (D7) and **whiffed its whole
  Multiattack** on Forge, and Vale's **hold person LANDED** — the wretch failed its WIS
  save (6 vs DC 13) and is **PARALYZED** (Vale concentrating; pact slots 2→1). See the
  **Encounter / combat** section above for the full chronology, ids, and the
  hidden-condition handling.
- **Already done this beat:** the **Hold Person narration is POSTED to #the-story**
  (dashboard Narrate editor → "Post to #the-story" → bot relayed; `narration_posts`
  row at **13:51:18 UTC**, Discord msg id **`1519701526946386084`**) — no re-post
  needed. The app was **redeployed ~13:45 UTC with the ISSUE-014 fix live** (DM
  Console now logs player combat actions; combat state preserved).
1. **Vale finishes her turn (init 19) — the player's call.** Her *action* is spent on
   hold person, but she still has her **30 ft of movement and her bonus action**. Wait
   for her to declare in `#in-character` (or `/move`); **do not act for her.** When she's
   done, `/done` (or End Turn) advances to Round 2.
2. **Round 2 opens with Forge (init 22)** standing adjacent (E7 ↔ D7, within 5 ft) to a
   **PARALYZED** target. Forge's melee attacks get **advantage and auto-crit on hit** —
   a big swing that should drop or nearly drop the wretch. **His choices are his own; the
   DM does not act for him.** (Forge is the remote 2nd player on the cloudflared tunnel.)
3. **Keep the wretch pinned:** the paralysis holds only while Vale concentrates. If the
   wretch (or anything) deals damage to Vale, she rolls a **CON save (DC max(10, ½ dmg))**
   or hold person drops and the wretch un-paralyzes. Track concentration on her panel.
4. **DM cadence each turn:** advance the Combat Manager turn queue (End Turn), narrate
   results to `#the-story` (describe the wretch as "rigid / seized", never "paralyzed"),
   apply damage/conditions on the combatant panels. If the lone wretch is trivial, a 2nd
   can claw up from the pit (see Reserve note above).
</content>
