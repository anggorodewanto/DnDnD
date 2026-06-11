package main

import "strings"

// discordCloseHint maps a Discord gateway session-open error to an
// operator-facing hint for the close codes that have an obvious config cause,
// or "" when the error carries no recognised code.
//
// 4004 (Authentication failed) almost always means a bad or expired
// DISCORD_BOT_TOKEN; 4014 (Disallowed intent) means a privileged intent — for
// DnDnD, the Server Members intent — was requested without being enabled in
// the Discord developer portal. discordgo surfaces both as the close code
// embedded in the Open() error string, so a substring match is sufficient and
// avoids depending on discordgo's unexported close-frame types.
func discordCloseHint(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "4004"):
		return "gateway rejected the credentials (close 4004 Authentication failed): check DISCORD_BOT_TOKEN"
	case strings.Contains(msg, "4014"):
		return "gateway rejected a privileged intent (close 4014): enable the Server Members Intent in the Discord developer portal"
	default:
		return ""
	}
}
