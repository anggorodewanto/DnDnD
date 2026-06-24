# AI Playtest Harness — Ledger (live track record)

> This is our **memory**. Update it **every session**. See
> [`README.md`](README.md) for how we work. The **current task** is always the
> first row below whose status is not `DONE`/`AUTOMATED`.

## Lifecycle / status legend

- `TODO` — not started.
- `EXPLORE` — understanding how the step works today.
- `AUTHOR` — building mode: running the step interactively, deciding assertions.
- `CRYSTALLIZE` — turning the confirmed step into a replayable case.
- `AUTOMATED` — case runs unattended (e.g. `make playtest-replay` / `make e2e`); green.
- `DONE` — non-test tasks (e.g. exploration) that are complete.
- `BLOCKED` — waiting on a decision or a dependency (note why).

---

## Decisions log

Record every settled decision here so a fresh agent doesn't re-litigate it.

| Date | Decision |
| --- | --- |
| 2026-06-19 | Harness goal = **all four** (bug hunting / acceptance / regression / living docs); achieved via the per-step lifecycle, not separate suites. |
| 2026-06-19 | Autonomy = **interactive while building a case**, **unattended once crystallized**. |
| 2026-06-19 | Discord-driving mechanism = **decide after exploration** (STEP-000 must recommend). |
| 2026-06-19 | **(STEP-000)** Mechanism = **hybrid anchored on the in-process e2e harness**. Player actions via `InjectInteraction`/`PlayerCommand` + assert `fake.Transcript()`; crystallize to `.jsonl`, run with `make playtest-replay`. DM setup via `/setup` + `SeedCampaign`/dashboard APIs. Real Discord (`cmd/playtest-player` paste flow) = periodic human-assisted smoke only, never in the auto loop. *Why:* bot-to-bot slash invocation is forbidden + user-token automation violates ToS, so real-Discord player input can't be automated; the harness runs the real router/handlers/DB with only the Discord wire faked. (Awaiting user sign-off.) |
| 2026-06-19 | **(STEP-000)** Crystallized cases = `.jsonl` in `internal/playtest/testdata/`, replayed by `TestE2E_ReplayFromFile`; `observed` `content` lines ARE the assertion (substring match after `DefaultNormalize`). |
| 2026-06-19 | **Real-Discord lane = human-assisted manual smoke only** (playtest-player paste). **Rejected: automating discordo / any self-bot** — user-token automation violates Discord ToS + risks account ban; "avoid detection" jitter = enforcement evasion, out of scope. No compliant way to auto-invoke another app's slash commands as a user. |
| 2026-06-19 | **Jitter / randomness / varied-content → build as a harness FUZZING + timing layer** (randomized inter-command timing + varied input on the in-process harness, to surface races/timing/input bugs). NOT a real-Discord evasion feature. Backlog item, after core steps. |
| 2026-06-19 | **STEP-001 = Player `/register`** (first authored step). |
| 2026-06-19 | **(STEP-001)** Crystallize route = **B**: per-transcript preconditions sidecar (`<transcript>.preconditions.json`) + `.jsonl` for Discord-visible assertions; DB row stays in the existing Go scenario. `.jsonl` replay can't assert DB — deferred replay-engine enhancement if ever needed. |
| 2026-06-20 | **(STEP-002)** Authored full `/move` in 2 increments (002a preconditions+confirm, 002b click model). Added transcript `click` direction (selector = CustomID **prefix**, since live CustomID embeds runtime UUIDs) + `Clicker` engine interface + `ClickButton` harness helper + preconditions `encounter` block. These generalize to all confirm-gated commands. |
| 2026-06-20 | **(STEP-003)** First DM/admin step. Authored `/setup` **existing-campaign DM happy path** (gate = identity only, no admin bit → no new permission infra). Added `dispatchAsDM` precondition (re-targets dispatcher at the seeded campaign's random `DmUserID`). `/setup` defers → success arrives via `InteractionResponseEdit`, so the `.jsonl` needs an **empty first observed line** for the deferred ack (observer is strictly sequential). Fake doesn't record channel creation → DB lock (10 channel IDs in settings JSONB) lives in net-new `TestE2E_SetupScenario`. **Deferred:** auto-create admin path + rejection paths need harness `Member.Permissions` support. → **Resolved by STEP-004.** |
| 2026-06-20 | **(STEP-005)** Picked player `/attack` via QnA. Built reusable combat infra: `withRoller` runOption (deterministic dice — harness boots `withRoller(dice.NewRoller(e2eDefaultRoll))`, an always-max die → d20 nat 20) + NPC-target seeding (`SeedNPCCombatant`, `isNpc` precondition flag; target NPCs by **grid coordinate** since their ShortID is randomised). **BUG FOUND while authoring (user chose: log + spin a fixing agent):** `/attack` announced the hit + damage and spent the attack resource but **never applied damage to the target's HP** — only the *secondary* Graze (mastery.go:342) and Cleave (mastery.go:113) damage routed through `combat.ApplyDamage`; the *primary* hit did not. Fix = a shared `Service.applyHitDamage` helper (extracted during /simplify) called after `resolveAndPersistAttack` in `Service.Attack`/`OffhandAttack`/`attackImprovised`, gated on `Hit && DamageTotal>0`, so R/I/V + temp-HP + death-saves + concentration fire identically to Graze/Cleave. Roll crystallized = NAT 20 crit (always-max → longsword 2d8+3=19; goblin 20→1 HP after fix). |
| 2026-06-20 | **(STEP-005 /simplify — NEW BUG LOGGED, NOT fixed)** The /simplify altitude pass found **two sibling instances of the same damage-not-applied bug** still open: `Service.MartialArtsBonusAttack` (monk.go:88, a 4th caller of `resolveAndPersistAttack`) and `Service.FlurryOfBlows` (monk.go:117, calls `ResolveAttack` directly) both resolve a hit, log it, spend resources, but apply **0 HP** to the target. Out of STEP-005 scope (behavior change, separate methods) → **logged, awaiting user decision** on whether to fix (candidate next step: "centralize primary-hit damage in `resolveAndPersistAttack` + audit all attack paths"). Note: centralizing in `resolveAndPersistAttack` would fix `MartialArtsBonusAttack` for free (it already funnels through that seam) but NOT FlurryOfBlows (separate path). |
| 2026-06-22 | **(STEP-006)** Fixed the 2 sibling damage bugs logged by STEP-005's `/simplify` (monk `MartialArtsBonusAttack` + `FlurryOfBlows` — announced a hit, spent resources, applied **0 HP**). Fix = extend shared `applyHitDamage` to return the post-damage combatant; call it at each monk site (flurry threads `dmgTarget` across its 2 strikes so the 2nd computes from post-1st HP). **Altitude decision (sustained by `/simplify` panel): per-call-site, NOT centralized in `resolveAndPersistAttack`** — because flurry bypasses that seam (centralizing wouldn't cover it) and it'd force deleting the 3 working calls. `/simplify` grep-verified the contract is now consistent across all 5 attack producers + no other combat method drops primary-hit damage (Cleave=different target; StunningStrike/DivineSmite=riders). User chose **extend harness + crystallize e2e** → added `SeedApprovedMonk` (classes raw-SQL, ki via `UpdateCharacterFeatureUses`) + `MarkTurnActionUsed` (raw SQL; no `action_used` on `CreateTurnParams`) + manifest `class/level/ki` + `turnHolderActionUsed`. Locked at 2 altitudes (unit red→green exact math + e2e DB-HP) + 2 `.jsonl` replays. 14/14 e2e green; cover-check clean. **Open follow-up (logged, not done):** convention-only invariant — candidate STEP-007 structural guard test. |
| 2026-06-24 | **(STEP-007)** Picked "initiative seeding" + **Full `Service.StartCombat`** scope + **Distinct DEX** determinism via QnA. Key finding: **initiative has no slash command** — it's the dashboard REST flow `POST /api/combat/start` → `Service.StartCombat` → `RollInitiative` (d20+DEX, sort roll-desc/dex-desc/name-asc, write `initiative_order`, activate encounter) → `AdvanceTurn`. So the artifact is a **Go scenario driving the real service directly** (not a `.jsonl`), matching the STEP-003/004 dashboard-action pattern. Added a production seam `withCombatServiceReady(func(*combat.Service))` (mirrors `withCommandRouterReady`; fired next to `onRouterReady`; no-op in prod; `main.go` is coverage-excluded) so the harness can reach the fully-wired (publisher+notifiers) service. **Determinism:** always-max roller ⇒ every d20=20 ⇒ order driven by DEX modifier; distinct PC DEX (20/6) bracketing the SRD goblin (14) gives a deterministic, *reorder-proving* order. **No bug found** — confirms current behavior is correct standard D&D. |
| 2026-06-20 | **(STEP-004)** Authored remaining `/setup` permission paths (admin auto-create success + non-DM reject + non-admin reject). Added harness **permission injection**: `PlayerCommandWithPermissions` + `dispatchAsAdmin` precondition (sets `discordgo.PermissionAdministrator`). Non-DM/non-admin identities need no new field (plain non-DM `player` / default perms 0). **First step to FIX a bug found while authoring (user signed off):** `GetCampaignForSetup` persisted the auto-created campaign *before* the admin gate, so a rejected non-admin silently became DM of a real campaign. Fixed by splitting the lookup into `FindCampaignForSetup` (no create) + `CreateCampaignForSetup` and gating-before-create; dropped unused `SetupCampaignInfo.AutoCreated`. DB-lock + regression scenarios in `TestE2E_SetupAutoCreateScenario` / `TestE2E_SetupRejectsNonAdminWithoutCreatingCampaign`. |

