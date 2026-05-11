<script>
  // med-37 / Phase 101: Character Overview Svelte UI. Minimal viable
  // implementation per the bundled task constraints — fetch the
  // /api/character-overview endpoint and render party cards plus the
  // shared-language rollup. No editing affordances; the DM dashboard
  // already exposes mutating endpoints via other panels.

  let { campaignId } = $props();

  let loading = $state(true);
  let error = $state(null);
  let characters = $state([]);
  let partyLanguages = $state([]);

  async function load() {
    if (!campaignId) {
      loading = false;
      return;
    }
    loading = true;
    error = null;
    try {
      const res = await fetch(
        `/api/character-overview?campaign_id=${encodeURIComponent(campaignId)}`,
        { credentials: 'same-origin' }
      );
      if (!res.ok) {
        throw new Error(`HTTP ${res.status}`);
      }
      const data = await res.json();
      characters = data.characters || [];
      partyLanguages = data.party_languages || [];
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  $effect(() => {
    if (campaignId) {
      load();
    }
  });

  function classSummary(rawClasses) {
    if (!rawClasses) return '';
    try {
      const parsed = typeof rawClasses === 'string' ? JSON.parse(rawClasses) : rawClasses;
      if (!Array.isArray(parsed)) return '';
      return parsed.map((c) => `${c.class} ${c.level}`).join(' / ');
    } catch (_) {
      return '';
    }
  }
</script>

<div class="character-overview">
  <h2>Party Overview</h2>

  {#if loading}
    <p>Loading party...</p>
  {:else if error}
    <p class="error">Failed to load: {error}</p>
  {:else if characters.length === 0}
    <p>No approved characters in this campaign yet.</p>
  {:else}
    <div class="grid">
      {#each characters as c (c.character_id)}
        <div class="card">
          <h3>{c.name}</h3>
          <p class="meta">{c.race} — {classSummary(c.classes) || `Level ${c.level}`}</p>
          <ul>
            <li>HP: {c.hp_current} / {c.hp_max}</li>
            <li>AC: {c.ac}</li>
            <li>Speed: {c.speed_ft} ft</li>
          </ul>
          {#if c.languages && c.languages.length > 0}
            <p class="langs">Languages: {c.languages.join(', ')}</p>
          {/if}
          {#if c.ddb_url}
            <a href={c.ddb_url} target="_blank" rel="noopener">D&amp;D Beyond sheet</a>
          {/if}
        </div>
      {/each}
    </div>

    {#if partyLanguages.length > 0}
      <h3>Shared Languages</h3>
      <ul class="langs-list">
        {#each partyLanguages as row}
          <li><strong>{row.language}:</strong> {row.characters.join(', ')}</li>
        {/each}
      </ul>
    {/if}
  {/if}
</div>

<style>
  .character-overview {
    max-width: 1000px;
  }
  h2, h3 {
    color: #e94560;
  }
  .grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
    gap: 1rem;
    margin-bottom: 1.5rem;
  }
  .card {
    background: #16213e;
    border: 1px solid #0f3460;
    border-radius: 6px;
    padding: 0.75rem 1rem;
  }
  .card h3 {
    margin: 0 0 0.25rem 0;
    color: #e0e0e0;
  }
  .meta {
    color: #b0b0c0;
    margin: 0 0 0.5rem 0;
    font-style: italic;
  }
  ul {
    list-style: none;
    padding: 0;
    margin: 0 0 0.5rem 0;
  }
  ul li {
    padding: 0.15rem 0;
  }
  .langs {
    margin: 0.25rem 0;
    color: #b0b0c0;
    font-size: 0.9rem;
  }
  .langs-list li {
    padding: 0.25rem 0;
  }
  .error {
    color: #ff6b6b;
  }
  a {
    color: #e94560;
  }
</style>
