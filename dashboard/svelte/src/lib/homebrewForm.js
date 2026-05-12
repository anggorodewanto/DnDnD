/**
 * Homebrew form helpers (Phase 99, G-99).
 *
 * The dashboard's structured homebrew editor binds form fields to a flat
 * model per category. These pure helpers translate that flat model into
 * the JSON shape the backend expects (matching the Go `refdata.Upsert*Params`
 * structs as serialized over the wire), and back the other way for editing
 * an existing entry.
 *
 * Backend reference shapes live in
 *   internal/refdata/{creatures,spells,weapons,magic_items,races,feats,classes}.sql.go
 *
 * Notable wire conventions inherited from the sqlc-generated types:
 *   - `sql.NullString` / `sql.NullBool` / `sql.NullInt32` / `sql.NullFloat64`
 *     serialize as `{ Valid: bool, String|Bool|Int32|Float64: <value> }` —
 *     but for inbound POST bodies the server's stdlib decoder also accepts
 *     either form. We submit `null` for unset optionals and the server
 *     happily writes a NULL.
 *   - `pqtype.NullRawMessage` and raw JSON fields submit as inline JSON.
 *   - `[]string` arrays submit as JSON arrays.
 *   - `uuid.NullUUID` for campaign_id is filled in by the backend handler
 *     from the URL query string, so we never submit it.
 */

/**
 * Categories supported by `/api/homebrew/*`.
 */
export const HOMEBREW_CATEGORIES = Object.freeze([
  { key: 'creatures', label: 'Creatures', path: 'creatures' },
  { key: 'spells', label: 'Spells', path: 'spells' },
  { key: 'weapons', label: 'Weapons', path: 'weapons' },
  { key: 'magic-items', label: 'Magic Items', path: 'magic-items' },
  { key: 'races', label: 'Races', path: 'races' },
  { key: 'feats', label: 'Feats', path: 'feats' },
  { key: 'classes', label: 'Classes', path: 'classes' },
  // Class-feature-only sub-path. The backend exposes only `/api/homebrew/classes`
  // today, so the UI builds a single-feature class skeleton and posts it
  // through the same endpoint. The class JSON carries a single entry in
  // features_by_level so the homebrew row exists alongside SRD classes.
  { key: 'class-features', label: 'Class Feature', path: 'classes' },
]);

/**
 * Helpers for parsing optional inputs into the wire shape.
 */
function trimOrNull(value) {
  if (value === null || value === undefined) return null;
  const s = String(value).trim();
  return s === '' ? null : s;
}

function intOrNull(value) {
  if (value === null || value === undefined || value === '') return null;
  const n = parseInt(value, 10);
  return Number.isNaN(n) ? null : n;
}

function floatOrNull(value) {
  if (value === null || value === undefined || value === '') return null;
  const n = parseFloat(value);
  return Number.isNaN(n) ? null : n;
}

function csvToList(value) {
  if (!value) return [];
  return String(value)
    .split(',')
    .map((s) => s.trim())
    .filter((s) => s.length > 0);
}

function parseJSONOrNull(value, fallback = null) {
  const s = trimOrNull(value);
  if (s === null) return fallback;
  try {
    return JSON.parse(s);
  } catch (_) {
    throw new Error('invalid JSON in structured field');
  }
}

/**
 * Empty form-model factory per category.
 * @param {string} category
 */
