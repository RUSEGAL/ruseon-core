/**
 * Client-side Reconnect Coordinator.
 * Prevents "Thundering Herd" when multiple cameras reconnect simultaneously (e.g. after network drop).
 * Queues reconnection requests and spreads them with jittered backoff.
 */

export interface ReconnectRequest {
  id: string;
  attempt: number;
  execute: () => Promise<void>;
  onSuccess?: () => void;
  onError?: (err: unknown) => void;
}

export class ReconnectCoordinator {
  private queue: Map<string, number> = new Map(); // id -> timeout handle
  private attempts: Map<string, number> = new Map(); // id -> count
  private maxConcurrency = 3;
  private runningCount = 0;
  private pendingExecutions: (() => void)[] = [];

  /**
   * Schedules a reconnect attempt with tiered backoff + jitter.
   */
  public schedule(
    id: string,
    execute: () => Promise<void>,
    options?: { baseDelayMs?: number; maxDelayMs?: number }
  ) {
    this.cancel(id);

    const currentAttempt = (this.attempts.get(id) || 0) + 1;
    this.attempts.set(id, currentAttempt);

    const baseDelay = options?.baseDelayMs ?? 1000;
    const maxDelay = options?.maxDelayMs ?? 30000;

    // Exponential backoff with 30% jitter
    const expDelay = Math.min(baseDelay * Math.pow(1.5, currentAttempt - 1), maxDelay);
    const jitter = expDelay * (0.7 + Math.random() * 0.6); // 0.7x to 1.3x

    const timer = window.setTimeout(() => {
      this.queue.delete(id);
      this.enqueueExecution(async () => {
        try {
          await execute();
          // Reset attempt on success
          this.attempts.delete(id);
        } catch (err) {
          // Failure will trigger another schedule by player
          console.warn(`[ReconnectCoordinator] Reconnect failed for camera ${id}:`, err);
        }
      });
    }, jitter);

    this.queue.set(id, timer);
  }

  public cancel(id: string) {
    const timer = this.queue.get(id);
    if (timer) {
      clearTimeout(timer);
      this.queue.delete(id);
    }
  }

  public reset(id: string) {
    this.cancel(id);
    this.attempts.delete(id);
  }

  private enqueueExecution(task: () => Promise<void>) {
    const run = async () => {
      this.runningCount++;
      try {
        await task();
      } finally {
        this.runningCount--;
        this.processNext();
      }
    };

    if (this.runningCount < this.maxConcurrency) {
      run();
    } else {
      this.pendingExecutions.push(run);
    }
  }

  private processNext() {
    if (this.runningCount < this.maxConcurrency && this.pendingExecutions.length > 0) {
      const next = this.pendingExecutions.shift();
      if (next) next();
    }
  }
}

export const globalReconnectCoordinator = new ReconnectCoordinator();
