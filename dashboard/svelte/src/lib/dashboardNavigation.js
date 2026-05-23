export const dashboardNavItems = Object.freeze([
  { id: 'home', label: 'Home', view: 'home', hash: '#home', group: ['home'] },
  { id: 'campaigns', label: 'Campaigns', view: 'campaigns', hash: '#campaigns', group: ['campaigns'] },
  { id: 'dashboard', label: 'Maps', view: 'list', hash: '#maps', group: ['list', 'editor'] },
  { id: 'combat', label: 'Combat', view: 'combat', hash: '#combat', group: ['combat'] },
  {
    id: 'encounters',
    label: 'Encounters',
    view: 'encounter-list',
    hash: '#encounters',
    group: ['encounter-list', 'encounter-editor'],
  },
  { id: 'turn-builder', label: 'Turn Builder', view: 'turn-builder', hash: '#turn-builder', group: ['turn-builder'] },
  {
    id: 'shops',
    label: 'Shops',
    view: 'shop-list',
    hash: '#shops',
    group: ['shop-list', 'shop-editor'],
  },
  { id: 'narrate', label: 'Narrate', view: 'narrate', hash: '#narrate', group: ['narrate'] },
  { id: 'homebrew', label: 'Homebrew', view: 'homebrew', hash: '#homebrew', group: ['homebrew'] },
  { id: 'party', label: 'Party', view: 'party', hash: '#party', group: ['party'] },
  {
    id: 'stat-block-library',
    label: 'Stat Blocks',
    view: 'stat-block-library',
    hash: '#stat-block-library',
    group: ['stat-block-library'],
  },
  {
    id: 'message-player',
    label: 'Message Player',
    view: 'message-player',
    hash: '#message-player',
    group: ['message-player'],
  },
  {
    id: 'open5e-sources',
    label: 'Open5e Sources',
    view: 'open5e-sources',
    hash: '#open5e-sources',
    group: ['open5e-sources'],
  },
  { id: 'dm-queue', label: 'DM Queue', view: 'dm-queue', hash: '#dm-queue', group: ['dm-queue'] },
  { id: 'approvals', label: 'Approvals', view: 'approvals', hash: '#approvals', group: ['approvals'] },
  { id: 'loot', label: 'Loot', view: 'loot', hash: '#loot', group: ['loot'] },
  { id: 'levelup', label: 'Level Up', view: 'levelup', hash: '#levelup', group: ['levelup'] },
  { id: 'characters-new', label: 'Create Character', view: 'characters-new', hash: '#characters-new', group: ['characters-new'] },
  { id: 'exploration', label: 'Exploration', view: 'exploration', hash: '#exploration', group: ['exploration'] },
  { id: 'errors', label: 'Errors', view: 'errors', hash: '#errors', group: ['errors'] },
]);

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
  homebrew: 'Homebrew Editor',
  party: 'Party Overview',
  'stat-block-library': 'Stat Block Library',
  'message-player': 'Message Player',
  'open5e-sources': 'Open5e Sources',
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
