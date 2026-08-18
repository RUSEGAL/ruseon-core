import { describe, it, expect, vi } from 'vitest';
import { PlayerOrchestrator, PROTOCOL_OPTIONS } from './orchestrator';

describe('PlayerOrchestrator', () => {
  it('exports valid protocol options list', () => {
    expect(PROTOCOL_OPTIONS.length).toBeGreaterThan(0);
    const auto = PROTOCOL_OPTIONS.find((p) => p.id === 'auto');
    expect(auto).toBeDefined();
    expect(auto?.label).toContain('Auto');
  });

  it('determines effective protocol and allows user override', () => {
    const orchestrator = new PlayerOrchestrator();

    const initial = orchestrator.getEffectiveProtocol('cam_test');
    expect(initial).toBeDefined();

    // Set user manual override
    orchestrator.setOverride('cam_test', 'hls');
    expect(orchestrator.getEffectiveProtocol('cam_test')).toBe('hls');

    // Reset to auto
    orchestrator.setOverride('cam_test', 'auto');
    expect(orchestrator.getEffectiveProtocol('cam_test')).toBeDefined();
  });

  it('subscribes and notifies on mode change', () => {
    const orchestrator = new PlayerOrchestrator();
    const listener = vi.fn();

    const unsubscribe = orchestrator.subscribe(listener);

    orchestrator.setOverride('cam_event', 'snapshot');
    expect(listener).toHaveBeenCalledWith(
      'cam_event',
      expect.anything(),
      'snapshot',
      'user_pinned'
    );

    unsubscribe();
    orchestrator.setOverride('cam_event', 'hls');
    expect(listener).toHaveBeenCalledTimes(1); // not called after unsubscribe
  });
});
