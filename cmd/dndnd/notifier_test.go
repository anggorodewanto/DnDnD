package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNoopNotifier_SendMessage(t *testing.T) {
	n := noopNotifier{}
	require.NoError(t, n.SendMessage("chan", "hello"))
}
