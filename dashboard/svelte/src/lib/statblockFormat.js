/**
 * Pure formatting helpers for the human-readable stat block viewer
 * (StatBlockView.svelte). Kept separate from the component so they can be
 * unit-tested under the node test environment.
 */

/** 5e ability modifier: floor((score - 10) / 2). */
export function abilityModifier(score) {
  return Math.floor((Number(score) - 10) / 2);
}

/** Render a modifier with an explicit sign, e.g. 3 -> "+3", -2 -> "-2". */
export function formatModifier(mod) {
  const n = Number(mod);
  return n >= 0 ? `+${n}` : `${n}`;
}

function capitalize(s) {
  return s.charAt(0).toUpperCase() + s.slice(1);
}

/**
 * Format a speed map. Walk speed renders unlabeled ("30 ft."); every other
 * movement mode is labeled ("fly 60 ft."). Returns '' for missing input.
 */
export function formatSpeed(speed) {
  if (!speed || typeof speed !== 'object') return '';
  const parts = [];
  if (speed.walk != null) parts.push(`${speed.walk} ft.`);
  for (const [mode, value] of Object.entries(speed)) {
    if (mode === 'walk' || value == null) continue;
    parts.push(`${mode} ${value} ft.`);
  }
  return parts.join(', ');
}

/**
 * Format a {key: modifier} map (saving throws, skills) as "Key +N, Key +N".
 * `labels` optionally overrides the displayed key (e.g. {str: 'STR'}).
 */
export function formatModifierMap(map, labels = null) {
  if (!map || typeof map !== 'object') return '';
  return Object.entries(map)
    .map(([key, value]) => `${labels?.[key] ?? capitalize(key)} ${formatModifier(value)}`)
    .join(', ');
}

/**
 * Format a senses map. Distance senses render in feet; passive_perception is
 * rendered as "passive Perception N" without units. Underscores become spaces.
 */
export function formatSenses(senses) {
  if (!senses || typeof senses !== 'object') return '';
  const parts = [];
  for (const [sense, value] of Object.entries(senses)) {
    if (value == null) continue;
    if (sense === 'passive_perception') {
      parts.push(`passive Perception ${value}`);
      continue;
    }
    parts.push(`${sense.replace(/_/g, ' ')} ${value} ft.`);
  }
  return parts.join(', ');
}

/**
 * Build a 5e-style attack sentence from an attack entry
 * ({ to_hit, reach_ft, range_ft, damage, damage_type }). Returns '' for
 * missing input.
 */
export function formatAttack(attack) {
  if (!attack || typeof attack !== 'object') return '';
  const segments = [];
  if (attack.to_hit != null) segments.push(`${formatModifier(attack.to_hit)} to hit`);
  if (attack.reach_ft != null) segments.push(`reach ${attack.reach_ft} ft`);
  if (attack.range_ft != null) segments.push(`range ${attack.range_ft} ft`);
  const prefix = segments.join(', ');
  if (attack.damage != null) {
    const type = attack.damage_type ? ` ${attack.damage_type}` : '';
    const hit = `Hit: ${attack.damage}${type} damage.`;
    return prefix ? `${prefix}. ${hit}` : hit;
  }
  return prefix ? `${prefix}.` : '';
}
