<script>
  import { isConcentration, formatCastingTime } from './lib/spell-perks.js';
  import { filterSpells, groupSpellsBySchool, availableLevels } from './lib/spell-filter.js';
  import { countAgainstCap, isSpellDisabled, disabledReason, toggleSelected, visibleSpells } from './lib/spell-picker.js';

  // Shared spell-selection UI for both the character builder's spell step and
  // the standalone spell-prep page. `selected` is the two-way bound list of
  // chosen spell ids. `max` caps how many count against the limit; `Infinity`
  // means uncapped. `selectableLevels` (null = no gate) restricts which leveled
  // spells can be picked while still letting every level be browsed.
  // `alwaysPrepared` are subclass-granted ids that stay locked-on and never
  // count against the cap.
  let {
    spells = [],
    selected = $bindable([]),
    max = Infinity,
    selectableLevels = null,
    alwaysPrepared = [],
  } = $props();

  let query = $state('');
  let levelFilter = $state('');
  let hideUnselectable = $state(false);

  let counted = $derived(countAgainstCap(selected, alwaysPrepared));
  let filtered = $derived(filterSpells(spells, { query, level: levelFilter }));
  let visible = $derived(
    visibleSpells(filtered, hideUnselectable, { selected, alwaysPrepared, max, selectableLevels }),
  );
  let groups = $derived(groupSpellsBySchool(visible));
  let levels = $derived(availableLevels(spells));
  let atCap = $derived(max !== Infinity && counted >= max);

  function toggle(id) {
    selected = toggleSelected(selected, id, alwaysPrepared);
  }
</script>

<div class="spell-picker">
  <div class="spell-toolbar">
    <input class="spell-search" type="text" placeholder="Search spells…" bind:value={query} />
    <select class="spell-level-filter" bind:value={levelFilter}>
      <option value="">All levels</option>
      {#each levels as lvl}
        <option value={lvl}>{lvl === 0 ? 'Cantrips' : `Level ${lvl}`}</option>
      {/each}
    </select>
    <label class="spell-hide-toggle" title="Hide spells you can't pick right now">
      <input type="checkbox" bind:checked={hideUnselectable} />
      Hide unselectable
    </label>
    <span class="spell-selected-count" class:at-cap={atCap}>
      {#if max === Infinity}{counted} selected{:else}{counted} / {max} prepared{/if}
      {#if alwaysPrepared.length > 0}<span class="always-note"> · +{alwaysPrepared.length} always</span>{/if}
    </span>
  </div>

  {#if visible.length === 0}
    <p class="spell-empty">
      {#if hideUnselectable && filtered.length > 0}No selectable spells — uncheck "Hide unselectable" to see the rest.{:else}No spells match your search.{/if}
    </p>
  {/if}

  {#each groups as group (group.school)}
    <details class="spell-school-group" open>
      <summary>
        <span class="school-name">{group.label}</span>
        <span class="school-count">{group.spells.length}</span>
      </summary>
      <div class="spell-grid">
        {#each group.spells as spell (spell.id)}
          {@const castingTime = formatCastingTime(spell.casting_time)}
          {@const isAlways = alwaysPrepared.includes(spell.id)}
          {@const isSelected = selected.includes(spell.id) || isAlways}
          {@const disabled = isSpellDisabled(spell, { selected, alwaysPrepared, max, selectableLevels })}
          {@const reason = disabledReason(spell, { selected, alwaysPrepared, max, selectableLevels })}
          <label
            class="spell-option"
            class:selected={isSelected}
            class:disabled={disabled && !isSelected}
            title={reason}
          >
            <span class="spell-row">
              <input type="checkbox" checked={isSelected} disabled={disabled} onchange={() => toggle(spell.id)} />
              <span class="spell-name">{spell.name}</span>
              <span class="spell-level">{spell.level === 0 ? 'Cantrip' : `Lvl ${spell.level}`}</span>
              {#if castingTime}
                <span class="spell-meta">{castingTime}</span>
              {/if}
              {#if isConcentration(spell.duration)}
                <span class="conc-tag">Concentration</span>
              {/if}
              {#if isAlways}
                <span class="always-tag">Always</span>
              {/if}
            </span>
            {#if spell.description}
              <span class="spell-desc" title={spell.description}>{spell.description}</span>
            {/if}
            {#if reason && !isSelected}
              <span class="spell-lock-reason">{reason}</span>
            {/if}
          </label>
        {/each}
      </div>
    </details>
  {/each}
</div>

<style>
  .spell-toolbar { position: sticky; top: 0; z-index: 2; display: flex; gap: 0.6rem; align-items: center; flex-wrap: wrap; padding: 0.5rem 0; margin-bottom: 0.5rem; background: #1a1a2e; }
  .spell-search { flex: 1 1 220px; padding: 0.45rem 0.6rem; background: #16213e; border: 1px solid #0f3460; border-radius: 6px; color: #eee; font-size: 0.9rem; }
  .spell-level-filter { padding: 0.45rem 0.5rem; background: #16213e; border: 1px solid #0f3460; border-radius: 6px; color: #eee; font-size: 0.9rem; }
  .spell-hide-toggle { display: flex; align-items: center; gap: 0.35rem; color: #ccc; font-size: 0.85rem; cursor: pointer; user-select: none; }
  .spell-selected-count { margin-left: auto; color: #e94560; font-weight: 600; font-size: 0.9rem; }
  .spell-selected-count.at-cap { color: #f5a623; }
  .always-note { color: #888; font-weight: 400; }
  .spell-empty { color: #888; font-style: italic; }

  .spell-school-group { margin-bottom: 0.6rem; border: 1px solid #0f3460; border-radius: 8px; overflow: hidden; }
  .spell-school-group > summary { cursor: pointer; list-style: none; padding: 0.5rem 0.7rem; background: #16213e; font-weight: 600; display: flex; align-items: center; gap: 0.5rem; }
  .spell-school-group > summary::-webkit-details-marker { display: none; }
  .spell-school-group > summary::before { content: '▸'; color: #888; }
  .spell-school-group[open] > summary::before { content: '▾'; }
  .spell-school-group[open] > summary { border-bottom: 1px solid #0f3460; }
  .school-count { color: #888; font-weight: 400; font-size: 0.85rem; }

  .spell-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(300px, 1fr)); gap: 0.5rem; padding: 0.6rem; }
  .spell-option { display: flex; flex-direction: column; align-items: stretch; gap: 0.25rem; padding: 0.5rem 0.6rem; border: 1px solid #0f3460; border-radius: 6px; background: #16213e; cursor: pointer; }
  .spell-option.selected { border-color: #e94560; background: #1f2a4d; }
  .spell-option.disabled { opacity: 0.45; cursor: not-allowed; }
  .spell-row { display: flex; align-items: center; gap: 0.5rem; flex-wrap: wrap; }
  .spell-name { font-weight: 600; }
  .spell-level { color: #888; font-size: 0.82rem; }
  .spell-meta { color: #888; font-size: 0.78rem; }
  .conc-tag { padding: 0.05rem 0.4rem; border: 1px solid #e94560; color: #e94560; border-radius: 8px; font-size: 0.7rem; }
  .always-tag { padding: 0.05rem 0.4rem; border: 1px solid #4a90d9; color: #4a90d9; border-radius: 8px; font-size: 0.7rem; }
  .spell-desc { display: -webkit-box; -webkit-line-clamp: 2; line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden; color: #aaa; font-size: 0.82rem; line-height: 1.3; margin-top: 0.1rem; }
  .spell-lock-reason { color: #f5a623; font-size: 0.72rem; font-style: italic; }
</style>
