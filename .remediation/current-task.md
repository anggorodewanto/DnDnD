finding_id: I-H01
severity: High
title: Dashboard DM-created chars miss background skill proficiencies
location: internal/dashboard/charcreate.go:221-253, 117
spec_ref: Spec §Manual Character Creation step 4; Phase 93a
problem: |
  classSkillProficiencies returns only the primary class's default skills. Background skill grants (acolyte → insight+religion, etc.) are never applied.
suggested_fix: |
  Apply the background-skill table before computing SkillModifier in DeriveDMStats.
acceptance_criterion: |
  A DM-created character with background "acolyte" gets insight and religion proficiencies. A test demonstrates this.
