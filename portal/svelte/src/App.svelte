<script>
  import { listRaces, listClasses, listSpells, listEquipment, getStartingEquipment, submitCharacter } from './lib/api.js';
  import { remainingPoints, abilityModifier, canIncrement, canDecrement, scoreCost } from './lib/pointbuy.js';

  let { token = '', campaignId = '' } = $props();

  // Steps
  const STEPS = ['Basics', 'Class', 'Ability Scores', 'Skills', 'Equipment', 'Spells', 'Review'];
  let currentStep = $state(0);

  // Form state preserved across steps
  let name = $state('');
  let race = $state('');
  let subrace = $state('');
  let background = $state('');
  let selectedClass = $state('');
  let subclass = $state('');
  let scores = $state({ str: 8, dex: 8, con: 8, int: 8, wis: 8, cha: 8 });
  let selectedSkills = $state([]);
  let equipment = $state([]);
  let selectedSpells = $state([]);

  // Reference data
  let races = $state([]);
  let classes = $state([]);
  let spells = $state([]);
  let allEquipment = $state([]);
  let startingPacks = $state([]);

  // Equipment selection state
  let packChoices = $state({});   // { choiceIndex: selectedOptionIndex }
  let manualEquipment = $state([]); // manually added item IDs
  let equipmentSearch = $state('');

  // UI state
  let loading = $state(false);
  let error = $state('');
  let submitted = $state(false);
  let submitting = $state(false);

  // Load reference data on mount
  $effect(() => {
    loadRefData();
  });

  async function loadRefData() {
    try {
      loading = true;
      const [r, c] = await Promise.all([listRaces(), listClasses()]);
      races = r;
      classes = c;
    } catch (e) {
      error = 'Failed to load reference data: ' + e.message;
    } finally {
      loading = false;
    }
  }

  // Load spells and starting equipment when class changes
  $effect(() => {
    if (selectedClass) {
      loadSpells(selectedClass);
      loadStartingEquipment(selectedClass);
    }
  });

  // Load full equipment list when entering equipment step
  $effect(() => {
    if (currentStep === 4 && allEquipment.length === 0) {
      loadEquipment();
    }
  });

  async function loadSpells(cls) {
    try {
      spells = await listSpells(cls);
    } catch (e) {
      spells = [];
    }
  }

  async function loadEquipment() {
    try {
      allEquipment = await listEquipment();
    } catch (e) {
      allEquipment = [];
    }
  }

  async function loadStartingEquipment(cls) {
    try {
      startingPacks = await getStartingEquipment(cls);
      packChoices = {};
    } catch (e) {
      startingPacks = [];
    }
  }

  function nextStep() {
    if (currentStep < STEPS.length - 1) currentStep++;
  }

  function prevStep() {
    if (currentStep > 0) currentStep--;
  }

  function goToStep(i) {
    currentStep = i;
  }

  function increment(ability) {
    if (canIncrement(scores, ability)) {
      scores = { ...scores, [ability]: scores[ability] + 1 };
    }
  }

  function decrement(ability) {
    if (canDecrement(scores, ability)) {
      scores = { ...scores, [ability]: scores[ability] - 1 };
    }
  }

  function toggleSkill(skill) {
    if (selectedSkills.includes(skill)) {
      selectedSkills = selectedSkills.filter(s => s !== skill);
    } else {
      selectedSkills = [...selectedSkills, skill];
    }
  }

  function selectPackChoice(choiceIdx, optionIdx) {
    packChoices = { ...packChoices, [choiceIdx]: optionIdx };
  }

  function addManualItem(itemId) {
    if (!manualEquipment.includes(itemId)) {
      manualEquipment = [...manualEquipment, itemId];
    }
  }

  function removeManualItem(itemId) {
    manualEquipment = manualEquipment.filter(id => id !== itemId);
  }

  // Derive the final equipment list from pack choices + manual items
  let selectedEquipment = $derived(() => {
    const items = [];
    // Add items from pack choices
    if (startingPacks.length > 0 && startingPacks[0]) {
      const pack = startingPacks[0];
      // Add guaranteed items
      if (pack.guaranteed) {
        for (const g of pack.guaranteed) {
          items.push(g.split(':')[0]);
        }
      }
      // Add chosen options
      if (pack.choices) {
        for (let i = 0; i < pack.choices.length; i++) {
          const chosen = packChoices[i];
          if (chosen !== undefined && pack.choices[i].options[chosen]) {
            const opt = pack.choices[i].options[chosen];
            // Option may contain comma-separated items with quantities
            for (const part of opt.split(',')) {
              items.push(part.split(':')[0]);
            }
          }
        }
      }
    }
    // Add manual items
    for (const id of manualEquipment) {
      if (!items.includes(id)) {
        items.push(id);
      }
    }
    return items;
  });

  // Filtered equipment for search
  let filteredEquipment = $derived(() => {
    if (!equipmentSearch) return allEquipment;
    const q = equipmentSearch.toLowerCase();
    return allEquipment.filter(e => e.name.toLowerCase().includes(q));
  });

  function toggleSpell(spellId) {
    if (selectedSpells.includes(spellId)) {
      selectedSpells = selectedSpells.filter(s => s !== spellId);
    } else {
      selectedSpells = [...selectedSpells, spellId];
    }
  }

  // Derived stats for review
  let selectedRaceData = $derived(races.find(r => r.id === race));
  let selectedClassData = $derived(classes.find(c => c.id === selectedClass));

  let racialBonuses = $derived(() => {
    if (!selectedRaceData?.ability_bonuses) return {};
    try {
      return typeof selectedRaceData.ability_bonuses === 'string'
        ? JSON.parse(selectedRaceData.ability_bonuses)
        : selectedRaceData.ability_bonuses;
    } catch { return {}; }
  });

  let finalScores = $derived(() => {
    const bonuses = racialBonuses();
    return {
      str: scores.str + (bonuses.str || 0),
      dex: scores.dex + (bonuses.dex || 0),
      con: scores.con + (bonuses.con || 0),
      int: scores.int + (bonuses.int || 0),
      wis: scores.wis + (bonuses.wis || 0),
      cha: scores.cha + (bonuses.cha || 0),
    };
  });

  function hitDieValue(hitDie) {
    const m = hitDie?.match(/d(\d+)/);
    return m ? parseInt(m[1]) : 8;
  }

  let derivedHP = $derived(() => {
    const fs = finalScores();
    const conMod = abilityModifier(fs.con);
    const hd = hitDieValue(selectedClassData?.hit_die);
    return hd + conMod;
  });

  let derivedAC = $derived(() => {
    const fs = finalScores();
    return 10 + abilityModifier(fs.dex);
  });

  let derivedSpeed = $derived(() => selectedRaceData?.speed_ft || 30);

  async function handleSubmit() {
    submitting = true;
    error = '';
    try {
      await submitCharacter({
        token,
        campaign_id: campaignId,
        name,
        race,
        subrace,
        background,
        class: selectedClass,
        subclass,
        ability_scores: scores,
        skills: selectedSkills,
        equipment: selectedEquipment(),
        spells: selectedSpells,
      });
      submitted = true;
    } catch (e) {
      error = 'Submission failed: ' + e.message;
    } finally {
      submitting = false;
    }
  }

  const ABILITIES = ['str', 'dex', 'con', 'int', 'wis', 'cha'];
  const ABILITY_NAMES = { str: 'Strength', dex: 'Dexterity', con: 'Constitution', int: 'Intelligence', wis: 'Wisdom', cha: 'Charisma' };

  const BACKGROUNDS = ['acolyte', 'charlatan', 'criminal', 'entertainer', 'folk-hero', 'guild-artisan', 'hermit', 'noble', 'outlander', 'sage', 'sailor', 'soldier', 'urchin'];

  const ALL_SKILLS = [
    'acrobatics', 'animal-handling', 'arcana', 'athletics', 'deception',
    'history', 'insight', 'intimidation', 'investigation', 'medicine',
    'nature', 'perception', 'performance', 'persuasion', 'religion',
    'sleight-of-hand', 'stealth', 'survival'
  ];
