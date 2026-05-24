<script>
  // Thin DM-dashboard wrapper around the shared character builder. The builder
  // lives in the portal Svelte source and is mode-aware; here we mount it in
  // "dm" mode. Its own ./lib/... imports resolve against the portal tree via
  // the @builder vite alias.
  import CharacterBuilder from '@builder/CharacterBuilder.svelte';

  let { campaignId = '' } = $props();
</script>

<CharacterBuilder mode="dm" {campaignId} />
