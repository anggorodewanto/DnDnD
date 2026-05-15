finding_id: F-C01
severity: Critical
title: Counterspell trigger is unreachable from the DM dashboard
location: dashboard/svelte/src/ActiveReactionsPanel.svelte:88-150
spec_ref: spec §Counterspell resolution lines 1093-1101; phases §Phase 72
problem: |
  The backend handler TriggerCounterspell exists at POST /{encounterID}/reactions/{reactionID}/counterspell/trigger, but the ActiveReactionsPanel only renders Resolve/Dismiss buttons. There is no "Trigger Counterspell" button that posts to the backend route, so a DM cannot start the counterspell flow from the UI.
suggested_fix: |
  Add a "Trigger" button on Counterspell-labelled declarations (detect by checking if reaction.description contains "counterspell" case-insensitively) that calls the TriggerCounterspell endpoint. The button should include inputs for spell name and level (or use defaults/prompts).
acceptance_criterion: |
  A reaction declaration containing "counterspell" in its description shows a "Trigger" button in the ActiveReactionsPanel. Clicking it calls the backend endpoint. A test verifies the button renders for counterspell declarations and not for other declarations.
