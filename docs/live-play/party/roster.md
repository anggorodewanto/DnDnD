# Party Roster — who's at the table (durable identity + kit)

> **Identity, class, and durable kit only.** Live **HP / position / conditions /
> current slots** are *generated* — read them from the **DM Console** (`state`, see
> [`../README.md`](../README.md)) or a PC's sheet/`/character`, never from a
> hand-kept column here (those drift — see [`../dm-rules.md`](../dm-rules.md) "Keep
> the record straight"). Full per-PC sheets are in the last column; load on demand.
> Onboarding flow: [`../runbook.md`](../runbook.md) "Onboarding players."

_Levels, AC, and skill mods below are **DB-verified 2026-07-27**. The party milestone-levelled
**L4 → L5** after "The Unlit Rows" (07-21); this table had been left at L4 until the 07-27
compaction pass._

| PC | Player | Race / Class / Lvl | AC | Durable kit (max resources / signature) | Sheet |
| --- | --- | --- | --- | --- | --- |
| **Vale** (she/her) | dewa (the user) | Tiefling Warlock **5** (Fiend) | **13** | AC comes from **Armor of Shadows** (*mage armor* at will), **no armour equipped** — a shield would read as 15 but Warlocks have no shield training, and 2024 untrained-shield = disadvantage on all Str/Dex D20 tests **and no spellcasting**; studded leather is a *downgrade* (12). Pact Magic **2× L3** slots, **spell save DC 15**; **Pact of the Tome** — cantrips live in `granted_spells`: guidance / minor-illusion / prestidigitation / disguise-self / mage-armor; **Invocations:** Agonizing Blast, Mask of Many Faces; Eldritch Blast, Hex, Misty Step, Hold Person, Shatter, Hellish Rebuke, Chill Touch, Mage Hand. Skills: **Deception +7** (proficient) vs **Persuasion +4** (raw CHA 18, *not* proficient) · **Stealth +0**. | [vale.md](vale.md) |
| **Forge Anvilbearer** (he/him) | JonathanEka (remote) | Hill-Dwarf Barbarian **5** (Berserker) | **14** *(stripped 07-27, and staying that way — a watch skiff hooked his breastplate out of the reeds as salvage and poled off with it. Still listed carried-not-worn on the sheet; **16** only if they get it back.)* | **Greataxe** (two-handed — a shield costs him the d12 and every GWM swing); **Extra Attack**; **Fast Movement** (35 ft); Rage ×3/long rest (B/P/S resist, Berserker Frenzy), Reckless Attack. **Athletics +5** · Insight +3 · Persuasion +2 · Intimidation +5. _L4 feat: **Great Weapon Master** — 2024 form: crit/kill → bonus-action melee swing via `/bonus gwm <target>`; the −5/+10 form is 2014 and is **not** this table._ Carries the **Jeweler's Reading-Lens**. | [forge.md](forge.md) |
| **Windreth** (he/him) | Windreth (own account) | High-Elf Rogue **5** (Thief) | **16** | Sneak Attack **3d6**; **Uncanny Dodge**; **Expertise: investigation + stealth** → **Stealth +10 / Investigation +6** · SoH +7 · Acrobatics +7 · Perception +5 · Insight +5. **STR 8.** Cunning Action; shortbow + **Silent Blade** (main hand, *vex*) / dagger (off, *nick*), thieves' tools. _L4 feat: **Defensive Duelist** (2024 half-feat: +1 DEX → 18; reaction +prof to AC vs a melee hit while wielding a finesse weapon)._ Carries **THE SEAL — Windreth's Kept Name** (`kept-name-scrap-warded`). | [windreth.md](windreth.md) |

_Add a row + a `party/<name>.md` sheet for each new PC on approval. Keep this table to durable
facts; for who's bloodied / where they stand right now, open the DM Console._

## New PCs joining (big party, 5-6 total)

The party is now **3 PCs** (Vale, Forge, Windreth). Two to three more friends are
expected (toward the 5-6 target). For each:

1. **Onboard** — player runs `/register` → builds in the portal (remote players
   reach it via the ngrok tunnel) → **DM approves** on the dashboard. Full
   steps: [`../runbook.md`](../runbook.md) "Onboarding players."
2. **Add to this roster** — a row above + a `party/<name>.md` sheet (copy the shape
   of [vale.md](vale.md) / [forge.md](forge.md)).
3. **Fold into the fiction** — narrate the new PC as a fellow traveler who reached
   (or was already inside) Ashfall Waystation; seed a per-PC hook (see
   [`../world.md`](../world.md) + [`../big-party.md`](../big-party.md) "Spotlight").

### Pending / planned slots

| Slot | Player | Status | Notes |
| --- | --- | --- | --- |
| PC 4 | TBD | not yet onboarded | — |
| PC 5 | TBD | not yet onboarded | — |
| PC 6 | TBD | not yet onboarded | — |

_(Update as players register; delete unused slots.)_
