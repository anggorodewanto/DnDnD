/**
 * Responsive layout helpers for the DM Dashboard (Phase 102).
 *
 * The dashboard is fully optimized for 1280px+. Anything at or below 1024px
 * (a common tablet-landscape threshold) is considered mobile-lite and only
 * exposes the six "on-the-go" destinations enumerated in the spec.
 */

/** Maximum width (inclusive) that is treated as mobile-lite. */
export const MOBILE_MAX_WIDTH = 1024;

/**
 * Returns true when the given viewport width should render the mobile-lite
 * layout. Invalid or non-positive widths return false so server-side rendering
 * or unit tests default to the desktop experience.
 * @param {number | null | undefined} width
 */
export function isMobileViewport(width) {
  if (typeof width !== 'number') return false;
  if (!Number.isFinite(width)) return false;
  if (width <= 0) return false;
  return width <= MOBILE_MAX_WIDTH;
}

/**
 * Mobile-lite tab definitions. Order matches the bottom tab bar left-to-right.
 * Each entry: { id, label, view }. `view` is the internal view token used by
 * the App shell router.
 */
export const mobileTabs = Object.freeze([
  { id: 'dm-queue', label: 'DM Queue', view: 'dm-queue' },
  { id: 'turn-queue', label: 'Turns', view: 'turn-queue' },
  { id: 'narrate', label: 'Narrate', view: 'narrate' },
  { id: 'approvals', label: 'Approvals', view: 'approvals' },
  { id: 'message-player', label: 'Message', view: 'message-player' },
  { id: 'quick-actions', label: 'Quick', view: 'quick-actions' },
]);

/** Views that are desktop-only and trigger a redirect message on mobile. */
export const desktopOnlyViews = Object.freeze([
  'map-editor',
  'encounter-builder',
  'combat-workspace',
  'stat-block-library',
  'asset-library',
]);

/** Human-readable labels for each desktop-only view. */
const desktopOnlyLabels = Object.freeze({
  'map-editor': 'Map Editor',
  'encounter-builder': 'Encounter Builder',
  'combat-workspace': 'Combat Workspace',
  'stat-block-library': 'Stat Block Library',
  'asset-library': 'Asset Library',
});

/**
 * @param {string | null | undefined} view
 * @returns {boolean}
 */
export function isDesktopOnly(view) {
  if (!view) return false;
  return desktopOnlyViews.includes(view);
}

/**
 * Returns the redirect banner message for the given view. Falls back to a
 * generic message for unknown views so the UI never renders a blank.
 * @param {string} view
 */
export function mobileRedirectMessage(view) {
  const label = desktopOnlyLabels[view];
  if (!label) return 'Open the dashboard on desktop for this feature.';
  return `Open the dashboard on desktop for ${label}.`;
}

/**
 * Resolve the effective view for a given viewport width. When mobile tries to
 * open a desktop-only view, the resolver returns a redirect descriptor so the
 * shell can render the banner without changing the current view.
 * @param {{ view: string, width: number }} params
 * @returns {{ view: string, redirect: boolean, message?: string }}
 */
export function resolveViewForViewport({ view, width }) {
  if (!isMobileViewport(width)) return { view, redirect: false };
  if (!isDesktopOnly(view)) return { view, redirect: false };
  return { view, redirect: true, message: mobileRedirectMessage(view) };
}
