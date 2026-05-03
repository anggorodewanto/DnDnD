package main

// This file holds the thin *discordgo.Session adapter. It is excluded
// from coverage (see Makefile COVER_EXCLUDE) because every method is a
// straight delegation that only exercises behaviour through a real
// Discord gateway; the meaningful logic in main.go is exercised against
// the fakeSession in main_test.go.

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
)

type liveSession struct{ s *discordgo.Session }

func (l *liveSession) ApplicationCommands(appID, guildID string) ([]*discordgo.ApplicationCommand, error) {
	return l.s.ApplicationCommands(appID, guildID)
}

func (l *liveSession) GuildChannels(guildID string) ([]*discordgo.Channel, error) {
	return l.s.GuildChannels(guildID)
}

func (l *liveSession) AddHandler(h any) func() { return l.s.AddHandler(h) }
func (l *liveSession) Close() error            { return l.s.Close() }

func openLiveSession(token string) (sessionClient, error) {
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, fmt.Errorf("discordgo.New: %w", err)
	}
	dg.Identify.Intents |= discordgo.IntentsGuildMessages | discordgo.IntentsDirectMessages | discordgo.IntentMessageContent
	if err := dg.Open(); err != nil {
		return nil, fmt.Errorf("discord open: %w", err)
	}
	return &liveSession{s: dg}, nil
}
