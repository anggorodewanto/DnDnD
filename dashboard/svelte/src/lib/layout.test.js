import { describe, it, expect } from 'vitest';
import {
  MOBILE_MAX_WIDTH,
  isMobileViewport,
  mobileTabs,
  desktopOnlyViews,
  isDesktopOnly,
  mobileRedirectMessage,
  resolveViewForViewport,
} from './layout.js';

describe('MOBILE_MAX_WIDTH', () => {
  it('is 1024 pixels (anything below full desktop optimization of 1280)', () => {
    expect(MOBILE_MAX_WIDTH).toBe(1024);
  });
});

describe('isMobileViewport', () => {
  it('returns true when width is at or below the mobile threshold', () => {
    expect(isMobileViewport(1024)).toBe(true);
    expect(isMobileViewport(600)).toBe(true);
    expect(isMobileViewport(320)).toBe(true);
  });

  it('returns false on desktop widths', () => {
    expect(isMobileViewport(1025)).toBe(false);
    expect(isMobileViewport(1280)).toBe(false);
    expect(isMobileViewport(1920)).toBe(false);
  });

  it('returns false for invalid/zero width', () => {
    expect(isMobileViewport(0)).toBe(false);
    expect(isMobileViewport(undefined)).toBe(false);
    expect(isMobileViewport(null)).toBe(false);
    expect(isMobileViewport(-10)).toBe(false);
  });
});

describe('mobileTabs', () => {
  it('exposes the six mobile-lite destinations in display order', () => {
    expect(mobileTabs).toHaveLength(6);
    const ids = mobileTabs.map((t) => t.id);
    expect(ids).toEqual([
      'dm-queue',
      'turn-queue',
      'narrate',
      'approvals',
      'message-player',
      'quick-actions',
    ]);
  });

  it('each tab has id, label, and view fields', () => {
    for (const tab of mobileTabs) {
      expect(typeof tab.id).toBe('string');
      expect(typeof tab.label).toBe('string');
      expect(typeof tab.view).toBe('string');
      expect(tab.label.length).toBeGreaterThan(0);
    }
  });
});

describe('desktopOnlyViews', () => {
  it('includes the five desktop-only features from the spec', () => {
    expect(desktopOnlyViews).toContain('map-editor');
    expect(desktopOnlyViews).toContain('encounter-builder');
    expect(desktopOnlyViews).toContain('combat-workspace');
    expect(desktopOnlyViews).toContain('stat-block-library');
    expect(desktopOnlyViews).toContain('asset-library');
    expect(desktopOnlyViews).toHaveLength(5);
  });
});

describe('isDesktopOnly', () => {
  it('returns true for desktop-only views', () => {
    expect(isDesktopOnly('map-editor')).toBe(true);
    expect(isDesktopOnly('encounter-builder')).toBe(true);
    expect(isDesktopOnly('combat-workspace')).toBe(true);
    expect(isDesktopOnly('stat-block-library')).toBe(true);
    expect(isDesktopOnly('asset-library')).toBe(true);
  });

  it('returns false for mobile-eligible views', () => {
    expect(isDesktopOnly('dm-queue')).toBe(false);
    expect(isDesktopOnly('turn-queue')).toBe(false);
    expect(isDesktopOnly('narrate')).toBe(false);
    expect(isDesktopOnly('approvals')).toBe(false);
    expect(isDesktopOnly('message-player')).toBe(false);
    expect(isDesktopOnly('quick-actions')).toBe(false);
  });

  it('returns false for unknown views (defensive default)', () => {
    expect(isDesktopOnly('')).toBe(false);
    expect(isDesktopOnly(null)).toBe(false);
    expect(isDesktopOnly(undefined)).toBe(false);
    expect(isDesktopOnly('random-unknown-view')).toBe(false);
  });
});

describe('mobileRedirectMessage', () => {
  it('includes the feature label and the canonical desktop hint', () => {
    expect(mobileRedirectMessage('map-editor')).toBe(
      'Open the dashboard on desktop for Map Editor.',
    );
    expect(mobileRedirectMessage('encounter-builder')).toBe(
      'Open the dashboard on desktop for Encounter Builder.',
    );
    expect(mobileRedirectMessage('combat-workspace')).toBe(
      'Open the dashboard on desktop for Combat Workspace.',
    );
    expect(mobileRedirectMessage('stat-block-library')).toBe(
      'Open the dashboard on desktop for Stat Block Library.',
    );
    expect(mobileRedirectMessage('asset-library')).toBe(
      'Open the dashboard on desktop for Asset Library.',
    );
  });

  it('falls back to a generic message for unknown views', () => {
    expect(mobileRedirectMessage('unknown')).toBe(
      'Open the dashboard on desktop for this feature.',
    );
  });
});

describe('resolveViewForViewport', () => {
  it('returns the view unchanged on desktop', () => {
    expect(resolveViewForViewport({ view: 'map-editor', width: 1280 })).toEqual({
      view: 'map-editor',
      redirect: false,
    });
    expect(resolveViewForViewport({ view: 'dm-queue', width: 1920 })).toEqual({
      view: 'dm-queue',
      redirect: false,
    });
  });

  it('returns a redirect descriptor when mobile tries to open a desktop-only view', () => {
    expect(
      resolveViewForViewport({ view: 'map-editor', width: 600 }),
    ).toEqual({
      view: 'map-editor',
      redirect: true,
      message: 'Open the dashboard on desktop for Map Editor.',
    });
  });

  it('returns the view unchanged on mobile for mobile-eligible views', () => {
    expect(resolveViewForViewport({ view: 'dm-queue', width: 600 })).toEqual({
      view: 'dm-queue',
      redirect: false,
    });
  });
});
