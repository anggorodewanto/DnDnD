<script>
  // Exploration dashboard panel. The DM picks a map from the campaign
  // roster, names the encounter, and starts exploration. Once an
  // encounter is live the panel exposes the transition-to-combat handoff
  // with optional per-PC position overrides (one `<character_id>=<coord>`
  // per line).
  import {
    fetchExplorationData,
    startExploration,
    transitionToCombat,
    parseOverridesText,
  } from './lib/exploration.js';

  let { campaignId = '' } = $props();

  let maps = $state([]);
  let loading = $state(true);
  let loadError = $state(null);

  let selectedMapId = $state('');
  let encounterName = $state('');
  let starting = $state(false);
  let startError = $state(null);

  // currentEncounter tracks the most recently started exploration encounter
  // so the DM has the encounter id handy for the transition-to-combat step.
  let currentEncounter = $state(null);

  let transitionEncounterId = $state('');
  let overridesText = $state('');
  let transitioning = $state(false);
  let transitionError = $state(null);
  let transitionResult = $state(null);

  async function loadMaps() {
    if (!campaignId) {
      loading = false;
      return;
    }
    loading = true;
    loadError = null;
    try {
      const data = await fetchExplorationData(campaignId);
      maps = Array.isArray(data?.maps) ? data.maps : [];
    } catch (e) {
      loadError = e.message;
    } finally {
      loading = false;
    }
  }

  $effect(() => {
    loadMaps();
  });

  async function handleStart() {
    if (!selectedMapId) {
      startError = 'select a map first';
      return;
    }
    starting = true;
    startError = null;
    try {
      const name = encounterName.trim();
      const out = await startExploration({
        campaignId,
        mapId: selectedMapId,
        name: name === '' ? undefined : name,
      });
      currentEncounter = out;
      if (out?.encounter_id) {
        transitionEncounterId = out.encounter_id;
      }
    } catch (e) {
      startError = e.message;
    } finally {
      starting = false;
    }
  }

  async function handleTransition() {
    if (!transitionEncounterId) {
      transitionError = 'encounter id is required';
      return;
    }
    transitioning = true;
    transitionError = null;
    transitionResult = null;
    let overrides;
    try {
      overrides = parseOverridesText(overridesText);
    } catch (e) {
      transitionError = e.message;
      transitioning = false;
      return;
    }
    try {
      const out = await transitionToCombat({
        encounterId: transitionEncounterId,
        overrides,
      });
      transitionResult = out;
    } catch (e) {
      transitionError = e.message;
    } finally {
      transitioning = false;
    }
  }

  function mapLabel(m) {
    if (m.width_squares && m.height_squares) {
      return `${m.name} (${m.width_squares}x${m.height_squares})`;
    }
    return m.name || m.id;
  }
</script>

