<script module>
  /**
   * FeatureUsesEditor — reusable DM control for editing a character's
   * limited-use feature resources (e.g. Barbarian rage uses). Mounted in the
   * CombatManager Manual Override panel and saved one feature at a time via
   * POST /api/combat/{enc}/override/character/{id}/feature-uses.
   *
   * The dashboard's vitest harness runs under the `node` environment with no
   * DOM, so the component can't be mounted in unit tests. Mirroring SlotEditor,
   * the pure value-shaping logic (row derivation, clamping, change building,
   * read-only summaries) lives in this module script so it can be imported and
   * exercised directly with no DOM.
   */

  function toInt(v) {
    const n = parseInt(v, 10);
    return Number.isFinite(n) ? n : 0;
  }

  /**
   * Clamp a `current` value into the inclusive 0..max range. A negative `max`
   * marks an unlimited pool, so only the lower bound (0) is enforced.
   * @param {any} current
   * @param {any} max
   * @returns {number}
   */
  export function clampCurrent(current, max) {
    const c = toInt(current);
    const m = toInt(max);
    if (c < 0) return 0;
    if (m >= 0 && c > m) return m;
    return c;
  }

  /**
   * Turn the backend's name-keyed feature-uses map into a stably-ordered
   * (alphabetical by feature name) array of editable rows. Tolerates null / {}
   * (returns []).
   * @param {Record<string,{current:any,max:any,recharge?:string}>|null|undefined} featureUses
   * @returns {{name:string,current:number,max:number,recharge:string}[]}
   */
  export function featureRowsFromProps(featureUses) {
    if (!featureUses || typeof featureUses !== 'object') return [];
    return Object.keys(featureUses)
      .sort((a, b) => a.localeCompare(b))
      .map((name) => ({
        name,
        current: toInt(featureUses[name]?.current),
        max: toInt(featureUses[name]?.max),
        recharge: featureUses[name]?.recharge || '',
      }));
  }

  /**
   * Build the save payload from the editor rows. Returns
   * `{changes:[{feature,current}], reason?}` containing ONLY the rows whose
   * `current` differs from the matching seed row. Each emitted `current` is
   * clamped into 0..max so the body is always valid. `reason` is included only
   * when non-empty after trimming. The parent saves each change separately
   * (the endpoint takes one feature per request).
   * @param {{name:string,current:any,max:any}[]} rows
   * @param {{name:string,current:any}[]} seedRows
   * @param {string} reason
   * @returns {{changes:{feature:string,current:number}[], reason?:string}}
   */
  export function buildFeatureUsesChanges(rows, seedRows, reason) {
    const seedByName = {};
    if (Array.isArray(seedRows)) {
      for (const r of seedRows) seedByName[r.name] = toInt(r.current);
    }
    const changes = [];
    if (Array.isArray(rows)) {
      for (const row of rows) {
        const current = clampCurrent(row.current, row.max);
        const seeded = row.name in seedByName ? seedByName[row.name] : null;
        if (seeded !== current) changes.push({ feature: row.name, current });
      }
    }
    const payload = { changes };
    const trimmed = (reason || '').trim();
    if (trimmed) payload.reason = trimmed;
    return payload;
  }

  /**
   * Compact read-only summary of feature uses, e.g.
   * "Features: bardic 2/3 · rage 1/∞". Returns '' when there are none.
   * @param {Record<string,{current:any,max:any}>|null|undefined} featureUses
   * @returns {string}
   */
  export function formatFeatureUsesSummary(featureUses) {
    const rows = featureRowsFromProps(featureUses);
    if (rows.length === 0) return '';
    return (
      'Features: ' +
      rows.map((r) => `${r.name} ${r.current}/${r.max < 0 ? '∞' : r.max}`).join(' · ')
    );
  }
</script>

<script>
  let {
    featureUses = null,
    busy = false,
    errorMessage = '',
    onSave,
    onCancel,
  } = $props();

  // Editable copies plus an immutable seed snapshot, re-seeded whenever the
  // source prop changes identity (e.g. the parent reopens the editor for a
  // different character). `lastSeed` is a plain (non-reactive) variable so
  // writing it never re-triggers the effect.
  let rows = $state([]);
  let seedRows = $state([]);
  let reason = $state('');
  let lastSeed;

  $effect(() => {
    if (featureUses === lastSeed) return;
    lastSeed = featureUses;
    rows = featureRowsFromProps(featureUses);
    seedRows = featureRowsFromProps(featureUses);
    reason = '';
  });

  let hasRows = $derived(rows.length > 0);

  function clampRow(row) {
    row.current = clampCurrent(row.current, row.max);
  }

  function handleSave() {
    onSave?.(buildFeatureUsesChanges(rows, seedRows, reason));
  }
