<script>
  import { fetchDiscordChecks, failingChecks } from './lib/discordChecks.js';

  let failures = $state([]);
  let dismissed = $state(false);

  async function load() {
    try {
      const report = await fetchDiscordChecks();
      failures = failingChecks(report);
    } catch {
      // Network errors are silent — the banner is an advisory surface, not
      // a critical UI element. Falling back to an empty failures list
      // keeps the rest of the dashboard usable.
      failures = [];
    }
  }

  $effect(() => {
    load();
  });

  function dismiss() {
    dismissed = true;
  }

  function shouldRender() {
    return !dismissed && failures.length > 0;
  }
</script>

{#if shouldRender()}
  <aside class="discord-banner" role="alert" aria-live="polite">
    <div class="content">
      <strong>Discord configuration issues detected:</strong>
      <ul>
        {#each failures as failure}
          <li>
            <span class="check-name">{failure.name}</span>
            {#if failure.detail}
              <span class="check-detail">— {failure.detail}</span>
            {/if}
          </li>
        {/each}
      </ul>
    </div>
    <button class="dismiss" aria-label="Dismiss" onclick={dismiss}>×</button>
  </aside>
{/if}

<style>
  .discord-banner {
    display: flex;
    gap: 1rem;
    align-items: flex-start;
    padding: 0.75rem 1rem;
    margin: 0 0 1rem 0;
    background: #4a1722;
    border: 1px solid #e94560;
    border-radius: 6px;
    color: #ffe5ea;
    font-size: 0.9rem;
  }

  .content {
    flex: 1;
    min-width: 0;
  }

  .discord-banner ul {
    margin: 0.25rem 0 0 1.25rem;
    padding: 0;
  }

  .discord-banner li {
    margin: 0.125rem 0;
  }

  .check-name {
    font-weight: 600;
  }

  .check-detail {
    opacity: 0.85;
  }

  .dismiss {
    background: transparent;
    border: 1px solid transparent;
    color: #ffe5ea;
    font-size: 1.25rem;
    line-height: 1;
    cursor: pointer;
    padding: 0 0.25rem;
  }

  .dismiss:hover {
    border-color: #e94560;
    border-radius: 4px;
  }
</style>
