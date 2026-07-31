import { useEffect, useRef, useState } from 'react';
import Hls from 'hls.js';
import { Loader2, Maximize, Minimize, WifiOff } from 'lucide-react';

interface VideoPlayerProps {
  streamId: string;
  autoPlay?: boolean;
}

type PlayerStatus = 'loading' | 'playing' | 'reconnecting' | 'error';

export function VideoPlayer({ streamId, autoPlay = true }: VideoPlayerProps) {
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

    const hlsUrl = `/stream/hls/${streamId}/index.m3u8`;

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
    initPlayer();

    return () => {
      if (hlsRef.current) {
        hlsRef.current.destroy();
      }
      if (retryTimeoutRef.current) {
        clearTimeout(retryTimeoutRef.current);
      }
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
      className="video-container relative group overflow-hidden bg-black/90 w-full h-full flex items-center justify-center"
      ref={containerRef}
      onMouseEnter={() => setShowControls(true)}
      onMouseLeave={() => setShowControls(false)}
      style={{ position: 'relative' }}
    >
      <video
        ref={videoRef}
        className={`w-full h-full object-contain transition-opacity duration-300 ${status === 'reconnecting' || status === 'loading' ? 'opacity-50 blur-sm' : 'opacity-100'} ${status === 'error' ? 'hidden' : 'block'}`}
        muted
        playsInline
      />
      
      {/* State Overlays */}
      {status === 'loading' && (
        <div className="absolute inset-0 flex flex-col items-center justify-center text-white/80 z-10 pointer-events-none">
          <Loader2 className="animate-spin mb-2" size={36} />
          <span className="text-sm font-medium tracking-wide">Connecting...</span>
        </div>
      )}
      
      {status === 'reconnecting' && (
        <div className="absolute inset-0 flex flex-col items-center justify-center text-white/90 z-10 bg-black/40 pointer-events-none">
          <Loader2 className="animate-spin mb-3 text-[var(--primary)]" size={42} />
          <span className="text-base font-semibold tracking-wide">Reconnecting ({retryCount}/{maxRetries})</span>
          <span className="text-xs text-white/60 mt-1">Please wait, restoring signal...</span>
        </div>
      )}

      {status === 'error' && (
        <div className="absolute inset-0 flex flex-col items-center justify-center text-[#ef4444] z-10 bg-[url('data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHdpZHRoPSI0IiBoZWlnaHQ9IjQiPjxyZWN0IHdpZHRoPSI0IiBoZWlnaHQ9IjQiIGZpbGw9IiMxMTEiLz48cmVjdCB3aWR0aD0iMSIgaGVpZ2h0PSIxIiBmaWxsPSIjMzMzIi8+PC9zdmc+')] pointer-events-none">
          <div className="bg-black/60 p-6 rounded-xl flex flex-col items-center backdrop-blur-sm border border-white/10 shadow-2xl">
            <WifiOff size={48} className="mb-3 opacity-80" />
            <h3 className="text-lg font-bold text-white mb-1 uppercase tracking-wider">Signal Lost</h3>
            <p className="text-sm text-white/60 max-w-[200px] text-center">{errorMsg}</p>
          </div>
        </div>
      )}

      {/* Custom Controls UI (Hover) */}
      <div className={`absolute inset-0 pointer-events-none transition-opacity duration-300 z-20 ${showControls || status !== 'playing' ? 'opacity-100' : 'opacity-0'}`}>
        
        {/* Top-left LIVE badge */}
        {status === 'playing' && (
          <div className="absolute top-4 left-4 flex items-center gap-2 bg-black/50 backdrop-blur-md px-3 py-1.5 rounded-full border border-white/10">
            <div className="w-2 h-2 rounded-full bg-red-500 animate-pulse"></div>
            <span className="text-xs font-bold text-white tracking-widest uppercase">Live</span>
          </div>
        )}

        {/* Bottom-right Fullscreen */}
        {(status === 'playing' || status === 'reconnecting' || status === 'loading') && (
          <button 
            onClick={toggleFullscreen}
            className="pointer-events-auto absolute bottom-4 right-4 bg-black/50 hover:bg-black/80 backdrop-blur-md p-2.5 rounded-xl border border-white/10 text-white transition-all transform hover:scale-105 active:scale-95"
            title="Toggle Fullscreen"
          >
            {isFullscreen ? <Minimize size={20} /> : <Maximize size={20} />}
          </button>
        )}
      </div>
    </div>
  );
}