</script>

<div class="feature-uses-editor" data-testid="feature-uses-editor">
  {#if errorMessage}
    <div class="feature-uses-error" data-testid="feature-uses-error">{errorMessage}</div>
  {/if}

  {#if !hasRows}
    <p class="feature-uses-empty" data-testid="feature-uses-editor-empty">
      No feature uses to edit.
    </p>
  {:else}
    {#each rows as row (row.name)}
      <div class="feature-uses-row" data-testid="feature-uses-row-{row.name}">
        <span class="feature-uses-label"
          >{row.name} {row.current}/{row.max < 0 ? '∞' : row.max}</span
        >
        <label>
          current
          <input
            type="number"
            min="0"
            max={row.max >= 0 ? row.max : undefined}
            data-testid="feature-uses-current-{row.name}"
            bind:value={row.current}
            onchange={() => clampRow(row)}
          />
        </label>
      </div>
    {/each}

    <label class="feature-uses-reason">
      Reason (optional)
      <input type="text" data-testid="feature-uses-reason" bind:value={reason} />
    </label>
  {/if}

  <div class="feature-uses-actions">
    <button
      class="feature-uses-save"
      data-testid="feature-uses-save"
      onclick={handleSave}
      disabled={busy || !hasRows}
    >
      {busy ? 'Saving…' : 'Save feature uses'}
    </button>
    <button
      class="feature-uses-cancel"
      data-testid="feature-uses-cancel"
      onclick={() => onCancel?.()}
      disabled={busy}
    >Cancel</button>
  </div>
</div>

<style>
  .feature-uses-editor {
    margin-top: 0.5rem;
    padding: 0.6rem;
    background: #14203a;
    border: 1px solid #0f3460;
    border-radius: 6px;
  }
  .feature-uses-error {
    background: #3e1a1a;
    color: #ff6b6b;
    padding: 0.5rem;
    border-radius: 4px;
    margin-bottom: 0.5rem;
    font-size: 0.85rem;
  }
  .feature-uses-empty {
    color: #a0a0c0;
    font-size: 0.85rem;
    margin: 0 0 0.5rem 0;
  }
  .feature-uses-row {
    display: flex;
    align-items: flex-end;
    gap: 0.5rem;
    margin-bottom: 0.5rem;
  }
  .feature-uses-label {
    color: #e0e0e0;
    font-size: 0.85rem;
    min-width: 8rem;
  }
  .feature-uses-row label,
  .feature-uses-reason {
    display: flex;
    flex-direction: column;
    gap: 0.2rem;
    color: #a0a0c0;
    font-size: 0.8rem;
  }
  .feature-uses-reason {
    margin-bottom: 0.5rem;
  }
  .feature-uses-editor input[type='number'] {
    width: 4.5rem;
  }
  .feature-uses-editor input[type='number'],
  .feature-uses-reason input[type='text'] {
    padding: 0.35rem;
    background: #1a1a2e;
    color: #e0e0e0;
    border: 1px solid #0f3460;
    border-radius: 4px;
    box-sizing: border-box;
  }
  .feature-uses-reason input[type='text'] {
    width: 100%;
  }
  .feature-uses-actions {
    display: flex;
    gap: 0.5rem;
  }
  .feature-uses-save {
    padding: 0.4rem 0.8rem;
    background: #e94560;
    color: white;
    border: none;
    border-radius: 4px;
    cursor: pointer;
    font-size: 0.85rem;
  }
  .feature-uses-save:hover { background: #c73852; }
  .feature-uses-save:disabled { opacity: 0.5; cursor: not-allowed; }
  .feature-uses-cancel {
    padding: 0.4rem 0.8rem;
    background: transparent;
    color: #a0a0c0;
    border: 1px solid #0f3460;
    border-radius: 4px;
    cursor: pointer;
    font-size: 0.85rem;
  }
  .feature-uses-cancel:hover { background: #0f3460; color: #e0e0e0; }
  .feature-uses-cancel:disabled { opacity: 0.4; cursor: not-allowed; }
</style>
