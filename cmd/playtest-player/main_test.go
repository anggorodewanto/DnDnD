package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/playtest"
)

type fakeSession struct {
	cmds       []*discordgo.ApplicationCommand
	cmdsErr    error
	channels   []*discordgo.Channel
	channelErr error
	handlers   []any
	closed     bool
}

func (f *fakeSession) ApplicationCommands(_, _ string) ([]*discordgo.ApplicationCommand, error) {
	return f.cmds, f.cmdsErr
}

func (f *fakeSession) GuildChannels(_ string) ([]*discordgo.Channel, error) {
	return f.channels, f.channelErr
}

func (f *fakeSession) AddHandler(h any) func() {
	f.handlers = append(f.handlers, h)
	return func() {}
}

func (f *fakeSession) Close() error { f.closed = true; return nil }

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestParseFlags_Defaults(t *testing.T) {
	cfg, err := parseFlags(nil, env(map[string]string{
		"DISCORD_BOT_TOKEN":      "tok",
		"DISCORD_APPLICATION_ID": "app",
		"GUILD_ID":               "g",
	}))
	require.NoError(t, err)
	assert.Equal(t, "tok", cfg.token)
	assert.Equal(t, "app", cfg.appID)
	assert.Equal(t, "g", cfg.guildID)
	assert.Empty(t, cfg.channels)
	assert.Empty(t, cfg.transcript)
}

func TestParseFlags_ChannelsAndRecord(t *testing.T) {
	cfg, err := parseFlags(
		[]string{"--record", "/tmp/x.jsonl", "--channel", "c1, c2 ,c3"},
		env(map[string]string{
			"DISCORD_BOT_TOKEN":      "tok",
			"DISCORD_APPLICATION_ID": "app",
			"GUILD_ID":               "g",
		}),
	)
	require.NoError(t, err)
	assert.Equal(t, []string{"c1", "c2", "c3"}, cfg.channels)
	assert.Equal(t, "/tmp/x.jsonl", cfg.transcript)
}

func TestParseFlags_MissingEnv(t *testing.T) {
	cases := []map[string]string{
		{},
		{"DISCORD_BOT_TOKEN": "tok"},
		{"DISCORD_BOT_TOKEN": "tok", "DISCORD_APPLICATION_ID": "app"},
	}
	for i, c := range cases {
		_, err := parseFlags(nil, env(c))
		assert.Error(t, err, "case %d", i)
	}
}

func TestParseFlags_BadFlag(t *testing.T) {
	_, err := parseFlags([]string{"--nope"}, env(map[string]string{
		"DISCORD_BOT_TOKEN": "t", "DISCORD_APPLICATION_ID": "a", "GUILD_ID": "g",
	}))
	assert.Error(t, err)
}

func TestRunREPL_DispatchAndHelp(t *testing.T) {
	tbl := playtest.NewCommandTable([]*discordgo.ApplicationCommand{
		{Name: "move", Options: []*discordgo.ApplicationCommandOption{
			{Name: "coordinate", Required: true},
		}},
		{Name: "status"},
	})
	var out bytes.Buffer
	var rec bytes.Buffer
	r := playtest.NewRecorder(&rec, func() time.Time {
		return time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC)
	})

	stdin := strings.NewReader(strings.Join([]string{
		"help",
		"/move A1",
		"/move",
		"/setup",
		"/recap",
		"not a slash",
		"",
	}, "\n") + "\n")

	err := runREPL(context.Background(), stdin, &out, tbl, r, "chan-1")
	require.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, "available commands:")
	assert.Contains(t, output, "/move")
	assert.Contains(t, output, "/status")
	assert.Contains(t, output, "PASTE THIS into Discord:")
	assert.Contains(t, output, "/move A1")
	assert.Contains(t, output, "missing required options")
	assert.Contains(t, output, "missing: coordinate")
	assert.Contains(t, output, "not a player command")
	assert.Contains(t, output, "not registered")
	assert.Contains(t, output, "parse error:")

	entries, err := playtest.LoadTranscript(&rec)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "/move A1", entries[0].Command)
	assert.Equal(t, "chan-1", entries[0].ChannelID)
}

func TestRunREPL_NoRecorder(t *testing.T) {
	tbl := playtest.NewCommandTable([]*discordgo.ApplicationCommand{{Name: "status"}})
	var out bytes.Buffer
	err := runREPL(context.Background(), strings.NewReader("/status\n"), &out, tbl, nil, "")
	require.NoError(t, err)
	assert.Contains(t, out.String(), "/status")
}

func TestRunREPL_StopsOnContextCancel(t *testing.T) {
	tbl := playtest.NewCommandTable([]*discordgo.ApplicationCommand{{Name: "status"}})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var out bytes.Buffer
	err := runREPL(ctx, strings.NewReader("/status\n"), &out, tbl, nil, "")
	assert.NoError(t, err)
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("boom") }

func TestRunREPL_ScannerError(t *testing.T) {
	tbl := playtest.NewCommandTable([]*discordgo.ApplicationCommand{{Name: "status"}})
	err := runREPL(context.Background(), errReader{}, io.Discard, tbl, nil, "")
	assert.Error(t, err)
}

func TestRun_FallsBackToLocalCommandsAndDefaultsChannels(t *testing.T) {
	fake := &fakeSession{
		channels: []*discordgo.Channel{
			{ID: "c1", Type: discordgo.ChannelTypeGuildText},
			{ID: "v1", Type: discordgo.ChannelTypeGuildVoice},
		},
	}
	open := func(string) (sessionClient, error) { return fake, nil }
	t.Setenv("DISCORD_BOT_TOKEN", "t")
	t.Setenv("DISCORD_APPLICATION_ID", "a")
	t.Setenv("GUILD_ID", "g")

	var out, errBuf bytes.Buffer
	err := run(context.Background(), nil, strings.NewReader(""), &out, &errBuf, open, time.Now)
	require.NoError(t, err)

	assert.Contains(t, out.String(), "playtest-player ready")
	assert.Contains(t, errBuf.String(), "guild had zero registered commands")
	assert.True(t, fake.closed)
	assert.Len(t, fake.handlers, 1)
}

