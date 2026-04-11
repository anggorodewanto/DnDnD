package main

import (
	"errors"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/stretchr/testify/assert"
)

func TestNewDMQueueChannelResolver_FindsByName(t *testing.T) {
	sess := &testSession{
		guildChannelsFunc: func(string) ([]*discordgo.Channel, error) {
			return []*discordgo.Channel{
				{ID: "100", Name: "general"},
				{ID: "200", Name: "dm-queue"},
				{ID: "300", Name: "the-story"},
			}, nil
		},
	}
	resolve := newDMQueueChannelResolver(sess)
	assert.Equal(t, "200", resolve("guild-1"))
}

func TestNewDMQueueChannelResolver_NoMatchReturnsEmpty(t *testing.T) {
	sess := &testSession{
		guildChannelsFunc: func(string) ([]*discordgo.Channel, error) {
			return []*discordgo.Channel{{ID: "1", Name: "general"}}, nil
		},
	}
	resolve := newDMQueueChannelResolver(sess)
	assert.Equal(t, "", resolve("g"))
}

func TestNewDMQueueChannelResolver_ErrorReturnsEmpty(t *testing.T) {
	sess := &testSession{
		guildChannelsFunc: func(string) ([]*discordgo.Channel, error) {
			return nil, errors.New("api down")
		},
	}
	resolve := newDMQueueChannelResolver(sess)
	assert.Equal(t, "", resolve("g"))
}
