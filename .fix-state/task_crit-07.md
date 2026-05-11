# task crit-07 — Player notifier nil for approvals (no DM on approve/reject)

## Finding (verbatim from chunk2_campaign_maps.md, Phase 16)

> ❌ **Player notification not wired in production.** `cmd/dndnd/main.go:580-587` constructs `NewApprovalHandler(... nil ...)` for the `notifier` arg with the explicit comment "Discord DMs on approve/reject are a follow-up." Phase 16 done-when says "player notification" is part of the approve flow — **missing**. Spec lines 53 and 41 require a Discord DM ping with the outcome.

Spec sections: Phase 16 done-when in `docs/phases.md`; spec lines 41 + 53; coverage map "Authentication & Authorization" → Phase 16.

Recommended approach (chunk2 follow-up #4): "Wire `PlayerNotifier` in `dashboard.NewApprovalHandler` (cmd/dndnd/main.go:587) using the existing `direct_messenger` for the three notify methods."

## Plan

1. Add `playerNotifierAdapter` (in `cmd/dndnd/discord_adapters.go`, where production adapters live) implementing `dashboard.PlayerNotifier` (NotifyApproval / NotifyChangesRequested / NotifyRejection).
2. Adapter wraps a `playerDirectMessenger` interface (subset of `*discord.DirectMessenger`) so it can be unit-tested without a live Discord session; production passes `discord.NewDirectMessenger(discordSession)`.
3. Each method formats a short message (verbatim character name + DM feedback for changes / rejection) and calls `SendDirectMessage`. Failures wrap with the user id for log triage.
4. In main.go, build `approvalNotifier` only when `discordSession != nil` (matches the `cardPoster` pattern), then pass it to `dashboard.NewApprovalHandler` instead of `nil`. Update the comment block.
5. Failing-first tests in `cmd/dndnd/discord_adapters_test.go` cover happy paths, propagated errors, and an interface-satisfaction guard.

## Files touched

- `cmd/dndnd/discord_adapters.go` — add `playerDirectMessenger` interface + `playerNotifierAdapter` (3 methods).
- `cmd/dndnd/main.go` — replace the `nil` notifier arg with `newPlayerNotifierAdapter(discord.NewDirectMessenger(discordSession))` (guarded on session presence); update docstring.
- `cmd/dndnd/discord_adapters_test.go` — add `dashboard` import, fake DM, 7 unit tests.

## Tests added

- `TestPlayerNotifierAdapter_NotifyApproval_SendsDMWithCharacterName` — happy path approval includes character name + "approved".
- `TestPlayerNotifierAdapter_NotifyChangesRequested_IncludesFeedback` — message contains both character name and DM feedback verbatim.
- `TestPlayerNotifierAdapter_NotifyRejection_IncludesFeedback` — same, for rejection.
- `TestPlayerNotifierAdapter_NotifyApproval_PropagatesDMError` — DM error wraps + propagates.
- `TestPlayerNotifierAdapter_NotifyChangesRequested_PropagatesDMError` — same.
- `TestPlayerNotifierAdapter_NotifyRejection_PropagatesDMError` — same.
- `TestPlayerNotifierAdapter_SatisfiesPlayerNotifier` — compile-time interface conformance guard so any future change to `dashboard.PlayerNotifier` fails this test before reaching main.go.

## Implementation notes

- `dashboard.PlayerNotifier` was already defined (`internal/dashboard/approval.go:54-58`); `approval_handler.go` invokes it with discord_user_id + character_name (+ feedback) — every field is on the `ApprovalDetail` row already, no schema/join expansion needed.
- `discord.DirectMessenger` (`internal/discord/direct_messenger.go`) was already in production use for `messageplayer` / level-up / dm-queue whisper paths — reused to keep one DM code path. Long bodies auto-chunk via `SendContentReturningIDs`.
- Adapter ignores `ctx` because `DirectMessenger.SendDirectMessage` does not take one (Discord REST is fire-and-forget here); errors from the discordgo session are wrapped + returned. Approval handler logs but does not abort the DB mutation on notifier failure (existing behaviour preserved at `approval_handler.go:266-268`, `308-310`, `331-333`).
- When the bot session is offline (e2e harness), `approvalNotifier` stays nil and the handler silently skips the DM — same fallback as `cardPoster`.
- `make cover-check`: overall 94.39%; all per-package thresholds met. `cmd/dndnd/discord_adapters.go` is on the coverage exclusion list but new code is still exercised by the seven adapter tests above.
- Out of scope (separate tasks): Phase 14 welcome DM (`docs/phases.md` line ~95), Phase 17 `OnCharacterUpdated` fan-out (cross-cutting #5), `RollHistoryLogger` / `MapRegenerator` wiring. No changes to those surfaces.

## Review (reviewer fills) — Verdict: PASS | REVISIT

STATUS: READY_FOR_REVIEW

## Review

Verdict: PASS

- main.go:588-592 replaces the prior `nil` notifier with `newPlayerNotifierAdapter(discord.NewDirectMessenger(discordSession))`, guarded on `discordSession != nil` (matches the `cardPoster` pattern — silent no-op when bot is offline).
- `playerNotifierAdapter` implements all three `dashboard.PlayerNotifier` methods (interface verified at internal/dashboard/approval.go:54-58). The compile-time conformance guard test pins the contract.
- DM bodies: NotifyApproval includes character name + "approved"; NotifyChangesRequested and NotifyRejection both include character name AND the DM's feedback verbatim (`fmt.Sprintf("...**DM feedback:** %s", ...)`) — satisfies the spec's "no reject without explanation" rule (spec lines 41 + 53).
- 7 tests in discord_adapters_test.go cover the 3 happy paths (each asserting on body content), 3 error-propagation paths (`require.ErrorIs`), and the interface-conformance guard. Errors are wrapped with the user id for log triage.
- Adapter ignores `ctx` because `DirectMessenger.SendDirectMessage` does not take one; consistent with existing direct_messenger surface.
- `make cover-check` previously verified by orchestrator; spot-check of cmd/dndnd / internal/dashboard: 83.6% / 94.5%, all green.
- Out-of-scope items (Phase 14 welcome DM, Phase 17 `OnCharacterUpdated` fan-out, RollHistoryLogger / MapRegenerator) correctly deferred.
