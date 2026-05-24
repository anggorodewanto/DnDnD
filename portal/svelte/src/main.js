import { mount } from 'svelte';
import App from './App.svelte';

const target = document.getElementById('character-builder');
if (target) {
  // Read token and campaign ID from the hidden inputs before clearing them.
  const token = document.getElementById('portal-token')?.value || '';
  const campaignId = document.getElementById('campaign-id')?.value || '';

  // svelte's mount() appends to the target without clearing it, so the
  // server-rendered "Loading character builder..." placeholder would linger.
  // Wipe it first, then mount into the empty container.
  target.innerHTML = '';

  mount(App, {
    target,
    props: { token, campaignId },
  });
}
