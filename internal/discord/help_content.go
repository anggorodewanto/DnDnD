package discord

// generalHelp is the response for /help with no arguments.
const generalHelp = "\U0001F4D6 **DnDnD Command Reference**\n" +
	"\n**Movement**\n" +
	"`/move [coordinate]` \u2014 Move your character on the battle map\n" +
	"`/fly [altitude]` \u2014 Fly to a given altitude in feet\n" +
	"\n**Combat**\n" +
	"`/attack [target]` \u2014 Attack a target with a weapon\n" +
	"`/cast [spell]` \u2014 Cast a spell\n" +
	"`/bonus [action]` \u2014 Use a bonus action\n" +
	"`/action [action]` \u2014 Use an action on your turn\n" +
	"`/shove [target]` \u2014 Shove a target creature\n" +
	"`/interact [description]` \u2014 Free object interaction\n" +
	"`/done` \u2014 End your turn\n" +
	"`/deathsave` \u2014 Roll a death saving throw\n" +
	"`/command [creature] [action]` \u2014 Command a companion creature\n" +
	"`/reaction` \u2014 Declare, cancel, or clear reactions\n" +
	"\n**Checks & Saves**\n" +
	"`/check [skill]` \u2014 Make an ability or skill check\n" +
	"`/save [ability]` \u2014 Make a saving throw\n" +
	"\n**Communication**\n" +
	"`/whisper [message]` \u2014 Send a private message to the DM\n" +
	"\n**Status & Inventory**\n" +
	"`/status` \u2014 Show your character's current status\n" +
	"`/equip [item]` \u2014 Equip an item\n" +
	"`/inventory` \u2014 Show your inventory\n" +
	"`/use [item]` \u2014 Use a consumable item\n" +
	"`/give [item] [target]` \u2014 Give an item to another creature\n" +
	"`/loot` \u2014 Loot the area or a defeated creature\n" +
	"`/attune [item]` \u2014 Attune to a magic item\n" +
	"`/prepare` \u2014 Prepare your spell list for the day\n" +
	"`/rest [type]` \u2014 Take a short or long rest\n" +
	"\n**Character Management**\n" +
	"`/register [name]` \u2014 Link to a character your DM already created\n" +
	"`/import [ddb-url]` \u2014 Import a character from D&D Beyond\n" +
	"`/create-character` \u2014 Build a character in the web portal\n" +
	"`/character` \u2014 Show your character sheet summary\n" +
	"`/recap` \u2014 Show a recap of recent combat rounds\n" +
	"`/distance [target]` \u2014 Measure distance between targets\n" +
	"\n**Utility**\n" +
	"`/undo` \u2014 Request to undo your last action\n" +
	"`/retire` \u2014 Retire your character from the campaign\n" +
	"`/help [topic]` \u2014 Show help for a specific command or class\n" +
	"`/setup` \u2014 Create the channel structure for this campaign\n" +
	"\nUse `/help [topic]` for detailed help on any command or class (e.g. `/help attack`, `/help rogue`, `/help ki`)"

// helpTopics maps help topic names to their detailed help text.
var helpTopics = map[string]string{
	"attack":           helpAttack,
	"action":           helpAction,
	"ki":               helpKi,
	"rogue":            helpRogue,
	"cleric":           helpCleric,
	"paladin":          helpPaladin,
	"metamagic":        helpMetamagic,
	"cast":             helpCast,
	"move":             helpMove,
	"check":            helpCheck,
	"save":             helpSave,
	"rest":             helpRest,
	"equip":            helpEquip,
	"inventory":        helpInventory,
	"use":              helpUse,
	"give":             helpGive,
	"loot":             helpLoot,
	"attune":           helpAttune,
	"prepare":          helpPrepare,
	"retire":           helpRetire,
	"register":         helpRegister,
	"import":           helpImport,
	"create-character": helpCreateCharacter,
	"character":        helpCharacter,
	"recap":            helpRecap,
	"distance":         helpDistance,
	"whisper":          helpWhisper,
	"status":           helpStatus,
	"done":             helpDone,
	"deathsave":        helpDeathsave,
	"command":          helpCommand,
	"reaction":         helpReaction,
	"interact":         helpInteract,
	"shove":            helpShove,
	"bonus":            helpBonus,
	"fly":              helpFly,
	"undo":             helpUndo,
	"help":             helpHelpTopic,
	"setup":            helpSetup,
}

