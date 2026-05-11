---
id: A-08-retire-created-via-schema
group: A
phase: 8
severity: HIGH
status: open
owner:
reviewer:
last_update: 2026-05-11
---

# Represent retire request in player_characters (created_via='retire')

## Finding
The `created_via` CHECK constraint allows only `('register','import','create','dm_dashboard')` — there is no DB representation for a retire request. `/retire` writes the request into `dm_queue_items` instead of `player_characters`, so Phase 16's approval handler (which expects `detail.CreatedVia == "retire"`) can never be true under the current schema.

## Code paths cited
- `db/migrations/20260310120007_create_player_characters.sql` — CHECK constraint allows `('register','import','create','dm_dashboard')` only.
- `internal/discord/retire_handler.go` — writes the retire request into `dm_queue_items` instead of `player_characters`.
- `internal/dashboard/approval_handler.go:248` — expects `detail.CreatedVia == "retire"`, which can never be true.

## Spec / phase-doc anchors
- `docs/dnd-async-discord-spec.md:33-43` — `/retire` → DM approval → `status='retired'`.
- `docs/phases.md` phase 8, phase 16

## Acceptance criteria (test-checkable)
- [ ] A new migration extends `created_via` CHECK to include `'retire'` (or equivalent representation per phase intent)
- [ ] `/retire` handler creates/updates the corresponding `player_characters` row with `created_via='retire'` so it flows into the dashboard approval queue
- [ ] Phase 16 retire branch (`approval_handler.go:248`) is reachable end-to-end
- [ ] Integration test exercising `/retire` → dashboard approval → `status='retired'` fails before the fix and passes after
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- Files likely also touched by: `A-08-retire-approved-transition`, `A-16-retire-approval-unreachable`

## Notes
Phase 16 done-when criterion ("On retire: character unlinked, card updated with 'Retired' badge") is not exercised end-to-end without this fix.
