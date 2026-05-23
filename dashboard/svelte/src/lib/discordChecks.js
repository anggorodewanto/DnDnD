/**
 * Discord startup self-check helpers.
 *
 * Mirrors the Go discordcheck.Report shape served by
 * GET /api/dashboard/discord-checks. The DiscordHealthBanner component calls
 * fetchDiscordChecks on mount and uses hasFailures to decide whether to
 * surface a warning strip at the top of the dashboard.
 */

/**
 * Fetch the latest Discord startup self-check report.
 *
 * @returns {Promise<{
 *   results: Array<{ name: string, ok: boolean, detail?: string }>,
 *   ran_at?: string,
 *   disabled?: boolean,
 * }>}
 */
export async function fetchDiscordChecks() {
  const res = await fetch('/api/dashboard/discord-checks');
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || `Request failed: ${res.status}`);
  }
  return res.json();
}

/**
 * Filter a report's results down to the failing entries. Returns an empty
 * array when the report is disabled or every check passed.
 *
 * @param {{ results?: Array<{ ok: boolean }>, disabled?: boolean }} report
 */
export function failingChecks(report) {
  if (!report) return [];
  if (report.disabled) return [];
  if (!Array.isArray(report.results)) return [];
  return report.results.filter((r) => r && r.ok === false);
}

/**
 * True when the report contains at least one failing check and is not
 * disabled. The banner uses this to decide whether to render at all.
 *
 * @param {{ results?: Array<{ ok: boolean }>, disabled?: boolean }} report
 */
export function hasFailures(report) {
  return failingChecks(report).length > 0;
}
