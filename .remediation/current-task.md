finding_id: A-C01
severity: Critical
title: `/setup` lets any guild member silently become the campaign DM
location: cmd/dndnd/discord_adapters.go:135-163, internal/discord/setup.go:217-249
spec_ref: spec §Authentication & Authorization (line 65) — "System verifies the authenticated Discord user ID matches the campaign's designated DM"; Phase 12
problem: |
  `GetCampaignForSetup` auto-creates the campaign row with the invoking user as DM whenever no row exists for that guild. The `/setup` slash command has `DefaultMemberPermissions: ManageChannels`, but Discord allows guild admins to override that and the handler itself does no server-side authorization check. The first non-owner who runs `/setup` becomes the permanent DM, which then gates the entire dashboard and player-management surface for that guild.
suggested_fix: |
  Make /setup require an explicit DM identity (e.g., compare invoker against the guild owner returned by Discord, or require a pre-provisioned `campaigns` row, or have an admin-only "claim DM" endpoint). At minimum, never let an arbitrary `interaction.Member` user implicitly create the campaign + DM binding.
  
  Specifically: after resolving the campaign (when it already exists), reject the interaction if `invokerUserID != info.DMUserID`. For the auto-create case (no campaign exists yet), require the invoker to be the guild owner (check `interaction.Member.Permissions` for Administrator bit, or compare against the guild's OwnerID).
acceptance_criterion: |
  When a campaign already exists for a guild, only the campaign's DM can run /setup (others get an error). When no campaign exists, only a guild administrator (or owner) can auto-create one.
