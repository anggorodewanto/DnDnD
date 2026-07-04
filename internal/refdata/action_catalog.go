package refdata

import "strings"

// ActionEconomy classifies what a character spends to take an action on their
// turn: the single Action, the single Bonus Action, the single Reaction, or a
// "free" slot (a free object interaction / dropping prone — things that cost no
// action). It mirrors the resource constants the combat engine tracks per Turn
// (internal/combat/turnresources.go).
type ActionEconomy string

const (
	EconomyAction      ActionEconomy = "action"
	EconomyBonusAction ActionEconomy = "bonus action"
	EconomyReaction    ActionEconomy = "reaction"
	EconomyFree        ActionEconomy = "free"
)

// ActionEconomyOrder is the display order for grouping actions on the character
// sheet. Single source so the portal and any future consumer group the same way.
var ActionEconomyOrder = []ActionEconomy{
	EconomyAction,
	EconomyBonusAction,
	EconomyReaction,
	EconomyFree,
}

// ActionCatalogEntry is one thing a character can do on their turn — the SSOT
// shared by the Discord command dispatch (which accepts these by Key) and the
// portal character sheet (which lists them as "Possible Actions" guidance).
//
// Why this exists: before this catalog, "what can a character do on a turn" was
// implied only by the imperative /action and /bonus dispatch switches
// (internal/discord/{action,bonus}_handler.go) and the player had no at-a-glance
// reference. Surfacing it on the sheet risks drift — the sheet advertising a
// command the bot rejects, or omitting a class ability the bot supports. The
// contract test TestActionCatalog_MatchesDiscordDispatch (internal/discord)
// pins the catalog to the dispatch key sets so that drift fails CI, the same way
// the item catalog's contract test guards equipment ids (ISSUE-013/-017).
//
// The catalog is reference/guidance, not live turn state: it answers "what CAN
// this character do?", never "what has this character spent this turn?" (that
// lives in the Turn row during an active encounter, which the read-only sheet
// has no access to).
type ActionCatalogEntry struct {
	// Key is the canonical, lower-case subcommand key. For entries dispatched
	// via /action or /bonus it matches the dispatch key (e.g. "dash", "rage")
	// so the contract test can pin them 1:1.
	Key string `json:"key"`
	// Name is the player-facing label, e.g. "Dash", "Cunning Action".
	Name string `json:"name"`
	// Economy is the turn resource this action spends.
	Economy ActionEconomy `json:"economy"`
	// Command is how the player invokes it in Discord, e.g. "/action dash".
	// A handful of always-available actions point at other commands ("/attack",
	// "/cast", "/shove") or are resolved automatically ("(automatic)").
	Command string `json:"command"`
	// Summary is a one-line description for the sheet.
	Summary string `json:"summary"`
	// Universal is true when any character can take the action (gated only by
	// situation or equipment, not class). Universal entries carry no Classes.
	Universal bool `json:"universal"`
	// Classes lists the lower-case class slugs that grant a non-universal
	// action. Matched case-insensitively against the character's classes.
	Classes []string `json:"classes,omitempty"`
	// MinLevel is the class level at which the action is gained (0 = level 1 /
	// any level of the listed class). For multi-class entries it is the lowest
	// level across the listed classes; display gating is guidance, not a precise
	// per-class gate.
	MinLevel int `json:"min_level,omitempty"`
}