func TestRun_RecordsObservedMessages(t *testing.T) {
	fake := &fakeSession{
		cmds:     []*discordgo.ApplicationCommand{{Name: "status"}},
		channels: []*discordgo.Channel{{ID: "c1", Type: discordgo.ChannelTypeGuildText}},
	}
	open := func(string) (sessionClient, error) { return fake, nil }
	t.Setenv("DISCORD_BOT_TOKEN", "t")
	t.Setenv("DISCORD_APPLICATION_ID", "a")
	t.Setenv("GUILD_ID", "g")

	transcript := filepath.Join(t.TempDir(), "t.jsonl")

	// io.Pipe so the test can inject MessageCreate events while the REPL is
	// still running (before stdin EOF triggers the deferred file close).
	pr, pw := io.Pipe()
	defer pr.Close()

	type result struct {
		err error
	}
	done := make(chan result, 1)
	var out, errBuf bytes.Buffer
	go func() {
		done <- result{err: run(
			context.Background(),
			[]string{"--record", transcript, "--channel", "c1"},
			pr,
			&out, &errBuf,
			open,
			func() time.Time { return time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC) },
		)}
	}()

	// Wait for the handler to be registered before injecting events.
	for i := 0; i < 100 && len(fake.handlers) == 0; i++ {
		time.Sleep(2 * time.Millisecond)
	}
	require.Len(t, fake.handlers, 1)
	handler := fake.handlers[0].(func(any, *discordgo.MessageCreate))

	handler(nil, &discordgo.MessageCreate{Message: &discordgo.Message{
		ChannelID: "c1",
		Content:   "Aria moved to A1.",
		Author:    &discordgo.User{Username: "DnDnD"},
	}})
	handler(nil, &discordgo.MessageCreate{Message: &discordgo.Message{
		ChannelID: "off-channel",
		Content:   "ignored",
		Author:    &discordgo.User{Username: "stranger"},
	}})
	handler(nil, nil)
	handler(nil, &discordgo.MessageCreate{Message: nil})

	_, _ = pw.Write([]byte("/status\n"))
	_ = pw.Close()

	res := <-done
	require.NoError(t, res.err)

	data, err := os.ReadFile(transcript)
	require.NoError(t, err)
	entries, err := playtest.LoadTranscript(bytes.NewReader(data))
	require.NoError(t, err)
	require.Len(t, entries, 2)
	assert.Equal(t, playtest.DirectionObserved, entries[0].Direction)
	assert.Equal(t, "Aria moved to A1.", entries[0].Content)
	assert.Equal(t, playtest.DirectionDispatch, entries[1].Direction)
	assert.Equal(t, "/status", entries[1].Command)
}

func TestRun_OpenFails(t *testing.T) {
	t.Setenv("DISCORD_BOT_TOKEN", "t")
	t.Setenv("DISCORD_APPLICATION_ID", "a")
	t.Setenv("GUILD_ID", "g")
	open := func(string) (sessionClient, error) { return nil, errors.New("no") }
	err := run(context.Background(), nil, strings.NewReader(""), io.Discard, io.Discard, open, time.Now)
	assert.Error(t, err)
}

func TestRun_ApplicationCommandsError(t *testing.T) {
	t.Setenv("DISCORD_BOT_TOKEN", "t")
	t.Setenv("DISCORD_APPLICATION_ID", "a")
	t.Setenv("GUILD_ID", "g")
	fake := &fakeSession{cmdsErr: errors.New("nope")}
	err := run(context.Background(), nil, strings.NewReader(""), io.Discard, io.Discard,
		func(string) (sessionClient, error) { return fake, nil }, time.Now)
	assert.Error(t, err)
}

func TestRun_GuildChannelsError(t *testing.T) {
	t.Setenv("DISCORD_BOT_TOKEN", "t")
	t.Setenv("DISCORD_APPLICATION_ID", "a")
	t.Setenv("GUILD_ID", "g")
	fake := &fakeSession{cmds: []*discordgo.ApplicationCommand{{Name: "status"}}, channelErr: errors.New("x")}
	err := run(context.Background(), nil, strings.NewReader(""), io.Discard, io.Discard,
		func(string) (sessionClient, error) { return fake, nil }, time.Now)
	assert.Error(t, err)
}

func TestRun_BadTranscriptPath(t *testing.T) {
	t.Setenv("DISCORD_BOT_TOKEN", "t")
	t.Setenv("DISCORD_APPLICATION_ID", "a")
	t.Setenv("GUILD_ID", "g")
	fake := &fakeSession{cmds: []*discordgo.ApplicationCommand{{Name: "status"}}}
	err := run(
		context.Background(),
		[]string{"--record", "/nonexistent-dir/abc/x.jsonl"},
		strings.NewReader(""), io.Discard, io.Discard,
		func(string) (sessionClient, error) { return fake, nil }, time.Now,
	)
	assert.Error(t, err)
}

func TestRun_FlagParseError(t *testing.T) {
	t.Setenv("DISCORD_BOT_TOKEN", "t")
	t.Setenv("DISCORD_APPLICATION_ID", "a")
	t.Setenv("GUILD_ID", "g")
	err := run(context.Background(), []string{"--nope"}, strings.NewReader(""), io.Discard, io.Discard, nil, time.Now)
	assert.Error(t, err)
}
