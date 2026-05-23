<script>
  // Character-creation wizard (Basics -> Classes -> Ability Scores ->
  // Equipment -> Spells -> Features -> Review) backed by the dashboard
  // character-create JSON APIs in internal/dashboard/charcreate_handler.go.
  import {
    fetchRaces,
    fetchClasses,
    fetchEquipment,
    fetchStartingEquipment,
    fetchSpells,
    fetchAbilityMethods,
    previewCharacter,
    createCharacter,
    parseSubclasses,
    roll4d6DropLowest,
    standardArrayScores,
    pointBuyStartingScores,
    labelForAbilityMethod,
  } from './lib/charcreate.js';

  let { campaignId = '' } = $props();

  // Wizard navigation
  const STEPS = [
    { id: 1, label: 'Basics' },
    { id: 2, label: 'Classes' },
    { id: 3, label: 'Ability Scores' },
    { id: 4, label: 'Equipment' },
    { id: 5, label: 'Spells' },
    { id: 6, label: 'Features' },
    { id: 7, label: 'Review' },
  ];
  let step = $state(1);

  // Basics
  let charName = $state('');
  let charRace = $state('');
  let charBackground = $state('');

  // Reference data
  let races = $state([]);
  let classOptions = $state([]);
  let allEquipment = $state([]);
  let abilityMethods = $state(['point_buy', 'standard_array', 'roll']);

  // Class entries
  /** @type {{ class: string, subclass: string, level: number }[]} */
  let classEntries = $state([{ class: '', subclass: '', level: 1 }]);

  // Ability scores
  let abilityMethod = $state('roll');
  let abilityScores = $state({ str: 10, dex: 10, con: 10, int: 10, wis: 10, cha: 10 });
  /** @type {Record<string, number[]>} */
  let abilityRolls = $state({});

  // Equipment
  /** @type {string[]} */
  let selectedEquipment = $state([]);
  let wornArmor = $state('');
  let equippedWeapon = $state('');

  // Spells
  let spellClass = $state('');
  /** @type {Record<string, object[]>} */
  let spellsCache = $state({});
  /** @type {object[]} */
  let availableSpells = $state([]);
  /** @type {string[]} */
  let selectedSpells = $state([]);

  // Preview / features / status
  let preview = $state(null);
  let featureLoading = $state(false);
  let creating = $state(false);
  let status = $state(null); // { type, msg }

  // Boot: pull reference data in parallel. Failures degrade gracefully —
  // the panel can still operate with partial data, the relevant select
  // just stays empty.
  $effect(() => {
    fetchRaces()
      .then((data) => {
        races = data || [];
      })
      .catch((err) => setError(err));
    fetchClasses()
      .then((data) => {
        classOptions = data || [];
      })
      .catch((err) => setError(err));
    fetchEquipment(campaignId)
      .then((data) => {
        allEquipment = data || [];
      })
      .catch((err) => setError(err));
    fetchAbilityMethods(campaignId)
      .then((methods) => {
        if (!methods || methods.length === 0) return;
        abilityMethods = methods;
        if (!methods.includes(abilityMethod)) abilityMethod = methods[0];
      })
      .catch((err) => setError(err));
  });

  // Class option lookup memoised so the subclass derivation is cheap.
  const classOptionByName = $derived.by(() => {
    const m = new Map();
    for (const c of classOptions) m.set(c.name, c);
    return m;
  });

  // Subclasses for each chosen class entry.
  const subclassChoices = $derived(
    classEntries.map((entry) => parseSubclasses(classOptionByName.get(entry.class)?.subclasses)),
  );

  // Equipment derived views (armor + weapons in the selected list).
  const equipmentByID = $derived.by(() => {
    const m = new Map();
    for (const it of allEquipment) m.set(it.id, it);
    return m;
  });

  const selectedArmorOptions = $derived(
    selectedEquipment.filter((id) => {
      if (id === 'shield') return true;
      const item = equipmentByID.get(id);
      return item && item.category === 'armor';
    }),
  );

  const selectedWeaponOptions = $derived(
    selectedEquipment.filter((id) => {
      const item = equipmentByID.get(id);
      return item && item.category === 'weapon';
    }),
  );

  // Class names the user has assigned to a non-empty value.
  const spellableClasses = $derived(
    classEntries
      .map((e) => e.class)
      .filter((name) => Boolean(name)),
  );

  function setError(err) {
    status = { type: 'error', msg: 'Error: ' + (err?.message || 'request failed') };
  }

  function clearStatus() {
    status = null;
  }

  function addClass() {
    classEntries = [...classEntries, { class: '', subclass: '', level: 1 }];
  }

  function removeClass(index) {
    if (index === 0) return; // keep at least one entry
    classEntries = classEntries.filter((_, i) => i !== index);
  }

  function onClassSelected(index, name) {
    classEntries = classEntries.map((entry, i) =>
      i === index ? { ...entry, class: name, subclass: '' } : entry,
    );
  }

  function onSubclassSelected(index, name) {
    classEntries = classEntries.map((entry, i) =>
      i === index ? { ...entry, subclass: name } : entry,
    );
  }

  function onLevelSelected(index, level) {
    const lvl = Number(level) || 1;
    classEntries = classEntries.map((entry, i) =>
      i === index ? { ...entry, level: lvl } : entry,
    );
  }

  function setAbilityMethod(method) {
    abilityMethod = method;
    abilityRolls = {};
    if (method === 'point_buy') abilityScores = pointBuyStartingScores();
    if (method === 'standard_array') abilityScores = standardArrayScores();
  }

  function rollAbilityScores() {
    const next = {};
    const rolls = {};
    for (const ab of ['str', 'dex', 'con', 'int', 'wis', 'cha']) {
      const { score, dice } = roll4d6DropLowest();
      next[ab] = score;
      rolls[ab] = dice;
    }
    abilityScores = next;
    abilityRolls = rolls;
  }

  // True when the ability scores grid should be read-only (the user
  // generated them via dice/array rather than typing them in).
  const scoresReadOnly = $derived(
    abilityMethod === 'standard_array' || abilityMethod === 'roll',
  );

  function addEquipmentItem(id) {
    if (!id) return;
    if (selectedEquipment.includes(id)) return;
    selectedEquipment = [...selectedEquipment, id];
  }

  function removeEquipmentItem(id) {
    selectedEquipment = selectedEquipment.filter((e) => e !== id);
    if (wornArmor === id) wornArmor = '';
    if (equippedWeapon === id) equippedWeapon = '';
  }

  let equipPickerValue = $state('');
  function onAddEquipmentClick() {
    addEquipmentItem(equipPickerValue);
    equipPickerValue = '';
  }

  async function loadStartingEquipment() {
    clearStatus();
    const firstClass = classEntries[0]?.class;
    if (!firstClass) {
      status = { type: 'error', msg: 'Select a class first.' };
      return;
    }
    try {
      const packs = await fetchStartingEquipment(firstClass);
      if (!packs || packs.length === 0) return;
      const pack = packs[0];
      const next = [...selectedEquipment];
      for (const raw of pack.guaranteed || []) {
        const [id, qtyRaw] = String(raw).split(':');
        const qty = qtyRaw ? parseInt(qtyRaw, 10) : 1;
        if (qty === 1) {
          if (!next.includes(id)) next.push(id);
          continue;
        }
        for (let i = 0; i < qty; i += 1) next.push(id);
      }
      for (const choice of pack.choices || []) {
        if (!choice.options || choice.options.length === 0) continue;
        const id = String(choice.options[0]).split(':')[0].split(',')[0];
        if (!next.includes(id)) next.push(id);
      }
      selectedEquipment = next;
    } catch (err) {
      setError(err);
    }
  }

  let spellPickerValue = $state('');
  async function onSpellClassSelected(name) {
    spellClass = name;
    if (!name) {
      availableSpells = [];
      return;
    }
    const maxLevel = preview?.max_spell_level || 0;
    const cacheKey = `${name}_${maxLevel}`;
    if (spellsCache[cacheKey]) {
      availableSpells = spellsCache[cacheKey];
      return;
    }
    try {
      const fetched = await fetchSpells(name, { maxLevel, campaignId });
      const list = fetched || [];
      spellsCache = { ...spellsCache, [cacheKey]: list };
      availableSpells = list;
    } catch (err) {
      setError(err);
    }
  }

  function addSpell() {
    if (!spellPickerValue) return;
    if (selectedSpells.includes(spellPickerValue)) return;
    selectedSpells = [...selectedSpells, spellPickerValue];
    spellPickerValue = '';
  }

  function removeSpell(id) {
    selectedSpells = selectedSpells.filter((s) => s !== id);
  }

  function gatherSubmission() {
    return {
      name: charName,
      race: charRace,
      background: charBackground,
      classes: classEntries
        .filter((e) => e.class)
        .map((e) => ({ class: e.class, subclass: e.subclass, level: Number(e.level) || 1 })),
      ability_scores: {
        str: Number(abilityScores.str) || 10,
        dex: Number(abilityScores.dex) || 10,
        con: Number(abilityScores.con) || 10,
        int: Number(abilityScores.int) || 10,
        wis: Number(abilityScores.wis) || 10,
        cha: Number(abilityScores.cha) || 10,
      },
      ability_method: abilityMethod,
      ability_rolls: abilityRolls,
      equipment: selectedEquipment,
      spells: selectedSpells,
      equipped_weapon: equippedWeapon,
      worn_armor: wornArmor,
    };
  }

  async function computePreview() {
    clearStatus();
    try {
      const stats = await previewCharacter(gatherSubmission());
      preview = stats;
    } catch (err) {
      setError(err);
    }
  }

  async function loadFeatures() {
    clearStatus();
    featureLoading = true;
    try {
      const stats = await previewCharacter(gatherSubmission());
      preview = stats;
    } catch (err) {
      setError(err);
    } finally {
      featureLoading = false;
    }
  }

  function goToStep(n) {
    step = n;
    if (n === 5 && !spellClass && spellableClasses.length > 0) {
      onSpellClassSelected(spellableClasses[0]);
      return;
    }
    if (n === 6) loadFeatures();
  }

  async function submitCharacter() {
    clearStatus();
    creating = true;
    try {
      const submission = gatherSubmission();
      const payload = { campaign_id: campaignId, ...submission };
      const result = await createCharacter(payload);
      status = {
        type: 'success',
        msg: `Character created! ID: ${result.character_id}`,
      };
    } catch (err) {
      setError(err);
    } finally {
      creating = false;
    }
  }

  const STAT_LABELS = ['STR', 'DEX', 'CON', 'INT', 'WIS', 'CHA'];
  const ABILITY_KEYS = ['str', 'dex', 'con', 'int', 'wis', 'cha'];

  function signed(value) {
    if (typeof value !== 'number') return value;
    return value >= 0 ? `+${value}` : String(value);
  }

  function nameFor(id) {
    const item = equipmentByID.get(id);
    return item ? item.name : id;
  }
