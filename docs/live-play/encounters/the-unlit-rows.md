# Encounter — The Unlit Rows: ambush in Sesh's dark undermarket (⚔ COMBAT LIVE 07-19)

> **⚔ COMBAT LIVE 2026-07-19.** Template `7e91023b-06e3-4f5d-b955-4eea6fa5d7f3` →
> **live encounter `6fffbb99-1584-4c3e-a642-a641dce1b2aa`**. All three PCs rolled
> `/initiative` (Windreth 18 / Vale 15 / Forge 12, auto-filled verbatim); enemies
> engine-rolled (ENF-NW 17). Order: **Windreth → ENF-NW → Vale → Forge → [lower]**. PCs
> bottom-center (Forge G11 / Vale H11 / Windreth I11); ambushers in the 4 booths (ENF-NW B2,
> NE O2, W B8, STK O8). **Round 1, Windreth's turn.**
>
> **🐛 Fog-of-war bug found + FIXED live:** the player map leaked all 4 ambushers. Root
> cause was NOT the map data — `buildVisionSources` seeded player fog from **enemy** vision
> too (each source force-shows its own tile even in `magical_darkness`). Fix = **player fog
> is PC-vision only** (`cmd/dndnd/discord_adapters.go`, redeployed) + cleared the stale
> `explored_cells` (bug had cached booth tiles as Explored; one-line DB repair). Walls also
> recolored **bold slate + moved beneath the fog pass** (`renderer.go`) so undiscovered walls
> stay hidden and DMs can read corridors. The magical_darkness booths are now belt-and-braces,
> NOT the token-hider — see the corrected [[reference_map_fog_vision_model]] and the fog recipe
> below (⚠ the old "darkness hides the token" claim was wrong). Purpose-built to
> show off the map engine's **walls + fog-of-war** — which it did, by surfacing this bug.
> Lore/spine: [`../campaign-arc.md`](../campaign-arc.md) (Leg 3, the four hands; blank
> people = the renegade's method). Rules: [`../dm-rules.md`](../dm-rules.md).

## Durable refs (built via in-page dashboard fetch, never raw SQL)

- **Map:** *The Unlit Rows* — id **`f2b2f184-4327-44a0-98be-8c53a9a92f1f`**, 16×12,
  tilewidth 48. Editable in the dashboard visual Map Editor (`#maps` → Edit) for tweaks.
  Authored as raw `tiled_json` → `POST /api/maps` (11 wall segments, 4 magical-darkness
  booths = 32 tiles, 6 difficult-terrain tiles, 5 spawn zones, terrain+lighting tilesets).
  - **Player spawn:** bottom-center, tiles (6,10)–(9,11) — the party enters from the
    canopy side.
  - **Enemy booths (magical_darkness):** the 4 corners — NW (0,0)–(2,2), NE (13,0)–(15,2),
    W (0,7)–(3,8), E (13,7)–(15,8). Ambushers wait here (see recipe — the darkness is
    load-bearing, not flavor).
  - **Walls:** an orthogonal lattice forming winding corridors + blind corners between the
    spawn and the booths (axis-aligned only — the pathfinder ignores diagonal wall segments
    for movement).
- **Statblocks (homebrew creatures, campaign `532b4774-…`):**
  - **Blank-Faced Enforcer** — `hb_d63836d7fe14`, CR 2, AC 14, HP 32, darkvision 60,
    Multiattack (2× Cutter 1d8+3), *Faceless* (immune charmed/frightened, can't be
    reasoned with), *Ambush Instinct* (+1d8 on a first strike vs a foe that hasn't seen it).
    Deploy **3–4** (one per booth).
  - **The Hollow Stalker** — `hb_675d828a823e`, CR 3, AC 15, HP 39, DEX 17, Stealth +6,
    darkvision 60, Silent Blade 2d6+3 / Flung Awl 1d6+3, *Sneak Strike* (+2d6),
    *Warded Dark* (market darkness/dim never hinders it), bonus action **Vanish** (Hide
    like Cunning Action). Deploy **1** — the Windreth-mirror that slips the dark and
    re-hides. Optional; drop for a lighter fight.

## ⚠ The fog recipe — how the walls + fog actually produce the ambush (non-obvious)

Verified against the render/vision code 2026-07-19. Get this right or the effect breaks:

- **Sight is a finite 60 ft (12-tile) radius for everyone, always** (`defaultBaseSightTiles=12`
  in `discord_adapters.go`), bounded by wall line-of-sight. There is **no "lit vs dark"
  map toggle** — limited vision is the default. So the **walls carve the black corridors**:
  anything behind a wall segment or beyond 60 ft renders solid black *Unexplored* on the
  player map. The player-facing (fogged) PNG is what posts to Discord; the DM sees the full
  board via the dashboard `?view=dm`.
- **Enemies the party hasn't line-of-sighted are auto-dropped from the player map** by
  `filterCombatantsForFog` (enemies on Unexplored tiles are removed) — so the ambushers are
  invisible until a PC rounds the corner into LOS. No manual hide needed for *that*.
- **⚠ The vision-bubble leak (the gotcha):** every living combatant — including an unseen
  ambusher — emits its own 60 ft vision, and all vision sources union onto the *same* player
  map. So a hidden enemy's own sight bubble **un-fogs the tiles around it and betrays its
  position**, even though its token is hidden. **Fix: keep ambushers inside `magical_darkness`**
  — that collapses their vision contribution to just their own tile (magical darkness zeroes
  ordinary vision *and* darkvision; only Devil's Sight / truesight / blindsight pierce). That
  is exactly why the 4 booths are painted magical_darkness. Do **not** place ambushers on
  lit corridor tiles pre-reveal.
- **`magical_darkness` (GID 3) is the ONLY enforced static darkness lever** — the non-magical
  `dim_light`/`darkness`/`fog` lighting brushes are parsed-but-dead (they render nothing to
  the fog). Diegetically this reads as *warded gloom* in a face-market, which fits Sesh.
- **No start-time hidden flag, no DM hide endpoint.** `is_visible` defaults true and only the
  in-combat **Hide** action sets it false; enemies auto-reveal on **attacking** (`AttackerRevealed`)
  or when a PC moves adjacent and out-perceives their Stealth. So the ambushers reveal
  themselves fairly when they strike — no need (or supported knob) to force turn-0 invisibility;
  the **magical-darkness booths already hide token *and* vision bubble** until they step out.

## Deploy recipe (when a fight triggers)

1. `POST /api/homebrew/creatures` already done — the two statblocks exist.
2. Build the encounter + `POST /api/combat/start` with map `f2b2f184`, PC positions in the
   player spawn zone, PC initiatives supplied via `character_initiatives` (per
   `project_combat_start_pc_init_seat_repair`). Place 3–4 Enforcers in the 4 magical-darkness
   booths (+ the Stalker in one). Enemy `is_visible` stays true (fog + darkness hide them anyway).
3. First rounds: run the ambushers via the Turn Builder. They hold in the dark until a PC
   enters LOS or they open with a strike (auto-reveal). The Stalker uses **Vanish** to slip
   back into a booth between hits.
4. Levers mid-fight: a PC toppling/dousing a lamp or Vale casting **Darkness** (an enforced
   `magical_darkness` zone) re-shapes who-sees-what live.

## Difficulty & scaling (party = Vale/Forge/Windreth, all L4)

- **3× Enforcer + 1× Stalker** ≈ a Hard fight for three L4 PCs (CR 2×3 + CR 3, action-economy
  weighted by the ambush + the fog denying the party target info round 1). Winnable: Forge's
  Rage soaks the slashing Cutters; Windreth out-stealths the Stalker; Vale's AoE punishes
  clustered booths once revealed.
- **Lighter:** drop the Stalker, 2–3 Enforcers.
- **Bigger party (→5–6):** ~1 Enforcer per 1.5 PCs + keep the single Stalker; see
  [`../big-party.md`](../big-party.md).
- **The fun is informational, not just numeric** — the fog + walls make it a fight about
  angles, lamplight, and who-has-darkvision, not a slugfest. If a PC has Devil's Sight
  (Warlock invocation), they pierce the warded booths — a real reason that PC scouts.

## Story hook (why this is earned, not a spawn)

Sabinnet warned the buyer's hand is close, and reading her missing = the soft clock. The
attackers are **blank-faced** — victims hollowed by the renegade's erasure craft (canon),
now the buyer's disposable muscle, sent to take back the prize (Windreth's name) the party
just had read. If the party lingers in Sesh, this is the natural intercept as they leave the
canopy. Adjust the framing to the fork the players actually pick (buyer's hand / spooked
fled-wardens / a lone made-hunter) — the map + statblocks reflavor to any of them.
