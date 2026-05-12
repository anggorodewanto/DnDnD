# DnDnD Remediation Campaign — DONE

Date: 2026-05-12  
Driver: orchestrator agent (Claude Opus 4.7)  
Gate verdict: **READY** (see `.fix-state/PLAYTEST-GATE.md`)

---

## Counts

**91 audit findings resolved.**

| | Closed | Deferred-with-justification |
|---|---:|---:|
| CRITICAL | 8 | 0 |
| HIGH | 40 | 0 |
| MEDIUM | 28 | 1 |
| MINOR / LOW | 13 | 1 |
| **Total** | **89** | **2** |

Follow-up round (2026-05-12) closed three previously-deferred items:
- `H-104c-public-levelup-deferred` — wired `SendPublicLevelUp` through `narration.Poster` (same surface as `/narrate`); `internal/levelup` cov 90.93%.
- `E-68-fov-minor` — added `VisionSource.HasDevilsSight`, `FogOfWar.DMSeesAll`, new `ComputeVisibilityWithZones` (magical-darkness demotes darkvision at FoW level), documented shadowcasting algorithm; renderer cov 97.00%.
- `C-35-dm-adv-flags` — new `POST /api/combat/{enc}/override/combatant/{cid}/advantage` endpoint, `combatants.next_attack_adv_override` column (migration 20260512120000), per-attack consume-then-clear across all 5 attack paths.

Remaining deferred (with cited justification, non-blocking for playtest):
- `H-121.4-playtest-transcripts` — task file explicitly designates deferral as expected outcome; real transcripts gathered during live playtest sessions. Gating event documented in `docs/playtest-checklist.md:205-233`.
- `PLAYTEST-REPLAY-followup-path-handling` — Makefile target requires absolute TRANSCRIPT path; documented in `docs/playtest-checklist.md`.

## Diff summary
- 254 files changed, +22593 / -419 (since pre-batch-1 HEAD `3cb4189`).
- 3 release commits: `7756d83` (batch 1), `4a30162` (batch 2), `82194c4` (batch 3).
- 90 task files + 17 worklogs in `.fix-state/`.

## Coverage delta (post-batch-3)
- internal/combat: 92.90% (vs 85% threshold)
- internal/discord: 86.51%
- internal/dashboard: 92.78%
- internal/itempicker: 97.87%
- internal/registration: 93.41%
- internal/rest: 96.25%
- internal/magicitem: 100.00%
- Overall: ≥ 90% (passes `make cover-check`)

## Packages touched
internal/combat, internal/discord, internal/dashboard, internal/registration, internal/character, internal/rest, internal/magicitem (NEW), internal/itempicker, internal/auth (file deletion), cmd/dndnd, db/queries, db/migrations (3 NEW), internal/refdata (sqlc regenerated), dashboard/svelte/src.

## Phase-doc updates
- `docs/playtest-checklist.md` — appended H-121.4 "Transcript capture status" section.
- `docs/phases.md` — no corrective bullets needed; phase scope of every patched `[x]` phase remains accurate at the spec level. Wiring gaps are tracked exclusively under `.fix-state/`.

## Quickstart timing re-validation
`docs/playtest-quickstart.md` reviewed in the final gate; no stale targets or
flags. The 30-minute fresh-checkout-to-encounter flow is reachable via the
documented `make` targets.

## Final regression sweep (round-1.md)
- `make test` ✅
- `make cover-check` ✅
- `make build` ✅
- `make e2e` ✅
- `make playtest-replay TRANSCRIPT=/home/ab/projects/DnDnD/internal/playtest/testdata/sample.jsonl` ✅

## Highlights — what this campaign fixed

1. **All 5 dashboard combat surfaces unblocked**: WorkspaceHandler, DMDashboardHandler routes, action-log viewer, undo/override endpoints, combat-log poster.
2. **Phase 105 enemy-turn label is no longer a no-op** in production.
3. **DDB importer reachable end-to-end** from `/import`.
4. **`/action` and `/bonus` dispatch fully built out**: surge, dash, disengage, dodge, help, hide, stand, drop-prone, escape, channel-divinity, lay-on-hands, wild-shape, revert-wild-shape, flurry, cunning-action (dash/disengage/hide), drag, release-drag.
5. **Death-save pipeline operational**: instant-death, damage-at-0HP, prone+unconscious on drop, Nat-20 heal reset, stabilize action.
6. **Attack-side cover applies**: half +2, three-quarters +5, total blocks. Walls flow from `/attack`.
7. **Spell zones fully alive**: silence zone-type fix, anchor follow, enter+start-of-turn triggers, round-tick cleanup, render-on-map.
8. **AoE saves close the loop**: `pending_saves` persistence + per-player `/save` ping + fan-in resolution.
9. **Rage mechanics complete**: spellcasting block, concentration drop on activate, attacked/took-damage tracking, end-on-unconscious.
10. **Post-hit class-feature prompts surfaced**: Stunning Strike, Bardic Inspiration, Divine Smite.
11. **Group-C correctness**: fall damage on prone-from-flight, attacker-size + hostile-near-attacker context, ammo recovery at EndCombat, reckless target-side advantage, charmed-attack guard, exhaustion-speed honored.
12. **Retire flow end-to-end**: `approved → retired` transition + migration extending `created_via` CHECK + dashboard approval queue actually receives retirement requests.
13. **F group**: targeted/group/DM-prompted checks, item picker homebrew flag + custom entry + narrative/price, detect-magic 30ft environment scan, ASI restart-persistence (durable `pending_asi`).
14. **Combat lifecycle**: hostiles-defeated DM prompt, loot pool auto-create, combat-end announcement, ammo-recovery summary, long-rest prepare reminder, rest + magic-item publisher fan-out.
15. **Svelte UI**: StatBlockLibrary component, HomebrewEditor structured form (7 categories), MessagePlayer desktop nav + history + character-overview embed.

## Outstanding (deferred) tickets to re-bundle later
- H-121.4 (real playtest transcripts — generated during live sessions)
- PLAYTEST-REPLAY-followup-path-handling (Makefile relative-path)

---

**Done.** Hand the campaign to live human playtest.