// --- Spec-defined detailed help topics ---

const helpAttack = `/attack — Attack a Target

Usage:
  /attack [target]                    Attack with equipped weapon
  /attack [target] [weapon]           Attack with a specific weapon
  /attack [target] improvised         Improvised weapon (1d4 bludgeoning, no proficiency)
  /attack [target] unarmed            Unarmed strike (1 + STR mod bludgeoning)

Flags:
  --twohanded         Use versatile weapon's two-handed damage (off-hand must be free)
  --gwm               Great Weapon Master: -5 to hit, +10 damage (heavy melee only)
  --sharpshooter      Sharpshooter: -5 to hit, +10 damage (ranged only)
  --reckless          Reckless Attack: advantage on melee STR attacks, enemies get
                      advantage against you until next turn (Barbarian only, first attack)
  --thrown             Throw a weapon with the thrown property (or improvised, range 20/60)

Extra Attack:
  Each /attack resolves one swing. Your remaining attacks are shown after each hit.
  Retarget freely between swings. Unused attacks are forfeited on /done.

Two-Weapon Fighting:
  /bonus offhand      Off-hand attack with a light weapon (no ability mod to damage
                      unless you have the Two-Weapon Fighting fighting style)

Improvised Weapons:
  Grab an object from the environment — no inventory needed.
  1d4 + STR bludgeoning, no proficiency bonus (Tavern Brawler grants proficiency).
  Throw with --thrown (range 20/60). DM can adjust damage type/amount after the fact.

Tips:
• Advantage/disadvantage is auto-detected from conditions, position, and lighting
• Finesse weapons auto-select the higher of STR or DEX
• Critical hit on nat 20 — all damage dice doubled
• Divine Smite prompt appears automatically after a Paladin melee hit`

const helpAction = `/action — Actions on Your Turn

Standard actions (auto-resolved):
  /action disengage       Move without provoking opportunity attacks
  /action dash            Double your movement this turn
  /action dodge           Attacks against you have disadvantage until next turn
  /action help [ally] [target]  Give an ally advantage on their next attack/check
  /action hide            Stealth vs passive Perception (must have cover/obscurement)
  /action grapple [target] Grapple a creature (contested Athletics check)
  /action escape          Break free from a grapple (contested check)
  /action stand           Stand up from prone (costs half your movement)
  /action drop-prone      Drop prone voluntarily (no cost)
  /action ready [trigger] Hold your action for a trigger (uses your reaction)
  /action surge           Extra action this turn (Fighter only)
  /action channel-divinity [option]  Channel Divinity (Cleric/Paladin)
  /action lay-on-hands [target] [hp]  Restore HP from healing pool (Paladin only)

Freeform actions (DM-resolved):
  /action [anything]      Describe a creative action — sent to DM for resolution
                          Examples: /action flip the table for cover
                                   /action grab the chandelier and swing to F2

Cancel:
  /action cancel          Withdraw your pending freeform action (before DM resolves it)

Tips:
• Standard actions cost your action for the turn (except stand/drop-prone)
• Freeform actions also cost your action — the DM decides the outcome
• Use /undo if you need to correct an already-resolved action`