export function emptyFormModel(category) {
  switch (category) {
    case 'creatures':
      return {
        id: '',
        name: '',
        size: 'Medium',
        type: 'humanoid',
        alignment: '',
        ac: 10,
        ac_type: '',
        hp_formula: '1d8',
        hp_average: 4,
        speed_json: '{"walk_ft":30}',
        ability_scores_json: '{"str":10,"dex":10,"con":10,"int":10,"wis":10,"cha":10}',
        damage_resistances: '',
        damage_immunities: '',
        damage_vulnerabilities: '',
        condition_immunities: '',
        languages: '',
        cr: '0',
        attacks_json: '[]',
      };
    case 'spells':
      return {
        id: '',
        name: '',
        level: 0,
        school: 'evocation',
        casting_time: '1 action',
        range_ft: '',
        range_type: 'feet',
        components: 'V,S',
        material_description: '',
        material_cost_gp: '',
        material_consumed: false,
        duration: 'Instantaneous',
        concentration: false,
        ritual: false,
        description: '',
        higher_levels: '',
        resolution_mode: 'none',
        classes: '',
      };
    case 'weapons':
      return {
        id: '',
        name: '',
        damage: '1d6',
        damage_type: 'slashing',
        weight_lb: '',
        properties: '',
        range_normal_ft: '',
        range_long_ft: '',
        versatile_damage: '',
        weapon_type: 'simple-melee',
      };
    case 'magic-items':
      return {
        id: '',
        name: '',
        base_item_type: '',
        base_item_id: '',
        rarity: 'common',
        requires_attunement: false,
        attunement_restriction: '',
        magic_bonus: '',
        description: '',
      };
    case 'races':
      return {
        id: '',
        name: '',
        speed_ft: 30,
        size: 'Medium',
        ability_bonuses_json: '{}',
        darkvision_ft: 0,
        traits_json: '[]',
        languages: '',
      };
    case 'feats':
      return {
        id: '',
        name: '',
        description: '',
      };
    case 'classes':
      return {
        id: '',
        name: '',
        hit_die: 'd8',
        primary_ability: 'str',
        save_proficiencies: '',
        armor_proficiencies: '',
        weapon_proficiencies: '',
        features_by_level_json: '[]',
        attacks_per_action_json: '{"1":1}',
        subclass_level: 3,
        subclasses_json: '[]',
      };
    case 'class-features':
      return {
        id: '',
        class_id: '',
        class_name: '',
        feature_name: '',
        level: 1,
        description: '',
      };
    default:
      return { id: '', name: '' };
  }
}

/**
 * Convert a flat form model into the JSON payload accepted by
 * `/api/homebrew/<category>`. Throws if a required field is missing.
 * @param {string} category
 * @param {object} model
 */
export function buildHomebrewPayload(category, model) {
  if (!model) throw new Error('form model required');
  switch (category) {
    case 'creatures':
      return buildCreaturePayload(model);
    case 'spells':
      return buildSpellPayload(model);
    case 'weapons':
      return buildWeaponPayload(model);
    case 'magic-items':
      return buildMagicItemPayload(model);
    case 'races':
      return buildRacePayload(model);
    case 'feats':
      return buildFeatPayload(model);
    case 'classes':
      return buildClassPayload(model);
    case 'class-features':
      return buildClassFeaturePayload(model);
    default:
      throw new Error(`unknown category: ${category}`);
  }
}

function requireField(value, name) {
  const s = trimOrNull(value);
  if (s === null) throw new Error(`${name} is required`);
  return s;
}

function buildCreaturePayload(m) {
  return {
    id: requireField(m.id, 'id'),
    name: requireField(m.name, 'name'),
    size: requireField(m.size, 'size'),
    type: requireField(m.type, 'type'),
    alignment: trimOrNull(m.alignment),
    ac: intOrNull(m.ac) ?? 10,
    ac_type: trimOrNull(m.ac_type),
    hp_formula: requireField(m.hp_formula, 'hp_formula'),
    hp_average: intOrNull(m.hp_average) ?? 1,
    speed: parseJSONOrNull(m.speed_json, {}),
    ability_scores: parseJSONOrNull(m.ability_scores_json, {}),
    damage_resistances: csvToList(m.damage_resistances),
    damage_immunities: csvToList(m.damage_immunities),
    damage_vulnerabilities: csvToList(m.damage_vulnerabilities),
    condition_immunities: csvToList(m.condition_immunities),
    languages: csvToList(m.languages),
    cr: requireField(m.cr, 'cr'),
    attacks: parseJSONOrNull(m.attacks_json, []),
    homebrew: true,
  };
}

