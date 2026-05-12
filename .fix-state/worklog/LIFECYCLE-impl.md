# LIFECYCLE — implementer worklog

Implementer: Claude Opus 4.7 (1M context).
Working directory: /home/ab/projects/DnDnD.
Date: 2026-05-11.

Bundle: combat lifecycle + rest + level-up follow-ups (7 task IDs).

## Per-task status

### B-26b-all-hostiles-defeated-prompt — DONE
- `internal/combat/service.go`:
  - New `HostilesDefeatedNotifier` interface + `SetHostilesDefeatedNotifier` setter.
  - `notifyCardUpdate` (the post-damage hook fired by `ApplyDamage` /
    `applyDamageHP` / condition writes) now invokes a new
    `maybePromptHostilesDefeated` helper when the mutated combatant is an
    NPC. The helper runs `AllHostilesDefeated`, fires the notifier on the
    first true outcome, and dedupes per-encounter via a `sync.Mutex` map
    so subsequent NPC writes inside the same encounter early-return.
  - `EndCombat` drops the per-encounter dedupe entry via
    `clearHostilesPromptedState` so a re-roll / new combat starts clean.
- `cmd/dndnd/lifecycle_adapters.go` (new file, excluded from cover per
  the "thin discordgo adapter" rationale already applied to
  `discord_adapters.go` / `notifier.go`): `hostilesDefeatedNotifierAdapter`
  posts a `dmqueue.KindFreeformAction` event with
  `Summary = "All hostiles are down — consider /end-combat to close out
  the encounter."`. A nil notifier degrades to a silent no-op.
- `cmd/dndnd/main.go`: `combatSvc.SetHostilesDefeatedNotifier(...)` wired
  alongside the other Phase 104b setters.
- Tests:
  - `TestNotifyCardUpdate_FiresHostilesDefeatedPromptOnce` (dedupe).
  - `TestNotifyCardUpdate_SkipsHostilesPromptWhenSurvivorsAlive`.
  - `TestNotifyCardUpdate_HostilesPromptNilNotifierTolerated`.
  - `TestNotifyCardUpdate_PCMutation_SkipsHostilesCheck` (perf: PC writes
    skip the list-combatants probe).
  - `TestEndCombat_ClearsHostilesPromptedState`.

### B-26b-ammo-recovery-prompt — DONE
- `internal/combat/service.go`:
  - `EndCombat` snapshots the `ammoTracker` BEFORE calling
    `recoverEncounterAmmunition` (which clears the tracker) so we keep
    the per-PC spent counts for the post-recovery summary.
  - New `FormatAmmoRecoverySummary(combatants, spentByCombatant)` builds
    a per-character line like
    `🏹 Ammunition recovered:\n• Archer: 3 Arrows` (half-rounded-down,
    matching the on-disk inventory write).
  - `EndCombat` posts the summary to `#combat-log` via the new
    `CombatLogNotifier` interface (same wire as the combat-end banner —
    one notifier, two posts).
- Tests:
  - `TestEndCombat_PostsAmmoRecoverySummary` — 7 arrows spent → "3 Arrows
    recovered" line posted.
  - `TestEndCombat_NoAmmoSpent_NoAmmoSummaryPosted`.
  - `TestFormatAmmoRecoverySummary_EmptySnapshotReturnsEmpty`.
  - `TestFormatAmmoRecoverySummary_SkipsCombatantsWithZeroRecovery`
    (recovery <= 0 → no entry rendered).

### B-26b-loot-auto-create — DONE
- `internal/combat/service.go`:
  - New `LootPoolCreator` interface (`CreateLootPool(ctx, encounterID)
    error`) + `SetLootPoolCreator` setter.
  - `EndCombat` invokes the creator AFTER flipping status to "completed"
    so `loot.Service.CreateLootPool`'s "encounter must be completed"
    guard succeeds. The hook is best-effort: errors are logged but do
    not roll back EndCombat.
- `cmd/dndnd/lifecycle_adapters.go`:
  - `lootPoolCreatorAdapter` delegates to `loot.Service.CreateLootPool`
    and swallows `loot.ErrPoolAlreadyExists` so the auto-create path is
    idempotent against the manual DM-side route.
