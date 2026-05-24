<script>
  import {
    abilityModifier,
    formatModifier,
    formatSpeed,
    formatModifierMap,
    formatSenses,
    formatAttack,
  } from './lib/statblockFormat.js';

  let { block } = $props();

  const ABILITIES = ['str', 'dex', 'con', 'int', 'wis', 'cha'];
  const ABILITY_LABELS = { str: 'STR', dex: 'DEX', con: 'CON', int: 'INT', wis: 'WIS', cha: 'CHA' };

  let scores = $derived(block?.ability_scores ?? {});
  let speedText = $derived(formatSpeed(block?.speed));
  let savesText = $derived(formatModifierMap(block?.saving_throws));
  let skillsText = $derived(formatModifierMap(block?.skills));
  let sensesText = $derived(formatSenses(block?.senses));
  let subtitle = $derived(buildSubtitle(block));

  function buildSubtitle(b) {
    const head = [b?.size, b?.type].filter(Boolean).join(' ');
    return b?.alignment ? `${head}, ${b.alignment}` : head;
  }

  function joinList(arr) {
    return Array.isArray(arr) ? arr.join(', ') : '';
  }
</script>

<div class="statblock-view" data-testid="statblock-view">
  {#if subtitle}
    <p class="subtitle">{subtitle}</p>
  {/if}

  <div class="rule"></div>
  <dl class="lines">
    <div class="line"><dt>Armor Class</dt><dd>{block.ac ?? '—'}{block.ac_type ? ` (${block.ac_type})` : ''}</dd></div>
    <div class="line"><dt>Hit Points</dt><dd>{block.hp_average ?? '—'}{block.hp_formula ? ` (${block.hp_formula})` : ''}</dd></div>
    {#if speedText}
      <div class="line"><dt>Speed</dt><dd>{speedText}</dd></div>
    {/if}
  </dl>

  <div class="rule"></div>
  <table class="abilities">
    <thead>
      <tr>{#each ABILITIES as a}<th>{ABILITY_LABELS[a]}</th>{/each}</tr>
    </thead>
    <tbody>
      <tr>
        {#each ABILITIES as a}
          <td>
            {#if scores[a] != null}
              {scores[a]} ({formatModifier(abilityModifier(scores[a]))})
            {:else}
              —
            {/if}
          </td>
        {/each}
      </tr>
    </tbody>
  </table>

  <div class="rule"></div>
  <dl class="lines">
    {#if savesText}<div class="line"><dt>Saving Throws</dt><dd>{savesText}</dd></div>{/if}
    {#if skillsText}<div class="line"><dt>Skills</dt><dd>{skillsText}</dd></div>{/if}
    {#if block.damage_resistances?.length}
      <div class="line"><dt>Damage Resistances</dt><dd>{joinList(block.damage_resistances)}</dd></div>
    {/if}
    {#if block.damage_immunities?.length}
      <div class="line"><dt>Damage Immunities</dt><dd>{joinList(block.damage_immunities)}</dd></div>
    {/if}
    {#if block.damage_vulnerabilities?.length}
      <div class="line"><dt>Damage Vulnerabilities</dt><dd>{joinList(block.damage_vulnerabilities)}</dd></div>
    {/if}
    {#if block.condition_immunities?.length}
      <div class="line"><dt>Condition Immunities</dt><dd>{joinList(block.condition_immunities)}</dd></div>
    {/if}
    {#if sensesText}<div class="line"><dt>Senses</dt><dd>{sensesText}</dd></div>{/if}
    {#if block.languages?.length}
      <div class="line"><dt>Languages</dt><dd>{joinList(block.languages)}</dd></div>
    {/if}
    {#if block.cr}<div class="line"><dt>Challenge</dt><dd>{block.cr}</dd></div>{/if}
  </dl>

  {#if block.abilities?.length}
    <div class="rule"></div>
    <section class="entries" data-testid="statblock-traits">
      {#each block.abilities as trait}
        <p><span class="entry-name">{trait.name}.</span> {trait.description}</p>
      {/each}
    </section>
  {/if}

  {#if block.attacks?.length}
    <h4>Actions</h4>
    <section class="entries" data-testid="statblock-actions">
      {#each block.attacks as attack}
        <p><span class="entry-name">{attack.name}.</span> {formatAttack(attack)}</p>
      {/each}
    </section>
  {/if}

  {#if block.bonus_actions?.length}
    <h4>Bonus Actions</h4>
    <section class="entries" data-testid="statblock-bonus-actions">
      {#each block.bonus_actions as bonus}
        <p><span class="entry-name">{bonus.name}.</span> {bonus.description}</p>
      {/each}
    </section>
  {/if}
</div>

<style>
  .statblock-view {
    color: #e0e0e0;
    font-size: 0.9rem;
  }
  .subtitle {
    margin: 0 0 0.5rem;
    font-style: italic;
    color: #b0b0c0;
  }
  .rule {
    height: 2px;
    background: #e94560;
    margin: 0.5rem 0;
    border-radius: 1px;
  }
  .lines {
    margin: 0;
  }
  .line {
    display: flex;
    gap: 0.4rem;
    padding: 0.1rem 0;
  }
  .line dt {
    font-weight: 600;
    color: #e94560;
    flex: 0 0 auto;
  }
  .line dd {
    margin: 0;
  }
  .abilities {
    width: 100%;
    border-collapse: collapse;
    text-align: center;
    margin: 0.25rem 0;
  }
  .abilities th {
    color: #e94560;
    font-size: 0.75rem;
    padding: 0.2rem 0;
  }
  .abilities td {
    padding: 0.2rem 0;
    white-space: nowrap;
  }
  h4 {
    color: #e94560;
    margin: 0.75rem 0 0.25rem;
    border-bottom: 1px solid #0f3460;
    padding-bottom: 0.15rem;
  }
  .entries p {
    margin: 0.35rem 0;
    line-height: 1.4;
  }
  .entry-name {
    font-weight: 600;
    font-style: italic;
    color: #e0e0e0;
  }
</style>
