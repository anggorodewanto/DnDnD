import { mount } from 'svelte';
import CharacterBuilder from './CharacterBuilder.svelte';
import SpellPrep from './SpellPrep.svelte';

const target = document.getElementById('character-builder');
if (target) {
  // The prep page injects a hidden #prep-character-id; its presence selects the
  // spell-prep app. Otherwise this is the character builder.
  const prepCharacterId = document.getElementById('prep-character-id')?.value || '';

  // svelte's mount() appends without clearing, so wipe the server-rendered
  // placeholder first, then mount into the empty container.
  target.innerHTML = '';

  if (prepCharacterId) {
    mount(SpellPrep, {
      target,
      props: { characterId: prepCharacterId },
    });
  } else {
    const token = document.getElementById('portal-token')?.value || '';
    const campaignId = document.getElementById('campaign-id')?.value || '';
    mount(CharacterBuilder, {
      target,
      props: { mode: 'player', token, campaignId },
    });
  }
}
