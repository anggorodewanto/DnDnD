// Command playtest-player is the Phase 121 manual playtest harness for
// the player side of a DnDnD session.
//
// Usage:
//
//	DISCORD_BOT_TOKEN=<player-bot-token> \
//	GUILD_ID=<server-id> \
//	playtest-player [--record transcript.jsonl] [--channel id1,id2]
//
// Behaviour:
//   - Connects to Discord as a bot (a *separate* bot from the dndnd bot;
//     create a second Discord application for it). Loads the bot's
//     registered application commands from the configured guild and
//     uses them as the validation table for the REPL.
//   - Tails MessageCreate events on the configured channels (defaults
//     to every text channel in the guild) and prints them with
//     timestamps + author tags.
//   - Reads slash-command lines from stdin (`/move A1`,
//     `/attack target:G2 weapon:handaxe`, …), parses + validates them,
//     and prints a "PASTE THIS" block for the human operator to paste
//     into Discord.
//
// Why "paste this" and not direct dispatch: bot accounts cannot invoke
// slash commands as users — Discord's `InteractionCreate` is read-only
// from the bot side. The human operator (or AI agent driving the
// human seat) does the paste; the recorder still captures both halves
// because the bot's response messages flow back through the gateway.
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"

	dnddiscord "github.com/ab/dndnd/internal/discord"
	"github.com/ab/dndnd/internal/playtest"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr, openLiveSession, time.Now); err != nil {
		fmt.Fprintln(os.Stderr, "playtest-player:", err)
		os.Exit(1)
	}
}

type config struct {
	appID      string
	guildID    string
	channels   []string
	transcript string
	token      string
}

func parseFlags(args []string, getenv func(string) string) (*config, error) {
	fs := flag.NewFlagSet("playtest-player", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	transcript := fs.String("record", "", "path to a JSON-lines transcript file")
	channelList := fs.String("channel", "", "comma-separated channel IDs to tail (default: every text channel in the guild)")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	token := getenv("DISCORD_BOT_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("DISCORD_BOT_TOKEN must be set")
	}
	appID := getenv("DISCORD_APPLICATION_ID")
	if appID == "" {
		return nil, fmt.Errorf("DISCORD_APPLICATION_ID must be set (the dndnd bot's app ID, used to load its slash commands)")
	}
	guildID := getenv("GUILD_ID")
	if guildID == "" {
		return nil, fmt.Errorf("GUILD_ID must be set")
	}

	var channels []string
	if *channelList != "" {
		for c := range strings.SplitSeq(*channelList, ",") {
			if c = strings.TrimSpace(c); c != "" {
				channels = append(channels, c)
			}
		}
	}

	return &config{
		token:      token,
		appID:      appID,
		guildID:    guildID,
		channels:   channels,
		transcript: *transcript,
	}, nil
}

// sessionClient is the narrow surface the REPL needs from a Discord
// session. The real implementation wraps *discordgo.Session; tests
// substitute a fake.
type sessionClient interface {
	ApplicationCommands(appID, guildID string) ([]*discordgo.ApplicationCommand, error)
	GuildChannels(guildID string) ([]*discordgo.Channel, error)
	AddHandler(handler any) func()
	Close() error
}

func run(
	ctx context.Context,
	args []string,
	stdin io.Reader,
	stdout, stderr io.Writer,
	open func(token string) (sessionClient, error),
	now func() time.Time,
) error {
	cfg, err := parseFlags(args, os.Getenv)
	if err != nil {
		return err
	}
	logger := slog.New(slog.NewTextHandler(stderr, nil))

	sess, err := open(cfg.token)
	if err != nil {
		return err
	}
	defer sess.Close()

	cmds, err := sess.ApplicationCommands(cfg.appID, cfg.guildID)
	if err != nil {
		return fmt.Errorf("load application commands: %w", err)
	}
	if len(cmds) == 0 {
		cmds = dnddiscord.CommandDefinitions()
		logger.Warn("guild had zero registered commands; using local CommandDefinitions() as the validation table")
	}
	table := playtest.NewCommandTable(cmds)

	channels := cfg.channels
	if len(channels) == 0 {
		gcs, err := sess.GuildChannels(cfg.guildID)
		if err != nil {
			return fmt.Errorf("guild channels: %w", err)
		}
		for _, c := range gcs {
			if c.Type == discordgo.ChannelTypeGuildText {
				channels = append(channels, c.ID)
			}
		}
	}
	channelSet := make(map[string]struct{}, len(channels))
	for _, c := range channels {
		channelSet[c] = struct{}{}
	}

	var rec *playtest.Recorder
	if cfg.transcript != "" {
		f, err := os.Create(cfg.transcript)
		if err != nil {
			return fmt.Errorf("open transcript: %w", err)
		}
		defer f.Close()
		rec = playtest.NewRecorder(f, now)
	}

	sess.AddHandler(func(_ any, m *discordgo.MessageCreate) {
		if m == nil || m.Message == nil {
			return
		}
		if _, ok := channelSet[m.ChannelID]; !ok && len(channelSet) > 0 {
			return
		}
		author := "unknown"
		if m.Author != nil {
			author = m.Author.Username
		}
		fmt.Fprintf(stdout, "[%s] #%s @%s: %s\n", now().Format("15:04:05"), m.ChannelID, author, m.Content)
		if rec != nil {
			if err := rec.Observe(m.ChannelID, author, m.Content); err != nil {
				logger.Error("transcript observe failed", "error", err)
			}
		}
	})

	fmt.Fprintf(stdout, "playtest-player ready — %d commands loaded, %d channels tailed\n", len(cmds), len(channels))
	fmt.Fprintln(stdout, "type a slash command (e.g. /move A1) and press enter; ^C to quit")

	dispatchChannel := ""
	if len(channels) > 0 {
		dispatchChannel = channels[0]
	}
	return runREPL(ctx, stdin, stdout, table, rec, dispatchChannel)
}

func runREPL(
	ctx context.Context,
	stdin io.Reader,
	stdout io.Writer,
	table *playtest.CommandTable,
	rec *playtest.Recorder,
	dispatchChannel string,
) error {
	scanner := bufio.NewScanner(stdin)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	for {
		if ctx.Err() != nil {
			return nil
		}
		fmt.Fprint(stdout, "> ")
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return fmt.Errorf("stdin: %w", err)
			}
			return nil
		}
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		if line == "help" || line == "?" {
			fmt.Fprintln(stdout, "available commands:")
			for _, n := range table.Names() {
				if _, ok := playtest.PlayerCommands[n]; ok {
					fmt.Fprintln(stdout, "  /"+n)
				}
			}
			continue
		}
		cmd, err := playtest.Parse(line)
		if err != nil {
			fmt.Fprintln(stdout, "parse error:", err)
			continue
		}
		res := playtest.Validate(cmd, table)
		if !res.OK {
			fmt.Fprintln(stdout, "rejected:", res.Reason)
			if len(res.Required) > 0 {
				fmt.Fprintln(stdout, "  missing:", strings.Join(res.Required, ", "))
			}
			continue
		}
		fmt.Fprintln(stdout, "PASTE THIS into Discord:")
		fmt.Fprintln(stdout, "  "+playtest.Format(cmd))
		if rec != nil {
			if err := rec.Dispatch(dispatchChannel, "playtest-player", cmd); err != nil {
				return fmt.Errorf("transcript dispatch: %w", err)
			}
		}
	}
}