### Open questions awaiting answers

- Q1: ~~How does the AI drive Discord?~~ **RESOLVED** by STEP-000 (see decisions). Awaiting user sign-off.
- Q2: Smallest first real-world step to author? → proposed to user (STEP-000 QnA). Candidates: DM `/setup`, player `/register`, player `/move`.
- Q3: ~~Crystallized-case format + location?~~ **RESOLVED**: `internal/playtest/testdata/*.jsonl`.
- Q4 (new): Do we want a true real-Discord smoke lane at all, or harness-only? → ask user.
- Q5: ~~Fix the 2 logged monk sibling damage bugs?~~ **RESOLVED** by STEP-006 (fixed + unit + e2e locked). No remaining unfixed primary-hit-damage paths.

---

## Steps

| ID | Real-world step | Phase | Artifact (test/case path) | Notes / bugs |
| --- | --- | --- | --- | --- |
| STEP-000 | Explore existing harness & refine the plan | `DONE` | docs only | Inventory confirmed (README §6), mechanism recommended (README §7). 5 reference scenarios + record/replay already exist. |
| STEP-001 | Player `/register` (create character) | `AUTOMATED` ✅ | `internal/playtest/testdata/register.jsonl` (+`.preconditions.json`) | `make playtest-replay TRANSCRIPT=…/register.jsonl` → PASS. Added per-transcript preconditions to `cmd/dndnd/e2e_replay_test.go`. DB row locked by `TestE2E_RegistrationScenario`. See [steps/STEP-001-player-register.md](steps/STEP-001-player-register.md). |
| STEP-002 | Player `/move` one tile (button-confirm) | `AUTOMATED` ✅ | `internal/playtest/testdata/move.jsonl` (+`.preconditions.json`) | Full confirm flow. Added: preconditions `encounter` block, transcript `click` direction, `Clicker`/`harnessClicker`, `ClickButton` helper. DB position locked by `TestE2E_MovementScenario`. See [steps/STEP-002-player-move.md](steps/STEP-002-player-move.md). |
| STEP-003 | DM `/setup` (build channel structure) | `AUTOMATED` ✅ | `internal/playtest/testdata/setup.jsonl` (+`.preconditions.json`) + `TestE2E_SetupScenario` | First DM/admin step. Authored existing-campaign DM happy path (identity gate, no admin bit). Added `dispatchAsDM` precondition. DB lock (10 channel IDs persisted) in `TestE2E_SetupScenario`. Replay + scenario PASS; full e2e green. See [steps/STEP-003-dm-setup.md](steps/STEP-003-dm-setup.md). |
| STEP-004 | DM `/setup` permission paths (admin auto-create + non-DM/non-admin rejects) | `AUTOMATED` ✅ | `internal/playtest/testdata/setup_autocreate_admin.jsonl`, `setup_reject_nondm.jsonl`, `setup_reject_nonadmin.jsonl` (+`.preconditions.json`) + `TestE2E_SetupAutoCreateScenario` + `TestE2E_SetupRejectsNonAdminWithoutCreatingCampaign` | Added harness permission injection (`PlayerCommandWithPermissions` + `dispatchAsAdmin`). **Found + fixed an auth bug:** non-admin reject silently created a campaign (gate ran after persistence) → split lookup into Find/Create, gate-before-create. All replays + 11/11 e2e green; cover-check clean (refdata pre-existing). See [steps/STEP-004-dm-setup-permissions.md](steps/STEP-004-dm-setup-permissions.md). |
| STEP-005 | Player `/attack` (one-shot, active combat) | `AUTOMATED` ✅ | `internal/playtest/testdata/attack.jsonl` (+`.preconditions.json`) + `TestE2E_AttackScenario` | First combat-action step. Added reusable infra: `withRoller` runOption (deterministic always-max dice) + `SeedNPCCombatant` / `isNpc` precondition (coordinate-targeted NPC). **Found + fixed a combat bug:** `/attack` never applied primary-hit damage to the target (only Graze/Cleave did) → shared `Service.applyHitDamage` helper now routes it through `combat.ApplyDamage`. Crystallized the deterministic NAT-20 longsword crit (19 dmg, goblin 20→1 HP). Replay + scenario PASS; 12/12 e2e green; cover-check clean. **/simplify logged 2 sibling bugs** (monk `MartialArtsBonusAttack`, `FlurryOfBlows` — same gap, unfixed, awaiting decision). See [steps/STEP-005-player-attack.md](steps/STEP-005-player-attack.md). |
| STEP-006 | Fix 2 monk primary-hit damage bugs (`/bonus martial-arts` + `/bonus flurry-of-blows`) | `AUTOMATED` ✅ | `internal/playtest/testdata/bonus_martial_arts.jsonl`, `bonus_flurry.jsonl` (+`.preconditions.json`) + `TestE2E_MartialArtsBonusAttackScenario` + `TestE2E_FlurryOfBlowsScenario` + unit tests in `internal/combat/monk_test.go` | Closes the STEP-005 `/simplify` open item. Both monk paths announced a hit + spent resources but applied **0 HP**; now routed through shared `applyHitDamage`→`ApplyDamage` (extended to return the post-damage combatant so flurry threads HP across its 2 strikes). Locked at 2 altitudes: unit (red→green, exact math) + e2e DB-HP. Harness gained `SeedApprovedMonk` + `MarkTurnActionUsed` + manifest `class/level/ki` + `turnHolderActionUsed`. `/simplify` confirmed the fix is **complete** (no other combat method drops primary-hit damage) and the per-call-site altitude is right. 14/14 e2e green; cover-check clean (combat 91.6%). See [steps/STEP-006-monk-damage-bugs.md](steps/STEP-006-monk-damage-bugs.md). |
| STEP-007 | Initiative seeding — DM starts combat (real `StartCombat`/`RollInitiative`) | `AUTOMATED` ✅ | `cmd/dndnd/e2e_scenarios_test.go` `TestE2E_InitiativeScenario` | First **dashboard/service** play step driven directly (no slash command — initiative = `POST /api/combat/start`). Drove real `Service.StartCombat` end-to-end via new `withCombatServiceReady` seam; replaced the hardcoded `PromoteEncounterToActive` initiative fake. Distinct-DEX combatants prove `SortByInitiative` reorders (Bram added after the goblin → order 1). Added infra: `withCombatServiceReady`/`onCombatServiceReady` (`main.go`), `e2eHarness.combatService`, `SeedEncounterTemplate`, `SeedApprovedPlayerWithDex`. **No bug found** (confirms current behavior). 15/15 e2e green; cover-check clean. No `.jsonl` (service action, not a player command). See [steps/STEP-007-initiative-seeding.md](steps/STEP-007-initiative-seeding.md). |
| STEP-008 | *(next — pick via QnA)* | `TODO` | — | Candidates: `/done` end-turn (turn-cycle via real `AdvanceTurn` — now reachable through STEP-007's combat-service seam; no dice) · `/cast` (reuses dice infra; concentration/slots) · **structural guard**: one table-driven test asserting every public attack-producing `Service` method writes `UpdateCombatantHP` on a hit (prevents a 6th method re-introducing the STEP-005/006 bug class) · conditions/concentration on the character card (deferred fields). |

### Refined backlog (smallest-first; mechanics noted)

Order reflects the real play journey **and** harness mechanics. "Covered?" =
whether a reference `TestE2E_*Scenario` already exists to template from.

| # | Real-world step | How to drive | Covered? |
| --- | --- | --- | --- |
| pre | Campaign exists | `SeedCampaign` (dashboard API in real life) | n/a (seeded) |
| 1 | DM runs `/setup` (build channel structure) | inject `/setup` interaction | ✅ STEP-003 (DM happy path) + STEP-004 (admin auto-create + rejections) |
| 2 | Player `/register` (create character) | `PlayerCommand` | yes (Registration) |
| 3 | DM approves the character | dashboard API / seed `SeedApprovedPlayer` | partial |
| 4 | DM starts an encounter | dashboard API / seed | partial |
| 5 | DM loads a map / places tokens | dashboard API / seed | partial |
| 6 | Player `/move` one tile | `PlayerCommand` | yes (Movement) |
| 7 | Combat: initiative | real `StartCombat`/`RollInitiative` via combat-service seam | ✅ STEP-007 |
| 8 | Player `/attack` or `/cast` (one action) | `PlayerCommand` | ✅ STEP-005 (`/attack`); `/cast` still open |
| 9 | Damage + condition applied | assert transcript + DB | partial (STEP-005 locks attack HP-apply; conditions open) |
| 10 | Player `/loot`, `/give`, `/save`, `/recap` | `PlayerCommand` | yes (Loot/Save/Recap) |

> Note: "Covered" scenarios are *reference examples*, not proof the journey is
> regression-locked. Re-walking them as small authored steps is still valuable —
> it confirms current behavior and turns each into a maintained `.jsonl` case.

### Cross-cutting feature backlog (not a play step)

- **Harness fuzzing + timing layer** — randomized inter-command timing (jitter)
  and varied input content driven into the in-process harness to surface race
  conditions / timing / input-handling bugs. Deterministic when seeded. Build
  *after* a few core steps exist to fuzz against. (See decisions log, 2026-06-19.)

---

## Session log

Append a short entry per working session: date, step touched, what happened,
what's next.

| Date | Step | What happened | Next |
| --- | --- | --- | --- |
| 2026-06-19 | — | Set up `docs/ai-playtest/` (README + this ledger). Captured mission, lifecycle, modes, rules. | Run STEP-000 (explore + refine plan). |
| 2026-06-19 | STEP-000 | Explored harness via 2 read-only subagents + verified claims. Found in-process e2e harness (real router/DB, fake Discord wire) + record/replay + 5 reference scenarios. Recommended hybrid mechanism. Updated README §6/§7 + ledger. | User sign-off on mechanism + pick STEP-001 (QnA sent). |
| 2026-06-19 | STEP-001 | EXPLORE'd `/register` (subagent). Hit harness limit: replay seeding fixed → built per-transcript preconditions feature. Crystallized `register.jsonl` (+sidecar); replay PASS first run; backward compat + gofmt/vet clean. | Commit/push (ask user), then pick STEP-002. |
| 2026-06-19 | STEP-001 | Committed+pushed (489e0d1). Demo'd drift (red) on a temp copy to prove the assertion bites. | Start STEP-002. |
| 2026-06-20 | STEP-002 | EXPLORE'd `/move` (subagent). Built encounter preconditions (002a, confirm ephemeral green) + transcript click model & Clicker (002b). `move.jsonl` PASS; full regression green (playtest unit, replay group, movement+registration scenarios). gofmt/vet clean. | Commit/push, pick STEP-003. |
| 2026-06-20 | STEP-002 | Committed+pushed (cec763f). **Session paused at user request** — clean stopping point: STEP-001 + STEP-002 + reusable infra (preconditions incl. encounter, dispatch/observe/click model, Clicker/ClickButton) all on `main`. | Resume: read README.md + this ledger; pick STEP-003 (candidates: `/attack` reusing click model, or DM `/setup`). |
| 2026-06-20 | STEP-003 | Picked DM `/setup` via QnA. EXPLORE'd via 2 subagents + verified handler/wiring/fake by hand. Built `dispatchAsDM` precondition; crystallized `setup.jsonl` (+sidecar) — replay PASS first run. User signed off behavior + asked for DB lock → added `TestE2E_SetupScenario` (10 channel IDs in settings JSONB). Full e2e green (9/9), gofmt/vet clean. | Commit/push (ask user), then pick STEP-004. |
| 2026-06-20 | STEP-005 | Picked player `/attack` via QnA. Scoped via 2 read-only subagents (combat cmd landscape + harness seeding gaps). Built infra (`withRoller` deterministic dice + `SeedNPCCombatant`/`isNpc`). AUTHOR surfaced a bug: primary-hit damage never applied to target HP. User chose log+fix → delegated a TDD fixing agent (routed through `ApplyDamage`; red/green + faithful mock updates; 3968 combat+discord tests green). Crystallized `attack.jsonl` (NAT-20 crit, HP 20→1) + `TestE2E_AttackScenario`; replay + 12/12 e2e green; cover-check clean. /simplify deduped the 3 ApplyDamage blocks into `applyHitDamage`, removed dead roller-swap machinery, and **logged 2 more sibling damage bugs** (monk paths). | Commit/push (ask user); decide STEP-006 (fix sibling bugs vs `/cast`). |
| 2026-06-22 | STEP-006 | Picked "fix 2 monk damage bugs" via QnA. EXPLORE'd monk paths + STEP-005 fix pattern (1 subagent for e2e feasibility + hand-read tests/handler). TDD red→green: 2 unit tests prove 0 HP applied, then extended `applyHitDamage` to return post-damage combatant + wired both monk paths (flurry threads across 2 strikes). User chose **extend harness + e2e** → built `SeedApprovedMonk`/`MarkTurnActionUsed` + manifest fields; authored 2 e2e scenarios (DB-HP locks) + crystallized 2 `.jsonl`. Ran `/simplify` (4-agent panel): confirmed fix complete + right altitude; applied cleanups (typed `UpdateCharacterFeatureUses`, dropped dead re-read, comment fixes). 14/14 e2e + 2 replays green; cover-check clean (combat 91.6%). | Commit/push (ask user); pick STEP-007 (candidates: `/cast`, `/done`, initiative seeding, or the structural damage-guard test). |
| 2026-06-24 | STEP-007 | Picked "initiative seeding" + Full StartCombat + Distinct-DEX via QnA. EXPLORE'd via 2 read-only subagents + hand-verified `StartCombat`/`RollInitiative`/run-options/notifier wiring. Found initiative = dashboard/service action (no slash cmd). Built `withCombatServiceReady` seam + `SeedEncounterTemplate`/`SeedApprovedPlayerWithDex`; authored `TestE2E_InitiativeScenario` driving real StartCombat (distinct-DEX → reorder-proving order). PASS first run; no bug found. Full e2e suite (15) green; cover-check clean; gofmt/vet clean. User confirmed behavior + commit. | Commit/push; pick STEP-008 (candidates: `/done` end-turn via real AdvanceTurn, `/cast`, structural damage-guard). |
| 2026-06-20 | STEP-004 | Picked `/setup` admin-path+rejections via QnA. EXPLORE'd via 2 subagents + verified source by hand. TDD: wrote 3 transcripts (admin-success RED until knob wired; 2 rejects green) → added `PlayerCommandWithPermissions`/`dispatchAsAdmin`. AUTHOR surfaced an auth bug (non-admin reject persisted a campaign before the gate); user chose **fix now** → split `CampaignLookup` into Find/Create, gate-before-create, dropped `AutoCreated`; updated mock + 6 unit tests + 6 adapter tests. Added e2e DB-lock + regression scenarios (RED→GREEN proven). 11/11 e2e green; gofmt/vet clean; cover-check overall 90.58%, discord 85.74% (refdata 84.13% pre-existing). | Commit/push (ask user), then pick STEP-005 (combat: `/attack` or initiative seeding). |
