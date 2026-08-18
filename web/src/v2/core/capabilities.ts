/**
 * Browser capability detector for video streaming protocols and codecs.
 * Probes WebRTC WHEP, WebCodecs (H.264, H.265), MSE / HLS, and hardware rendering.
 */

export interface BrowserCapabilities {
  hasWebRTC: boolean;
  hasWebCodecs: boolean;
  hasWebCodecsH264: boolean;
  hasWebCodecsH265: boolean;
  hasMSE: boolean;
  hasWebGPU: boolean;
  isSecureContext: boolean;
}

let cachedCapabilities: BrowserCapabilities | null = null;

export async function probeBrowserCapabilities(): Promise<BrowserCapabilities> {
  if (cachedCapabilities) {
    return cachedCapabilities;
  }

  const isSecure = window.isSecureContext || window.location.hostname === 'localhost' || window.location.hostname === '127.0.0.1';
  const hasWebRTC = typeof RTCPeerConnection !== 'undefined';
  const hasMSE = typeof window.MediaSource !== 'undefined' || typeof (window as unknown as { WebKitMediaSource: unknown }).WebKitMediaSource !== 'undefined';
  const hasWebGPU = typeof navigator !== 'undefined' && 'gpu' in navigator;

  let hasWebCodecs = false;
  let hasWebCodecsH264 = false;
  let hasWebCodecsH265 = false;

  if (typeof window !== 'undefined' && 'VideoDecoder' in window && isSecure) {
    hasWebCodecs = true;
    try {
      // Probe H.264 baseline/high
      const h264Support = await VideoDecoder.isConfigSupported({
        codec: 'avc1.640028', // H.264 High Profile Level 4.0
      });
      hasWebCodecsH264 = !!h264Support.supported;
    } catch {
      hasWebCodecsH264 = false;
    }

    try {
      // Probe H.265 Main Profile
      const h265Support = await VideoDecoder.isConfigSupported({
        codec: 'hev1.1.6.L93.B0', // H.265 Main Profile
      });
      hasWebCodecsH265 = !!h265Support.supported;
    } catch {
      hasWebCodecsH265 = false;
    }
  }

  cachedCapabilities = {
    hasWebRTC,
    hasWebCodecs,
    hasWebCodecsH264,
    hasWebCodecsH265,
    hasMSE,
    hasWebGPU: !!hasWebGPU,
    isSecureContext: isSecure,
  };

  return cachedCapabilities;
}

export function getCachedCapabilities(): BrowserCapabilities {
  if (cachedCapabilities) return cachedCapabilities;
  return {
    hasWebRTC: typeof RTCPeerConnection !== 'undefined',
    hasWebCodecs: false,
    hasWebCodecsH264: false,
    hasWebCodecsH265: false,
    hasMSE: true,
    hasWebGPU: false,
    isSecureContext: typeof window !== 'undefined' ? !!window.isSecureContext : true,
  };
}