const helpKi = "\U0001F44A Ki Abilities — Monk\n" +
	"\n" +
	"  Martial Arts (free):\n" +
	"    /bonus martial-arts              Free unarmed strike after Attack action (no ki cost)\n" +
	"\n" +
	"  Ki abilities (1 ki each, recharge on short rest):\n" +
	"    /bonus flurry-of-blows           2 unarmed strikes after Attack action (replaces martial-arts)\n" +
	"    /bonus patient-defense            Dodge as bonus action (disadv on attacks against you)\n" +
	"    /bonus step-of-the-wind           Dash or Disengage as bonus action + double jump\n" +
	"\n" +
	"  On-hit (prompted automatically):\n" +
	"    Stunning Strike (lvl 5+)         1 ki \u2014 target CON save or Stunned (prompted on melee hit)\n" +
	"\n" +
	"  Ki points: Monk level (use /status to check)    Recharge: short rest\n" +
	"\n" +
	"  Martial Arts die: 1d4 (lvl 1) \u2192 1d6 (5) \u2192 1d8 (11) \u2192 1d10 (17)\n" +
	"  Unarmored Movement: +10ft (lvl 2) \u2192 +15ft (6) \u2192 +20ft (10) \u2192 +25ft (14) \u2192 +30ft (18)"

const helpRogue = "\U0001F5E1\uFE0F Rogue Abilities\n" +
	"\n" +
	"  Cunning Action (bonus action, level 2+):\n" +
	"    /bonus cunning-action dash          Dash as bonus action\n" +
	"    /bonus cunning-action disengage     Disengage as bonus action (no OAs this turn)\n" +
	"    /bonus cunning-action hide          Hide as bonus action (Stealth vs passive Perception)\n" +
	"\n" +
	"  Sneak Attack (automatic, once per turn):\n" +
	"    Triggered on hit with finesse or ranged weapon when you have advantage\n" +
	"    OR when an ally is within 5ft of the target (and you don't have disadvantage)\n" +
	"    Damage: 1d6 per 2 Rogue levels (rounded up) \u2014 e.g., 3d6 at level 5\n" +
	"\n" +
	"  Expertise (passive):\n" +
	"    Double proficiency on selected skills \u2014 auto-applied to all checks\n" +
	"\n" +
	"  Uncanny Dodge (lvl 5+, reaction):\n" +
	"    /reaction uncanny-dodge             Halve damage from one attack you can see\n" +
	"    (Prompted automatically when hit by an attack)\n" +
	"\n" +
	"  Evasion (lvl 7+, passive):\n" +
	"    DEX saves: success = no damage, fail = half damage (auto-applied)"

const helpCleric = "\u271D\uFE0F Cleric Abilities\n" +
	"\n" +
	"  Channel Divinity (action, level 2+):\n" +
	"    /action channel-divinity turn-undead    Force undead within 30ft to flee (WIS save)\n" +
	"    /action channel-divinity [subclass]     Use your domain's Channel Divinity option\n" +
	"\n" +
	"    Destroy Undead (lvl 5+): undead below CR threshold are destroyed on failed save\n" +
	"    Uses: 1 (lvl 2) \u2192 2 (lvl 6) \u2192 3 (lvl 18)    Recharge: short rest\n" +
	"\n" +
	"  Spellcasting:\n" +
	"    /cast [spell] [target]       Cast a prepared spell\n" +
	"    /prepare                     Change prepared spells (after long rest)\n" +
	"    /cast [spell] --ritual       Ritual cast without expending a slot (out of combat)\n" +
	"\n" +
	"  Domain spells: always prepared, don't count against your limit (shown separately in /prepare)\n" +
	"\n" +
	"  Use /status to check Channel Divinity uses and active effects"

const helpPaladin = "\u2694\uFE0F Paladin Abilities\n" +
	"\n" +
	"  Channel Divinity (action, level 3+):\n" +
	"    /action channel-divinity [option]       Use your oath's Channel Divinity option\n" +
	"    Uses: 1 (lvl 3) \u2192 2 (lvl 15)           Recharge: short rest\n" +
	"\n" +
	"  Divine Smite (on melee hit, prompted automatically):\n" +
	"    Spend a spell slot for extra radiant damage (2d8 + 1d8 per slot above 1st)\n" +
	"    +1d8 bonus vs undead and fiends \u2014 doubled on crit\n" +
	"\n" +
	"  Lay on Hands (action):\n" +
	"    /action lay-on-hands [target] [hp]      Restore HP from your healing pool\n" +
	"    Pool: 5 \u00d7 Paladin level HP              Recharge: long rest\n" +
	"\n" +
	"  Spellcasting:\n" +
	"    /cast [spell] [target]       Cast a prepared spell\n" +
	"    /prepare                     Change prepared spells (after long rest)\n" +
	"\n" +
	"  Aura of Protection (lvl 6+): you and allies within 10ft add your CHA mod to saves (auto-applied)\n" +
	"  Oath spells: always prepared, don't count against your limit\n" +
	"\n" +
	"  Use /status to check Channel Divinity uses, smite slots, and lay on hands pool"

