<script>
  import { listRaces, listClasses, listSpells, listEquipment, getStartingEquipment, listAbilityMethods, submitCharacter } from './lib/api.js';
  import { remainingPoints, abilityModifier, canIncrement, canDecrement, scoreCost } from './lib/pointbuy.js';
  import { skillsForBackground, mergeBackgroundSkills, backgroundDetails, formatLanguages } from './lib/backgrounds.js';
  import { abilityLabel } from './lib/skills.js';
  import { formatAbilityBonuses, parseTraits, formatDarkvision, subracePerks } from './lib/race-perks.js';
  import { isConcentration, formatCastingTime } from './lib/spell-perks.js';
  import { filterSpells, groupSpellsBySchool, availableLevels } from './lib/spell-filter.js';
  import { formatProperties, armorACText } from './lib/equipment-perks.js';
  import { raceGrantedSkills, mergeGrantedSkills } from './lib/race-skills.js';
  import { raceGrantedWeaponProficiencies, weaponProficiencyLabel } from './lib/race-weapon-proficiencies.js';
  import { formatSkillChoices } from './lib/class-perks.js';
  import {
    subraceOptions, subclassOptions, isSubclassEligible,
    emptyClassRow, addClassRow, removeClassRow, updateClassRow,
  } from './lib/builder-options.js';
  import { draftKey, serializeDraft, parseDraft } from './lib/builder-draft.js';

  let { token = '', campaignId = '' } = $props();

  // Steps
  const STEPS = ['Basics', 'Class', 'Ability Scores', 'Skills', 'Equipment', 'Spells', 'Review'];
  let currentStep = $state(0);

  // Form state preserved across steps
  let name = $state('');
  let race = $state('');
  let subrace = $state('');
  let background = $state('');
  // Multi-class entries — the first row drives the primary class for spell
  // list / starting equipment loading. selectedClass / subclass are kept as
  // derived mirrors of classEntries[0] for compatibility with the existing
  // single-class UI code paths.
  let classEntries = $state([emptyClassRow()]);
  let scores = $state({ str: 8, dex: 8, con: 8, int: 8, wis: 8, cha: 8 });
  let abilityMethod = $state('point_buy');
  let abilityRolls = $state({});
  let selectedSkills = $state([]);
  let selectedSpells = $state([]);

  // Reference data
  let races = $state([]);
  let classes = $state([]);
  let spells = $state([]);
  let spellQuery = $state('');
  let spellLevelFilter = $state('');
  let allEquipment = $state([]);
  let startingPacks = $state([]);
  let abilityMethods = $state(['point_buy', 'standard_array', 'roll']);

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
      const [r, c, methods] = await Promise.all([listRaces(), listClasses(), listAbilityMethods(campaignId)]);
      races = r;
      classes = c;
      abilityMethods = methods.length > 0 ? methods : ['point_buy', 'standard_array', 'roll'];
      if (!abilityMethods.includes(abilityMethod)) {
        setAbilityMethod(abilityMethods[0]);
      }
    } catch (e) {
      error = 'Failed to load reference data: ' + e.message;
    } finally {
      loading = false;
    }
  }

  // The primary class drives spell/equipment loading and HP. We mirror
  // classEntries[0].class for compatibility with the legacy single-class
  // pickers + downstream review code.
  let selectedClass = $derived(classEntries[0]?.class || '');
  let subclass = $derived(classEntries[0]?.subclass || '');

  // Load spells and starting equipment when the primary class changes.
  // Reset pack choices only when the user switches to a *different* class —
  // never on the initial restore of a saved draft. lastClassForPacks starts
  // '' so a fresh first selection doesn't wipe anything either.
  let lastClassForPacks = '';
  $effect(() => {
    if (!selectedClass) return;
    loadSpells(selectedClass);
    loadStartingEquipment(selectedClass);
    if (selectedClass !== lastClassForPacks) {
      if (lastClassForPacks !== '') packChoices = {};
      lastClassForPacks = selectedClass;
    }
  });

  // Auto-add background skills whenever the user changes background. We
  // only ever add (never remove) so manual deselection still works after.
  let lastBackground = '';
  $effect(() => {
    if (background && background !== lastBackground) {
      selectedSkills = mergeBackgroundSkills(selectedSkills, background);
      lastBackground = background;
    }
  });

  // Auto-add race-granted skill proficiencies whenever the race changes,
  // mirroring the background merge above. Add-only so manual deselect works.
  let lastRaceForSkills = '';
  $effect(() => {
    if (race && race !== lastRaceForSkills) {
      const granted = raceGrantedSkills(selectedRaceData?.traits);
      if (granted.length > 0) {
        selectedSkills = mergeGrantedSkills(selectedSkills, granted);
      }
      lastRaceForSkills = race;
    }
  });

  // --- Draft persistence (localStorage) --------------------------------
  // Survive an accidental reload: restore unsubmitted fields on init,
  // re-save on every change, and clear once the character is submitted.
  function readDraftRaw() {
    if (typeof localStorage === 'undefined') return null;
    try {
      return localStorage.getItem(draftKey(token));
    } catch {
      return null;
    }
  }

  function writeDraftRaw(raw) {
    if (typeof localStorage === 'undefined') return;
    try {
      localStorage.setItem(draftKey(token), raw);
    } catch {
      /* quota exceeded or storage disabled — skip silently */
    }
  }

  function clearDraft() {
    if (typeof localStorage === 'undefined') return;
    try {
      localStorage.removeItem(draftKey(token));
    } catch {
      /* ignore */
    }
  }

  function applyDraft(d) {
    if (d.currentStep !== undefined) currentStep = d.currentStep;
    if (d.name !== undefined) name = d.name;
    if (d.race !== undefined) race = d.race;
    if (d.subrace !== undefined) subrace = d.subrace;
    if (d.background !== undefined) background = d.background;
    if (Array.isArray(d.classEntries) && d.classEntries.length > 0) classEntries = d.classEntries;
    if (d.scores !== undefined) scores = d.scores;
    if (d.abilityMethod !== undefined) abilityMethod = d.abilityMethod;
    if (d.abilityRolls !== undefined) abilityRolls = d.abilityRolls;
    if (Array.isArray(d.selectedSkills)) selectedSkills = d.selectedSkills;
    if (Array.isArray(d.selectedSpells)) selectedSpells = d.selectedSpells;
    if (d.packChoices !== undefined) packChoices = d.packChoices;
    if (Array.isArray(d.manualEquipment)) manualEquipment = d.manualEquipment;
    // Prime the merge/reset guards to the restored values so the auto-merge
    // effects don't re-add deselected skills and the class effect doesn't wipe
    // restored pack choices.
    lastBackground = d.background || '';
    lastRaceForSkills = d.race || '';
    lastClassForPacks = d.classEntries?.[0]?.class || '';
  }

  // Restore once, synchronously during init (before any effect runs).
  const restoredDraft = parseDraft(readDraftRaw());
  if (restoredDraft) applyDraft(restoredDraft);

  // Persist on every change to a tracked field; never write after submit.
  $effect(() => {
    const snapshot = $state.snapshot({
      currentStep, name, race, subrace, background,
      classEntries, scores, abilityMethod, abilityRolls,
      selectedSkills, selectedSpells, packChoices, manualEquipment,
    });
    if (submitted) return;
    writeDraftRaw(serializeDraft(snapshot));
  });

  // Load the full equipment list up front: the Equipment step needs it, and
  // the Basics-step race panel needs weapon ids to label race weapon proficiencies.
  $effect(() => {
    if (allEquipment.length === 0) {
      loadEquipment();
    }
  });

  async function loadSpells(cls) {
    try {
      spells = await listSpells(cls, campaignId);
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

  function setAbilityMethod(method) {
    abilityMethod = method;
    abilityRolls = {};
    if (method === 'standard_array') {
      scores = { str: 15, dex: 14, con: 13, int: 12, wis: 10, cha: 8 };
      return;
    }
    if (method === 'point_buy') {
      scores = { str: 8, dex: 8, con: 8, int: 8, wis: 8, cha: 8 };
    }
  }

  function rollAbilityScores() {
    const nextScores = {};
    const nextRolls = {};
    for (const ability of ABILITIES) {
      const dice = Array.from({ length: 4 }, () => Math.floor(Math.random() * 6) + 1);
      const sorted = [...dice].sort((a, b) => a - b);
      nextScores[ability] = sorted[1] + sorted[2] + sorted[3];
      nextRolls[ability] = dice;
    }
    scores = nextScores;
    abilityRolls = nextRolls;
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
  let weaponIds = $derived(allEquipment.filter(e => e.category === 'weapon').map(e => e.id));

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
      // Re-merge background skills at submit time as a safety net in case
      // the user toggled them off after picking a background.
      let skills = mergeBackgroundSkills(selectedSkills, background);
      // Same safety net for race-granted skill proficiencies.
      skills = mergeGrantedSkills(skills, raceGrantedSkills(selectedRaceData?.traits));
      // Filter out incomplete class rows so the backend never sees a blank
      // class entry.
      const classes = classEntries
        .filter(c => c.class)
        .map(c => ({ class: c.class, level: Number(c.level) || 1, subclass: c.subclass || '' }));
      await submitCharacter({
        token,
        campaign_id: campaignId,
        name,
        race,
        subrace,
        background,
        class: selectedClass,
        subclass,
        classes,
        ability_scores: finalScores(),
        ability_method: abilityMethod,
        ability_rolls: abilityRolls,
        skills,
        equipment: selectedEquipment(),
        spells: selectedSpells,
      });
      submitted = true;
      clearDraft();
    } catch (e) {
      error = 'Submission failed: ' + e.message;
    } finally {
      submitting = false;
    }
  }

  const ABILITIES = ['str', 'dex', 'con', 'int', 'wis', 'cha'];
  const ABILITY_NAMES = { str: 'Strength', dex: 'Dexterity', con: 'Constitution', int: 'Intelligence', wis: 'Wisdom', cha: 'Charisma' };

  // Derived helpers for the basics/class steps.
  let subraceList = $derived(subraceOptions(selectedRaceData));

  // Reset subrace when race changes if the current value is no longer valid.
  // Skip while race data is still loading, otherwise a restored subrace gets
  // wiped before its parent race's subrace options exist.
  $effect(() => {
    if (races.length === 0) return;
    const opts = subraceList;
    if (subrace && !opts.some(o => o.id === subrace)) {
      subrace = '';
    }
  });

  function setClassRow(idx, patch) {
    classEntries = updateClassRow(classEntries, idx, patch);
  }

  function appendClassRow() {
    classEntries = addClassRow(classEntries);
  }

  function dropClassRow(idx) {
    classEntries = removeClassRow(classEntries, idx);
  }

  function classOptionsFor(idx) {
    // Stop a class from being selected twice across rows.
    const taken = new Set(classEntries.filter((_, i) => i !== idx).map(c => c.class).filter(Boolean));
    return classes.filter(c => !taken.has(c.id));
  }

  function subclassListFor(classId) {
    const cls = classes.find(c => c.id === classId);
    return subclassOptions(cls);
  }

  function isSubclassPickerVisible(row) {
    const cls = classes.find(c => c.id === row.class);
    return isSubclassEligible(cls, Number(row.level) || 0);
  }

  // PHB backgrounds — slugs match keys of BACKGROUND_SKILLS in lib/backgrounds.js.
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
        {#if subraceList.length > 0}
          <label>
            Subrace
            <select bind:value={subrace}>
              <option value="">Select a subrace...</option>
              {#each subraceList as sr}
                <option value={sr.id}>{sr.name}</option>
              {/each}
            </select>
          </label>
        {/if}
        {#snippet traitList(traits)}
          <ul class="trait-list">
            {#each traits as trait}
              <li class="trait-item"><strong>{trait.name}</strong> — {trait.description}</li>
            {/each}
          </ul>
        {/snippet}
        {#if selectedRaceData}
          {@const raceBonuses = formatAbilityBonuses(selectedRaceData.ability_bonuses)}
          {@const raceTraits = parseTraits(selectedRaceData.traits)}
          {@const darkvision = formatDarkvision(selectedRaceData.darkvision_ft)}
          {@const raceWeapons = raceGrantedWeaponProficiencies(selectedRaceData?.traits, weaponIds)}
          <div class="race-info">
            {#if raceBonuses}
              <p><strong>Ability Score Increase:</strong> {raceBonuses}</p>
            {/if}
            <p><strong>Speed:</strong> {selectedRaceData.speed_ft} ft</p>
            <p><strong>Size:</strong> {selectedRaceData.size}</p>
            {#if darkvision}
              <p><strong>Darkvision:</strong> {darkvision}</p>
            {/if}
            {#if selectedRaceData.languages?.length > 0}
              <p><strong>Languages:</strong> {selectedRaceData.languages.join(', ')}</p>
            {/if}
            {#if raceWeapons.length > 0}
              <p class="race-weapons"><strong>Weapon Training:</strong> {raceWeapons.map(weaponProficiencyLabel).join(', ')}</p>
            {/if}
            {#if raceTraits.length > 0}
              <p><strong>Traits:</strong></p>
              {@render traitList(raceTraits)}
            {/if}
            {#if subrace}
              {@const sp = subracePerks(selectedRaceData, subrace)}
              {#if sp}
                {@const subraceBonuses = formatAbilityBonuses(sp.abilityBonuses)}
                {@const subraceTraits = parseTraits(sp.traits)}
                <div class="subrace-info">
                  <p><strong>Subrace: {subraceList.find(s => s.id === subrace)?.name || subrace}</strong></p>
                  {#if subraceBonuses}
                    <p><strong>Ability Score Increase:</strong> {subraceBonuses}</p>
                  {/if}
                  {#if subraceTraits.length > 0}
                    {@render traitList(subraceTraits)}
                  {/if}
                </div>
              {/if}
            {/if}
          </div>
        {/if}
        <label>
          Background
          <select bind:value={background}>
            <option value="">Select a background...</option>
            {#each BACKGROUNDS as bg}
              <option value={bg}>{bg.replace(/-/g, ' ')}</option>
            {/each}
          </select>
        </label>
        {#if backgroundDetails(background)}
          {@const bd = backgroundDetails(background)}
          <div class="bg-info">
            <p>
              <strong>Skills:</strong>
              {#each bd.skills as sk}
                <span class="bg-skill-tag">{sk.replace(/-/g, ' ')}</span>
              {/each}
            </p>
            {#if bd.tools.length > 0}
              <p><strong>Tools:</strong> {bd.tools.join(', ')}</p>
            {/if}
            {#if formatLanguages(bd.languages)}
              <p><strong>Languages:</strong> {formatLanguages(bd.languages)}</p>
            {/if}
            <p class="bg-feature"><strong>{bd.feature.name}</strong> — {bd.feature.description}</p>
          </div>
        {/if}
      </div>

    <!-- Step 1: Class (with multiclass support) -->
    {:else if currentStep === 1}
      <div class="step-content">
        <h3>Choose Your Class</h3>
        {#each classEntries as row, idx (idx)}
          <div class="class-row">
            <label class="class-row-field">
              Class
              <select
                value={row.class}
                onchange={(e) => setClassRow(idx, { class: e.currentTarget.value, subclass: '' })}
              >
                <option value="">Select a class...</option>
                {#each classOptionsFor(idx) as c}
                  <option value={c.id}>{c.name} (Hit Die: {c.hit_die})</option>
                {/each}
              </select>
            </label>
            <label class="class-row-field class-row-level">
              Level
              <input
                type="number"
                min="1"
                max="20"
                value={row.level}
                oninput={(e) => setClassRow(idx, { level: Math.max(1, Math.min(20, Number(e.currentTarget.value) || 1)) })}
              />
            </label>
            {#if isSubclassPickerVisible(row)}
              <label class="class-row-field">
                Subclass
                <select
                  value={row.subclass}
                  onchange={(e) => setClassRow(idx, { subclass: e.currentTarget.value })}
                >
                  <option value="">Select a subclass...</option>
                  {#each subclassListFor(row.class) as sc}
                    <option value={sc.id}>{sc.name}</option>
                  {/each}
                </select>
              </label>
            {/if}
            {#if idx > 0}
              <button type="button" class="row-remove-btn" onclick={() => dropClassRow(idx)} aria-label="Remove class">x</button>
            {/if}
          </div>
        {/each}
        <button type="button" class="row-add-btn" onclick={appendClassRow} disabled={classEntries.length >= 4}>
          + Add another class
        </button>
        {#if selectedClassData}
          <div class="class-info">
            <p><strong>Primary Class Hit Die:</strong> {selectedClassData.hit_die}</p>
            {#if selectedClassData.primary_ability}
              <p><strong>Primary Ability:</strong> {selectedClassData.primary_ability.toUpperCase()}</p>
            {/if}
            {#if selectedClassData.save_proficiencies}
              <p><strong>Save Proficiencies:</strong> {selectedClassData.save_proficiencies.join(', ').toUpperCase()}</p>
            {/if}
            {#if selectedClassData.armor_proficiencies?.length > 0}
              <p><strong>Armor:</strong> {selectedClassData.armor_proficiencies.join(', ')}</p>
            {/if}
            {#if selectedClassData.weapon_proficiencies?.length > 0}
              <p><strong>Weapons:</strong> {selectedClassData.weapon_proficiencies.join(', ')}</p>
            {/if}
            {#if formatSkillChoices(selectedClassData.skill_choices)}
              <p><strong>Skills:</strong> {formatSkillChoices(selectedClassData.skill_choices)}</p>
            {/if}
          </div>
        {/if}
      </div>

    <!-- Step 2: Ability Scores -->
    {:else if currentStep === 2}
      <div class="step-content">
        <h3>Ability Scores</h3>
        <div class="method-tabs">
          {#each abilityMethods as method}
            <button
              type="button"
              class:active={abilityMethod === method}
              onclick={() => setAbilityMethod(method)}
            >
              {method === 'point_buy' ? 'Point Buy' : method === 'standard_array' ? 'Standard Array' : 'Roll'}
            </button>
          {/each}
        </div>
        {#if abilityMethod === 'point_buy'}
          <p class="points-remaining">
            Points Remaining: <strong class:overspent={remainingPoints(scores) < 0}>{remainingPoints(scores)}</strong> / 27
          </p>
        {:else if abilityMethod === 'roll'}
          <button type="button" class="roll-btn" onclick={rollAbilityScores}>Roll 4d6</button>
        {/if}
        <div class="ability-grid">
          {#each ABILITIES as ability}
            {@const raceBonus = racialBonuses()[ability]}
            <div class="ability-row">
              <span class="ability-name">{ABILITY_NAMES[ability]}</span>
              {#if abilityMethod === 'point_buy'}
                <button class="score-btn" onclick={() => decrement(ability)} disabled={!canDecrement(scores, ability)}>-</button>
              {/if}
              <span class="score-value">{scores[ability]}</span>
              {#if abilityMethod === 'point_buy'}
                <button class="score-btn" onclick={() => increment(ability)} disabled={!canIncrement(scores, ability)}>+</button>
              {/if}
              <span class="score-mod">({abilityModifier(scores[ability]) >= 0 ? '+' : ''}{abilityModifier(scores[ability])})</span>
              {#if abilityMethod === 'point_buy'}
                <span class="score-cost">{scoreCost(scores[ability])} pts</span>
              {:else if abilityMethod === 'roll' && abilityRolls[ability]}
                <span class="score-cost">{abilityRolls[ability].join(', ')}</span>
              {/if}
              {#if typeof raceBonus === 'number' && raceBonus > 0}
                <span class="race-bonus">+{raceBonus} race → {scores[ability] + raceBonus}</span>
              {/if}
            </div>
          {/each}
        </div>
      </div>

    <!-- Step 3: Skills -->
    {:else if currentStep === 3}
      {@const raceGranted = raceGrantedSkills(selectedRaceData?.traits)}
      {@const bgSkills = skillsForBackground(background)}
      <div class="step-content">
        <h3>Skills & Proficiencies</h3>
        <p>Select your skill proficiencies:</p>
        {#if background && bgSkills.length > 0}
          <p class="bg-skill-hint">
            From <strong>{background.replace(/-/g, ' ')}</strong> background:
            {#each bgSkills as sk}
              <span class="bg-skill-tag">{sk.replace(/-/g, ' ')}</span>
            {/each}
          </p>
        {/if}
        {#if selectedRaceData && raceGranted.length > 0}
          <p class="bg-skill-hint">
            From <strong>{selectedRaceData.name}</strong> race:
            {#each raceGranted as sk}
              <span class="bg-skill-tag">{sk.replace(/-/g, ' ')}</span>
            {/each}
          </p>
        {/if}
        <div class="skill-grid">
          {#each ALL_SKILLS as skill}
            <label class="skill-option" class:bg-granted={bgSkills.includes(skill)}>
              <input type="checkbox" checked={selectedSkills.includes(skill)} onchange={() => toggleSkill(skill)} />
              {skill.replace(/-/g, ' ')}
              {#if abilityLabel(skill)}
                <span class="skill-ability">({abilityLabel(skill)})</span>
              {/if}
              {#if bgSkills.includes(skill)}
                <span class="bg-skill-tag-inline">background</span>
              {/if}
              {#if raceGranted.includes(skill)}
                <span class="bg-skill-tag-inline">race</span>
              {/if}
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
                  {#if item.properties?.length > 0}
                    <span class="item-detail">{formatProperties(item.properties)}</span>
                  {/if}
                  {#if item.mastery}
                    <span class="item-mastery">{weaponProficiencyLabel(item.mastery)}</span>
                  {/if}
                  {#if item.category === 'armor'}
                    <span class="item-detail">{armorACText(item.armor_type, item.ac_base)}</span>
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
          {@const filtered = filterSpells(spells, { query: spellQuery, level: spellLevelFilter })}
          {@const groups = groupSpellsBySchool(filtered)}
          {@const levels = availableLevels(spells)}
          <div class="spell-toolbar">
            <input class="spell-search" type="text" placeholder="Search spells…" bind:value={spellQuery} />
            <select class="spell-level-filter" bind:value={spellLevelFilter}>
              <option value="">All levels</option>
              {#each levels as lvl}
                <option value={lvl}>{lvl === 0 ? 'Cantrips' : `Level ${lvl}`}</option>
              {/each}
            </select>
            <span class="spell-selected-count">{selectedSpells.length} selected</span>
          </div>
          {#if filtered.length === 0}
            <p class="spell-empty">No spells match your search.</p>
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
                  {@const isSelected = selectedSpells.includes(spell.id)}
                  <label class="spell-option" class:selected={isSelected}>
                    <span class="spell-row">
                      <input type="checkbox" checked={isSelected} onchange={() => toggleSpell(spell.id)} />
                      <span class="spell-name">{spell.name}</span>
                      <span class="spell-level">{spell.level === 0 ? 'Cantrip' : `Lvl ${spell.level}`}</span>
                      {#if castingTime}
                        <span class="spell-meta">{castingTime}</span>
                      {/if}
                      {#if isConcentration(spell.duration)}
                        <span class="conc-tag">Concentration</span>
                      {/if}
                    </span>
                    {#if spell.description}
                      <span class="spell-desc" title={spell.description}>{spell.description}</span>
                    {/if}
                  </label>
                {/each}
              </div>
            </details>
          {/each}
        {/if}
      </div>

    <!-- Step 6: Review -->
    {:else if currentStep === 6}
      <div class="step-content">
        <h3>Review & Submit</h3>
        <div class="review-section">
          <h4>Basics</h4>
          <p><strong>Name:</strong> {name || '(not set)'}</p>
          <p><strong>Race:</strong> {selectedRaceData?.name || race || '(not set)'}{#if subrace} / {subraceList.find(s => s.id === subrace)?.name || subrace}{/if}</p>
          <p><strong>Background:</strong> {background ? background.replace(/-/g, ' ') : '(not set)'}</p>
          <p><strong>Classes:</strong>
            {#each classEntries.filter(c => c.class) as c, i}
              {#if i > 0} / {/if}
              {classes.find(x => x.id === c.class)?.name || c.class} {c.level}{#if c.subclass} ({subclassListFor(c.class).find(s => s.id === c.subclass)?.name || c.subclass}){/if}
            {:else}
              (not set)
            {/each}
          </p>
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
  .method-tabs { display: flex; gap: 0.5rem; flex-wrap: wrap; margin-bottom: 1rem; }
  .method-tabs button, .roll-btn {
    padding: 0.45rem 0.8rem; border: 1px solid #0f3460; background: #16213e;
    color: #e0e0e0; border-radius: 4px; cursor: pointer;
  }
  .method-tabs button.active { background: #e94560; border-color: #e94560; color: white; }
  .roll-btn { margin-bottom: 1rem; }
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
  .skill-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 0.25rem; }
  .skill-option { display: flex; align-items: center; gap: 0.5rem; padding: 0.25rem 0; }

  .spell-toolbar { position: sticky; top: 0; z-index: 2; display: flex; gap: 0.6rem; align-items: center; flex-wrap: wrap; padding: 0.5rem 0; margin-bottom: 0.5rem; background: #1a1a2e; }
  .spell-search { flex: 1 1 220px; padding: 0.45rem 0.6rem; background: #16213e; border: 1px solid #0f3460; border-radius: 6px; color: #eee; font-size: 0.9rem; }
  .spell-level-filter { padding: 0.45rem 0.5rem; background: #16213e; border: 1px solid #0f3460; border-radius: 6px; color: #eee; font-size: 0.9rem; }
  .spell-selected-count { margin-left: auto; color: #e94560; font-weight: 600; font-size: 0.9rem; }
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
  .spell-row { display: flex; align-items: center; gap: 0.5rem; flex-wrap: wrap; }
  .spell-name { font-weight: 600; }
  .spell-level { color: #888; font-size: 0.82rem; }
  .spell-meta { color: #888; font-size: 0.78rem; }
  .conc-tag {
    padding: 0.05rem 0.4rem; border: 1px solid #e94560; color: #e94560;
    border-radius: 8px; font-size: 0.7rem;
  }
  .spell-desc { display: -webkit-box; -webkit-line-clamp: 2; line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden; color: #aaa; font-size: 0.82rem; line-height: 1.3; margin-top: 0.1rem; }
  .class-info { margin-top: 1rem; padding: 1rem; background: #1a1a2e; border-radius: 4px; border: 1px solid #0f3460; }
  .race-info { margin-top: 1rem; padding: 1rem; background: #1a1a2e; border-radius: 4px; border: 1px solid #0f3460; }
  .trait-list { margin: 0.25rem 0 0.5rem; padding-left: 1.25rem; list-style: disc; }
  .trait-item { color: #aaa; font-size: 0.85rem; margin-bottom: 0.25rem; }
  .trait-item strong { color: #e0e0e0; }
  .subrace-info { margin-top: 0.75rem; padding-top: 0.75rem; border-top: 1px solid #16213e; }
  .race-bonus { color: #e94560; font-size: 0.8rem; }
  .skill-ability { color: #888; font-size: 0.8rem; margin-left: 0.3rem; }
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
  .item-mastery { display: inline-block; padding: 0.05rem 0.4rem; border: 1px solid #e94560; color: #e94560; border-radius: 8px; font-size: 0.7rem; margin-left: 0.4rem; }
  .race-weapons { color: #ccc; }
  .add-btn, .remove-btn {
    padding: 0.2rem 0.6rem; border: none; border-radius: 3px; cursor: pointer; font-size: 0.8rem;
  }
  .add-btn { background: #0f3460; color: #e0e0e0; }
  .remove-btn { background: #e94560; color: white; }
  .selected-list { margin: 0.25rem 0; padding-left: 1.5rem; }
  .selected-list li { text-transform: capitalize; }
  .truncated { color: #888; font-size: 0.85rem; font-style: italic; }
  .class-row {
    display: grid;
    grid-template-columns: 1fr 80px 1fr 32px;
    gap: 0.5rem;
    align-items: end;
    padding: 0.5rem;
    margin-bottom: 0.5rem;
    background: #1a1a2e;
    border-radius: 4px;
    border: 1px solid #0f3460;
  }
  .class-row-field { margin-bottom: 0; }
  .class-row-level input {
    text-align: center;
    padding: 0.4rem;
  }
  .row-remove-btn {
    background: #441111;
    color: #ff8888;
    border: 1px solid #663333;
    border-radius: 4px;
    cursor: pointer;
    height: 38px;
  }
  .row-add-btn {
    padding: 0.5rem 1rem;
    background: #16213e;
    color: #e0e0e0;
    border: 1px dashed #0f3460;
    border-radius: 4px;
    cursor: pointer;
    margin-bottom: 1rem;
  }
  .row-add-btn:disabled { opacity: 0.4; cursor: not-allowed; }
  .bg-skill-hint {
    margin: 0.5rem 0;
    color: #aaa;
    font-size: 0.9rem;
  }
  .bg-skill-tag {
    display: inline-block;
    padding: 0.1rem 0.5rem;
    margin: 0 0.15rem;
    background: #0f3460;
    color: #e0e0e0;
    border-radius: 10px;
    font-size: 0.8rem;
    text-transform: capitalize;
  }
  .bg-skill-tag-inline {
    margin-left: 0.4rem;
    padding: 0.05rem 0.4rem;
    background: #0f3460;
    color: #aac;
    border-radius: 8px;
    font-size: 0.7rem;
  }
  .skill-option.bg-granted { color: #e0e0e0; }
  .bg-info { margin-top: 1rem; padding: 1rem; background: #1a1a2e; border-radius: 4px; border: 1px solid #0f3460; }
  .bg-feature { color: #aaa; font-size: 0.85rem; margin-top: 0.5rem; }
</style>
