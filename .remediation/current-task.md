finding_id: H-H05
severity: High
title: Builder service: token redeem races and isn't user-bound
location: internal/portal/builder_service.go:219-238 (CreateCharacter)
spec_ref: Phase 91a "one-time link generation … single-use token"
problem: |
  RedeemToken is called AFTER CreateCharacterRecord succeeds, so concurrent double-submit can produce two characters. Also the token's discord_user_id is never compared against the session userID.
suggested_fix: |
  Validate token first, compare tok.DiscordUserID == userID, then atomically mark-used before inserting the character.
acceptance_criterion: |
  CreateCharacter validates token ownership (rejects mismatched user) and redeems before creating. A test demonstrates both.
