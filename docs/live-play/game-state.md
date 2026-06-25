# Game State — live save file

> **Update this file as play advances.** It is the single source of truth for
> "where are we right now." Timestamps in the campaign's local fiction are loose;
> real-world dates are absolute.

_Last updated: 2026-06-25 (session 1 — combat LIVE: Vale cast hold person → escalated
to initiative; Round 1, Forge's turn first vs the cellar wretch)._

## Stack status

- **UP** via `make local-up` (docker compose). App `localhost:8080`, DB
  `localhost:5432`. Bot `DnDnD` (id `1507904367301496862`) connected to guild
  `DnDnD`.

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
- **Public URL (EPHEMERAL):** `https://pillow-reproduction-centers-feel.trycloudflare.com`
  — changes every time the tunnel restarts. On change, `make tunnel-up` redoes the
  `.env` repoint + app restart automatically; you still must re-register the new
  `…/portal/auth/callback` in the Discord dev portal.
- `.env` changed: `BASE_URL` + `OAUTH_REDIRECT_URL` now point at the tunnel
  (backup: `.env.bak.preTunnel`). App restarted; OAuth `redirect_uri` confirmed =
  `<tunnel>/portal/auth/callback`.
- **Manual step still owed by the DM:** register
  `https://pillow-reproduction-centers-feel.trycloudflare.com/portal/auth/callback`
  in Discord Developer Portal → app (`DISCORD_CLIENT_ID` 1507…) → OAuth2 →
  Redirects (Discord rejects unlisted redirect URIs → login fails without it).
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

- **LIVE — Round 1.** Encounter "Waystation — the cellar wretch" (combat id
  `6f317490-c43e-44a0-a1d0-b6ed51e58a3e`), on the common-room map above.
  - **Initiative:** Forge **22** → the wretch **19** → Vale **19** (Forge up first).
  - **Threat:** 1× **Ghoul** stat block (G1) — **HP 22, AC 12**, placed at the cellar
    mouth (2,7), ~35 ft (7 sq) from the party by the door. **DM RULING: it is a
    LIVING wretch (Humanoid), not undead** — a person rotted/maddened by whatever's
    in the cellar. Reflavored so Vale's *hold person* is a valid target (the engine
    just labels the stat block "Ghoul"; ignore the type tag). Ghoul claws/bite +
    paralysing touch reflavored as a sickening grip; run RAW numbers.
  - **Vale's pending action:** declared *hold person* pre-initiative; per the player
    it's HELD and **resolves on Vale's turn** (3rd, after the wretch moves) — WIS save
    vs **DC 13**. Don't pre-resolve it. Vale has 1 pact slot left after it (2→1).
  - **Token positions (2026-06-25):** Forge **J7**, Vale **K6** (party by the
    door), the wretch **C7** (cellar mouth, just NE of the corner pit). All three
    render on the board. Monster HP hidden from players in #initiative-tracker (good).
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

## Next action

- **COMBAT IS LIVE — Round 1, Forge's turn.** Vale answered the emergence by casting
  *hold person* (`#in-character`, 11:31 AM); the DM (per the player) escalated straight
  to initiative rather than resolving the cast in narration. Initiative rolled, combat
  opened, and the DM posted the combat-start beat to `#the-story` (11:46 AM):
  the wretch drops to all fours, **established as alive** (heaving ribs, ticking pulse,
  drool) so *hold person* is on the table; ~35 ft of flagstone between it and the party.
  See the **Encounter / combat** section above for ids, initiative, stats, and the
  living-wretch ruling.
1. **Forge (init 22) is up first** — his player declares in `#in-character` (or `/move`,
   `/attack`). **His choices are his own; the DM does not act for him.** Forge is the
   remote 2nd player (cloudflared tunnel).
2. **Then the wretch (19):** DM runs it. From (2,7) it can move ~6 sq (30 ft) toward the
   party and likely reach melee on whoever's closest unless Forge intercepts/blocks. Run
   its turn from the Combat Manager (move + Multiattack: 2 claws / bite; on a hit by
   claws vs a non-elf, the target's CON save or be reflavored-"gripped"/restrained — DM
   call; keep RAW numbers).
3. **Then Vale (19):** resolve her held *hold person* — the wretch makes a **WIS save vs
   DC 13**. Fail → paralysed (advantage to attackers, auto-crit melee in 5 ft = huge for
   Forge); save → no effect, slot spent (Vale 2→1 pact slots).
4. **DM cadence each turn:** advance the Combat Manager turn queue (End Turn), narrate
   results to `#the-story`, apply damage/conditions on the combatant panels. If the lone
   wretch is trivial, a 2nd can claw up from the pit (see Reserve note above).
</content>
