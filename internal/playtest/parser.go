// Package playtest implements the parser, command-table validator, and
// transcript recorder for the cmd/playtest-player REPL (Phase 121).
//
// Discord platform note: bot accounts cannot invoke slash commands as a
// user (`InteractionCreate` is read-only from the bot's side). The REPL
// therefore parses + validates the player's command, then prints a
// copy-pasteable line for the human operator (or AI agent driving the
// human seat) to paste into Discord. The recorder captures both the
// dispatched command and the bot's response messages observed via the
// gateway, so transcripts replay cleanly through the Phase 120 harness.
package playtest

import (
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
)

// ParsedCommand is the structured form of a slash command line typed at
// the REPL. Args preserves the order the user supplied; named options
// land in NamedArgs when the user uses the `key:value` form.
type ParsedCommand struct {
	Name      string
	Args      []string
	NamedArgs map[string]string
}

// Parse converts a single REPL line like `/move A1` or
// `/attack target:G2 weapon:handaxe` into a ParsedCommand. It returns
// an error for blank input or input missing the leading `/`.
func Parse(line string) (ParsedCommand, error) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return ParsedCommand{}, fmt.Errorf("empty input")
	}
	if !strings.HasPrefix(trimmed, "/") {
		return ParsedCommand{}, fmt.Errorf("commands must start with /")
	}
	fields := strings.Fields(trimmed[1:])
	if len(fields) == 0 {
		return ParsedCommand{}, fmt.Errorf("missing command name")
	}
	cmd := ParsedCommand{Name: fields[0], NamedArgs: map[string]string{}}
	for _, f := range fields[1:] {
		if k, v, ok := strings.Cut(f, ":"); ok && k != "" {
			cmd.NamedArgs[k] = v
			continue
		}
		cmd.Args = append(cmd.Args, f)
	}
	return cmd, nil
}

// CommandTable indexes the bot's ApplicationCommand definitions by name
// for O(1) validation lookups.
type CommandTable struct {
	byName map[string]*discordgo.ApplicationCommand
}

// NewCommandTable builds a CommandTable from the slice returned by the
// bot (`session.ApplicationCommands`) or from the local definitions in
// internal/discord.CommandDefinitions().
func NewCommandTable(defs []*discordgo.ApplicationCommand) *CommandTable {
	t := &CommandTable{byName: make(map[string]*discordgo.ApplicationCommand, len(defs))}
	for _, d := range defs {
		if d == nil || d.Name == "" {
			continue
		}
		t.byName[d.Name] = d
	}
	return t
}

// Names returns the registered command names in stable (alphabetical)
// order. Used by the REPL for the available-commands list and tab
// completion.
func (t *CommandTable) Names() []string {
	out := make([]string, 0, len(t.byName))
	for name := range t.byName {
		out = append(out, name)
	}
	// stable sort without importing sort; small N, insertion sort is fine.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1] > out[j]; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
}

// PlayerCommands is the allow-list of commands the player-agent REPL
// will accept. DM-only commands (e.g. /setup) and admin commands are
// rejected with a clear error. Keeping this list explicit prevents the
// player agent from accidentally driving the DM side of a session.
var PlayerCommands = map[string]struct{}{
	"register":         {},
	"import":           {},
	"create-character": {},
	"move":             {},
	"attack":           {},
	"cast":             {},
	"action":           {},
	"distance":         {},
	"loot":             {},
	"inventory":        {},
	"give":             {},
	"status":           {},
	"done":             {},
	"save":             {},
	"recap":            {},
	"character":        {},
	"fly":              {},
	"bonus":            {},
	"interact":         {},
	"shove":            {},
	"undo":             {},
	"help":             {},
}

// ValidationResult captures the outcome of validating a ParsedCommand
// against a CommandTable + the PlayerCommands allow-list.
type ValidationResult struct {
	OK       bool
	Reason   string
	Required []string // names of required options the user did not supply
}

// Validate checks that the command exists in the table, is in the
// player allow-list, and that all required options are supplied either
// positionally or via `key:value`. Positional args are mapped to
// required options in declaration order.
func Validate(cmd ParsedCommand, table *CommandTable) ValidationResult {
	if _, ok := PlayerCommands[cmd.Name]; !ok {
		return ValidationResult{Reason: fmt.Sprintf("/%s is not a player command", cmd.Name)}
	}
	def, ok := table.byName[cmd.Name]
	if !ok {
		return ValidationResult{Reason: fmt.Sprintf("/%s is not registered with the bot", cmd.Name)}
	}
	var required []string
	for _, opt := range def.Options {
		if opt.Required {
			required = append(required, opt.Name)
		}
	}
	supplied := len(cmd.Args)
	for _, name := range required {
		if _, ok := cmd.NamedArgs[name]; ok {
			supplied++
		}
	}
	if supplied < len(required) {
		missing := required[len(cmd.Args):]
		// trim names already supplied via NamedArgs
		filtered := missing[:0]
		for _, name := range missing {
			if _, ok := cmd.NamedArgs[name]; !ok {
				filtered = append(filtered, name)
			}
		}
		return ValidationResult{Reason: "missing required options", Required: filtered}
	}
	return ValidationResult{OK: true}
}

// Format renders a ParsedCommand back to the canonical
// `/name arg1 key:value` form for paste-into-Discord.
func Format(cmd ParsedCommand) string {
	var b strings.Builder
	b.WriteString("/")
	b.WriteString(cmd.Name)
	for _, a := range cmd.Args {
		b.WriteString(" ")
		b.WriteString(a)
	}
	// stable iteration over named args
	keys := make([]string, 0, len(cmd.NamedArgs))
	for k := range cmd.NamedArgs {
		keys = append(keys, k)
	}
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j-1] > keys[j]; j-- {
			keys[j-1], keys[j] = keys[j], keys[j-1]
		}
	}
	for _, k := range keys {
		b.WriteString(" ")
		b.WriteString(k)
		b.WriteString(":")
		b.WriteString(cmd.NamedArgs[k])
	}
	return b.String()
}
