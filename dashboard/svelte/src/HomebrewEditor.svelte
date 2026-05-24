<script>
  // Homebrew editor: per-category structured form. Bindings flow through
  // lib/homebrewForm.js so the payload matches refdata.Upsert*Params on the
  // backend. The "class-features" sub-mode emits a single-feature class
  // skeleton through /api/homebrew/classes — the backend exposes only the
  // unified classes endpoint today.
  import {
    HOMEBREW_CATEGORIES,
    emptyFormModel,
    buildHomebrewPayload,
    entryToFormModel,
  } from './lib/homebrewForm.js';

  let { campaignId } = $props();

  let category = $state('creatures');
  let entries = $state([]);
  let loading = $state(false);
  let error = $state(null);
  let formOpen = $state(false);
  let formModel = $state(emptyFormModel('creatures'));
  let formError = $state(null);
  let editingId = $state(null);

  // Turn a failed fetch Response into a human-readable message. Falls back to
  // the server's JSON {error} field when present.
  async function errorMessage(res, noun, verb) {
    let detail = '';
    try {
      const data = await res.json();
      if (data && typeof data.error === 'string') detail = data.error;
    } catch {
      // non-JSON body — ignore
    }
    if (res.status === 400) return detail || `That ${noun} request was invalid.`;
    if (res.status === 401 || res.status === 403)
      return `You don't have permission to ${verb} ${noun} for this campaign.`;
    if (res.status === 404)
      return `Couldn't ${verb} ${noun} — this feature is unavailable. The server may need updating.`;
    if (res.status >= 500)
      return `The server hit an error trying to ${verb} ${noun}. Please try again.`;
    return detail || `Couldn't ${verb} ${noun} (HTTP ${res.status}).`;
  }

  // Network-level failures (fetch throws a TypeError) get a connectivity hint;
  // anything else surfaces its own message.
  function networkOrMessage(e, noun, verb) {
    if (e instanceof TypeError) {
      return `Couldn't reach the server to ${verb} ${noun}. Check your connection and try again.`;
    }
    return e.message;
  }

  // Upstream route segment for the active category. List, create, update, and
  // delete all hit /api/homebrew/${categoryPath()}. class-features reuses the
  // classes path, matching the backend mount.
  function categoryPath() {
    const cat = HOMEBREW_CATEGORIES.find((c) => c.key === category);
    return cat ? cat.path : category;
  }

  async function loadEntries() {
    if (!campaignId) {
      entries = [];
      return;
    }
    loading = true;
    error = null;
    try {
      const url = `/api/homebrew/${categoryPath()}?campaign_id=${encodeURIComponent(campaignId)}&homebrew=true`;
      const res = await fetch(url, { credentials: 'same-origin' });
      if (!res.ok) {
        error = await errorMessage(res, category, 'load');
        return;
      }
      const data = await res.json();
      let list = (Array.isArray(data) ? data : data.items || []).filter((e) => e.homebrew !== false);
      // For class-features sub-mode, only surface single-feature skeletons.
      if (category === 'class-features') {
        list = list.filter(
          (e) => Array.isArray(e.features_by_level) && e.features_by_level.length === 1,
        );
      }
      entries = list;
    } catch (e) {
      error = networkOrMessage(e, category, 'load');
    } finally {
      loading = false;
    }
  }

  function openCreate() {
    editingId = null;
    formModel = emptyFormModel(category);
    formError = null;
    formOpen = true;
  }

  function openEdit(entry) {
    editingId = entry.id;
    formModel = entryToFormModel(category, entry);
    formError = null;
    formOpen = true;
  }

  async function submitForm() {
    formError = null;
    let body;
    try {
      body = buildHomebrewPayload(category, formModel);
    } catch (e) {
      formError = e.message;
      return;
    }
    const baseURL = `/api/homebrew/${categoryPath()}`;
    const url = editingId
      ? `${baseURL}/${encodeURIComponent(editingId)}?campaign_id=${encodeURIComponent(campaignId)}`
      : `${baseURL}?campaign_id=${encodeURIComponent(campaignId)}`;
    const method = editingId ? 'PUT' : 'POST';
    try {
      const res = await fetch(url, {
        method,
        credentials: 'same-origin',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      });
      if (!res.ok) {
        formError = await errorMessage(res, category, editingId ? 'update' : 'create');
        return;
      }
      formOpen = false;
      await loadEntries();
    } catch (e) {
      formError = networkOrMessage(e, category, editingId ? 'update' : 'create');
    }
  }

  async function handleDelete(entry) {
    if (!confirm(`Delete ${category} "${entry.name || entry.id}"?`)) return;
    try {
      const url = `/api/homebrew/${categoryPath()}/${encodeURIComponent(entry.id)}?campaign_id=${encodeURIComponent(campaignId)}`;
      const res = await fetch(url, { method: 'DELETE', credentials: 'same-origin' });
      if (!res.ok) {
        error = await errorMessage(res, category, 'delete');
        return;
      }
      await loadEntries();
    } catch (e) {
      error = networkOrMessage(e, category, 'delete');
    }
  }

  $effect(() => {
    if (campaignId && category) {
      loadEntries();
    }
  });

  // Reset the form model whenever the category changes so the bound
  // controls don't accidentally carry stale fields between categories.
  function selectCategory(key) {
    category = key;
    formOpen = false;
    editingId = null;
    formModel = emptyFormModel(key);
  }
