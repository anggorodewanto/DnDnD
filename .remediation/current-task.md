finding_id: H-H01
severity: High
title: Player-identity not validated on ASI button / select interactions
location: internal/discord/asi_handler.go:354,391,647,731
spec_ref: spec §"ASI path" line 2484; Phase 89d
problem: |
  HandleASIChoice, HandleASISelect, HandleASIFeatSelect, HandleASIFeatSubChoiceSelect extract discordUserID but never check that the interacting user is the character owner. Any guild member can press the buttons.
suggested_fix: |
  Resolve ASICharacterData.DiscordUserID and reject if interaction.Member.User.ID != charData.DiscordUserID.
acceptance_criterion: |
  ASI handlers reject interactions from users who don't own the character. A test demonstrates this.
