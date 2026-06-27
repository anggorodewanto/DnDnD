<script module>
  /**
   * SlotEditor — reusable DM control for editing a character's spell slots and
   * pact-magic slots. Used both out of combat (CharacterOverview, saving via
   * POST /api/character-overview/{id}/slots) and in combat (CombatManager,
   * saving via POST /api/combat/{enc}/override/character/{id}/slots).
   *
   * The dashboard's vitest harness runs under the `node` environment with no
   * DOM, so the component can't be mounted in unit tests. Following the
   * characterStatus.js convention, the pure value-shaping logic (row
   * derivation, clamping, payload building, read-only summaries) lives in this
   * module script so it can be imported and exercised directly with no DOM.
   */

  function toInt(v) {
    const n = parseInt(v, 10);
    return Number.isFinite(n) ? n : 0;
  }

  /**
   * Clamp a `current` value into the inclusive 0..max range.
   * @param {any} current
   * @param {any} max
   * @returns {number}
   */
  export function clampCurrent(current, max) {
    const c = toInt(current);
    const m = toInt(max);
    if (c < 0) return 0;
    if (c > m) return m;
    return c;
  }

  /**
   * Turn the backend's string-keyed spell-slot map into a numerically-sorted
   * array of editable rows. Tolerates null / {} (returns []).
   * @param {Record<string,{current:any,max:any}>|null|undefined} spellSlots
   * @returns {{level:number,current:number,max:number}[]}
   */
  export function spellRowsFromProps(spellSlots) {
    if (!spellSlots || typeof spellSlots !== 'object') return [];
    return Object.keys(spellSlots)
      .map((k) => ({ level: parseInt(k, 10), data: spellSlots[k] }))
      .filter((e) => Number.isFinite(e.level))
      .sort((a, b) => a.level - b.level)
      .map((e) => ({
        level: e.level,
        current: toInt(e.data?.current),
        max: toInt(e.data?.max),
      }));
  }

  /**
   * Build the save payload from the editor rows. `spell_slots` is included only
   * when there were spell-slot rows, `pact_magic_slots` only when a pact row was
   * present, and `reason` only when non-empty. Every `current` is clamped into
   * 0..max so the body is always valid regardless of UI state.
   * @param {{level:number,current:any,max:any}[]} spellRows
   * @param {{slot_level:any,current:any,max:any}|null} pactRow
   * @param {string} reason
   * @returns {{spell_slots?:object, pact_magic_slots?:object, reason?:string}}
   */
  export function buildSlotPayload(spellRows, pactRow, reason) {
    const payload = {};
    if (Array.isArray(spellRows) && spellRows.length > 0) {
      const spell_slots = {};
      for (const row of spellRows) {
        const max = toInt(row.max);
        spell_slots[String(row.level)] = { current: clampCurrent(row.current, max), max };
      }
      payload.spell_slots = spell_slots;
    }
    if (pactRow) {
      const max = toInt(pactRow.max);
      payload.pact_magic_slots = {
        slot_level: toInt(pactRow.slot_level),
        current: clampCurrent(pactRow.current, max),
        max,
      };
    }
    const trimmed = (reason || '').trim();
    if (trimmed) payload.reason = trimmed;
    return payload;
  }

  /**
   * Compact read-only summary of spell slots, e.g. "Slots: L1 2/4 · L2 0/2".
   * Returns '' when there are no spell slots.
   * @param {Record<string,{current:any,max:any}>|null|undefined} spellSlots
   * @returns {string}
   */
  export function formatSpellSummary(spellSlots) {
    const rows = spellRowsFromProps(spellSlots);
    if (rows.length === 0) return '';
    return 'Slots: ' + rows.map((r) => `L${r.level} ${r.current}/${r.max}`).join(' · ');
  }

  /**
   * Compact read-only summary of pact magic, e.g. "Pact: L2 0/2". Returns ''
   * when there is no pact magic.
   * @param {{slot_level:any,current:any,max:any}|null|undefined} pactSlots
   * @returns {string}
   */
  export function formatPactSummary(pactSlots) {
    if (!pactSlots) return '';
    return `Pact: L${toInt(pactSlots.slot_level)} ${toInt(pactSlots.current)}/${toInt(pactSlots.max)}`;
  }
</script>

<script>
  let {
    spellSlots = null,
    pactSlots = null,
    busy = false,
    errorMessage = '',
    onSave,
    onCancel,
  } = $props();

  // Editable copies, re-seeded whenever the source props change identity (e.g.
  // the parent reopens the editor for a different character). `lastSeed` is a
  // plain (non-reactive) variable so writing it never re-triggers the effect.
  let spellRows = $state([]);
  let pactRow = $state(null);
  let reason = $state('');
  let lastSeed;

  $effect(() => {
    if (spellSlots === lastSeed?.spell && pactSlots === lastSeed?.pact) return;
    lastSeed = { spell: spellSlots, pact: pactSlots };
    spellRows = spellRowsFromProps(spellSlots);
    pactRow = pactSlots
      ? {
          slot_level: pactSlots.slot_level,
          current: pactSlots.current,
          max: pactSlots.max,
        }
      : null;
    reason = '';
  });

  let hasRows = $derived(spellRows.length > 0 || pactRow !== null);

  function clampSpellRow(row) {
    row.current = clampCurrent(row.current, row.max);
  }

  function clampPactRow() {
    if (pactRow) pactRow.current = clampCurrent(pactRow.current, pactRow.max);
  }

  function handleSave() {
    onSave?.(buildSlotPayload(spellRows, pactRow, reason));
  }
