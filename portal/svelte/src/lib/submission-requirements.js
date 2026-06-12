// Pure helpers for the portal character builder's Submit gate.
//
// `submissionRequirements` turns builder state into an ordered, human-readable
// checklist so the UI can both explain what's missing and disable Submit until
// every requirement is met. Keeping it pure (no Svelte, no DOM) makes it cheap
// to unit-test and reuse.

const ABILITIES = ['str', 'dex', 'con', 'int', 'wis', 'cha'];
const MIN_DICE = 4;

function nameMet(name) {
  return typeof name === 'string' && name.trim().length > 0;
}

function rolledAbility(rolls, ability) {
  const dice = rolls[ability];
  if (!Array.isArray(dice)) return false;
  return dice.length >= MIN_DICE;
}

function rollsMet(abilityRolls) {
  const rolls = abilityRolls ?? {};
  return ABILITIES.every((ability) => rolledAbility(rolls, ability));
}

/**
 * Compute the ordered submission-requirement checklist for the character
 * builder. Each entry is { key, label, met }. Submit is allowed only when
 * every requirement is met.
 * @param {{ name?: string, race?: string, selectedClass?: string,
 *           abilityMethod?: string, abilityRolls?: Record<string, number[]> }} state
 * @returns {{ key: string, label: string, met: boolean }[]}
 */
export function submissionRequirements(state) {
  const s = state ?? {};
  const requirements = [
    { key: 'name', label: 'Name your character', met: nameMet(s.name) },
    { key: 'race', label: 'Choose a race', met: Boolean(s.race) },
    { key: 'class', label: 'Choose a class', met: Boolean(s.selectedClass) },
  ];

  if (s.abilityMethod !== 'roll') return requirements;

  requirements.push({
    key: 'rolls',
    label: 'Roll your ability scores',
    met: rollsMet(s.abilityRolls),
  });
  return requirements;
}

/**
 * @param {{ met: boolean }[]} requirements
 * @returns {boolean} true when every requirement is met
 */
export function canSubmit(requirements) {
  return requirements.every((r) => r.met);
}
