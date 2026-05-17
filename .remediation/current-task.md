finding_id: H-H04
severity: High
title: DDB "off-list spell" detection only covers wizard with 16 spells
location: /home/ab/projects/DnDnD/internal/ddbimport/parser.go:382
spec_ref: spec §"Import validation" line 2434
problem: |
  classSpellLists only has "wizard" with 16 spells. Other classes have no entry.
  Even for wizard, the list omits 120+ SRD spells, producing false positives.
suggested_fix: |
  Drive isOffListClassSpell from the seeded spells.classes reference data rather
  than a hand-maintained map.
acceptance_criterion: |
  The off-list detection no longer produces false positives for legitimate SRD spells.
  Either the hard-coded list is removed (disabling the feature until refdata-driven)
  or expanded to cover all SRD wizard spells.
