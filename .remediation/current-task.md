finding_id: H-H11
severity: High
title: DDB class names not normalised to internal IDs
location: internal/ddbimport/parser.go:177
spec_ref: Phase 90 "Parser converts DDB JSON into internal format"
problem: |
  DDB returns capitalised class names ("Fighter", "Wizard"). The parser stores them verbatim, while the rest of the system uses lowercase slugs ("fighter"). Imported characters can't level up via the dashboard.
suggested_fix: |
  Lowercase/slugify DDB class names before storing.
acceptance_criterion: |
  Parsed class names from DDB are lowercased. A test demonstrates "Fighter" becomes "fighter".
