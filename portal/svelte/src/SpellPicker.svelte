<script>
  import { isConcentration, formatCastingTime } from './lib/spell-perks.js';
  import { filterSpells, groupSpellsBySchool, availableLevels } from './lib/spell-filter.js';
  import { countAgainstCap, countByBucket, isSpellDisabled, disabledReason, toggleSelected, visibleSpells } from './lib/spell-picker.js';
  import { spellHeadline, spellDetailMeta } from './lib/spell-details.js';

  // Shared spell-selection UI for both the character builder's spell step and
  // the standalone spell-prep page. `selected` is the two-way bound list of
  // chosen spell ids. `max` caps how many *leveled* spells count against the
  // limit; `Infinity` means uncapped. `cantripMax` (default `Infinity`) caps
  // cantrips separately — when finite the picker enforces two budgets, otherwise
  // cantrips share `max` (single-cap mode, used by the prep page).
  // `selectableLevels` (null = no gate) restricts which leveled spells can be
  // picked while still letting every level be browsed. `alwaysPrepared` are
  // subclass-granted ids that stay locked-on and never count against a cap.
  let {
    spells = [],
    selected = $bindable([]),
    max = Infinity,
    cantripMax = Infinity,
    selectableLevels = null,
    alwaysPrepared = [],
  } = $props();

  let query = $state('');
  let levelFilter = $state('');
  let hideUnselectable = $state(false);
  let activeSpellId = $state(null);

  let dualBudget = $derived(Number.isFinite(cantripMax));
  let buckets = $derived(countByBucket(selected, { spells, alwaysPrepared, cantripMax }));
  let counted = $derived(countAgainstCap(selected, alwaysPrepared));
  let pickerOpts = $derived({ selected, alwaysPrepared, max, cantripMax, selectableLevels, spells });
  let filtered = $derived(filterSpells(spells, { query, level: levelFilter }));
  let visible = $derived(visibleSpells(filtered, hideUnselectable, pickerOpts));
  let groups = $derived(groupSpellsBySchool(visible));
  let levels = $derived(availableLevels(spells));
  let atCap = $derived(
    dualBudget
      ? buckets.cantrips >= cantripMax && (max === Infinity || buckets.leveled >= max)
      : max !== Infinity && counted >= max,
  );

  // Detail sidebar: resolve the clicked spell from the live catalog so its
  // selected/disabled state stays fresh as picks change.
  let activeSpell = $derived(activeSpellId == null ? null : spells.find((s) => s.id === activeSpellId) || null);
  let activeIsAlways = $derived(activeSpell != null && alwaysPrepared.includes(activeSpell.id));
  let activeIsSelected = $derived(activeSpell != null && (selected.includes(activeSpell.id) || activeIsAlways));
  let activeDisabled = $derived(activeSpell != null && isSpellDisabled(activeSpell, pickerOpts));
  let activeReason = $derived(activeSpell == null ? '' : disabledReason(activeSpell, pickerOpts));
  let activeMeta = $derived(spellDetailMeta(activeSpell));

  function toggle(id) {
    selected = toggleSelected(selected, id, alwaysPrepared);
  }

  function openDetails(spell) {
    activeSpellId = spell.id;
  }

  function closeDetails() {
    activeSpellId = null;
  }

  function onKeydown(event) {
    if (event.key === 'Escape') closeDetails();
  }
</script>

<svelte:window onkeydown={onKeydown} />

