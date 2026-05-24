/**
 * PHB Background skill proficiencies.
 *
 * Each background grants two fixed skill proficiencies (per the PHB).
 * The builder uses this map to auto-add background skills onto the
 * character's skill list, deduped against any class-skill picks.
 *
 * IDs match the slugs used elsewhere in the portal (lowercase, kebab-case).
 */
export const BACKGROUND_SKILLS = {
  acolyte: ['insight', 'religion'],
  charlatan: ['deception', 'sleight-of-hand'],
  criminal: ['deception', 'stealth'],
  entertainer: ['acrobatics', 'performance'],
  'folk-hero': ['animal-handling', 'survival'],
  'guild-artisan': ['insight', 'persuasion'],
  hermit: ['medicine', 'religion'],
  noble: ['history', 'persuasion'],
  outlander: ['athletics', 'survival'],
  sage: ['arcana', 'history'],
  sailor: ['athletics', 'perception'],
  soldier: ['athletics', 'intimidation'],
  urchin: ['sleight-of-hand', 'stealth'],
};

/**
 * Structured PHB mechanical grants per background, beyond the two skills.
 *
 *   tools     — readable tool-proficiency labels ([] if none)
 *   languages — count of bonus languages of the player's choice
 *   feature   — { name, description }; descriptions are brief original
 *               paraphrases (NOT copied PHB/SRD prose) capturing the gist.
 *
 * Slugs mirror BACKGROUND_SKILLS exactly.
 */
export const BACKGROUND_DETAILS = {
  acolyte: {
    tools: [],
    languages: 2,
    feature: {
      name: 'Shelter of the Faithful',
      description: 'Temples of your faith give you free care and a place to rest.',
    },
  },
  charlatan: {
    tools: ['Disguise kit', 'Forgery kit'],
    languages: 0,
    feature: {
      name: 'False Identity',
      description: 'You maintain a convincing second identity with forged documents.',
    },
  },
  criminal: {
    tools: ["Thieves' tools", 'One gaming set'],
    languages: 0,
    feature: {
      name: 'Criminal Contact',
      description: 'You know a reliable contact who links you to the criminal network.',
    },
  },
  entertainer: {
    tools: ['Disguise kit', 'One musical instrument'],
    languages: 0,
    feature: {
      name: 'By Popular Demand',
      description: 'Your performances earn welcome, lodging, and food wherever you play.',
    },
  },
  'folk-hero': {
    tools: ["One type of artisan's tools", 'Vehicles (land)'],
    languages: 0,
    feature: {
      name: 'Rustic Hospitality',
      description: 'Common folk shelter and shield you as one of their own.',
    },
  },
  'guild-artisan': {
    tools: ["One type of artisan's tools"],
    languages: 1,
    feature: {
      name: 'Guild Membership',
      description: 'Your guild offers lodging, contacts, and political backing.',
    },
  },
  hermit: {
    tools: ['Herbalism kit'],
    languages: 1,
    feature: {
      name: 'Discovery',
      description: 'Your seclusion granted a unique, powerful secret or revelation.',
    },
  },
  noble: {
    tools: ['One gaming set'],
    languages: 1,
    feature: {
      name: 'Position of Privilege',
      description: 'Your noble standing earns deference, audiences, and easy welcome.',
    },
  },
  outlander: {
    tools: ['One musical instrument'],
    languages: 1,
    feature: {
      name: 'Wanderer',
      description: 'You recall terrain and can find food and water in the wild.',
    },
  },
  sage: {
    tools: [],
    languages: 2,
    feature: {
      name: 'Researcher',
      description: 'You know where or from whom to seek almost any lore.',
    },
  },
  sailor: {
    tools: ["Navigator's tools", 'Vehicles (water)'],
    languages: 0,
    feature: {
      name: "Ship's Passage",
      description: 'You can secure free passage aboard ships for yourself and allies.',
    },
  },
  soldier: {
    tools: ['One gaming set', 'Vehicles (land)'],
    languages: 0,
    feature: {
      name: 'Military Rank',
      description: 'Soldiers recognize your former rank and defer to its authority.',
    },
  },
  urchin: {
    tools: ['Disguise kit', "Thieves' tools"],
    languages: 0,
    feature: {
      name: 'City Secrets',
      description: 'You know hidden city paths and travel them at double speed.',
    },
  },
};

/**
 * Returns the canonical skill list for a background id, or an empty
 * array if the background is unknown.
 * @param {string} bg
 * @returns {string[]}
 */
export function skillsForBackground(bg) {
  if (!bg) return [];
  return BACKGROUND_SKILLS[bg] || [];
}

/**
 * Merges background skills into the player's skill picks, deduped.
 * Returns a new array (does not mutate the input).
 * @param {string[]} chosenSkills
 * @param {string} background
 * @returns {string[]}
 */
export function mergeBackgroundSkills(chosenSkills, background) {
  const bgSkills = skillsForBackground(background);
  if (bgSkills.length === 0) return [...(chosenSkills || [])];
  const merged = [...(chosenSkills || [])];
  for (const s of bgSkills) {
    if (!merged.includes(s)) merged.push(s);
  }
  return merged;
}

/**
 * Returns the full mechanical profile for a background id, merging skills
 * with the structured detail data, or null if the background is unknown.
 * @param {string} bg
 * @returns {{ skills: string[], tools: string[], languages: number, feature: { name: string, description: string } } | null}
 */
export function backgroundDetails(bg) {
  if (!bg) return null;
  const detail = BACKGROUND_DETAILS[bg];
  if (!detail) return null;
  return {
    skills: skillsForBackground(bg),
    tools: detail.tools,
    languages: detail.languages,
    feature: detail.feature,
  };
}

/**
 * Renders a bonus-language count as display text.
 * Returns '' for 0/falsy, the singular form for 1, and a counted plural
 * for higher values.
 * @param {number} n
 * @returns {string}
 */
export function formatLanguages(n) {
  if (!n) return '';
  if (n === 1) return 'One language of your choice';
  return `${n} languages of your choice`;
}
