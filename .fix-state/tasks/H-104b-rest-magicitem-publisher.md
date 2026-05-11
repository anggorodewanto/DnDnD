---
id: H-104b-rest-magicitem-publisher
group: H
phase: 104b
severity: MEDIUM
status: open
owner:
reviewer:
last_update: 2026-05-11
---

# rest.Service and magicitem.Service missing publisher hooks

## Finding
Phase 104b scope explicitly listed `rest.Service` and `magicitem.Service` for encounter-publisher fan-out, but neither package exposes a `SetPublisher` hook or calls the dashboard publisher. A `/rest` (HP / hit dice / spell-slot restoration) taken mid-encounter — e.g. a short rest in a safe room while a sibling encounter is still active — therefore won't refresh that encounter's dashboard snapshot. The phase note rationale ("skip services that only touch out-of-combat state") doesn't match the explicit spec list.

## Code paths cited
- `internal/rest/` — pure-logic package, no stateful service surface to attach a publisher to
- `internal/magicitem/` — pure-logic package, no stateful service surface to attach a publisher to
- `internal/inventory/api_handler.go:61-87` — reference SetPublisher pattern
- `internal/levelup/service.go:120-150` — reference SetPublisher pattern
- `cmd/dndnd/main.go:691-694,743-748` — publisher wiring sites for the analogous services

## Spec / phase-doc anchors
- `docs/phases.md:632-829` (Group H, Phase 104b)
- Phase 104b scope: "Encounter Publisher Fan-out & Combat Store Adapter Cleanup"

## Acceptance criteria (test-checkable)
- [ ] `rest.Service` (or equivalent stateful surface) exposes `SetPublisher(publisher, encLookup)` and invokes it after any HP / hit-dice / spell-slot mutation
- [ ] `magicitem.Service` (or equivalent stateful surface) exposes `SetPublisher(publisher, encLookup)` and invokes it after any item-state mutation
- [ ] Both services are wired in `cmd/dndnd/main.go` alongside the existing inventory / levelup `SetPublisher` calls
- [ ] Test in `internal/rest/<package>_test.go` (and `internal/magicitem/<package>_test.go`) fails before the fix and passes after, asserting publisher fan-out fires
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- None known in Group H. Touches `cmd/dndnd/main.go` wiring shared with `H-105b-enemy-turn-notifier`.

## Notes
Both packages currently exist as pure-logic. The fix likely requires introducing a stateful service surface in each (or wiring publisher fan-out at the handler/router layer). Confirm which architectural option matches the rest of Phase 104b before implementing.
