<script>
  import { mobileTabs } from './lib/layout.js';
  import ActionResolver from './ActionResolver.svelte';
  import TurnQueue from './TurnQueue.svelte';
  import NarratePanel from './NarratePanel.svelte';
  import MessagePlayerPanel from './MessagePlayerPanel.svelte';
  import QuickActionsPanel from './QuickActionsPanel.svelte';
  import { getCombatWorkspace } from './lib/api.js';

  let { campaignId } = $props();

  let activeTab = $state('dm-queue');
  let activeEncounterId = $state(null);
  let combatants = $state([]);

  $effect(() => {
    if (campaignId) loadWorkspace();
  });

  async function loadWorkspace() {
    try {
      const data = await getCombatWorkspace(campaignId);
      const list = data.encounters || [];
      if (list.length === 0) return;
      activeEncounterId = list[0].id;
      combatants = list[0].combatants || [];
    } catch (e) {
      // Tabs without an active encounter show a placeholder.
      activeEncounterId = null;
    }
  }
</script>

<div class="mobile-shell" data-testid="mobile-shell">
  <main class="mobile-main">
    {#if activeTab === 'dm-queue'}
      {#if activeEncounterId}
        <ActionResolver encounterId={activeEncounterId} {combatants} />
      {:else}
        <p class="placeholder">No active encounter with pending items.</p>
      {/if}
    {:else if activeTab === 'turn-queue'}
      {#if activeEncounterId}
        <TurnQueue encounterId={activeEncounterId} readOnly={true} />
      {:else}
        <p class="placeholder">No active encounter.</p>
      {/if}
    {:else if activeTab === 'narrate'}
      <NarratePanel {campaignId} />
    {:else if activeTab === 'approvals'}
      <div class="approvals-link">
        <p>Character Approval Queue</p>
        <a href="/dashboard/approval" target="_self">Open approvals page</a>
      </div>
    {:else if activeTab === 'message-player'}
      <MessagePlayerPanel {campaignId} />
    {:else if activeTab === 'quick-actions'}
      <QuickActionsPanel {campaignId} />
    {/if}
  </main>

  <nav class="bottom-tabs" data-testid="mobile-bottom-tabs">
    {#each mobileTabs as tab}
      <button
        class:active={activeTab === tab.id}
        onclick={() => (activeTab = tab.id)}
        data-testid="mobile-tab-{tab.id}"
      >
        {tab.label}
      </button>
    {/each}
  </nav>
</div>

<style>
  .mobile-shell {
    display: flex;
    flex-direction: column;
    min-height: 100vh;
    padding-bottom: 4rem;
  }
  .mobile-main {
    padding: 0.75rem;
    flex: 1;
  }
  .placeholder {
    color: #a0aec0;
    font-style: italic;
  }
  .approvals-link {
    padding: 1rem;
    background: #16213e;
    border-radius: 4px;
    border: 1px solid #0f3460;
  }
  .approvals-link a {
    display: inline-block;
    margin-top: 0.5rem;
    padding: 0.5rem 1rem;
    background: #e94560;
    color: white;
    text-decoration: none;
    border-radius: 4px;
  }
  .bottom-tabs {
    position: fixed;
    left: 0;
    right: 0;
    bottom: 0;
    display: flex;
    background: #0f1a2e;
    border-top: 1px solid #0f3460;
  }
  .bottom-tabs button {
    flex: 1;
    padding: 0.75rem 0.25rem;
    background: transparent;
    color: #a0aec0;
    border: none;
    border-right: 1px solid #0f3460;
    cursor: pointer;
    font-size: 0.75rem;
  }
  .bottom-tabs button:last-child { border-right: none; }
  .bottom-tabs button.active {
    color: #e94560;
    background: #16213e;
    font-weight: bold;
  }
</style>
