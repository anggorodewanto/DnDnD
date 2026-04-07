/**
 * Narration template helpers (Phase 100b).
 *
 * Mirrors internal/narration/template.go so the dashboard can extract and
 * substitute placeholder tokens entirely on the client.
 */

const PLACEHOLDER_RE = /\{([a-zA-Z_][a-zA-Z0-9_]*)\}/g;

/**
 * Return the unique placeholder names appearing in body in first-occurrence
 * order.
 *
 * @param {string|null|undefined} body
 * @returns {string[]}
 */
export function extractPlaceholders(body) {
  if (!body) return [];
  const seen = new Set();
  const out = [];
  for (const match of String(body).matchAll(PLACEHOLDER_RE)) {
    const name = match[1];
    if (seen.has(name)) continue;
    seen.add(name);
    out.push(name);
  }
  return out;
}

/**
 * Replace each `{name}` token in body with its value, leaving unknown tokens
 * untouched.
 *
 * @param {string|null|undefined} body
 * @param {Record<string,string>|null|undefined} values
 * @returns {string}
 */
export function substitutePlaceholders(body, values) {
  if (body == null) return '';
  if (!values || Object.keys(values).length === 0) return String(body);
  return String(body).replace(PLACEHOLDER_RE, (match, name) => {
    return Object.prototype.hasOwnProperty.call(values, name) ? values[name] : match;
  });
}
