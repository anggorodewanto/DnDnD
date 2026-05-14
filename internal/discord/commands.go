package discord

import (
	"fmt"

	"github.com/bwmarrin/discordgo"

	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/refdata"
)

// incapacitatedRejection returns ("", false) if the combatant can issue
// turn-consuming commands (move, attack, cast, bonus, action). Returns the
// formatted ephemeral rejection string and true when the combatant is
// dying, unconscious, stunned, paralyzed, or otherwise incapacitated.
//
// C-43-block-commands / Phase 43: a dying or incapacitated combatant cannot
// take actions on their own behalf. Initiative auto-skips them in combat
// mode, but a dying PC can still try to fire off /move or /attack out of
// order — this guard rejects those before any service call runs.
//
// `/deathsave` and the off-turn DM-side commands intentionally never
// consult this guard.
func incapacitatedRejection(c refdata.Combatant) (string, bool) {
	if msg, ok := dyingRejection(c); ok {
		return msg, true
	}
	if !combat.IsIncapacitatedRaw(c.Conditions) {
		return "", false
	}
	return "You are incapacitated and cannot take that action.", true
}

// dyingRejection returns the "you are dying" rejection when the combatant
// is alive at 0 HP and not yet stabilized. Split from incapacitatedRejection
// so the dying message is more specific than the generic incapacitated copy.
func dyingRejection(c refdata.Combatant) (string, bool) {
	ds, err := combat.ParseDeathSaves(c.DeathSaves.RawMessage)
	if err != nil {
		return "", false
	}
	if !combat.IsDying(c.IsAlive, int(c.HpCurrent), ds) {
		return "", false
	}
	return "You are dying — only `/deathsave` is available until you stabilize.", true
}

// setupPermission requires ManageChannels to run /setup.
var setupPermission int64 = discordgo.PermissionManageChannels

