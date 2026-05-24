<script>
  import SpellPicker from './SpellPicker.svelte';
  import { getPreparation, savePreparation } from './lib/api.js';

  // Standalone spell-preparation page. Reuses the same SpellPicker as the
  // character builder so there is one spell-selection UI to maintain. The cap,
  // always-prepared list, and castable slot levels are computed server-side and
  // delivered by GET /portal/api/characters/{id}/preparation.
  let { characterId = '' } = $props();

  let loading = $state(true);
  let error = $state('');
  let info = $state(null);
  let selected = $state([]); // player's chosen non-always spell ids
  let saving = $state(false);
  let saved = $state(false);
  let saveError = $state('');

  $effect(() => {
    load();
  });

  async function load() {
    loading = true;
    error = '';
    try {
      info = await getPreparation(characterId);
      const always = new Set(info.always_prepared || []);
      selected = (info.current_prepared || []).filter((id) => !always.has(id));
    } catch (e) {
      error = e.message || 'Failed to load spell preparation.';
    } finally {
      loading = false;
    }
  }

  async function save() {
    saving = true;
    saved = false;
    saveError = '';
    try {
      const res = await savePreparation(characterId, selected);
      info = { ...info, max_prepared: res.max_prepared, current_prepared: selected };
      saved = true;
    } catch (e) {
      saveError = e.message || 'Failed to save spell preparation.';
    } finally {
      saving = false;
    }
  }

  // Clear the saved flag as soon as the player edits the selection again.
  let lastSelectedLen = -1;
  $effect(() => {
    if (selected.length !== lastSelectedLen) {
      lastSelectedLen = selected.length;
      saved = false;
    }
  });
</script>

<div class="prep">
  {#if loading}
    <p class="prep-status">Loading spell preparation…</p>
  {:else if error}
    <p class="prep-error">{error}</p>
  {:else if info}
    <header class="prep-head">
      <h2>{info.character_name} — Prepare Spells</h2>
      <p class="prep-sub">
        {info.class}{#if info.subclass} · {info.subclass}{/if}
      </p>
    </header>

    <SpellPicker
      spells={info.spells}
      bind:selected
      max={info.max_prepared}
      selectableLevels={info.available_slot_levels}
      alwaysPrepared={info.always_prepared}
    />

    <div class="prep-actions">
      <button class="prep-save" onclick={save} disabled={saving}>
        {saving ? 'Saving…' : 'Save Preparation'}
      </button>
      {#if saved}<span class="prep-ok">Saved.</span>{/if}
      {#if saveError}<span class="prep-error">{saveError}</span>{/if}
    </div>
  {/if}
</div>

<style>
  .prep { max-width: 900px; margin: 0 auto; padding: 1rem; color: #eee; }
  .prep-head { margin-bottom: 1rem; }
  .prep-head h2 { margin: 0 0 0.2rem; }
  .prep-sub { margin: 0; color: #aaa; text-transform: capitalize; }
  .prep-status { color: #aaa; }
  .prep-actions { display: flex; align-items: center; gap: 0.8rem; margin-top: 1rem; position: sticky; bottom: 0; padding: 0.6rem 0; background: #1a1a2e; }
  .prep-save { padding: 0.55rem 1.2rem; background: #e94560; color: #fff; border: none; border-radius: 6px; font-weight: 600; cursor: pointer; }
  .prep-save:disabled { opacity: 0.6; cursor: default; }
  .prep-ok { color: #5cb85c; font-weight: 600; }
  .prep-error { color: #e94560; font-weight: 600; }
</style>
