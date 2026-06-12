// Maps raw character-submission errors (server response bodies or network
// error `.message` strings) to a single player-friendly sentence with a clear
// next step. Pure logic, no I/O.

const TOKEN_MSG =
  'Your character link has expired or already been used. Run /create-character in Discord to get a fresh link.';
const ROLL_MSG =
  'Re-roll your ability scores on the Ability Scores step, then submit again.';
const CAMPAIGN_MSG =
  'This character link is missing its campaign. Run /create-character in Discord to get a new link.';
const SERVER_MSG =
  'Something went wrong on our end. Please try again in a moment.';

const ROLL_PHRASES = ['must include four d6', 'rolled score', 'non-d6', '4d6'];

function isRollError(lower) {
  if (ROLL_PHRASES.some((phrase) => lower.includes(phrase))) return true;
  return lower.includes('roll') && lower.includes('d6');
}

function isServerError(lower) {
  if (lower.includes('internal server error')) return true;
  return /(?:request failed:|status)\s*5\d{2}/.test(lower);
}

function genericValidation(trimmed) {
  const remainder = trimmed.slice('validation failed:'.length);
  const items = remainder
    .split(';')
    .map((item) => item.trim())
    .filter((item) => item.length > 0);
  if (items.length === 0) return SERVER_MSG;
  return 'Please fix: ' + items.join('; ');
}

/**
 * Convert a raw submission error (server body or network error message) into a
 * single player-friendly sentence with a clear next step. Never returns an
 * empty string.
 * @param {string} raw
 * @returns {string}
 */
export function humanizeSubmitError(raw) {
  const trimmed = (raw ?? '').toString().trim();
  if (trimmed.length === 0) return SERVER_MSG;

  const lower = trimmed.toLowerCase();

  if (lower.includes('token')) return TOKEN_MSG;
  if (isRollError(lower)) return ROLL_MSG;
  if (lower.includes('campaign_id is required') || lower.includes('campaign is required')) {
    return CAMPAIGN_MSG;
  }
  if (isServerError(lower)) return SERVER_MSG;
  if (lower.startsWith('validation failed:')) return genericValidation(trimmed);

  return trimmed;
}
