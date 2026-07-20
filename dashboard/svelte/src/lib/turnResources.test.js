import { describe, it, expect } from 'vitest';
import { toTurnResourcesPayload } from './turnResources.js';

describe('toTurnResourcesPayload', () => {
  it('omits every field left on "unchanged" so the backend leaves them alone', () => {
    const payload = toTurnResourcesPayload({
      actionUsed: '',
      bonusActionUsed: '',
      reactionUsed: '',
      movementRemainingFt: '',
      attacksRemaining: '',
      reason: 'mis-adjudicated grapple',
    });
    expect(payload).toEqual({ reason: 'mis-adjudicated grapple' });
  });

  it('maps the tri-state selects to real booleans', () => {
    const payload = toTurnResourcesPayload({
      actionUsed: 'false',
      bonusActionUsed: 'true',
      reactionUsed: 'false',
      reason: 'undo',
    });
    expect(payload).toEqual({
      action_used: false,
      bonus_action_used: true,
      reaction_used: false,
      reason: 'undo',
    });
  });

  it('coerces the numeric fields and keeps an explicit 0', () => {
    const payload = toTurnResourcesPayload({
      movementRemainingFt: '30',
      attacksRemaining: 0,
      reason: 'restrained',
    });
    expect(payload).toEqual({
      movement_remaining_ft: 30,
      attacks_remaining: 0,
      reason: 'restrained',
    });
  });

  it('drops unparseable numbers rather than sending NaN', () => {
    const payload = toTurnResourcesPayload({
      movementRemainingFt: 'abc',
      attacksRemaining: null,
      reason: 'typo',
    });
    expect(payload).toEqual({ reason: 'typo' });
  });

  it('trims the reason and still sends it when blank so the backend 400 is authoritative', () => {
    expect(toTurnResourcesPayload({ actionUsed: 'false', reason: '  spacing  ' }))
      .toEqual({ action_used: false, reason: 'spacing' });
    expect(toTurnResourcesPayload({ actionUsed: 'false' }))
      .toEqual({ action_used: false, reason: '' });
  });

  it('tolerates being called with no fields at all', () => {
    expect(toTurnResourcesPayload()).toEqual({ reason: '' });
  });
});