const helpMetamagic = "\u26A1 Metamagic \u2014 Sorcery Point Options\n" +
	"\n" +
	"Apply Metamagic by adding a flag to /cast:\n" +
	"\n" +
	"  --careful     (1 SP)  Allies in AoE auto-succeed on save\n" +
	"  --distant     (1 SP)  Double spell range (touch \u2192 30ft)\n" +
	"  --empowered   (1 SP)  Reroll up to CHA mod damage dice (combinable)\n" +
	"  --extended    (1 SP)  Double spell duration (max 24h)\n" +
	"  --heightened  (3 SP)  One target has disadvantage on first save\n" +
	"  --quickened   (2 SP)  Cast action spell as bonus action\n" +
	"  --subtle      (1 SP)  No V/S components (bypasses Silence & Counterspell)\n" +
	"  --twinned     (Lvl SP) Second target for single-target spell (1 SP for cantrips)\n" +
	"\n" +
	"Only one option per cast (except --empowered, which stacks).\n" +
	"\n" +
	"Convert resources:\n" +
	"  /bonus font-of-magic convert --slot N   Slot \u2192 SP (gain = slot level)\n" +
	"  /bonus font-of-magic create --level N   SP \u2192 Slot (cost: 1st=2, 2nd=3, 3rd=5, 4th=6, 5th=7)\n" +
	"\n" +
	"Current SP: use /status to check    Recharge: long rest"

// --- Brief help for remaining commands ---

const helpCast = `/cast — Cast a Spell

Usage:
  /cast [spell] [target]              Cast a spell at a target
  /cast [spell] --level N             Upcast at a higher spell slot level
  /cast [spell] --ritual              Ritual cast without expending a slot (out of combat)

Metamagic flags (Sorcerer only): --subtle, --twin, --careful, --heightened, --distant, --quickened, --empowered
Use /help metamagic for full metamagic details.`

const helpMove = `/move — Move Your Character

Usage:
  /move [coordinate]                  Move to a coordinate on the battle map (e.g. D4)

Movement costs are calculated automatically based on terrain and conditions.
Prone characters are prompted to stand or crawl.`

const helpCheck = `/check — Make an Ability or Skill Check

Usage:
  /check [skill]                      Roll a skill or ability check
  /check [skill] --adv                Roll with advantage
  /check [skill] --disadv             Roll with disadvantage
  /check [skill] --target [id]        Check against a specific creature
  /check [skill] --dc [N]             Set a difficulty class

Proficiency, expertise, and modifiers are applied automatically.`

const helpSave = `/save — Make a Saving Throw

Usage:
  /save [ability]                     Roll a saving throw (e.g. /save dex, /save wis)

Proficiency and modifiers are applied automatically.`

const helpRest = `/rest — Take a Rest

Usage:
  /rest short                         Take a short rest (spend hit dice to heal)
  /rest long                          Take a long rest (full HP, restore resources)`

const helpEquip = `/equip — Equip an Item

Usage:
  /equip [item]                       Equip an item from your inventory
  /equip [item] --offhand             Equip in your off-hand
  /equip [item] --armor               Equip as body armor`

const helpInventory = `/inventory — Show Your Inventory

Usage:
  /inventory                          Display all items you are carrying

Shows equipped items, attuned items, and bag contents.`

const helpUse = `/use — Use a Consumable Item

Usage:
  /use [item]                         Use a consumable item (e.g. healing-potion)`

const helpGive = `/give — Give an Item

Usage:
  /give [item] [target]               Give an item to another creature within range`

