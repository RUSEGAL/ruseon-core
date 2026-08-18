import React, { useEffect, useRef, useState } from 'react';
import Hls from 'hls.js';
import { Loader2, WifiOff } from 'lucide-react';
import { V2MetadataOverlay } from './V2MetadataOverlay';
import type { MetadataPayload } from '../../../types';

interface HlsPlayerV2Props {
  streamId: string;
  sourceUrl?: string;
  autoPlay?: boolean;
  muted?: boolean;
  showMetadata?: boolean;
  onError?: (err: string) => void;
  onConnected?: () => void;
  onTimeUpdate?: (currentTime: number) => void;
}

export const HlsPlayerV2: React.FC<HlsPlayerV2Props> = ({
  streamId,
  sourceUrl,
  autoPlay = true,
  muted = true,
  showMetadata = false,
  onError,
  onConnected,
  onTimeUpdate,
}) => {
  const videoRef = useRef<HTMLVideoElement>(null);
  const hlsRef = useRef<Hls | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [localMetadata] = useState<MetadataPayload | null>(null);

  const onErrorRef = useRef(onError);
  const onConnectedRef = useRef(onConnected);
  const autoPlayRef = useRef(autoPlay);

  useEffect(() => {
    onErrorRef.current = onError;
    onConnectedRef.current = onConnected;
    autoPlayRef.current = autoPlay;
  });

  useEffect(() => {
    const video = videoRef.current;
    if (!video) return;

    if (hlsRef.current) {
      hlsRef.current.destroy();
      hlsRef.current = null;
    }

    setLoading(true);
    setError(null);

    const token = localStorage.getItem('token');
    const hlsUrl =
      sourceUrl ||
      `/stream/hls/${streamId}/index.m3u8${token ? `?token=${encodeURIComponent(token)}` : ''}`;

    if (Hls.isSupported()) {
      const hls = new Hls({
        enableWorker: true,
        lowLatencyMode: true,
        backBufferLength: 60,
        manifestLoadingTimeOut: 8000,
        manifestLoadingMaxRetry: 2,
        fragLoadingTimeOut: 8000,
        fragLoadingMaxRetry: 2,
      });
      hlsRef.current = hls;

      hls.loadSource(hlsUrl);
      hls.attachMedia(video);

      hls.on(Hls.Events.MANIFEST_PARSED, () => {
        setLoading(false);
        if (autoPlayRef.current) {
          video.play().catch(() => {});
        }
        if (onConnectedRef.current) onConnectedRef.current();
      });

      hls.on(Hls.Events.ERROR, (_event, data) => {
        if (data.fatal) {
          switch (data.type) {
            case Hls.ErrorTypes.NETWORK_ERROR:
              hls.startLoad();
              break;
            case Hls.ErrorTypes.MEDIA_ERROR:
              hls.recoverMediaError();
              break;
            default:
              const msg = `Fatal HLS Error: ${data.details}`;
              setError(msg);
              hls.destroy();
              if (onErrorRef.current) onErrorRef.current(msg);
              break;
          }
        }
      });
    } else if (video.canPlayType('application/vnd.apple.mpegurl')) {
      video.src = hlsUrl;
      video.addEventListener('loadedmetadata', () => {
        setLoading(false);
        if (autoPlayRef.current) video.play().catch(() => {});
        if (onConnectedRef.current) onConnectedRef.current();
      });
    } else {
      setError('HLS is not supported in this browser');
      if (onErrorRef.current) onErrorRef.current('HLS unsupported');
    }

    return () => {
      if (hlsRef.current) {
        hlsRef.current.destroy();
        hlsRef.current = null;
      }
    };
  }, [streamId, sourceUrl]);

  return (
    <div style={{ width: '100%', height: '100%', position: 'relative', background: '#000' }}>
      <video
        ref={videoRef}
        data-media-target="true"
        autoPlay={autoPlay}
        muted={muted}
        playsInline
        onTimeUpdate={(e) => {
          if (onTimeUpdate) {
            onTimeUpdate((e.target as HTMLVideoElement).currentTime);
          }
        }}
        style={{
          width: '100%',
          height: '100%',
          objectFit: 'contain',
          display: error ? 'none' : 'block',
        }}
      />

      {showMetadata && localMetadata && (
        <V2MetadataOverlay metadata={localMetadata} videoRef={videoRef} />
      )}

      {loading && !error && (
        <div
          style={{
            position: 'absolute',
            inset: 0,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            background: 'rgba(0,0,0,0.5)',
            gap: '8px',
            color: '#a5b4fc',
            fontSize: '0.85rem',
          }}
        >
          <Loader2 className="animate-spin" size={20} />
          <span>Buffering HLS...</span>
        </div>
      )}

      {error && (
        <div
          style={{
            position: 'absolute',
            inset: 0,
            display: 'flex',
            flexDirection: 'column',
            alignItems: 'center',
            justifyContent: 'center',
            gap: '8px',
            color: '#ef4444',
            fontSize: '0.82rem',
            padding: '1rem',
            textAlign: 'center',
          }}
        >
          <WifiOff size={24} />
          <span>{error}</span>
        </div>
      )}
    </div>
  );
};