<div class="spell-picker" class:has-detail={activeSpell != null}>
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
      {#if dualBudget}{buckets.cantrips} / {cantripMax} cantrips · {buckets.leveled}{#if max !== Infinity} / {max}{/if} spells{:else if max === Infinity}{counted} selected{:else}{counted} / {max} prepared{/if}
      {#if alwaysPrepared.length > 0}<span class="always-note"> · +{alwaysPrepared.length} always</span>{/if}
    </span>
  </div>

  <div class="spell-layout">
    <div class="spell-list">
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
              {@const disabled = isSpellDisabled(spell, pickerOpts)}
              {@const reason = disabledReason(spell, pickerOpts)}
              <div
                class="spell-option"
                class:selected={isSelected}
                class:disabled={disabled && !isSelected}
                class:active={activeSpellId === spell.id}
                title={reason}
              >
                <div class="spell-row">
                  <input type="checkbox" checked={isSelected} disabled={disabled} onchange={() => toggle(spell.id)} />
                  <button type="button" class="spell-open" aria-expanded={activeSpellId === spell.id} onclick={() => openDetails(spell)}>
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
                  </button>
                </div>
                {#if spell.description}
                  <button type="button" class="spell-desc" onclick={() => openDetails(spell)}>{spell.description}</button>
                {/if}
                {#if reason && !isSelected}
                  <span class="spell-lock-reason">{reason}</span>
                {/if}
              </div>
            {/each}
          </div>
        </details>
      {/each}
    </div>

    {#if activeSpell}
      <aside class="spell-detail" aria-label="Spell details">
        <div class="detail-head">
          <h3 class="detail-name">{activeSpell.name}</h3>
          <button type="button" class="detail-close" aria-label="Close spell details" onclick={closeDetails}>×</button>
        </div>
        <p class="detail-headline">
          {spellHeadline(activeSpell)}
          {#if isConcentration(activeSpell.duration)}<span class="conc-tag">Concentration</span>{/if}
          {#if activeIsAlways}<span class="always-tag">Always prepared</span>{/if}
        </p>

        {#if activeMeta.length > 0}
          <dl class="detail-meta">
            {#each activeMeta as row (row.label)}
              <div class="detail-meta-row">
                <dt>{row.label}</dt>
                <dd>{row.value}</dd>
              </div>
            {/each}
          </dl>
        {/if}

        {#if activeSpell.description}
          <p class="detail-desc">{activeSpell.description}</p>
        {/if}

        {#if activeIsAlways}
          <p class="detail-lock">Granted by your subclass — always prepared.</p>
        {:else}
          <button
            type="button"
            class="detail-toggle"
            class:remove={activeIsSelected}
            disabled={activeDisabled && !activeIsSelected}
            onclick={() => toggle(activeSpell.id)}
          >
            {activeIsSelected ? 'Remove from selection' : 'Add to selection'}
          </button>
          {#if activeReason && !activeIsSelected}
            <p class="detail-lock">{activeReason}</p>
          {/if}
        {/if}
      </aside>
    {/if}
  </div>
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

  .spell-layout { display: flex; gap: 1rem; align-items: flex-start; }
  .spell-list { flex: 1 1 auto; min-width: 0; }

  .spell-school-group { margin-bottom: 0.6rem; border: 1px solid #0f3460; border-radius: 8px; overflow: hidden; }
  .spell-school-group > summary { cursor: pointer; list-style: none; padding: 0.5rem 0.7rem; background: #16213e; font-weight: 600; display: flex; align-items: center; gap: 0.5rem; }
  .spell-school-group > summary::-webkit-details-marker { display: none; }
  .spell-school-group > summary::before { content: '▸'; color: #888; }
  .spell-school-group[open] > summary::before { content: '▾'; }
  .spell-school-group[open] > summary { border-bottom: 1px solid #0f3460; }
  .school-count { color: #888; font-weight: 400; font-size: 0.85rem; }

  .spell-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(300px, 1fr)); gap: 0.5rem; padding: 0.6rem; }
  .spell-option { display: flex; flex-direction: column; align-items: stretch; gap: 0.25rem; padding: 0.5rem 0.6rem; border: 1px solid #0f3460; border-radius: 6px; background: #16213e; }
  .spell-option.selected { border-color: #e94560; background: #1f2a4d; }
  .spell-option.disabled { opacity: 0.45; }
  .spell-option.active { box-shadow: 0 0 0 2px #4a90d9; border-color: #4a90d9; }
  .spell-row { display: flex; align-items: center; gap: 0.5rem; flex-wrap: wrap; }
  .spell-open { flex: 1; display: flex; align-items: center; gap: 0.5rem; flex-wrap: wrap; min-width: 0; padding: 0; border: none; background: none; color: inherit; font: inherit; text-align: left; cursor: pointer; }
  .spell-name { font-weight: 600; }
  .spell-level { color: #888; font-size: 0.82rem; }
  .spell-meta { color: #888; font-size: 0.78rem; }
  .conc-tag { padding: 0.05rem 0.4rem; border: 1px solid #e94560; color: #e94560; border-radius: 8px; font-size: 0.7rem; }
  .always-tag { padding: 0.05rem 0.4rem; border: 1px solid #4a90d9; color: #4a90d9; border-radius: 8px; font-size: 0.7rem; }
  .spell-desc { display: -webkit-box; -webkit-line-clamp: 2; line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden; width: 100%; padding: 0; border: none; background: none; text-align: left; cursor: pointer; color: #aaa; font-size: 0.82rem; line-height: 1.3; margin-top: 0.1rem; }
  .spell-desc:hover { color: #ccc; }
  .spell-lock-reason { color: #f5a623; font-size: 0.72rem; font-style: italic; }

  .spell-detail { flex: 0 0 320px; position: sticky; top: 3.2rem; max-height: calc(100vh - 4rem); overflow-y: auto; padding: 0.9rem 1rem; background: #16213e; border: 1px solid #0f3460; border-radius: 8px; }
  .detail-head { display: flex; align-items: flex-start; gap: 0.5rem; }
  .detail-name { margin: 0; flex: 1; font-size: 1.1rem; color: #e94560; }
  .detail-close { flex: 0 0 auto; padding: 0 0.4rem; border: none; background: none; color: #aaa; font-size: 1.3rem; line-height: 1; cursor: pointer; }
  .detail-close:hover { color: #fff; }
  .detail-headline { margin: 0.2rem 0 0.7rem; color: #ccc; font-size: 0.85rem; text-transform: capitalize; display: flex; align-items: center; gap: 0.4rem; flex-wrap: wrap; }
  .detail-meta { margin: 0 0 0.7rem; display: grid; gap: 0.35rem; }
  .detail-meta-row { display: flex; gap: 0.5rem; font-size: 0.82rem; }
  .detail-meta-row dt { flex: 0 0 6.5rem; color: #888; }
  .detail-meta-row dd { margin: 0; color: #e0e0e0; }
  .detail-desc { margin: 0 0 0.9rem; color: #cfcfcf; font-size: 0.85rem; line-height: 1.45; white-space: pre-wrap; }
  .detail-toggle { width: 100%; padding: 0.5rem; border: 1px solid #e94560; border-radius: 6px; background: #e94560; color: #fff; font-weight: 600; font-size: 0.88rem; cursor: pointer; }
  .detail-toggle.remove { background: none; color: #e94560; }
  .detail-toggle:disabled { opacity: 0.45; cursor: not-allowed; }
  .detail-lock { margin: 0.5rem 0 0; color: #f5a623; font-size: 0.78rem; font-style: italic; }

  @media (max-width: 720px) {
    .spell-layout { flex-direction: column; }
    .spell-detail { flex: 1 1 auto; width: 100%; position: static; max-height: none; }
  }
</style>