</script>

<div class="homebrew-editor" data-testid="homebrew-editor">
  <h2>Homebrew Editor</h2>

  <div class="category-tabs">
    {#each HOMEBREW_CATEGORIES as cat}
      <button
        class:active={category === cat.key}
        onclick={() => selectCategory(cat.key)}>{cat.label}</button>
    {/each}
  </div>

  <div class="actions">
    <button class="create-btn" onclick={openCreate}>+ New {category}</button>
  </div>

  {#if loading}
    <p>Loading...</p>
  {:else if error}
    <p class="error">{error}</p>
  {:else if entries.length === 0}
    <p>No homebrew {category} yet.</p>
  {:else}
    <ul class="entry-list">
      {#each entries as entry (entry.id)}
        <li>
          <strong>{entry.name || entry.id}</strong>
          <span class="muted">{entry.id}</span>
          <button onclick={() => openEdit(entry)}>Edit</button>
          <button class="delete-btn" onclick={() => handleDelete(entry)}>Delete</button>
        </li>
      {/each}
    </ul>
  {/if}

  {#if formOpen}
    <div class="form">
      <h3>{editingId ? 'Edit' : 'New'} {category}</h3>

      {#if category === 'creatures'}
        <label>ID
          <input bind:value={formModel.id} placeholder="goblin-king" />
        </label>
        <label>Name
          <input bind:value={formModel.name} />
        </label>
        <label>Size
          <select bind:value={formModel.size}>
            {#each ['Tiny', 'Small', 'Medium', 'Large', 'Huge', 'Gargantuan'] as s}
              <option value={s}>{s}</option>
            {/each}
          </select>
        </label>
        <label>Type
          <input bind:value={formModel.type} />
        </label>
        <label>Alignment
          <input bind:value={formModel.alignment} />
        </label>
        <label>AC
          <input type="number" bind:value={formModel.ac} />
        </label>
        <label>AC type
          <input bind:value={formModel.ac_type} />
        </label>
        <label>HP formula
          <input bind:value={formModel.hp_formula} placeholder="2d8+2" />
        </label>
        <label>HP average
          <input type="number" bind:value={formModel.hp_average} />
        </label>
        <label>Speed JSON
          <textarea bind:value={formModel.speed_json} rows="2"></textarea>
        </label>
        <label>Ability scores JSON
          <textarea bind:value={formModel.ability_scores_json} rows="3"></textarea>
        </label>
        <label>Damage resistances (csv)
          <input bind:value={formModel.damage_resistances} />
        </label>
        <label>Damage immunities (csv)
          <input bind:value={formModel.damage_immunities} />
        </label>
        <label>Damage vulnerabilities (csv)
          <input bind:value={formModel.damage_vulnerabilities} />
        </label>
        <label>Condition immunities (csv)
          <input bind:value={formModel.condition_immunities} />
        </label>
        <label>Languages (csv)
          <input bind:value={formModel.languages} />
        </label>
        <label>CR
          <input bind:value={formModel.cr} placeholder="1/4 or 5" />
        </label>
        <label>Attacks JSON
          <textarea bind:value={formModel.attacks_json} rows="4"></textarea>
        </label>
      {:else if category === 'spells'}
        <label>ID
          <input bind:value={formModel.id} />
        </label>
        <label>Name
          <input bind:value={formModel.name} />
        </label>
        <label>Level
          <input type="number" bind:value={formModel.level} />
        </label>
        <label>School
          <input bind:value={formModel.school} />
        </label>
        <label>Casting time
          <input bind:value={formModel.casting_time} />
        </label>
        <label>Range (ft)
          <input type="number" bind:value={formModel.range_ft} />
        </label>
        <label>Range type
          <input bind:value={formModel.range_type} />
        </label>
        <label>Components (csv)
          <input bind:value={formModel.components} placeholder="V,S,M" />
        </label>
        <label>Material description
          <input bind:value={formModel.material_description} />
        </label>
        <label>Material cost (gp)
          <input type="number" step="0.01" bind:value={formModel.material_cost_gp} />
        </label>
        <label class="checkbox">
          <input type="checkbox" bind:checked={formModel.material_consumed} />
          Material consumed
        </label>
        <label>Duration
          <input bind:value={formModel.duration} />
        </label>
        <label class="checkbox">
          <input type="checkbox" bind:checked={formModel.concentration} /> Concentration
        </label>
        <label class="checkbox">
          <input type="checkbox" bind:checked={formModel.ritual} /> Ritual
        </label>
        <label>Description
          <textarea bind:value={formModel.description} rows="4"></textarea>
        </label>
        <label>Higher levels
          <textarea bind:value={formModel.higher_levels} rows="2"></textarea>
        </label>
        <label>Resolution mode
          <input bind:value={formModel.resolution_mode} placeholder="attack, save, or none" />
        </label>
        <label>Classes (csv)
          <input bind:value={formModel.classes} />
        </label>
      {:else if category === 'weapons'}
        <label>ID
          <input bind:value={formModel.id} />
        </label>
        <label>Name
          <input bind:value={formModel.name} />
        </label>
        <label>Damage
          <input bind:value={formModel.damage} placeholder="1d8" />
        </label>
        <label>Damage type
          <input bind:value={formModel.damage_type} />
        </label>
        <label>Weight (lb)
          <input type="number" step="0.1" bind:value={formModel.weight_lb} />
        </label>
        <label>Properties (csv)
          <input bind:value={formModel.properties} />
        </label>
        <label>Range normal (ft)
          <input type="number" bind:value={formModel.range_normal_ft} />
        </label>
        <label>Range long (ft)
          <input type="number" bind:value={formModel.range_long_ft} />
        </label>
        <label>Versatile damage
          <input bind:value={formModel.versatile_damage} />
        </label>
        <label>Weapon type
          <input bind:value={formModel.weapon_type} />
        </label>
      {:else if category === 'magic-items'}
        <label>ID
          <input bind:value={formModel.id} />
        </label>
        <label>Name
          <input bind:value={formModel.name} />
        </label>
        <label>Base item type
          <input bind:value={formModel.base_item_type} />
        </label>
        <label>Base item id
          <input bind:value={formModel.base_item_id} />
        </label>
        <label>Rarity
          <select bind:value={formModel.rarity}>
            {#each ['common', 'uncommon', 'rare', 'very-rare', 'legendary', 'artifact'] as r}
              <option value={r}>{r}</option>
            {/each}
          </select>
        </label>
        <label class="checkbox">
          <input type="checkbox" bind:checked={formModel.requires_attunement} />
          Requires attunement
        </label>
        <label>Attunement restriction
          <input bind:value={formModel.attunement_restriction} />
        </label>
        <label>Magic bonus
          <input type="number" bind:value={formModel.magic_bonus} />
        </label>
        <label>Description
          <textarea bind:value={formModel.description} rows="4"></textarea>
        </label>
      {:else if category === 'races'}
        <label>ID
          <input bind:value={formModel.id} />
        </label>
        <label>Name
          <input bind:value={formModel.name} />
        </label>
        <label>Speed (ft)
          <input type="number" bind:value={formModel.speed_ft} />
        </label>
        <label>Size
          <select bind:value={formModel.size}>
            {#each ['Small', 'Medium', 'Large'] as s}
              <option value={s}>{s}</option>
            {/each}
          </select>
        </label>
        <label>Ability bonuses JSON
          <textarea bind:value={formModel.ability_bonuses_json} rows="3"></textarea>
        </label>
        <label>Darkvision (ft)
          <input type="number" bind:value={formModel.darkvision_ft} />
        </label>
        <label>Traits JSON
          <textarea bind:value={formModel.traits_json} rows="4"></textarea>
        </label>
        <label>Languages (csv)
          <input bind:value={formModel.languages} />
        </label>
      {:else if category === 'feats'}
        <label>ID
          <input bind:value={formModel.id} />
        </label>
        <label>Name
          <input bind:value={formModel.name} />
        </label>
        <label>Description
          <textarea bind:value={formModel.description} rows="6"></textarea>
        </label>
      {:else if category === 'classes'}
        <label>ID
          <input bind:value={formModel.id} />
        </label>
        <label>Name
          <input bind:value={formModel.name} />
        </label>
        <label>Hit die
          <input bind:value={formModel.hit_die} placeholder="d8" />
        </label>
        <label>Primary ability
          <input bind:value={formModel.primary_ability} placeholder="str" />
        </label>
        <label>Save proficiencies (csv)
          <input bind:value={formModel.save_proficiencies} />
        </label>
        <label>Armor proficiencies (csv)
          <input bind:value={formModel.armor_proficiencies} />
        </label>
        <label>Weapon proficiencies (csv)
          <input bind:value={formModel.weapon_proficiencies} />
        </label>
        <label>Features by level JSON
          <textarea bind:value={formModel.features_by_level_json} rows="6"></textarea>
        </label>
        <label>Attacks per action JSON
          <textarea bind:value={formModel.attacks_per_action_json} rows="2"></textarea>
        </label>
        <label>Subclass level
          <input type="number" bind:value={formModel.subclass_level} />
        </label>
        <label>Subclasses JSON
          <textarea bind:value={formModel.subclasses_json} rows="4"></textarea>
        </label>
      {:else if category === 'class-features'}
        <p class="hint">
          Adds a single class feature as a homebrew class skeleton. The parent
          class fields are stored on the feature for traceability.
        </p>
        <label>ID
          <input bind:value={formModel.id} placeholder="fighter-cleave" />
        </label>
        <label>Parent class ID
          <input bind:value={formModel.class_id} />
        </label>
        <label>Parent class name
          <input bind:value={formModel.class_name} />
        </label>
        <label>Feature name
          <input bind:value={formModel.feature_name} />
        </label>
        <label>Level
          <input type="number" bind:value={formModel.level} />
        </label>
        <label>Description
          <textarea bind:value={formModel.description} rows="6"></textarea>
        </label>
      {/if}

      {#if formError}
        <p class="error">{formError}</p>
      {/if}
      <div class="form-actions">
        <button onclick={submitForm}>Save</button>
        <button onclick={() => (formOpen = false)}>Cancel</button>
      </div>
    </div>
  {/if}
</div>

<style>
  .homebrew-editor {
    max-width: 1000px;
  }
  h2 {
    color: #e94560;
  }
  .category-tabs {
    display: flex;
    gap: 0.25rem;
    margin-bottom: 1rem;
    flex-wrap: wrap;
  }
  .category-tabs button {
    padding: 0.4rem 0.75rem;
    background: #16213e;
    color: #e0e0e0;
    border: 1px solid #0f3460;
    border-radius: 4px;
    cursor: pointer;
  }
  .category-tabs button.active {
    background: #e94560;
    border-color: #e94560;
  }
  .actions {
    margin-bottom: 0.75rem;
  }
  .create-btn {
    padding: 0.5rem 1rem;
    background: #e94560;
    color: white;
    border: none;
    border-radius: 4px;
    cursor: pointer;
  }
  .entry-list {
    list-style: none;
    padding: 0;
  }
  .entry-list li {
    background: #16213e;
    padding: 0.5rem 0.75rem;
    border-radius: 4px;
    margin-bottom: 0.25rem;
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }
  .muted {
    color: #888;
    font-size: 0.85rem;
  }
  .delete-btn {
    background: #aa2030;
    color: white;
    border: none;
    border-radius: 3px;
    padding: 0.25rem 0.5rem;
    cursor: pointer;
  }
  .form {
    margin-top: 1rem;
    background: #16213e;
    padding: 1rem;
    border-radius: 6px;
    border: 1px solid #0f3460;
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 0.5rem 1rem;
  }
  .form h3 {
    grid-column: 1 / -1;
    margin: 0;
  }
  .form label {
    display: flex;
    flex-direction: column;
    color: #a0aec0;
    font-size: 0.8rem;
  }
  .form label.checkbox {
    flex-direction: row;
    align-items: center;
    gap: 0.5rem;
  }
  .form input,
  .form textarea,
  .form select {
    background: #0f1626;
    color: #e0e0e0;
    border: 1px solid #0f3460;
    border-radius: 4px;
    padding: 0.4rem 0.5rem;
    font: inherit;
    margin-top: 0.15rem;
  }
  .form textarea {
    font-family: monospace;
  }
  .form-actions {
    grid-column: 1 / -1;
    display: flex;
    gap: 0.5rem;
  }
  .form-actions button {
    padding: 0.5rem 1rem;
    background: #0f3460;
    color: #e0e0e0;
    border: none;
    border-radius: 4px;
    cursor: pointer;
  }
  .form-actions button:first-child {
    background: #e94560;
    color: white;
  }
  .hint {
    grid-column: 1 / -1;
    color: #b0b0c0;
    font-size: 0.85rem;
    margin: 0;
  }
  .error {
    grid-column: 1 / -1;
    color: #ff6b6b;
  }
  @media (max-width: 768px) {
    .form {
      grid-template-columns: 1fr;
    }
  }
</style>
