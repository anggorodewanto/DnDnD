# Async D&D via Discord — System Specification

## Overview

A system for running D&D 5e asynchronously through Discord. Players and the DM take turns through chat on their own schedule — no shared session required. Game state is managed through a backend, surfaced into Discord via a bot, and controlled by the DM through a dedicated web dashboard.

---

## Core Architecture

### Mental Model

Discord is the **display and input terminal**, not the source of truth. The system has three distinct layers:

- **Game State Layer** — a backend database that owns all truth: HP, positions, initiative, inventory, spell slots, conditions
- **DM Tool Layer** — a web dashboard where the DM manages combat and assets
- **Discord Layer** — where players read output and submit commands

### Guiding Rule
> Game state lives in the database. Discord is read-only output. Any state change goes through the bot or DM dashboard — never manually through Discord.

### Authentication & Authorization

**Discord user → Character mapping:**
- Player runs `/register <character_name>` in the Discord server
- Bot creates a `discord_user_id → character_id` mapping in the database
- DM confirms/approves the registration via the dashboard
- One player = one character per campaign (DM can override in dashboard if needed)

**Out-of-turn prevention:**
- On every combat command (`/move`, `/attack`, `/cast`, `/bonus`, `/interact`, `/action`, `/done`, `/deathsave`), the backend validates that the requesting Discord user ID matches the active turn's character owner
- If not their turn, bot replies: "It's not your turn. Current turn: **[Character]** (@player)"
- Exception: `/reaction` can be submitted at any time — it's a declaration, not a turn action
- Exception: `/check`, `/save` are usable any time (in or out of combat, no turn required — DM may call for checks mid-combat)
- Exception: `/rest` operates outside active combat only

**DM dashboard authentication:**
- Discord OAuth2 — DM logs in with their Discord account
- System verifies the authenticated Discord user ID matches the campaign's designated DM
- No separate passwords or accounts to manage

**Multi-campaign support:**
- One bot instance serves multiple Discord servers (multi-tenant)
- All database queries are scoped by `guild_id` / `campaign_id`
- One campaign per Discord server (keeps channel structure clean and unambiguous)

### Concurrency Model

All combat state mutations are serialized through a **per-turn pessimistic lock** using PostgreSQL advisory locks keyed on `turn_id`.

**Rapid player commands** (e.g., two `/attack` commands sent before the first resolves):
- Each command acquires the turn lock before processing
- If the lock is held, the second command **blocks** (waits) rather than failing
- Once the first command completes and releases the lock, the second processes against updated state
- Player experiences at most a few hundred milliseconds of delay — no "conflict, retry" errors

**DM + player concurrent actions:**
- DM dashboard mutations go through the same backend and acquire the same per-turn lock
- If the DM ends a player's turn, any in-flight player command either completes first (if it holds the lock) or fails with "It's no longer your turn"

**WebSocket state sync** — server-authoritative, push-only:
- The Go backend + PostgreSQL is the single source of truth
- The DM dashboard renders state received via WebSocket pushes; it does not maintain its own game state copy
- Flow: command → backend acquires lock → resolves → releases lock → pushes state over WebSocket → dashboard re-renders

**Dashboard optimistic UI** — when a WebSocket push arrives while the DM has a form open:
- Read-only display areas (initiative tracker, HP bars) update immediately
- Active form inputs are **not clobbered** — the DM's draft is preserved with a subtle indicator: "HP updated to 3 by player action"

---

## Discord Server Structure

```
📋 SYSTEM
  #initiative-tracker   ← bot posts/edits current turn order
  #combat-log           ← mechanical results ("Goblin #1 rolled 14 to hit Aria — HIT")
  #roll-history         ← all dice rolls, timestamped

🎭 NARRATION
  #the-story            ← DM narration only, clean prose
  #in-character         ← player roleplay, dialogue, and actions (IC)
  #player-chat          ← out-of-character chatter (OOC)

⚔️ COMBAT
  #combat-map           ← bot posts the grid image each turn
  #your-turn            ← bot pings the active player; players submit commands and in-character speech here

🎒 REFERENCE
  #character-cards      ← auto-updated character info per player
  #dm-queue             ← bot pings DM for every event requiring their attention
```

### Bot Permissions

- `Send Messages` — post to all bot-managed channels
- `Attach Files` — upload map PNGs and text file attachments
- `Manage Messages` — pin/edit the initiative tracker message
- `Use Application Commands` — register and respond to slash commands
- `Mention Everyone` — ping players on turn start (or a configurable `@combat` role)

### Server Setup

The `/setup` slash command auto-creates the channel structure with appropriate permission overrides (e.g., `#the-story` is DM-write-only, `#combat-map` is bot-write-only, `#in-character` is player-and-DM writable). DM runs `/setup` once after inviting the bot. Channels that already exist are skipped.

The bot uses **plain text messages** for most output. For very large output (20+ combatant initiative orders, detailed character cards), the bot uploads a **text file attachment** instead. No embeds required.

### Character Cards (`#character-cards`)

The bot maintains one auto-updated message per registered character in `#character-cards`. Each card shows the character's current state and is re-edited whenever relevant state changes (level up, equipment change, condition applied/removed, rest completed, etc.):

```
⚔️ Aria (AR) — Level 8 Half-Elf Fighter 5 (Champion) / Rogue 3 (Thief)
HP: 38/45 | AC: 16 | Speed: 30ft
STR 10 | DEX 18 | CON 14 | WIS 14 | INT 12 | CHA 10
Equipped: Longbow (main) | Shortsword (off-hand)
Spell Slots: 1st: 3/4 | 2nd: 1/2
Conditions: Blessed (2 rounds remaining)
Concentration: —
Gold: 47gp
```

Fields shown: name, short ID, total level, race, class/subclass (multiclass shows all, e.g., "Fighter 5 (Champion) / Rogue 3 (Thief)"), HP (current/max), AC, speed, ability scores, equipped weapons, spell slots (if caster), active conditions with remaining duration, concentration spell (if any), temp HP (if any), exhaustion level (if any), and gold. Cards are visible to all players — allied HP and conditions are public information.

---

## Player Input Model

### Combatant Targeting

All combatants — enemies and player characters — are assigned short stable IDs at the start of combat. The same ID system is used for all targeting: attacks, spells, shoves, and any command that takes a target.

**Enemy IDs:** unique enemies get an abbreviated code; duplicates are numbered.

```
Goblin #1  (G1)  [B4]  Uninjured
Goblin #2  (G2)  [C6]  Bloodied
Orc Shaman (OS)  [D5]  Uninjured
```

**Player character IDs:** derived from character name initials. Duplicates are disambiguated by appending a number.

```
Aria       (AR)  [E3]  Uninjured
Thorn      (TH)  [F4]  Bloodied
Kael       (KA)  [G2]  Uninjured
```

All combatant IDs appear on map token labels and in `#initiative-tracker`, so players can target by sight. Examples: `/attack G1`, `/cast healing-word AR`, `/cast cure-wounds TH`.

**Enemy HP is hidden from players.** The bot never reveals exact HP, AC, or stat block details in player-visible channels. Instead, enemies show a **descriptive health tier:**
- **Uninjured** — 100% HP
- **Scratched** — 75–99% HP
- **Bloodied** — 25–74% HP (standard 5e convention: below half)
- **Critical** — 1–24% HP
- **Dying / Dead** — 0 HP

**Player character HP** is visible to all players as exact values (allies can see each other's current and max HP in `#initiative-tracker` and on character tokens).

**Dying vs dead on map:** dying characters (0 HP, making death saves) and dead characters use distinct token visual states — `dying` shows the token dimmed with a heartbeat icon; `dead` shows the token greyed out with an X. Both are clearly distinguishable at a glance.

The DM sees exact numbers for all combatants in the dashboard. IDs are stable for the entire encounter — if G1 dies, G2 stays G2.

### Grid Movement

The map uses a standard chess-style coordinate grid: **columns as letters, rows as numbers**. Columns use spreadsheet-style lettering: A–Z for the first 26, then AA, AB, … AZ, BA, BB, … for larger maps.

```
  A   B   C   D   E   F
1 [ ] [ ] [ ] [ ] [ ] [ ]
2 [ ] [G1] [ ] [ ] [ ] [ ]
3 [ ] [ ] [AR] [ ] [G2] [ ]
4 [ ] [ ] [ ] [OS] [ ] [ ]
```

Movement is expressed as a destination coordinate:

```
/move D4
/move E6
/move AA12     ← valid on maps wider than 26 columns
```

The backend validates every movement command: remaining speed, tile occupancy, difficult terrain, and obstacles. Invalid moves return a specific reason. Movement can be split across actions: `/move D4` → `/attack G1` → `/move E5` — each `/move` deducts from remaining speed.

**Movement confirmation prompt:** every valid `/move` command responds with an ephemeral confirmation before committing:

```
🏃 Move to D4 — 20ft (includes difficult terrain), 10ft remaining after. [✅ Confirm] [❌ Cancel]
```

The player clicks **Confirm** to commit the move or **Cancel** to abort. The prompt shows total path cost (noting difficult terrain if applicable) and remaining movement after the move. If the move is invalid, the rejection message is shown immediately with no confirmation step. The confirmation uses Discord buttons and is visible only to the acting player.

**Moving through occupied tiles (5e rules):**
- **Allied creatures:** you can move through an allied creature's space freely, but you cannot end your turn there
- **Hostile creatures:** you cannot move through a hostile creature's space, unless it is two or more sizes larger or smaller than you (e.g., a Medium character can move through a Huge dragon's space)
- **Ending movement:** you can never end your turn in another creature's space, ally or enemy. If `/done` is issued while sharing a tile, the command is rejected: "You can't end your turn in another creature's space — use `/move` to leave [Creature]'s tile"
- Size comparison uses `races.size` (PCs) and `creatures.size` (NPCs/monsters). Size categories: Tiny, Small, Medium, Large, Huge, Gargantuan

**Diagonal movement:** diagonals cost 5ft, same as cardinal movement. Deliberate simplification for async play — the PHB alternating 5/10 variant is not supported.

### Altitude & Elevation

Tokens carry an **altitude** value (integer feet, default 0) representing height above the ground plane.

```
/fly 30        ← rise to 30ft altitude (costs 30ft of movement)
/fly 0         ← descend to ground level
/move D4       ← horizontal movement while maintaining current altitude
```

- Ascending and descending costs movement **1:1**
- Altitude displayed as a **label suffix**: `AR↑30` means Aria at 30ft
- **Distance calculation** uses 3D Euclidean distance (rounded to nearest 5ft) for range checks
- Tokens at different altitudes **do not block** each other's ground tile
- Falling: if a flying creature is knocked prone or loses fly speed, fall damage is 1d6 per 10ft (standard 5e), applied automatically

### Structured Commands

Players submit slash commands in `#your-turn` (where they receive their turn ping), but commands work from any channel in the server — the bot validates and routes output to the correct channels regardless.

**In-character speech:** players can speak in-character during combat by typing plain messages (non-commands) in `#your-turn`. Speaking is free per 5e rules — it costs no action, bonus action, or any other resource. Players can speak on their own turn or briefly on others' turns. Out-of-character discussion goes in `#player-chat`. The bot ignores non-command messages in `#your-turn` (they're just player chat).

**Out-of-combat roleplay:** outside of combat, players use `#in-character` for all in-character dialogue, emotes, and actions. The DM reads `#in-character` to follow what the party is doing and narrates outcomes in `#the-story`. Players can describe what their character says, does, or attempts — anything that requires a mechanical resolution (skill check, attack, etc.) should use the appropriate slash command or be picked up by the DM from context. The flow is: players act in `#in-character` → DM narrates results in `#the-story` → players react in `#in-character`.

**Combat commands** (only usable on your turn, except `/reaction`):

| Command | Example | Description |
|---|---|---|
| `/move` | `/move D4` | Move to coordinate. Shows path cost and remaining movement, then asks for confirmation before committing. Repeatable for split movement as long as total ≤ speed |
| `/fly` | `/fly 30` | Set altitude in feet. Costs movement 1:1 |
| `/attack` | `/attack G2` or `/attack G2 handaxe --gwm` or `/attack G2 --twohanded` | Attack a target. One `/attack` per swing; backend tracks attacks remaining. `--twohanded` for versatile weapons |
| `/cast` | `/cast fireball D5` or `/cast fireball D5 --slot 5` or `/cast detect-magic --ritual` or `/cast fireball D5 --quickened` | Cast a spell at a target coordinate or enemy ID. `--slot N` to upcast; `--ritual` for ritual casting. Bonus action spells (e.g., Healing Word) are auto-detected from `spells_ref.casting_time` — no need for `/bonus cast`; the system deducts the bonus action instead of the action. Sorcerers can add Metamagic flags (e.g., `--quickened`, `--twinned [target]`, `--subtle`) — see Metamagic |
| `/bonus` | `/bonus cunning-action dash` or `/bonus cunning-action disengage` or `/bonus cunning-action hide` | Bonus action |
| `/bonus rage` | `/bonus rage` | Enter rage — Barbarian only, costs bonus action (auto-resolved) |
| `/bonus wild-shape` | `/bonus wild-shape wolf` or `/bonus wild-shape brown-bear` | Wild Shape — Druid only, transform into a beast (auto-resolved, see Wild Shape) |
| `/bonus revert` | `/bonus revert` | Revert from Wild Shape to true form — Druid only, costs bonus action (auto-resolved) |
| `/bonus font-of-magic` | `/bonus font-of-magic convert --slot 2` or `/bonus font-of-magic create --level 3` | Sorcerer only — convert spell slots to sorcery points or vice versa (auto-resolved) |
| `/bonus bardic-inspiration` | `/bonus bardic-inspiration AR` | Bard only — grant an inspiration die to an ally (auto-resolved, see Bardic Inspiration) |
| `/bonus martial-arts` | `/bonus martial-arts` | Monk only — free unarmed strike after Attack action, no ki cost (auto-resolved) |
| `/bonus flurry-of-blows` | `/bonus flurry-of-blows` | Monk only — 2 unarmed strikes after Attack action, costs 1 ki (auto-resolved) |
| `/bonus patient-defense` | `/bonus patient-defense` | Monk only — Dodge as bonus action, costs 1 ki (auto-resolved) |
| `/bonus step-of-the-wind` | `/bonus step-of-the-wind` | Monk only — Dash or Disengage as bonus action, costs 1 ki (auto-resolved) |
| `/shove` | `/shove OS` | Shove a target (push or knock prone) |
| `/interact` | `/interact draw longsword` | Object interaction (first per turn is free; see Free Object Interaction) |
| `/action disengage` | `/action disengage` | Disengage — move without provoking opportunity attacks (auto-resolved) |
| `/action dash` | `/action dash` | Dash — double movement this turn (auto-resolved) |
| `/action dodge` | `/action dodge` | Dodge — attacks against you have disadvantage until next turn (auto-resolved) |
| `/action help` | `/action help Thorn G1` | Help — grant ally advantage on next attack/check (auto-resolved) |
| `/action ready` | `/action ready I attack when the goblin moves past me` | Ready — hold action for a trigger (see Reactions) |
| `/action hide` | `/action hide` | Hide — Stealth vs passive Perception (auto-resolved) |
| `/action escape` | `/action escape` or `/action escape --acrobatics` | Escape a grapple — contested check (auto-resolved) |
| `/action stand` | `/action stand` | Stand up from prone — costs half your movement speed, no action (auto-resolved) |
| `/action drop-prone` | `/action drop-prone` | Drop prone voluntarily — no action or movement cost (auto-resolved) |
| `/action` | `/action flip the table` | Freeform action — routed to `#dm-queue` |
| `/action cancel` | `/action cancel` | Cancel your pending freeform action before the DM resolves it |
| `/reaction` | `/reaction Shield if I get hit` | Pre-declare reaction intent (usable any time) — routed to `#dm-queue` |
| `/deathsave` | `/deathsave` | Roll a death saving throw (only at 0 HP) |
| `/command` | `/command FAM attack G1` or `/command SW attack G2` | Issue a command to a summoned creature you control (see Summoned Creatures & Companions) |
| `/action channel-divinity` | `/action channel-divinity turn-undead` or `/action channel-divinity preserve-life` | Channel Divinity — Cleric (lvl 2+) / Paladin (lvl 3+), costs action (auto-resolved or DM queue, see Channel Divinity) |
| `/action surge` | `/action surge` | Action Surge — gain an additional action this turn (Fighter only, auto-resolved) |
| `/done` | `/done` | End turn, advance initiative |

**Anytime commands** (usable in or out of combat, no turn required):

| Command | Example | Description |
|---|---|---|
| `/check` | `/check perception` or `/check athletics --adv` or `/check medicine AR` | Skill/ability check (DM-prompted or player-initiated). Optional target for targeted checks (e.g., stabilization) |
| `/save` | `/save dex` | Saving throw (DM-prompted) |
| `/rest` | `/rest short` or `/rest long` | Initiate a rest (DM must approve, not during combat) |

