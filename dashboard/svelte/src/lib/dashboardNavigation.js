export const dashboardNavItems = Object.freeze([
  // Campaign — top-level context the DM starts from.
  { id: 'home', label: 'Home', view: 'home', hash: '#home', section: 'Campaign', group: ['home'] },
  { id: 'campaigns', label: 'Campaigns', view: 'campaigns', hash: '#campaigns', section: 'Campaign', group: ['campaigns'] },
  { id: 'party', label: 'Party', view: 'party', hash: '#party', section: 'Campaign', group: ['party'] },

  // Prep — content built before the session.
  { id: 'dashboard', label: 'Maps', view: 'list', hash: '#maps', section: 'Prep', group: ['list', 'editor'] },
  {
    id: 'encounters',
    label: 'Encounters',
    view: 'encounter-list',
    hash: '#encounters',
    section: 'Prep',
    group: ['encounter-list', 'encounter-editor'],
  },
  {
    id: 'shops',
    label: 'Shops',
    view: 'shop-list',
    hash: '#shops',
    section: 'Prep',
    group: ['shop-list', 'shop-editor'],
  },
  { id: 'homebrew', label: 'Homebrew', view: 'homebrew', hash: '#homebrew', section: 'Prep', group: ['homebrew'] },
  {
    id: 'stat-block-library',
    label: 'Stat Blocks',
    view: 'stat-block-library',
    hash: '#stat-block-library',
    section: 'Prep',
    group: ['stat-block-library'],
  },
  { id: 'loot', label: 'Loot', view: 'loot', hash: '#loot', section: 'Prep', group: ['loot'] },
  {
    id: 'open5e-sources',
    label: 'Open5e Sources',
    view: 'open5e-sources',
    hash: '#open5e-sources',
    section: 'Prep',
    group: ['open5e-sources'],
  },

  // Run Session — live at the table.
  { id: 'combat', label: 'Combat', view: 'combat', hash: '#combat', section: 'Run Session', group: ['combat'] },
  { id: 'exploration', label: 'Exploration', view: 'exploration', hash: '#exploration', section: 'Run Session', group: ['exploration'] },
  { id: 'narrate', label: 'Narrate', view: 'narrate', hash: '#narrate', section: 'Run Session', group: ['narrate'] },
  { id: 'post-channel', label: 'Post to Channel', view: 'post-channel', hash: '#post-channel', section: 'Run Session', group: ['post-channel'] },
  { id: 'dm-console', label: 'DM Console', view: 'dm-console', hash: '#dm-console', section: 'Run Session', group: ['dm-console'] },
  { id: 'dm-queue', label: 'DM Queue', view: 'dm-queue', hash: '#dm-queue', section: 'Run Session', group: ['dm-queue'] },

  // Players — managing the people at the table.
  { id: 'characters-new', label: 'Create Character', view: 'characters-new', hash: '#characters-new', section: 'Players', group: ['characters-new'] },
  { id: 'levelup', label: 'Level Up', view: 'levelup', hash: '#levelup', section: 'Players', group: ['levelup'] },
  { id: 'approvals', label: 'Approvals', view: 'approvals', hash: '#approvals', section: 'Players', group: ['approvals'] },
  {
    id: 'message-player',
    label: 'Message Player',
    view: 'message-player',
    hash: '#message-player',
    section: 'Players',
    group: ['message-player'],
  },

  // System — operational tooling.
  { id: 'errors', label: 'Errors', view: 'errors', hash: '#errors', section: 'System', group: ['errors'] },
]);

/**
 * dashboardNavSections groups dashboardNavItems into ordered, titled sections by
 * walking the flat list and starting a new section whenever the `section` tag
 * changes. Order and item identity are preserved, so the sidebar render stays a
 * pure projection of dashboardNavItems.
 * @type {ReadonlyArray<{ title: string, items: typeof dashboardNavItems[number][] }>}
 */
export const dashboardNavSections = Object.freeze(
  dashboardNavItems.reduce((sections, item) => {
    const current = sections[sections.length - 1];
    if (current && current.title === item.section) {
      current.items.push(item);
      return sections;
    }
    sections.push({ title: item.section, items: [item] });
    return sections;
  }, []),
);

const VIEW_TITLES = Object.freeze({
  home: 'Campaign Home',
  combat: 'Combat Manager',
  campaigns: 'Campaigns',
  list: 'Maps',
  errors: 'Errors',
  exploration: 'Exploration',
  'characters-new': 'Create Character',
  editor: 'Map Editor',
  'encounter-list': 'Encounter Builder',
  'encounter-editor': 'Encounter Builder',
  'turn-builder': 'Turn Builder',
  'shop-list': 'Shops & Merchants',
  'shop-editor': 'Shops & Merchants',
  narrate: 'Narrate',
  'post-channel': 'Post to Channel',
  homebrew: 'Homebrew Editor',
  party: 'Party Overview',
  'stat-block-library': 'Stat Block Library',
  'message-player': 'Message Player',
  'open5e-sources': 'Open5e Sources',
  'dm-console': 'DM Console',
  'dm-queue': 'DM Queue',
  loot: 'Loot Pool',
  levelup: 'Level Up',
  approvals: 'Character Approvals',
});

/**
 * @param {string} view
 * @returns {string}
 */
export function dashboardViewTitle(view) {
  return VIEW_TITLES[view] || 'Dashboard';
}

/**
 * @param {{ group?: string[] }} item
 * @param {string} currentView
 * @returns {boolean}
 */
export function isDashboardNavItemActive(item, currentView) {
  return Array.isArray(item.group) && item.group.includes(currentView);
}
