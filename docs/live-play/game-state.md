# Game State — live save file

> **Update this file as play advances.** It is the single source of truth for
> "where are we right now." Timestamps in the campaign's local fiction are loose;
> real-world dates are absolute.

_Last updated: 2026-06-25 (session 1 — scene live; Vale opened the cellar door)._

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

- **None** (old "New Map" deleted during clean-slate). Import a battle map only
  when the scene needs one — `docs/testdata/sample.tmj` is a 10×10 sample to
  import via the dashboard (`POST /api/maps/import`). Record the new map ID here.

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
    entertainer's pack (instrument, costume), leather armor (unequipped). Languages
    empty (ISSUE-009 cosmetic gap).

## Encounter / combat

- **None yet.** No encounters, no active combat. To be built once the character
  exists and the scene calls for it.

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

- **Scene is live.** Opening narration + the cellar-open beat are both posted to
  `#the-story`. Vale cast **mage hand** and opened the cellar door (unbarred → no
  roll); it's now standing open onto dark stone steps, a slow dragging *scrape*
  heard from below.
1. **Player (Vale):** next move — descend / cast light / listen / retreat / talk.
   Descending toward the scrape = the fight is ready (import the 10×10 waystation
   map for the cellar).
2. **Claude (DM):** respond to Vale's move via the Narrate tool (`#the-story`).
   If she descends and combat starts: import the map, build the encounter, open
   combat. Whatever's in the cellar clawed to get *out* — pick/stat the threat.
</content>