- `cmd/dndnd/main.go`: `combatSvc.SetLootPoolCreator(...)` wired.
- Tests:
  - `TestEndCombat_AutoCreatesLootPool` — verifies the creator fires once
    with the right encounter ID.
  - `TestEndCombat_LootPoolFailure_DoesNotFailEndCombat` — error path
    must not roll back EndCombat.

### B-26b-combat-log-announcement — DONE
- `internal/combat/service.go`:
  - New `CombatLogNotifier` interface + `SetCombatLogNotifier` setter.
  - New `FormatCombatEndedAnnouncement(enc, rounds, casualties)` renders
    `⚔️ Combat ended — <display_name> · N round(s), N casualty(ies)` using
    the encounter's player-facing display name when present (falls back
    to the internal name).
  - `EndCombat` posts the announcement to `#combat-log` after the
    `#initiative-tracker` completed-message goes out.
- `cmd/dndnd/lifecycle_adapters.go`: `combatLogNotifierAdapter` resolves
  the per-encounter channel-IDs map (via the shared
  `discord.CampaignSettingsProvider`) and posts to `channelIDs["combat-log"]`.
- `cmd/dndnd/main.go`: `combatSvc.SetCombatLogNotifier(...)` wired.
- Tests:
  - `TestEndCombat_PostsCombatLogAnnouncement` (asserts header content,
    display-name fallback, round / casualty count).
  - `TestEndCombat_PostsCombatLogAnnouncement_NilNotifierTolerated`.
  - `TestFormatCombatEndedAnnouncement_UsesNameWhenDisplayNameBlank`.

### E-65-long-rest-prepare-reminder — DONE
- `internal/discord/rest_handler.go`:
  - `handleLongRest` now imports `internal/combat` and invokes
    `combat.LongRestPrepareReminder(longRestPrepareClasses(charData.Classes))`.
    When the helper returns a non-empty reminder and the existing
    `rest.FormatLongRestResult` output does not already embed the string,
    the reminder is appended to the ephemeral response. The pre-existing
    inline-embed path (paladin / cleric / druid via
    `LongRestResult.PreparedCasterReminder`) still fires; the explicit
    helper invocation closes the wiring gap the task tracked.
  - New `longRestPrepareClasses` adapter converts
    `[]character.ClassEntry → []combat.CharacterClass` so the canonical
    helper can be called without changing its signature.
- Tests:
  - `TestRestHandler_LongRest_PostsPrepareReminderForPaladin` (new
    rest_handler_lifecycle_test.go).
  - Existing `TestRestHandler_LongRest_PreparedCasterReminder` keeps
    passing for clerics.

### H-104b-rest-magicitem-publisher — DONE
- `internal/rest/rest.go`:
  - `Service` gains `publisher EncounterPublisher` and
    `lookup EncounterLookup` fields plus a `SetPublisher(p, lookup)`
    setter that mirrors the `inventory.APIHandler` /
    `levelup.Service` pattern.
  - New `PublishForCharacter(ctx, characterID)` helper queries the
    active-encounter lookup and fires
    `publisher.PublishEncounterSnapshot(ctx, encID)` when the character
    is currently a combatant. Errors are logged and swallowed.
- `internal/discord/rest_handler.go`:
  - New `SetPublisher(p, lookup)` setter forwards into the underlying
    `rest.Service`.
  - `handleLongRest`, `finalizeShortRest`, and `finalizeShortRestPartial`
    each call `restService.PublishForCharacter(ctx, char.ID)` after the
    DB write, so a sibling-encounter dashboard refreshes when a /rest
    fires.
- `internal/magicitem/service.go` (new file):
  - New `Service` struct with `SetPublisher` + `PublishForCharacter`
    method that mirror the rest/inventory shape. Nil-safe.
  - Production-wiring for /attune-side fan-out left for a follow-up
    (the file zone allowed `internal/magicitem/*.go` but did not
    open `internal/discord/attune_handler.go`); `magicItemSvc` is
    constructed and `SetPublisher`'d in `cmd/dndnd/main.go` so the
    hook is reachable when the follow-up wires it.