</script>

<div class="slot-editor" data-testid="slot-editor">
  {#if errorMessage}
    <div class="slot-error" data-testid="slot-error">{errorMessage}</div>
  {/if}

  {#if !hasRows}
    <p class="slot-empty" data-testid="slot-editor-empty">No spell slots to edit.</p>
  {:else}
    {#each spellRows as row (row.level)}
      <div class="slot-row" data-testid="slot-row-{row.level}">
        <span class="slot-label">Level {row.level}</span>
        <label>
          current
          <input
            type="number"
            min="0"
            max={row.max}
            data-testid="slot-current-{row.level}"
            bind:value={row.current}
            onchange={() => clampSpellRow(row)}
          />
        </label>
        <label>
          max
          <input
            type="number"
            min="0"
            data-testid="slot-max-{row.level}"
            bind:value={row.max}
            onchange={() => clampSpellRow(row)}
          />
        </label>
      </div>
    {/each}

    {#if pactRow}
      <div class="slot-row" data-testid="pact-row">
        <span class="slot-label">Pact (Level {pactRow.slot_level})</span>
        <label>
          current
          <input
            type="number"
            min="0"
            max={pactRow.max}
            data-testid="pact-current"
            bind:value={pactRow.current}
            onchange={clampPactRow}
          />
        </label>
        <label>
          max
          <input
            type="number"
            min="0"
            data-testid="pact-max"
            bind:value={pactRow.max}
            onchange={clampPactRow}
          />
        </label>
      </div>
    {/if}

    <label class="slot-reason">
      Reason (optional)
      <input type="text" data-testid="slot-reason" bind:value={reason} />
    </label>
  {/if}

  <div class="slot-actions">
    <button
      class="slot-save"
      data-testid="slot-save"
      onclick={handleSave}
      disabled={busy || !hasRows}
    >
      {busy ? 'Saving…' : 'Save slots'}
    </button>
    <button
      class="slot-cancel"
      data-testid="slot-cancel"
      onclick={() => onCancel?.()}
      disabled={busy}
    >Cancel</button>
  </div>
</div>

<style>
  .slot-editor {
    margin-top: 0.5rem;
    padding: 0.6rem;
    background: #14203a;
    border: 1px solid #0f3460;
    border-radius: 6px;
  }
  .slot-error {
    background: #3e1a1a;
    color: #ff6b6b;
    padding: 0.5rem;
    border-radius: 4px;
    margin-bottom: 0.5rem;
    font-size: 0.85rem;
  }
  .slot-empty {
    color: #a0a0c0;
    font-size: 0.85rem;
    margin: 0 0 0.5rem 0;
  }
  .slot-row {
    display: flex;
    align-items: flex-end;
    gap: 0.5rem;
    margin-bottom: 0.5rem;
  }
  .slot-label {
    color: #e0e0e0;
    font-size: 0.85rem;
    min-width: 6rem;
  }
  .slot-row label,
  .slot-reason {
    display: flex;
    flex-direction: column;
    gap: 0.2rem;
    color: #a0a0c0;
    font-size: 0.8rem;
  }
  .slot-reason {
    margin-bottom: 0.5rem;
  }
  .slot-editor input[type='number'] {
    width: 4.5rem;
  }
  .slot-editor input[type='number'],
  .slot-reason input[type='text'] {
    padding: 0.35rem;
    background: #1a1a2e;
    color: #e0e0e0;
    border: 1px solid #0f3460;
    border-radius: 4px;
    box-sizing: border-box;
  }
  .slot-reason input[type='text'] {
    width: 100%;
  }
  .slot-actions {
    display: flex;
    gap: 0.5rem;
  }
  .slot-save {
    padding: 0.4rem 0.8rem;
    background: #e94560;
    color: white;
    border: none;
    border-radius: 4px;
    cursor: pointer;
    font-size: 0.85rem;
  }
  .slot-save:hover { background: #c73852; }
  .slot-save:disabled { opacity: 0.5; cursor: not-allowed; }
  .slot-cancel {
    padding: 0.4rem 0.8rem;
    background: transparent;
    color: #a0a0c0;
    border: 1px solid #0f3460;
    border-radius: 4px;
    cursor: pointer;
    font-size: 0.85rem;
  }
  .slot-cancel:hover { background: #0f3460; color: #e0e0e0; }
  .slot-cancel:disabled { opacity: 0.4; cursor: not-allowed; }
</style>
