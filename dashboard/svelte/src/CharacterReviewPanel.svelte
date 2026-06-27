<script>
  import { reviewChanges, reviewFields } from './lib/characterReview.js';

  // id = player_characters id (approval id); characterId = characters id for
  // the full-sheet deep link.
  let { id, characterId } = $props();

  let detail = $state(null);
  let loading = $state(false);
  let error = $state(null);

  // The panel mounts only when the DM expands an entry, so this load is lazy.
  $effect(() => {
    load();
  });

  async function load() {
    loading = true;
    error = null;
    try {
      const res = await fetch(`/dashboard/api/approvals/${id}`, { credentials: 'same-origin' });
      if (!res.ok) throw new Error(`${res.status}`);
      detail = await res.json();
    } catch (e) {
      error = e.message;
      detail = null;
    } finally {
      loading = false;
    }
  }

  const fields = $derived(detail ? reviewFields(detail.review) : []);
  const changes = $derived(detail && detail.review_before ? reviewChanges(detail.review_before, detail.review) : []);
</script>

<div class="review-panel" data-testid="character-review-panel">
  {#if loading}
    <p class="placeholder">Loading review…</p>
  {:else if error}
    <p class="error">Failed to load character review.</p>
  {:else if detail}
    {#if detail.review_before}
      <section class="changes" data-testid="review-changes">
        <h4>Changes since last approval</h4>
        {#each changes as c}
          <div class="change-row" data-field={c.field}>
            <span class="change-label">{c.label}</span>
            {#if c.kind === 'list'}
              <span class="change-value">
                {#each c.added as a}<span class="added">+{a}</span>{/each}
                {#each c.removed as r}<span class="removed">−{r}</span>{/each}
              </span>
            {:else}
              <span class="change-value">
                <span class="before">{c.before ?? '∅'}</span>
                <span class="arrow">→</span>
                <span class="after">{c.after ?? '∅'}</span>
              </span>
            {/if}
          </div>
        {:else}
          <p class="placeholder">No reviewable changes.</p>
        {/each}
      </section>
    {/if}

    <section class="full-review">
      <h4>Character</h4>
      {#each fields as f}
        <div class="review-row" data-field={f.field}>
          <span class="review-label">{f.label}</span>
          <span class="review-value">{f.value}</span>
        </div>
      {/each}
    </section>

    <a class="full-sheet-link" href={`/portal/character/${characterId}`} target="_blank" rel="noopener">
      Open full sheet ↗
    </a>
  {/if}
</div>

<style>
  .review-panel {
    width: 100%;
    margin-top: 0.5rem;
    padding: 0.75rem;
    background: #0f1830;
    border: 1px solid #0f3460;
    border-radius: 4px;
    font-size: 0.85rem;
  }
  h4 {
    margin: 0 0 0.4rem;
    color: #e0e0e0;
    font-size: 0.85rem;
  }
  .changes {
    margin-bottom: 0.75rem;
    padding-bottom: 0.5rem;
    border-bottom: 1px solid #0f3460;
  }
  .change-row,
  .review-row {
    display: flex;
    gap: 0.5rem;
    padding: 0.15rem 0;
  }
  .change-label,
  .review-label {
    flex: 0 0 9rem;
    color: #a0aec0;
  }
  .change-value,
  .review-value {
    color: #e0e0e0;
    display: flex;
    flex-wrap: wrap;
    gap: 0.35rem;
  }
  .arrow {
    color: #a0aec0;
  }
  .before {
    color: #e94560;
  }
  .after {
    color: #5ad17a;
  }
  .added {
    color: #5ad17a;
  }
  .removed {
    color: #e94560;
  }
  .full-sheet-link {
    display: inline-block;
    margin-top: 0.6rem;
    color: #6aa6ff;
    text-decoration: none;
    font-size: 0.8rem;
  }
  .full-sheet-link:hover {
    text-decoration: underline;
  }
  .placeholder {
    color: #a0aec0;
    font-style: italic;
  }
  .error {
    color: #e94560;
  }
</style>
