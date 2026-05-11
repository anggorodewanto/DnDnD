---
id: A-16-retire-approval-unreachable
group: A
phase: 16, 8
severity: HIGH
status: open
owner:
reviewer:
last_update: 2026-05-11
---

# Retire approval branch is unreachable end-to-end

## Finding
`Approve` checks `detail.CreatedVia == "retire"`, but the `created_via` CHECK constraint forbids `'retire'` and `/retire` writes to `dm_queue_items` instead of `player_characters`. Retirement requests never enter the approval queue and the `isRetire` branch is unreachable. The Phase 16 done-when criterion "On retire: character unlinked, card updated with 'Retired' badge" is therefore not exercised end-to-end. Additionally, the unreachable `RetireCharacter` store method still requires `pending` status, so even if the retire path did land a row in `player_characters`, transitioning it from `approved` would fail.

## Code paths cited
- `internal/dashboard/approval_handler.go:248` — `detail.CreatedVia == "retire"` branch.
- `db/migrations/20260310120007_create_player_characters.sql` — CHECK constraint forbids `'retire'`.
- `internal/discord/retire_handler.go` — writes retire request to `dm_queue_items`, not `player_characters`.
- `internal/dashboard/approval_store.go` (RetireCharacter) — requires `pending` status, blocking `approved → retired`.

## Spec / phase-doc anchors
- `docs/dnd-async-discord-spec.md:33-43` — `/retire` → DM approval → `status='retired'`, card updated with Retired badge.
- `docs/phases.md` phase 16 done-when: "On retire: character unlinked, card updated with 'Retired' badge".

## Acceptance criteria (test-checkable)
- [ ] Approving a retire request in the dashboard transitions the character to `status='retired'`
- [ ] Character card is updated in place with a "Retired" badge
- [ ] Character is unlinked from the player as per spec
- [ ] End-to-end test (`/retire` → dashboard approve → DB state + card update) fails before the fix and passes after
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- Files likely also touched by: `A-08-retire-approved-transition`, `A-08-retire-created-via-schema`

## Notes
This task ties together the Phase 8 schema/transition fixes with the Phase 16 approval handler so the full retire flow is reachable and exercised end-to-end.