Note: saving throws triggered by spells and attacks (e.g., Fireball's DEX save) prompt affected players to roll via `/save` — the bot pings them in `#your-turn`. Enemy saves are rolled by the DM from the dashboard.

**Save modifier auto-calculation:** `/save` automatically includes all applicable modifiers — proficiency (if proficient in that save), ability modifier, feature bonuses (e.g., Paladin's Aura of Protection via `modify_save`), spell effects (e.g., Bless +1d4), condition effects (e.g., Exhaustion 3 disadvantage, Dodge advantage on DEX saves), and magic item bonuses. The combat log shows a full breakdown so players can verify:
```
🎲  Aria rolls DEX save — 18 (11 + 2 DEX + 3 prof + 2 Aura) vs DC 15 — Success!
🎲  Kael rolls WIS save — 9 (7 + 2 WIS) vs DC 14 — Fail!
```

**Utility commands** (usable any time):

| Command | Example | Description |
|---|---|---|
| `/status` | `/status` | View your active conditions, concentration, temp HP, exhaustion, and reaction declarations (ephemeral) |
| `/equip` | `/equip longsword`, `/equip shield`, `/equip shortsword --offhand` | Equip a weapon or shield (see Equipment Management below) |
| `/undo` | `/undo` or `/undo wrong target` | Request the DM to undo your last action (see Undo & Corrections) |
| `/inventory` | `/inventory` | View your inventory, equipment, and gold |
| `/use` | `/use healing-potion` | Use a consumable item |
| `/give` | `/give healing-potion AR` | Give an item to an adjacent ally |
| `/loot` | `/loot` | Pick up items from the loot pool after combat |
| `/attune` | `/attune cloak-of-protection` | Attune to a magic item (requires short rest, max 3 attuned items) |
| `/unattune` | `/unattune cloak-of-protection` | End attunement with a magic item (frees an attunement slot) |
| `/prepare` | `/prepare` | Change prepared spells (prepared casters only, out of combat) |
| `/register` | `/register Thorn` | Link Discord account to a character (DM approves) |
| `/setup` | `/setup` | Auto-create channel structure (DM only, run once) |
| `/recap` | `/recap` or `/recap 3` | Show combat log entries since your last turn (or last N rounds). Ephemeral |
| `/help` | `/help` or `/help attack` | Show command list, or detailed usage for a specific command |

**Command discoverability:** Discord's built-in slash command UI is the primary discovery mechanism — typing `/` shows all registered commands with parameter hints. The `/help` command supplements this with usage examples, available flags (e.g., `--gwm`, `--adv`), and context-specific tips (e.g., remaining attacks, available spell slots).

### Attack Mechanics

**Weapon selection:** each character has an equipped weapon (set via `/equip`). `/attack G2` uses it; `/attack G2 handaxe` overrides for that swing. Backend validates the character has the weapon.

**Extra Attack:** resolved one swing at a time. Each `/attack` resolves a single attack roll. Backend tracks attacks remaining by class/level (Fighter 5 = 2, Fighter 11 = 3, Fighter 20 = 4). After each swing, the bot reports remaining attacks. Players retarget freely between swings. Unused attacks forfeited on `/done`. Extra Attack does not stack across multiclassed classes — the system uses the highest `attacks_per_action` value from all class entries at their respective levels.

**Two-Weapon Fighting:** when a character attacks with a light melee weapon in their main hand (`equipped_main_hand`), they can use their bonus action to attack with a different light melee weapon held in the off-hand (`equipped_off_hand`). Invoked via `/bonus offhand`. The off-hand attack does not add the ability modifier to damage unless the character has the Two-Weapon Fighting fighting style. System validates both `equipped_main_hand` and `equipped_off_hand` have the "light" property.

**Rage (Barbarian):** `/bonus rage` activates rage. Costs the bonus action (`bonus_action_used = true`). Sets `is_raging = true` on the combatant. Requires Barbarian class and remaining rage uses in `feature_uses["rage"]` (2/day at level 1, scaling to 6/day at level 17, unlimited at level 20). While raging, all Rage effects from the Feature Effect System are active (damage bonus on melee STR attacks, resistance to B/P/S, advantage on STR checks/saves). Rage lasts up to 1 minute (10 rounds), tracked via `rage_rounds_remaining` on the combatant. Rage ends early if:
- The Barbarian's turn ends and they have neither attacked a hostile nor taken damage since their last turn — the system auto-tracks `rage_attacked_this_round` and `rage_took_damage_this_round` (reset at turn start, checked at turn end). If neither is true, rage ends automatically with a combat log message.
- The Barbarian falls unconscious (`hp_current = 0`) — rage ends immediately.
- The Barbarian chooses to end it: `/bonus end-rage` (free, no action cost).
- 10 rounds elapse — system auto-ends at turn start.

Cannot rage while wearing heavy armor (system validates `equipped_armor` weight class). Cannot cast spells or concentrate on spells while raging (system blocks `/cast` and drops active concentration when rage activates).

Combat log output:
```
🔥  Kael enters a Rage! (3 rages remaining today)
📋 Remaining: 🏃 30ft move | ⚔️ 2 attacks | 🤚 Free interact | 🛡️ Reaction

🔥  Kael's Rage ends — didn't attack or take damage this round
🔥  Kael ends their Rage
```

**Wild Shape (Druid):** `/bonus wild-shape [beast-name]` transforms the Druid into a beast form. Costs the bonus action (`bonus_action_used = true`). Requires Druid class and remaining Wild Shape uses in `feature_uses["wild-shape"]` (2 uses, recharges on short rest).

**Validation:** the system checks:
- Beast exists in the `creatures` table with `type = 'beast'`
- Beast CR ≤ the Druid's Wild Shape CR limit (CR ¼ at level 2, CR ½ at level 4, CR 1 at level 8; Circle of the Moon: CR 1 at level 2, CR scaling to class level ÷ 3 at level 6+)
- Swimming speed: beast can't have a swim speed unless Druid is level 4+
- Flying speed: beast can't have a fly speed unless Druid is level 8+
- If validation fails: "❌ Can't Wild Shape into [beast] — CR too high (max CR ¼ at Druid level 2)" or similar

**Stat swap on transformation:** the system snapshots the Druid's current state into `wild_shape_original` JSONB on the combatant, then overwrites combat-relevant stats from the beast's creature entry:
- `hp_max` and `hp_current` → beast's `hp_average` (original HP preserved in snapshot)
- `ac` → beast's `ac`
- `ability_scores` (STR, DEX, CON) → beast's scores. INT, WIS, CHA remain the Druid's own
- `speed_ft` → beast's `speed.walk`; flying/swimming speeds available if the beast has them
- `attacks` available via `/attack` pulled from beast's `attacks` JSONB (bite, claw, etc.)
- Equipped weapons and armor are suppressed (cannot use equipment in beast form)

**Retained from Druid form:** INT, WIS, CHA scores; skill proficiencies and saving throw proficiencies (use beast's physical stats but Druid's proficiency bonus if higher); class features, racial traits, and feats (as long as the beast form can physically perform them — e.g., can't cast spells unless level 18+ with Beast Spells feature); personality, alignment, language comprehension (but can't speak)

**Spellcasting restriction:** while in Wild Shape, `/cast` is blocked: "❌ Can't cast spells in Wild Shape." Exception: Druids with the Beast Spells feature (level 18+) can cast spells with somatic and verbal components in beast form. Concentration on pre-existing spells is maintained — the Druid does not lose concentration on transformation.

**HP and damage in beast form:**
- Damage reduces beast form HP first
- When beast form HP reaches 0, the Druid reverts to true form and excess damage carries over to original HP
- If overflow damage reduces original HP to 0, the Druid falls unconscious and begins death saves
- Healing in beast form applies to beast form HP (capped at beast `hp_max`)

**Reverting:**
- `/bonus revert` — voluntary revert, costs bonus action. Restores original stats from `wild_shape_original` snapshot
- Automatic revert when beast form HP drops to 0 (no action cost)
- Automatic revert when the Druid falls unconscious, is incapacitated, or dies
- On revert, `wild_shape_original` is cleared from the combatant

**Map token:** while in Wild Shape, the combatant's token changes to indicate beast form (beast icon with the Druid's short ID). On revert, the token returns to normal.

**Edge cases routed to DM queue:**
- "Can I use my equipment in this form?" — `/action` freeform → DM queue
- "Can I speak to my allies?" — RAW no, but DM may allow gestures/signals
- "Can I open a door with paws?" — depends on the beast; DM adjudicates

Combat log output:
```
🐺  Elara Wild Shapes into a Wolf! (1 use remaining)
     ❤️  HP: 11 | 🛡️ AC: 13 | 🏃 40ft
     ⚔️  Attacks: Bite (+4, 2d4+2 piercing, DC 11 STR save or prone)
📋 Remaining: 🏃 40ft move | ⚔️ 1 attack | 🤚 Free interact | 🛡️ Reaction

🐺  Elara's wolf form drops to 0 HP! Reverts to Druid form (5 overflow damage → 23/28 HP)

🐺  Elara reverts from Wild Shape
```

**Monk Ki & Martial Arts:** Monks use ki points to fuel special abilities. Tracked in `feature_uses["ki"]` with `max` equal to Monk class level (e.g., `{current: 4, max: 4, recharge: "short"}`). Ki points are available starting at Monk level 2 and recharge on short or long rest.

**Martial Arts (passive, level 1+):** when a Monk attacks with an unarmed strike or a monk weapon (any simple melee weapon without the heavy or two-handed property, plus shortswords), the following apply automatically via the Feature Effect System:
- Can use DEX instead of STR for attack and damage rolls (system auto-selects the higher modifier, same as finesse)
- Damage die is replaced by the Martial Arts die (1d4 at level 1, 1d6 at level 5, 1d8 at level 11, 1d10 at level 17) if it would be higher than the weapon's base damage die
- **Martial Arts bonus attack:** after taking the Attack action with an unarmed strike or monk weapon, the Monk can make one unarmed strike as a bonus action at no cost — invoked via `/bonus martial-arts`. This is separate from and mutually exclusive with Flurry of Blows (if you use Flurry, you don't also get the free Martial Arts bonus attack)

**Ki abilities (level 2+):**

| Ability | Command | Cost | Effect |
|---|---|---|---|
| Flurry of Blows | `/bonus flurry-of-blows` | 1 ki | Immediately after taking the Attack action, make 2 unarmed strikes as a bonus action (replaces the free Martial Arts bonus attack) |
| Patient Defense | `/bonus patient-defense` | 1 ki | Take the Dodge action as a bonus action (attacks against you have disadvantage until your next turn, tracked via `is_dodging` on combatant) |
| Step of the Wind | `/bonus step-of-the-wind` | 1 ki | Take the Dash or Disengage action as a bonus action. Also doubles jump distance for the turn |

**Flurry of Blows workflow:**
1. Monk takes at least one `/attack` with an unarmed strike or monk weapon
2. Monk uses `/bonus flurry-of-blows` — system validates: Attack action was used this turn, Monk class, ki ≥ 1
3. System deducts 1 ki point, sets `bonus_action_used = true`, and grants 2 unarmed strike attacks
4. Monk resolves each strike with `/attack [target] unarmed` (can retarget between strikes)
5. If Flurry is used, the free Martial Arts bonus attack is no longer available

**Stunning Strike (level 5+):** after a Monk hits a creature with a melee weapon attack, the bot posts an ephemeral prompt: "💫 Stunning Strike? [Yes — 1 ki] [No]" (only if `feature_uses["ki"].current ≥ 1`). On selection:
- 1 ki point is deducted
- Target must make a CON saving throw (DC = 8 + proficiency bonus + WIS modifier)
- On failure: target is Stunned until the end of the Monk's next turn (condition added with `duration_rounds: 1`)
- On success: no effect, ki is still spent
- The prompt appears on every melee hit (including Flurry of Blows strikes), but the Monk can decline
- Driven by the `resource_on_hit` effect type in the Feature Effect System, same pattern as Divine Smite

**Ki point validation:** if insufficient ki remains, the ability is rejected: "❌ Not enough ki — Flurry of Blows costs 1 ki (you have 0 remaining)."

**Monk Unarmored Defense:** already handled by `ac_formula = "10 + DEX + WIS"` on the character (see question #17). Only applies when the Monk wears no armor and has no shield equipped.

**Monk Unarmored Movement (level 2+):** speed bonus (+10ft at level 2, scaling to +30ft at level 18) applied automatically via the Feature Effect System's `modify_speed` effect type when the Monk is not wearing armor or a shield.

Combat log output:
```
👊  Ren uses Flurry of Blows! (1 ki, 3 remaining)
📋 Remaining: 🏃 20ft move | ⚔️ 2 unarmed strikes | 🛡️ Reaction

👊  Ren hits Goblin #1 — 1d6+4 bludgeoning → 8 damage
    💫 Stunning Strike! Goblin #1 CON save DC 14... rolled 9 — STUNNED until end of Ren's next turn

👊  Ren hits Goblin #2 — 1d6+4 bludgeoning → 6 damage
    💫 Stunning Strike? [Yes — 1 ki] [No] → declined

🛡️  Ren uses Patient Defense! (1 ki, 2 remaining)
    Dodge active — attacks against Ren have disadvantage

💨  Ren uses Step of the Wind — Dash! (1 ki, 2 remaining)
    🏃 Movement: 40ft → 80ft this turn
```

**`/help ki` output** (ephemeral, shown when a Monk types `/help ki`):
```
👊 Ki Abilities — Monk

  Martial Arts (free):
    /bonus martial-arts              Free unarmed strike after Attack action (no ki cost)

  Ki abilities (1 ki each, recharge on short rest):
    /bonus flurry-of-blows           2 unarmed strikes after Attack action (replaces martial-arts)
    /bonus patient-defense            Dodge as bonus action (disadv on attacks against you)
    /bonus step-of-the-wind           Dash or Disengage as bonus action + double jump

  On-hit (prompted automatically):
    Stunning Strike (lvl 5+)         1 ki — target CON save or Stunned (prompted on melee hit)

  Ki points: Monk level (use /status to check)    Recharge: short rest

  Martial Arts die: 1d4 (lvl 1) → 1d6 (5) → 1d8 (11) → 1d10 (17)
  Unarmored Movement: +10ft (lvl 2) → +15ft (6) → +20ft (10) → +25ft (14) → +30ft (18)
```

**Bardic Inspiration (Bard):** Bards can grant an inspiration die to an ally as a bonus action. Uses are tracked in `feature_uses["bardic-inspiration"]` with `max` equal to CHA modifier (minimum 1), e.g., `{current: 3, max: 3, recharge: "long"}`. At Bard level 5+ (Font of Inspiration), recharge changes to `"short"`.

**Granting:** `/bonus bardic-inspiration [target-short-id]` (e.g., `/bonus bardic-inspiration AR`). System validates: Bard class, bonus action available, uses remaining, target is an ally (not self). On success:
- Bonus action consumed (`bonus_action_used = true`)
- `feature_uses["bardic-inspiration"].current` decremented
- Target's combatant gains `bardic_inspiration` field: `{die: "d6", source: "Thorn"}`
- Target receives a notification in `#your-turn`: "🎵 You received Bardic Inspiration (d6) from Thorn! You can add it to one attack roll, ability check, or saving throw."
- Combat log: "🎵 Thorn grants Bardic Inspiration (d6) to Aria"

**Die scaling:** d6 (level 1), d8 (level 5), d10 (level 10), d12 (level 15). Die size derived from Bard class level.

**Using the die:** when a creature with Bardic Inspiration makes an attack roll, ability check, or saving throw, the bot posts an ephemeral prompt after the roll: "🎵 Use Bardic Inspiration? [Yes — +d6] [No]". The player can see the initial roll result before deciding (per 5e RAW). On use:
- The inspiration die is rolled and added to the total
- `bardic_inspiration` field is cleared from the combatant
- Combat log appends: "🎵 Aria uses Bardic Inspiration — +4 (d6) → new total: 18"
- If the prompt times out (30 seconds), the die is not used and remains available

**Turn status visibility:** when a character has Bardic Inspiration, `🎵 Bardic Inspiration (d6)` is appended to their turn status prompt (both at turn start and after each command), so the player never forgets it is available.

**Expiration:** Bardic Inspiration expires after 10 minutes. In async play, the system tracks grant time; if the die is unused after 10 minutes of real time, it is auto-removed and the bot notifies the holder: "🎵 Your Bardic Inspiration from Thorn has expired." The DM can extend or waive the timer from the dashboard for long async gaps.

**`/help rogue` output** (ephemeral, shown when a Rogue types `/help rogue`):
```
🗡️ Rogue Abilities

  Cunning Action (bonus action, level 2+):
    /bonus cunning-action dash          Dash as bonus action
    /bonus cunning-action disengage     Disengage as bonus action (no OAs this turn)
    /bonus cunning-action hide          Hide as bonus action (Stealth vs passive Perception)

  Sneak Attack (automatic, once per turn):
    Triggered on hit with finesse or ranged weapon when you have advantage
    OR when an ally is within 5ft of the target (and you don't have disadvantage)
    Damage: 1d6 per 2 Rogue levels (rounded up) — e.g., 3d6 at level 5

  Expertise (passive):
    Double proficiency on selected skills — auto-applied to all checks

  Uncanny Dodge (lvl 5+, reaction):
    /reaction uncanny-dodge             Halve damage from one attack you can see
    (Prompted automatically when hit by an attack)

  Evasion (lvl 7+, passive):
    DEX saves: success = no damage, fail = half damage (auto-applied)
```

**Finesse weapons:** weapons with the "finesse" property (rapier, dagger, shortsword, etc.) allow the attacker to use either STR or DEX for attack and damage rolls. The system auto-selects the higher of the two modifiers — no player input required.

**Loading property:** weapons with the "loading" property (crossbows) can only fire once per action, bonus action, or reaction regardless of Extra Attack, unless the character has the Crossbow Expert feat. System limits attacks to 1 when a loading weapon is used.

**Unarmed strikes:** every creature can make an unarmed strike. Represented as a built-in pseudo-weapon in the weapons table (`id: "unarmed-strike"`, `damage: "0"`, `damage_type: "bludgeoning"`, `weapon_type: "simple_melee"`). Damage is 1 + STR modifier (the flat 1 is inherent to unarmed strikes, not a die roll). When `equipped_main_hand` is null, `/attack` defaults to unarmed strike; players can also explicitly use `/attack [target] unarmed`. Monks' Martial Arts feature overrides the damage via the Feature Effect System, replacing the flat 1 with their Martial Arts die (e.g., 1d4 → 1d6 → 1d8 → 1d10 by level).

**Versatile weapons:** weapons with the "versatile" property can be used one-handed or two-handed for increased damage. Use `/attack [target] --twohanded` to roll the `versatile_damage` die instead of the base damage die. The `--twohanded` flag is rejected if `equipped_off_hand` is not null (off-hand must be free to grip with both hands).

**Reach weapons:** weapons with the "reach" property (glaive, halberd, pike) extend melee range to 10ft instead of the standard 5ft. The system validates attack distance against the weapon's reach when processing `/attack`. If the target is beyond reach, the command is rejected: "Target is out of melee range (10ft reach)."

**Heavy weapons:** weapons with the "heavy" property (greataxe, greatsword, heavy crossbow, etc.) are unwieldy for small creatures. Small or Tiny creatures have disadvantage on attack rolls with heavy weapons. Auto-detected from the creature/character's `size` (via `races.size` or `creatures.size`) and the weapon's `properties` array.

**Ammunition:** weapons with the "ammunition" property (longbow, crossbow, etc.) consume one piece of ammunition per attack. The system auto-deducts from the character's `inventory` on each `/attack` with an ammunition weapon. If no ammunition remains, the command is rejected: "No arrows remaining." After combat ends, half of expended ammunition can be recovered — the DM triggers recovery from the dashboard, and the system restores half (rounded down) to inventory.

**Thrown weapons:** weapons with the "thrown" property (handaxe, javelin, dagger, etc.) can be thrown for a ranged attack using `range_normal_ft` / `range_long_ft`. Beyond normal range: disadvantage (per standard ranged rules). Beyond long range: attack auto-rejected. After a thrown attack, the weapon is removed from the character's hand. The character must draw another weapon (free object interaction) or retrieve the thrown weapon (requires movement to the target's square + free object interaction).

**Attack modifier flags** (opt-in per swing):
- `--gwm` — Great Weapon Master: -5 to hit, +10 damage. Requires a heavy melee weapon.
- `--sharpshooter` — Sharpshooter: -5 to hit, +10 damage. Requires a ranged weapon.
- `--reckless` — Reckless Attack (Barbarian): advantage on melee STR attacks this turn, enemies get advantage against you until next turn. First attack only. Requires Barbarian class.
- Invalid flags return an error explaining why.

**Advantage/disadvantage** — auto-detected from game state:
- **From conditions:** applied automatically per the Condition Effects tables (e.g., blinded attacker → disadv, stunned target → adv, prone target within 5ft → adv / beyond 5ft → disadv). See Conditions & Combat Mechanics for full details.
- **From combat context:** Reckless Attack (adv), invisible attacker/target (adv/disadv as appropriate), ranged attack while hostile within 5ft (disadv — applies to both ranged weapon attacks and ranged spell attacks; Crossbow Expert feat removes this penalty for ranged weapon attacks only), ranged attack beyond normal range (disadv), Small/Tiny creature using a Heavy weapon (disadv), attacking into/from heavily obscured zone without appropriate vision (disadv/adv per Blinded rules — see Obscurement & Lighting Zones)
- **Not auto-detected in MVP:** flanking (optional rule — may add as campaign toggle later)
- **DM override:** DM can force advantage or disadvantage from the dashboard. Posts to `#combat-log`.
- **Stacking:** when both apply, they cancel out per 5e rules — rolled normally regardless of source count.

**Critical hits:** a natural 20 on the attack roll is always a hit regardless of AC, and all damage dice are doubled (roll twice as many dice, then add modifiers once). The system auto-detects nat 20s and doubles the dice in the damage formula.

**Auto-crit:** melee attacks within 5ft against paralyzed or unconscious targets are automatic critical hits (per 5e rules) — the attack auto-hits and damage dice are doubled, same as a nat 20.

**Divine Smite (Paladin):** after a melee weapon attack hits, if the Paladin has spell slots remaining, the bot posts an ephemeral prompt with Discord buttons: "⚡ Divine Smite? [1st] [2nd] [3rd] [No]" — only showing slot levels the Paladin currently has available. The Paladin picks a slot level or declines. On selection:
- The chosen spell slot is consumed (`spell_slots` decremented)
- Smite damage is `2d8 radiant` at 1st level, +1d8 per slot level above 1st (max 5d8 at 4th-level slot)
- +1d8 bonus damage if the target is undead or fiend (auto-detected from `creatures.creature_type`)
- On a critical hit, all smite dice are doubled (the prompt notes: "🎯 Critical — smite dice doubled!")
- Smite damage is appended to the original attack's combat log entry
- If the Paladin declines or the prompt times out (30 seconds), no smite is applied and the turn continues
- The prompt only appears on melee weapon hits — not ranged attacks, not misses
- Driven by the `resource_on_hit` effect type in the Feature Effect System, so future on-hit features (e.g., Battlemaster maneuvers) follow the same prompt pattern

### Channel Divinity (Cleric / Paladin)

Clerics (level 2+) and Paladins (level 3+) can invoke Channel Divinity to produce powerful effects. Uses are tracked in `feature_uses["channel-divinity"]` with `recharge: "short"`:

| Class | Uses | Progression |
|-------|------|-------------|
| Cleric | 1 (lvl 2), 2 (lvl 6), 3 (lvl 18) | `max` increases at listed levels |
| Paladin | 1 (lvl 3), 2 (lvl 15) | `max` increases at listed levels |

**Command:** `/action channel-divinity [option]` — costs the action (`action_used = true`). System validates: correct class, level requirement met, uses remaining (`feature_uses["channel-divinity"].current ≥ 1`).

**Turn Undead (all Clerics):** `/action channel-divinity turn-undead` — auto-resolved:
- Each undead within 30ft that can see or hear the Cleric must make a WIS saving throw vs the Cleric's spell save DC
- Auto-detected from `creatures.creature_type = "undead"` and distance calculation
- On fail: Turned condition applied for 1 minute (10 rounds). Turned creature must Dash away from the Cleric; can't willingly move closer or take reactions. Ends early if the creature takes damage
- On success: no effect
- Combat log: "✝️ Thorn channels Turn Undead — Skeleton #1 🎲 WIS save: 8 vs DC 14 — Turned! | Zombie #2 🎲 WIS save: 16 vs DC 14 — Resists!"

**Destroy Undead (Cleric 5+):** when Turn Undead is used and the Cleric is level 5+, undead that fail the save and are below the CR threshold are instantly destroyed instead of Turned:

| Cleric Level | Destroys CR |
|-------------|-------------|
| 5 | ½ or lower |
| 8 | 1 or lower |
| 11 | 2 or lower |
| 14 | 3 or lower |
| 17 | 4 or lower |

Combat log: "✝️ Skeleton #1 (CR ¼) is destroyed by Turn Undead!"

**Subclass Channel Divinity options:** each subclass gains one or more Channel Divinity options, declared in `classes.subclasses[].features_by_level` using the Feature Effect System. Options fall into two categories:

1. **Auto-resolved** — options with clear mechanical effects use the standard effect type vocabulary:
   - *Preserve Life (Life Domain):* distribute up to `5 × cleric_level` HP among creatures within 30ft, each restored to at most half their max HP. Bot prompts with eligible targets and HP budget via Discord buttons
   - *Sacred Weapon (Devotion Paladin):* add CHA modifier to attack rolls for 1 minute. Applied via `modify_attack_roll` effect with duration tracking
   - *Vow of Enmity (Vengeance Paladin):* advantage on attack rolls against one creature within 10ft for 1 minute. Applied via `conditional_advantage` effect on target

2. **DM-resolved** — options that are narrative, conditional, or open-ended route to `#dm-queue`:
   - *Knowledge of the Ages (Knowledge Domain):* gain proficiency in a skill or tool for 10 minutes — DM confirms
   - *Charm Animals and Plants (Nature Domain):* charm creatures within 30ft — DM adjudicates targets and effects
   - The DM resolves from the dashboard and the system applies any mechanical changes

**`/help cleric` output** (ephemeral, shown when a Cleric types `/help cleric`):
```
✝️ Cleric Abilities

  Channel Divinity (action, level 2+):
    /action channel-divinity turn-undead    Force undead within 30ft to flee (WIS save)
    /action channel-divinity [subclass]     Use your domain's Channel Divinity option

    Destroy Undead (lvl 5+): undead below CR threshold are destroyed on failed save
    Uses: 1 (lvl 2) → 2 (lvl 6) → 3 (lvl 18)    Recharge: short rest

  Spellcasting:
    /cast [spell] [target]       Cast a prepared spell
    /prepare                     Change prepared spells (after long rest)
    /cast [spell] --ritual       Ritual cast without expending a slot (out of combat)

  Domain spells: always prepared, don't count against your limit (shown separately in /prepare)

  Use /status to check Channel Divinity uses and active effects
```

**`/help paladin` output** (ephemeral, shown when a Paladin types `/help paladin`):
```
⚔️ Paladin Abilities

  Channel Divinity (action, level 3+):
    /action channel-divinity [option]       Use your oath's Channel Divinity option
    Uses: 1 (lvl 3) → 2 (lvl 15)           Recharge: short rest

  Divine Smite (on melee hit, prompted automatically):
    Spend a spell slot for extra radiant damage (2d8 + 1d8 per slot above 1st)
    +1d8 bonus vs undead and fiends — doubled on crit

  Lay on Hands (action):
    /action lay-on-hands [target] [hp]      Restore HP from your healing pool
    Pool: 5 × Paladin level HP              Recharge: long rest

  Spellcasting:
    /cast [spell] [target]       Cast a prepared spell
    /prepare                     Change prepared spells (after long rest)

  Aura of Protection (lvl 6+): you and allies within 10ft add your CHA mod to saves (auto-applied)
  Oath spells: always prepared, don't count against your limit

  Use /status to check Channel Divinity uses, smite slots, and lay on hands pool
```

### Equipment Management

**Weapon equipping:**
- `/equip longsword` — equips to main hand (`equipped_main_hand`)
- `/equip shortsword --offhand` — equips to off-hand (`equipped_off_hand`)
- `/equip none` — unequips main hand (defaults to unarmed strike)
- `/equip none --offhand` — unequips off-hand (frees hand)

Weapon equipping costs the **free object interaction** in combat (drawing/stowing). If the free interaction is already spent, the system rejects: "❌ Free object interaction already used this turn." Out of combat, equipping is instant.

**Shield equipping:**
- `/equip shield` — equips shield to off-hand (`equipped_off_hand`), AC recalculated (+2)
- `/equip none --offhand` — unequips shield, AC recalculated (−2)

Shields follow 5e donning/doffing rules:
- **In combat:** equipping or unequipping a shield costs an **action**. The system deducts the action resource and rejects if already spent: "❌ Action already used — donning/doffing a shield requires an action."
- **Out of combat:** instant, no cost.
- Equipping a shield while the off-hand already holds a weapon automatically stows the weapon (no extra cost). Equipping an off-hand weapon while a shield is equipped requires doffing the shield first (action cost).

**Armor equipping:**
- `/equip chain-mail --armor` — equips body armor (`equipped_armor`), AC recalculated
- `/equip none --armor` — unequips body armor, AC falls back to base (10 + DEX or `ac_formula`)

Armor donning/doffing takes minutes in 5e, so:
- **In combat:** blocked. "❌ You can't don or doff armor during combat." DM can override from the dashboard if the narrative allows it (e.g., a multi-round lull).
- **Out of combat:** instant, no time tracking. DM handles timing narratively for ambush scenarios (e.g., sleeping without armor).

Combat log output:
```
🔄  Aria equips Plate Armor (AC → 18)
🔄  Aria removes Chain Mail (AC → 12)
```

**Validation rules:**
- Two-handed weapons (`--twohanded` flag) require `equipped_off_hand` to be null
- Grappling requires a free hand — rejected if both hands occupied
- Somatic spell components require a free hand (or the hand holding the focus/shield with War Caster feat)

Combat log output:
```
🔄  Aria equips Shield (+2 AC)
🔄  Aria doffs Shield (−2 AC)
🔄  Aria draws Shortsword (off-hand)
🔄  Aria stows Longsword
```

### Spell Casting Details

**AoE targeting:** `/cast fireball D5` targets a coordinate. Backend calculates affected creatures by shape/radius from spell data (`{ shape: "sphere", radius_ft: 20 }`, `{ shape: "cone", length_ft: 15 }`, `{ shape: "line", length_ft: 60, width_ft: 5 }`). Cones originate from the caster toward the target. All affected creatures (including allies) listed in `#combat-log`.

**Spell saves:** when a spell requires saves, the bot pings each affected player in `#your-turn` to roll `/save <ability>`. Enemy saves are rolled by the DM from the dashboard. Spell damage/effects are applied once all saves are resolved.

**Concentration:** fully tracked by backend:
- One concentration spell at a time; new cast auto-drops the previous
- Taking damage triggers a concentration check — bot pings the caster to roll `/save con` (DC = max(10, half damage)); failure breaks concentration
- Being incapacitated (stunned, paralyzed, unconscious, petrified) auto-breaks concentration immediately — no save prompted
- Entering a Silence zone (or similar effect preventing verbal/somatic components) breaks concentration on spells requiring those components — auto-detected when a concentrating caster's position overlaps a Silence zone
- **Casting blocked in Silence:** on `/cast`, the system checks if the caster's position overlaps an active Silence zone. If the spell has verbal or somatic components (`components.v = true` or `components.s = true`), the cast is rejected: "You cannot cast [spell] — you are inside a zone of Silence (requires verbal/somatic components)." Spells with only material components (no V or S) are unaffected.
- Active effects (Fog Cloud zone, Spirit Guardians aura) tracked on the map

**Material components:** a spellcasting focus or component pouch is assumed for all casters — ordinary material components (no gold cost) are automatically satisfied and never block casting. For spells with **costly material components** (gold value in `material_cost_gp`), the system checks on `/cast`:

1. **Component in inventory:** if the character has the required item (e.g., "diamond" for Revivify), the cast proceeds. If `material_consumed = true`, the item is removed from inventory after casting.
2. **No component, but sufficient gold:** if the item is missing but the character has enough gold, the system offers a fallback prompt: "You don't have a diamond (300gp) — buy one for 300gp? [✅ Buy & Cast] [❌ Cancel]". Clicking Buy & Cast deducts the gold and proceeds. If `material_consumed = true`, no item is added to inventory (it's immediately consumed). If not consumed, the item is added to inventory for future casts.
3. **Neither component nor gold:** the cast is rejected: "Requires a diamond worth 300gp — you don't have one and can't afford it (current gold: 50gp)."

The gold fallback represents the assumption that components can be acquired from merchants, temples, or other sources available in the game world. The DM can also stock components directly in player inventories via the dashboard, and players can buy items from in-game merchants (DM creates a shop via the dashboard, posts available items to `#the-story`, and transfers purchased items/deducts gold through the dashboard).

**Bonus action spell auto-detection:** `/cast` is the only command needed for all spells. The system reads `spells_ref.casting_time`; if it is `'bonus action'`, the cast deducts the bonus action (`bonus_action_used = true`) instead of the action. The bot confirms in the response: "🎁 Cast as bonus action." There is no `/bonus cast` syntax.

**Bonus action spell restriction (both directions):** Per 5e rules (Sage Advice Compendium), if a player casts any spell as a bonus action on their turn, the only other spell they can cast that turn is a cantrip with a casting time of 1 action — and this applies regardless of casting order:
- **Bonus action spell first:** If a bonus action spell was cast (`bonus_action_spell_cast = true`), `/cast` with a non-cantrip action spell is rejected: "You already cast a bonus action spell this turn — you can only cast a cantrip with your action."
- **Leveled action spell first:** If a leveled (non-cantrip) action spell was cast (`action_spell_cast = true`), `/cast` with a bonus action spell is rejected: "You already cast a leveled spell with your action this turn — you cannot cast a bonus action spell."

**Metamagic (Sorcerer):** Sorcerers can modify spells using Metamagic options, powered by sorcery points. Tracked in `feature_uses["sorcery-points"]` with `max` equal to Sorcerer class level (e.g., `{current: 5, max: 5, recharge: "long"}`). Sorcerers learn Metamagic options at level 3 (2 options), level 10 (3rd), and level 17 (4th), stored in the character's `features` array.

**Syntax:** Metamagic is applied via flags on `/cast`:
```
/cast fireball D5 --quickened          ← cast as bonus action (2 SP)
/cast haste AR --twinned TH            ← twin to second target (3 SP)
/cast fireball D5 --empowered          ← reroll low damage dice (1 SP)
/cast counterspell --subtle            ← no V/S components (1 SP)
/cast fireball D5 --careful            ← allies auto-succeed on save (1 SP)
/cast fire-bolt G1 --distant           ← double range (1 SP)
/cast mage-armor --extended            ← double duration (1 SP)
/cast hold-person G1 --heightened      ← target has disadv on first save (3 SP)
```

Only one Metamagic option per spell unless otherwise noted (Empowered can combine with another).

**Metamagic options (all 8 SRD options, auto-resolved):**

| Option | Flag | Cost | Effect | Validation |
|---|---|---|---|---|
| Careful Spell | `--careful` | 1 SP | Choose up to CHA mod creatures in the AoE — they auto-succeed on the spell's saving throw | Spell must have an area and a save. Bot prompts: "Pick allies to protect: [AR] [TH] [KL]" via buttons |
| Distant Spell | `--distant` | 1 SP | Double the spell's range. Touch spells become 30ft range | Spell must have range > 0 or be touch. System doubles `range_ft` (or sets 30 for touch) for this cast |
| Empowered Spell | `--empowered` | 1 SP | Reroll up to CHA mod damage dice, must use new rolls | Spell must deal damage. Bot shows rolled dice and prompts: "Reroll which dice? [4] [2] [1] [6] [3]" via buttons. Can combine with another Metamagic option |
| Extended Spell | `--extended` | 1 SP | Double the spell's duration (max 24 hours) | Spell must have duration ≥ 1 minute. System doubles duration for tracking/expiration |
| Heightened Spell | `--heightened` | 3 SP | One target has disadvantage on its first saving throw against the spell | Spell must require a save. If multiple targets, bot prompts which target to heighten |
| Quickened Spell | `--quickened` | 2 SP | Change casting time from 1 action to 1 bonus action | Spell must have casting time of "1 action". Deducts bonus action instead of action. **Bonus action spell restriction still applies** — casting a quickened leveled spell means the caster can only use their action for a cantrip |
| Subtle Spell | `--subtle` | 1 SP | Remove verbal and somatic components | No restrictions. Spell bypasses Silence zones and cannot be Counterspelled (Counterspell requires seeing a creature casting — with no V/S, there is nothing to perceive) |
| Twinned Spell | `--twinned [target]` | SP = spell level (1 for cantrips) | Target a second creature with a single-target spell | Spell must target only one creature and not have a range of Self. Rejects AoE spells and self-only spells. Second target must be in range. Both targets resolved independently (separate attack rolls or saves) |

**Sorcery point validation:** if insufficient points remain, the cast is rejected: "❌ Not enough sorcery points — Twinned Spell costs 3 SP (you have 2 remaining)."

**Font of Magic** (`/bonus font-of-magic`): Sorcerers can convert between spell slots and sorcery points as a bonus action. Available at Sorcerer level 2+.

- **Slots → points:** `/bonus font-of-magic convert --slot 2` — expend a 2nd-level spell slot, gain 2 sorcery points. Points gained = slot level. Cannot exceed sorcery point maximum.
- **Points → slots:** `/bonus font-of-magic create --level 3` — spend sorcery points to create a spell slot. Cost follows the table: 1st = 2 SP, 2nd = 3 SP, 3rd = 5 SP, 4th = 6 SP, 5th = 7 SP. Cannot create slots above 5th level. Created slots vanish on long rest.

Combat log output:
```
✨  Elara casts Fireball at D5 — Quickened Spell! (2 SP, 3 remaining)
    🎁 Cast as bonus action
    💥 8d6 fire → 28 damage (DEX save DC 15 for half)

✨  Elara casts Haste on Aria — Twinned Spell → also targets Thorn! (3 SP, 2 remaining)
    🛡️ Aria: +2 AC, doubled speed, extra action
    🛡️ Thorn: +2 AC, doubled speed, extra action

✨  Elara casts Counterspell — Subtle Spell! (1 SP, 4 remaining)
    🤫 No verbal/somatic components — cannot be countered

✨  Elara casts Fireball at D5 — Careful Spell! (1 SP, 4 remaining)
    💥 8d6 fire → 31 damage (DEX save DC 15 for half)
    🛡️ Aria and Thorn auto-succeed on the save

✨  Elara casts Fire Bolt at G1 — Empowered Spell! (1 SP, 4 remaining)
    🎲 Rerolled: [2, 1] → [8, 5]
    💥 2d10 fire → 13 damage

🔮  Elara converts a 2nd-level spell slot → 2 sorcery points (5 SP remaining)
🔮  Elara creates a 3rd-level spell slot (5 SP → 0 SP)
```

**`/help metamagic` output** (ephemeral, shown when a Sorcerer types `/help metamagic`):
```
⚡ Metamagic — Sorcery Point Options

Apply Metamagic by adding a flag to /cast:

  --careful     (1 SP)  Allies in AoE auto-succeed on save
  --distant     (1 SP)  Double spell range (touch → 30ft)
  --empowered   (1 SP)  Reroll up to CHA mod damage dice (combinable)
  --extended    (1 SP)  Double spell duration (max 24h)
  --heightened  (3 SP)  One target has disadvantage on first save
  --quickened   (2 SP)  Cast action spell as bonus action
  --subtle      (1 SP)  No V/S components (bypasses Silence & Counterspell)
  --twinned     (Lvl SP) Second target for single-target spell (1 SP for cantrips)

Only one option per cast (except --empowered, which stacks).

Convert resources:
  /bonus font-of-magic convert --slot N   Slot → SP (gain = slot level)
  /bonus font-of-magic create --level N   SP → Slot (cost: 1st=2, 2nd=3, 3rd=5, 4th=6, 5th=7)

Current SP: use /status to check    Recharge: long rest
```

**Spell save DC:** calculated as `8 + proficiency_bonus + spellcasting_ability_modifier`. The spellcasting ability varies by class (referenced from `classes.spellcasting.ability`): INT for Wizards, WIS for Clerics/Druids/Rangers, CHA for Bards/Paladins/Sorcerers/Warlocks. Creature stat blocks store the DC directly in their abilities data.

**Spell attack rolls:** some spells require a spell attack roll instead of a saving throw. The attack roll is `d20 + proficiency_bonus + spellcasting_ability_modifier` vs the target's AC. Spells are categorized by `attack_type` on the `spells` table: `'melee'` (e.g., Shocking Grasp), `'ranged'` (e.g., Fire Bolt), or `NULL` (save-based or auto-hit). Melee spell attacks within 5ft of a prone target get advantage; ranged spell attacks against a prone target get disadvantage (same as weapon attacks).

**Spell slots:** tracked and enforced. Backend knows slots per level, deducts on cast, rejects `/cast` if no slots remaining.

**Pact Magic (Warlock):** Warlocks use a separate slot pool (`pact_magic_slots`) instead of standard `spell_slots`. Pact Magic slots are fewer (1-4 depending on level), all at the same level (scaling with Warlock level per the Warlock table), and recharge on short rest. When a Warlock casts a spell, `/cast` draws from `pact_magic_slots` by default. If the Warlock is multiclassed and has both pools, the system uses Pact Magic slots first; the player can override with `--spell-slot` to draw from regular `spell_slots` instead. Upcasting with `--slot` must still be ≤ the pact slot level. For multiclass casters, both pools are tracked and displayed separately in the spell slot UI.

**Spell preparation** (`/prepare`): Prepared casters (Cleric, Druid, Paladin) can change their prepared spell list after completing a long rest. Per 5e, they choose from their full class spell list (filtered by slots they have), up to a maximum of ability modifier + class level (minimum 1). Rangers and Wizards are known-spell casters — they change spells on level-up, not daily.

How `/prepare` works:
1. Player types `/prepare` (only available out of combat, i.e., `encounter.status != 'active'`)
2. Bot responds with an ephemeral message showing:
   - Current prepared spells (checked)
   - Full class spell list for available slot levels (unchecked), with spell school and brief description
   - Remaining preparation slots: "**N / M** spells prepared"
3. Player selects/deselects spells via Discord select menus (paginated by spell level)
4. Player clicks **Confirm** to save, or **Cancel** to discard changes
5. System validates: count ≤ max prepared, all spells are on the class spell list, character has slots of that level
6. Updated list stored in character data; confirmation posted as ephemeral message

After a long rest, the system reminds prepared casters: "You can change your prepared spells with `/prepare`." This is a hint, not a requirement — players keep their existing list if they don't act.

Domain/Oath/Circle spells (always-prepared subclass spells) are shown separately and cannot be removed. They do not count against the preparation limit.

**Upcasting:** spells can be cast at higher levels for increased effect by specifying `/cast fireball D5 --slot 5` to use a 5th-level slot. The system parses the spell's `higher_levels` field (e.g., "1d6 per slot level above 3rd") and auto-calculates the scaled damage or healing. If `--slot` is omitted, the system defaults to the lowest available slot of sufficient level. The `--slot` value must be ≥ the spell's base level and the character must have a slot available at that level.

**Ritual casting:** spells with `ritual: true` can be cast without expending a spell slot by using `/cast detect-magic --ritual`. Ritual casting adds 10 minutes to the casting time and is only available outside active combat (`encounter.status != 'active'`). Only classes with the Ritual Casting feature (Wizard, Cleric, Druid, Bard with Ritual Casting) can use this option. The system checks both the spell's `ritual` flag and the character's class features.

**Spell range:** enforced by backend. Touch spells require adjacency (5ft), self spells need no target.

**Teleportation spells:** spells with a non-null `teleport` JSONB field bypass all path validation — no movement cost, no difficult terrain, no occupied-tile checks, no opportunity attacks. The `/cast` handler reads the `teleport` field to determine relocation behavior:

- `teleport.target`: `"self"` (caster teleports), `"creature"` (target creature teleports), `"self+creature"` (caster + one willing companion)
- `teleport.requires_sight`: `true` if the destination must be visible to the caster (e.g., Misty Step), `false` if not (e.g., Dimension Door to described location)
- `teleport.companion_range_ft`: for `"self+creature"` spells, max distance the companion must be from the caster (e.g., Dimension Door = 5)
- `teleport.additional_effects`: optional — e.g., Thunder Step deals 3d10 thunder damage to creatures within 10ft of the departure point

**Validation:** the system checks only: (1) destination tile is unoccupied, (2) destination is within spell range, (3) line of sight to destination if `requires_sight = true`, (4) companion is willing and within `companion_range_ft` if applicable. Path between origin and destination is **not** validated.

**SRD teleportation spells:**

| Spell | Level | `teleport` data |
|---|---|---|
| Misty Step | 2 | `{target: "self", requires_sight: true}` |
| Thunder Step | 3 | `{target: "self+creature", requires_sight: true, companion_range_ft: 5, additional_effects: "3d10 thunder to creatures within 10ft of departure"}` |
| Dimension Door | 4 | `{target: "self+creature", requires_sight: false, companion_range_ft: 5}` |
| Far Step | 5 | `{target: "self", requires_sight: true}` |
| Arcane Gate | 6 | `{target: "portal", requires_sight: true}` |
| Teleport | 7 | `{target: "party", requires_sight: false}` |
| Scatter | 6 | `{target: "creatures", requires_sight: true}` |
| Word of Recall | 6 | `{target: "party", requires_sight: false}` |

Note: higher-level teleportation spells (Teleport, Word of Recall) that move beyond the current map are resolved via `#dm-queue` as narrative events rather than grid repositioning.

**Cantrip damage scaling:** cantrip damage dice scale automatically based on character level — 2 dice at level 5, 3 dice at level 11, 4 dice at level 17. The system auto-calculates the correct number of dice from the caster's character level using the spell's `damage` JSONB field (`cantrip_scaling: true` flag). No player input required. Example: Fire Bolt deals 1d10 at levels 1–4, 2d10 at 5–10, 3d10 at 11–16, 4d10 at 17+.

### Reactions

Players pre-declare reaction intent using `/reaction`. The DM resolves all reactions manually.

**Declaration:**
- `/reaction Shield if I get hit`
- `/reaction OA if G1 moves away`
- `/reaction Counterspell if enemy casts`
- Declarations are **freeform text** — players are encouraged to use short IDs (e.g., `G1`, `OS`) for clarity, but any natural language is accepted. Ambiguous declarations (e.g., "if goblin moves away" when multiple goblins are present) are resolved by the DM using battlefield context; the DM may ping the player for clarification if intent is unclear
- Players can have **multiple active declarations** simultaneously (e.g., Shield and Counterspell both active)
- Declarations persist until used, cancelled (`/reaction cancel [description]` to cancel a specific one, `/reaction cancel-all` to clear all), or the encounter ends
- One reaction per round per player (per 5e rules) — tracked by `turns.reaction_used`. Reaction resets at the start of the creature's turn (not the start of the round), matching 5e rules and ensuring correct sequencing in async play
- When multiple declarations trigger from the same event (e.g., enemy casts a spell at you — both Shield and Counterspell could fire), the DM chooses which to surface and resolve. After one fires, `reaction_used = true` and remaining declarations stay active but dormant until the reaction resets next round.

**DM workflow:**
1. DM sees all active declarations in the dashboard's **Active Reactions Panel** (see DM Dashboard) — grouped by combatant, showing declaration text and status
2. When the trigger occurs during enemy/NPC turns, DM decides whether it fires (and which one, if multiple match)
3. DM resolves in the dashboard (rolls, applies effects) and posts the result
4. System marks the player's reaction as spent for the round

**Counterspell resolution:** when a Counterspell declaration triggers (an enemy casts a spell within 60ft), the system follows a two-step flow that does not stall combat:

1. **DM triggers Counterspell** from the Active Reactions Panel. The system pings the declaring player in `#your-turn` with: "Enemy is casting **[Spell Name]**. Use Counterspell? Pick a slot level:" followed by Discord buttons `[3] [4] [5] … [max available]` and `[Pass]`.
   - The spell **name** is revealed, but the **cast level** is not — preserving the strategic tension of slot selection.
   - If the player picks Pass, the enemy spell resolves normally and the reaction is not consumed.
2. **If the player's Counterspell slot level ≥ enemy cast level:** the enemy spell is automatically countered. Posted to `#combat-log`: "[Player] counters [Enemy]'s [Spell]!"
3. **If the player's Counterspell slot level < enemy cast level:** the system immediately prompts the player to roll an ability check: "Your Counterspell must overcome a level [N] spell — roll `/check spellcasting`" (DC = 10 + enemy spell level). The player rolls using their spellcasting ability modifier (no proficiency — per 5e RAW). Success counters the spell; failure means the enemy spell resolves and the Counterspell slot is still expended.
   - The enemy cast level is revealed only at this step — after the player has committed their slot.
4. **Async timing:** the enemy turn continues while the Counterspell prompt is pending. If the Counterspell succeeds, the DM retroactively removes the spell's effects (same pattern as opportunity attacks). If the player does not respond within the turn timeout, the Counterspell is forfeited and the enemy spell resolves.

**Readied Actions:** a player can use their action to ready a response to a trigger via `/action ready [description]` (e.g., `/action ready I attack when the goblin moves past me`). This costs the action for the turn. When the trigger occurs, the readied action fires using the creature's reaction (`reaction_used = true`). If the trigger never occurs before the creature's next turn, the readied action is lost. For readied spells: the spell slot is expended when readying (not when releasing), and the caster must hold concentration on the readied spell until the trigger fires — if concentration is broken, the spell is lost along with the slot. Readied actions follow the same DM-resolution flow as other `/reaction` declarations.

**Readied action expiry notice:** when a player's turn starts and they had a readied action that expired unused, the turn-start prompt includes a notice:
```
⏳ Your readied action expired unused: "I attack when the goblin moves past me"
```
For readied spells, the notice also confirms the consequence:
```
⏳ Your readied action expired unused: "Cast Hold Person if shaman moves"
   → Concentration on Hold Person ended. 2nd-level spell slot lost.
```
This ensures the player is never surprised by a lost action or spell slot in async play, where a single round can span hours or days. Players can also check active readied actions mid-round via `/status`.

**System-generated reaction triggers:** opportunity attacks (see Opportunity Attacks section) bypass the `/reaction` declaration flow — the system auto-detects and prompts directly. Unlike other reactions, OA prompts use a queue-and-continue model: movement is not paused, and the hostile has until end-of-round to respond (see Opportunity Attacks).

**Why this works for async:** zero stalling — combat never pauses for a reaction response. Players declare intent on their own time. DM has full control over timing and adjudication.

### Freeform Actions

For anything that can't be expressed through structured commands, `/action` routes to the DM:

```
/action I want to flip the table at B3 for half cover and duck behind it
/action I grab the chandelier and swing across to F2
```

These post to `#dm-queue`. The DM resolves them in the dashboard, applies state changes, and the bot posts the result. Structured commands handle ~90% of combat; `/action` is the escape hatch for creative play.

**Cancelling a pending freeform action:** `/action cancel` withdraws the player's most recent pending freeform action, as long as the DM has not yet started resolving it. The bot marks the `#dm-queue` message with ~~strikethrough~~ and "Cancelled by player", then confirms to the player with an ephemeral message: "✅ Pending action cancelled: *flip the table*". If there is no pending freeform action, the command is rejected: "❌ No pending action to cancel." If the DM has already resolved it, the command is rejected: "❌ That action has already been resolved — use `/undo` to request a correction instead."

### Standard Actions (Auto-Resolved)

The following standard actions are recognized by `/action` and resolved automatically without routing to `#dm-queue`:

**Disengage:** `/action disengage` prevents the character from provoking opportunity attacks for the rest of the turn. Costs the action (`action_used = true`). Tracked via `has_disengaged = true` on the `turns` table. Rogues can Disengage as a bonus action via `/bonus cunning-action disengage` (costs bonus action instead). Monks can Disengage as a bonus action by spending 1 ki point via `/bonus step-of-the-wind`.

**Dash:** `/action dash` adds the character's speed to their remaining movement for the turn. Costs the action (`action_used = true`). Rogues can Dash as a bonus action via `/bonus cunning-action dash` (costs bonus action instead). Monks can Dash as a bonus action by spending 1 ki point via `/bonus step-of-the-wind`. The extra movement is subject to difficult terrain, prone costs, and all other movement modifiers.

**Dodge:** `/action dodge` grants two benefits until the start of the character's next turn: attacks against the character have disadvantage, and the character has advantage on DEX saving throws. Tracked via a "dodge" condition with 1-round duration. Already referenced in Turn Timeout (auto-skip applies Dodge).

**Help:** `/action help [ally] [target]` grants an ally advantage on their next attack roll against the specified target, or advantage on their next ability check. Costs the action. For attack help, the helper must be within 5ft of the target. The advantage applies to the next qualifying roll only, then expires. Tracked as a temporary effect: `{condition: "helped", source_combatant_id: [helper], target_combatant_id: [enemy], duration: "next_roll"}`.

**Escape:** `/action escape` allows a grappled (or creature-restrained) character to break free. Costs the action (`action_used = true`). System runs a contested check: the escaping character's Athletics (STR) or Acrobatics (DEX) vs the grappler's Athletics (STR). By default the system uses whichever of the character's two modifiers is higher; use `--athletics` or `--acrobatics` to override. On success, the grappled condition is removed and speed is restored. On failure, the character remains grappled and the action is spent. Rejected if the character is not currently grappled or restrained by a creature.

**Stand:** `/action stand` allows a prone character to stand up. Costs half the character's maximum movement speed (deducted from remaining movement for the turn), but does not cost the action. If the character has insufficient remaining movement, the command is rejected: "❌ Not enough movement to stand — requires 15ft, 10ft remaining." Standing removes the prone condition. If the character is not prone, the command is rejected: "❌ You are not prone." Standing can also happen implicitly when a prone character issues `/move` (existing behavior), but `/action stand` allows standing without moving.

**Drop Prone:** `/action drop-prone` causes the character to voluntarily drop prone. Costs no action and no movement (per 5e RAW, dropping prone uses zero movement). Applies the prone condition. If the character is already prone, the command is rejected: "❌ You are already prone." Useful tactically to impose disadvantage on incoming ranged attacks beyond 5ft.

**Hide:** `/action hide` is described in the Stealth & Hiding section.

**Action Surge (Fighter):** `/action surge` grants an additional action on the current turn. Available to Fighters at level 2+ (1 use per short rest; 2 uses at level 17+). The command deducts one use from `feature_uses["action-surge"]` and resets `action_used = false` and `attacks_remaining` to the character's full attacks per action. Sets `action_surged = true` on the `turns` table (prevents using Action Surge twice in the same turn). The surge action can be used for any standard action: Attack (with full Extra Attack sequence), Cast a Spell, Dash, Disengage, Dodge, Help, Hide, or Use an Object. Action Surge does not grant an additional bonus action or reaction. Recharges on short rest (already handled by `recharge: "short"` in `feature_uses`). Rejected if: no uses remaining ("❌ No Action Surge uses remaining"), already surged this turn ("❌ Already used Action Surge this turn"), or character is not a Fighter ("❌ Action Surge is a Fighter feature").

Combat log output:
```
⚡  Aria uses Action Surge! (1 use remaining)
📋 Remaining: 🏃 15ft move | ⚔️ 2 attacks | 🎁 Bonus action | 🤚 Free interact
```

**`/help action` output** (ephemeral, shown when a player types `/help action`):
```
/action — Actions on Your Turn

Standard actions (auto-resolved):
  /action disengage       Move without provoking opportunity attacks
  /action dash            Double your movement this turn
  /action dodge           Attacks against you have disadvantage until next turn
  /action help [ally] [target]  Give an ally advantage on their next attack/check
  /action hide            Stealth vs passive Perception (must have cover/obscurement)
  /action escape          Break free from a grapple (contested check)
  /action stand           Stand up from prone (costs half your movement)
  /action drop-prone      Drop prone voluntarily (no cost)
  /action ready [trigger] Hold your action for a trigger (uses your reaction)
  /action surge           Extra action this turn (Fighter only)
  /action channel-divinity [option]  Channel Divinity (Cleric/Paladin)

Freeform actions (DM-resolved):
  /action [anything]      Describe a creative action — sent to DM for resolution
                          Examples: /action flip the table for cover
                                   /action grab the chandelier and swing to F2

Cancel:
  /action cancel          Withdraw your pending freeform action (before DM resolves it)

Tips:
• Standard actions cost your action for the turn (except stand/drop-prone)
• Freeform actions also cost your action — the DM decides the outcome
• Use /undo if you need to correct an already-resolved action
```

### Free Object Interaction

Each creature gets one free object interaction per turn — drawing or sheathing a weapon, opening a door, picking up a dropped item, etc. Tracked via `turns.free_interact_used`.

**Enforcement:**
- First `/interact` per turn: free (sets `free_interact_used = true`, does not cost the action)
- Second `/interact` per turn: costs the action (`action_used = true`). If the action is already spent, the command is rejected: "Free interaction already used and action is spent."
- Simple interactions that the DM has pre-flagged as auto-resolvable (draw/sheathe weapon, open unlocked door) are resolved immediately. All others route to `#dm-queue` for DM adjudication.

---

## Conditions & Combat Mechanics

Conditions and combat modifiers are auto-applied by the backend whenever a relevant command is processed. All auto-applied effects are logged to `#combat-log`.

### Condition Effects

**Effects on saving throws:**

| Condition | Effect |
|-----------|--------|
| Paralyzed | Auto-fail STR and DEX saves |
| Stunned | Auto-fail STR and DEX saves |
| Unconscious | Auto-fail STR and DEX saves |
| Petrified | Auto-fail STR and DEX saves |
| Restrained | Disadvantage on DEX saves |
| Dodge action | Advantage on DEX saves |

When a save is triggered (spell, concentration check, etc.) and the target has one of these conditions, the system auto-resolves the save as failed (or applies disadvantage/advantage) without prompting the player to roll. Dodge action grants advantage on DEX saves, making the auto-skip Dodge action (for AFK/timeout) mechanically meaningful.

**Effects on ability checks:**

| Condition | Effect |
|-----------|--------|
| Frightened | Disadvantage on ability checks while source of fear is within line of sight |
| Poisoned | Disadvantage on ability checks |
| Blinded | Auto-fail ability checks requiring sight |
| Deafened | Auto-fail ability checks requiring hearing |

Auto-applied when `/check` is used. For blinded creatures, checks that require sight (e.g., Perception checks to spot something visual) are auto-failed. For deafened creatures, checks that require hearing (e.g., Perception checks to hear something) are auto-failed. The DM flags which checks require sight or hearing via the dashboard.

**Effects on attack rolls:**

| Condition | Effect on attacker | Effect on attacks against |
|-----------|-------------------|--------------------------|
| Blinded | Disadvantage on attack rolls | Advantage on attacks against |
| Frightened | No direct attack effect | — |
| Invisible | Advantage on attack rolls | Disadvantage on attacks against |
| Poisoned | Disadvantage on attack rolls | — |
| Prone | Disadvantage on attack rolls | Advantage (within 5ft) / Disadvantage (beyond 5ft) |
| Restrained | Disadvantage on attack rolls | Advantage on attacks against |
| Stunned | — | Advantage on attacks against |
| Paralyzed | — | Advantage on attacks against; auto-crit within 5ft |
| Unconscious | — | Advantage on attacks against; auto-crit within 5ft |
| Petrified | — | Advantage on attacks against |

Auto-applied when `/attack` is processed. The system checks conditions on both the attacker and the target. Multiple sources of advantage or disadvantage cancel per 5e rules (see Attack Mechanics).

**Effects on speed:**

| Condition | Effect |
|-----------|--------|
| Grappled | Speed becomes 0 |
| Restrained | Speed becomes 0 |
| Prone | Standing costs half movement speed |
| Frightened | Can't move closer to source of fear |

When a grappled or restrained condition is applied, the combatant's effective speed is set to 0. `/move` commands are rejected with an explanation (e.g., "You can't move — you are grappled"). Speed restores when the condition is removed. For frightened creatures, `/move` commands that would decrease distance to the fear source are rejected. The fear source is tracked as metadata on the frightened condition (`{source_combatant_id}`).

Standing from prone: a prone combatant can stand explicitly via `/action stand` (costs half their max speed from remaining movement, no action cost) or by choosing to stand when issuing `/move` (see below). If insufficient movement remains for `/action stand`, the command is rejected. Dropping prone voluntarily is done via `/action drop-prone` (no movement or action cost).

**Moving while prone:** when a prone combatant uses `/move`, the system prompts with Discord buttons before the standard movement confirmation:

```
You are prone. How do you want to move? [🧍 Stand & Move] [🐛 Crawl]
```

- **Stand & Move:** half max speed is deducted for standing, then the path is calculated at normal cost from remaining movement. If insufficient movement remains after standing, standing is still allowed but no further movement is possible.
- **Crawl:** movement costs double while staying prone (e.g., 10ft of crawling costs 20ft of movement). The character remains prone after moving. Stacks with difficult terrain for ×3 total cost.

The subsequent `/move` confirmation prompt reflects the chosen mode:
```
🏃 Stand & move to D4 — 15ft stand + 10ft move, 5ft remaining after. [✅ Confirm] [❌ Cancel]
🐛 Crawl to D4 — 20ft (10ft × 2 crawling), 10ft remaining after. [✅ Confirm] [❌ Cancel]
```

If the character has already stood this turn (via `/action stand` or a previous Stand & Move), subsequent `/move` commands skip the prompt and move at normal cost.

**Action blocking:**

| Condition | Effect |
|-----------|--------|
| Incapacitated | Can't take actions or reactions |
| Stunned | Can't take actions or reactions (includes incapacitated) |
| Paralyzed | Can't take actions or reactions (includes incapacitated) |
| Unconscious | Can't take actions or reactions (includes incapacitated) |
| Petrified | Can't take actions or reactions (includes incapacitated) |

When a combatant with one of these conditions attempts `/attack`, `/cast`, `/action`, or `/reaction`, the command is rejected (e.g., "You can't act — you are stunned"). The combatant's turn is auto-skipped with a `#combat-log` message. This generalizes the unconscious-at-0-HP behavior to all incapacitating conditions.

**Charmed attack restriction:**

Charmed creatures can't attack the charmer or target them with harmful abilities. `/attack` and `/cast` (harmful) commands targeting the charm source are rejected. The charm source is tracked as metadata on the charmed condition (`{source_combatant_id}`). Non-harmful interactions with the charmer are still allowed.

**Invisible condition:**

The Invisible condition is a trackable condition in `conditions_ref` (id: `"invisible"`), distinct from the `is_visible` stealth flag:
- **Invisible condition** (from spells like Invisibility, Greater Invisibility, Gloom Stalker's Umbral Sight): creature is magically unseen. Grants advantage on own attack rolls, disadvantage on attacks against. Spells requiring the caster to "see the target" cannot target an Invisible creature. Area-of-effect spells still affect them (no targeting required). Enemies may still know the creature's location from noise or tracks — the creature is unseen, not undetectable.
- **`is_visible = false`** (from `/action hide`): creature is hidden via Stealth. Position is concealed from the map. Broken by attacking, making noise, or being detected.
- **Interaction:** a creature can be Invisible but not hidden (enemies hear it and know its square, but can't see it — attacks still have disadvantage). A creature can be hidden but not Invisible (mundane stealth). Both can be active simultaneously (Invisible + Hide = unseen and unlocated).
- **Breaking Invisibility:** the Invisibility spell (non-Greater) ends when the creature attacks or casts a spell. The system auto-removes the Invisible condition after `/attack` or `/cast` if the source spell is "Invisibility" (tracked via `source_combatant_id` and source spell metadata on the condition). Greater Invisibility does not break on attack — it only expires via duration or concentration loss.

### Duration Tracking & Auto-Expiration

Conditions and spell effects with limited durations are tracked via `duration_rounds` and `started_round` on each condition entry. At specific trigger points, the system checks for expired effects:

**Expiration check timing:**
- **Start of source creature's turn:** compare `encounters.round_number` against `started_round + duration_rounds`. If expired, auto-remove the condition and post to `#combat-log`: "⏱️ [Effect] on [Target] has expired (placed by [Source])."
- **End of source creature's turn:** same check, for effects with `expires_on: "end_of_turn"`.

**Duration tracking fields** (on each condition in `combatants.conditions` JSONB):
- `duration_rounds` — NULL for indefinite (e.g., grappled until escape); integer for timed effects
- `started_round` — the `encounters.round_number` when applied
- `source_combatant_id` — who applied it (determines whose turn triggers expiration)
- `expires_on` — `"start_of_turn"` (default) or `"end_of_turn"`

**Turn-start sequence** (updated):
1. Check for expired conditions/spell effects on this combatant → auto-remove and log
2. Apply start-of-turn effects (e.g., ongoing damage zones)
3. Ping player with available resources

**Indefinite conditions** (grappled, prone, charmed without duration) have `duration_rounds: NULL` and are only removed by specific actions (escape, standing up, spell ending).

### Damage Processing

**Resistance, immunity, and vulnerability:**

When damage is dealt, the system checks the target's `damage_resistances`, `damage_immunities`, and `damage_vulnerabilities` arrays:
- **Resistance**: damage of that type is halved (rounded down)
- **Immunity**: damage of that type is reduced to 0
- **Vulnerability**: damage of that type is doubled
- **Resistance + Vulnerability:** if a creature has both resistance and vulnerability to the same damage type, they cancel out — normal damage is dealt
- **Immunity precedence:** immunity always takes priority over resistance and vulnerability — damage is reduced to 0 regardless of other modifiers

Petrified creatures have resistance to all damage types, auto-applied in addition to existing resistances (no double-stacking).

**Temporary hit points:**

Temp HP acts as a damage buffer and follows these rules:
- Damage is absorbed by `temp_hp` before `hp_current` — system deducts temp HP first, then applies remainder to real HP
- Temp HP **does not stack** — if a new source grants temp HP, the creature keeps the higher value (current remaining or new grant)
- Temp HP **cannot be healed** — healing spells and effects only restore `hp_current`, never `temp_hp`
- Temp HP expires at the end of the duration specified by the granting effect (tracked via the condition/effect system), or on long rest if no duration specified

**Exhaustion (progressive condition):**

| Level | Effect |
|-------|--------|
| 1 | Disadvantage on ability checks |
| 2 | Speed halved |
| 3 | Disadvantage on attack rolls and saving throws |
| 4 | Hit point maximum halved |
| 5 | Speed reduced to 0 |
| 6 | Death |

Effects are cumulative. Exhaustion level is tracked as an integer (0–6) on the combatant. Each level's effects auto-apply:
- Levels 1, 3: system applies disadvantage to relevant rolls
- Level 2, 5: system modifies effective speed
- Level 4: system reduces max HP (and current HP if above new max)
- Level 6: character dies immediately

Exhaustion increases from forced march, starvation, environmental effects — applied by DM from dashboard. Decreases by 1 level per long rest (with food/water).

**Condition immunity:**

When a condition is being applied to a creature, the system checks `condition_immunities`. If immune, the condition is not applied and a message is posted to `#combat-log`.

### Cover

Cover is computed dynamically from map geometry — walls, obstacles, and other creatures in `tiled_json` between attacker and target. Cover is not stored on combatants; it is calculated at the moment of each attack or spell.

| Cover Type | AC Bonus | DEX Save Bonus | Determination |
|------------|----------|----------------|---------------|
| Half cover | +2 | +2 | Target is behind an obstacle that covers at least half their body (low wall, furniture, another creature) |
| Three-quarters cover | +5 | +5 | Target is behind an obstacle covering at least three-quarters of their body (portcullis, arrow slit) |
| Full cover | — | — | Target is completely concealed — cannot be targeted by attacks or spells requiring line of sight |

**Calculation (DMG grid variant):** the system picks a corner of the attacker's square and draws lines to each corner of the target's square. If 1–2 lines are blocked by obstacles, half cover applies. If 3 lines are blocked, three-quarters cover. If all 4 are blocked, full cover. The system tests from the corner that gives the attacker the best (least) cover result. Creature-granted cover (another creature between attacker and target) counts as half cover.

**Integration with attacks:** cover AC bonus is added to the target's effective AC during attack resolution. If full cover applies, `/attack` and targeted `/cast` commands are rejected: "Target has full cover — no line of sight."

**Integration with saves:** cover DEX save bonus applies to area-of-effect spells (e.g., Fireball). The system checks cover between the spell's point of origin and each affected creature.

### Difficult Terrain

Difficult terrain (rubble, undergrowth, mud, shallow water, etc.) doubles movement cost — each foot of movement in difficult terrain costs 2 feet of speed. Defined per tile in the map's `tiled_json` data using a terrain property (`difficult: true`).

**Enforcement:** on each `/move`, the system calculates the path cost including difficult terrain multipliers. If the character lacks sufficient remaining movement, the command is rejected with the shortfall: "Not enough movement — path requires 40ft (includes difficult terrain), 25ft remaining."

**Interactions:**
- Stacks with prone stand-up cost (half speed deducted first, then difficult terrain doubles remaining path cost)
- Dash action grants extra movement that is also subject to difficult terrain costs
- Difficult terrain does not affect flying creatures above ground level (altitude > 0) unless the difficult terrain is airborne (e.g., Wind Wall)
- Crawling (moving while prone without standing) also costs double, stacking with difficult terrain for 3x total cost

### Opportunity Attacks

When a creature moves out of a hostile creature's melee reach without taking the Disengage action, the system auto-detects the trigger and prompts the hostile for an opportunity attack.

**Detection logic:**
1. On each `/move` command, the system checks whether the moving creature leaves any hostile creature's threatened area
2. Threatened area = `reach_ft` from the hostile's position (default 5ft; 10ft for reach weapons)
3. If the moving creature used `/action disengage` (costs their action) or `/bonus cunning-action disengage` (Rogue Cunning Action), no opportunity attacks are triggered — the system tracks `has_disengaged` on the `turns` table

**Resolution (queue-and-continue):**
Movement is **not** paused when an OA triggers. The moving creature completes its full `/move`, and the system records the OA trigger point (the tile where the creature left the hostile's reach). The hostile then has until end-of-round to respond:
- For **player-controlled hostiles:** the bot pings the player in `#your-turn`: "⚔️ [Enemy] moved out of your reach (left [tile]) — use your reaction for an opportunity attack? `/reaction oa [target]`"
- For **DM-controlled hostiles:** the dashboard surfaces the opportunity attack option; DM confirms or declines
- Opportunity attacks use the hostile's reaction for the round (`reaction_used = true`)
- If the OA hits and reduces the target to 0 HP, the system notifies the DM that the creature should have dropped at the trigger point. The DM retroactively corrects the creature's position and any subsequent effects via the dashboard. The system does not auto-invalidate movement.
- If the hostile does not respond by end-of-round, the OA opportunity is forfeited

**Interaction with reactions:** opportunity attacks are system-generated reaction triggers — they consume the creature's reaction and follow the same one-per-round limit.

### Grapple & Shove

Grapple and shove are contested ability check actions available via `/action grapple [target]` and `/shove [target]`.

**Grapple:**
- Requires a free hand (system checks equipped items — rejected if both hands occupied)
- Target must be no more than one size category larger than the grappler
- Contested check: attacker Athletics (STR) vs target's choice of Athletics (STR) or Acrobatics (DEX)
- Success: target gains the "grappled" condition (`{condition: "grappled", source_combatant_id: [grappler]}`)
- Grappled creature's speed becomes 0; grappler can drag the target (see Dragging below)
- Escape: target uses `/action escape` (costs their action) to repeat the contested check; success removes the grappled condition. See Standard Actions below

**Dragging:**

When a grappler uses `/move`, the system detects grappled targets and adds a drag confirmation step before the standard movement confirmation. The prompt lists all creatures grappled by the mover:

```
🤼 You are grappling: Goblin #1, Orc Shaman. Drag them with you? [✅ Drag] [❌ Release & Move]
```

- **Drag:** movement costs double (half speed). The standard `/move` confirmation follows with adjusted cost: `🏃 Move to D4 — 20ft (10ft × 2 dragging), 10ft remaining after. [✅ Confirm] [❌ Cancel]`. All grappled creatures move to the grappler's destination tile.
- **Release & Move:** grapple condition is removed from all listed targets before the move. Grappler moves at normal speed.

If the doubled movement cost exceeds remaining speed, the Drag option is still shown but the subsequent `/move` confirmation will reflect the shortfall and the player can Cancel or pick a closer destination. Multiple grappled creatures do not further multiply cost — dragging always costs ×2 regardless of how many targets are held.

**Shove:**
- Target must be no more than one size category larger
- Contested check: attacker Athletics (STR) vs target's choice of Athletics (STR) or Acrobatics (DEX)
- Attacker chooses mode via flag: `/shove OS --prone` or `/shove OS --push`
  - `--prone`: target is knocked prone
  - `--push`: target is pushed 5ft away from the attacker (system validates destination is unoccupied)

### Stealth & Hiding

The Hide action allows a creature to become unseen during combat.

**Hiding:**
- Player uses their action: `/action hide`
- Rogues can Hide as a bonus action via `/bonus cunning-action hide` (costs bonus action instead of action). Requires Rogue class (level 2+, when Cunning Action is gained)
- System rolls Stealth (DEX) check for the hider vs **passive Perception** of each hostile creature with line of sight
- If the Stealth result beats all hostiles' passive Perception: `is_visible` is set to `false` on the combatant; token is hidden from the player-facing map
- If any hostile's passive Perception meets or exceeds the roll: hide fails, `is_visible` remains `true`

**While hidden:**
- Attacks against the hidden creature have disadvantage (attacker can't see target)
- The hidden creature's first attack has advantage (unseen attacker)
- After attacking (hit or miss), the creature is automatically revealed: `is_visible = true`
- Making noise (casting a spell with verbal components, etc.) may reveal the creature — DM adjudicates via dashboard

**Passive Perception:** calculated as `10 + Perception modifier` (including proficiency if proficient). Stored on the combatant for quick lookup.

### Equipment Enforcement

The system enforces armor-related penalties automatically during combat.

**Heavy armor strength requirement:**
- If a character's STR score is below their equipped armor's `strength_req`, their effective speed is reduced by 10ft
- Checked when combat begins and when equipment changes
- Example: Plate armor requires STR 15; a character with STR 13 moves at `speed_ft - 10`

**Stealth disadvantage:**
- Armor with `stealth_disadv: true` (e.g., chain mail, plate) automatically applies disadvantage to Stealth checks
- Applied when the character uses `/action hide` or rolls `/check stealth`

**AC calculation:**
- **Standard (armor-based):** `ac_formula` is NULL. AC is computed from `equipped_armor` using the armor table (`ac_base` + DEX modifier, capped by `ac_dex_max` for medium armor, ignored for heavy armor) + 2 if a shield is in `equipped_off_hand`. The `ac` field is updated when equipment changes.
- **Unarmored Defense / Natural Armor:** `ac_formula` is set (e.g., `"10 + DEX + WIS"` for Monk, `"10 + DEX + CON"` for Barbarian, `"13 + DEX"` for Lizardfolk natural armor). The system evaluates the formula against current `ability_scores` and updates the `ac` field whenever ability scores change (ASI, magic item, effect). If the character equips armor, standard armor AC is used instead — Unarmored Defense only applies when no armor is worn (shield is allowed). The system takes the higher of formula AC and armor AC if armor is equipped, matching 5e rules.
- **`modify_ac` effects** (Shield of Faith, Shield spell, etc.) are applied on top of the base AC at resolution time, not baked into the cached `ac` value.
- Stacks with other sources of disadvantage per standard 5e rules (multiple disadvantage sources still result in a single disadvantage)

---

## Feature Effect System

Class features, racial traits, creature abilities, and spell effects share a common data-driven resolution system. Rather than hardcoding each feature individually, features declare their mechanical effects using a vocabulary of **effect types** and **trigger points**. The combat engine processes these declarations at the appropriate moments.

### Effect Types

Each feature declares one or more effects from this vocabulary:

| Effect Type | Description | Example |
|-------------|-------------|---------|
| `modify_attack_roll` | Add/subtract from attack rolls | Archery fighting style (+2 ranged) |
| `modify_damage_roll` | Add/subtract flat damage | Rage damage bonus (+2/+3/+4) |
| `extra_damage_dice` | Add extra dice to damage | Sneak Attack (1d6–10d6), Divine Smite (2d8) |
| `modify_ac` | Add/subtract from AC | Shield of Faith (+2), Unarmored Defense |
| `modify_save` | Add/subtract from saving throws | Aura of Protection (CHA mod to saves) |
| `modify_check` | Add/subtract from ability checks | Guidance (+1d4), Jack of All Trades |
| `modify_speed` | Add/subtract from speed | Barbarian Fast Movement (+10ft) |
| `grant_resistance` | Grant damage type resistance | Rage (resistance to B/P/S), Tiefling (fire) |
| `grant_immunity` | Grant condition or damage immunity | Paladin Aura of Courage (immunity to frightened) |
| `extra_attack` | Grant additional attacks per action | Extra Attack, Thirsting Blade |
| `modify_hp` | Modify HP maximum or temporary HP | Tough feat (+2 per level), Heroism (temp HP) |
| `conditional_advantage` | Grant advantage/disadvantage on rolls | Reckless Attack (advantage on STR melee) |
| `resource_on_hit` | Trigger resource use on hit/damage | Battlemaster maneuvers, Divine Smite on hit |
| `reaction_trigger` | Define automatic reaction prompts | Shield spell, Uncanny Dodge, Sentinel |
| `aura` | Apply effects to creatures within radius | Paladin auras, Spirit Guardians |

### Trigger Points

Effects are evaluated at specific moments during combat resolution:

| Trigger | When Evaluated | Examples |
|---------|---------------|----------|
| `on_attack_roll` | After rolling d20, before comparing to AC | Archery, conditional advantage |
| `on_damage_roll` | After hit confirmed, calculating damage | Rage bonus, Sneak Attack |
| `on_take_damage` | When receiving damage, before applying | Resistance, Uncanny Dodge |
| `on_save` | When making a saving throw | Aura of Protection, Evasion |
| `on_check` | When making an ability check | Jack of All Trades, Expertise |
| `on_turn_start` | At the start of the creature's turn | Regeneration, ongoing damage |
| `on_turn_end` | At the end of the creature's turn | End-of-turn saves, aura damage |
| `on_rest` | When completing a short or long rest | Feature use recharge |

### Condition Filters

Each effect can specify conditions that must be true for it to apply:

```json
{
  "effect_type": "modify_damage_roll",
  "modifier": 2,
  "conditions": {
    "when_raging": true,
    "attack_type": "melee",
    "ability_used": "str"
  }
}
```

Common condition filters:
- `when_raging`, `when_concentrating` — active state checks
- `weapon_property: "heavy"` / `"finesse"` / `"ranged"` — weapon type constraints
- `attack_type: "melee"` / `"ranged"` — attack category (includes both weapon and spell attacks of that type)
- `ability_used: "str"` / `"dex"` — ability score used for the roll
- `target_condition: "frightened"` — target must have a specific condition
- `ally_within: 5` — an ally within N feet of the target (for Sneak Attack, Pack Tactics)
- `uses_remaining: true` — feature has remaining uses (checked against `feature_uses`)

### Resolution Priority

When multiple effects apply to the same roll, they are resolved in this order:

1. **Immunities** — condition immunities prevent effects from applying
2. **Resistances / Vulnerabilities** — damage type modifications (see Damage Processing for cancel/immunity rules)
3. **Flat modifiers** — summed (+2 from Archery, +CHA from Aura of Protection)
4. **Dice modifiers** — additional dice added (Sneak Attack, Divine Smite)
5. **Advantage / Disadvantage** — cancel out if both apply (see Attack Mechanics for details)

### Example Declarations

**Rage (Barbarian):**
```json
{
  "feature": "Rage",
  "effects": [
    {"type": "modify_damage_roll", "modifier": 2, "trigger": "on_damage_roll",
     "conditions": {"when_raging": true, "attack_type": "melee", "ability_used": "str"}},
    {"type": "grant_resistance", "damage_types": ["bludgeoning", "piercing", "slashing"],
     "trigger": "on_take_damage", "conditions": {"when_raging": true}},
    {"type": "conditional_advantage", "on": "str_check", "trigger": "on_check",
     "conditions": {"when_raging": true, "ability_used": "str"}}
  ]
}
```

**Sneak Attack (Rogue):**
```json
{
  "feature": "Sneak Attack",
  "effects": [
    {"type": "extra_damage_dice", "dice": "level_scaled", "trigger": "on_damage_roll",
     "conditions": {"weapon_property": "finesse_or_ranged", "once_per_turn": true,
                    "any_of": [{"has_advantage": true}, {"ally_within": 5}]}}
  ]
}
```

**Aura of Protection (Paladin 6+):**
```json
{
  "feature": "Aura of Protection",
  "effects": [
    {"type": "modify_save", "modifier": "cha_mod", "trigger": "on_save",
     "conditions": {"aura_radius": 10, "target": "self_and_allies"}}
  ]
}
```

**Bless (Spell):**
```json
{
  "feature": "Bless",
  "effects": [
    {"type": "modify_attack_roll", "modifier": "1d4", "trigger": "on_attack_roll"},
    {"type": "modify_save", "modifier": "1d4", "trigger": "on_save"}
  ]
}
```

### Integration with Existing Systems

- **`conditions_ref.mechanical_effects`** uses the same effect type vocabulary — condition effects (e.g., blinded → `disadvantage_on_attack`) are processed alongside feature effects
- **`classes.features_by_level`** and **`characters.features`** store effect declarations in the `mechanical_effect` field using this format
- **Auto-Detected Class & Creature Features** (Sneak Attack, Rage, Evasion, Pack Tactics) are specific instances of this general system — their logic is driven by their effect declarations, not hardcoded
- The combat engine processes all applicable effects at each trigger point using a **single-pass processor**: collect all active effects for the trigger → filter by conditions → apply in priority order → return modified result

---

## Combat Turn Flow

### Overview

```
DM clicks "Start Combat" in dashboard
  → backend initializes combat state, rolls initiative
  → bot posts initiative order to #initiative-tracker
  → bot posts map image to #combat-map
  → bot pings first player/enemy in #your-turn

Player logs in, sees ping, submits commands
  → bot validates and resolves mechanical actions
  → results posted to #combat-log, rolls to #roll-history
  → freeform /action posts to #dm-queue if needed

DM reviews, resolves any queued actions in dashboard
  → clicks "Apply", "Next Turn"
  → bot regenerates map image in #combat-map
  → bot pings next player in #your-turn
```

### Surprise

When combat begins from an ambush or when some combatants are unaware, the DM marks specific combatants as **surprised** during encounter setup in the dashboard.

**Implementation:**
- Surprise is applied as a condition: `{condition: "surprised", duration_rounds: 1, started_round: 0}`
- Surprised combatants cannot move, take actions, use bonus actions, or use reactions during round 1
- The system auto-skips surprised combatants' turns in round 1 (no Dodge action — they are fully inert)
- A surprised creature can use reactions after its turn ends in round 1 (the "surprised" condition is removed at the end of their skipped turn)

**Combat log output:**
```
⏭️ Goblin #2 is surprised — turn skipped
```

**Interaction with initiative:** surprised creatures still roll initiative normally. Their position in the initiative order matters because it determines when during round 1 they stop being surprised (and can start using reactions).

### Initiative Tiebreaking

When two or more combatants roll the same initiative total, ties are broken automatically:

1. **Higher DEX modifier** goes first
2. If DEX modifiers are also tied, **alphabetical by display name**

The system assigns `initiative_order` deterministically during encounter setup — no DM input or roll-off needed.

### Player Turns

Turns are **sequential** — players send commands one at a time and see results before deciding their next action.

**Turn resources tracked by the backend:**
- Movement (feet remaining out of speed)
- Action (used / not used)
- Bonus action (used / not used)
- Free object interaction (used / not used)
- Attacks remaining (by class/level)
- Reaction (used / not used, per round)

Each command validates against remaining resources. If a player tries to use something already spent, the bot replies with a specific error.

**Turn status prompt:** the bot shows available resources at two points:

1. **Turn start** — included in the ping message in `#your-turn`:
```
🔔 @Aria — it's your turn!
📋 Available: 🏃 30ft move | ⚔️ 2 attacks | 🎁 Bonus action | 🤚 Free interact | 🛡️ Reaction | ⚡ Action Surge (1)
```

2. **After every command** — appended to the command's response in `#combat-log`:
```
⚔️  Aria attacks Goblin #1 with Longsword (attack 1 of 2)
    → Roll to hit: 19 (14 + 5) — HIT
    → Damage: 9 slashing
    → Goblin #1 is now Bloodied

📋 Remaining: 🏃 5ft move | ⚔️ 1 attack | 🎁 Bonus action | 🤚 Free interact | 🎵 Bardic Inspiration (d8)
```

Spent resources are omitted from the list. For Fighters with Action Surge available, `⚡ Action Surge (N)` is appended (where N = remaining uses). If the character holds Bardic Inspiration, `🎵 Bardic Inspiration (die)` is appended. When nothing remains (and no Action Surge or Bardic Inspiration available), the prompt shows: "📋 All actions spent — type `/done` to end your turn."

### Combat Log Output Reference

Every auto-resolved action, auto-detected modifier, and auto-rejected command posts a clear message to `#combat-log` so all players can follow the action — not just the active player.

**Standard actions:**
```
💨  Aria takes the Disengage action
🏃  Aria takes the Dash action (+30ft movement this turn)
🛡️  Aria takes the Dodge action (attacks against her have disadvantage until next turn)
🤝  Aria takes the Help action — granting Thorn advantage on next attack against Goblin #1
🙈  Aria attempts to Hide — 🎲 Stealth: 17 — Hidden from all hostiles
🙈  Aria attempts to Hide — 🎲 Stealth: 8 — Failed (spotted by Orc Shaman)
💪  Aria attempts to escape Goblin #1's grapple — 🎲 Acrobatics: 14 vs Athletics: 11 — Escaped!
💪  Aria attempts to escape Goblin #1's grapple — 🎲 Athletics: 9 vs Athletics: 15 — Failed
⏳  Aria readies an action: "I attack when the goblin moves past me"
```

**Grapple and shove:**
```
🤼  Aria grapples Goblin #1 — 🎲 Athletics: 16 vs Acrobatics: 12 — Grappled!
🤼  Aria attempts to grapple Orc Shaman — 🎲 Athletics: 10 vs Athletics: 18 — Failed
📌  Aria shoves Goblin #1 (push) — 🎲 Athletics: 15 vs Athletics: 10 — Pushed to E4
📌  Aria shoves Goblin #1 (prone) — 🎲 Athletics: 15 vs Acrobatics: 12 — Knocked prone!
📌  Aria attempts to shove Orc Shaman (push) — 🎲 Athletics: 8 vs Athletics: 14 — Failed
🤼  Aria drags Goblin #1 to D4
🤼  Aria releases Goblin #1 (grapple ended)
```

**Auto-detected advantage/disadvantage on attacks** (appended to the attack roll line):
```
⚔️  Aria attacks Goblin #1 with Longbow (disadvantage — hostile within 5ft)
    → Roll to hit: 8 / 14 (lower: 8 + 5 = 13) — MISS
⚔️  Aria attacks Goblin #1 with Longsword (advantage — target prone within 5ft)
    → Roll to hit: 7 / 18 (higher: 18 + 5 = 23) — HIT
⚔️  Aria attacks Goblin #1 with Greatsword (disadvantage — Heavy weapon, Small creature)
    → Roll to hit: 15 / 4 (lower: 4 + 5 = 9) — MISS
⚔️  Aria attacks Goblin #1 with Longsword (advantage — target blinded)
    → Roll to hit: 11 / 16 (higher: 16 + 5 = 21) — HIT
⚔️  Aria attacks Goblin #1 with Longbow (disadvantage — target prone, beyond 5ft)
    → Roll to hit: 17 / 6 (lower: 6 + 5 = 11) — MISS
⚔️  Aria attacks Goblin #1 with Longsword (advantage + disadvantage cancel — normal roll)
    → Roll to hit: 14 (14 + 5 = 19) — HIT
```

**Auto-detected critical hits:**
```
⚔️  Aria attacks Goblin #1 with Longsword
    → Roll to hit: 🎯 NAT 20 — CRITICAL HIT!
    → Damage: 18 slashing (doubled dice: 2d8 + 5)
⚔️  Aria attacks Goblin #1 with Longsword (auto-crit — target paralyzed within 5ft)
    → Damage: 18 slashing (doubled dice: 2d8 + 5)
```

**Divine Smite (on-hit prompt result):**
```
⚔️  Aria attacks Goblin #1 with Longsword
    → Roll to hit: 17 (12 + 5) — HIT
    → Damage: 8 slashing (1d8 + 5)
    ⚡ Divine Smite (2nd-level slot) — 3d8 radiant: 14
    → Total: 22 damage — Goblin #1 is now Bloodied

⚔️  Aria attacks Zombie #1 with Warhammer
    → Roll to hit: 🎯 NAT 20 — CRITICAL HIT!
    → Damage: 19 bludgeoning (doubled dice: 2d8 + 4)
    ⚡ Divine Smite (1st-level slot, crit) — 4d8 radiant (doubled) +2d8 vs undead: 28
    → Total: 47 damage — Zombie #1 is destroyed!
```

**Auto-detected saving throw modifiers:**
```
🎲  Aria rolls CON save — 15 (12 + 3) — DC 10 — Concentration maintained
🎲  Aria rolls DEX save — auto-fail (paralyzed)
🎲  Aria rolls STR save — auto-fail (stunned)
🎲  Aria rolls DEX save (advantage — Dodge active) — 8 / 16 (higher: 16 + 2 = 18) — DC 15 — Success!
🎲  Aria rolls DEX save (disadvantage — restrained) — 14 / 6 (lower: 6 + 2 = 8) — DC 15 — Failure
```

**Condition application and removal:**
```
⚠️  Aria is now Grappled (by Goblin #1)
⚠️  Aria is now Prone
⚠️  Aria is now Frightened (source: Orc Shaman)
✅  Grappled removed from Aria (escaped)
⏱️  Dodge on Aria has expired (start of next turn)
```

**Dropping to 0 HP and death:**
```
💔  Aria drops to 0 HP — unconscious and dying (death saves begin next turn)
💀  Aria is killed outright! (26 overflow damage ≥ 24 max HP — instant death, no death saves)
💀  Aria fails 3 death saves — dead
🎲  Aria rolls death save — 14 — Success (2S / 1F)
🎲  Aria rolls death save — 🎯 NAT 20 — Aria regains 1 HP and is conscious! (tallies reset)
🎲  Aria rolls death save — 💥 NAT 1 — 2 failures! (1S / 3F) — dead
⚠️  Aria takes damage at 0 HP — 1 death save failure (1S / 2F)
⚠️  Aria takes a critical hit at 0 HP — 2 death save failures (1S / 3F) — dead
💚  Aria receives 7 HP of healing — conscious at 7 HP (death save tallies reset)
```

**Concentration:**
```
🔮  Aria loses concentration on Bless (incapacitated — stunned)
🔮  Aria loses concentration on Fog Cloud (failed CON save)
🔮  Aria drops concentration on Hold Person (cast new concentration spell: Bless)
```

**Auto-rejected commands** (shown only to the acting player, not in `#combat-log`):
```
❌  You can't move — you are grappled
❌  You can't act — you are stunned
❌  You can't attack Orc Shaman — you are charmed by them
❌  You can't move closer to Orc Shaman — you are frightened of them
❌  Target has full cover — no line of sight
❌  Target is out of melee range (10ft reach)
❌  Not enough movement — path requires 40ft (includes difficult terrain), 25ft remaining
❌  No arrows remaining
❌  You already cast a bonus action spell this turn — you can only cast a cantrip with your action
❌  You already cast a leveled spell with your action this turn — you cannot cast a bonus action spell
❌  You can't end your turn in another creature's space — use /move to leave Goblin #1's tile
❌  Free interaction already used and action is spent
❌  Can't use --twohanded — off-hand is not free
❌  You aren't grappled or restrained by a creature
```

**Turn auto-skip (incapacitated or AFK):**
```
⏭️  Aria's turn is auto-skipped (stunned — can't take actions)
⏭️  Aria's turn is auto-skipped (timed out — Dodge action applied)
💀  Aria auto-rolls death save (timed out) — 🎲 14 — Success (2 of 3)
```

**Ending a turn:**
- **Explicit:** player sends `/done`. If the player has unused resources (action, bonus action, or remaining attacks), the system shows an ephemeral confirmation prompt before ending the turn:
  ```
  ⚠️ You still have: ⚔️ 1 attack | 🎁 Bonus action. End turn? [✅ End Turn] [❌ Cancel]
  ```
  If all resources are spent, `/done` ends the turn immediately with no prompt.
- **DM override:** DM can end any turn from the dashboard
- **Timeout:** remaining unused actions are forfeited (see Turn Timeout below)

### Enemy / NPC Turns

Each enemy takes its own turn in initiative order. The DM resolves enemy turns through the dashboard with **smart defaults** — the system pre-fills suggestions, DM confirms or overrides.

**Dashboard flow:**
1. Dashboard highlights the active enemy and pre-fills:
   - **Suggested move:** shortest path toward nearest hostile (reuses pathfinding)
   - **Suggested attack:** nearest target in range; defaults to creature's primary attack from stat block
   - **Suggested ability:** if the creature has a special ability (e.g. Breath Weapon), suggest it when conditions are met (multiple targets in cone/line)
2. DM clicks **Confirm** to accept defaults, or overrides any field
3. DM rolls to-hit and damage from the dashboard; system applies HP changes
4. DM sees results and can adjust before posting (e.g. fudge a crit that would one-shot a level 2 player)
5. On confirm, results post to `#combat-log` and map updates

**`#combat-log` output** for enemy turns (same format as player actions):
```
🏃 Goblin 1 moves to D5
⚔️ Goblin 1 attacks Thorn — 🎲 14 vs AC 18 — Miss!
🏃 Goblin 2 moves to E4
⚔️ Goblin 2 attacks Kael — 🎲 19 vs AC 15 — Hit! 7 slashing damage
```

**Pending reactions:** if a player has a pre-declared `/reaction` that triggers during the enemy turn, the dashboard surfaces it to the DM before confirming that step. DM resolves the reaction inline.

### Summoned Creatures & Companions

Summoned creatures (familiars, animal companions, Spiritual Weapon, conjured creatures, animated undead, etc.) are **player-controlled** via the `/command` slash command. The summoning player issues actions on behalf of their creatures.

**Summoning flow:**
1. Player casts a summoning spell (e.g., `/cast find-familiar`, `/cast spiritual-weapon D5`, `/cast conjure-animals`)
2. System creates combatant entries for summoned creatures using reference stat blocks. Each gets a short ID (e.g., `FAM`, `SW`, `WF1`–`WF8` for wolves)
3. Summoned creatures appear on the initiative tracker and map

**Initiative placement:**
- Creatures with their own turns (familiars, conjured animals, animated dead) roll initiative and act on their own turn in the initiative order
- Spell effects that act on the caster's turn (Spiritual Weapon, Flaming Sphere) are commanded during the caster's turn using `/command`
- Companion creatures (Ranger's companion) act on the player's turn immediately after the player's actions

**`/command` syntax:**
```
/command [creature-id] [action] [target?] [flags?]
```

Examples:
```
/command FAM help Thorn G1     -- familiar uses Help action
/command SW attack G2           -- Spiritual Weapon attacks target
/command WF1 attack G3          -- wolf #1 attacks goblin
/command FAM move C5            -- familiar moves to C5
/command FAM done               -- end familiar's turn
```

**Validation:**
- `/command` only works for creatures the player summoned (`summoner_id` matches)
- Actions are validated against the creature's stat block (available attacks, movement speed, abilities)
- Turn resources (action, movement, bonus action) are tracked per creature, same as any combatant
- The summoning player must be the one issuing `/command` — other players cannot control another player's summons

**Turn flow for summoned creatures with their own initiative:**
1. When the summoned creature's turn comes up, the summoning player is pinged in `#your-turn`: "🔔 @PlayerName — your Wolf #1 (WF1)'s turn!"
2. Player issues `/command WF1 move ...`, `/command WF1 attack ...`, etc.
3. Player types `/command WF1 done` to end the creature's turn
4. Same timeout rules apply — if the player doesn't act for their summoned creature, the turn is skipped

**Dismissal and death:**
- Summoned creatures that drop to 0 HP are removed from the encounter (no death saves)
- Concentration-based summons (Conjure Animals, Animate Dead while active) are dismissed when concentration ends
- Player can dismiss voluntarily: `/command FAM dismiss`
- Dismissed/dead creatures are removed from initiative and map

**Combat log output:**
```
🐾  Aria's Owl (FAM) uses Help on Thorn targeting Goblin #1
⚔️  Kael's Spiritual Weapon (SW) attacks Goblin #2 — 🎲 17 vs AC 13 — Hit! 9 force damage
🐺  Aria's Wolf #1 (WF1) attacks Goblin #3 — 🎲 12 vs AC 13 — Miss!
💨  Aria dismisses Owl (FAM)
```

### Turn Timeout & AFK Handling

Turn timeout: **24 hours**, DM-configurable per campaign (1h–72h range).

**Escalation:**

- **50% reminder** — light nudge in `#your-turn`:
```
⏰ @Aria — it's still your turn! 12h remaining. Use /recap to catch up.
```

- **75% final warning** — context-rich tactical summary in `#your-turn`:
```
⚠️ @Aria — your turn will be skipped in 6 hours!
❤️ HP: 28/45 | 🛡️ AC: 16 | ⚠️ Conditions: Poisoned
📋 Available: 🏃 30ft move | ⚔️ 2 attacks | 🎁 Bonus action | 🤚 Free interact
🎯 Adjacent enemies: Goblin #1 (G1), Orc Shaman (OS)
💡 Check #combat-map for current positions.
```
The 75% warning includes: HP and AC, active conditions, remaining turn resources, adjacent enemies, and a pointer to `#combat-map`. This mirrors the turn-start prompt with added battlefield context so the player can act immediately without extra commands. If the character has concentration, Bardic Inspiration, or Action Surge, those are included as in the normal turn status prompt.

- **100% auto-skip** — player takes the **Dodge action with no movement**

**DM manual overrides (via dashboard):**
- **Skip now** — immediately advance past a player
- **Extend timer** — grant more time without changing the campaign default
- **Pause combat** — freeze all timers

**Prolonged absence:**
- After 3 consecutive auto-skips, the character is flagged as "absent" in the dashboard
- DM decides: auto-pilot the character, narrate a retreat, or remove from initiative
- Initiative slot stays reserved so the player can return seamlessly

### Combat Recap

In async play, combat can span days of real time. `#combat-log` serves as the persistent record of everything that happened, and `/recap` gives players a filtered view of it.

**`/recap`** (no arguments) — shows all `#combat-log` entries since the player's last turn as an ephemeral message. Covers every turn that occurred while the player was waiting: attacks, damage, movement, conditions applied/removed, deaths, spells cast, and reactions triggered.

**`/recap N`** — shows `#combat-log` entries from the last N rounds (e.g., `/recap 3` for the last 3 rounds).

The recap is a direct replay of combat log entries — no summarization or AI rewriting. Entries are grouped by round and turn for readability:

```
📜 Recap — Rounds 4–6 (since your last turn)

── Round 4 ──
Goblin #1 (G1): attacked Thorn — 🎲 11 vs AC 18 — Miss!
Thorn: attacked Goblin #1 with Greatsword — 🎲 19 — Hit! 14 slashing → Goblin #1 Bloodied
Aria: cast Fireball at E5 — Goblin #2 💀 killed | Goblin #3 🎲 DEX save 8 — fail, 28 fire
  ⚠️ Goblin #3 is now Dead

── Round 5 ──
Goblin #1 (G1): Disengaged, moved to H3
Thorn: moved to G3, attacked Goblin #1 — 🎲 22 — Hit! 11 slashing → Goblin #1 💀 killed

🏁 Combat ended — all hostiles defeated
```

Usable from any channel, during or after combat (until the encounter is archived). If no combat is active and the most recent encounter is completed, `/recap` shows the final rounds of that encounter.

### Death Saves & Unconsciousness

When a character drops to 0 HP, they fall **unconscious** and begin making death saving throws.

**Dropping to 0 HP:**
- Character status becomes **Dying** (unconscious, prone)
- Concentration is broken automatically
- All commands except `/deathsave` are blocked on their turn
- Token state changes to "dying" on the map (distinct visual)

**Instant death check:**
- If damage remaining after reaching 0 HP ≥ character's max HP → instant death
- Token state → "dead", no death saves

**Death saving throws:**
- When initiative reaches a dying player, bot pings them to send `/deathsave`
- System rolls d20 (no modifiers unless granted by a feature like Diamond Soul)
- ≥10 = success, <10 = failure
- **Nat 20** → regain 1 HP, conscious, still prone. Tallies reset
- **Nat 1** → counts as 2 failures
- 3 successes → **stabilized** (unconscious at 0 HP, no more death saves)
- 3 failures → **dead**
- Rolls posted publicly in `#combat-log`
- If the player doesn't send `/deathsave` before timeout, the system auto-rolls the death save on their behalf — same d20 rules apply (nat 1 = 2 failures, nat 20 = regain 1 HP). Result posts to `#combat-log` marked as "(auto-rolled — player timed out)"

**Taking damage while at 0 HP:**
- Each hit = 1 automatic death save failure
- Critical hit (attacker within 5ft) = 2 failures

**Stabilization:**
- 3 death save successes, Medicine check (DC 10), or Spare the Dying cantrip
- **Medicine check:** `/check medicine AR` — the target parameter specifies the dying creature. System validates: target must be at 0 HP (dying), and the checker must be adjacent (within 5ft). Costs the character's action (`action_used = true`). On success (DC 10), the target is stabilized. On failure, the target's death save state is unchanged (no penalty)
- **Spare the Dying:** `/cast spare-the-dying AR` — uses the standard `/cast` flow. System validates touch range (adjacent, within 5ft). Costs the action (cantrip with casting time of 1 action). Auto-succeeds with no roll — the target is stabilized immediately. Grave Domain Clerics can cast at 30ft range (handled via Feature Effect System `modify_range`)
- Stable characters remain unconscious at 0 HP, no further death saves
- Regain 1 HP after 1d4 hours (post-combat only)

**Healing from 0 HP:**
- Any healing sets HP to 0 + healing amount
- Status → conscious, still **prone** (costs half movement to stand)
- Death save tallies reset to zero

**Token states:** normal / bloodied / dying / stable / dead

### Example: A Full Player Turn

Player inputs (level 5 Fighter with longsword equipped, 2 attacks per action):
```
/move D4
/attack G1
```

Bot output in `#combat-log`:
```
🗺️  Aria moves from C3 → D4  (25ft used of 30ft)
⚔️  Aria attacks Goblin #1 with Longsword (attack 1 of 2)
    → Roll to hit: 19 (14 + 5) — HIT
    → Damage: 9 slashing
    → Goblin #1 is now Bloodied
    ℹ️  1 attack remaining
```

Player sees the result and decides to finish off G1 or switch targets:
```
/attack G2
```

```
⚔️  Aria attacks Goblin #2 with Longsword (attack 2 of 2)
    → Roll to hit: 11 (6 + 5) — MISS
    ℹ️  No attacks remaining
```

Then:
- Map image regenerates with Aria at D4
- G1's token gets a visual indicator (color shift or damage overlay)
- Bot pings next player in `#your-turn`

---

## Map System

### Map Rendering

- Grid images generated **server-side** as PNGs on every state change
- Bot **appends a new message** in `#combat-map` — creates a visual log players can scroll through
- Token labels display enemy IDs (G1, OS) and player initials
- Token visual states: normal / bloodied / dying / stable / dead
- Tile size: 32–48px per square to stay within Discord's 8MB file limit
- Obstacles and difficult terrain drawn as part of the base map layer

### Dynamic Fog of War

Fog of war is computed automatically based on **shared party vision** — the union of all player tokens' visible cells. One map image is rendered per update (no per-player maps).

**How it works:**
1. Each token carries vision properties: `base_vision_ft`, `darkvision_ft`, `blindsight_ft`, `truesight_ft`
2. Server runs **shadowcasting** from each player token's position against walls and obstacles
3. Visible cells for all party tokens are unioned → the "party known" area
4. Previously seen but currently out-of-range cells rendered as **dim/greyed out** (explored but not active)
5. Never-seen cells are **fully fogged** (black)
6. Enemy tokens in fogged cells are **hidden**; enemies in dim cells are visible but greyed

**Vision sources & modifiers:**
- **Darkvision** (60/120/300ft by race/feat) — darkness → dim, dim → normal
- **Light sources** — torches (20ft bright + 20ft dim), Light cantrip, Daylight spell — point lights on the grid
- **Blindsight / Tremorsense / Truesight** — ignore fog/obstacles within range
- **Devil's Sight** — sees through magical darkness

**Obscurement & lighting zones (DM-placed on grid):**

The DM places zones from the dashboard to define areas of non-standard lighting or obscurement. Each zone has a type and optional source label (spell, terrain feature, etc.). Zones affect combat mechanics automatically.

| Zone type | Visibility | Combat effects | Darkvision interaction |
|---|---|---|---|
| **Dim light** | Lightly obscured | Disadvantage on Perception (sight) checks; `/action hide` is available | Darkvision treats as bright light (no penalty) |
| **Darkness** | Heavily obscured | Effectively Blinded — auto-disadvantage on attacks, auto-advantage for attackers against you; blocks line-of-sight spells | Darkvision treats as dim light (Perception disadvantage only, not Blinded) |
| **Magical darkness** | Heavily obscured | Same as Darkness, but Darkvision does not help — only Devil's Sight, Blindsight, or Truesight penetrate | Darkvision has no effect |
| **Fog / heavy obscurement** | Heavily obscured | Same as Darkness (effectively Blinded); blocks line of sight | Darkvision has no effect (fog is not darkness) |
| **Light obscurement** | Lightly obscured | Same as Dim light (Perception disadvantage, Hide available) | No special interaction |

**Zone sources:**
- `Darkness` spell → magical darkness zone
- `Fog Cloud` → heavy obscurement zone
- `Wall of Fire / Stone` → blocks line of sight through the wall
- Heavy foliage / smoke → light or heavy obscurement
- Unlit dungeon rooms, nighttime outdoors → darkness or dim light zones

**Auto-applied combat modifiers:** when a creature is in an obscurement/lighting zone, the system checks the creature's vision capabilities (`darkvision_ft`, `blindsight_ft`, `truesight_ft`, Devil's Sight) and applies the effective visibility level. Modifiers are added to the existing advantage/disadvantage auto-detection pipeline:
- Attacks from or into heavily obscured zones: disadvantage/advantage per Blinded rules (unless attacker has appropriate vision)
- `/check perception` in lightly obscured zones: disadvantage (unless Darkvision negates)
- `/action hide` available in lightly or heavily obscured zones (normally requires something to hide behind)

Combat log shows the lighting modifier when it applies:
```
⚔️  Thorn attacks Goblin #1 — 🎲 14 (disadv: dim light, no darkvision) — Miss!
⚔️  Aria attacks Goblin #1 — 🎲 18 (darkvision: darkness → dim, no attack penalty) — Hit!
```

**Rendering layers (bottom to top):**
1. Base map (terrain, walls, obstacles)
2. Fog overlay (black for unknown, semi-transparent grey for explored-but-not-visible)
3. Tokens (only drawn if their cell is visible or explored)
4. Grid lines and labels

### Map Creation & Authoring

The DM creates and edits maps through the **Svelte-based map editor** in the web dashboard.

**Creating a new map:**
1. DM specifies grid dimensions (width × height in squares, e.g., 30×20)
2. Editor opens a blank grid with default terrain (open ground)
3. DM paints terrain, walls, and obstacles
4. DM optionally imports a background image as a visual underlay
5. Map is saved and available for encounters

**Map-making tools:**
- **Terrain brush** — paint per tile: open ground, difficult terrain, water, lava, pit, etc.
- **Wall tool** — draw walls along tile edges (block movement and line of sight)
- **Object placement** — doors (open/closed/locked), traps, furniture, interactables with custom properties
- **Elevation painting** — set ground elevation per tile (cliffs, platforms, stairs)
- **Spawn zones** — mark player and enemy token placement areas at encounter start

**Image import:**
- Upload a pre-made battle map image (PNG/JPG) as a **background layer** beneath the grid
- Grid overlaid with adjustable opacity
- Walls and terrain still need tool definitions (the image is purely visual; server needs structured data for pathfinding and fog of war)

**Storage format — Tiled-compatible JSON:**

Maps are stored using a format based on the **Tiled map editor's JSON specification** (`.tmj`):
- **Tile layers** — terrain types as tile GIDs referencing a tileset
- **Object layers** — walls, doors, spawn points, traps as typed objects with custom properties
- **Tileset references** — external `.tsj` files defining tile images and properties
- **Custom properties** — arbitrary key-value data on any element

Benefits:
- DMs can **import maps from the Tiled desktop editor** (free, widely used in TTRPG/game dev)
- Future support for **tileset-based map painting**
- Go backend parses via [`go-tiled`](https://github.com/lafriks/go-tiled) or `encoding/json`
- Maps export for backup, sharing, or community map packs

**Phase 1 (MVP):** blank grid + terrain/wall tools + image import. Maps stored as Tiled-compatible JSON.
**Phase 2:** tileset support — load `.tsj` tilesets, paint with tile brushes, import full `.tmj` maps from Tiled.

---

## Character Creation & Import

Characters are fully 5e-compatible — all SRD races, classes, backgrounds, and features. Characters can be created manually via the DM dashboard or imported from D&D Beyond.

### Internal Character Format

Characters are stored using a schema based on [BrianWendt/dnd5e_json_schema](https://github.com/BrianWendt/dnd5e_json_schema). The schema covers ability scores, class features, spells, equipment, and all mechanical fields needed for combat resolution.

The internal format is not exposed directly to users — it's the canonical storage representation that the dashboard UI and D&D Beyond importer both write to.

### Manual Character Creation (DM Dashboard)

The DM creates characters through a guided workflow:

1. **Basics** — name, race, background
2. **Classes** — add one or more class entries: class, subclass (if available at current level), and class level. Multiclass characters add multiple entries (e.g., Fighter 5 / Rogue 3)
3. **Ability scores** — manual entry (rolled or point-buy, DM's choice — system doesn't enforce a generation method)
4. **Derived stats** — HP, AC, proficiency bonus, saving throws, skill proficiencies auto-calculated from race + classes + ability scores + total level using SRD rules
5. **Equipment** — select from SRD weapons/armor/items; set equipped weapon and worn armor
6. **Spells** — for caster classes, select known/prepared spells from class spell list (filtered by level). Spell slots auto-calculated using 5e multiclass spellcasting table when applicable
7. **Features** — racial traits, class features, and subclass features auto-populated from SRD data based on race + class/subclass + level

After creation, the player links to the character via `/register <character_name>` in Discord (DM approves).

**Class/subclass/feat interactions:** the system implements SRD class and subclass features mechanically (Extra Attack, Sneak Attack damage, Rage bonus, Champion's Improved Critical, Life Domain's Disciple of Life, etc.) and auto-applies them during combat. Subclass features are loaded from `classes.subclasses[subclass_id].features_by_level` and merged into the character's `features` array alongside base class features. Non-SRD content can be added manually by the DM as custom features with mechanical effects.

### D&D Beyond Import

1. Player provides their D&D Beyond character URL (e.g., `https://www.dndbeyond.com/characters/12345678`)
2. System fetches from DDB's undocumented character API (`character-service.dndbeyond.com/character/v5/character/{id}`)
3. Parser converts DDB JSON into internal format — mapping ability scores, features, equipment, spells, HP, AC
4. DM reviews and approves in the dashboard

**Implementation reference:** [MrPrimate/ddb-importer](https://github.com/MrPrimate/ddb-importer) (Foundry VTT's DDB import module) — the most mature open-source DDB character parser.

**Caveats:**
- No official public API — endpoint is undocumented and may change
- Character must be set to **public** sharing on D&D Beyond
- Rate limiting/CAPTCHAs possible; importer includes exponential backoff
- Non-SRD content imports as names/descriptions but may need DM manual setup for mechanics

**Re-sync:** players can re-import to pull updates (level-ups, new equipment). System diffs and shows DM what changed before applying.

### Character Leveling

**Progression model:** milestone only. The DM decides when characters level up based on story progression — there is no XP tracking, no XP rewards, and no XP fields in the data model. This keeps async play simple and avoids bookkeeping between sessions.

**Leveling workflow:**
- DM edits a specific class entry's level in the dashboard (e.g., increase Fighter from 5 → 6, or add a new class entry for multiclassing)
- System auto-recalculates total `level`, HP (`hp_max` adds the new class's hit die + CON mod), proficiency bonus, spell slots (using 5e multiclass spellcasting table if multiclassed), attacks per action
- DM selects subclass when class level reaches the subclass threshold (varies by class: 1 for Cleric, 2 for Wizard, 3 for most others)
- DM selects new class/subclass features and spells if applicable
- If the level grants an ASI, the bot prompts the player with interactive buttons (see ASI/Feat selection below); the player's choice goes to `#dm-queue` for DM approval
- For DDB-imported characters, player levels up in D&D Beyond and re-imports

**Level-up notification:** when the DM applies a level-up, the bot sends two messages:

1. **Public announcement** in `#the-story`:
```
🎉  Aria has reached Level 6!
```

2. **Private detail ping** in `#your-turn` (pings the player):
```
🎉  Aria leveled up! Fighter 5 → Fighter 6
   ❤️  HP: 38 → 44 (+6)
   📈  Proficiency bonus: +3
   ⚔️  New feature: Extra Attack (2 attacks per action)
   🎓  ASI/Feat available — choose below!
```

The private message lists all mechanical changes (HP increase, new features, new spell slots, proficiency bonus changes) and flags any pending player choices (ASI/feat, new spells, subclass selection). The character card in `#character-cards` auto-updates as usual.

**ASI/Feat selection:** when a level-up grants an Ability Score Improvement (typically class levels 4, 8, 12, 16, 19), the bot appends an interactive prompt to the level-up notification in `#your-turn`:

```
🎓  Ability Score Improvement available!
    [+2 to One Score]  [+1 to Two Scores]  [Choose a Feat]
```

**ASI path:** player clicks `[+2 to One Score]` or `[+1 to Two Scores]`. The bot shows a follow-up with Discord select menus listing ability scores and their current values (e.g., "STR (16 → 18)"). Scores are capped at 20. If a player tries to exceed 20, the bot rejects with "❌ STR is already 20 — choose a different ability."

**Feat path:** player clicks `[Choose a Feat]`. The bot shows a paginated select menu of SRD feats the character qualifies for (prerequisites checked automatically — e.g., Heavy Armor Master requires heavy armor proficiency, Ritual Caster requires INT or WIS 13+). Each feat shows its name and a one-line summary. Feats the character already has are excluded.

After the player selects, the bot posts a structured request to `#dm-queue`:

```
🎓 ASI/Feat — Aria (Fighter 8) chose: Great Weapon Master
   Prerequisites: ✅ met
   Effects: -5 attack / +10 damage option on heavy weapons; bonus attack on crit/kill
   [Approve]  [Deny — message player]
```

The DM reviews and clicks `[Approve]` from the dashboard. On approval:
- **ASI:** ability scores updated, all derived stats recalculated (AC, attack/damage modifiers, save DCs, skill bonuses, HP if CON changed)
- **Feat:** added to the character's `features` array with its `mechanical_effect` declarations (processed via the Feature Effect System). Feats granting a +1 ASI (e.g., Resilient, Actor) apply the score increase simultaneously
- Character card in `#character-cards` auto-updates
- Bot confirms to the player in `#your-turn`: "✅ Great Weapon Master applied!"

If the DM denies (e.g., campaign doesn't allow certain feats), the player receives a message with the DM's reason and the prompt re-appears for a new selection.

**Feats with choices:** some feats require an additional selection (e.g., Resilient — choose an ability score; Elemental Adept — choose a damage type; Skilled — choose three proficiencies). The bot presents a follow-up select menu for these choices before posting to `#dm-queue`.

**Pending state:** until the ASI/feat choice is approved, the character card shows "⏳ ASI/Feat pending" and the player can re-trigger the prompt by clicking the original buttons. Pending choices do not block gameplay — the character can still participate in combat and exploration.

**Multiclass spell slots:** for characters with multiple spellcasting classes, spell slot totals are calculated from the 5e multiclass spellcasting table. Each class contributes a caster level based on its progression: full casters (Wizard, Cleric, Druid, Bard, Sorcerer) contribute class level × 1, half casters (Paladin, Ranger) contribute class level × ½ (rounded down), third casters (Eldritch Knight, Arcane Trickster) contribute class level × ⅓ (rounded down). The sum determines slots from the multiclass table. Warlock pact slots remain separate and are not included in this calculation.

**Multiclass spellcasting ability:** each spell belongs to one or more class spell lists. When a multiclass character casts a spell, the system uses the spellcasting ability of the class that provides that spell. If a spell appears on multiple of the character's class lists, the system uses the higher modifier.

---

## Reference Data Sources

### SRD Content (Seeded at Startup)

The system ships with the full **D&D 5e SRD** content, seeded into PostgreSQL on first run. Data sourced from [5e-bits/5e-database](https://github.com/5e-bits/5e-database) — a comprehensive, MIT-licensed JSON dataset.

**Included:**
- **Monsters** — ~325 creature stat blocks (name, HP formula, AC, speed, attacks, abilities, CR)
- **Spells** — ~320 spells (name, level, school, range, components, duration, area, damage, save type, concentration)
- **Weapons** — all SRD weapons (damage, damage type, properties: finesse, heavy, ranged, thrown, etc.)
- **Armor** — all SRD armor (AC formula, type, stealth disadvantage, strength requirement)
- **Equipment** — adventuring gear, tools, packs
- **Classes** — all 12 SRD classes with features by level, hit dice, proficiencies, spell lists
- **Feats** — all SRD feats with prerequisites, mechanical effects, and ASI bonuses
- **Races** — all SRD races with traits, ability score bonuses, speed, darkvision
- **Conditions** — all 15 standard conditions with mechanical effects
- **Skills** — all 18 skills mapped to ability scores

**Licensing:** SRD 5.1 content used under **CC-BY-4.0** (attribution only, included in about/credits page).

### Extended Content (Open5e)

For content beyond the core SRD, the system can optionally pull from the [Open5e API](https://api.open5e.com/):
- ~3,200 monsters (vs ~325 in SRD alone)
- ~1,400 spells
- Content from Tome of Beasts, Creature Codex, Deep Magic, and other OGL publishers

Fetched on-demand and cached locally. DM enables/disables third-party sources per campaign.

### Homebrew Content

DMs create custom entries for any reference data type via the dashboard:
- Custom monsters (full stat block editor)
- Custom spells, weapons, items
- Custom races and class features (name + mechanical effect)

Homebrew entries are scoped to the campaign and stored alongside SRD data with a `homebrew: true` flag.

---

## Non-Combat Gameplay

### Skill & Ability Checks

**Player-initiated:**
```
/check perception           ← d20 + WIS mod + proficiency (if proficient)
/check athletics --adv      ← roll with advantage
/check stealth --disadv     ← roll with disadvantage
/check dexterity            ← raw ability check (no skill proficiency)
/check medicine AR          ← targeted check (e.g., stabilize a dying ally)
```

**Targeted checks:** some checks require a target, specified as a combatant ID after the skill name. The system validates adjacency and target state as appropriate. During combat, targeted checks that require physical contact (e.g., Medicine to stabilize) cost the character's action.

**DM-prompted:** DM requests a check from the dashboard, pinging the player in `#your-turn`.

**Mechanics:**
- Roll formula: `d20 + ability_modifier + proficiency_bonus` (if proficient)
- Expertise (Rogue, Bard): doubles proficiency bonus — tracked as `{skill: "expertise"}`
- Jack of All Trades (Bard): adds half proficiency to non-proficient checks
- Passive checks: `10 + modifier`, displayed on character card. DM uses passive Perception for hidden checks
- All rolls post to `#roll-history`; DM narrates outcome in `#the-story`

**Group checks:** DM triggers (e.g., "group Stealth"). All players pinged simultaneously. System waits for responses (subject to timeout), reports results. Group succeeds if at least half succeed (per 5e).

**Contested checks:** DM triggers from dashboard (e.g., grapple: Athletics vs Athletics/Acrobatics). Both roll; system compares and reports.

### Short & Long Rests

**Short Rest** (`/rest short`):
1. Player types `/rest short` → posts to `#dm-queue`
2. DM approves from dashboard
3. System prompts the player with a Discord button menu to spend hit dice:
   - Single-class: Bot posts "You have **N** hit dice remaining (d**X**). Spend how many?" with buttons `[0] [1] [2] … [N]`
   - Multiclass: Bot posts hit dice grouped by type — "You have **3** d10 (Fighter) and **2** d8 (Rogue). Spend how many?" with separate button rows per die type
   - Each hit die heals `1dX + CON modifier` (X = hit die size for that class)
   - System rolls and applies healing automatically, capped at `hp_max`
   - Player selects 0 to remaining dice per type; selecting 0 for all skips hit dice spending
4. System resets all features with `recharge: "short"` (e.g., Action Surge, Channel Divinity, Second Wind)
5. Warlock `pact_magic_slots.current` restored to `pact_magic_slots.max`
6. Results posted to `#combat-log`

**Long Rest** (`/rest long`):
1. Player types `/rest long` → posts to `#dm-queue`
2. DM approves from dashboard
3. System applies:
   - HP restored to `hp_max`
   - All spell slots restored (both `spell_slots` and `pact_magic_slots`)
   - All features with `recharge: "short"` or `"long"` reset
   - Hit dice restored: regain up to half total character level (minimum 1) worth of hit dice, player's choice of which types to restore
   - Death save tallies reset to 0/0
4. Results posted to `#combat-log`
5. Prepared casters (Cleric, Druid, Paladin) receive a reminder: "You can change your prepared spells with `/prepare`."

**Constraints:**
- Only one long rest per 24 in-game hours (DM tracks narrative time; system does not enforce calendar)
- Rests cannot be initiated during active combat (system checks `encounter.status != 'active'`)
- If interrupted (DM cancels mid-rest from dashboard), partial benefits at DM discretion via manual override

### Inventory Management

Player inventory is stored in the `inventory` JSONB field on the characters table. Each entry has `{item_id, quantity, equipped, type}`. Gold is tracked as a separate `gold` INTEGER field on the characters table.

**Viewing inventory:** `/inventory` shows an ephemeral message listing all carried items grouped by type (weapons, armor, consumables, other), quantities, equipped status, and current gold. Example:

```
🎒 Aria's Inventory (23 gp)
⚔️ Weapons: +1 Longsword [uncommon] ✨ (equipped, main hand, attuned), Shortbow
🛡️ Armor: Chain Mail (equipped), Shield (equipped, off-hand)
💍 Magic Items: Cloak of Protection [uncommon] ✨ (attuned)
🧪 Consumables: Healing Potion ×2, Antitoxin ×1
🏹 Ammunition: Arrows ×18
📦 Other: Rope (50ft), Torch ×3

✨ = attuned (2/3 slots)
```

**Using consumables:** `/use healing-potion` consumes one item and applies its effect. The system auto-resolves items with defined effects:
- **Healing Potion:** roll 2d4+2, apply healing. Result posted to `#combat-log` in combat or `#roll-history` out of combat
- **Greater Healing Potion:** roll 4d4+4
- **Antitoxin:** grants advantage on saves vs poison for 1 hour (tracked as timed condition)
- Items without auto-resolve effects (e.g., "Ball Bearings") are routed to `#dm-queue` for DM adjudication

Using a consumable in combat costs an action (`action_used = true`). The DM can configure whether drinking a potion costs an action or a bonus action via a campaign setting (`potion_bonus_action: true/false`, default: action).

**Giving items:** `/give healing-potion AR` transfers one item to an adjacent ally (within 5ft). Both inventories are updated. In combat, giving an item uses the free object interaction (or costs an action if already used). Out of combat, no action cost.

**Looting:** after an encounter ends (`encounter.status = 'completed'`), the DM can populate a **loot pool** from the dashboard — selecting items from defeated creatures' inventories or adding custom loot. The system posts the loot pool to `#combat-log`:

```
💰 Loot available: Shortsword ×2, 15 gp, Healing Potion ×1, Mysterious Key
Type /loot to claim items
```

Players type `/loot` to see the loot pool and claim items via Discord buttons. Each item can only be claimed once. Unclaimed items remain in the pool until the DM clears it. Gold can be split evenly (DM clicks "Split Gold" in dashboard) or claimed individually.

**Gold tracking:** the `gold` field on characters is an integer representing total gold pieces. The DM can add/remove gold from the dashboard. Gold changes are logged to `#combat-log` or `#the-story`. Electrum, silver, copper, and platinum are converted to gold equivalents for simplicity (DM handles narrative flavor).

**DM inventory management:** the DM can add, remove, or transfer items between any characters from the dashboard. All DM-initiated changes are logged to `#combat-log`.

### Magic Items

Magic items are tracked via a `magic_items` reference table and stored in the character's `inventory` with additional fields (`is_magic`, `magic_bonus`, `magic_properties`, `requires_attunement`, `rarity`). The DM creates and assigns magic items from the dashboard, referencing existing entries from `magic_items` (SRD items pre-loaded) or creating custom/homebrew items.

**Bonus weapons and armor:** items with `magic_bonus` (+1, +2, +3) auto-apply the bonus to attack rolls, damage rolls (weapons), or AC (armor/shields) via the Feature Effect System. A +1 longsword adds +1 to both attack and damage; +2 plate adds +2 to AC. These bonuses stack with all other modifiers and are shown in combat log breakdowns:
```
⚔️  Aria attacks Goblin #1 with +1 Longsword
    → Roll to hit: 22 (14 + 5 STR + 2 prof + 1 magic) — HIT
    → Damage: 12 slashing (7 + 4 STR + 1 magic)
```

**Passive effects:** magic items can declare passive effects using the Feature Effect System vocabulary. These effects are active whenever the item is equipped (and attuned, if required). Examples:
- *Cloak of Protection:* `[{type: "modify_ac", modifier: 1}, {type: "modify_save", modifier: 1}]`
- *Boots of Speed:* `[{type: "modify_speed", modifier: "double", trigger: "on_activate"}]`
- *Ring of Resistance:* `[{type: "grant_resistance", damage_type: "fire"}]`

Passive effects are processed alongside class features and spell effects — the combat engine treats them identically.

**Active abilities:** some magic items have abilities that require activation, tracked via `feature_uses` on the character with item-specific recharge rules:
- *Wand of Fireballs (7 charges):* `/use wand-of-fireballs` → prompts for charges to spend (1-3) → casts Fireball at corresponding level. Tracked in `feature_uses["wand-of-fireballs"]` with `{current: 7, max: 7, recharge: "dawn", recharge_dice: "1d6+1"}`
- *Staff of Healing (10 charges):* `/use staff-of-healing` → prompts which spell to cast (Cure Wounds = 1 charge, Lesser Restoration = 2, Mass Cure Wounds = 5)
- Recharge at dawn: system rolls `recharge_dice` and restores charges. If `destroy_on_zero: true` and current reaches 0, roll d20 — on a 1, item is destroyed (DM notified)
- Active abilities that cast spells use the existing `/cast` infrastructure for targeting, saves, and damage

**Attunement:**
- Some magic items require attunement before their effects activate. `requires_attunement = true` on the item
- `/attune [item]` — attunes to a magic item. Requires a short rest (can be done during `/rest short` flow). System validates: item is in inventory, item requires attunement, character has fewer than 3 attuned items, and any attunement restrictions are met (e.g., "by a cleric" checks class)
- `/unattune [item]` — ends attunement immediately (no rest required). Item's passive effects and active abilities are deactivated. Frees an attunement slot
- Characters can attune to a maximum of 3 items at once. Attempting a 4th returns: "❌ You already have 3 attuned items. Use `/unattune [item]` to free a slot."
- Attunement tracked in `attunement_slots` JSONB on the characters table (array of `{item_id, name}`, max 3 entries)
- Equipping an unattunded item that requires attunement: item can be equipped but passive effects and active abilities do not function. The system warns: "⚠️ This item requires attunement. Use `/attune [item]` during a short rest to activate its properties."

**Inventory display:** `/inventory` shows magic items with rarity, attunement status, and magic bonus:
```
🎒 Aria's Inventory (23 gp)
⚔️ Weapons: +1 Longsword [uncommon] ✨ (equipped, main hand, attuned), Shortbow
🛡️ Armor: Chain Mail (equipped), Shield (equipped, off-hand)
💍 Magic Items: Cloak of Protection [uncommon] ✨ (equipped, attuned), Wand of Fireballs [rare] ✨ (attuned, 5/7 charges)
🧪 Consumables: Healing Potion ×2, Antitoxin ×1
🏹 Ammunition: Arrows ×18
📦 Other: Rope (50ft), Torch ×3

✨ = attuned (3/3 slots)
```

**Identifying magic items:** by default, magic items are identified when the DM assigns them (the DM controls what information is revealed). For unidentified items, the DM can set `identified: false` on the item — it shows as "Unidentified [type]" in inventory. Players can identify items via `/cast identify` (or `/cast detect-magic` to detect magical aura), or the DM reveals properties from the dashboard.

### Exploration, Social & Travel

Narrative-driven — no dedicated mechanical systems needed in MVP:

- **Exploration:** DM narrates in `#the-story`, players describe actions in `#in-character` or via `/action`. DM calls for checks as needed. If combat breaks out, DM starts an encounter from the dashboard.
- **Social:** Players roleplay in `#in-character`. DM reads player dialogue and actions there, then narrates outcomes in `#the-story`. DM calls for Charisma checks when uncertain. Discord's text format is ideal for RP.
- **Travel:** DM narrates distance and terrain in `#the-story`. Players describe travel activities in `#in-character`. Random encounters DM-triggered. Forced march / exhaustion checks via `/check constitution`.

`#dm-queue` serves as the universal escape hatch for anything that doesn't map to a command.

### DM Notification System

`#dm-queue` is the DM's single notification hub. The bot posts a structured message and pings the DM (or a configurable `@dm` role) for **every event requiring DM attention**:

| Event | Trigger | Example message |
|---|---|---|
| Freeform action | Player uses `/action` with non-auto-resolvable text | **🎭 Action** — Thorn: "flip the table" |
| Action cancelled | Player uses `/action cancel` on a pending freeform action | ~~🎭 Action — Thorn: "flip the table"~~ Cancelled by player |
| Reaction declaration | Player uses `/reaction` | **⚡ Reaction** — Aria: "Shield if I get hit" |
| Rest request | Player uses `/rest short` or `/rest long` | **🛏️ Rest** — Kael requests a short rest |
| Skill check narration | Auto-resolved `/check` completes | **🎲 Check** — Thorn: Athletics 18 (awaiting narration) |
| Consumable without effect | Player uses `/use` on item without auto-resolve | **🧪 Item** — Aria uses Ball Bearings |
| Enemy turn ready | Initiative advances to a DM-controlled combatant | **⚔️ Enemy Turn** — Goblin G2 is up |
| Narrative teleport | Teleportation spell beyond current map | **✨ Spell** — Kael casts Teleport (narrative resolution) |

Each notification includes the player name, action context, and a **"Resolve →"** link to the relevant dashboard panel.

**Resolved items** are edited by the bot to show ✅ and a brief outcome summary, so the DM can scan `#dm-queue` history and see what's been handled.

**DM-only channel:** `#dm-queue` is not visible to players. The DM configures Discord notifications for this channel to match their preferred responsiveness (push notifications, desktop alerts, etc.).

---

## DM Dashboard

The DM manages everything through a web app — they never type raw commands into Discord.

**Features:**
- **Combat Manager** — drag tokens on a grid, click to move, auto-calculate distance and range
- **HP & Condition Tracker** — click to apply damage, healing, and status conditions
- **Turn Queue** — shows initiative order; "End Turn" auto-advances and pings next player
- **Action Resolver** — view `#dm-queue` items, apply outcomes with a click
- **Active Reactions Panel** — always-visible sidebar showing all active `/reaction` declarations grouped by combatant. Each entry shows the player name, declaration text, and status (active / used this round / dormant). When the DM is resolving an enemy turn, matching declarations are highlighted. DM clicks to resolve or dismiss. Consumed reactions are greyed out until the creature's next turn resets them.
- **Stat Block Library** — preloaded monster stat blocks, reusable across encounters
- **Asset Library** — maps, token images, tilesets, custom monsters
- **Map Editor** — create and edit battle maps (see Map System)
- **Character Overview** — read-only view of all player character sheets

### Undo & Corrections

**Player-initiated undo request:** players use `/undo` (optionally with a reason: `/undo wrong target`) to request a correction. The bot:

1. Responds with an ephemeral acknowledgment: "✅ Undo request sent to DM."
2. Posts a structured request to `#dm-queue`:
```
🔄 **Undo Request** — Aria
   Last action: ⚔️ Attack Goblin #1 with Longsword — Hit for 9 damage
   Reason: "wrong target"
```

The DM reviews in the dashboard and either applies the undo or dismisses the request. The player is not automatically reverted — the DM always decides.

**DM undo tools (dashboard):**
- **Undo Last Action** — reverts the most recent mutation by restoring its `before_state` from the action log. Repeatable to walk back multiple steps within a turn. DM-only.
- **Manual State Override** — directly edit any value at any time: HP, position, conditions, spell slots, initiative order. Overrides go through the per-turn lock.
- **Discord Corrections** — every undo or override posts a correction to `#combat-log`: "⚠️ **DM Correction:** Goblin #1 HP adjusted (resistance to fire was missed)". Original messages are never edited or deleted.

**Not in MVP:** full turn rewind (reverting an entire multi-action turn) or automatic player-initiated undo. DM uses manual overrides instead.

---

## Tech Stack

| Layer | Technology | Purpose |
|---|---|---|
| Discord Bot | [discordgo](https://github.com/bwmarrin/discordgo) | Bot logic, slash commands, message editing |
| Backend API | Go stdlib `net/http` + Chi router | Game state management, command processing |
| Database | PostgreSQL + [sqlc](https://sqlc.dev) | Type-safe Go from raw SQL |
| DM Web App | Go templates for admin pages, [Svelte](https://svelte.dev) SPA for map editor | Svelte compiles to static JS/CSS, embedded in Go binary via `embed.FS` |
| Map Rendering | Go `image/draw` stdlib + [gg](https://github.com/fogleman/gg) | Server-side PNG generation |
| Real-time Sync | [nhooyr/websocket](https://github.com/nhooyr/websocket) | Live dashboard ↔ backend updates |
| Deployment | Single Go binary (dashboard embedded via `embed.FS`) | One artifact to deploy |

---

## Data Model

The database schema below defines the core entities and their relationships. All tables use UUID primary keys and include `created_at` / `updated_at` timestamps (omitted for brevity).

### Campaign & Player Tables

```sql
campaigns
  id              UUID PK
  guild_id        TEXT NOT NULL UNIQUE   -- Discord server ID (one campaign per server)
  dm_user_id      TEXT NOT NULL          -- Discord user ID of the DM
  name            TEXT NOT NULL
  settings        JSONB                  -- turn_timeout_hours, diagonal_rule, open5e_sources[], etc.
  status          TEXT NOT NULL          -- 'active', 'paused', 'archived'

characters
  id              UUID PK
  campaign_id     UUID FK → campaigns
  name            TEXT NOT NULL
  race            TEXT NOT NULL          -- FK → reference race or homebrew
  classes         JSONB NOT NULL         -- [{class: "fighter", subclass: "champion", level: 5}, {class: "rogue", subclass: "thief", level: 3}]
                                         -- single-class example: [{class: "wizard", subclass: "evocation", level: 8}]
                                         -- subclass may be null if class level hasn't reached subclass threshold
  level           INTEGER NOT NULL DEFAULT 1  -- cached total (sum of all class levels); indexed for queries
  ability_scores  JSONB NOT NULL         -- { str, dex, con, int, wis, cha }
  hp_max          INTEGER NOT NULL
  hp_current      INTEGER NOT NULL
  temp_hp         INTEGER NOT NULL DEFAULT 0
  ac              INTEGER NOT NULL       -- effective/cached AC value
  ac_formula      TEXT                   -- NULL = standard armor-based AC; set for Unarmored Defense / natural armor
                                         -- e.g., "10 + DEX + WIS" (Monk), "10 + DEX + CON" (Barbarian), "13 + DEX" (Lizardfolk)
                                         -- when non-null, system recalculates `ac` on ability score changes
  speed_ft        INTEGER NOT NULL DEFAULT 30
  proficiency_bonus INTEGER NOT NULL
  equipped_main_hand TEXT                -- FK → weapons (primary weapon)
  equipped_off_hand  TEXT                -- FK → weapons or armor (second weapon or shield; null = free hand)
  equipped_armor  TEXT                   -- FK → armor (body armor only, not shield)
  spell_slots     JSONB                  -- { "1": {current: 2, max: 4}, "2": {current: 3, max: 3}, ... }
  pact_magic_slots JSONB                 -- Warlock only: { "slot_level": 3, "current": 2, "max": 2 }
                                         -- NULL for non-Warlocks. Separate pool from spell_slots.
                                         -- Recharges on short rest (not long rest only like spell_slots).
                                         -- For multiclass Warlocks, both pools exist independently.
  hit_dice_remaining JSONB NOT NULL       -- {"d10": 3, "d8": 2} — keyed by die size, tracks remaining per class die type
  feature_uses    JSONB                  -- { "action-surge": {current: 1, max: 1, recharge: "short"}, ... }
  features        JSONB                  -- [{name, source, level, description, mechanical_effect}]
  proficiencies   JSONB                  -- {saves: [str, con], skills: [athletics, perception], weapons: [...], armor: [...]}
  gold            INTEGER NOT NULL DEFAULT 0  -- total gold pieces (all currency converted to gp)
  attunement_slots JSONB                 -- [{item_id, name}] max 3 entries; tracks attuned magic items
  inventory       JSONB                  -- [{item_id, quantity, equipped, type, is_magic, magic_bonus, magic_properties, requires_attunement, rarity}] type: weapon/armor/consumable/ammunition/other
  character_data  JSONB                  -- full dnd5e_json_schema blob for import/export fidelity
  ddb_url         TEXT                   -- D&D Beyond URL if imported
  homebrew        BOOLEAN DEFAULT false

player_characters
  id              UUID PK
  campaign_id     UUID FK → campaigns
  character_id    UUID FK → characters   UNIQUE per campaign
  discord_user_id TEXT NOT NULL
  approved        BOOLEAN DEFAULT false  -- DM must approve /register
  UNIQUE(campaign_id, discord_user_id)   -- one character per player per campaign
```

### Encounter & Combat Tables

```sql
encounters
  id              UUID PK
  campaign_id     UUID FK → campaigns
  map_id          UUID FK → maps
  name            TEXT
  status          TEXT NOT NULL          -- 'preparing', 'active', 'completed'
  round_number    INTEGER DEFAULT 0
  current_turn_id UUID FK → turns        -- nullable; set when combat is active

combatants
  id              UUID PK
  encounter_id    UUID FK → encounters
  character_id    UUID FK → characters   -- NULL for creatures
  creature_ref_id TEXT                   -- FK → creatures reference data; NULL for PCs
  short_id        TEXT NOT NULL          -- "G1", "OS", "AR" — stable for entire encounter
  display_name    TEXT NOT NULL          -- "Goblin #1", "Orc Shaman", "Aria"
  initiative_roll INTEGER NOT NULL
  initiative_order INTEGER NOT NULL      -- tiebreaker ordering
  position_col    TEXT NOT NULL          -- grid column: "A", "AA", etc.
  position_row    INTEGER NOT NULL       -- grid row: 1, 2, ...
  altitude_ft     INTEGER DEFAULT 0
  hp_max          INTEGER NOT NULL       -- instance HP (creatures may vary from template)
  hp_current      INTEGER NOT NULL
  temp_hp         INTEGER DEFAULT 0
  ac              INTEGER NOT NULL
  conditions      JSONB DEFAULT '[]'     -- [{condition, source_combatant_id, duration_rounds, started_round, expires_on}]
  exhaustion_level INTEGER DEFAULT 0    -- 0-6; effects are cumulative (see Exhaustion)
  death_saves     JSONB                  -- {successes: 0, failures: 0} — PCs only
  is_visible      BOOLEAN DEFAULT true   -- hidden enemies (ambush, stealth)
  is_alive        BOOLEAN DEFAULT true
  is_npc          BOOLEAN DEFAULT false  -- DM-controlled ally vs enemy
  is_raging       BOOLEAN DEFAULT false  -- Barbarian rage state
  rage_rounds_remaining INTEGER          -- countdown from 10; NULL when not raging
  rage_attacked_this_round BOOLEAN DEFAULT false  -- reset at turn start, set on melee attack
  rage_took_damage_this_round BOOLEAN DEFAULT false  -- reset at turn start, set on taking damage
  is_wild_shaped  BOOLEAN DEFAULT false  -- Druid Wild Shape state
  wild_shape_creature_ref TEXT            -- FK → creatures; beast form creature ID; NULL when not shifted
  wild_shape_original JSONB               -- snapshot of pre-transformation stats (hp_max, hp_current, ac, ability_scores, speed_ft, attacks); NULL when not shifted
  summoner_id     UUID FK → combatants   -- nullable; links summoned creature to summoning player's combatant

turns
  id              UUID PK
  encounter_id    UUID FK → encounters
  combatant_id    UUID FK → combatants
  round_number    INTEGER NOT NULL
  status          TEXT NOT NULL          -- 'active', 'completed', 'skipped'
  movement_remaining_ft INTEGER NOT NULL
  action_used     BOOLEAN DEFAULT false
  bonus_action_used BOOLEAN DEFAULT false
  bonus_action_spell_cast BOOLEAN DEFAULT false  -- tracks bonus action spell restriction (forward direction)
  action_spell_cast BOOLEAN DEFAULT false        -- tracks leveled action spell cast (reverse direction of bonus action spell restriction)
  reaction_used   BOOLEAN DEFAULT false  -- per-round, not per-turn
  free_interact_used BOOLEAN DEFAULT false
  attacks_remaining INTEGER NOT NULL DEFAULT 1
  has_disengaged  BOOLEAN DEFAULT false   -- tracks Disengage action for opportunity attack suppression
  action_surged   BOOLEAN DEFAULT false   -- tracks Action Surge used this turn (prevents double surge)
  started_at      TIMESTAMPTZ
  timeout_at      TIMESTAMPTZ           -- started_at + campaign timeout setting
  completed_at    TIMESTAMPTZ

action_log
  id              UUID PK
  turn_id         UUID FK → turns
  encounter_id    UUID FK → encounters   -- denormalized for fast queries
  action_type     TEXT NOT NULL          -- 'move', 'attack', 'cast', 'damage', 'heal', 'condition_add', 'condition_remove', 'death_save', 'dm_override', etc.
  actor_id        UUID FK → combatants
  target_id       UUID FK → combatants   -- nullable
  description     TEXT                   -- human-readable summary
  before_state    JSONB NOT NULL         -- snapshot of affected fields before mutation
  after_state     JSONB NOT NULL         -- snapshot after mutation
  dice_rolls      JSONB                  -- [{die, count, results[], modifier, total, purpose}]
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
```

### Reaction Declarations

```sql
reaction_declarations
  id              UUID PK
  encounter_id    UUID FK → encounters
  combatant_id    UUID FK → combatants
  description     TEXT NOT NULL          -- "Shield if I get hit", "OA if goblin moves away"
  status          TEXT NOT NULL          -- 'active', 'used', 'cancelled'
  created_at      TIMESTAMPTZ NOT NULL
  resolved_at     TIMESTAMPTZ
  resolution_note TEXT                   -- DM's note on how it resolved
```

### Maps

```sql
maps
  id              UUID PK
  campaign_id     UUID FK → campaigns
  name            TEXT NOT NULL
  width_squares   INTEGER NOT NULL
  height_squares  INTEGER NOT NULL
  tiled_json      JSONB NOT NULL         -- full Tiled-compatible .tmj data
  background_image_url TEXT              -- optional uploaded battle map image
  tileset_refs    JSONB                  -- [{name, source_url, first_gid}]
```

### Reference Data (SRD + Homebrew)

```sql
creatures
  id              TEXT PK               -- slug: "goblin", "adult-red-dragon"
  campaign_id     UUID FK → campaigns   -- NULL for SRD entries
  name            TEXT NOT NULL
  size            TEXT NOT NULL
  type            TEXT NOT NULL          -- beast, humanoid, undead, etc.
  alignment       TEXT
  ac              INTEGER NOT NULL
  ac_type         TEXT                   -- "natural armor", "chain mail", etc.
  hp_formula      TEXT NOT NULL          -- "2d6+2"
  hp_average      INTEGER NOT NULL
  speed           JSONB NOT NULL         -- {walk: 30, fly: 60, swim: 30}
  ability_scores  JSONB NOT NULL         -- {str, dex, con, int, wis, cha}
  saving_throws   JSONB                  -- {dex: 4, wis: 2}
  skills          JSONB                  -- {perception: 4, stealth: 6}
  damage_resistances   TEXT[]
  damage_immunities    TEXT[]
  damage_vulnerabilities TEXT[]
  condition_immunities TEXT[]
  senses          JSONB                  -- {darkvision: 60, passive_perception: 14}
  languages       TEXT[]
  cr              TEXT NOT NULL          -- "1/4", "1", "17"
  attacks         JSONB NOT NULL         -- [{name, to_hit, damage, damage_type, reach_ft, range_ft, description}]
  abilities       JSONB                  -- [{name, description, recharge}] — special abilities, legendary actions
  homebrew        BOOLEAN DEFAULT false
  source          TEXT DEFAULT 'srd'     -- 'srd', 'open5e:tome-of-beasts', 'homebrew'

spells
  id              TEXT PK               -- slug: "fireball", "cure-wounds"
  campaign_id     UUID FK → campaigns   -- NULL for SRD entries
  name            TEXT NOT NULL
  level           INTEGER NOT NULL       -- 0 for cantrips
  school          TEXT NOT NULL          -- evocation, abjuration, etc.
  casting_time    TEXT NOT NULL          -- "1 action", "1 bonus action", "1 reaction"
  range_ft        INTEGER                -- NULL for "self", "touch" = 5
  range_type      TEXT NOT NULL          -- 'ranged', 'touch', 'self'
  components      JSONB NOT NULL         -- {v: true, s: true, m: "a tiny ball of bat guano"}
  material_cost_gp INTEGER               -- NULL if no costly component; gold value if costly (e.g., 300 for Revivify)
  material_consumed BOOLEAN DEFAULT false -- true if the material component is consumed on cast
  duration        TEXT NOT NULL          -- "instantaneous", "1 minute", "concentration, up to 1 hour"
  concentration   BOOLEAN DEFAULT false
  ritual          BOOLEAN DEFAULT false
  area            JSONB                  -- {shape: "sphere", radius_ft: 20} or {shape: "cone", length_ft: 15}
  attack_type     TEXT                   -- 'melee', 'ranged', or NULL (save-based/auto-hit)
  save_type       TEXT                   -- "dex", "wis", etc. NULL if no save
  damage          JSONB                  -- {dice: "8d6", type: "fire", higher_levels: "1d6 per slot above 3rd", cantrip_scaling: true}
  healing         JSONB                  -- {dice: "1d8+mod", higher_levels: "1d8 per slot above 1st"}
  effects         TEXT                   -- description of non-damage effects
  teleport        JSONB                  -- NULL for non-teleport spells. See Teleportation Spells below for schema
  classes         TEXT[] NOT NULL        -- ["wizard", "sorcerer"]
  homebrew        BOOLEAN DEFAULT false
  source          TEXT DEFAULT 'srd'

weapons
  id              TEXT PK               -- slug: "longsword", "longbow"
  name            TEXT NOT NULL
  damage          TEXT NOT NULL          -- "1d8"
  damage_type     TEXT NOT NULL          -- slashing, piercing, bludgeoning
  weight_lb       REAL
  properties      TEXT[] NOT NULL        -- ["versatile", "finesse", "heavy", "two-handed", "ranged", "thrown", "light", "ammunition", "reach", "loading"]
  range_normal_ft INTEGER               -- 80 for longbow, NULL for melee
  range_long_ft   INTEGER               -- 320 for longbow
  versatile_damage TEXT                  -- "1d10" for longsword
  weapon_type     TEXT NOT NULL          -- 'simple_melee', 'simple_ranged', 'martial_melee', 'martial_ranged'

armor
  id              TEXT PK
  name            TEXT NOT NULL
  ac_base         INTEGER NOT NULL       -- 11 for leather, 18 for plate
  ac_dex_bonus    BOOLEAN DEFAULT true   -- false for heavy armor
  ac_dex_max      INTEGER                -- 2 for medium armor, NULL for light
  strength_req    INTEGER                -- 15 for plate
  stealth_disadv  BOOLEAN DEFAULT false
  armor_type      TEXT NOT NULL          -- 'light', 'medium', 'heavy', 'shield'
  weight_lb       REAL

feats
  id              TEXT PK               -- slug: "great-weapon-master", "sentinel"
  name            TEXT NOT NULL
  description     TEXT NOT NULL
  prerequisites   JSONB                  -- NULL (no prereqs) or {ability: {str: 13}, proficiency: "heavy_armor", spellcasting: true}
  asi_bonus       JSONB                  -- NULL or {choose_ability: 1, from: ["str", "con"]} for feats granting +1 to a score
  mechanical_effect JSONB               -- [{effect_type, ...}] uses Feature Effect System vocabulary
                                         -- e.g., Sentinel: [{type: "on_hit", trigger: "opportunity_attack", effect: "set_speed_0"}]

classes
  id              TEXT PK               -- "fighter", "wizard"
  name            TEXT NOT NULL
  hit_die         TEXT NOT NULL          -- "d10"
  primary_ability TEXT NOT NULL          -- "str" or "dex"
  save_proficiencies TEXT[] NOT NULL     -- ["str", "con"]
  armor_proficiencies TEXT[]
  weapon_proficiencies TEXT[]
  skill_choices   JSONB                  -- {choose: 2, from: ["athletics", "perception", ...]}
  spellcasting    JSONB                  -- {ability: "int", slot_progression: "full"} or NULL
  features_by_level JSONB NOT NULL       -- {"1": [{name, description, mechanical_effect}], "2": [...], ...}
  attacks_per_action JSONB NOT NULL      -- {"1": 1, "5": 2, "11": 3, "20": 4} (Fighter example)
  subclass_level  INTEGER NOT NULL       -- level at which subclass is chosen (1 for Cleric, 2 for Wizard, 3 for most)
  subclasses      JSONB NOT NULL         -- {"champion": {name: "Champion", features_by_level: {"3": [...], "7": [...], ...}},
                                         --  "battle-master": {name: "Battle Master", features_by_level: {...}}}
                                         -- subclass features are merged into character's features array at the appropriate levels
  multiclass_prereqs JSONB               -- {str: 13} or {dex: 13, wis: 13} — ability score requirements to multiclass into this class
  multiclass_proficiencies JSONB          -- proficiencies gained when multiclassing into this class (subset of base class proficiencies per 5e rules)

races
  id              TEXT PK               -- "elf", "dwarf", "human"
  name            TEXT NOT NULL
  speed_ft        INTEGER NOT NULL
  size            TEXT NOT NULL          -- "Medium", "Small"
  ability_bonuses JSONB NOT NULL         -- {dex: 2} or {all: 1}
  darkvision_ft   INTEGER DEFAULT 0
  traits          JSONB NOT NULL         -- [{name, description, mechanical_effect}]
  languages       TEXT[]
  subraces        JSONB                  -- [{id, name, ability_bonuses, traits}]

magic_items
  id              TEXT PK               -- slug: "longsword-plus-1", "cloak-of-protection"
  campaign_id     UUID FK → campaigns   -- NULL for SRD entries
  name            TEXT NOT NULL          -- "+1 Longsword", "Cloak of Protection"
  base_item_type  TEXT                   -- "weapon", "armor", "wondrous", "ring", "wand", "staff", "rod", "potion", "scroll"
  base_item_id    TEXT                   -- FK → weapons/armor (e.g., "longsword") if applicable, NULL for wondrous items
  rarity          TEXT NOT NULL          -- "common", "uncommon", "rare", "very_rare", "legendary", "artifact"
  requires_attunement BOOLEAN DEFAULT false
  attunement_restriction TEXT            -- NULL or restriction: "by a cleric", "by a spellcaster"
  magic_bonus     INTEGER               -- +1, +2, +3 for weapons/armor/shields; NULL for non-bonus items
  passive_effects JSONB                  -- [{effect_type, ...}] uses Feature Effect System vocabulary
                                         -- e.g., Cloak of Protection: [{type: "modify_ac", modifier: 1}, {type: "modify_save", modifier: 1}]
  active_abilities JSONB                 -- [{name, description, charges_cost, action_type, spell_id, mechanical_effect}]
                                         -- e.g., Wand of Fireballs: [{name: "Fireball", charges_cost: 1, spell_id: "fireball", action_type: "action"}]
  charges         JSONB                  -- {max: 7, recharge: "dawn", recharge_dice: "1d6+1", destroy_on_zero: true} or NULL
  description     TEXT NOT NULL
  homebrew        BOOLEAN DEFAULT false
  source          TEXT DEFAULT 'srd'

conditions_ref
  id              TEXT PK               -- "blinded", "prone", "stunned"
  name            TEXT NOT NULL
  description     TEXT NOT NULL
  mechanical_effects JSONB NOT NULL      -- [{effect_type, ...}] — uses Feature Effect System vocabulary
                                         -- see Condition Effects section for full mechanics per condition
```

### Key Design Decisions

1. **Combatants as instances** — the `combatants` table joins encounters to characters/creatures with instance-level state (position, current HP, conditions). A single creature template spawns multiple combatants (G1, G2, G3) each tracking independent HP and conditions.

2. **JSONB for flexible data** — ability scores, features, spell slots, and inventory use JSONB columns rather than normalized tables. This matches the nested structure of 5e character data and avoids dozens of join tables for rarely-queried data. Combat-critical fields (HP, AC, position) remain as typed columns for indexed queries.

3. **Reference data with campaign scope** — SRD entries have `campaign_id = NULL` (global). Homebrew entries are scoped to a campaign. Queries use `WHERE campaign_id = $1 OR campaign_id IS NULL` to merge both.

4. **Action log for undo** — every mutation records `before_state` / `after_state` as JSONB snapshots. The DM's undo operation restores `before_state`. The log also serves as the audit trail for `#combat-log` and `#roll-history` Discord channels.

5. **Character data blob** — the `character_data` JSONB column stores the full dnd5e_json_schema representation. This preserves import fidelity (D&D Beyond data that doesn't map to typed columns) and enables future export. The typed columns are the source of truth for gameplay; `character_data` is the source of truth for display and re-export.

6. **No separate combat state table** — the encounter tracks `status` and `current_turn_id`. Current combat state is derived from the encounter's combatants + the active turn row.

7. **Single-pass effect processor** — all combat modifiers (conditions, class features, spell effects, racial traits) are resolved through one unified pipeline using the Feature Effect System's effect type vocabulary. At each trigger point, the processor collects all applicable effects, filters by conditions, and applies them in priority order. This avoids scattered hardcoded checks and makes new features data-driven.

---

## Risks & Mitigations

| Risk | Mitigation |
|---|---|
| State drift from manual Discord edits | Enforce read-only Discord policy; all state changes via bot or dashboard only |
| Map images exceeding 8MB | Cap tile size at 48px; compress PNGs; limit grid size |
| Enemy ID ambiguity | IDs are stable, map-labeled, and confirmed in every combat log response |
| Complex player actions breaking automation | `/action` routes to DM; no attempt to auto-parse freeform intent |
| D&D Beyond API instability | Undocumented endpoint may change; importer includes fallback to manual creation |
| Turn stalls from AFK players | 24h configurable timeout with escalating pings and auto-skip to Dodge |
| Reaction timing in async | Pre-declaration model — no combat pauses; DM resolves manually. OA uses queue-and-continue: movement completes, hostile responds by end-of-round, DM handles retroactive correction if OA drops target to 0 HP |
| Concurrent commands corrupting state | Per-turn PostgreSQL advisory locks serialize all mutations |

---

## MVP Scope

**Included:**
- Discord bot with full channel structure (`/setup` auto-creates)
- All slash commands: `/move`, `/attack`, `/cast`, `/bonus`, `/shove`, `/fly`, `/interact`, `/action`, `/reaction`, `/deathsave`, `/done`, `/command`, `/equip`, `/status`, `/inventory`, `/use`, `/give`, `/loot`, `/register`, `/check`, `/save`, `/rest`
- Combat state: HP, position, turn order, conditions, death saves, spell slots, concentration
- Turn timeout with escalating pings and auto-skip
- Enemy/NPC turns with smart-default suggestions for DM
- Server-side map PNG generation with dynamic fog of war
- DM web dashboard: grid view, token drag-drop, HP management, turn advancement, undo, manual overrides
- Character creation (full 5e SRD: race, class, abilities, equipment, spells)
- D&D Beyond character import
- SRD reference data seeded at startup (monsters, spells, weapons, armor, classes, races, conditions)
- Skill/ability checks and rest mechanics with feature-use recharge
- Dice rolls auto-logged to `#roll-history`
- Map editor: blank grid + terrain/wall tools + image import (Tiled-compatible JSON)
- Data-driven Feature Effect System for class features, racial traits, and spell effects
- Core combat rules: opportunity attacks, cover, difficult terrain, surprise, critical hits, temp HP, two-weapon fighting, grapple/shove, stealth/hiding
- Spell mechanics: spell save DC, spell attack rolls, upcasting, ritual casting, cantrip damage scaling
- Weapon mechanics: finesse auto-select, loading, versatile, reach validation, heavy weapon size restriction, ammunition tracking, thrown weapon range
- Condition/spell duration auto-expiration
- Equipment enforcement (armor STR requirements, stealth disadvantage, free object interaction limits)
- Standard actions: Dash, Disengage, Dodge, Escape, Help, Hide, Ready (auto-resolved where deterministic)
- Monk Ki: Martial Arts bonus attack, Flurry of Blows, Patient Defense, Step of the Wind, Stunning Strike, ki point tracking
- Metamagic: all 8 SRD options via `--metamagic` flags on `/cast`, sorcery points, Font of Magic slot/point conversion
- Wild Shape: `/bonus wild-shape`, `/bonus revert`, beast stat swap, dual HP pool, CR validation, auto-revert
- Summoned creatures & companions: player-controlled via `/command`, initiative tracking, dismissal
- Inventory management: `/inventory`, `/use`, `/give`, `/loot`, gold tracking, consumable auto-resolution, post-combat loot pool

**Future phases:**
- Tileset-based map painting + Tiled desktop import
- Full asset library
- Open5e third-party content integration
- Campaign/session management
