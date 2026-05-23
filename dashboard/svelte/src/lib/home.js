/**
 * Campaign Home data + action helpers.
 *
 * Kept in its own module so the Svelte HomePanel stays thin and the network
 * surface can be unit-tested without a DOM. Mirrors the JSON contract of the
 * Go HomeHandler in internal/dashboard/home.go.
 */

/**
 * Fetch the Campaign Home payload.
 *
 * @returns {Promise<{
 *   campaign_id: string,
 *   campaign_status: string,
 *   dm_queue_count: number,
 *   pending_approvals: number,
 *   active_encounters: string[],
 *   saved_encounters: string[],
 * }>}
 */
export async function getHomeData() {
  const res = await fetch('/api/dashboard/home');
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || `Request failed: ${res.status}`);
  }
  return res.json();
}

/**
 * Pause an active campaign.
 * @param {string} campaignId
 */
export async function pauseCampaign(campaignId) {
  return postStatusTransition(campaignId, 'pause');
}

/**
 * Resume a paused campaign.
 * @param {string} campaignId
 */
export async function resumeCampaign(campaignId) {
  return postStatusTransition(campaignId, 'resume');
}

/**
 * Flip the campaign status: pause when currently active, resume otherwise.
 * @param {string} campaignId
 * @param {string} currentStatus
 */
export async function toggleCampaignStatus(campaignId, currentStatus) {
  if (currentStatus === 'active') return pauseCampaign(campaignId);
  return resumeCampaign(campaignId);
}

/**
 * Mirror of the Go CampaignHomeData.PauseButtonLabel helper. Returns the label
 * the Pause/Resume button should display given the current campaign status.
 * @param {string | undefined} status
 */
export function pauseButtonLabel(status) {
  if (status === 'paused') return 'Resume Campaign';
  return 'Pause Campaign';
}

async function postStatusTransition(campaignId, action) {
  const res = await fetch(`/api/campaigns/${campaignId}/${action}`, { method: 'POST' });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || `Request failed: ${res.status}`);
  }
  return res.json();
}
