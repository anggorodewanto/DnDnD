package discord

// This file is the single in-package source for the /action and /bonus
// subcommand key sets. isDispatchSubcommand (action_handler.go) and the /bonus
// "unknown action" hint (bonus_handler.go) both derive from it, and the
// action-catalog contract test (action_catalog_contract_test.go) pins these
// keys to refdata.ActionCatalog so the portal "Possible Actions" list can never
// drift from what the bot actually accepts (see ActionCatalogEntry doc).

// actionSubcommandKeys are the canonical /action subcommands routed to a
// dedicated combat service (everything else is freeform / ready / cancel).
// Aliases (e.g. "dropprone") are resolved to these via actionSubcommandAliases.
var actionSubcommandKeys = []string{
	"surge", "dash", "disengage", "dodge", "help", "hide",
	"stand", "drop-prone", "escape", "grapple",
	"channel-divinity", "lay-on-hands", "stabilize",
}

// actionSubcommandAliases maps accepted alternate spellings to their canonical
// key, preserving the historical accept-set of isDispatchSubcommand.
var actionSubcommandAliases = map[string]string{
	"action-surge":    "surge",
	"dropprone":       "drop-prone",
	"channeldivinity": "channel-divinity",
	"layonhands":      "lay-on-hands",
}

// bonusSubcommandKeys are the canonical /bonus subcommands. Order is the order
// shown in the /bonus "unknown action" hint. Some are lifecycle/secondary
// (end-rage, revert-wild-shape, drag, release-drag) and are not player-facing
// "abilities" on the sheet — the contract test documents those exclusions.
var bonusSubcommandKeys = []string{
	"offhand", "rage", "end-rage", "martial-arts", "polearm", "crossbow", "step-of-the-wind",
	"patient-defense", "font-of-magic", "lay-on-hands", "bardic-inspiration",
	"second-wind", "wild-shape", "revert-wild-shape", "flurry", "cunning-action",
	"drag", "release-drag",
}

// canonicalActionSubcommand resolves a normalized /action subcommand name to
// its canonical key, or "" when the name does not route to a dispatch service.
func canonicalActionSubcommand(sub string) string {
	for _, k := range actionSubcommandKeys {
		if sub == k {
			return k
		}
	}
	if canon, ok := actionSubcommandAliases[sub]; ok {
		return canon
	}
	return ""
}
