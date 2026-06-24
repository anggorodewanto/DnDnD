# Game State — live save file

> **Update this file as play advances.** It is the single source of truth for
> "where are we right now." Timestamps in the campaign's local fiction are loose;
> real-world dates are absolute.

_Last updated: 2026-06-24 (session 1, setup — campaign clean-slated)._

## Stack status

- **UP** via `make local-up` (docker compose). App `localhost:8080`, DB
  `localhost:5432`. Bot `DnDnD` (id `1507904367301496862`) connected to guild
  `DnDnD`.

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

- **None yet.** Plan: user builds a fresh character via `/register` → Build New →
  web builder. Claude approves on the dashboard. Record name / class / race /
  level / HP / AC / player_character ID here once approved.

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

1. **User:** run `/register` → **🆕 Build New** in Discord; build a character via
   the portal link; submit (goes pending → `#dm-queue` + Approvals).
2. **Claude:** approve from the dashboard Approvals view; then open the scene
   (tailor "Ashfall Waystation" to the character).
</content>
