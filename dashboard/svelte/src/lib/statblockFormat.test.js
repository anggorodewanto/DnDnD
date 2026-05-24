import { describe, it, expect } from 'vitest';
import {
  abilityModifier,
  formatModifier,
  formatSpeed,
  formatModifierMap,
  formatSenses,
  formatAttack,
} from './statblockFormat.js';

describe('abilityModifier', () => {
  it('computes the 5e ability modifier', () => {
    expect(abilityModifier(10)).toBe(0);
    expect(abilityModifier(11)).toBe(0);
    expect(abilityModifier(8)).toBe(-1);
    expect(abilityModifier(16)).toBe(3);
    expect(abilityModifier(20)).toBe(5);
    expect(abilityModifier(1)).toBe(-5);
  });
});

describe('formatModifier', () => {
  it('prefixes a sign', () => {
    expect(formatModifier(0)).toBe('+0');
    expect(formatModifier(3)).toBe('+3');
    expect(formatModifier(-2)).toBe('-2');
  });
});

describe('formatSpeed', () => {
  it('renders walk unlabeled and other modes labeled', () => {
    expect(formatSpeed({ walk: 30, fly: 60 })).toBe('30 ft., fly 60 ft.');
  });

  it('handles a single non-walk mode', () => {
    expect(formatSpeed({ swim: 40 })).toBe('swim 40 ft.');
  });

  it('returns empty string for missing/invalid input', () => {
    expect(formatSpeed(null)).toBe('');
    expect(formatSpeed(undefined)).toBe('');
    expect(formatSpeed({})).toBe('');
  });
});

describe('formatModifierMap', () => {
  it('capitalizes keys and signs values', () => {
    expect(formatModifierMap({ dex: 2, con: 4 })).toBe('Dex +2, Con +4');
  });

  it('honors a label override map', () => {
    expect(formatModifierMap({ str: 5 }, { str: 'STR' })).toBe('STR +5');
  });

  it('returns empty string for missing/invalid input', () => {
    expect(formatModifierMap(null)).toBe('');
    expect(formatModifierMap({})).toBe('');
  });
});

describe('formatSenses', () => {
  it('renders distance senses in feet and passive perception specially', () => {
    expect(formatSenses({ darkvision: 60, passive_perception: 14 })).toBe(
      'darkvision 60 ft., passive Perception 14',
    );
  });

  it('humanizes underscored sense names', () => {
    expect(formatSenses({ blindsight: 10, true_seeing: 30 })).toBe(
      'blindsight 10 ft., true seeing 30 ft.',
    );
  });

  it('returns empty string for missing/invalid input', () => {
    expect(formatSenses(null)).toBe('');
    expect(formatSenses({})).toBe('');
  });
});

describe('formatAttack', () => {
  it('builds a full melee attack sentence', () => {
    expect(
      formatAttack({ name: 'Scimitar', to_hit: 4, reach_ft: 5, damage: '1d6+2', damage_type: 'slashing' }),
    ).toBe('+4 to hit, reach 5 ft. Hit: 1d6+2 slashing damage.');
  });

  it('builds a ranged attack with range', () => {
    expect(formatAttack({ name: 'Shortbow', to_hit: 4, range_ft: 80, damage: '1d6+2', damage_type: 'piercing' })).toBe(
      '+4 to hit, range 80 ft. Hit: 1d6+2 piercing damage.',
    );
  });

  it('omits damage clause when no damage', () => {
    expect(formatAttack({ name: 'Shove', to_hit: 4, reach_ft: 5 })).toBe('+4 to hit, reach 5 ft.');
  });

  it('omits damage type when absent', () => {
    expect(formatAttack({ name: 'Slam', to_hit: 3, reach_ft: 5, damage: '1d4' })).toBe(
      '+3 to hit, reach 5 ft. Hit: 1d4 damage.',
    );
  });

  it('returns empty string for missing/invalid input', () => {
    expect(formatAttack(null)).toBe('');
    expect(formatAttack({})).toBe('');
  });
});