</script>

<div class="builder">
  {#if submitted}
    <div class="success">
      <h3>Character Submitted!</h3>
      <p>Your character <strong>{name}</strong> has been submitted for DM approval. You'll be notified when it's reviewed.</p>
    </div>
  {:else if loading}
    <p>Loading reference data...</p>
  {:else}
    <!-- Step navigation -->
    <nav class="steps">
      {#each STEPS as step, i}
        <button
          class="step-btn"
          class:active={i === currentStep}
          class:completed={i < currentStep}
          onclick={() => goToStep(i)}
        >
          {i + 1}. {step}
        </button>
      {/each}
    </nav>

    {#if error}
      <div class="error">{error}</div>
    {/if}

    <!-- Step 0: Basics -->
    {#if currentStep === 0}
      <div class="step-content">
        <h3>Character Basics</h3>
        <label>
          Name
          <input type="text" bind:value={name} placeholder="Character name" />
        </label>
        <label>
          Race
          <select bind:value={race}>
            <option value="">Select a race...</option>
            {#each races as r}
              <option value={r.id}>{r.name}</option>
            {/each}
          </select>
        </label>
        <label>
          Background
          <select bind:value={background}>
            <option value="">Select a background...</option>
            {#each BACKGROUNDS as bg}
              <option value={bg}>{bg.replace(/-/g, ' ')}</option>
            {/each}
          </select>
        </label>
      </div>

    <!-- Step 1: Class -->
    {:else if currentStep === 1}
      <div class="step-content">
        <h3>Choose Your Class</h3>
        <label>
          Class
          <select bind:value={selectedClass}>
            <option value="">Select a class...</option>
            {#each classes as c}
              <option value={c.id}>{c.name} (Hit Die: {c.hit_die})</option>
            {/each}
          </select>
        </label>
        {#if selectedClassData}
          <div class="class-info">
            <p><strong>Hit Die:</strong> {selectedClassData.hit_die}</p>
            {#if selectedClassData.save_proficiencies}
              <p><strong>Save Proficiencies:</strong> {selectedClassData.save_proficiencies.join(', ').toUpperCase()}</p>
            {/if}
          </div>
        {/if}
      </div>

    <!-- Step 2: Ability Scores -->
    {:else if currentStep === 2}
      <div class="step-content">
        <h3>Ability Scores (Point Buy)</h3>
        <p class="points-remaining">
          Points Remaining: <strong class:overspent={remainingPoints(scores) < 0}>{remainingPoints(scores)}</strong> / 27
        </p>
        <div class="ability-grid">
          {#each ABILITIES as ability}
            <div class="ability-row">
              <span class="ability-name">{ABILITY_NAMES[ability]}</span>
              <button class="score-btn" onclick={() => decrement(ability)} disabled={!canDecrement(scores, ability)}>-</button>
              <span class="score-value">{scores[ability]}</span>
              <button class="score-btn" onclick={() => increment(ability)} disabled={!canIncrement(scores, ability)}>+</button>
              <span class="score-mod">({abilityModifier(scores[ability]) >= 0 ? '+' : ''}{abilityModifier(scores[ability])})</span>
              <span class="score-cost">{scoreCost(scores[ability])} pts</span>
            </div>
          {/each}
        </div>
      </div>

    <!-- Step 3: Skills -->
    {:else if currentStep === 3}
      <div class="step-content">
        <h3>Skills & Proficiencies</h3>
        <p>Select your skill proficiencies:</p>
        <div class="skill-grid">
          {#each ALL_SKILLS as skill}
            <label class="skill-option">
              <input type="checkbox" checked={selectedSkills.includes(skill)} onchange={() => toggleSkill(skill)} />
              {skill.replace(/-/g, ' ')}
            </label>
          {/each}
        </div>
      </div>

    <!-- Step 4: Equipment -->
    {:else if currentStep === 4}
      <div class="step-content">
        <h3>Starting Equipment</h3>
        {#if selectedClassData}
          <p><strong>Class:</strong> {selectedClassData.name}</p>
        {/if}
        {#if background}
          <p><strong>Background:</strong> {background.replace(/-/g, ' ')}</p>
        {/if}

        <!-- Starting equipment packs -->
        {#if startingPacks.length > 0 && startingPacks[0]}
          <div class="equipment-section">
            <h4>Starting Equipment Choices</h4>
            {#if startingPacks[0].guaranteed && startingPacks[0].guaranteed.length > 0}
              <div class="guaranteed-items">
                <p><strong>Guaranteed items:</strong></p>
                <ul>
                  {#each startingPacks[0].guaranteed as item}
                    <li>{item.replace(/-/g, ' ')}</li>
                  {/each}
                </ul>
              </div>
            {/if}

            {#if startingPacks[0].choices}
              {#each startingPacks[0].choices as choice, choiceIdx}
                <div class="equipment-choice">
                  <p><strong>{choice.label}:</strong></p>
                  {#each choice.options as option, optIdx}
                    <label class="equipment-option">
                      <input
                        type="radio"
                        name="equip-choice-{choiceIdx}"
                        checked={packChoices[choiceIdx] === optIdx}
                        onchange={() => selectPackChoice(choiceIdx, optIdx)}
                      />
                      {option.replace(/-/g, ' ').replace(/:/g, ' x').replace(/,/g, ', ')}
                    </label>
                  {/each}
                </div>
              {/each}
            {/if}
          </div>
        {/if}

        <!-- Manual equipment selection -->
        <div class="equipment-section">
          <h4>Additional Equipment (SRD Items)</h4>
          <input
            type="text"
            bind:value={equipmentSearch}
            placeholder="Search weapons and armor..."
          />
          {#if filteredEquipment().length > 0}
            <div class="equipment-list">
              {#each filteredEquipment().slice(0, 20) as item}
                <div class="equipment-item">
                  <span class="item-name">{item.name}</span>
                  <span class="item-type">{item.category}</span>
                  {#if item.damage}
                    <span class="item-detail">{item.damage} {item.damage_type}</span>
                  {/if}
                  {#if item.ac_base}
                    <span class="item-detail">AC {item.ac_base}</span>
                  {/if}
                  {#if manualEquipment.includes(item.id)}
                    <button class="remove-btn" onclick={() => removeManualItem(item.id)}>Remove</button>
                  {:else}
                    <button class="add-btn" onclick={() => addManualItem(item.id)}>Add</button>
                  {/if}
                </div>
              {/each}
              {#if filteredEquipment().length > 20}
                <p class="truncated">Showing first 20 of {filteredEquipment().length} items. Refine your search.</p>
              {/if}
            </div>
          {/if}
        </div>

        <!-- Selected equipment summary -->
        {#if selectedEquipment().length > 0}
          <div class="equipment-section">
            <h4>Selected Equipment ({selectedEquipment().length} items)</h4>
            <ul class="selected-list">
              {#each selectedEquipment() as itemId}
                <li>{itemId.replace(/-/g, ' ')}</li>
              {/each}
            </ul>
          </div>
        {/if}
      </div>

    <!-- Step 5: Spells -->
    {:else if currentStep === 5}
      <div class="step-content">
        <h3>Spells</h3>
        {#if spells.length === 0}
          <p>No spells available for your class, or your class is not a spellcaster.</p>
        {:else}
          <p>Select your known spells:</p>
          <div class="spell-grid">
            {#each spells as spell}
              <label class="spell-option">
                <input type="checkbox" checked={selectedSpells.includes(spell.id)} onchange={() => toggleSpell(spell.id)} />
                <span class="spell-name">{spell.name}</span>
                <span class="spell-level">Lvl {spell.level}</span>
                <span class="spell-school">{spell.school}</span>
              </label>
            {/each}
          </div>
        {/if}
      </div>

    <!-- Step 6: Review -->
    {:else if currentStep === 6}
      <div class="step-content">
        <h3>Review & Submit</h3>
        <div class="review-section">
          <h4>Basics</h4>
          <p><strong>Name:</strong> {name || '(not set)'}</p>
          <p><strong>Race:</strong> {selectedRaceData?.name || race || '(not set)'}</p>
          <p><strong>Background:</strong> {background ? background.replace(/-/g, ' ') : '(not set)'}</p>
          <p><strong>Class:</strong> {selectedClassData?.name || selectedClass || '(not set)'}</p>
        </div>

        <div class="review-section">
          <h4>Ability Scores</h4>
          <div class="review-scores">
            {#each ABILITIES as ability}
              <div class="review-score">
                <span class="ability-label">{ability.toUpperCase()}</span>
                <span class="ability-value">{finalScores()[ability]}</span>
                <span class="ability-mod">({abilityModifier(finalScores()[ability]) >= 0 ? '+' : ''}{abilityModifier(finalScores()[ability])})</span>
              </div>
            {/each}
          </div>
        </div>

        <div class="review-section">
          <h4>Derived Stats</h4>
          <p><strong>HP:</strong> {derivedHP()}</p>
          <p><strong>AC:</strong> {derivedAC()}</p>
          <p><strong>Speed:</strong> {derivedSpeed()} ft</p>
          <p><strong>Proficiency Bonus:</strong> +2</p>
        </div>

        {#if selectedSkills.length > 0}
          <div class="review-section">
            <h4>Skills</h4>
            <p>{selectedSkills.map(s => s.replace(/-/g, ' ')).join(', ')}</p>
          </div>
        {/if}

        {#if selectedEquipment().length > 0}
          <div class="review-section">
            <h4>Equipment</h4>
            <p>{selectedEquipment().map(id => id.replace(/-/g, ' ')).join(', ')}</p>
          </div>
        {/if}

        {#if selectedSpells.length > 0}
          <div class="review-section">
            <h4>Spells</h4>
            <p>{selectedSpells.join(', ')}</p>
          </div>
        {/if}

        <button class="submit-btn" onclick={handleSubmit} disabled={submitting || !name || !race || !selectedClass}>
          {submitting ? 'Submitting...' : 'Submit for DM Approval'}
        </button>
      </div>
    {/if}

    <!-- Navigation buttons -->
    <div class="nav-buttons">
      {#if currentStep > 0}
        <button class="nav-btn" onclick={prevStep}>Previous</button>
      {/if}
      {#if currentStep < STEPS.length - 1}
        <button class="nav-btn primary" onclick={nextStep}>Next</button>
      {/if}
    </div>
  {/if}
</div>

<style>
  .builder { font-family: system-ui, sans-serif; }
  .steps { display: flex; flex-wrap: wrap; gap: 0.25rem; margin-bottom: 1.5rem; }
  .step-btn {
    padding: 0.4rem 0.8rem; border: 1px solid #0f3460; background: #16213e; color: #e0e0e0;
    cursor: pointer; border-radius: 4px; font-size: 0.85rem;
  }
  .step-btn.active { background: #e94560; color: white; border-color: #e94560; }
  .step-btn.completed { background: #0f3460; }
  .step-content { margin-bottom: 1.5rem; }
  .step-content h3 { color: #e94560; margin-bottom: 1rem; }
  label { display: block; margin-bottom: 0.75rem; }
  input[type="text"], select {
    display: block; width: 100%; padding: 0.5rem; margin-top: 0.25rem;
    background: #1a1a2e; color: #e0e0e0; border: 1px solid #0f3460; border-radius: 4px;
  }
  .points-remaining { font-size: 1.1rem; margin-bottom: 1rem; }
  .overspent { color: #ff4444; }
  .ability-grid { display: flex; flex-direction: column; gap: 0.5rem; }
  .ability-row { display: flex; align-items: center; gap: 0.75rem; }
  .ability-name { width: 120px; }
  .score-btn {
    width: 32px; height: 32px; background: #0f3460; color: white; border: none;
    border-radius: 4px; cursor: pointer; font-size: 1.1rem;
  }
  .score-btn:disabled { opacity: 0.3; cursor: not-allowed; }
  .score-value { width: 30px; text-align: center; font-weight: bold; font-size: 1.1rem; }
  .score-mod { color: #aaa; width: 40px; }
  .score-cost { color: #888; font-size: 0.85rem; }
  .skill-grid, .spell-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 0.25rem; }
  .skill-option, .spell-option { display: flex; align-items: center; gap: 0.5rem; padding: 0.25rem 0; }
  .spell-level, .spell-school { color: #888; font-size: 0.85rem; }
  .class-info { margin-top: 1rem; padding: 1rem; background: #1a1a2e; border-radius: 4px; border: 1px solid #0f3460; }
  .review-section { margin-bottom: 1rem; padding: 1rem; background: #1a1a2e; border-radius: 4px; border: 1px solid #0f3460; }
  .review-section h4 { color: #e94560; margin-bottom: 0.5rem; }
  .review-scores { display: flex; gap: 1rem; flex-wrap: wrap; }
  .review-score { text-align: center; padding: 0.5rem; }
  .ability-label { display: block; font-weight: bold; font-size: 0.85rem; }
  .ability-value { display: block; font-size: 1.3rem; font-weight: bold; }
  .ability-mod { color: #aaa; }
  .nav-buttons { display: flex; gap: 0.75rem; margin-top: 1rem; }
  .nav-btn {
    padding: 0.6rem 1.5rem; border: 1px solid #0f3460; background: #16213e;
    color: #e0e0e0; cursor: pointer; border-radius: 4px;
  }
  .nav-btn.primary { background: #e94560; border-color: #e94560; }
  .submit-btn {
    margin-top: 1rem; padding: 0.75rem 2rem; background: #e94560; color: white;
    border: none; border-radius: 4px; cursor: pointer; font-size: 1rem;
  }
  .submit-btn:disabled { opacity: 0.5; cursor: not-allowed; }
  .success { padding: 2rem; text-align: center; }
  .success h3 { color: #4caf50; }
  .error { background: #441111; border: 1px solid #ff4444; padding: 0.75rem; border-radius: 4px; margin-bottom: 1rem; color: #ff8888; }
  .equipment-section { margin-top: 1rem; padding: 1rem; background: #1a1a2e; border-radius: 4px; border: 1px solid #0f3460; }
  .equipment-section h4 { color: #e94560; margin-bottom: 0.5rem; }
  .equipment-choice { margin-bottom: 0.75rem; }
  .equipment-option { display: flex; align-items: center; gap: 0.5rem; padding: 0.2rem 0; cursor: pointer; }
  .guaranteed-items ul { margin: 0.25rem 0; padding-left: 1.5rem; }
  .guaranteed-items li { text-transform: capitalize; }
  .equipment-list { max-height: 300px; overflow-y: auto; margin-top: 0.5rem; }
  .equipment-item {
    display: flex; align-items: center; gap: 0.75rem; padding: 0.4rem 0.5rem;
    border-bottom: 1px solid #0f3460;
  }
  .item-name { flex: 1; }
  .item-type { color: #888; font-size: 0.85rem; text-transform: capitalize; }
  .item-detail { color: #aaa; font-size: 0.85rem; }
  .add-btn, .remove-btn {
    padding: 0.2rem 0.6rem; border: none; border-radius: 3px; cursor: pointer; font-size: 0.8rem;
  }
  .add-btn { background: #0f3460; color: #e0e0e0; }
  .remove-btn { background: #e94560; color: white; }
  .selected-list { margin: 0.25rem 0; padding-left: 1.5rem; }
  .selected-list li { text-transform: capitalize; }
  .truncated { color: #888; font-size: 0.85rem; font-style: italic; }
</style>
