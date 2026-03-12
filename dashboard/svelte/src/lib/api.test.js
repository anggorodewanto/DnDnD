import { describe, it, expect, vi, beforeEach } from 'vitest';
import { uploadAsset } from './api.js';

describe('uploadAsset', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('sends multipart form data and returns response', async () => {
    const mockResponse = { id: 'abc-123', url: '/api/assets/abc-123' };
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockResponse),
    });

    const file = new File(['fake-image-data'], 'map.png', { type: 'image/png' });
    const result = await uploadAsset({
      campaignId: 'campaign-uuid',
      type: 'map_background',
      file,
    });

    expect(result).toEqual(mockResponse);
    expect(fetch).toHaveBeenCalledTimes(1);

    const [url, options] = fetch.mock.calls[0];
    expect(url).toBe('/api/assets/upload');
    expect(options.method).toBe('POST');
    expect(options.body).toBeInstanceOf(FormData);

    const formData = options.body;
    expect(formData.get('campaign_id')).toBe('campaign-uuid');
    expect(formData.get('type')).toBe('map_background');
    expect(formData.get('file')).toBeInstanceOf(File);
    expect(formData.get('file').name).toBe('map.png');
  });

  it('throws on non-ok response', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 400,
      text: () => Promise.resolve('invalid asset type'),
    });

    const file = new File(['data'], 'bad.png', { type: 'image/png' });
    await expect(uploadAsset({
      campaignId: 'campaign-uuid',
      type: 'bogus',
      file,
    })).rejects.toThrow('invalid asset type');
  });
});