</script>

<section class="charcreate-panel">
  <header>
    <h2>Create Character</h2>
    <p class="hint">
      Build a DM-controlled character. campaignId: {campaignId || '(none)'}
    </p>
  </header>

  <ol class="wizard-steps">
    {#each STEPS as s}
      <li
        class:active={s.id === step}
        class:done={s.id < step}
        data-step={s.id}
      >
        {s.id}. {s.label}
      </li>
    {/each}
  </ol>

  {#if status}
    <div class="alert alert-{status.type}" role={status.type === 'error' ? 'alert' : 'status'}>
      {status.msg}
    </div>
  {/if}

  {#if step === 1}
    <div class="form-group">
      <label for="cc-name">Character Name</label>
      <input id="cc-name" type="text" bind:value={charName} placeholder="Enter character name" />
    </div>
    <div class="form-group">
      <label for="cc-race">Race</label>
      <select id="cc-race" bind:value={charRace}>
        <option value="">Select race...</option>
        {#each races as race}
          <option value={race.name}>{race.name}</option>
        {/each}
      </select>
    </div>
    <div class="form-group">
      <label for="cc-bg">Background</label>
      <input id="cc-bg" type="text" bind:value={charBackground} placeholder="Enter background" />
    </div>
    <div class="actions">
      <button class="btn btn-primary" type="button" onclick={() => goToStep(2)}>Next: Classes</button>
    </div>
  {/if}

  {#if step === 2}
    <div class="class-entries">
      {#each classEntries as entry, i (i)}
        <div class="class-entry">
          <select
            class="class-select"
            value={entry.class}
            onchange={(e) => onClassSelected(i, e.currentTarget.value)}
          >
            <option value="">Class...</option>
            {#each classOptions as c}
              <option value={c.name}>{c.name}</option>
            {/each}
          </select>
          <select
            class="subclass-select"
            value={entry.subclass}
            onchange={(e) => onSubclassSelected(i, e.currentTarget.value)}
          >
            <option value="">Subclass...</option>
            {#each subclassChoices[i] as sub}
              <option value={sub.value}>{sub.label}</option>
            {/each}
          </select>
          <input
            class="level-input"
            type="number"
            min="1"
            max="20"
            value={entry.level}
            onchange={(e) => onLevelSelected(i, e.currentTarget.value)}
          />
          {#if i > 0}
            <button class="btn btn-secondary btn-x" type="button" onclick={() => removeClass(i)}>X</button>
          {/if}
        </div>
      {/each}
    </div>
    <button class="btn btn-secondary spaced-top" type="button" onclick={addClass}>+ Add Class</button>
    <div class="actions">
      <button class="btn btn-secondary" type="button" onclick={() => goToStep(1)}>Back</button>
      <button class="btn btn-primary" type="button" onclick={() => goToStep(3)}>Next: Ability Scores</button>
    </div>
  {/if}

  {#if step === 3}
    <div class="method-tabs">
      {#each abilityMethods as method}
        <button
          type="button"
          class="btn btn-secondary"
          class:active={method === abilityMethod}
          onclick={() => setAbilityMethod(method)}
        >
          {labelForAbilityMethod(method)}
        </button>
      {/each}
    </div>

    {#if abilityMethod === 'roll'}
      <button class="btn btn-secondary spaced-bottom" type="button" onclick={rollAbilityScores}>
        Roll 4d6
      </button>
    {/if}

    <div class="ability-grid">
      {#each ABILITY_KEYS as ab, i}
        <div class="ability-item">
          <label for={`cc-score-${ab}`}>{STAT_LABELS[i]}</label>
          <input
            id={`cc-score-${ab}`}
            type="number"
            min="1"
            max="30"
            bind:value={abilityScores[ab]}
            readonly={scoresReadOnly}
          />
        </div>
      {/each}
    </div>

    {#if preview}
      <div class="preview-panel">
        <h3>Derived Stats Preview</h3>
        <div class="stat-grid">
          <div class="stat-item"><div class="label">HP</div><div class="value">{preview.hp_max}</div></div>
          <div class="stat-item"><div class="label">AC</div><div class="value">{preview.ac}</div></div>
          <div class="stat-item"><div class="label">Speed</div><div class="value">{preview.speed_ft} ft</div></div>
          <div class="stat-item"><div class="label">Prof</div><div class="value">+{preview.proficiency_bonus}</div></div>
          <div class="stat-item"><div class="label">Level</div><div class="value">{preview.total_level}</div></div>
          {#if preview.saves}
            {#each ABILITY_KEYS as ab, i}
              <div class="stat-item">
                <div class="label">{STAT_LABELS[i]} Save</div>
                <div class="value">{signed(preview.saves[ab])}</div>
              </div>
            {/each}
          {/if}
          {#if preview.spell_slots}
            {#each Object.keys(preview.spell_slots).sort() as lvl}
              <div class="stat-item">
                <div class="label">Slot Lvl {lvl}</div>
                <div class="value">{preview.spell_slots[lvl]}</div>
              </div>
            {/each}
          {/if}
        </div>
      </div>
    {/if}

    <div class="actions">
      <button class="btn btn-secondary" type="button" onclick={() => goToStep(2)}>Back</button>
      <button class="btn btn-primary" type="button" onclick={computePreview}>Preview Stats</button>
      <button class="btn btn-primary" type="button" onclick={() => goToStep(4)}>Next: Equipment</button>
    </div>
  {/if}

  {#if step === 4}
    <button class="btn btn-secondary spaced-bottom" type="button" onclick={loadStartingEquipment}>
      Load Starting Equipment for Class
    </button>

    <div class="form-group">
      <label for="cc-equip-select">Add Equipment</label>
      <select id="cc-equip-select" bind:value={equipPickerValue}>
        <option value="">Select item...</option>
        {#each allEquipment as item}
          <option value={item.id}>{item.name} ({item.category})</option>
        {/each}
      </select>
      <button class="btn btn-secondary spaced-top" type="button" onclick={onAddEquipmentClick}>+ Add</button>
    </div>

    <div class="item-list">
      {#each selectedEquipment as id (id)}
        <span class="item-tag">
          {nameFor(id)}
          <span
            class="remove"
            role="button"
            tabindex="0"
            onclick={() => removeEquipmentItem(id)}
            onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') removeEquipmentItem(id); }}
          >×</span>
        </span>
      {/each}
    </div>

    <div class="form-group spaced-top">
      <label for="cc-worn-armor">Worn Armor</label>
      <select id="cc-worn-armor" bind:value={wornArmor}>
        <option value="">None</option>
        {#each selectedArmorOptions as id}
          <option value={id}>{nameFor(id)}</option>
        {/each}
      </select>
    </div>
    <div class="form-group">
      <label for="cc-equipped-weapon">Equipped Weapon</label>
      <select id="cc-equipped-weapon" bind:value={equippedWeapon}>
        <option value="">None</option>
        {#each selectedWeaponOptions as id}
          <option value={id}>{nameFor(id)}</option>
        {/each}
      </select>
    </div>

    <div class="actions">
      <button class="btn btn-secondary" type="button" onclick={() => goToStep(3)}>Back</button>
      <button class="btn btn-primary" type="button" onclick={() => goToStep(5)}>Next: Spells</button>
    </div>
  {/if}

  {#if step === 5}
    <div class="form-group">
      <label for="cc-spell-class">Class Spell List</label>
      <select id="cc-spell-class" value={spellClass} onchange={(e) => onSpellClassSelected(e.currentTarget.value)}>
        <option value="">Select class...</option>
        {#each spellableClasses as name}
          <option value={name}>{name}</option>
        {/each}
      </select>
    </div>
    <div class="form-group">
      <label for="cc-spell-select">Add Spell</label>
      <select id="cc-spell-select" bind:value={spellPickerValue}>
        <option value="">Select spell...</option>
        {#each availableSpells as sp}
          <option value={sp.id}>{sp.name} (Lvl {sp.level})</option>
        {/each}
      </select>
      <button class="btn btn-secondary spaced-top" type="button" onclick={addSpell}>+ Add</button>
    </div>

    <div class="item-list">
      {#each selectedSpells as id (id)}
        <span class="item-tag">
          {id}
          <span
            class="remove"
            role="button"
            tabindex="0"
            onclick={() => removeSpell(id)}
            onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') removeSpell(id); }}
          >×</span>
        </span>
      {/each}
    </div>

    <div class="actions">
      <button class="btn btn-secondary" type="button" onclick={() => goToStep(4)}>Back</button>
      <button class="btn btn-primary" type="button" onclick={() => goToStep(6)}>Next: Features</button>
    </div>
  {/if}

  {#if step === 6}
    <p class="hint">
      Features are auto-populated from your class, subclass, and race choices.
    </p>
    {#if featureLoading}
      <p class="hint">Loading features...</p>
    {:else if preview && preview.features && preview.features.length > 0}
      <div class="features-list">
        {#each preview.features as f}
          <div class="feature-card">
            <div class="feat-name">{f.name}</div>
            <div class="feat-source">{f.source} (Level {f.level})</div>
            {#if f.description}
              <div class="feat-desc">{f.description}</div>
            {/if}
          </div>
        {/each}
      </div>
    {:else}
      <p class="hint">No features available for current class selection.</p>
    {/if}
    <div class="actions">
      <button class="btn btn-secondary" type="button" onclick={() => goToStep(5)}>Back</button>
      <button class="btn btn-primary" type="button" onclick={() => goToStep(7)}>Next: Review</button>
    </div>
  {/if}

  {#if step === 7}
    {@const submission = gatherSubmission()}
    <div class="review-summary">
      <h3>Review</h3>
      <p><strong>Name:</strong> {submission.name || '(none)'}</p>
      <p><strong>Race:</strong> {submission.race || '(none)'}</p>
      <p><strong>Background:</strong> {submission.background || '(none)'}</p>
      <p>
        <strong>Classes:</strong>
        {#if submission.classes.length === 0}
          (none)
        {:else}
          {#each submission.classes as c, i}
            {#if i > 0} / {/if}
            {c.class} {c.level}{#if c.subclass} ({c.subclass}){/if}
          {/each}
        {/if}
      </p>
      <p>
        <strong>Equipment:</strong>
        {selectedEquipment.length > 0 ? selectedEquipment.join(', ') : '(none)'}
      </p>
      {#if submission.worn_armor}
        <p><strong>Worn Armor:</strong> {submission.worn_armor}</p>
      {/if}
      {#if submission.equipped_weapon}
        <p><strong>Equipped Weapon:</strong> {submission.equipped_weapon}</p>
      {/if}
      <p>
        <strong>Spells:</strong>
        {selectedSpells.length > 0 ? selectedSpells.join(', ') : '(none)'}
      </p>
    </div>
    <div class="actions">
      <button class="btn btn-secondary" type="button" onclick={() => goToStep(6)}>Back</button>
      <button class="btn btn-success" type="button" onclick={submitCharacter} disabled={creating}>
        {creating ? 'Creating...' : 'Create Character'}
      </button>
    </div>
  {/if}
</section>

<style>
  .charcreate-panel {
    max-width: 900px;
    background: #16213e;
    border: 1px solid #0f3460;
    border-radius: 4px;
    padding: 1rem;
  }
  header h2 {
    color: #e94560;
    margin: 0 0 0.25rem;
  }
  .hint {
    color: #b0b0c0;
    margin: 0 0 1rem;
    font-size: 0.9rem;
  }
  .wizard-steps {
    list-style: none;
    display: flex;
    flex-wrap: wrap;
    gap: 0.5rem;
    margin: 0 0 1rem;
    padding: 0;
  }
  .wizard-steps li {
    padding: 0.5rem 1rem;
    background: #1a1a2e;
    border: 1px solid #0f3460;
    border-radius: 6px;
    color: #999;
    font-size: 0.9rem;
  }
  .wizard-steps li.active {
    border-color: #e94560;
    color: #e94560;
    font-weight: 700;
  }
  .wizard-steps li.done {
    border-color: #27ae60;
    color: #27ae60;
  }
  .form-group {
    margin-bottom: 1rem;
  }
  .form-group label {
    display: block;
    margin-bottom: 0.25rem;
    color: #e94560;
    font-weight: 700;
  }
  .form-group input,
  .form-group select {
    width: 100%;
    padding: 0.5rem;
    border-radius: 4px;
    border: 1px solid #0f3460;
    background: #16213e;
    color: #e0e0e0;
    font-size: 1rem;
    box-sizing: border-box;
  }
  .ability-grid {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 1rem;
  }
  .ability-item {
    text-align: center;
  }
  .ability-item label {
    display: block;
    color: #b0b0c0;
    font-size: 0.9rem;
  }
  .ability-item input {
    width: 80px;
    text-align: center;
    margin: 0 auto;
    display: block;
    background: #1a1a2e;
    color: #e0e0e0;
    border: 1px solid #0f3460;
    border-radius: 4px;
    padding: 0.4rem;
  }
  .class-entries {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
  }
  .class-entry {
    display: flex;
    gap: 0.5rem;
    align-items: center;
    background: #1a1a2e;
    padding: 0.5rem;
    border: 1px solid #0f3460;
    border-radius: 4px;
  }
  .class-entry select,
  .class-entry .level-input {
    flex: 1;
    padding: 0.4rem;
    border: 1px solid #0f3460;
    background: #16213e;
    color: #e0e0e0;
    border-radius: 4px;
  }
  .class-entry .level-input {
    flex: 0 0 80px;
    text-align: center;
  }
  .btn-x {
    padding: 0.2rem 0.5rem;
    font-size: 0.8rem;
  }
  .method-tabs {
    display: flex;
    gap: 0.5rem;
    flex-wrap: wrap;
    margin-bottom: 1rem;
  }
  .method-tabs .btn.active {
    background: #e94560;
    border-color: #e94560;
    color: #fff;
  }
  .preview-panel {
    background: #1a1a2e;
    border: 1px solid #0f3460;
    border-radius: 4px;
    padding: 1rem;
    margin-top: 1rem;
  }
  .preview-panel h3 {
    color: #e94560;
    margin: 0 0 0.5rem;
  }
  .stat-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(110px, 1fr));
    gap: 0.5rem;
  }
  .stat-item {
    background: #16213e;
    padding: 0.5rem;
    border-radius: 4px;
    text-align: center;
  }
  .stat-item .label {
    color: #999;
    font-size: 0.8rem;
    text-transform: uppercase;
  }
  .stat-item .value {
    color: #e94560;
    font-size: 1.1rem;
    font-weight: 700;
  }
  .item-list {
    display: flex;
    flex-wrap: wrap;
    gap: 0.4rem;
    margin-top: 0.4rem;
  }
  .item-tag {
    background: #0f3460;
    padding: 0.3rem 0.6rem;
    border-radius: 4px;
    font-size: 0.85rem;
    display: inline-flex;
    align-items: center;
    gap: 0.3rem;
  }
  .item-tag .remove {
    cursor: pointer;
    color: #e94560;
    font-weight: 700;
    padding: 0 0.15rem;
  }
  .actions {
    display: flex;
    gap: 0.5rem;
    margin-top: 1rem;
  }
  .btn {
    padding: 0.5rem 1rem;
    border: 1px solid transparent;
    border-radius: 4px;
    cursor: pointer;
    font-size: 0.95rem;
    color: #fff;
  }
  .btn:disabled {
    opacity: 0.6;
    cursor: not-allowed;
  }
  .btn-primary {
    background: #e94560;
  }
  .btn-primary:hover:not(:disabled) {
    background: #c73852;
  }
  .btn-secondary {
    background: #0f3460;
    border-color: #0f3460;
  }
  .btn-secondary:hover:not(:disabled) {
    background: #0a2540;
  }
  .btn-success {
    background: #27ae60;
  }
  .btn-success:hover:not(:disabled) {
    background: #219a52;
  }
  .feature-card {
    background: #1a1a2e;
    border: 1px solid #0f3460;
    border-radius: 4px;
    padding: 0.5rem;
    margin-bottom: 0.4rem;
  }
  .feature-card .feat-name {
    color: #e94560;
    font-weight: 700;
  }
  .feature-card .feat-source {
    color: #999;
    font-size: 0.8rem;
  }
  .feature-card .feat-desc {
    margin-top: 0.25rem;
    font-size: 0.9rem;
  }
  .spaced-top {
    margin-top: 0.5rem;
  }
  .spaced-bottom {
    margin-bottom: 1rem;
  }
  .alert {
    padding: 0.6rem 0.8rem;
    border-radius: 4px;
    margin-bottom: 1rem;
  }
  .alert-success {
    background: #1b4332;
    border: 1px solid #2d6a4f;
    color: #d8f3dc;
  }
  .alert-error {
    background: #4a1b1b;
    border: 1px solid #6a2d2d;
    color: #ff6b6b;
  }
  .review-summary {
    background: #1a1a2e;
    border: 1px solid #0f3460;
    border-radius: 4px;
    padding: 1rem;
  }
  .review-summary h3 {
    color: #e94560;
    margin: 0 0 0.5rem;
  }
  .review-summary p {
    margin: 0.25rem 0;
  }
  @media (max-width: 768px) {
    .ability-grid {
      grid-template-columns: 1fr;
    }
  }
</style>
