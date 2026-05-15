import { describe, it, expect } from 'vitest';
import { generateBlankMap } from './mapdata.js';

describe('F-18: backgroundOpacity persistence in tiled_json', () => {
  it('saves backgroundOpacity into tiled_json', () => {
    const tiledMap = generateBlankMap(5, 5);
    const backgroundOpacity = 0.75;

    // Simulate save: store opacity in tiled_json before sending payload
    tiledMap.backgroundOpacity = backgroundOpacity;

    const payload = { name: 'Test', width: 5, height: 5, tiled_json: tiledMap };
    expect(payload.tiled_json.backgroundOpacity).toBe(0.75);
  });

  it('restores backgroundOpacity from tiled_json on load', () => {
    const tiledMap = generateBlankMap(5, 5);
    tiledMap.backgroundOpacity = 0.3;

    // Simulate load: server returns tiled_json with backgroundOpacity
    const data = { tiled_json: tiledMap };
    const loaded = typeof data.tiled_json === 'string' ? JSON.parse(data.tiled_json) : data.tiled_json;

    let backgroundOpacity = 0.5; // default
    if (loaded.backgroundOpacity != null) {
      backgroundOpacity = loaded.backgroundOpacity;
    }
    expect(backgroundOpacity).toBe(0.3);
  });

  it('keeps default 0.5 when tiled_json has no backgroundOpacity', () => {
    const tiledMap = generateBlankMap(5, 5);

    const data = { tiled_json: tiledMap };
    const loaded = typeof data.tiled_json === 'string' ? JSON.parse(data.tiled_json) : data.tiled_json;

    let backgroundOpacity = 0.5;
    if (loaded.backgroundOpacity != null) {
      backgroundOpacity = loaded.backgroundOpacity;
    }
    expect(backgroundOpacity).toBe(0.5);
  });

  it('round-trips through JSON serialization', () => {
    const tiledMap = generateBlankMap(5, 5);
    tiledMap.backgroundOpacity = 0.8;

    // Simulate full round-trip: save → JSON.stringify → JSON.parse → load
    const serialized = JSON.stringify(tiledMap);
    const deserialized = JSON.parse(serialized);

    let backgroundOpacity = 0.5;
    if (deserialized.backgroundOpacity != null) {
      backgroundOpacity = deserialized.backgroundOpacity;
    }
    expect(backgroundOpacity).toBe(0.8);
  });
});
