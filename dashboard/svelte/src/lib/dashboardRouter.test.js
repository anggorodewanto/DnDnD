import { describe, expect, it } from 'vitest';
import { resolveDashboardViewFromHash } from './dashboardRouter.js';

describe('resolveDashboardViewFromHash', () => {
  it('routes sidebar hashes to their desktop views', () => {
    expect(resolveDashboardViewFromHash('#home')).toBe('home');
    expect(resolveDashboardViewFromHash('#approvals')).toBe('approvals');
    expect(resolveDashboardViewFromHash('#campaigns')).toBe('campaigns');
    expect(resolveDashboardViewFromHash('#encounters')).toBe('encounter-list');
    expect(resolveDashboardViewFromHash('#stat-block-library')).toBe('stat-block-library');
    expect(resolveDashboardViewFromHash('#list')).toBe('list');
    expect(resolveDashboardViewFromHash('#maps')).toBe('list');
    expect(resolveDashboardViewFromHash('#errors')).toBe('errors');
    expect(resolveDashboardViewFromHash('#exploration')).toBe('exploration');
    expect(resolveDashboardViewFromHash('#characters-new')).toBe('characters-new');
    expect(resolveDashboardViewFromHash('#dm-console')).toBe('dm-console');
    expect(resolveDashboardViewFromHash('#dm-queue')).toBe('dm-queue');
  });

  it('preserves the turn-builder hash prefix', () => {
    expect(resolveDashboardViewFromHash('#turn-builder:abc')).toBe('turn-builder');
  });

  it('defaults unknown or invalid hashes to the home view', () => {
    expect(resolveDashboardViewFromHash('#assets')).toBe('home');
    expect(resolveDashboardViewFromHash('')).toBe('home');
    expect(resolveDashboardViewFromHash(null)).toBe('home');
  });
});
