/**
 * Campaign Quick Actions (Phase 102 mobile-lite tab).
 *
 * Thin wrappers around the campaign pause/resume HTTP endpoints so the UI
 * component can stay declarative and these helpers can be unit-tested under
 * the node environment.
 */

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
 * Return the status a toggle would move the campaign into.
 * @param {string | undefined} current
 */
export function nextCampaignStatus(current) {
  if (current === 'active') return 'paused';
  return 'active';
}

/**
 * Toggle the campaign between active and paused based on its current status.
 * @param {string} campaignId
 * @param {string} currentStatus
 */
export async function toggleCampaignStatus(campaignId, currentStatus) {
  if (currentStatus === 'active') return pauseCampaign(campaignId);
  return resumeCampaign(campaignId);
}

async function postStatusTransition(campaignId, action) {
  const url = `/api/campaigns/${campaignId}/${action}`;
  const res = await fetch(url, { method: 'POST' });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || `Request failed: ${res.status}`);
  }
  return res.json();
}
