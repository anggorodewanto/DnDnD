# Remediation Progress Log

Append-only journal of all remediation activity.

---

## 2026-05-15T15:11 — Queue initialized

- 448 findings parsed from 11 review files
- Critical: 35, High: 98, Medium: 173, Low: 142
- Branch: fix/review-findings-all

## 2026-05-15T15:15 — A-C01 done

- Finding: `/setup` lets any guild member silently become the campaign DM
- Commit: e2d1c33
- Reviewer: approved
- Notes: Two early-return auth guards added to SetupHandler.Handle

## 2026-05-15T15:18 — A-C02 done

- Finding: Dashboard approval endpoints aren't scoped to the DM's own campaign
- Commit: c9e55e9
- Reviewer: approved
- Notes: checkCampaignOwnership guard added to all 3 mutation endpoints

## 2026-05-15T15:21 — B-C01 done

- Finding: ParseExpression mangles modifiers with multiple +/- operators
- Commit: 9790feb
- Reviewer: approved
- Notes: sumSignedTokens helper replaces broken strip-and-concat approach

## 2026-05-15T15:24 — B-C02 done

- Finding: cryptoRand / RollD20 panic on degenerate dice (Nd0)
- Commit: 6de7aa2
- Reviewer: approved
- Notes: 3-line validation guard in ParseExpression rejects Count<1 or Sides<1

## 2026-05-15T15:27 — C-C01 done

- Finding: Multi-letter column labels truncated by colToIndex
- Commit: 78488e3
- Reviewer: approved
- Notes: Base-26 conversion loop replaces single-byte parsing

## 2026-05-15T15:30 — C-C02 done

- Finding: Reckless Attack advantage missing on attacks 2+
- Commit: 560deac
- Reviewer: approved
- Notes: Added "reckless" case to attacker-conditions in DetectAdvantage, gated on melee+STR

## 2026-05-15T15:33 — C-C03 done

- Finding: Off-hand (TWF) attack lacks Attack-action prerequisite and melee weapon check
- Commit: 9c25f59
- Reviewer: approved
- Notes: Added attack-taken prerequisite + IsRangedWeapon checks for both hands

## 2026-05-15T15:36 — C-C04 done

- Finding: /fly performs no fly-speed validation
- Commit: c3866db
- Reviewer: approved
- Notes: HasFlySpeed field + CombatantHasFlySpeed helper checks conditions for fly_speed

## 2026-05-15T17:56 — D-C01 done

- Finding: Rage damage resistance never fires for seed-created barbarians
- Commit: e5446b1
- Reviewer: approved
- Notes: Changed seed mechanical_effect from descriptive tokens to "rage" literal

## 2026-05-15T17:59 — D-C02 done

- Finding: Feature uses never initialized at character creation
- Commit: c3fc75c
- Reviewer: approved
- Notes: New InitFeatureUses helper computes all class feature pools at creation time

## 2026-05-15T18:02 — D-C03 done

- Finding: Rage advantage on STR ability checks never wired
- Commit: 0460fe6
- Reviewer: approved
- Notes: Added IsRaging+Ability fields to SingleCheckInput, rage-advantage logic in SingleCheck

## 2026-05-15T18:05 — D-C04 done

- Finding: Save handler never sets IsRaging in EffectContext
- Commit: 05f8934
- Reviewer: approved
- Notes: 4 lines added to populate IsRaging from combatant state in save handler

## 2026-05-15T18:08 — E-C01 done

- Finding: Single-target spell casts never apply damage or healing
- Commit: 758f247
- Reviewer: approved
- Notes: Added damage roll+apply on hit and healing roll+apply paths to Cast()

## 2026-05-15T18:11 — E-C02 done

- Finding: AoE damage path ignores upcasting and cantrip scaling
- Commit: 85f3ff8
- Reviewer: approved
- Notes: Encoded slot/char level in pending save source, ScaleSpellDice called at resolve time

## 2026-05-15T18:14 — E-C03 done

- Finding: Dodge condition does not impose disadvantage on attackers
- Commit: 9dfe265
- Reviewer: approved
- Notes: Single case added to target-conditions switch in DetectAdvantage

## 2026-05-15T18:17 — F-C01 done

- Finding: Counterspell trigger is unreachable from the DM dashboard
- Commit: 244105c
- Reviewer: approved
- Notes: Added Trigger Counterspell button + isCounterspellReaction helper to Svelte panel

## 2026-05-15T18:20 — F-C02 done

- Finding: Heavy-armor STR speed penalty computed but never applied
- Commit: 0f37116
- Reviewer: approved
- Notes: 5 lines in ResolveTurnResources check armor STR req and subtract 10ft penalty
