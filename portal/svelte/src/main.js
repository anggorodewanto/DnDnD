import { mount } from 'svelte';
import CharacterBuilder from './CharacterBuilder.svelte';
import SpellPrep from './SpellPrep.svelte';
import { bootstrap } from './bootstrap.js';

bootstrap(document, mount, { CharacterBuilder, SpellPrep });
