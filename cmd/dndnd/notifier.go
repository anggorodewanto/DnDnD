package main

// noopNotifier is the fallback combat.Notifier used when DISCORD_BOT_TOKEN is
// unset. Turn-timer messages are silently dropped so the timer can still
// update DB state (nudge_sent_at, warning_sent_at, etc.) without needing a
// live Discord session.
type noopNotifier struct{}

func (noopNotifier) SendMessage(_ string, _ string) error { return nil }
