import { describe, expect, it } from 'vitest';
import { resolveDashboardViewFromHash } from './dashboardRouter.js';

describe('resolveDashboardViewFromHash', () => {
  it('routes sidebar hashes to their desktop views', () => {
    expect(resolveDashboardViewFromHash('#approvals')).toBe('approvals');
    expect(resolveDashboardViewFromHash('#campaigns')).toBe('campaigns');
    expect(resolveDashboardViewFromHash('#encounters')).toBe('encounter-list');
    expect(resolveDashboardViewFromHash('#stat-block-library')).toBe('stat-block-library');
    expect(resolveDashboardViewFromHash('#list')).toBe('list');
  });

  it('preserves the turn-builder hash prefix', () => {
    expect(resolveDashboardViewFromHash('#turn-builder:abc')).toBe('turn-builder');
  });

  it('defaults unknown or invalid hashes to the map list', () => {
    expect(resolveDashboardViewFromHash('#assets')).toBe('list');
    expect(resolveDashboardViewFromHash('')).toBe('list');
    expect(resolveDashboardViewFromHash(null)).toBe('list');
  });
});