<div class="exploration-panel">
  <header class="panel-header">
    <h2>Exploration Mode</h2>
    <p class="meta">Campaign: <code>{campaignId || '(none)'}</code></p>
  </header>

  {#if !campaignId}
    <p class="empty">Select a campaign to view exploration options.</p>
  {:else if loading}
    <p>Loading maps...</p>
  {:else if loadError}
    <p class="error">{loadError}</p>
  {:else}
    <section class="start-section">
      <h3>Start Exploration</h3>
      {#if maps.length === 0}
        <p class="empty">No maps found for this campaign. Create one first.</p>
      {:else}
        <label>
          Map:
          <select bind:value={selectedMapId}>
            <option value="">-- pick a map --</option>
            {#each maps as m}
              <option value={m.id}>{mapLabel(m)}</option>
            {/each}
          </select>
        </label>
        <label>
          Encounter name:
          <input
            type="text"
            bind:value={encounterName}
            placeholder="Exploration"
          />
        </label>
        <button onclick={handleStart} disabled={starting || !selectedMapId}>
          {starting ? 'Starting...' : 'Start Exploration'}
        </button>
        {#if startError}
          <p class="error">{startError}</p>
        {/if}
      {/if}
    </section>

    {#if currentEncounter}
      <section class="current-section">
        <h3>Current Exploration</h3>
        <p>
          Encounter <code>{currentEncounter.encounter_id}</code> is now in
          <strong>{currentEncounter.mode}</strong> mode.
        </p>
        {#if currentEncounter.pcs && Object.keys(currentEncounter.pcs).length > 0}
          <table class="pc-table">
            <thead><tr><th>Character</th><th>Position</th></tr></thead>
            <tbody>
              {#each Object.entries(currentEncounter.pcs) as [charId, pos]}
                <tr><td><code>{charId}</code></td><td>{pos.Col}{pos.Row}</td></tr>
              {/each}
            </tbody>
          </table>
        {/if}
      </section>
    {/if}

    <section class="transition-section">
      <h3>Transition to Combat</h3>
      <p class="hint">
        PC positions carry over from exploration. Optionally override any
        per-PC position with one <code>&lt;character_id&gt;=&lt;coord&gt;</code>
        line (e.g. <code>D5</code>).
      </p>
      <label>
        Encounter ID:
        <input type="text" bind:value={transitionEncounterId} />
      </label>
      <label>
        Overrides:
        <textarea
          rows="3"
          bind:value={overridesText}
          placeholder="character-uuid=D5"
        ></textarea>
      </label>
      <button onclick={handleTransition} disabled={transitioning || !transitionEncounterId}>
        {transitioning ? 'Transitioning...' : 'Transition to Combat'}
      </button>
      {#if transitionError}
        <p class="error">{transitionError}</p>
      {/if}
      {#if transitionResult}
        <div class="result">
          <p>Encounter mode is now <strong>{transitionResult.mode}</strong>.</p>
          {#if transitionResult.positions && Object.keys(transitionResult.positions).length > 0}
            <table class="pc-table">
              <thead><tr><th>Character</th><th>Position</th></tr></thead>
              <tbody>
                {#each Object.entries(transitionResult.positions) as [charId, pos]}
                  <tr><td><code>{charId}</code></td><td>{pos.col}{pos.row}</td></tr>
                {/each}
              </tbody>
            </table>
          {/if}
        </div>
      {/if}
    </section>
  {/if}
</div>

<style>
  .exploration-panel {
    max-width: 900px;
  }
  .panel-header h2 {
    color: #e94560;
    margin: 0 0 0.25rem 0;
  }
  .meta {
    color: #a0a0c0;
    font-size: 0.85rem;
    margin: 0 0 1rem 0;
  }
  section {
    background: #16213e;
    border: 1px solid #0f3460;
    border-radius: 6px;
    padding: 1rem;
    margin-bottom: 1rem;
  }
  section h3 {
    color: #e94560;
    margin: 0 0 0.75rem 0;
  }
  label {
    display: block;
    margin-bottom: 0.75rem;
    color: #e0e0e0;
  }
  input[type='text'],
  select,
  textarea {
    display: block;
    width: 100%;
    box-sizing: border-box;
    padding: 0.4rem 0.6rem;
    background: #0f1a2e;
    color: #e0e0e0;
    border: 1px solid #0f3460;
    border-radius: 4px;
    margin-top: 0.25rem;
  }
  button {
    padding: 0.4rem 0.8rem;
    background: #0f3460;
    color: #e0e0e0;
    border: 1px solid #0f3460;
    border-radius: 4px;
    cursor: pointer;
  }
  button:disabled {
    opacity: 0.6;
    cursor: not-allowed;
  }
  .pc-table {
    width: 100%;
    border-collapse: collapse;
    margin-top: 0.5rem;
  }
  .pc-table th,
  .pc-table td {
    text-align: left;
    padding: 0.4rem 0.6rem;
    border-bottom: 1px solid #0f3460;
  }
  .pc-table th {
    background: #0f3460;
    color: #e94560;
  }
  .hint {
    color: #a0a0c0;
    font-size: 0.85rem;
  }
  .empty {
    color: #888;
    font-style: italic;
  }
  .error {
    color: #ff4444;
  }
  .result {
    margin-top: 0.5rem;
  }
</style>
