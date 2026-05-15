finding_id: F-C01
status: done
files_changed:
  - dashboard/svelte/src/lib/api.js
  - dashboard/svelte/src/ActiveReactionsPanel.svelte
  - dashboard/svelte/src/ActiveReactionsPanel.test.js
test_command_that_validates: cd dashboard/svelte && npx vitest run src/ActiveReactionsPanel.test.js
acceptance_criterion_met: yes
notes: Added `isCounterspellReaction` helper and `triggerCounterspell` API function to api.js. The Svelte component now imports these and renders a "Trigger Counterspell" button (purple, matching the readied-badge style) only for reactions whose description contains "counterspell" (case-insensitive). The button POSTs to the backend TriggerCounterspell endpoint with minimal defaults (enemy_spell_name: "Unknown", enemy_cast_level: 1, is_subtle: false). Three tests validate: the helper detects counterspell descriptions correctly, and the API function calls the correct endpoint with the correct method/body.
follow_ups:
  - Consider adding a modal/prompt for DMs to specify spell name and cast level before triggering (instead of hardcoded defaults)
  - Add component-level rendering test once @testing-library/svelte is added to devDependencies
