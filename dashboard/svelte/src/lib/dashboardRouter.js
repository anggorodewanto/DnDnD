const HASH_TO_VIEW = Object.freeze({
  '#home': 'home',
  '#combat': 'combat',
  '#campaigns': 'campaigns',
  '#errors': 'errors',
  '#exploration': 'exploration',
  '#characters-new': 'characters-new',
  '#encounters': 'encounter-list',
  '#encounter-new': 'encounter-editor',
  '#shops': 'shop-list',
  '#shop-new': 'shop-editor',
  '#narrate': 'narrate',
  '#homebrew': 'homebrew',
  '#party': 'party',
  '#stat-block-library': 'stat-block-library',
  '#message-player': 'message-player',
  '#open5e-sources': 'open5e-sources',
  '#dm-console': 'dm-console',
  '#dm-queue': 'dm-queue',
  '#loot': 'loot',
  '#levelup': 'levelup',
  '#approvals': 'approvals',
  '#list': 'list',
  '#maps': 'list',
});

/**
 * Resolves a dashboard SPA hash to the internal desktop view token.
 * Unknown hashes intentionally fall back to the map list to preserve the
 * dashboard's existing default.
 *
 * @param {string | null | undefined} hash
 * @returns {string}
 */
export function resolveDashboardViewFromHash(hash) {
  if (typeof hash !== 'string') return 'home';
  if (hash.startsWith('#turn-builder')) return 'turn-builder';
  return HASH_TO_VIEW[hash] || 'home';
}
