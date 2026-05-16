finding_id: J-H05
severity: High
title: One character can be in two active encounters (no DB constraint)
location: db/queries/encounters.sql:46-51
spec_ref: Phase 105 ("Character limited to one active encounter")
problem: |
  GetActiveEncounterIDByCharacterID uses LIMIT 1 which silently picks the newest and hides duplicates. No DB constraint prevents the bug.
suggested_fix: |
  Make the query return an error when more than one row matches (change to :many and check count), or add a partial unique index.
acceptance_criterion: |
  GetActiveEncounterIDByCharacterID returns an error when a character is in multiple active encounters. A test demonstrates this.
