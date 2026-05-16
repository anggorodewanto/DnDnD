# Worker Report: I-H01

**Worker:** worker-I-H01
**Finding:** Dashboard DM-created chars miss background skill proficiencies
**Status:** ✅ FIXED

## Changes Made

### `internal/dashboard/charcreate.go`

1. Added `backgroundSkillProficiencies(background string) []string` — maps SRD backgrounds to their granted skill proficiencies (acolyte→insight+religion, criminal→deception+stealth, folk hero→animal-handling+survival, noble→history+persuasion, sage→arcana+history, soldier→athletics+intimidation, charlatan→deception+sleight-of-hand, entertainer→acrobatics+performance, hermit→medicine+religion, outlander→athletics+survival, sailor→athletics+perception, urchin→sleight-of-hand+stealth).

2. In `DeriveDMStats`, merged background skills into `skillProfs` via:
   ```go
   skillProfs = append(skillProfs, backgroundSkillProficiencies(sub.Background)...)
   ```

### `internal/dashboard/charcreate_test.go`

1. `TestDeriveDMStats_BackgroundSkillProficiencies_Acolyte` — verifies a Fighter with Acolyte background gets insight (+4) and religion (+2) with proficiency bonus applied.
2. `TestBackgroundSkillProficiencies` — table-driven test covering all 12 SRD backgrounds plus unknown/empty cases.

## Verification

- `make test` — all tests pass
- `make cover-check` — all coverage thresholds met
