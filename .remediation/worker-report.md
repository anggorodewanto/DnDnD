# Worker Report: A-H07

## Finding

**HandleGuildMemberAdd** sent a welcome DM to every non-bot user joining any guild, regardless of whether a campaign had been configured via `/setup`.

## Fix Applied

Added a `hasCampaign(guildID)` check in `HandleGuildMemberAdd` (bot.go). If no campaign name has been set for the guild, the welcome DM is skipped entirely.

### Files Changed

- `internal/discord/bot.go` — Added `hasCampaign` method; gated `SendWelcomeDM` call on it.
- `internal/discord/bot_test.go` — Replaced `TestBot_HandleGuildMemberAdd_DefaultCampaignName` (which asserted a DM was sent without a campaign) with `TestBot_HandleGuildMemberAdd_NoCampaign_SkipsWelcome` (asserts no DM is sent).

## TDD Evidence

- **Red:** `TestBot_HandleGuildMemberAdd_NoCampaign_SkipsWelcome` failed with "should not send welcome DM when no campaign exists for the guild".
- **Green:** After adding the `hasCampaign` guard, all tests pass.

## Verification

- `make test` — PASS (all packages)
- `make cover-check` — PASS ("OK: coverage thresholds met")
