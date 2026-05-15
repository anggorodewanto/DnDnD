finding_id: A-H07
severity: High
title: Welcome DM sent to every joining member even when no campaign exists
location: internal/discord/bot.go:119-131 + internal/discord/welcome.go:6-19
spec_ref: spec §Player Onboarding (lines 184-200); Phase 9a
problem: |
  HandleGuildMemberAdd sends the welcome DM regardless of whether /setup has been run or a campaign exists. Confuses users in non-DnDnD servers.
suggested_fix: |
  Gate SendWelcomeDM on the existence of a campaign row for the guild.
acceptance_criterion: |
  Welcome DM is NOT sent when no campaign exists for the guild. A test demonstrates this.
