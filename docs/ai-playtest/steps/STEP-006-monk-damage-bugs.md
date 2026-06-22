# STEP-006 — Fix the 2 monk primary-hit damage bugs (`/bonus martial-arts`, `/bonus flurry-of-blows`)

> Sibling of the STEP-005 `/attack` bug. STEP-005's `/simplify` pass logged two
> more attack paths that announced a hit + spent resources but never applied
> damage to the target's HP. This step fixes both, locks them with unit + e2e
> tests, and crystallizes replay cases.

## RESULT (2026-06-22) — `AUTOMATED` ✅

- Both monk paths now apply primary-hit damage through the shared
  `combat.ApplyDamage` pipeline — the same seam `/attack`, Graze, and Cleave use.
- Locked at two altitudes: combat **unit tests** (red→green, exact damage math)
  + **e2e scenarios** that assert the target's HP drops in the DB, plus two
    replayable `.jsonl` transcripts.
- Full suite green: combat unit tests, `make cover-check` (combat 91.6%, all
  thresholds met), `make e2e` 14/14, both new replays.

## The bugs (confirmed via red tests)

| Method | File | What it did | Why damage was lost |
| --- | --- | --- | --- |
| `Service.MartialArtsBonusAttack` | `internal/combat/monk.go:88` | rolled the unarmed strike, logged the hit, spent the bonus action — **0 HP applied** | funnels through `resolveAndPersistAttack` but never called `applyHitDamage` |
| `Service.FlurryOfBlows` | `internal/combat/monk.go:~162` | resolved 2 unarmed strikes, logged them, spent ki + bonus action — **0 HP applied** | calls `ResolveAttack` directly ×2, no damage application at all |

## The fix

- Extended the shared `Service.applyHitDamage` helper (`internal/combat/attack.go`)
  to **return the post-damage combatant**: `(refdata.Combatant, error)`. The 3
  pre-existing callers (`Attack`, `attackImprovised`, `OffhandAttack`) ignore it
  with `_, err :=`.
- `MartialArtsBonusAttack`: single `applyHitDamage` call after
  `resolveAndPersistAttack`, gated `Hit && DamageTotal>0` (same as `/attack`).
- `FlurryOfBlows`: applies damage after each `ResolveAttack`, **threading the
  returned combatant** (`dmgTarget`) into the next strike — so the 2nd strike
  computes its HP write from the post-first-strike HP instead of overwriting the
  original snapshot. The to-hit/damage roll still reads `cmd.Target` (AC/position
  unchanged by the first strike).

### Altitude decision (per `/simplify` review)

Kept the **per-call-site** approach over centralizing damage inside
`resolveAndPersistAttack`. Reasons: (1) `FlurryOfBlows` bypasses
`resolveAndPersistAttack` entirely, so centralizing there would NOT cover it;
(2) centralizing would force deleting the 3 working external calls to avoid
double-apply — wider blast radius on the most-exercised path; (3)
`resolveAndPersistAttack` is scoped to roll + turn/visibility persistence and
never touches target HP.

A `/simplify` altitude agent grepped every caller of `resolveAndPersistAttack`
and `ResolveAttack`: the contract is now **consistent across all 5
attack-producing methods**, and no other `internal/combat` method silently
drops primary-hit damage. (Cleave applies its own different-target damage;
`StunningStrike`/`DivineSmite` are save/rider effects, not to-hit producers.)
Residual: the invariant is convention-enforced — a future 6th attack method
could re-introduce the bug. Candidate future guard: one table-driven test
asserting every public attack method writes `UpdateCombatantHP` on a hit.

## Harness features added (reusable)

- `e2eHarness.SeedApprovedMonk(discordUserID, charName, monkLevel, ki)` — seeds
  an approved Monk. `classes` patched via raw SQL (no cheap typed writer); `ki`
  via the `UpdateCharacterFeatureUses` sqlc writer.
- `e2eHarness.MarkTurnActionUsed(turnID)` — flips `turns.action_used=true`
  (bonus-action gate requires the Attack action this turn). `CreateTurnParams`
  has no `action_used` field, so raw SQL.
- Preconditions manifest: `approvedPlayerSeed.{class,level,ki}` +
  `encounterSeed.turnHolderActionUsed`. `class:"monk"` routes seeding to
  `SeedApprovedMonk`; `turnHolderActionUsed:true` calls `MarkTurnActionUsed`
  after promotion.

## Crystallized assertions (deterministic — always-max roller → nat-20 crit)

Monk 5 (1d6 martial-arts die), STR 16 (+3). Each unarmed crit = doubled 2d6 (12)
+ 3 = **15 bludgeoning**.

- **martial-arts**: Goblin 20 → **5 HP** (survives). `.jsonl` asserts combat-log
  `Kira attacks Goblin with Unarmed Strike` + ephemeral `NAT 20 — CRITICAL HIT!`.
- **flurry**: Goblin 20 → 5 → **0 (dead)**; ki 5 → 4. `.jsonl` asserts
  `uses Flurry of Blows! (1 ki spent, 4 remaining)` on combat-log + ephemeral.

## Artifacts

- Production fix: `internal/combat/monk.go`, `internal/combat/attack.go`.
- Unit tests: `internal/combat/monk_test.go` —
  `TestServiceMartialArtsBonusAttack_AppliesDamageToTargetHP`,
  `TestServiceFlurryOfBlows_AppliesDamageToTargetHP` (locks sequential stacking
  20→14→8).
- e2e scenarios: `cmd/dndnd/e2e_scenarios_test.go` —
  `TestE2E_MartialArtsBonusAttackScenario`, `TestE2E_FlurryOfBlowsScenario`
  (DB-HP locks).
- Replays: `internal/playtest/testdata/bonus_martial_arts.jsonl`,
  `bonus_flurry.jsonl` (+ `.preconditions.json` sidecars).
- Harness: `cmd/dndnd/e2e_harness_test.go`, `cmd/dndnd/e2e_replay_test.go`.

## Run it

```sh
go test ./internal/combat/ -run 'AppliesDamageToTargetHP' -count=1
make e2e   # includes TestE2E_{MartialArtsBonusAttack,FlurryOfBlows}Scenario
make playtest-replay TRANSCRIPT=../../internal/playtest/testdata/bonus_martial_arts.jsonl
make playtest-replay TRANSCRIPT=../../internal/playtest/testdata/bonus_flurry.jsonl
```

> Replay `TRANSCRIPT` path is relative to `cmd/dndnd/` (the `go test` working
> dir), hence the `../../` prefix.

## Notes

- `make cover-check` clean (refdata pre-existing pass; combat 91.6%).
- This closes the STEP-005 `/simplify` open item. No new sibling bugs found.