function buildSpellPayload(m) {
  return {
    id: requireField(m.id, 'id'),
    name: requireField(m.name, 'name'),
    level: intOrNull(m.level) ?? 0,
    school: requireField(m.school, 'school'),
    casting_time: requireField(m.casting_time, 'casting_time'),
    range_ft: intOrNull(m.range_ft),
    range_type: requireField(m.range_type, 'range_type'),
    components: csvToList(m.components),
    material_description: trimOrNull(m.material_description),
    material_cost_gp: floatOrNull(m.material_cost_gp),
    material_consumed: !!m.material_consumed,
    duration: requireField(m.duration, 'duration'),
    concentration: !!m.concentration,
    ritual: !!m.ritual,
    description: trimOrNull(m.description) ?? '',
    higher_levels: trimOrNull(m.higher_levels),
    resolution_mode: requireField(m.resolution_mode, 'resolution_mode'),
    classes: csvToList(m.classes),
    homebrew: true,
  };
}

function buildWeaponPayload(m) {
  return {
    id: requireField(m.id, 'id'),
    name: requireField(m.name, 'name'),
    damage: requireField(m.damage, 'damage'),
    damage_type: requireField(m.damage_type, 'damage_type'),
    weight_lb: floatOrNull(m.weight_lb),
    properties: csvToList(m.properties),
    range_normal_ft: intOrNull(m.range_normal_ft),
    range_long_ft: intOrNull(m.range_long_ft),
    versatile_damage: trimOrNull(m.versatile_damage),
    weapon_type: requireField(m.weapon_type, 'weapon_type'),
    homebrew: true,
  };
}

function buildMagicItemPayload(m) {
  return {
    id: requireField(m.id, 'id'),
    name: requireField(m.name, 'name'),
    base_item_type: trimOrNull(m.base_item_type),
    base_item_id: trimOrNull(m.base_item_id),
    rarity: requireField(m.rarity, 'rarity'),
    requires_attunement: !!m.requires_attunement,
    attunement_restriction: trimOrNull(m.attunement_restriction),
    magic_bonus: intOrNull(m.magic_bonus),
    description: trimOrNull(m.description) ?? '',
    homebrew: true,
  };
}

function buildRacePayload(m) {
  return {
    id: requireField(m.id, 'id'),
    name: requireField(m.name, 'name'),
    speed_ft: intOrNull(m.speed_ft) ?? 30,
    size: requireField(m.size, 'size'),
    ability_bonuses: parseJSONOrNull(m.ability_bonuses_json, {}),
    darkvision_ft: intOrNull(m.darkvision_ft) ?? 0,
    traits: parseJSONOrNull(m.traits_json, []),
    languages: csvToList(m.languages),
    homebrew: true,
  };
}

function buildFeatPayload(m) {
  return {
    id: requireField(m.id, 'id'),
    name: requireField(m.name, 'name'),
    description: trimOrNull(m.description) ?? '',
    homebrew: true,
  };
}

function buildClassPayload(m) {
  return {
    id: requireField(m.id, 'id'),
    name: requireField(m.name, 'name'),
    hit_die: requireField(m.hit_die, 'hit_die'),
    primary_ability: requireField(m.primary_ability, 'primary_ability'),
    save_proficiencies: csvToList(m.save_proficiencies),
    armor_proficiencies: csvToList(m.armor_proficiencies),
    weapon_proficiencies: csvToList(m.weapon_proficiencies),
    features_by_level: parseJSONOrNull(m.features_by_level_json, []),
    attacks_per_action: parseJSONOrNull(m.attacks_per_action_json, {}),
    subclass_level: intOrNull(m.subclass_level) ?? 3,
    subclasses: parseJSONOrNull(m.subclasses_json, []),
  };
}

/**
 * Build a class-feature-only homebrew payload as a minimal class skeleton.
 * The wire shape matches `/api/homebrew/classes` POST. The UI exposes only
 * the feature fields and we synthesize the surrounding class skeleton so a
 * single feature can live alongside SRD content without requiring a new
 * backend route. The class id encodes the parent class for traceability.
 */