const helpLoot = `/loot — Loot the Area

Usage:
  /loot                               Loot the area or a defeated creature

A loot pool is presented with claim buttons for each item.`

const helpAttune = `/attune — Attune to a Magic Item

Usage:
  /attune [item]                      Attune to a magic item (requires a short rest, max 3 attunements)`

const helpPrepare = `/prepare — Prepare Spells

Usage:
  /prepare                            Change your prepared spell list (after a long rest)

Domain/oath spells are always prepared and shown separately.`

const helpRetire = `/retire — Retire Your Character

Usage:
  /retire                             Retire your character from the campaign
  /retire --reason [text]             Provide a reason for retirement`

const helpRegister = `/register — Link to a Character

Usage:
  /register [name]                    Link your Discord account to a character your DM created`

const helpImport = `/import — Import from D&D Beyond

Usage:
  /import [ddb-url]                   Import a character from a D&D Beyond character URL`

const helpCreateCharacter = `/create-character — Build a Character

Usage:
  /create-character                   Opens the web portal to build a new character`

const helpCharacter = `/character — Character Sheet

Usage:
  /character                          Show your character sheet summary`

const helpRecap = `/recap — Combat Recap

Usage:
  /recap                              Show a recap of recent combat rounds
  /recap [rounds]                     Show recap for a specific number of rounds`

const helpDistance = `/distance — Measure Distance

Usage:
  /distance [target]                  Measure distance from you to a target
  /distance [target] [target2]        Measure distance between two targets`

const helpWhisper = `/whisper — Private Message to DM

Usage:
  /whisper [message]                  Send a private message visible only to the DM`

const helpStatus = `/status — Character Status

Usage:
  /status                             Show your current HP, conditions, resources, and position`

const helpDone = `/done — End Your Turn

Usage:
  /done                               End your turn and pass to the next combatant

Unused attacks and movement are forfeited.`

const helpDeathsave = `/deathsave — Death Saving Throw

Usage:
  /deathsave                          Roll a death saving throw when at 0 HP

Nat 20: regain 1 HP. Nat 1: two failures. 3 failures: death. 3 successes: stable.`

const helpCommand = `/command — Command a Companion

Usage:
  /command [creature_id] [action]             Command your companion creature
  /command [creature_id] [action] [target]    Command with a target`

const helpReaction = `/reaction — Manage Reactions

Usage:
  /reaction declare [description]     Declare a reaction for this round
  /reaction cancel [description]      Cancel a declared reaction by substring
  /reaction cancel-all                Cancel all active reactions this round`

const helpInteract = `/interact — Object Interaction

Usage:
  /interact [description]             Free object interaction (e.g. draw longsword, open door)`

const helpShove = `/shove — Shove a Creature

Usage:
  /shove [target]                     Shove a target (contested Athletics check)

Push the target 5ft away or knock them prone.`

const helpBonus = `/bonus — Bonus Action

Usage:
  /bonus [action]                     Use a bonus action
  /bonus [action] [args]              Bonus action with additional arguments

Examples: /bonus offhand, /bonus cunning-action dash, /bonus rage, /bonus flurry-of-blows`

const helpFly = `/fly — Fly to Altitude

Usage:
  /fly [altitude]                     Fly to a given altitude in feet (e.g. /fly 30)

Requires a flying speed. Movement cost is calculated automatically.`

const helpUndo = `/undo — Undo Last Action

Usage:
  /undo                               Request to undo your last action
  /undo --reason [text]               Provide a reason for the undo request`

const helpHelpTopic = `/help — Command Help

Usage:
  /help                               Show the full command list
  /help [topic]                       Show detailed help for a command or class

Topics: attack, action, cast, move, check, save, rest, equip, inventory, bonus, done,
        status, ki, rogue, cleric, paladin, metamagic, and more.`

const helpSetup = `/setup — Campaign Setup

Usage:
  /setup                              Create the full channel structure for this campaign

Requires Manage Channels permission. Creates all necessary categories and channels.`
