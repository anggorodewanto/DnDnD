<script>
  import { tick } from 'svelte';
  import { makeBuilderApi } from './lib/api.js';
  import { remainingPoints, abilityModifier, canIncrement, canDecrement, scoreCost } from './lib/pointbuy.js';
  import { skillsForBackground, backgroundDetails, formatLanguages } from './lib/backgrounds.js';
  import { raceBaseLanguages, availableLanguageChoices, assembleLanguages, bonusLanguageCount } from './lib/languages.js';
  import { abilityLabel } from './lib/skills.js';
  import { formatAbilityBonuses, parseTraits, formatDarkvision, subracePerks, applyAbilityBonuses, removeAbilityBonuses, raceAbilityBonuses } from './lib/race-perks.js';
  import SpellPicker from './SpellPicker.svelte';
  import { anyCaster, multiclassCantripCap, multiclassLeveledCap, levelsUpTo } from './lib/spellcasting.js';
  import { formatProperties, armorACText } from './lib/equipment-perks.js';
  import { raceGrantedSkills } from './lib/race-skills.js';
  import { raceGrantedWeaponProficiencies, weaponProficiencyLabel } from './lib/race-weapon-proficiencies.js';
  import { proficientWeaponIds, masteryEligibleWeapons } from './lib/weapon-proficiency.js';
  import { formatSkillChoices } from './lib/class-perks.js';
  import { computeSkillState, reconcileSkills, isSkillSelectionComplete, computeExpertiseState, reconcileExpertise } from './lib/skill-selection.js';
  import {
    subraceOptions, subclassOptions, isSubclassEligible,
    emptyClassRow, addClassRow, removeClassRow, updateClassRow,
  } from './lib/builder-options.js';
  import { draftKey, draftScope, serializeDraft, parseDraft, draftHasContent } from './lib/builder-draft.js';
  import { humanizeSubmitError } from './lib/submit-error.js';
  import { submissionRequirements, canSubmit } from './lib/submission-requirements.js';
  import { nextStep as computeNextStep, prevStep as computePrevStep, isStepVisible, spellStepState } from './lib/builder-steps.js';
  import { armorOptionIds, weaponOptionIds, reconcileEquipPick } from './lib/equip-selection.js';
  import { assembleEquipment } from './lib/equipment-assembly.js';

  let { mode = 'player', token = '', campaignId = '', editCharacterId = '' } = $props();

  // Edit mode reuses this builder to modify an existing character. The campaign
  // is unknown at mount (no token) and is resolved from the edit-data response,
  // so the campaign used by the API/draft is held in state rather than the prop.
  const editMode = editCharacterId !== '';
  // Seeded from the mount-time prop; in edit mode it is replaced by the
  // campaign resolved from edit-data. The initial-value capture is intentional.
  // svelte-ignore state_referenced_locally
  let campaign = $state(campaignId);

  // Mode-aware API client. The portal and DM dashboard share this component
  // but hit different URL prefixes / request shapes; the factory hides that.
  // Derived so prop reads stay reactive (props are fixed at mount in practice,
  // but this keeps the component warning-clean and correct either way).
  let api = $derived(makeBuilderApi(mode, { campaignId: campaign, token }));

  // localStorage draft is keyed by campaign (not the single-use token) so a
  // reissued /create-character link restores the in-progress draft instead of a
  // blank form; the mode prefix keeps player and DM drafts for the same
  // campaign from colliding in shared localStorage. See draftScope().
  let draftIdentity = $derived(draftScope(mode, campaign, token));

  // Steps
  const STEPS = ['Basics', 'Class', 'Ability Scores', 'Skills', 'Equipment', 'Spells', 'Review'];
  let currentStep = $state(0);

  // Form state preserved across steps
  let name = $state('');
  let race = $state('');
  let subrace = $state('');
  let background = $state('');
  // Free-form flavor text — optional, persisted in the draft and sent in the
  // submission payload as `appearance` / `backstory`.
  let appearance = $state('');
  let backstory = $state('');
  // Multi-class entries — the first row drives the primary class for spell
  // list / starting equipment loading. selectedClass / subclass are kept as
  // derived mirrors of classEntries[0] for compatibility with the existing
  // single-class UI code paths.
  let classEntries = $state([emptyClassRow()]);
  let scores = $state({ str: 8, dex: 8, con: 8, int: 8, wis: 8, cha: 8 });
  let abilityMethod = $state('point_buy');
  let abilityRolls = $state({});
  let selectedSkills = $state([]);
  // Expertise skills (Rogue L1 / Bard L3+) — a subset of the proficient skills
  // that gets double proficiency. Empty for every other class. ISSUE-005.
  let selectedExpertise = $state([]);
  let selectedSpells = $state([]);
  // Concrete bonus languages chosen by the player. The race's base languages
  // are read-only; this holds only the background-granted picks (ISSUE-009).
  let chosenLanguages = $state([]);
  let selectedMasteries = $state([]); // weapon ids whose mastery the character knows

  // Reference data
  let races = $state([]);
  let classes = $state([]);
  let spells = $state([]);
  // Spell-list load status, kept distinct from an empty list so the Spells step
  // can offer a Retry on a genuine load failure instead of misreporting "not a
  // spellcaster" (Finding: Player onboarding T39).
  let spellsLoading = $state(false);
  let spellsError = $state('');
  let allEquipment = $state([]);
  let startingPacks = $state([]);
  let abilityMethods = $state(['point_buy', 'standard_array', 'roll']);

  // Equipment selection state
  let packChoices = $state({});   // { choiceIndex: selectedOptionIndex }
  let manualEquipment = $state([]); // manually added item IDs
  let equipmentSearch = $state('');
  let wornArmor = $state('');      // item id of equipped armor/shield
  let equippedWeapon = $state(''); // item id of the actively-wielded weapon

  // Server-computed authoritative review preview (DerivedStats + features).
  let preview = $state(null);
  let previewLoading = $state(false);
  let previewError = $state('');

  // UI state
  let loading = $state(false);
  let error = $state('');
  let submitted = $state(false);
  let submitting = $state(false);
  // Submit failures render inline beside the Submit button (bottom of the long
  // Review page) — not in the top-of-page `error` banner, which would scroll
  // off-screen — and are humanized into actionable guidance (Finding: Player
  // onboarding T37).
  let submitError = $state('');

  // Load reference data on mount. Memoized so edit-mode prefill can await the
  // same fetch (it needs the race catalog to strip racial bonuses) without
  // triggering a second round of requests.
  let refDataPromise = null;
  function ensureRefData() {
    if (!refDataPromise) refDataPromise = loadRefData();
    return refDataPromise;
  }
  $effect(() => {
    ensureRefData();
  });

  async function loadRefData() {
    try {
      loading = true;
      const [r, c, methods] = await Promise.all([api.listRaces(), api.listClasses(), api.listAbilityMethods()]);
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

  // Keep the skill selection legal as inputs change: re-add the locked
  // (background + race-fixed) skills and prune any discretionary picks that
  // exceed the class/race budgets or fall off the class list. Guarded so it
  // never runs before the race/class catalog has loaded — otherwise the class
  // pool would look empty and valid restored picks would be wrongly dropped.
  $effect(() => {
    if (race && !selectedRaceData) return;
    if (selectedClass && !selectedClassData) return;
    const next = reconcileSkills({
      background,
      raceTraits: selectedRaceData?.traits ?? null,
      classChoices: selectedClassData?.skill_choices ?? null,
      selected: selectedSkills,
    });
    if (!sameSkillSet(next, selectedSkills)) selectedSkills = next;
  });

  // Keep Expertise legal as proficiencies / class / level change: prune picks
  // that are no longer proficient, exceed the class+level grant, or belong to a
  // non-expert class. Mirrors the skills reconcile effect. ISSUE-005.
  $effect(() => {
    if (selectedClass && !selectedClassData) return;
    const next = reconcileExpertise({
      className: selectedClass,
      level: Number(classEntries[0]?.level) || 1,
      proficientSkills: selectedSkills,
      selected: selectedExpertise,
    });
    if (!sameSkillSet(next, selectedExpertise)) selectedExpertise = next;
  });

  // Keep bonus language picks legal as race / background change: drop any pick
  // that is no longer a valid bonus choice (e.g. it became a race base language)
  // and never let the count exceed the background's bonus grant. Mirrors the
  // expertise prune effect. ISSUE-009. Compared against the race base alone so a
  // pick isn't pruned just for appearing in its own `knownLanguages`.
  $effect(() => {
    const selectable = new Set(
      availableLanguageChoices(raceLanguages).map(l => l.toLowerCase()),
    );
    const next = [];
    for (const lang of chosenLanguages) {
      if (next.length >= languageBonusCount) break;
      if (typeof lang !== 'string' || lang.trim() === '') continue;
      if (!selectable.has(lang.toLowerCase())) continue;
      if (next.some(l => l.toLowerCase() === lang.toLowerCase())) continue;
      next.push(lang);
    }
    if (!sameSkillSet(next, chosenLanguages)) chosenLanguages = next;
  });

  // --- Draft persistence (localStorage) --------------------------------
  // Survive an accidental reload: restore unsubmitted fields on init,
  // re-save on every change, and clear once the character is submitted.
  function readDraftRaw() {
    if (typeof localStorage === 'undefined') return null;
    try {
      return localStorage.getItem(draftKey(draftIdentity));
    } catch {
      return null;
    }
  }

  function writeDraftRaw(raw) {
    if (typeof localStorage === 'undefined') return;
    try {
      localStorage.setItem(draftKey(draftIdentity), raw);
    } catch {
      /* quota exceeded or storage disabled — skip silently */
    }
  }

  function clearDraft() {
    if (typeof localStorage === 'undefined') return;
    try {
      localStorage.removeItem(draftKey(draftIdentity));
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
    if (d.appearance !== undefined) appearance = d.appearance;
    if (d.backstory !== undefined) backstory = d.backstory;
    if (Array.isArray(d.classEntries) && d.classEntries.length > 0) classEntries = d.classEntries;
    if (d.scores !== undefined) scores = d.scores;
    if (d.abilityMethod !== undefined) abilityMethod = d.abilityMethod;
    if (d.abilityRolls !== undefined) abilityRolls = d.abilityRolls;
    if (Array.isArray(d.selectedSkills)) selectedSkills = d.selectedSkills;
    if (Array.isArray(d.selectedExpertise)) selectedExpertise = d.selectedExpertise;
    if (Array.isArray(d.selectedSpells)) selectedSpells = d.selectedSpells;
    if (Array.isArray(d.chosenLanguages)) chosenLanguages = d.chosenLanguages;
    if (Array.isArray(d.selectedMasteries)) selectedMasteries = d.selectedMasteries;
    if (d.packChoices !== undefined) packChoices = d.packChoices;
    if (Array.isArray(d.manualEquipment)) manualEquipment = d.manualEquipment;
    if (d.wornArmor !== undefined) wornArmor = d.wornArmor;
    if (d.equippedWeapon !== undefined) equippedWeapon = d.equippedWeapon;
    // Prime the pack-reset guard to the restored class so the class effect
    // doesn't wipe restored pack choices. (Skills are reconciled, not guarded:
    // the reconcile effect prunes any now-illegal restored picks on load.)
    lastClassForPacks = d.classEntries?.[0]?.class || '';
  }

  // Plain snapshot of every persisted field — shared by the persistence effect
  // and the submit/hydration paths so they can never drift apart.
  function currentDraftSnapshot() {
    return $state.snapshot({
      currentStep, name, race, subrace, background, appearance, backstory,
      classEntries, scores, abilityMethod, abilityRolls,
      selectedSkills, selectedExpertise, selectedSpells, chosenLanguages, selectedMasteries, packChoices, manualEquipment,
      wornArmor, equippedWeapon,
    });
  }

  // Restore once, synchronously during init (before any effect runs). Edit mode
  // never uses drafts — it prefills from the saved character instead.
  if (!editMode) {
    const restoredDraft = parseDraft(readDraftRaw());
    if (restoredDraft) applyDraft(restoredDraft);
    // No usable local draft (blank page / different device / cleared on submit)?
    // Fall back to the server-persisted draft so a player who re-ran
    // /create-character after a "request changes" lands on their work, not a
    // blank form (usability T11 / Finding 4·b).
    if (!draftHasContent(restoredDraft)) hydrateFromServerDraft();
  }

  // Edit-mode prefill: fetch the saved character once, resolve its campaign,
  // and map it into the wizard. Plain flag (not $state) so the effect that
  // kicks it off never re-runs.
  let editStarted = false;
  let editLoadError = $state('');
  async function loadEditData() {
    try {
      // Wait for the race catalog so applyEditData can strip racial bonuses
      // from the stored (post-racial) ability scores.
      await ensureRefData();
      const data = await api.editData(editCharacterId);
      campaign = data.campaign_id || '';
      applyEditData(data.character || {});
    } catch (e) {
      editLoadError = e.message || 'Failed to load this character for editing.';
    }
  }
  $effect(() => {
    if (editMode && !editStarted) {
      editStarted = true;
      loadEditData();
    }
  });

  // Map an edit-data submission (snake_case API shape) into wizard state. The
  // inverse of gatherSubmission. Equipment is restored flat into manualEquipment
  // (pack provenance isn't persisted); gatherSubmission re-emits it on save.
  function applyEditData(ch) {
    name = ch.name || '';
    race = ch.race || '';
    subrace = ch.subrace || '';
    background = ch.background || '';
    appearance = ch.appearance || '';
    backstory = ch.backstory || '';
    if (Array.isArray(ch.classes) && ch.classes.length > 0) {
      classEntries = ch.classes.map((c) => ({
        class: c.class || '',
        subclass: c.subclass || '',
        level: c.level || 1,
      }));
    }
    if (ch.ability_scores) {
      const s = ch.ability_scores;
      const postRacial = { str: s.str ?? 8, dex: s.dex ?? 8, con: s.con ?? 8, int: s.int ?? 8, wis: s.wis ?? 8, cha: s.cha ?? 8 };
      // Stored scores are post-racial, but the point-buy widget edits base
      // scores and re-adds racials in finalScores(). Seeding base directly from
      // post-racial would double-count them (e.g. Tiefling CHA 16 → 18) and be
      // rejected by point-buy on save. Strip race + subrace bonuses to recover
      // the base. raceData is read from the loaded catalog (ensureRefData has
      // resolved) rather than the reactive selectedRaceData, which lags here.
      const raceData = races.find((r) => r.id === ch.race);
      const rb = raceAbilityBonuses(raceData);
      const sb = subracePerks(raceData, ch.subrace || '')?.abilityBonuses || {};
      scores = removeAbilityBonuses(postRacial, rb, sb);
    }
    // The generation method isn't persisted; an edit re-validates as point-buy.
    abilityMethod = 'point_buy';
    selectedSkills = Array.isArray(ch.skills) ? ch.skills : [];
    selectedExpertise = Array.isArray(ch.expertise) ? ch.expertise : [];
    selectedSpells = Array.isArray(ch.spells) ? ch.spells : [];
    selectedMasteries = Array.isArray(ch.weapon_masteries) ? ch.weapon_masteries : [];
    chosenLanguages = Array.isArray(ch.languages) ? ch.languages : [];
    manualEquipment = Array.isArray(ch.equipment) ? ch.equipment : [];
    packChoices = {};
    wornArmor = ch.worn_armor || '';
    equippedWeapon = ch.equipped_weapon || '';
    lastClassForPacks = ch.classes?.[0]?.class || '';
  }

  async function hydrateFromServerDraft() {
    let serverDraft;
    try {
      serverDraft = await api.getCharacterDraft();
    } catch {
      return; // a blank form is an acceptable fallback
    }
    if (!serverDraft) return;
    // getCharacterDraft already deserialized the blob; re-stringify so parseDraft
    // applies the same version check + field allow-list it uses for local drafts.
    const parsed = parseDraft(JSON.stringify(serverDraft));
    if (!parsed) return;
    // Don't clobber input the player began typing before the fetch resolved.
    if (draftHasContent(currentDraftSnapshot())) return;
    applyDraft(parsed);
  }

  // Persist on every change to a tracked field; never write after submit. Edit
  // mode does not touch drafts (it would clobber the player's create draft).
  $effect(() => {
    const snapshot = currentDraftSnapshot();
    if (submitted || editMode) return;
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
    spellsError = '';
    spellsLoading = true;
    try {
      spells = await api.listSpells(cls);
    } catch (e) {
      spells = [];
      spellsError = e.message || 'Failed to load spells.';
    } finally {
      spellsLoading = false;
    }
  }

  async function loadEquipment() {
    try {
      allEquipment = await api.listEquipment();
    } catch (e) {
      allEquipment = [];
    }
  }

  async function loadStartingEquipment(cls) {
    try {
      startingPacks = await api.getStartingEquipment(cls);
    } catch (e) {
      startingPacks = [];
    }
  }

  // Step navigation skips the Spells step for non-casters (it would only show
  // a misleading empty state) — see lib/builder-steps.js.
  function nextStep() {
    currentStep = computeNextStep(currentStep, STEPS.length, isCaster);
  }

  function prevStep() {
    currentStep = computePrevStep(currentStep, isCaster);
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

  function toggleExpertise(skill) {
    if (selectedExpertise.includes(skill)) {
      selectedExpertise = selectedExpertise.filter(s => s !== skill);
    } else {
      selectedExpertise = [...selectedExpertise, skill];
    }
  }

  // Set the bonus language in picker slot `index`. A blank value clears the
  // slot; a real value replaces it, dropping any duplicate of that language in
  // another slot so the same language can't be picked twice. The prune effect
  // then re-normalizes (caps to the bonus count, drops now-illegal picks).
  function setLanguageChoice(index, value) {
    const next = [...chosenLanguages];
    if (!value) {
      next.splice(index, 1);
    } else {
      const dup = next.findIndex((l, i) => i !== index && l.toLowerCase() === value.toLowerCase());
      if (dup !== -1) next.splice(dup, 1);
      next[index] = value;
    }
    chosenLanguages = next.filter(l => typeof l === 'string' && l.trim() !== '');
  }

  // Membership-only comparison (order-insensitive) so the reconcile effect
  // only writes selectedSkills when the set actually changed — avoids a
  // write/re-run loop on every reconcile pass.
  function sameSkillSet(a, b) {
    if (a.length !== b.length) return false;
    const set = new Set(b);
    return a.every(s => set.has(s));
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

  // Bare-id equipment list (quantities stripped) — feeds the equipped-weapon /
  // armor pickers, display, and dedup, all of which match on plain ids.
  let selectedEquipment = $derived(() =>
    assembleEquipment({
      pack: startingPacks.length > 0 ? startingPacks[0] : null,
      packChoices,
      manualEquipment,
    })
  );

  // Submission list — identical contents but PRESERVES ":N" quantities so the
  // backend seeds real stack sizes (a full quiver of 20 bolts, two handaxes)
  // instead of one of each. Backend parses the suffix in EquipmentToInventory.
  let submissionEquipment = $derived(() =>
    assembleEquipment({
      pack: startingPacks.length > 0 ? startingPacks[0] : null,
      packChoices,
      manualEquipment,
      preserveQuantities: true,
    })
  );

  // Filtered equipment for search
  let filteredEquipment = $derived(() => {
    if (!equipmentSearch) return allEquipment;
    const q = equipmentSearch.toLowerCase();
    return allEquipment.filter(e => e.name.toLowerCase().includes(q));
  });

  // Equipped-weapon / worn-armor pickers draw from whatever the character has
  // actually selected. Mirrors CharCreatePanel's selectedArmor/WeaponOptions.
  let equipmentById = $derived(() => {
    const m = new Map();
    for (const it of allEquipment) m.set(it.id, it);
    return m;
  });

  function equipmentName(id) {
    const item = equipmentById().get(id);
    return item ? item.name : id.replace(/-/g, ' ');
  }

  // Equippable armor/weapon ids drawn from the selected equipment. The lib
  // helpers recognise an id as armor/weapon from the loaded catalog's category
  // OR a static SRD id fallback, so a pack pick (leather, light crossbow) stays
  // selectable even before allEquipment resolves (ISSUE-011).
  let selectedArmorOptions = $derived(() => armorOptionIds(selectedEquipment(), equipmentById()));
  let selectedWeaponOptions = $derived(() => weaponOptionIds(selectedEquipment(), equipmentById()));

  // Clear worn/equipped picks only when they genuinely fall out of the option
  // list — reconcileEquipPick keeps a still-present pack pick that the catalog
  // hasn't resolved yet, so a legitimate selection is never silently dropped
  // before submit (ISSUE-011).
  $effect(() => {
    const next = reconcileEquipPick(wornArmor, selectedArmorOptions());
    if (next !== wornArmor) wornArmor = next;
  });
  $effect(() => {
    const next = reconcileEquipPick(equippedWeapon, selectedWeaponOptions());
    if (next !== equippedWeapon) equippedWeapon = next;
  });

  function toggleMastery(weaponId) {
    if (selectedMasteries.includes(weaponId)) {
      selectedMasteries = selectedMasteries.filter(id => id !== weaponId);
      return;
    }
    if (selectedMasteries.length >= masteryCount) return;
    selectedMasteries = [...selectedMasteries, weaponId];
  }

  // Derived stats for review
  let selectedRaceData = $derived(races.find(r => r.id === race));
  let selectedClassData = $derived(classes.find(c => c.id === selectedClass));

  // Languages (ISSUE-009): the race grants fixed base languages (locked); the
  // background grants a COUNT of bonus languages the player picks concretely.
  // `knownLanguages` (race base + already-chosen) drives both the prune effect
  // and the per-slot picker options so no language can be chosen twice.
  let raceLanguages = $derived(raceBaseLanguages(selectedRaceData));
  let languageBonusCount = $derived(bonusLanguageCount(backgroundDetails(background)));
  let knownLanguages = $derived([...raceLanguages, ...chosenLanguages]);
  let availableLanguages = $derived(availableLanguageChoices(knownLanguages));

  let weaponIds = $derived(allEquipment.filter(e => e.category === 'weapon').map(e => e.id));
  let weaponList = $derived(allEquipment.filter(e => e.category === 'weapon'));
  let raceWeaponIds = $derived(raceGrantedWeaponProficiencies(selectedRaceData?.traits, weaponIds));
  let proficientIds = $derived(proficientWeaponIds(weaponList, selectedClassData?.weapon_proficiencies, raceWeaponIds));
  let masteryWeapons = $derived(masteryEligibleWeapons(weaponList, proficientIds));
  let masteryCount = $derived(selectedClassData?.weapon_mastery_count || 0);

  let racialBonuses = $derived(() => {
    if (!selectedRaceData?.ability_bonuses) return {};
    try {
      return typeof selectedRaceData.ability_bonuses === 'string'
        ? JSON.parse(selectedRaceData.ability_bonuses)
        : selectedRaceData.ability_bonuses;
    } catch { return {}; }
  });

  // Subrace ability bonuses (e.g. High Elf's +1 INT) are advertised in the race
  // step but must also feed the final scores, not just the display. subracePerks
  // returns null when no/invalid subrace is chosen, so fall back to {}.
  let subraceBonuses = $derived(() => subracePerks(selectedRaceData, subrace)?.abilityBonuses || {});

  let finalScores = $derived(() => applyAbilityBonuses(scores, racialBonuses(), subraceBonuses()));

  // Spellcasting limits for the spell step. The prepared-spell cap (ability
  // modifier + class level) is computed live for instant feedback; the castable
  // spell level comes from the server preview (max_spell_level) so it never
  // drifts from the authoritative slot math. While the preview is still loading
  // we gate all leveled spells (cantrips stay selectable); if it errors we drop
  // the level gate and lean on the server-side count cap.
  let primaryLevel = $derived(Number(classEntries[0]?.level) || 1);
  // The Fighter/Eldritch Knight and Rogue/Arcane Trickster subclasses turn an
  // otherwise non-casting base class into an INT third-caster at class level 3,
  // so the caster gate and every spell budget below must consider the selected
  // subclass + level, not just the base class. In a multiclass build the gate
  // and budgets aggregate across ALL caster entries — a non-caster primary must
  // not hide the Spells step when a secondary casts (e.g. Fighter 1 / Wizard 3).
  let isCaster = $derived(anyCaster(classEntries));
  // Per-class spell counts use that class's own spellcasting ability, so pass
  // the modifier for each casting ability and let the helper pick per entry.
  let spellMods = $derived({
    int: abilityModifier(finalScores().int),
    wis: abilityModifier(finalScores().wis),
    cha: abilityModifier(finalScores().cha),
  });
  // Cantrips and leveled spells have separate budgets: cantrips known is a flat
  // per-class/level count, while leveled spells use the class's prepared cap
  // (ability mod + level) or Spells Known table. Counting cantrips against the
  // leveled cap blocked legal builds (Finding 5), so they are tracked apart. In
  // a multiclass build each is summed over the caster entries (5e computes
  // known/prepared/cantrip counts per class; only spell slots combine).
  let cantripCap = $derived(isCaster ? multiclassCantripCap(classEntries) : Infinity);
  let leveledCap = $derived(isCaster ? multiclassLeveledCap(classEntries, spellMods) : Infinity);
  let spellSelectableLevels = $derived(
    !isCaster ? null : preview ? levelsUpTo(preview.max_spell_level) : previewError ? null : []
  );

  // Submit-gate checklist: surfaces *why* Submit is disabled (instead of a dead
  // button) and blocks submission until ability scores are actually rolled when
  // the Roll method is chosen — default 8s would otherwise fail server-side
  // (Finding: Player onboarding T38).
  let requirements = $derived(submissionRequirements({ name, race, selectedClass, abilityMethod, abilityRolls }));
  let submitReady = $derived(canSubmit(requirements));

  // Build the common snake_case submission object both modes POST. The API
  // factory layers on token / campaign_id per mode.
  function gatherSubmission() {
    // Reconcile to a legal set at submit time as a safety net: locked skills
    // present, nothing beyond the class/race budgets or off the class list.
    // Mirrors the server-side validation so a valid build never gets rejected.
    const skills = reconcileSkills({
      background,
      raceTraits: selectedRaceData?.traits ?? null,
      classChoices: selectedClassData?.skill_choices ?? null,
      selected: selectedSkills,
    });
    // Reconcile Expertise against the final proficient set so a stale draft or
    // a class/level change never submits an illegal expertise pick (ISSUE-005).
    const expertise = reconcileExpertise({
      className: selectedClass,
      level: Number(classEntries[0]?.level) || 1,
      proficientSkills: skills,
      selected: selectedExpertise,
    });
    // Filter out incomplete class rows so the backend never sees a blank
    // class entry.
    const classes = classEntries
      .filter(c => c.class)
      .map(c => ({ class: c.class, level: Number(c.level) || 1, subclass: c.subclass || '' }));
    return {
      name,
      race,
      subrace,
      background,
      appearance,
      backstory,
      class: selectedClass,
      subclass,
      classes,
      ability_scores: finalScores(),
      ability_method: abilityMethod,
      ability_rolls: abilityRolls,
      skills,
      expertise,
      equipment: submissionEquipment(),
      spells: selectedSpells,
      languages: assembleLanguages(raceBaseLanguages(selectedRaceData), chosenLanguages),
      weapon_masteries: selectedMasteries.filter(id => masteryWeapons.some(w => w.id === id)),
      equipped_weapon: equippedWeapon,
      worn_armor: wornArmor,
    };
  }

  // Server-authoritative preview for the Review step. Computes derived stats
  // and the features list. Runs whenever the user enters the Review step.
  async function loadPreview() {
    previewLoading = true;
    previewError = '';
    try {
      preview = await api.previewCharacter(gatherSubmission());
    } catch (e) {
      preview = null;
      previewError = 'Preview failed: ' + e.message;
    } finally {
      previewLoading = false;
    }
  }

  // Load the server preview when entering the Spells step (step 5 — supplies
  // the castable max spell level for the picker) and the Review step (the full
  // stat block). Re-fire only on step change, not on in-step edits.
  const PREVIEW_STEPS = [5, STEPS.length - 1];
  let lastPreviewedStep = -1;
  $effect(() => {
    if (PREVIEW_STEPS.includes(currentStep) && currentStep !== lastPreviewedStep) {
      lastPreviewedStep = currentStep;
      loadPreview();
    } else if (!PREVIEW_STEPS.includes(currentStep)) {
      lastPreviewedStep = -1;
    }
  });

  async function handleSubmit() {
    submitting = true;
    submitError = '';
    try {
      if (editMode) {
        await api.updateCharacter(editCharacterId, gatherSubmission());
        submitted = true;
        return;
      }
      await api.submitCharacter(gatherSubmission(), JSON.parse(serializeDraft(currentDraftSnapshot())));
      submitted = true;
      // Clear only the local draft. The server-side draft is intentionally left
      // in place — it is overwritten on the next submit and cascade-deleted with
      // the campaign — so a later "request changes" can still rehydrate the form.
      clearDraft();
    } catch (e) {
      // Humanize the raw server body and surface it next to the button, then
      // scroll it into view so a failure at the bottom of the Review page is
      // never silent ("the button did nothing").
      submitError = humanizeSubmitError(e.message);
      await tick();
      if (typeof document !== 'undefined') {
        document.getElementById('submit-error')?.scrollIntoView({ behavior: 'smooth', block: 'center' });
      }
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
      {#if editMode}
        <h3>Changes Saved!</h3>
        <p>The character <strong>{name}</strong> has been updated. A player's edit is sent back to the DM for re-approval.</p>
      {:else if mode === 'dm'}
        <h3>Character Created!</h3>
        <p>The character <strong>{name}</strong> has been created and added to the campaign.</p>
      {:else}
        <h3>Character Submitted!</h3>
        <p>Your character <strong>{name}</strong> has been submitted for DM approval. You'll be notified when it's reviewed.</p>
      {/if}
    </div>
  {:else if loading}
    <p>Loading reference data...</p>
  {:else}
    <!-- Step navigation -->
    <nav class="steps">
      {#each STEPS as step, i}
        {#if isStepVisible(i, isCaster)}
          <button
            class="step-btn"
            class:active={i === currentStep}
            class:completed={i < currentStep}
            onclick={() => goToStep(i)}
          >
            {i + 1}. {step}
          </button>
        {/if}
      {/each}
    </nav>

    {#if error}
      <div class="error">{error}</div>
    {/if}
    {#if editLoadError}
      <div class="error" role="alert">{editLoadError}</div>
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
        <label>
          Appearance (optional)
          <textarea
            bind:value={appearance}
            rows="3"
            placeholder="Height, build, hair, eyes, skin, distinguishing marks…"
          ></textarea>
        </label>
        <label>
          Backstory (optional)
          <textarea
            bind:value={backstory}
            rows="4"
            placeholder="Where they're from, what drives them, why they're adventuring…"
          ></textarea>
        </label>
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
      {@const skillState = computeSkillState({
        allSkills: ALL_SKILLS,
        background,
        raceTraits: selectedRaceData?.traits ?? null,
        classChoices: selectedClassData?.skill_choices ?? null,
        selected: selectedSkills,
      })}
      {@const sk = skillState.summary}
      {@const proficientSkills = skillState.skills.filter(s => s.checked).map(s => s.skill)}
      {@const expertiseState = computeExpertiseState({
        className: selectedClass,
        level: Number(classEntries[0]?.level) || 1,
        proficientSkills,
        selected: selectedExpertise,
      })}
      <div class="step-content">
        <h3>Skills & Proficiencies</h3>
        {#if !selectedClass}
          <p class="skill-warn">Pick a class on the Class step to choose your class skills.</p>
        {/if}
        <p class="skill-budget">
          <strong>Class skills:</strong> {sk.classChosen}/{sk.classMax}
          {#if sk.raceMax > 0}
            &nbsp;·&nbsp; <strong>{selectedRaceData?.name} bonus skills:</strong> {sk.raceChosen}/{sk.raceMax}
          {/if}
          {#if !isSkillSelectionComplete({ background, raceTraits: selectedRaceData?.traits ?? null, classChoices: selectedClassData?.skill_choices ?? null, selected: selectedSkills })}
            <span class="skill-incomplete">— choose the remaining skill{sk.classMax + sk.raceMax - sk.classChosen - sk.raceChosen === 1 ? '' : 's'}</span>
          {/if}
        </p>
        {#if background && bgSkills.length > 0}
          <p class="bg-skill-hint">
            From <strong>{background.replace(/-/g, ' ')}</strong> background:
            {#each bgSkills as s}
              <span class="bg-skill-tag">{s.replace(/-/g, ' ')}</span>
            {/each}
          </p>
        {/if}
        {#if selectedRaceData && raceGranted.length > 0}
          <p class="bg-skill-hint">
            From <strong>{selectedRaceData.name}</strong> race:
            {#each raceGranted as s}
              <span class="bg-skill-tag">{s.replace(/-/g, ' ')}</span>
            {/each}
          </p>
        {/if}
        <div class="skill-grid">
          {#each skillState.skills as s}
            <label
              class="skill-option"
              class:bg-granted={s.locked}
              class:skill-disabled={s.disabled && !s.locked}
            >
              <input type="checkbox" checked={s.checked} disabled={s.disabled} onchange={() => toggleSkill(s.skill)} />
              {s.skill.replace(/-/g, ' ')}
              {#if abilityLabel(s.skill)}
                <span class="skill-ability">({abilityLabel(s.skill)})</span>
              {/if}
              {#if s.locked}
                <span class="bg-skill-tag-inline">{s.source}</span>
              {/if}
            </label>
          {/each}
        </div>

        <!-- Expertise (Rogue L1 / Bard L3+): double proficiency on chosen
             skills, selectable only from skills the character is proficient in.
             Hidden for every other class / level. ISSUE-005. -->
        {#if expertiseState.max > 0}
          <div class="expertise-section">
            <h3>Expertise</h3>
            <p class="skill-budget">
              <strong>Expertise:</strong> {expertiseState.chosen}/{expertiseState.max}
              {#if expertiseState.chosen < expertiseState.max}
                <span class="skill-incomplete">— choose {expertiseState.max - expertiseState.chosen} more (doubles your proficiency bonus)</span>
              {/if}
            </p>
            {#if expertiseState.skills.length === 0}
              <p class="skill-warn">Pick your proficient skills above first, then choose Expertise from them.</p>
            {:else}
              <div class="skill-grid">
                {#each expertiseState.skills as s}
                  <label
                    class="skill-option"
                    class:skill-disabled={s.disabled}
                  >
                    <input type="checkbox" checked={s.checked} disabled={s.disabled} onchange={() => toggleExpertise(s.skill)} />
                    {s.skill.replace(/-/g, ' ')}
                    {#if abilityLabel(s.skill)}
                      <span class="skill-ability">({abilityLabel(s.skill)})</span>
                    {/if}
                  </label>
                {/each}
              </div>
            {/if}
          </div>
        {/if}

        <!-- Languages (ISSUE-009): race base languages are fixed (read-only
             chips); the background grants a count of bonus languages the player
             picks here. Concrete strings flow into CharacterSubmission.Languages. -->
        {#if selectedRaceData}
          <div class="language-section">
            <h3>Languages</h3>
            {#if raceLanguages.length > 0}
              <p class="bg-skill-hint">
                From <strong>{selectedRaceData.name}</strong> race:
                {#each raceLanguages as lang}
                  <span class="bg-skill-tag lang-locked">{lang}</span>
                {/each}
              </p>
            {/if}
            {#if languageBonusCount > 0}
              <p class="skill-budget">
                <strong>Bonus languages:</strong> {chosenLanguages.length}/{languageBonusCount}
                {#if chosenLanguages.length < languageBonusCount}
                  <span class="skill-incomplete">— choose {languageBonusCount - chosenLanguages.length} more from your {background.replace(/-/g, ' ')} background</span>
                {/if}
              </p>
              <div class="language-picker">
                {#each Array(languageBonusCount) as _, i}
                  {@const current = chosenLanguages[i] ?? ''}
                  {@const options = current ? [current, ...availableLanguages] : availableLanguages}
                  <select
                    class="language-select"
                    value={current}
                    onchange={(e) => setLanguageChoice(i, e.currentTarget.value)}
                  >
                    <option value="">— choose a language —</option>
                    {#each options as opt}
                      <option value={opt}>{opt}</option>
                    {/each}
                  </select>
                {/each}
              </div>
            {:else}
              <p class="bg-skill-hint">Your background grants no bonus languages.</p>
            {/if}
          </div>
        {/if}
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

        <!-- Worn armor + equipped weapon pickers -->
        <div class="equipment-section">
          <h4>Active Loadout</h4>
          <label>
            Worn Armor
            <select bind:value={wornArmor}>
              <option value="">None</option>
              {#each selectedArmorOptions() as id}
                <option value={id}>{equipmentName(id)}</option>
              {/each}
            </select>
          </label>
          <label>
            Equipped Weapon
            <select bind:value={equippedWeapon}>
              <option value="">None</option>
              {#each selectedWeaponOptions() as id}
                <option value={id}>{equipmentName(id)}</option>
              {/each}
            </select>
          </label>
        </div>

        {#if masteryCount > 0}
          <div class="equipment-section mastery-section">
            <h4>Weapon Mastery</h4>
            <p class="mastery-hint">Choose up to {masteryCount} weapon{masteryCount === 1 ? '' : 's'} you're proficient with to gain its mastery property. ({selectedMasteries.length}/{masteryCount})</p>
            {#if masteryWeapons.length === 0}
              <p class="muted">Select your class first — masteries come from weapons you're proficient with.</p>
            {:else}
              <div class="mastery-grid">
                {#each masteryWeapons as w (w.id)}
                  {@const chosen = selectedMasteries.includes(w.id)}
                  <label class="mastery-option" class:selected={chosen}>
                    <input type="checkbox" checked={chosen} disabled={!chosen && selectedMasteries.length >= masteryCount} onchange={() => toggleMastery(w.id)} />
                    <span class="mastery-weapon-name">{w.name}</span>
                    <span class="item-mastery">{weaponProficiencyLabel(w.mastery)}</span>
                  </label>
                {/each}
              </div>
            {/if}
          </div>
        {/if}
      </div>

    <!-- Step 5: Spells -->
    {:else if currentStep === 5}
      {@const spellState = spellStepState({ isCaster, loading: spellsLoading, error: spellsError, count: spells.length })}
      <div class="step-content">
        <h3>Spells</h3>
        {#if spellState === 'not-caster'}
          <p>Your class doesn't cast spells — continue to Review.</p>
        {:else if spellState === 'loading'}
          <p class="muted">Loading spells…</p>
        {:else if spellState === 'error'}
          <p class="preview-error">Couldn't load the spell list. Check your connection and try again.</p>
          <button type="button" class="nav-btn" onclick={() => loadSpells(selectedClass)}>Retry</button>
        {:else if spellState === 'empty'}
          <p>No spells are available for your class yet.</p>
        {:else}
          <p class="spell-cap-hint">
            {#if cantripCap > 0}
              Choose up to <strong>{cantripCap}</strong> cantrip{cantripCap === 1 ? '' : 's'} and
              <strong>{leveledCap}</strong> leveled spell{leveledCap === 1 ? '' : 's'}.
            {:else}
              Choose up to <strong>{leveledCap}</strong> leveled spell{leveledCap === 1 ? '' : 's'}.
            {/if}
            Browse every level — you can only pick spells you have slots for.
          </p>
          <SpellPicker
            {spells}
            bind:selected={selectedSpells}
            max={leveledCap}
            cantripMax={cantripCap}
            selectableLevels={spellSelectableLevels}
          />
        {/if}
      </div>

    <!-- Step 6: Review -->
    {:else if currentStep === 6}
      {@const submission = gatherSubmission()}
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

        <!-- Server-authoritative derived stats. Supersedes the old
             client-side estimates so the player/DM sees the same numbers the
             backend will persist. -->
        <div class="review-section">
          <h4>Derived Stats</h4>
          {#if previewLoading}
            <p class="muted">Computing stats…</p>
          {:else if previewError}
            <p class="preview-error">{previewError}</p>
          {:else if preview}
            <div class="review-scores">
              <div class="review-score"><span class="ability-label">HP</span><span class="ability-value">{preview.hp_max}</span></div>
              <div class="review-score"><span class="ability-label">AC</span><span class="ability-value">{preview.ac}</span></div>
              <div class="review-score"><span class="ability-label">Speed</span><span class="ability-value">{preview.speed_ft} ft</span></div>
              <div class="review-score"><span class="ability-label">Prof</span><span class="ability-value">+{preview.proficiency_bonus}</span></div>
              <div class="review-score"><span class="ability-label">Level</span><span class="ability-value">{preview.total_level}</span></div>
            </div>
            {#if preview.saves}
              <h4 class="subhead">Saving Throws</h4>
              <div class="review-scores">
                {#each ABILITIES as ability}
                  <div class="review-score">
                    <span class="ability-label">{ability.toUpperCase()}</span>
                    <span class="ability-value">{preview.saves[ability] >= 0 ? '+' : ''}{preview.saves[ability]}</span>
                  </div>
                {/each}
              </div>
            {/if}
            {#if preview.spell_slots && Object.keys(preview.spell_slots).length > 0}
              <h4 class="subhead">Spell Slots</h4>
              <div class="review-scores">
                {#each Object.keys(preview.spell_slots).sort() as lvl}
                  <div class="review-score">
                    <span class="ability-label">Lvl {lvl}</span>
                    <span class="ability-value">{preview.spell_slots[lvl]}</span>
                  </div>
                {/each}
              </div>
            {/if}
          {:else}
            <p class="muted">No stats available.</p>
          {/if}
        </div>

        <!-- Class / subclass / racial features computed by the server. -->
        <div class="review-section">
          <h4>Features</h4>
          {#if previewLoading}
            <p class="muted">Loading features…</p>
          {:else if preview && preview.features && preview.features.length > 0}
            <div class="features-list">
              {#each preview.features as f}
                <div class="feature-card">
                  <div class="feat-name">{f.name}</div>
                  <div class="feat-source">{f.source}{#if f.level} (Level {f.level}){/if}</div>
                  {#if f.description}
                    <div class="feat-desc">{f.description}</div>
                  {/if}
                </div>
              {/each}
            </div>
          {:else}
            <p class="muted">No features for the current selection.</p>
          {/if}
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

        {#if selectedMasteries.length > 0}
          <div class="review-section">
            <h4>Weapon Masteries</h4>
            <p>{masteryWeapons.filter(w => selectedMasteries.includes(w.id)).map(w => `${w.name} (${weaponProficiencyLabel(w.mastery)})`).join(', ')}</p>
          </div>
        {/if}

        {#if submission.worn_armor || submission.equipped_weapon}
          <div class="review-section">
            <h4>Active Loadout</h4>
            {#if submission.worn_armor}
              <p><strong>Worn Armor:</strong> {equipmentName(submission.worn_armor)}</p>
            {/if}
            {#if submission.equipped_weapon}
              <p><strong>Equipped Weapon:</strong> {equipmentName(submission.equipped_weapon)}</p>
            {/if}
          </div>
        {/if}

        {#if submitError}
          <div id="submit-error" class="error submit-error" role="alert">{submitError}</div>
        {/if}

        {#if !submitReady}
          <div class="submit-checklist">
            <p class="submit-checklist-head">Before you can submit:</p>
            <ul>
              {#each requirements as req (req.key)}
                <li class:met={req.met}>
                  <span class="check-mark" aria-hidden="true">{req.met ? '✓' : '○'}</span>{req.label}
                </li>
              {/each}
            </ul>
          </div>
        {/if}

        <button class="submit-btn" onclick={handleSubmit} disabled={submitting || !submitReady}>
          {#if submitting}
            {editMode ? 'Saving...' : mode === 'dm' ? 'Creating...' : 'Submitting...'}
          {:else if editMode}
            Save changes
          {:else}
            {mode === 'dm' ? 'Create Character' : 'Submit for DM Approval'}
          {/if}
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
  input[type="text"], select, textarea {
    display: block; width: 100%; padding: 0.5rem; margin-top: 0.25rem;
    background: #1a1a2e; color: #e0e0e0; border: 1px solid #0f3460; border-radius: 4px;
  }
  textarea { font-family: inherit; resize: vertical; box-sizing: border-box; }
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
  .skill-option.skill-disabled { color: #666; }
  .skill-option.skill-disabled input { cursor: not-allowed; }
  .skill-budget { color: #ccc; font-size: 0.9rem; margin: 0 0 0.6rem; }
  .skill-budget strong { color: #e94560; }
  .skill-incomplete { color: #e9a045; font-size: 0.85rem; margin-left: 0.25rem; }
  .skill-warn { color: #e9a045; font-size: 0.85rem; margin: 0 0 0.5rem; }
  .expertise-section { margin-top: 1.25rem; padding-top: 1rem; border-top: 1px solid #2a2a3a; }
  .expertise-section h3 { margin: 0 0 0.5rem; }

  .language-section { margin-top: 1.25rem; padding-top: 1rem; border-top: 1px solid #2a2a3a; }
  .language-section h3 { margin: 0 0 0.5rem; }
  .lang-locked { text-transform: none; }
  .language-picker { display: flex; flex-direction: column; gap: 0.5rem; max-width: 320px; }
  .language-select { width: 100%; }

  .spell-cap-hint { color: #aaa; font-size: 0.85rem; margin: 0 0 0.6rem; }
  .spell-cap-hint strong { color: #e94560; }
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
  .review-section h4.subhead { margin-top: 0.75rem; font-size: 0.95rem; }
  .preview-error { color: #ff8888; font-size: 0.9rem; }
  .features-list { display: flex; flex-direction: column; gap: 0.4rem; }
  .feature-card { background: #16213e; border: 1px solid #0f3460; border-radius: 4px; padding: 0.5rem; }
  .feature-card .feat-name { color: #e94560; font-weight: 700; }
  .feature-card .feat-source { color: #999; font-size: 0.8rem; }
  .feature-card .feat-desc { margin-top: 0.25rem; font-size: 0.9rem; color: #ccc; }
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
  .submit-error { margin-top: 1rem; }
  .submit-checklist { margin-top: 1rem; padding: 0.75rem 1rem; background: #1a1a2e; border: 1px solid #0f3460; border-radius: 4px; }
  .submit-checklist-head { margin: 0 0 0.4rem; color: #e9a045; font-size: 0.9rem; }
  .submit-checklist ul { margin: 0; padding-left: 0; list-style: none; }
  .submit-checklist li { color: #e9a045; font-size: 0.9rem; padding: 0.1rem 0; }
  .submit-checklist li.met { color: #4caf50; }
  .submit-checklist .check-mark { display: inline-block; width: 1.2rem; font-weight: bold; }
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
  .muted { color: #888; font-size: 0.85rem; }
  .mastery-hint { color: #aaa; font-size: 0.85rem; margin-bottom: 0.5rem; }
  .mastery-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(220px, 1fr)); gap: 0.4rem; }
  .mastery-option { display: flex; align-items: center; gap: 0.5rem; padding: 0.4rem 0.5rem; border: 1px solid #0f3460; border-radius: 6px; background: #16213e; cursor: pointer; }
  .mastery-option.selected { border-color: #e94560; }
  .mastery-weapon-name { font-weight: 600; }
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

  /* Phones (most players reach the builder via the Discord in-app browser):
     the 2-column skill grid and 4-column class row are too cramped under
     ~480px, so stack them into a single column (Finding: Player onboarding
     T40). */
  @media (max-width: 480px) {
    .skill-grid { grid-template-columns: 1fr; }
    .class-row { grid-template-columns: 1fr; }
  }
</style>
