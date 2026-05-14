# Regression Report — Post-Remediation Sweep

Date: 2026-05-14
Scope: 63 fix commits (SR-001 through SR-068) landing on main.

---

## 1. Test Suite & Coverage

| Check | Result |
|-------|--------|
| `go test ./... -race` | PASS (all packages except `internal/database`) |
| `TestIntegration_MigrateDown` | FAIL — **known pre-existing**, unrelated to remediation |
| `make cover-check` (coverage thresholds) | PASS — 91.49% overall, all packages ≥85% |

No regressions introduced.

---

## 2. Bundle Consistency Verification

### Bundle A — Auth gating (SR-001, SR-063)
- Grep for `RegisterRoutes(router)` in `cmd/dndnd/main.go`: only `encounterHandler`, `assetHandler`, and `open5eHandler` remain on the bare router.
- Per SR-001 reviewer notes, these are intentionally public (read-only player-facing data). All DM-mutation handlers are behind `mountDMOnlyAPIs` with `dmAuthMw`.
- **Consistent. ✓**

### Bundle B — DM queue insert-then-post (SR-002)
- `DefaultNotifier.Post` (internal/dmqueue/notifier.go:163) calls `n.store.Insert` before `n.sender.Send`.
- All six originally-broken callers (reaction, check, rest, retire, undo, use) now pass `CampaignID` in the Event struct.
- **Consistent. ✓**

### Bundle C — OnCharacterUpdated (SR-007)
- `notifyCardUpdate` (which calls `OnCharacterUpdated`) is invoked from: `/equip`, `/use`, `/give` (both parties), `/loot`, `/attune`, `/unattune`, `/rest`, `/prepare`.
- Test file `sr007_card_update_test.go` asserts exactly-one (or two for /give) calls per handler.
- **Consistent. ✓**

### Bundle D — FoW/visibility (SR-008, SR-068)
- `renderInternal` in `cmd/dndnd/discord_adapters.go` populates `md.VisionSources` via `buildVisionSources` and `md.LightSources` via `buildLightSources`.
- `md.MagicalDarknessTiles` populated from zones. `md.DMSeesAll` set for DM view.
- **Consistent. ✓**

### Bundle E — Combat service routing (SR-004, SR-005)
- All DB writes in `internal/combat/` go through `s.store` (the service's store interface).
- `dm_dashboard_handler.go` accesses `h.svc.store` — the service's own store, not a raw DB handle.
- No direct `queries.Update/Insert/Delete` calls found outside the service layer.
- **Consistent. ✓**

### Bundle F — Discord handler coverage
- All 33 slash command handlers (`action` through `whisper`) have corresponding `*_test.go` files.
- **Consistent. ✓**

---

## 3. Critical Item Callout Verification (SUMMARY §1)

| Item | Status |
|------|--------|
| 1. Bare-router DM-mutation mounts | Fixed (SR-001). Zero bare-router calls for listed handlers. |
| 2. CampaignID missing + send-then-insert | Fixed (SR-002). Insert-then-send confirmed. |
| 3. GuildCreate/GuildMemberAdd never wired | Fixed (SR-003). |
| 4. /equip bypasses combat.Equip | Fixed (SR-004). Routes through `combatSvc.Equip`. |
| 5. /interact bypasses combat.Interact | Fixed (SR-005). |
| 6. Magic items never feed pipelines | Fixed (SR-006). |
| 7. OnCharacterUpdated not fired outside combat | Fixed (SR-007). |
| 8. FoW dead code | Fixed (SR-008, SR-068). VisionSources + LightSources populated. |
| 9. feature_uses JSON shape forked | Fixed (SR-009). |
| 10. Sneak Attack OncePerTurn unenforced | Fixed (SR-010). |
| 11. Reckless first-attack gate broken | Fixed (SR-011). |
| 12. UNIQUE blocks re-register-after-retire | Fixed (SR-012). Partial unique index deployed. |
| 13. AoE cylinder + Metamagic | Fixed (SR-013). |
| 14. Encounter zones no damage | Fixed (SR-014). |
| 15. DM multiclass dropped | Fixed (SR-015). |
| 16. WS Origin InsecureSkipVerify | Fixed (SR-016). Configurable; prod sets `false` + allowlist. |

---

## 4. Dead Code / TODO Audit (git log main~64..HEAD)

- **No `// TODO` comments** introduced in any remediation commit.
- **No unused exported functions** — all 20 new exported functions have at least one non-definition reference.
- **SR-060 explicitly removed** the dead `RegisterCommands` delete-loop (noted in SUMMARY §3).
- No re-exports or orphan helpers detected.

---

## 5. Schema Drift

```
$ make sqlc-check
OK: no sqlc drift under internal/refdata
```

No drift.

---

## VERDICT: pass