// actionCatalog is the canonical, ordered list. Declaration order is the
// display order within each economy group. Keys for /action and /bonus entries
// MUST match the Discord dispatch keys; the contract test enforces it.
//
// Classes / MinLevel are hand-authored gating used only for display (which
// abilities to show on the sheet). They mirror the class feature levels seeded
// in seed_classes.go (e.g. Rage at barbarian 1, Cunning Action at rogue 2) and
// should be kept in step with it when features change — only the Keys are
// pinned by the contract test, not these gating values.
var actionCatalog = []ActionCatalogEntry{
	// --- Universal actions (everyone, every turn) ---
	{Key: "attack", Name: "Attack", Economy: EconomyAction, Command: "/attack <target> [weapon]", Universal: true,
		Summary: "Make a melee or ranged attack with an equipped weapon (Extra Attack grants more)."},
	{Key: "cast-spell", Name: "Cast a Spell", Economy: EconomyAction, Command: "/cast <spell> [target]", Universal: true,
		Summary: "Cast a spell with a casting time of 1 action (requires known/prepared spells)."},
	{Key: "dash", Name: "Dash", Economy: EconomyAction, Command: "/action dash", Universal: true,
		Summary: "Gain extra movement equal to your speed for the turn."},
	{Key: "disengage", Name: "Disengage", Economy: EconomyAction, Command: "/action disengage", Universal: true,
		Summary: "Your movement doesn't provoke opportunity attacks this turn."},
	{Key: "dodge", Name: "Dodge", Economy: EconomyAction, Command: "/action dodge", Universal: true,
		Summary: "Attacks against you have disadvantage; you make Dexterity saves with advantage."},
	{Key: "help", Name: "Help", Economy: EconomyAction, Command: "/action help <ally> [target]", Universal: true,
		Summary: "Give an ally advantage on their next ability check or attack against an adjacent foe."},
	{Key: "hide", Name: "Hide", Economy: EconomyAction, Command: "/action hide", Universal: true,
		Summary: "Make a Dexterity (Stealth) check to become hidden (requires cover or obscurement)."},
	{Key: "grapple", Name: "Grapple", Economy: EconomyAction, Command: "/action grapple <target>", Universal: true,
		Summary: "Make a contested Athletics check to seize a creature."},
	{Key: "shove", Name: "Shove", Economy: EconomyAction, Command: "/shove <target> [prone|push]", Universal: true,
		Summary: "Contested check to push a creature 5 ft away or knock it prone."},
	{Key: "escape", Name: "Escape", Economy: EconomyAction, Command: "/action escape", Universal: true,
		Summary: "Make a contested check to break free of a grapple."},
	{Key: "ready", Name: "Ready", Economy: EconomyAction, Command: "/action ready <trigger>", Universal: true,
		Summary: "Prepare an action now to trigger as a reaction when a condition you name occurs."},
	{Key: "stabilize", Name: "Stabilize", Economy: EconomyAction, Command: "/action stabilize <target>", Universal: true,
		Summary: "Make a DC 10 Medicine check to stabilize a dying creature within reach."},

	// --- Universal bonus action (gated by equipment, not class) ---
	{Key: "offhand", Name: "Off-Hand Attack", Economy: EconomyBonusAction, Command: "/bonus offhand <target>", Universal: true,
		Summary: "When two-weapon fighting with light melee weapons, attack with your other weapon."},

	// --- Universal reactions ---
	{Key: "opportunity-attack", Name: "Opportunity Attack", Economy: EconomyReaction, Command: "(automatic)", Universal: true,
		Summary: "Strike a creature that leaves your reach — resolved automatically by the engine."},
	{Key: "reaction-declare", Name: "Declare a Reaction", Economy: EconomyReaction, Command: "/reaction declare <description>", Universal: true,
		Summary: "Declare a custom reaction or the trigger you're watching for this round."},

	// --- Universal free / movement options ---
	{Key: "interact", Name: "Interact with an Object", Economy: EconomyFree, Command: "/interact <description>", Universal: true,
		Summary: "One free object interaction per turn (draw a weapon, open a door, pick up an item)."},
	{Key: "stand", Name: "Stand Up", Economy: EconomyFree, Command: "/action stand", Universal: true,
		Summary: "Stand up from prone, spending half your movement."},
	{Key: "drop-prone", Name: "Drop Prone", Economy: EconomyFree, Command: "/action drop-prone", Universal: true,
		Summary: "Drop to the ground prone — no action required."},

	// --- Class actions ---
	{Key: "surge", Name: "Action Surge", Economy: EconomyAction, Command: "/action surge", Classes: []string{"fighter"}, MinLevel: 2,
		Summary: "Take one additional action on your turn (once per short rest)."},
	{Key: "channel-divinity", Name: "Channel Divinity", Economy: EconomyAction, Command: "/action channel-divinity <option>", Classes: []string{"cleric", "paladin"}, MinLevel: 2,
		Summary: "Channel divine energy for a special effect (Turn Undead, Sacred Weapon, …)."},
	{Key: "lay-on-hands", Name: "Lay on Hands", Economy: EconomyAction, Command: "/action lay-on-hands <target> <hp>", Classes: []string{"paladin"}, MinLevel: 1,
		Summary: "Spend from your pool of healing to restore HP, or cure poison/disease."},

	// --- Class bonus actions ---
	{Key: "rage", Name: "Rage", Economy: EconomyBonusAction, Command: "/bonus rage", Classes: []string{"barbarian"}, MinLevel: 1,
		Summary: "Enter a rage: bonus melee damage and resistance to bludgeoning/piercing/slashing."},
	{Key: "cunning-action", Name: "Cunning Action", Economy: EconomyBonusAction, Command: "/bonus cunning-action <dash|disengage|hide>", Classes: []string{"rogue"}, MinLevel: 2,
		Summary: "Take Dash, Disengage, or Hide as a bonus action."},
	{Key: "martial-arts", Name: "Martial Arts", Economy: EconomyBonusAction, Command: "/bonus martial-arts <target>", Classes: []string{"monk"}, MinLevel: 1,
		Summary: "Make an unarmed strike as a bonus action after you Attack."},
	{Key: "flurry", Name: "Flurry of Blows", Economy: EconomyBonusAction, Command: "/bonus flurry <target>", Classes: []string{"monk"}, MinLevel: 2,
		Summary: "Spend 1 ki to make two unarmed strikes as a bonus action after you Attack."},
	{Key: "step-of-the-wind", Name: "Step of the Wind", Economy: EconomyBonusAction, Command: "/bonus step-of-the-wind <dash|disengage>", Classes: []string{"monk"}, MinLevel: 2,
		Summary: "Spend 1 ki to Dash or Disengage as a bonus action; your jump distance doubles."},
	{Key: "patient-defense", Name: "Patient Defense", Economy: EconomyBonusAction, Command: "/bonus patient-defense", Classes: []string{"monk"}, MinLevel: 2,
		Summary: "Spend 1 ki to take the Dodge action as a bonus action."},
	{Key: "font-of-magic", Name: "Font of Magic", Economy: EconomyBonusAction, Command: "/bonus font-of-magic <convert|create> <level>", Classes: []string{"sorcerer"}, MinLevel: 2,
		Summary: "Convert sorcery points into a spell slot, or a spell slot into sorcery points."},
	{Key: "bardic-inspiration", Name: "Bardic Inspiration", Economy: EconomyBonusAction, Command: "/bonus bardic-inspiration <ally>", Classes: []string{"bard"}, MinLevel: 1,
		Summary: "Give an ally a Bardic Inspiration die to add to a later roll."},
	{Key: "wild-shape", Name: "Wild Shape", Economy: EconomyBonusAction, Command: "/bonus wild-shape <beast>", Classes: []string{"druid"}, MinLevel: 2,
		Summary: "Transform into a beast you have seen before."},
	{Key: "second-wind", Name: "Second Wind", Economy: EconomyBonusAction, Command: "/bonus second-wind", Classes: []string{"fighter"}, MinLevel: 1,
		Summary: "Regain 1d10 + your fighter level HP as a bonus action (once per short rest)."},
}

