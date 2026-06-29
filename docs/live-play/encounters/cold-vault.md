# Encounter — The Cold Vault: the keeper (PRE-BUILT, one click to run)

> Design-intent card. The encounter, the monster, the map, and all mechanical stat
> lines (AC / HP / Multiattack / Life Drain / placement) are **live in the dashboard
> combat builder** — open it there, don't hand-copy them here (numbers drift). This
> file holds only the non-derivable DM design + the staged narration. Lore:
> [`../world.md`](../world.md); the journal clue that points here:
> [`../sessions/session-01.md`](../sessions/session-01.md) (06-29). Start Combat:
> [`../runbook.md`](../runbook.md) §4.

- **Encounter (template) id:** `adc064e7-2800-4787-8cb8-5deb23d1fc1f` (durable
  reference — DM name **"Cold Vault — the keeper"**, player-facing **"The Cold Vault"**).
- **Map:** *Ashfall Waystation — the cold vault* (`2899165e-3d1b-46e9-962f-9065e4e3529a`,
  12×10, built in-app). Blank stone — **features narrated, not painted** (same convention
  as the cellar / common room). **PC spawn zone** is the **bottom-center** edge (the cold
  door they came through). The keeper waits at **top-center (6,1)**, the far end of the vault.
- **Composition:** **1× Wight** (SRD, CR 3), reflavored as the **vault-keeper** — a
  frost-rimed thing in the keeper's old clothes, the source the brood fled. It does not
  bite; it lays a hand and the *warmth and years* drain out (Life Drain). **Surprise toggle
  OFF** — adjudicate surprise live. (No adds pre-placed — see Reserve below.)

## The hook that lands them here

The keeper's journal (read 06-29) named a **cold door** below the cellar that the keeper
unlocked, and the wretches **came up fleeing whatever was behind it** — *"do not turn it."*
**Vale's cold iron key opens that door.** So the route in is: descend the cellar (past the
dead brood) → the cold door → **turn the key** (player's call; the keeper warned against it)
→ this vault. Run the descent + the door as narration (staged below); **Start Combat only
when they cross into the vault.**

## The escalation — their easy button is gone

The upstairs/cellar wretches were ruled **LIVING** Humanoids, so Vale's **hold person**
paralysed them and Forge auto-crit them down. **The keeper is genuinely UNDEAD** — *hold
person* (Humanoid-only) **simply fails** here. Telegraph it: the first time Vale tries it,
the spell *guppies out* / finds nothing living to grip. This is deliberate — the boss beat
should make them reach past the combo that carried the brood fights.

- **Vale's live tools that DO bite undead:** *shatter* (thunder, no type immunity),
  *hellish rebuke* (fire — she has a 1/day free one too), *chill touch* (cantrip: target
  **can't regain HP**, **and an undead has disadvantage on attacks against her** while
  marked — strong here). Spell save **DC 13**.
- **Forge:** **Rage** resists the keeper's **slashing** (its blade) — a real survivability
  lever — but **Life Drain is necrotic**, *not* resisted by Rage. Reckless Attack trades
  defense for advantage; watch his HP against the drain.

## Run the Wight straight from the builder (secret — never quote to players)

Pull live AC/HP/attacks from the combat workspace; mask all of it in prose
([`../dm-rules.md`](dm-rules.md) "Enemy HP and AC are secret"). Reflavor on contact:

- **Multiattack** — two strikes (the rimed blade). Describe wounds, not numbers.
- **Life Drain** — the *touch*: CON save (DC 13) or **max HP drops** until a long rest
  (the cold takes something that rest alone won't give back). Narrate it as years/warmth
  leaving, frost climbing the limb — not "you lose 9 max HP." **A humanoid dropped to 0 by
  the drain would rise under its control** — a live threat to dangle if a PC falls, but the
  keeper's true menace is the max-HP erosion grinding them down.

## Difficulty & scaling (2× L3 PCs, both at full)

- **As built — 1 Wight (CR 3) vs two L3 PCs is a Hard→Deadly boss duel.** Single CR 3 ≈ the
  pair's deadly line, and the **<3-PC action-economy bump** makes it *feel* deadly. Winnable
  with Rage + spent slots + smart use of chill touch; lethal if they trade blows carelessly
  or burn the fight long enough for Life Drain to stack. Tuned to be **tense, not a TPK** —
  a real "we should've been ready" beat after two easy wretch fights.
- **Reserve mechanic (only if it's too easy):** 1–2 **reanimated husks** (SRD **Zombie**,
  CR 1/4) lie in the wall alcoves — the keeper's older victims. Have them **shudder upright
  on round 2, or when the keeper is bloodied**, if the PCs are cruising. Add them live in the
  builder; they are intentionally **not** pre-placed so the saved encounter can't TPK on open.
- **Lighter:** if a PC is already hurt going in, hold the husks and let the keeper fight solo.
- **Bigger party (4-6 PCs):** keep 1 keeper, scale the husks (~1 per 1.5 PCs) and/or add a
  second keeper-spawn climbing from a deeper crack. See [`../big-party.md`](../big-party.md)
  "Encounter scaling."

## Loot / thread (DM's call — don't pre-promise)

The vault is an older shrine to a **forgotten god**, its name chiselled out of every surface
(ties Vale's patron — a story-hungry being met *chasing a forgotten deity*). On the keeper or
the dais, seed the **next breadcrumb**: a fragment of that scratched-out name, a relic, or the
"story" Vale's patron sent her to collect. Leave it open — this is the campaign's next pull,
not a wrapped bow.

## To run

1. **Await the players actually descending** (#in-character — **don't act for them**). They've
   committed to it (Forge: *"I'm in"*; Vale's patron is pulling her down).
2. Post the **descent** read-aloud (A below) → the **cold door** beat → if they **turn the
   key**, post the **vault** read-aloud (B below).
3. Open **"Cold Vault — the keeper"** → **Start Combat**; adjudicate surprise live. The board
   auto-posts to #combat-map on Start. Players roll their own dice; keep AC/HP secret.

## Staged narration (ready to post — wrap each in the Narrate read-aloud block)

**(A) The descent — past the dead brood to the cold door.**
```
The stair drops you into the cold the brood died fleeing. Your own kills lie where
they fell, faces slack and almost human again now the hunger has gone out of them.
The reek thins as you go deeper, and the dark takes on an edge — a still, mineral
cold that has nothing to do with the moor above. The passage ends at a door of black
iron, rimed white with frost, sweating cold into the stone. Set in it, a lock the
shape of the key on Vale's thong. Up close you can hear it: behind the door, very
faint, something that is not wind and not water — patient, and waiting.
```

**(B) Turning the key — the vault and the thing in it.** *(Post only if they turn the key.)*
```
The cold iron key turns as easily as the keeper swore it would. The door swings in on
a breath of grave-cold air, and the lantern light spills into a low round vault older
than the road — a shrine to some god whose name has been chiselled out of every stone,
over and over, by a frightened hand. At the far end something rises that was a person
once, in the keeper's own frost-grey clothes, and where its face should warm to the
light it only goes colder. It does not lunge. It lifts one hand, almost gently — and
the air between you turns to winter.
```
