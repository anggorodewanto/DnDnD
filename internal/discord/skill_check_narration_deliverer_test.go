package discord

import (
	"errors"
	"testing"

	"github.com/bwmarrin/discordgo"
)

func TestSkillCheckNarrationDeliverer_DeliversToChannel(t *testing.T) {
	var captured struct {
		channelID string
		content   string
	}
	sess := newTestMock()
	sess.ChannelMessageSendFunc = func(channelID, content string) (*discordgo.Message, error) {
		captured.channelID = channelID
		captured.content = content
		return &discordgo.Message{ID: "msg-1"}, nil
	}

	d := NewSkillCheckNarrationDeliverer(sess)
	if err := d.DeliverSkillCheckNarration("chan-7", "You spot the trap"); err != nil {
		t.Fatalf("DeliverSkillCheckNarration: %v", err)
	}
	if captured.channelID != "chan-7" {
		t.Errorf("channel id = %q want chan-7", captured.channelID)
	}
	if captured.content != "You spot the trap" {
		t.Errorf("content = %q", captured.content)
	}
}

func TestSkillCheckNarrationDeliverer_RejectsEmptyChannel(t *testing.T) {
	d := NewSkillCheckNarrationDeliverer(newTestMock())
	if err := d.DeliverSkillCheckNarration("", "x"); err == nil {
		t.Fatal("expected error for empty channel id")
	}
}

func TestSkillCheckNarrationDeliverer_PropagatesSendError(t *testing.T) {
	sess := newTestMock()
	sess.ChannelMessageSendFunc = func(string, string) (*discordgo.Message, error) {
		return nil, errors.New("discord down")
	}
	d := NewSkillCheckNarrationDeliverer(sess)
	err := d.DeliverSkillCheckNarration("chan-7", "x")
	if err == nil {
		t.Fatal("expected error from underlying send")
	}
}
