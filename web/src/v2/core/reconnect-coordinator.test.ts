import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { ReconnectCoordinator } from './reconnect-coordinator';

describe('ReconnectCoordinator', () => {
  let coordinator: ReconnectCoordinator;

  beforeEach(() => {
    vi.useFakeTimers();
    coordinator = new ReconnectCoordinator();
  });

  afterEach(() => {
    vi.restoreAllMocks();
    vi.useRealTimers();
  });

  it('schedules execution with backoff and executes callback', async () => {
    const mockTask = vi.fn().mockResolvedValue(undefined);

    coordinator.schedule('cam_1', mockTask, { baseDelayMs: 1000, maxDelayMs: 5000 });

    expect(mockTask).not.toHaveBeenCalled();

    // Fast-forward past max jittered base delay (1000 * 1.3 = 1300ms)
    await vi.advanceTimersByTimeAsync(2000);

    expect(mockTask).toHaveBeenCalledTimes(1);
  });

  it('cancels scheduled execution cleanly', async () => {
    const mockTask = vi.fn().mockResolvedValue(undefined);

    coordinator.schedule('cam_2', mockTask, { baseDelayMs: 1000 });
    coordinator.cancel('cam_2');

    await vi.advanceTimersByTimeAsync(5000);

    expect(mockTask).not.toHaveBeenCalled();
  });

  it('resets attempts counter on reset()', async () => {
    const mockTask = vi.fn().mockResolvedValue(undefined);

    coordinator.schedule('cam_3', mockTask);
    coordinator.reset('cam_3');

    await vi.advanceTimersByTimeAsync(5000);

    expect(mockTask).not.toHaveBeenCalled();
  });
});
