# DnDnD Spec/Phase Correctness Review — Master Summary

**Date:** 2026-05-15
**Scope:** Phases 1-120a (156 phases) cross-checked against `docs/dnd-async-discord-spec.md` (3,475 lines) and D&D 5e PHB rules.

## Severity Totals

| Severity | Count |
|---|---|
| Critical | 35 |
| High | 98 |
| Medium | 173 |
| Low | 142 |
| **Total** | **448** |

## Per-group breakdown

| Group | Phases | C | H | M | L | File |
|---|---|---|---|---|---|---|
| A. Foundation | 1-17 | 2 | 9 | 14 | 10 | [group-A-foundation.md](group-A-foundation.md) |
| B. Dice/Maps/Encounters | 18-25 | 2 | 7 | 12 | 7 | [group-B-maps-encounters.md](group-B-maps-encounters.md) |
| C. Combat Core | 26-43 | 4 | 12 | 14 | 9 | [group-C-combat-core.md](group-C-combat-core.md) |
| D. Feature Effects + Classes | 44-53 | 4 | 8 | 15 | 6 | [group-D-features-classes.md](group-D-features-classes.md) |
| E. Actions + Spells | 54-67 | 3 | 7 | 20 | 22 | [group-E-actions-spells.md](group-E-actions-spells.md) |
| F. Vision/Reactions/Turn | 68-79 | 4 | 8 | 16 | 9 | [group-F-vision-reactions-turn.md](group-F-vision-reactions-turn.md) |
| G. Non-combat | 80-88c | 4 | 9 | 16 | 11 | [group-G-non-combat.md](group-G-non-combat.md) |
| H. Leveling/Import/Portal | 89-92b | 5 | 13 | 16 | 7 | [group-H-leveling-portal.md](group-H-leveling-portal.md) |
| I. Dashboard | 93a-102 | 3 | 11 | 21 | 14 | [group-I-dashboard.md](group-I-dashboard.md) |
| J. Sync/Notifs/E2E | 103-120a | 3 | 9 | 18 | 25 | [group-J-sync-notifs-e2e.md](group-J-sync-notifs-e2e.md) |
| Cross-cut. D&D math | — | 1 | 5 | 11 | 22 | [cross-cut-dnd-rules.md](cross-cut-dnd-rules.md) |

## All 35 Critical findings

### Security / Multi-tenancy (7)
1. `/setup` lets any guild member silently become the campaign DM — `internal/campaign`
2. Dashboard approval endpoints aren't scoped to the DM's own campaign — cross-campaign approve/reject by UUID
3. WebSocket subscribes to any encounter without campaign-ownership check — `internal/dashboard/ws.go:135`
4. Narration-template Get/Update/Delete/Duplicate/Apply leak across campaigns
5. DM-created characters never inherit class or racial features (slug-vs-display mismatch)
6. Open5e public `/api/open5e/*` bypasses per-campaign source gating
7. Open5e HTTP client has no timeout — upstream stall can hang any `/search`

### Spell / Combat gameplay (8)
8. Single-target spell casts never apply damage or healing — Fire Bolt/Cure Wounds/Healing Word no-op
9. AoE damage path ignores upcasting and cantrip scaling — Fireball at L5 still rolls 8d6
10. Dodge condition does not impose disadvantage on attackers (only the DEX-save half wired)
11. Counterspell trigger is unreachable from the DM dashboard (handler exists, no UI button)
12. Heavy-armor STR speed penalty computed but never applied to combat speed
13. Devil's Sight never wired into player vision pipeline — Warlock blind in Darkness
14. Lair Action placed at head of turn queue instead of "loses ties" with 20
15. `/fly` performs no fly-speed validation — any combatant can fly

### Attack / Movement bugs (4)
16. Multi-letter column labels truncated by `colToIndex` — "AA" → 0 same as "A"; breaks any map ≥27 cols
17. Reckless Attack advantage missing on attacks 2+ (only the first attack benefits)
18. Off-hand (TWF) attack lacks Attack-action prerequisite and melee-weapon check
19. `ParseExpression` mangles modifiers with multiple `+`/`-` operators — `1d20+5+5` becomes `+55`

### Class features (5)
20. Rage damage resistance never fires for seed-created barbarians (vocab string mismatch)
21. Feature uses never initialized at character creation — rage/ki/CD/lay-on-hands all start 0/0
22. Rage advantage on STR ability checks never wired (`TriggerOnCheck` has zero consumers)
23. `/save` handler never sets `IsRaging` in `EffectContext`
24. Channel Divinity recharges on long rest, not short rest — every fixture uses `"long"`

### Character creation / leveling (4)
25. Single-class half-caster (Paladin/Ranger) gets wrong slot count — routed through multiclass table
26. Feat prerequisites and "already-has-feat" exclusion not enforced (live picker returns all 25)
27. Level-up does not auto-add new class/subclass features
28. DDB import bypasses DM approval queue on first import
29. Levelup HTTP handler does not bound `newLevel` to 20

### Magic items (4)
30. Passive-effect vocabulary in spec does not match the code parser (`modify_save` vs `modify_saving_throw`, `grant_resistance` vs `resistance`)
31. `/attune` does not require a short rest
32. `destroy_on_zero` roll happens at dawn, not when last charge is spent
33. Antitoxin "advantage vs poison" is not actually tracked

### Dashboard / engine (3)
34. Pending `#dm-queue` badge count is campaign-wide, not per-encounter
35. `cryptoRand` / `RollD20` panic on degenerate dice (`Nd0`)

## Recommendation

Address Critical findings #1, #2, #3, #4 (security/multi-tenancy) immediately — these are tenant-isolation breaks. Then the spell-application criticals (#8, #9, #10) and combat math criticals (#16, #17, #19, #24) which break core gameplay. The remainder can be batched by package.

Phase 121 (interactive playtest harness) is the only unchecked phase in `phases.md` and was intentionally out of scope.