// CommandDefinitions returns the full set of slash commands the bot registers per guild.
func CommandDefinitions() []*discordgo.ApplicationCommand {
	return []*discordgo.ApplicationCommand{
		// --- Movement ---
		{
			Name:        "move",
			Description: "Move your character to a coordinate on the battle map",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "coordinate",
					Description: "Target coordinate (e.g. D4)",
					Required:    true,
				},
			},
		},
		{
			Name:        "fly",
			Description: "Fly to a given altitude in feet",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionInteger,
					Name:        "altitude",
					Description: "Altitude in feet (e.g. 30)",
					Required:    true,
				},
			},
		},
		// --- Combat ---
		{
			Name:        "attack",
			Description: "Attack a target with a weapon",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "target",
					Description: "Target coordinate or creature ID (e.g. G2)",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "weapon",
					Description: "Weapon to use (e.g. handaxe)",
				},
				{
					Type:        discordgo.ApplicationCommandOptionBoolean,
					Name:        "gwm",
					Description: "Use Great Weapon Master (-5/+10)",
				},
				{
					Type:        discordgo.ApplicationCommandOptionBoolean,
					Name:        "sharpshooter",
					Description: "Use Sharpshooter (-5/+10, ranged)",
				},
				{
					Type:        discordgo.ApplicationCommandOptionBoolean,
					Name:        "reckless",
					Description: "Use Reckless Attack (Barbarian)",
				},
				{
					Type:        discordgo.ApplicationCommandOptionBoolean,
					Name:        "twohanded",
					Description: "Use two-handed grip (versatile)",
				},
				{
					Type:        discordgo.ApplicationCommandOptionBoolean,
					Name:        "offhand",
					Description: "Bonus-action off-hand attack (TWF)",
				},
				{
					Type:        discordgo.ApplicationCommandOptionBoolean,
					Name:        "thrown",
					Description: "Throw a melee weapon with the thrown property",
				},
				{
					Type:        discordgo.ApplicationCommandOptionBoolean,
					Name:        "improvised",
					Description: "Improvised weapon (1d4 bludgeoning)",
				},
			},
		},
		{
			Name:        "cast",
			Description: "Cast a spell",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "spell",
					Description: "Spell name (e.g. fireball)",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "target",
					Description: "Target coordinate or creature ID",
				},
				{
					Type:        discordgo.ApplicationCommandOptionInteger,
					Name:        "level",
					Description: "Spell slot level to cast at",
				},
				{
					Type:        discordgo.ApplicationCommandOptionBoolean,
					Name:        "subtle",
					Description: "Use Subtle Spell metamagic",
				},
				{
					Type:        discordgo.ApplicationCommandOptionBoolean,
					Name:        "twin",
					Description: "Use Twinned Spell metamagic",
				},
				{
					Type:        discordgo.ApplicationCommandOptionBoolean,
					Name:        "careful",
					Description: "Use Careful Spell metamagic",
				},
				{
					Type:        discordgo.ApplicationCommandOptionBoolean,
					Name:        "heightened",
					Description: "Use Heightened Spell metamagic",
				},
				{
					Type:        discordgo.ApplicationCommandOptionBoolean,
					Name:        "distant",
					Description: "Use Distant Spell metamagic",
				},
				{
					Type:        discordgo.ApplicationCommandOptionBoolean,
					Name:        "quickened",
					Description: "Use Quickened Spell metamagic",
				},
				{
					Type:        discordgo.ApplicationCommandOptionBoolean,
					Name:        "empowered",
					Description: "Use Empowered Spell metamagic",
				},
				{
					Type:        discordgo.ApplicationCommandOptionBoolean,
					Name:        "extended",
					Description: "Use Extended Spell metamagic (doubles spell duration up to 24h)",
				},
				{
					Type:        discordgo.ApplicationCommandOptionBoolean,
					Name:        "spell-slot",
					Description: "Force using a regular spell slot instead of pact magic slot (multiclass warlocks)",
				},
				{
					Type:        discordgo.ApplicationCommandOptionBoolean,
					Name:        "ritual",
					Description: "Cast as a ritual (no slot consumed; out of combat only)",
				},
			},
		},
		{
			Name:        "bonus",
			Description: "Use a bonus action",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "action",
					Description: "Bonus action to take (e.g. cunning-action dash, rage)",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "args",
					Description: "Additional arguments",
				},
			},
		},
		{
			Name:        "action",
			Description: "Use an action on your turn",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "action",
					Description: "Action to take (e.g. grapple, dash, ready)",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "args",
					Description: "Additional arguments (e.g. target, readied action)",
				},
				// E-71-followup-discord-ready-spell-flags: optional spell
				// + slot for `/action ready`. The service-side ReadyAction
				// expends the slot at ready-time and sets concentration when
				// these are present. Both are no-ops for non-spell readies.
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "spell",
					Description: "Spell name to ready (slash-cast on the trigger)",
				},
				{
					Type:        discordgo.ApplicationCommandOptionInteger,
					Name:        "slot",
					Description: "Spell slot level to expend at ready-time (1-9)",
				},
			},
		},
		{
			Name:        "shove",
			Description: "Shove a target creature (push, prone, or grapple)",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "target",
					Description: "Target creature ID (e.g. OS)",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "mode",
					Description: "Shove mode: push (default), prone, or grapple",
				},
			},
		},
		{
			Name:        "interact",
			Description: "Interact with an object (free object interaction)",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "description",
					Description: "What you do (e.g. draw longsword)",
					Required:    true,
				},
			},
		},
		{
			Name:        "done",
			Description: "End your turn",
		},
		{
			Name:        "deathsave",
			Description: "Roll a death saving throw",
		},
		{
			Name:        "command",
			Description: "Command a companion creature",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "creature_id",
					Description: "Creature identifier (e.g. FAM)",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "action",
					Description: "Action for the creature (e.g. attack)",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "target",
					Description: "Target for the action (e.g. G1)",
				},
			},
		},
		{
			Name:        "reaction",
			Description: "Declare, cancel, or clear reactions for this round",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "declare",
					Description: "Declare a reaction for this round",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "description",
							Description: "Reaction description (e.g. Shield if I get hit)",
							Required:    true,
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "cancel",
					Description: "Cancel a declared reaction by description substring",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "description",
							Description: "Substring of the reaction description to cancel",
							Required:    true,
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "cancel-all",
					Description: "Cancel all your active reactions this round",
				},
			},
		},
		// --- Checks & Saves ---
		{
			Name:        "check",
			Description: "Make an ability or skill check",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "skill",
					Description: "Skill or ability to check (e.g. perception, medicine)",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionBoolean,
					Name:        "adv",
					Description: "Roll with advantage",
				},
				{
					Type:        discordgo.ApplicationCommandOptionBoolean,
					Name:        "disadv",
					Description: "Roll with disadvantage",
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "target",
					Description: "Target creature ID (e.g. AR)",
				},
				{
					Type:        discordgo.ApplicationCommandOptionInteger,
					Name:        "dc",
					Description: "Difficulty Class — used for trivial nat 1/nat 20 outcomes",
				},
			},
		},
		{
			Name:        "save",
			Description: "Make a saving throw",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "ability",
					Description: "Ability to save with (e.g. dex, wis)",
					Required:    true,
				},
			},
		},
		{
			Name:        "rest",
			Description: "Take a short or long rest",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "type",
					Description: "Rest type: short or long",
					Required:    true,
				},
			},
		},
		// --- Communication ---
		{
			Name:        "whisper",
			Description: "Send a private message to the DM",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "message",
					Description: "Your private message",
					Required:    true,
				},
			},
		},
		// --- Status & Inventory ---
		{
			Name:        "status",
			Description: "Show your character's current status",
		},
		{
			Name:        "equip",
			Description: "Equip an item",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "item",
					Description: "Item to equip (e.g. longsword)",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionBoolean,
					Name:        "offhand",
					Description: "Equip in off-hand",
				},
				{
					Type:        discordgo.ApplicationCommandOptionBoolean,
					Name:        "armor",
					Description: "Equip as body armor",
				},
			},
		},
		{
			Name:        "undo",
			Description: "Request to undo your last action",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "reason",
					Description: "Reason for undo",
				},
			},
		},
		{
			Name:        "inventory",
			Description: "Show your character's inventory",
		},
		{
			Name:        "use",
			Description: "Use a consumable item",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "item",
					Description: "Item to use (e.g. healing-potion)",
					Required:    true,
				},
			},
		},
		{
			Name:        "give",
			Description: "Give an item to another creature",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "item",
					Description: "Item to give (e.g. healing-potion)",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "target",
					Description: "Recipient creature ID (e.g. AR)",
					Required:    true,
				},
			},
		},
		{
			Name:        "loot",
			Description: "Loot the area or a defeated creature",
		},
		{
			Name:        "attune",
			Description: "Attune to a magic item",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "item",
					Description: "Item to attune to (e.g. cloak-of-protection)",
					Required:    true,
				},
			},
		},
		{
			Name:        "unattune",
			Description: "End attunement with a magic item",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "item",
					Description: "Item to unattune from (e.g. cloak-of-protection)",
					Required:    true,
				},
			},
		},
		{
			Name:        "prepare",
			Description: "Prepare your spell list for the day",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "spells",
					Description: "Comma-separated spell IDs to prepare (omit to preview)",
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "class",
					Description: "Class to prepare for (multiclass; defaults to first prepared caster class)",
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "subclass",
					Description: "Subclass override (defaults to character's subclass)",
				},
			},
		},
		{
			Name:        "retire",
			Description: "Retire your character from the campaign",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "reason",
					Description: "Reason for retirement",
				},
			},
		},
		// --- Character Management (existing) ---
		{
			Name:        "register",
			Description: "Link to a character your DM already created",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "name",
					Description: "Character name",
					Required:    true,
				},
			},
		},
		{
			Name:        "import",
			Description: "Import a character from D&D Beyond",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "ddb-url",
					Description: "D&D Beyond character URL",
					Required:    true,
				},
			},
		},
		{
			Name:        "create-character",
			Description: "Build a character in the web portal",
		},
		{
			Name:        "character",
			Description: "Show your character sheet summary",
		},
		{
			Name:        "recap",
			Description: "Show a recap of recent combat rounds",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionInteger,
					Name:        "rounds",
					Description: "Number of rounds to recap",
				},
			},
		},
		{
			Name:        "distance",
			Description: "Measure distance between targets on the battle map",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "target",
					Description: "First target coordinate or creature ID (e.g. G1)",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "target2",
					Description: "Second target (defaults to your position)",
				},
			},
		},
		// --- Utility (existing) ---
		{
			Name:        "help",
			Description: "Show a full command list",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "topic",
					Description: "Command or class to get help on (e.g. attack, rogue, ki)",
				},
			},
		},
		{
			Name:                     "setup",
			Description:              "Create the full channel structure for this campaign",
			DefaultMemberPermissions: &setupPermission,
		},
	}
}

// RegisterCommands registers the current command set for a guild.
// ApplicationCommandBulkOverwrite atomically replaces all commands, removing stale ones.
func RegisterCommands(s Session, appID, guildID string) error {
	defs := CommandDefinitions()
	_, err := s.ApplicationCommandBulkOverwrite(appID, guildID, defs)
	if err != nil {
		return fmt.Errorf("bulk overwriting commands for guild %s: %w", guildID, err)
	}
	return nil
}
