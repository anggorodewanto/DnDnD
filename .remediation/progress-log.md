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
