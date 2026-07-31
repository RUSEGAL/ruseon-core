import { useEffect, useRef, useState } from 'react';
import Hls from 'hls.js';
import { Loader2, Maximize, Minimize, WifiOff } from 'lucide-react';

import type { HlsTelemetry } from '../types';

interface VideoPlayerProps {
  streamId: string;
  sourceUrl?: string; // Optional custom URL (e.g. for archive)
  autoPlay?: boolean;
  onTelemetryUpdate?: (stats: HlsTelemetry | null) => void;
  onTimeUpdate?: (time: number) => void;
}

type PlayerStatus = 'loading' | 'playing' | 'reconnecting' | 'error';

export function VideoPlayer({ streamId, sourceUrl, autoPlay = true, onTelemetryUpdate, onTimeUpdate }: VideoPlayerProps) {
  const videoRef = useRef<HTMLVideoElement>(null);
  const containerRef = useRef<HTMLDivElement>(null);
  
  const [status, setStatus] = useState<PlayerStatus>('loading');
  const [retryCount, setRetryCount] = useState(0);
  const [errorMsg, setErrorMsg] = useState<string | null>(null);
  const [isFullscreen, setIsFullscreen] = useState(false);
  const [showControls, setShowControls] = useState(false);

  const maxRetries = 5;
  const hlsRef = useRef<Hls | null>(null);
  const retryTimeoutRef = useRef<number | null>(null);

  const initPlayer = () => {
    const video = videoRef.current;
    if (!video) return;

    if (hlsRef.current) {
      hlsRef.current.destroy();
      hlsRef.current = null;
    }

    const hlsUrl = sourceUrl || `/stream/hls/${streamId}/index.m3u8`;

    if (Hls.isSupported()) {
      const hls = new Hls({
        enableWorker: true,
        lowLatencyMode: true,
        backBufferLength: 90,
        manifestLoadingTimeOut: 10000,
        manifestLoadingMaxRetry: 1, // Handle retries manually
        levelLoadingTimeOut: 10000,
        levelLoadingMaxRetry: 1,
        fragLoadingTimeOut: 10000,
        fragLoadingMaxRetry: 1,
      });
      hlsRef.current = hls;

      hls.loadSource(hlsUrl);
      hls.attachMedia(video);

      hls.on(Hls.Events.MANIFEST_PARSED, () => {
        if (autoPlay) {
          video.play().catch(e => console.warn('AutoPlay failed:', e));
        }
      });

      hls.on(Hls.Events.FRAG_CHANGED, () => {
        if (onTelemetryUpdate) {
          const v = videoRef.current;
          let dropped = 0;
          if (v && (v as any).webkitDroppedFrameCount !== undefined) {
             dropped = (v as any).webkitDroppedFrameCount;
          }
          
          let bufLen = 0;
          if (v && v.buffered.length > 0) {
             bufLen = v.buffered.end(v.buffered.length - 1) - v.currentTime;
          }
          if (bufLen < 0) bufLen = 0;

          onTelemetryUpdate({
            bandwidth: hls.bandwidthEstimate || 0,
            bufferLength: bufLen,
            latency: hls.latency || 0,
            droppedFrames: dropped
          });
        }
      });

      hls.on(Hls.Events.ERROR, (_event, data) => {
        if (data.fatal) {
          switch (data.type) {
            case Hls.ErrorTypes.NETWORK_ERROR:
              handleNetworkError(hls);
              break;
            case Hls.ErrorTypes.MEDIA_ERROR:
              console.warn('HLS Media Error, trying to recover');
              hls.recoverMediaError();
              break;
            default:
              failPlayer('Fatal error occurred');
              hls.destroy();
              break;
          }
        }
      });
    } else if (video.canPlayType('application/vnd.apple.mpegurl')) {
      video.src = hlsUrl;
      video.addEventListener('error', handleNativeError);
      video.addEventListener('loadedmetadata', () => {
        if (autoPlay) {
          video.play().catch(e => console.warn('AutoPlay failed:', e));
        }
      });
    } else {
      failPlayer('HLS is not supported in this browser');
    }
  };

  const handleNetworkError = (hls: Hls) => {
    setRetryCount(prev => {
      const nextCount = prev + 1;
      if (nextCount <= maxRetries) {
        setStatus('reconnecting');
        hls.destroy();
        retryTimeoutRef.current = window.setTimeout(() => {
          initPlayer();
        }, 2000);
        return nextCount;
      } else {
        failPlayer('Signal Lost');
        return prev;
      }
    });
  };

  const handleNativeError = () => {
    setRetryCount(prev => {
      const nextCount = prev + 1;
      if (nextCount <= maxRetries) {
        setStatus('reconnecting');
        retryTimeoutRef.current = window.setTimeout(() => {
          if (videoRef.current) {
            videoRef.current.src = `/stream/hls/${streamId}/index.m3u8?t=${Date.now()}`;
            videoRef.current.load();
          }
        }, 2000);
        return nextCount;
      } else {
        failPlayer('Signal Lost');
        return prev;
      }
    });
  };

  const failPlayer = (msg: string) => {
    setStatus('error');
    setErrorMsg(msg);
  };

  useEffect(() => {
    setStatus('loading');
    setRetryCount(0);
    setErrorMsg(null);
    if (onTelemetryUpdate) onTelemetryUpdate(null);
    initPlayer();

    return () => {
      if (hlsRef.current) {
        hlsRef.current.destroy();
      }
      if (retryTimeoutRef.current) {
        clearTimeout(retryTimeoutRef.current);
      }
      if (onTelemetryUpdate) onTelemetryUpdate(null);
    };
  }, [streamId, autoPlay]);

  useEffect(() => {
    const video = videoRef.current;
    if (!video) return;

    const handlePlaying = () => {
      setStatus('playing');
      setRetryCount(0);
    };

    const handleWaiting = () => {
      if (status === 'playing') {
        setStatus('loading');
      }
    };

    video.addEventListener('playing', handlePlaying);
    video.addEventListener('waiting', handleWaiting);

    return () => {
      video.removeEventListener('playing', handlePlaying);
      video.removeEventListener('waiting', handleWaiting);
      video.removeEventListener('error', handleNativeError);
    };
  }, [status]);

  const toggleFullscreen = (e: React.MouseEvent) => {
    e.stopPropagation();
    if (!containerRef.current) return;
    
    if (!document.fullscreenElement) {
      containerRef.current.requestFullscreen().catch(err => {
        console.warn(`Error attempting to enable fullscreen: ${err.message}`);
      });
    } else {
      document.exitFullscreen();
    }
  };

  useEffect(() => {
    const onFullscreenChange = () => {
      setIsFullscreen(!!document.fullscreenElement);
    };
    document.addEventListener('fullscreenchange', onFullscreenChange);
    return () => document.removeEventListener('fullscreenchange', onFullscreenChange);
  }, []);

  return (
    <div 
      className="video-container"
      ref={containerRef}
      onMouseEnter={() => setShowControls(true)}
      onMouseLeave={() => setShowControls(false)}
    >
      <video
        ref={videoRef}
        className={`video-player ${status === 'reconnecting' || status === 'loading' ? 'blurred' : ''} ${status === 'error' ? 'hidden' : ''}`}
        muted={autoPlay}
        playsInline
        onTimeUpdate={e => onTimeUpdate && onTimeUpdate((e.target as HTMLVideoElement).currentTime)}
      />
      
      {/* State Overlays */}
      {status === 'loading' && (
        <div className="player-overlay">
          <Loader2 className="spinner mb-2" size={36} />
          <span className="text-sm font-semibold tracking-wide text-white-80">Connecting...</span>
        </div>
      )}
      
      {status === 'reconnecting' && (
        <div className="player-overlay bg-dark">
          <Loader2 className="spinner mb-3 text-primary" size={42} />
          <span className="text-base font-semibold tracking-wide text-white-90">Reconnecting ({retryCount}/{maxRetries})</span>
          <span className="text-sm text-white-60 mt-1">Please wait, restoring signal...</span>
        </div>
      )}

      {status === 'error' && (
        <div className="player-overlay bg-error">
          <div className="error-box">
            <WifiOff size={48} className="error-icon text-danger" />
            <h3>Signal Lost</h3>
            <p>{errorMsg}</p>
          </div>
        </div>
      )}

      {/* Custom Controls UI (Hover) */}
      <div className={`player-controls ${showControls || status !== 'playing' ? 'visible' : 'hidden'}`}>
        
        {/* Top-left LIVE badge */}
        {status === 'playing' && (
          <div className="badge-live">
            <div className="dot"></div>
            <span>Live</span>
          </div>
        )}

        {/* Bottom-right Fullscreen */}
        {(status === 'playing' || status === 'reconnecting' || status === 'loading') && (
          <button 
            onClick={toggleFullscreen}
            className="btn-fullscreen"
            title="Toggle Fullscreen"
          >
            {isFullscreen ? <Minimize size={20} /> : <Maximize size={20} />}
          </button>
        )}
      </div>
    </div>
  );
}
