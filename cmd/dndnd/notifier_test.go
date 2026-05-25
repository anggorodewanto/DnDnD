package main

import (
	"context"
	"testing"

	"github.com/ab/dndnd/internal/refdata"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNoopNotifier_SendMessage(t *testing.T) {
	n := noopNotifier{}
	require.NoError(t, n.SendMessage("chan", "hello"))
}

// recordingSender is a dmqueue.Sender that records the last Send call.
type recordingSender struct {
	called    bool
	channelID string
	content   string
}

func (r *recordingSender) Send(channelID, content string) (string, error) {
	r.called = true
	r.channelID = channelID
	r.content = content
	return "msg-1", nil
}

func (r *recordingSender) Edit(_, _, _ string) error { return nil }

// A nil sender (bot offline) is a silent no-op.
func TestPortalDMQueueNotifier_NilSender_NoOp(t *testing.T) {
	n := &portalDMQueueNotifier{sender: nil, queries: refdata.New(nil)}
	require.NoError(t, n.NotifyDMQueue(context.Background(), uuid.New().String(), "Thorin", "user-1", "portal-create"))
}

// An unparseable campaign id is a silent no-op and never reaches the sender.
func TestPortalDMQueueNotifier_BadCampaignID_NoOp(t *testing.T) {
	sender := &recordingSender{}
	n := &portalDMQueueNotifier{sender: sender, queries: refdata.New(nil)}
	require.NoError(t, n.NotifyDMQueue(context.Background(), "not-a-uuid", "Thorin", "user-1", "portal-create"))
	assert.False(t, sender.called)
}
