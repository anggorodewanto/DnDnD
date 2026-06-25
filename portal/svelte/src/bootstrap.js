// bootstrap decides which Svelte app to mount, and with what props, based on
// the hidden inputs the server template renders inside #character-builder.
//
// It is extracted from main.js (and parameterised on the document, the mount
// function, and the components) so the wiring can be unit-tested against a DOM
// without a real svelte runtime.
export function bootstrap(doc, mountFn, { CharacterBuilder, SpellPrep }) {
  const target = doc.getElementById('character-builder');
  if (!target) {
    return null;
  }

  // Read every hidden input BEFORE wiping the container: they are children of
  // #character-builder, so target.innerHTML = '' removes them from the DOM and
  // any later getElementById would return null (empty token/campaign_id -> a
  // 500 on submit).
  const prepCharacterId = doc.getElementById('prep-character-id')?.value || '';
  const token = doc.getElementById('portal-token')?.value || '';
  const campaignId = doc.getElementById('campaign-id')?.value || '';
  const editCharacterId = doc.getElementById('edit-character-id')?.value || '';

  // svelte's mount() appends without clearing, so wipe the server-rendered
  // placeholder first, then mount into the empty container.
  target.innerHTML = '';

  if (prepCharacterId) {
    return mountFn(SpellPrep, { target, props: { characterId: prepCharacterId } });
  }

  return mountFn(CharacterBuilder, {
    target,
    props: { mode: 'player', token, campaignId, editCharacterId },
  });
}
