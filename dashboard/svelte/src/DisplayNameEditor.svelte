<script>
  // Phase 105c — DM display-name editor.
  //
  // Thin wrapper around lib/displayNameEditor.js. Used by both the Encounter
  // Builder (template-level display name) and the Combat Workspace header
  // (live-encounter display name). The parent supplies the onCommit callback;
  // this component only owns the input UX and transient saving/error state.

  import { commitDisplayName } from './lib/displayNameEditor.js';

  let {
    value = '',
    fallback = '',
    onCommit,
    disabled = false,
    label = 'Display Name',
    placeholder = null,
  } = $props();

  let draft = $state('');
  let saving = $state(false);
  let errorMsg = $state(null);
  let savedFlash = $state(false);
  let lastProp = $state(undefined);

  // Re-sync draft when the parent's value prop changes (e.g. a refresh from
  // the server). We only overwrite when the parent value actually changes to
  // avoid stomping an in-progress edit.
  $effect(() => {
    if (value !== lastProp) {
      lastProp = value;
      draft = value ?? '';
    }
  });

  async function handleCommit() {
    if (disabled) return;
    if (saving) return;
    saving = true;
    errorMsg = null;
    const result = await commitDisplayName({
      current: value ?? '',
      next: draft,
      onCommit,
    });
    saving = false;
    if (result.status === 'error') {
      errorMsg = result.error;
      return;
    }
    draft = result.value;
    if (result.status === 'saved') {
      savedFlash = true;
      setTimeout(() => { savedFlash = false; }, 1200);
    }
  }

  function handleKeydown(ev) {
    if (ev.key !== 'Enter') return;
    ev.preventDefault();
    ev.target.blur();
  }
</script>

<div class="display-name-editor" data-testid="display-name-editor">
  <label>
    {label}:
    <input
      type="text"
      bind:value={draft}
      onblur={handleCommit}
      onkeydown={handleKeydown}
      placeholder={placeholder ?? (fallback ? `Defaults to ${fallback}` : 'Defaults to internal name')}
      {disabled}
      data-testid="display-name-input"
    />
  </label>
  {#if saving}
    <span class="status saving" data-testid="display-name-saving">Saving…</span>
  {:else if errorMsg}
    <span class="status error" data-testid="display-name-error">{errorMsg}</span>
  {:else if savedFlash}
    <span class="status saved" data-testid="display-name-saved">Saved</span>
  {/if}
</div>

<style>
  .display-name-editor {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }
  .display-name-editor label {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    flex: 1;
  }
  .display-name-editor input {
    flex: 1;
    min-width: 12ch;
  }
  .status {
    font-size: 0.85em;
  }
  .status.saving { color: #888; }
  .status.error { color: #c33; }
  .status.saved { color: #2a7; }
</style>
