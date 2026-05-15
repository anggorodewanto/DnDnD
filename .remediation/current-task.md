finding_id: H-C04
severity: Critical
title: DDB import bypasses DM approval queue on first import
location: internal/ddbimport/service.go:139
spec_ref: spec §"D&D Beyond Import" line 2426
problem: |
  On a fresh import (no existing DDB-URL row), Service.Import calls CreateCharacter immediately, mutating the DB before the DM has seen the preview. Re-syncs are correctly staged via pending_ddb_imports, but the first import inserts a live character row.
suggested_fix: |
  Mirror the re-sync path for first imports: stage the create-params in pending_ddb_imports keyed by a new id and only call CreateCharacter from ApproveImport. Or at minimum mark the character row inactive (approval_status='pending_ddb') until DM clicks Approve.
acceptance_criterion: |
  A first-time DDB import does NOT create a live character row immediately. Instead it stages the import for DM approval. A test demonstrates the character is not created until approval.
