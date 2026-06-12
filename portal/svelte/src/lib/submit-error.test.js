import { describe, it, expect } from 'vitest';
import { humanizeSubmitError } from './submit-error.js';

const TOKEN_MSG =
  'Your character link has expired or already been used. Run /create-character in Discord to get a fresh link.';
const ROLL_MSG =
  'Re-roll your ability scores on the Ability Scores step, then submit again.';
const CAMPAIGN_MSG =
  'This character link is missing its campaign. Run /create-character in Discord to get a new link.';
const SERVER_MSG = 'Something went wrong on our end. Please try again in a moment.';

describe('humanizeSubmitError', () => {
  describe('token problems (precedence 1)', () => {
    it('maps any token-related message to the link-expired guidance', () => {
      expect(humanizeSubmitError('token is required')).toBe(TOKEN_MSG);
      expect(humanizeSubmitError('token expired')).toBe(TOKEN_MSG);
      expect(humanizeSubmitError('token already used')).toBe(TOKEN_MSG);
      expect(humanizeSubmitError('token does not belong to this user')).toBe(TOKEN_MSG);
      expect(humanizeSubmitError('token not found')).toBe(TOKEN_MSG);
    });

    it('is case-insensitive', () => {
      expect(humanizeSubmitError('TOKEN IS REQUIRED')).toBe(TOKEN_MSG);
      expect(humanizeSubmitError('Token Expired')).toBe(TOKEN_MSG);
    });

    it('wins over ability-roll problems (precedence 1 beats 2)', () => {
      expect(
        humanizeSubmitError(
          'validation failed: STR roll must include four d6 results; token is required',
        ),
      ).toBe(TOKEN_MSG);
    });
  });

  describe('ability-roll problems (precedence 2)', () => {
    it('matches the various roll-related phrasings', () => {
      expect(humanizeSubmitError('STR roll must include four d6 results')).toBe(ROLL_MSG);
      expect(humanizeSubmitError('rolled score is invalid')).toBe(ROLL_MSG);
      expect(humanizeSubmitError('non-d6 die detected')).toBe(ROLL_MSG);
      expect(humanizeSubmitError('expected 4d6 drop lowest')).toBe(ROLL_MSG);
      expect(humanizeSubmitError('roll contained a d6 mismatch')).toBe(ROLL_MSG);
    });

    it('is case-insensitive', () => {
      expect(humanizeSubmitError('STR Roll Must Include Four D6 Results')).toBe(ROLL_MSG);
    });

    it('does not match a stray "roll" without d6', () => {
      expect(humanizeSubmitError('payroll is required')).not.toBe(ROLL_MSG);
    });
  });

  describe('missing campaign (precedence 3)', () => {
    it('maps campaign-id and campaign messages to the campaign guidance', () => {
      expect(humanizeSubmitError('campaign_id is required')).toBe(CAMPAIGN_MSG);
      expect(humanizeSubmitError('campaign is required')).toBe(CAMPAIGN_MSG);
    });

    it('is case-insensitive', () => {
      expect(humanizeSubmitError('CAMPAIGN_ID IS REQUIRED')).toBe(CAMPAIGN_MSG);
    });
  });

  describe('server/internal errors (precedence 4)', () => {
    it('maps empty, undefined, and whitespace input to the server guidance', () => {
      expect(humanizeSubmitError('')).toBe(SERVER_MSG);
      expect(humanizeSubmitError(undefined)).toBe(SERVER_MSG);
      expect(humanizeSubmitError('   ')).toBe(SERVER_MSG);
    });

    it('maps explicit internal-server-error text', () => {
      expect(humanizeSubmitError('internal server error')).toBe(SERVER_MSG);
      expect(humanizeSubmitError('Internal Server Error')).toBe(SERVER_MSG);
    });

    it('maps 5xx-style failures', () => {
      expect(humanizeSubmitError('Request failed: 500')).toBe(SERVER_MSG);
      expect(humanizeSubmitError('Request failed: 503 Service Unavailable')).toBe(SERVER_MSG);
      expect(humanizeSubmitError('status 502')).toBe(SERVER_MSG);
    });
  });

  describe('generic validation (precedence 5)', () => {
    it('strips the prefix and joins the items', () => {
      expect(humanizeSubmitError('validation failed: name is required; race is required')).toBe(
        'Please fix: name is required; race is required',
      );
    });

    it('is case-insensitive on the prefix', () => {
      expect(humanizeSubmitError('Validation Failed: name is required')).toBe(
        'Please fix: name is required',
      );
    });

    it('trims items and drops empties', () => {
      expect(
        humanizeSubmitError('validation failed:  name is required ;  ; race is required '),
      ).toBe('Please fix: name is required; race is required');
    });

    it('does not append a trailing period', () => {
      const out = humanizeSubmitError('validation failed: name is required');
      expect(out.endsWith('.')).toBe(false);
    });
  });

  describe('fallback (precedence 6)', () => {
    it('returns the trimmed raw string unchanged', () => {
      expect(humanizeSubmitError('something unexpected happened')).toBe(
        'something unexpected happened',
      );
      expect(humanizeSubmitError('  trimmed me  ')).toBe('trimmed me');
    });
  });

  it('never returns an empty string', () => {
    const inputs = ['', '   ', undefined, 'token', 'validation failed: ', 'x'];
    for (const input of inputs) {
      expect(humanizeSubmitError(input).length).toBeGreaterThan(0);
    }
  });
});