function buildClassFeaturePayload(m) {
  const id = requireField(m.id, 'id');
  const featureName = requireField(m.feature_name, 'feature_name');
  const level = intOrNull(m.level) ?? 1;
  const description = trimOrNull(m.description) ?? '';
  const parentClassId = trimOrNull(m.class_id);
  const parentClassName = trimOrNull(m.class_name) ?? 'Custom Feature';
  return {
    id,
    name: `${parentClassName}: ${featureName}`,
    hit_die: 'd8',
    primary_ability: 'str',
    save_proficiencies: [],
    armor_proficiencies: [],
    weapon_proficiencies: [],
    features_by_level: [
      {
        level,
        name: featureName,
        description,
        parent_class_id: parentClassId,
        parent_class_name: parentClassName,
      },
    ],
    attacks_per_action: { 1: 1 },
    subclass_level: 3,
    subclasses: [],
  };
}

/**
 * Convert a backend entry (e.g. `refdata.Creature`) into the flat form
 * model for editing. Best-effort: unknown fields are left at the empty
 * defaults so the form still loads.
 * @param {string} category
 * @param {object} entry
 */
export function entryToFormModel(category, entry) {
  if (!entry) return emptyFormModel(category);
  const base = emptyFormModel(category);
  // Generic id/name fields exist on every category.
  if (entry.id) base.id = entry.id;
  if (entry.name) base.name = entry.name;
  switch (category) {
    case 'creatures':
      if (entry.size) base.size = entry.size;
      if (entry.type) base.type = entry.type;
      if (entry.alignment) base.alignment = entry.alignment;
      if (typeof entry.ac === 'number') base.ac = entry.ac;
      if (entry.ac_type) base.ac_type = entry.ac_type;
      if (entry.hp_formula) base.hp_formula = entry.hp_formula;
      if (typeof entry.hp_average === 'number') base.hp_average = entry.hp_average;
      if (entry.speed !== undefined) base.speed_json = JSON.stringify(entry.speed, null, 2);
      if (entry.ability_scores !== undefined)
        base.ability_scores_json = JSON.stringify(entry.ability_scores, null, 2);
      if (Array.isArray(entry.damage_resistances))
        base.damage_resistances = entry.damage_resistances.join(', ');
      if (Array.isArray(entry.damage_immunities))
        base.damage_immunities = entry.damage_immunities.join(', ');
      if (Array.isArray(entry.damage_vulnerabilities))
        base.damage_vulnerabilities = entry.damage_vulnerabilities.join(', ');
      if (Array.isArray(entry.condition_immunities))
        base.condition_immunities = entry.condition_immunities.join(', ');
      if (Array.isArray(entry.languages)) base.languages = entry.languages.join(', ');
      if (entry.cr !== undefined && entry.cr !== null) base.cr = String(entry.cr);
      if (entry.attacks !== undefined) base.attacks_json = JSON.stringify(entry.attacks, null, 2);
      break;
    case 'spells':
      if (typeof entry.level === 'number') base.level = entry.level;
      if (entry.school) base.school = entry.school;
      if (entry.casting_time) base.casting_time = entry.casting_time;
      if (entry.range_ft !== undefined && entry.range_ft !== null)
        base.range_ft = String(entry.range_ft);
      if (entry.range_type) base.range_type = entry.range_type;
      if (Array.isArray(entry.components)) base.components = entry.components.join(', ');
      if (entry.material_description) base.material_description = entry.material_description;
      if (entry.material_cost_gp !== undefined && entry.material_cost_gp !== null)
        base.material_cost_gp = String(entry.material_cost_gp);
      if (typeof entry.material_consumed === 'boolean')
        base.material_consumed = entry.material_consumed;
      if (entry.duration) base.duration = entry.duration;
      if (typeof entry.concentration === 'boolean') base.concentration = entry.concentration;
      if (typeof entry.ritual === 'boolean') base.ritual = entry.ritual;
      if (entry.description) base.description = entry.description;
      if (entry.higher_levels) base.higher_levels = entry.higher_levels;
      if (entry.resolution_mode) base.resolution_mode = entry.resolution_mode;
      if (Array.isArray(entry.classes)) base.classes = entry.classes.join(', ');
      break;
    case 'weapons':
      if (entry.damage) base.damage = entry.damage;
      if (entry.damage_type) base.damage_type = entry.damage_type;
      if (entry.weight_lb !== undefined && entry.weight_lb !== null)
        base.weight_lb = String(entry.weight_lb);
      if (Array.isArray(entry.properties)) base.properties = entry.properties.join(', ');
      if (entry.range_normal_ft !== undefined && entry.range_normal_ft !== null)
        base.range_normal_ft = String(entry.range_normal_ft);
      if (entry.range_long_ft !== undefined && entry.range_long_ft !== null)
        base.range_long_ft = String(entry.range_long_ft);
      if (entry.versatile_damage) base.versatile_damage = entry.versatile_damage;
      if (entry.weapon_type) base.weapon_type = entry.weapon_type;
      break;
    case 'magic-items':
      if (entry.base_item_type) base.base_item_type = entry.base_item_type;
      if (entry.base_item_id) base.base_item_id = entry.base_item_id;
      if (entry.rarity) base.rarity = entry.rarity;
      if (typeof entry.requires_attunement === 'boolean')
        base.requires_attunement = entry.requires_attunement;
      if (entry.attunement_restriction) base.attunement_restriction = entry.attunement_restriction;
      if (entry.magic_bonus !== undefined && entry.magic_bonus !== null)
        base.magic_bonus = String(entry.magic_bonus);
      if (entry.description) base.description = entry.description;
      break;
    case 'races':
      if (typeof entry.speed_ft === 'number') base.speed_ft = entry.speed_ft;
      if (entry.size) base.size = entry.size;
      if (entry.ability_bonuses !== undefined)
        base.ability_bonuses_json = JSON.stringify(entry.ability_bonuses, null, 2);
      if (typeof entry.darkvision_ft === 'number') base.darkvision_ft = entry.darkvision_ft;
      if (entry.traits !== undefined) base.traits_json = JSON.stringify(entry.traits, null, 2);
      if (Array.isArray(entry.languages)) base.languages = entry.languages.join(', ');
      break;
    case 'feats':
      if (entry.description) base.description = entry.description;
      break;
    case 'classes':
      if (entry.hit_die) base.hit_die = entry.hit_die;
      if (entry.primary_ability) base.primary_ability = entry.primary_ability;
      if (Array.isArray(entry.save_proficiencies))
        base.save_proficiencies = entry.save_proficiencies.join(', ');
      if (Array.isArray(entry.armor_proficiencies))
        base.armor_proficiencies = entry.armor_proficiencies.join(', ');
      if (Array.isArray(entry.weapon_proficiencies))
        base.weapon_proficiencies = entry.weapon_proficiencies.join(', ');
      if (entry.features_by_level !== undefined)
        base.features_by_level_json = JSON.stringify(entry.features_by_level, null, 2);
      if (entry.attacks_per_action !== undefined)
        base.attacks_per_action_json = JSON.stringify(entry.attacks_per_action, null, 2);
      if (typeof entry.subclass_level === 'number') base.subclass_level = entry.subclass_level;
      if (entry.subclasses !== undefined)
        base.subclasses_json = JSON.stringify(entry.subclasses, null, 2);
      break;
    case 'class-features': {
      // Read back a feature from the synthesized class skeleton.
      const feature = Array.isArray(entry.features_by_level) ? entry.features_by_level[0] : null;
      if (feature) {
        base.feature_name = feature.name || '';
        base.description = feature.description || '';
        if (typeof feature.level === 'number') base.level = feature.level;
        if (feature.parent_class_id) base.class_id = feature.parent_class_id;
        if (feature.parent_class_name) base.class_name = feature.parent_class_name;
      }
      break;
    }
  }
  return base;
}
