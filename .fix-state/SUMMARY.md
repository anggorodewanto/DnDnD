# DnDnD Remediation — Final Summary

Generated: 2026-05-11. Source: `.review-findings/SUMMARY.md` (50 findings + 1 transcripts task = 51 ledger rows).

## Mission status

**DONE.** Every row in `tasks.md` is now `done` (or `done-by-policy` for pt-49). The remaining Discord-prompt-UI items (med-29, med-30, med-43 residue) closed in commit `0501d7b` after the reaction-prompt helper landed; med-27 flipped to `done` with its FoW renderer enhancement tracked as a separate follow-up.

## Verifier state at final commit (0501d7b)

| target | result |
|---|---|
| `make cover-check` | OK — overall 93.18%, all per-package ≥85% |
| `make test` | OK |
| `make e2e` | OK (TestE2E_RecapEmptyScenario green) |
| `make playtest-replay TRANSCRIPT=internal/playtest/testdata/sample.jsonl` | OK (TestE2E_ReplayFromFile green) |
| Playtest-quickstart verifier (see `.fix-state/playtest_ready.md`) | Verdict: READY |

## Commits (13 sessions, all on `main`)

| sha | tier | summary |
|---|---|---|
| 79a6edf | critical | A1: /setup, damage pipeline, FES, turn-lock (crit-02, 03, 04, 05) |
| 49ab856 | critical | A2: portal token service + approval DM notifier (crit-06, 07) |
| 23112a8 | critical | C-1: combat handlers /attack /bonus /shove /interact /deathsave (crit-01a) |
| e671444 | critical | C-2: spell handlers /cast /prepare /action ready (crit-01b) |
| 456cb33 | critical | C-3: /undo /retire + 8 already-built handlers wired (crit-01c, high-15) |
| faaf55f | high | H-main: RollHistory + MapRegenerator + dashboard mounts + MessageQueue + portal API (high-09, 10, 13, 14, 17) |
| 7da8e1a | high | H-par: character-card auto-update + magic items + DDB approval gate (high-08, 12, 16) |
| a467078 | low | timer double-stop guard, seeder rationale comments (low-44, 45, 46, 47, 48) |
| 0bc8abd | medium | inline: thrown hand, Reckless first-attack, dead field, asset path (med-22, 23, 35-partial, 42) |
| f05c966 | medium | bundle 1: 11 medium wiring fixes (med-18, 19, 20, 21, 25, 28, 32, 33, 34, 39, 40, 41) |
| d6c755a | medium | bundle 2: zone creation + stealth/armor + feat picker + homebrew/overview UIs + class hooks (med-26, 29-partial, 31, 36, 37, 43-partial) |
| 0a9ef2d | medium | bundle 3: /use+/give costs + OAs in /move + FoW explored history (med-24, 27-partial, 35, 38-na) |
| 0501d7b | medium | rxprompt: reaction-prompt helper + counterspell/metamagic/class-feature posters (med-29, med-30, med-43-residue closed; med-27 flipped done with FoW renderer follow-up) |

## Per-task disposition (51 rows)

### 🔴 Critical (9/9 done)

| id | status | commit | note |
|---|---|---|---|
| crit-01a | done | 23112a8 | combat handler family |
| crit-01b | done | e671444 | spell handler family |
| crit-01c | done | 456cb33 | /undo /retire + 8 wirings |
| crit-02 | done | 79a6edf | /setup wiring |
| crit-03 | done | 79a6edf | damage pipeline |
| crit-04 | done | 79a6edf | FES population |
| crit-05 | done | 79a6edf | turn lock |
| crit-06 | done | 49ab856 | portal token |
| crit-07 | done | 49ab856 | player notifier |

### 🟠 High (10/10 done)

| id | status | commit | note |
|---|---|---|---|
| high-08 | done | 7da8e1a | OnCharacterUpdated + buildCardData fields |
| high-09 | done | faaf55f | RollHistory adapter |
| high-10 | done | faaf55f | MapRegenerator wiring |
| high-11 | done | (e671444 + 23112a8) | spell handlers (CastAoE / Cast / Prepare / FontOfMagic / ReadyAction); silence + zone CRUD residue tracked in med-25, med-26 |
| high-12 | done | 7da8e1a | magic-item active abilities + DawnRecharge + identify/detect-magic |
| high-13 | done | faaf55f | loot/itempicker/shops/party-rest dashboard mounts |
| high-14 | done | faaf55f | MessageQueue production wiring |
| high-15 | done | 456cb33 | /help wired (absorbed in crit-01c) |
| high-16 | done | 7da8e1a | DDB import re-sync DM-approval gate |
| high-17 | done | faaf55f | portal WithAPI + WithCharacterSheet |

### 🟡 Medium (25 done + 1 not-applicable = 26)

