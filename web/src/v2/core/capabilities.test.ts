import { describe, it, expect } from 'vitest';
import { getCachedCapabilities, probeBrowserCapabilities } from './capabilities';

describe('capabilities', () => {
  it('returns valid cached capabilities with default fallback values', () => {
    const caps = getCachedCapabilities();
    expect(typeof caps.hasWebRTC).toBe('boolean');
    expect(typeof caps.hasMSE).toBe('boolean');
    expect(typeof caps.hasWebCodecs).toBe('boolean');
    expect(typeof caps.isSecureContext).toBe('boolean');
  });

  it('probes browser capabilities without throwing', async () => {
    const caps = await probeBrowserCapabilities();
    expect(caps).toBeDefined();
    expect(typeof caps.hasWebRTC).toBe('boolean');
    expect(typeof caps.hasWebGPU).toBe('boolean');
  });
});
