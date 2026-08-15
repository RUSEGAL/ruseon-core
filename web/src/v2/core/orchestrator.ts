/**
 * Player Orchestrator State Machine.
 * Manages protocol candidate chains, automatic degrade/upgrade lifecycle,
 * and per-camera stream selection.
 */

import { getCachedCapabilities, probeBrowserCapabilities } from './capabilities';
import { globalReconnectCoordinator } from './reconnect-coordinator';

export type StreamingProtocol = 'webrtc' | 'webcodecs' | 'hls' | 'snapshot' | 'auto';

export interface ProtocolOption {
  id: StreamingProtocol;
  label: string;
  latency: string;
  description: string;
}

export const PROTOCOL_OPTIONS: ProtocolOption[] = [
  { id: 'auto', label: 'Auto (Adaptive)', latency: '< 200ms', description: 'Lowest latency with automatic fallback' },
  { id: 'webrtc', label: 'WebRTC (WHEP)', latency: '< 150ms', description: 'Ultra-low latency sub-second stream' },
  { id: 'webcodecs', label: 'WebCodecs', latency: '< 100ms', description: 'Hardware decoded canvas stream' },
  { id: 'hls', label: 'HLS (Low Latency)', latency: '1 - 3s', description: 'Standard HTTP Live Streaming with fMP4/TS' },
  { id: 'snapshot', label: 'Snapshot Polling', latency: '3 - 5s', description: 'Low power mode for background/unfocused view' },
];

export type ModeChangeCallback = (
  cameraId: string,
  fromMode: StreamingProtocol,
  toMode: StreamingProtocol,
  reason: string
) => void;

export class PlayerOrchestrator {
  private modeListeners: Set<ModeChangeCallback> = new Set();
  private overrides: Map<string, StreamingProtocol> = new Map();
  private currentActive: Map<string, StreamingProtocol> = new Map();
  private candidateChains: Map<string, StreamingProtocol[]> = new Map();

  constructor() {
    // Probe asynchronously on init
    probeBrowserCapabilities().catch(() => {});
  }

  public subscribe(cb: ModeChangeCallback): () => void {
    this.modeListeners.add(cb);
    return () => this.modeListeners.delete(cb);
  }

  public getEffectiveProtocol(cameraId: string, cameraCodec?: string): StreamingProtocol {
    const override = this.overrides.get(cameraId);
    if (override && override !== 'auto') {
      return override;
    }

    const current = this.currentActive.get(cameraId);
    if (current) return current;

    // Build initial chain
    const chain = this.buildCandidateChain(cameraCodec);
    this.candidateChains.set(cameraId, chain);
    const chosen = chain[0] || 'hls';
    this.currentActive.set(cameraId, chosen);
    return chosen;
  }

  public setOverride(cameraId: string, protocol: StreamingProtocol | null) {
    const prev = this.currentActive.get(cameraId) || 'auto';
    if (!protocol || protocol === 'auto') {
      this.overrides.delete(cameraId);
      const chain = this.candidateChains.get(cameraId) || this.buildCandidateChain();
      const next = chain[0] || 'hls';
      this.currentActive.set(cameraId, next);
      this.notify(cameraId, prev, next, 'user_auto');
    } else {
      this.overrides.set(cameraId, protocol);
      this.currentActive.set(cameraId, protocol);
      this.notify(cameraId, prev, protocol, 'user_pinned');
    }
  }

  public reportFailure(cameraId: string, failedProtocol: StreamingProtocol, errorMsg: string, cameraCodec?: string) {
    const override = this.overrides.get(cameraId);
    if (override && override !== 'auto') {
      // User pinned this protocol, do not auto-degrade, schedule reconnect via coordinator
      globalReconnectCoordinator.schedule(cameraId, async () => {});
      return;
    }

    let chain = this.candidateChains.get(cameraId);
    if (!chain || chain.length === 0) {
      chain = this.buildCandidateChain(cameraCodec);
      this.candidateChains.set(cameraId, chain);
    }

    const currentIndex = chain.indexOf(failedProtocol);
    if (currentIndex >= 0 && currentIndex < chain.length - 1) {
      const nextProtocol = chain[currentIndex + 1];
      this.currentActive.set(cameraId, nextProtocol);
      this.notify(cameraId, failedProtocol, nextProtocol, `degrade: ${errorMsg}`);
    } else {
      // Reached end of chain, loop back to first with delay
      const first = chain[0] || 'hls';
      this.currentActive.set(cameraId, first);
      this.notify(cameraId, failedProtocol, first, `reset_retry: ${errorMsg}`);
    }
  }

  public reportHealthy(cameraId: string) {
    globalReconnectCoordinator.reset(cameraId);
  }

  private buildCandidateChain(cameraCodec?: string): StreamingProtocol[] {
    const caps = getCachedCapabilities();
    const isH265 = (cameraCodec || '').toLowerCase().includes('265') || (cameraCodec || '').toLowerCase().includes('hevc');
    const chain: StreamingProtocol[] = [];

    if (caps.hasWebRTC) {
      if (!isH265 || caps.hasWebCodecsH265) {
        chain.push('webrtc');
      }
    }

    if (caps.hasWebCodecs) {
      if (!isH265 || caps.hasWebCodecsH265) {
        chain.push('webcodecs');
      }
    }

    // Universal robust fallback: HLS
    chain.push('hls');
    chain.push('snapshot');

    return chain;
  }

  private notify(cameraId: string, fromMode: StreamingProtocol, toMode: StreamingProtocol, reason: string) {
    this.modeListeners.forEach((cb) => {
      try {
        cb(cameraId, fromMode, toMode, reason);
      } catch (err) {
        console.error('[PlayerOrchestrator] Listener error:', err);
      }
    });
  }
}

export const globalPlayerOrchestrator = new PlayerOrchestrator();
