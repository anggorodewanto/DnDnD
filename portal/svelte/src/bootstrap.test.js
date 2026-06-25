// @vitest-environment jsdom
import { describe, it, expect, vi } from 'vitest';
import { bootstrap } from './bootstrap.js';

const CharacterBuilder = { name: 'CharacterBuilder' };
const SpellPrep = { name: 'SpellPrep' };

function render(html) {
  document.body.innerHTML = html;
}

describe('bootstrap', () => {
  it('passes the token and campaign_id hidden inputs through to CharacterBuilder', () => {
    // The hidden inputs live *inside* #character-builder, which bootstrap
    // wipes before mounting — so it must read them first or they come back
    // empty (the 500-on-submit regression).
    render(`
      <div id="character-builder">
        <p>Loading character builder...</p>
        <input type="hidden" id="portal-token" value="tok-abc">
        <input type="hidden" id="campaign-id" value="camp-123">
      </div>
    `);
    const mountFn = vi.fn();

    bootstrap(document, mountFn, { CharacterBuilder, SpellPrep });

    expect(mountFn).toHaveBeenCalledTimes(1);
    const [component, options] = mountFn.mock.calls[0];
    expect(component).toBe(CharacterBuilder);
    expect(options.props).toEqual({
      mode: 'player',
      token: 'tok-abc',
      campaignId: 'camp-123',
      editCharacterId: '',
    });
  });

  it('passes edit-character-id through for the edit flow', () => {
    render(`
      <div id="character-builder">
        <input type="hidden" id="portal-token" value="">
        <input type="hidden" id="campaign-id" value="">
        <input type="hidden" id="edit-character-id" value="char-456">
      </div>
    `);
    const mountFn = vi.fn();

    bootstrap(document, mountFn, { CharacterBuilder, SpellPrep });

    const [component, options] = mountFn.mock.calls[0];
    expect(component).toBe(CharacterBuilder);
    expect(options.props.editCharacterId).toBe('char-456');
  });

  it('mounts SpellPrep with the character id when a prep input is present', () => {
    render(`
      <div id="character-builder">
        <input type="hidden" id="prep-character-id" value="char-789">
        <input type="hidden" id="portal-token" value="tok-abc">
        <input type="hidden" id="campaign-id" value="camp-123">
      </div>
    `);
    const mountFn = vi.fn();

    bootstrap(document, mountFn, { CharacterBuilder, SpellPrep });

    expect(mountFn).toHaveBeenCalledTimes(1);
    const [component, options] = mountFn.mock.calls[0];
    expect(component).toBe(SpellPrep);
    expect(options.props).toEqual({ characterId: 'char-789' });
  });

  it('does nothing when the builder container is absent', () => {
    render('<div>no builder here</div>');
    const mountFn = vi.fn();

    const result = bootstrap(document, mountFn, { CharacterBuilder, SpellPrep });

    expect(result).toBeNull();
    expect(mountFn).not.toHaveBeenCalled();
  });
});