// ActionCount is the number of rows in the canonical action catalog. Derived so
// it never drifts from the data (mirrors ItemCount).
var ActionCount = len(actionCatalog)

// ActionCatalog returns the full canonical action catalog in display order.
func ActionCatalog() []ActionCatalogEntry {
	out := make([]ActionCatalogEntry, len(actionCatalog))
	copy(out, actionCatalog)
	return out
}

// ActionCatalogByKey returns the catalog keyed by Key for O(1) lookups.
func ActionCatalogByKey() map[string]ActionCatalogEntry {
	entries := ActionCatalog()
	byKey := make(map[string]ActionCatalogEntry, len(entries))
	for _, e := range entries {
		byKey[e.Key] = e
	}
	return byKey
}

// AvailableActions returns the catalog entries a character can take, given a map
// of lower-or-mixed-case class slug → that character's level in the class.
// Universal entries are always included; class-gated entries are included only
// when the character has one of the listed classes at or above MinLevel.
// Catalog (display) order is preserved.
func AvailableActions(classLevels map[string]int) []ActionCatalogEntry {
	// Normalize the caller's class keys to lower-case once.
	levels := make(map[string]int, len(classLevels))
	for class, lvl := range classLevels {
		levels[strings.ToLower(class)] = lvl
	}

	var out []ActionCatalogEntry
	for _, e := range actionCatalog {
		if e.Universal || classGrantsAction(e, levels) {
			out = append(out, e)
		}
	}
	return out
}

// classGrantsAction reports whether one of the character's classes meets the
// entry's class + min-level gate.
func classGrantsAction(e ActionCatalogEntry, levels map[string]int) bool {
	for _, class := range e.Classes {
		if lvl, ok := levels[class]; ok && lvl >= e.MinLevel {
			return true
		}
	}
	return false
}
