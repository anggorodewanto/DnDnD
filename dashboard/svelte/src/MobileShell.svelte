<script>
  import { mobileTabs } from './lib/layout.js';
  import CharacterApprovalQueue from './CharacterApprovalQueue.svelte';
  import TurnQueue from './TurnQueue.svelte';
  import NarratePanel from './NarratePanel.svelte';
  import MessagePlayerPanel from './MessagePlayerPanel.svelte';
  import QuickActionsPanel from './QuickActionsPanel.svelte';
  import DMQueuePanel from './DMQueuePanel.svelte';
  import { getCombatWorkspace } from './lib/api.js';

  let { campaignId } = $props();

  let activeTab = $state('dm-queue');
  let encounters = $state([]);
  let selectedEncounterIdx = $state(0);
  let activeEncounterId = $derived(encounters[selectedEncounterIdx]?.id || null);
  let combatants = $derived(encounters[selectedEncounterIdx]?.combatants || []);

  $effect(() => {
    if (campaignId) loadWorkspace();
  });

  async function loadWorkspace() {
    try {
      const data = await getCombatWorkspace(campaignId);
      encounters = data.encounters || [];
    } catch (e) {
      encounters = [];
    }
  }
</script>

<div class="mobile-shell" data-testid="mobile-shell">
  <main class="mobile-main">
    {#if activeTab === 'dm-queue'}
      <DMQueuePanel />
    {:else if activeTab === 'turn-queue'}
      {#if encounters.length > 1}
        <div class="encounter-select">
          <select bind:value={selectedEncounterIdx}>
            {#each encounters as enc, i}
              <option value={i}>{enc.display_name || enc.name}</option>
            {/each}
          </select>
        </div>
      {/if}
      {#if activeEncounterId}
        <TurnQueue encounterId={activeEncounterId} readOnly={true} />
      {:else}
        <p class="placeholder">No active encounter.</p>
      {/if}
    {:else if activeTab === 'narrate'}
      <NarratePanel {campaignId} />
    {:else if activeTab === 'approvals'}
      <CharacterApprovalQueue {campaignId} />
    {:else if activeTab === 'message-player'}
      <MessagePlayerPanel {campaignId} />
    {:else if activeTab === 'quick-actions'}
      <QuickActionsPanel {campaignId} {encounters} />
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
  .encounter-select {
    margin-bottom: 0.5rem;
  }
  .encounter-select select {
    width: 100%;
    padding: 0.5rem;
    background: #1a1a2e;
    color: #e0e0e0;
    border: 1px solid #0f3460;
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