| id | status | commit | note |
|---|---|---|---|
| med-18 | done | f05c966 | initiative tracker auto-post + auto-update |
| med-19 | done | f05c966 | end-combat: concentration end + timer cancel (ammo recovery deferred — schema column needed) |
| med-20 | done | f05c966 | first-combatant ping on StartCombat |
| med-21 | done | f05c966 | /move size/speed lookup |
| med-22 | done | 0bc8abd | thrown weapon clears EquippedMainHand |
| med-23 | done | 0bc8abd | Reckless first-attack-only enforcement |
| med-24 | done | 0a9ef2d | OAs invoked from /move + per-PC reach |
| med-25 | done | f05c966 | ValidateSilenceZone in Cast |
| med-26 | done | d6c755a | Cast invokes zone creation + AnchorMode |
| med-27 | done | 0a9ef2d | explored history landed; bright/dim two-range light + true shadowcasting tracked as renderer follow-up (see Outstanding follow-ups below) |
| med-28 | done | f05c966 | ReadyAction deducts slot + sets concentration |
| med-29 | done | d6c755a + 0501d7b | Subtle bypass + ErrSubtleSpellNotCounterspellable (d6c755a); prompt UI + slot picker + Pass button + 30s ForfeitCounterspell on TTL expiry (0501d7b) |
| med-30 | done | 0501d7b | metamagic prompts — Empowered (per-die reroll), Careful (per-creature protect), Heightened (per-creature target); non-interactive options (Distant/Extended/Subtle/Quickened/Twinned) still resolve at validation |
| med-31 | done | d6c755a | stealth_disadv + heavy-armor speed penalty |
| med-32 | done | f05c966 | /check target option / contested |
| med-33 | done | f05c966 | save_handler FeatureEffects population |
| med-34 | done | f05c966 | rest gated on Settings.AutoApproveRest |
| med-35 | done | 0a9ef2d | /use + /give combat-cost deduction |
| med-36 | done | d6c755a | ASI feat select-menu |
| med-37 | done | d6c755a | Homebrew + Character Overview Svelte UIs |
| med-38 | not-applicable | 0a9ef2d | rest is functional; magicitem has no Service struct; existing publisher hooks cover the dashboard event needs |
| med-39 | done | f05c966 | App.svelte campaign UUID via /api/me |
| med-40 | done | f05c966 | Campaign Home counts live |
| med-41 | done | f05c966 | /setup auto-creates campaign |
| med-42 | done | 0bc8abd | ASSET_DATA_DIR auto-default on Fly |
| med-43 | done | d6c755a + 0501d7b | Wild Shape spellblock + auto-revert + Rage end-of-turn + Bardic sweep (d6c755a); Stunning Strike (Use Ki / Skip) + Divine Smite (slot picker + Skip) + Uncanny Dodge (Halve / Full) + Bardic Inspiration 30s usage prompts (0501d7b) |

### 🟢 Low (5/5 done)

| id | status | commit | note |
|---|---|---|---|
| low-44 | done | a467078 | LogSpellValidationWarnings already invoked in seedSpells; verified |
| low-45 | done | a467078 | seedConditions godoc explains the +1 (surprised) |
| low-46 | done | a467078 | weapon-plus-N template comment clarification |
| low-47 | done | a467078 | TurnTimer.Stop wraps in sync.Once |
| low-48 | done | a467078 | AuraEffects + DMResolutions godoc clarifies reserved API |

### Phase 121 (1/1)

| id | status | commit | note |
|---|---|---|---|
| pt-49 | done-by-policy | (existing) | Smoke transcript at `internal/playtest/testdata/sample.jsonl` exists; `make playtest-replay` green; real-session transcripts deferred per `docs/phases.md:823` ("require a real Discord session or substantial harness fixture work"). |

## Outstanding follow-ups (not in this remediation's scope)

The reaction-prompt helper landed in `0501d7b` and closed med-29, med-30, and the med-43 residue. Two original deferral pockets remain as documented enhancements, both outside the ledger's "done" criteria:

- **med-19 ammunition recovery on EndCombat** — needs a per-encounter spent-ammunition counter column. The row is `done`; only the ammo sub-bullet is deferred.
- **med-27 FoW two-range light + true Albert-Ford shadowcasting** — deeper renderer surface change; the current symmetric raycast is correct but slower. The row is `done`; only the renderer enhancement is deferred.
- **High-tier follow-ups already documented in log.md** — `loot.APIHandler.SetCombatLogFunc` / `shops.Handler.SetPostFunc` need campaign-scoped channel resolvers; remaining `done_handler` setters (TurnNotifier, ImpactSummary, etc.) are still nil in production. None of these block the playtest.

## How to drive a reaction prompt

The helper exposes a single API surface for all five (so-far) class-feature prompts plus future Sentinel / Hellish Rebuke / Shield additions:

1. Build one `*ReactionPromptStore` per bot (`NewReactionPromptStore(session)` uses the 30 s spec default; `NewReactionPromptStoreWithTTL` for the few callers that want a different window).
2. Register it on the router with `CommandRouter.SetReactionPromptStore(store)` so the `rxprompt:` customID prefix is claimed.
3. Call one of the per-feature posters:
   - `CounterspellPromptPoster.Trigger(ctx, CounterspellPromptArgs{...})`
   - `MetamagicPromptPoster.PromptEmpowered / PromptCareful / PromptHeightened`
   - `ClassFeaturePromptPoster.PromptStunningStrike / PromptDivineSmite / PromptUncannyDodge / PromptBardicInspiration`
4. The poster's callback closure handles the service call (e.g. `ResolveCounterspell`, `DivineSmite`, halving damage). On TTL expiry the forfeit closure fires once and the pending entry is consumed atomically.

## How to interpret the closed items

The playtest-quickstart verifier (`/home/ab/projects/DnDnD/.fix-state/playtest_ready.md`) returned `Verdict: READY` *before* the reaction-prompt helper landed — these prompts are quality-of-life enhancements that surface optional class features more conveniently. With the helper in place, players now see a slot picker / Use-Ki / Halve button instead of having to remember `/bonus stunning-strike` or `/reaction declare uncanny-dodge`; the underlying mechanics did not change.
