import { useEffect, useRef, useState } from 'react';
import Hls from 'hls.js';
import { AlertCircle } from 'lucide-react';

interface VideoPlayerProps {
  streamId: string;
  autoPlay?: boolean;
}

export function VideoPlayer({ streamId, autoPlay = true }: VideoPlayerProps) {
  const videoRef = useRef<HTMLVideoElement>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const video = videoRef.current;
    if (!video) return;

    // TODO: move to env or config
    const hlsUrl = `http://localhost:8080/stream/hls/${streamId}/index.m3u8`;
    
    let hls: Hls | null = null;

    if (Hls.isSupported()) {
      hls = new Hls({
        enableWorker: true,
        lowLatencyMode: true, // Optimizing for low latency
        backBufferLength: 90,
      });

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
              setError('Network error: unable to load stream');
              hls?.startLoad();
              break;
            case Hls.ErrorTypes.MEDIA_ERROR:
              setError('Media error: trying to recover');
              hls?.recoverMediaError();
              break;
            default:
              setError('Fatal error occurred');
              hls?.destroy();
              break;
          }
        }
      });
    } else if (video.canPlayType('application/vnd.apple.mpegurl')) {
      // For Safari native HLS support
      video.src = hlsUrl;
      video.addEventListener('loadedmetadata', () => {
        if (autoPlay) {
          video.play().catch(e => console.warn('AutoPlay failed:', e));
        }
      });
    } else {
      setError('HLS is not supported in this browser');
    }

    return () => {
      if (hls) {
        hls.destroy();
      }
    };
  }, [streamId, autoPlay]);

  return (
    <div className="video-container">
      {error ? (
        <div className="flex flex-col items-center justify-center h-full text-center p-4 text-[#ef4444] gap-2">
          <AlertCircle size={32} />
          <p className="text-sm font-medium">{error}</p>
        </div>
      ) : (
        <video
          ref={videoRef}
          className="video-player"
          controls
          muted // Muted by default to allow autoplay
          playsInline
        />
      )}
    </div>
  );
}