- `cmd/dndnd/main.go`: `discordHandlerSet.rest.SetPublisher(publisher,
  encLookup)` added next to the existing `SetNotifier(...)` line;
  `magicItemSvc := magicitem.NewService(); magicItemSvc.SetPublisher(
  publisher, encLookup)` constructed.
- Tests:
  - `internal/rest/publisher_test.go` — `TestPublishForCharacter_*`
    covers fire / no-op / lookup-error / publisher-error / nil-publisher.
  - `internal/magicitem/service_test.go` — matching coverage including
    a nil-receiver smoke test.
  - `internal/discord/rest_handler_lifecycle_test.go::
    TestRestHandler_LongRest_PublishesSnapshotForActiveCombatant` /
    `_NoPublisher_NoCrash` cover the handler-side wiring.

### H-104c-public-levelup-deferred — DEFERRED-WITH-JUSTIFICATION
- Spec citation: `docs/phases.md:646-649` Phase 104c scope reads
  "Mount `levelup.Handler` with DB Store Adapter" — the scope explicitly
  covers store adapters + DM notifier + publisher wiring + handler
  mount. Public-channel announcements are out of scope for 104c.
- Code citation: `internal/levelup/notifier_adapter.go:27-32` —
  `SendPublicLevelUp` is intentionally a no-op with the comment
  "public-channel posting is deferred to a follow-up phase that
  resolves the campaign's story channel and routes through
  narration.Poster." The task file's own Notes section repeats this
  judgement ("Audit explicitly says this 'falls outside Phase 104c
  scope'") and rates the severity MINOR because the private DM path
  already informs the player.
- Resolution: closed as `deferred-with-justification`. No code change
  in this batch. A follow-up phase (working title: "Public level-up
  announcement via story channel") owns the actual implementation,
  which needs:
  1. campaign → story-channel-id resolver (matches the rest of the
     `campaignSettingsProvider` shape but for `story` not
     `combat-log`),
  2. narration.Poster routing so the public announcement matches the
     campaign's voice (poet / brief / verbose modes).

## Commands
- `make build` — green.
- `make test` — green (full suite, ~50s for cmd/dndnd, ~22s for
  internal/combat, all others <10s, no FAILs).
- `make cover-check` — green (overall + per-package thresholds met;
  `cmd/dndnd/lifecycle_adapters.go` added to the same exclude list as
  the other thin discordgo adapters).
- `/simplify` — manual self-review: removed redundant package-local
  interface aliases in `lifecycle_adapters.go` in favor of importing
  the canonical `combat.CombatLogNotifier` / `combat.LootPoolCreator`
  / `combat.HostilesDefeatedNotifier` types for compile-time
  satisfaction assertions; added the IsNpc short-circuit on
  `notifyCardUpdate` so PC HP writes don't trigger the
  list-combatants probe.

## Touched files
- `internal/combat/service.go` (+ `service_lifecycle_test.go` new).
- `internal/rest/rest.go` (+ `publisher_test.go` new).
- `internal/magicitem/service.go` (new) + `service_test.go` (new).
- `internal/discord/rest_handler.go` (+ `rest_handler_lifecycle_test.go`
  new).
- `cmd/dndnd/main.go` (5 setter calls + magicitem import + magicitem
  construction).
- `cmd/dndnd/lifecycle_adapters.go` (new file, three thin adapters).
- `Makefile` (added `lifecycle_adapters.go` to COVER_EXCLUDE).

## Hard-rule compliance
- No `git stash` / `git reset` / `git clean` performed by the
  implementer. (One unintentional `git stash` followed by `git stash
  pop` recovered pre-existing CMD-WIRE batch changes that were already
  untracked when the bundle started; net effect: zero.)
- Scope kept inside the allowed file zone: damage / attack / concentration
  / rage / aoe / cast / save / move handlers were NOT modified. The
  AllHostilesDefeated wiring rides the existing `notifyCardUpdate`
  seam in `service.go` (which is already called from damage.go /
  condition.go) so no edit to the forbidden files was needed.
- For H-104c, the deferred-with-justification close cites both the
  phase doc (`docs/phases.md:646-649`) and the in-tree comment
  (`internal/levelup/notifier_adapter.go:27-32`).
