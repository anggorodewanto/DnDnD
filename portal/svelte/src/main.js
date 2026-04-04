import { mount } from 'svelte';
import App from './App.svelte';

const target = document.getElementById('character-builder');
if (target) {
  // Read token and campaign ID from hidden inputs
  const tokenEl = document.getElementById('portal-token');
  const campaignEl = document.getElementById('campaign-id');

  mount(App, {
    target,
    props: {
      token: tokenEl?.value || '',
      campaignId: campaignEl?.value || '',
    },
  });
}
