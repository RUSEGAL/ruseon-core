import React, { useState, useEffect, useRef } from 'react';
import {
  Maximize,
  Minimize,
  Volume2,
  VolumeX,
  Camera as CameraIcon,
  Navigation,
  Eye,
  EyeOff,
  Info,
  Maximize2,
} from 'lucide-react';
import {
  globalPlayerOrchestrator,
} from '../../core/orchestrator';
import type { StreamingProtocol } from '../../core/orchestrator';
import { WebRTCPlayerV2 } from './WebRTCPlayerV2';
import { WebCodecsPlayerV2 } from './WebCodecsPlayerV2';
import { HlsPlayerV2 } from './HlsPlayerV2';
import { SnapshotPlayerV2 } from './SnapshotPlayerV2';
import { ProtocolSwitcher } from './ProtocolSwitcher';
import { PtzControlOverlay } from './PtzControlOverlay';
import { usePageVisibility } from '../../hooks/usePageVisibility';
import { AiOverlay } from '../../ai/AiOverlay';
import { useCameraAiDetection } from '../../hooks/useCameraAiDetection';

interface UniversalCameraPlayerProps {
  cameraId: string;
  cameraName?: string;
  codec?: string;
  sourceUrl?: string;
  isLive?: boolean;
  autoPlay?: boolean;
  isMaximized?: boolean;
  onOpenDetails?: () => void;
  onMaximizeToggle?: () => void;
  onFullscreenChange?: (isFullscreen: boolean) => void;
  onTimeUpdate?: (currentTime: number) => void;
}

export const UniversalCameraPlayer: React.FC<UniversalCameraPlayerProps> = ({
  cameraId,
  cameraName,
  codec,
  sourceUrl,
  isLive = true,
  autoPlay = true,
  isMaximized,
  onOpenDetails,
  onMaximizeToggle,
  onFullscreenChange,
  onTimeUpdate,
}) => {
  const containerRef = useRef<HTMLDivElement>(null);
  const isPageVisible = usePageVisibility();

  const [activeProtocol, setActiveProtocol] = useState<StreamingProtocol>(() => {
    return globalPlayerOrchestrator.getEffectiveProtocol(cameraId, codec);
  });

  const [muted, setMuted] = useState(true);
  const [isFullscreen, setIsFullscreen] = useState(false);
  const [showPtz, setShowPtz] = useState(false);
  const [showMetadata, setShowMetadata] = useState(false);

  useEffect(() => {
    const unsub = globalPlayerOrchestrator.subscribe((id, _from, to) => {
      if (id === cameraId) {
        setActiveProtocol(to);
      }
    });
    return unsub;
  }, [cameraId]);

  const effectiveProtocol: StreamingProtocol = !isPageVisible && isLive ? 'snapshot' : activeProtocol;

  const handleProtocolError = (err: string) => {
    globalPlayerOrchestrator.reportFailure(cameraId, activeProtocol, err, codec);
  };

  const handleConnected = () => {
    globalPlayerOrchestrator.reportHealthy(cameraId);
  };

  const toggleFullscreen = () => {
    if (!containerRef.current) return;
    if (!document.fullscreenElement) {
      containerRef.current.requestFullscreen().then(() => {
        setIsFullscreen(true);
        if (onFullscreenChange) onFullscreenChange(true);
      });
    } else {
      document.exitFullscreen().then(() => {
        setIsFullscreen(false);
        if (onFullscreenChange) onFullscreenChange(false);
      });
    }
  };

  const captureSnapshot = () => {
    const video = containerRef.current?.querySelector('video');
    if (!video) return;
    const canvas = document.createElement('canvas');
    canvas.width = video.videoWidth || 1280;
    canvas.height = video.videoHeight || 720;
    const ctx = canvas.getContext('2d');
    if (ctx) {
      ctx.drawImage(video, 0, 0, canvas.width, canvas.height);
      const link = document.createElement('a');
      link.download = `${cameraName || cameraId}_snapshot_${new Date().toISOString().replace(/[:.]/g, '-')}.jpg`;
      link.href = canvas.toDataURL('image/jpeg', 0.95);
      link.click();
    }
  };

  const { detections: aiDetections } = useCameraAiDetection({
    cameraId,
    enabled: showMetadata && isLive,
    getTargetElement: () => {
      if (!containerRef.current) return null;
      return containerRef.current.querySelector('[data-media-target="true"]');
    },
  });

  return (
    <div
      ref={containerRef}
      style={{
        width: '100%',
        height: '100%',
        position: 'relative',
        background: '#000',
        borderRadius: isFullscreen ? 0 : '8px',
        overflow: 'hidden',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
      }}
    >
      {effectiveProtocol === 'webrtc' && (
        <WebRTCPlayerV2
          streamId={cameraId}
          autoPlay={autoPlay}
          muted={muted}
          showMetadata={showMetadata}
          onError={handleProtocolError}
          onConnected={handleConnected}
        />
      )}

      {effectiveProtocol === 'webcodecs' && (
        <WebCodecsPlayerV2
          streamId={cameraId}
          autoPlay={autoPlay}
          muted={muted}
          showMetadata={showMetadata}
          onError={handleProtocolError}
          onConnected={handleConnected}
        />
      )}

      {effectiveProtocol === 'hls' && (
        <HlsPlayerV2
          streamId={cameraId}
          sourceUrl={sourceUrl}
          autoPlay={autoPlay}
          muted={muted}
          showMetadata={showMetadata}
          onError={handleProtocolError}
          onConnected={handleConnected}
          onTimeUpdate={onTimeUpdate}
        />
      )}

      {effectiveProtocol === 'snapshot' && (
        <SnapshotPlayerV2
          streamId={cameraId}
          onError={handleProtocolError}
        />
      )}

      {/* Futuristic AI Bounding Boxes Overlay (WebGPU Local Inference & Server Hybrid) */}
      <AiOverlay detections={aiDetections} visible={showMetadata} />

      {showPtz && (
        <PtzControlOverlay
          cameraId={cameraId}
          onClose={() => setShowPtz(false)}
        />
      )}

      {/* Top Header Overlay - Unified & Non-overlapping */}
      <div className="v2-camera-cell-header">
        {/* Left: Info button + Camera ID + Codec Badge */}
        <div style={{ display: 'flex', alignItems: 'center', gap: '6px', minWidth: 0, overflow: 'hidden' }}>
          {onOpenDetails && (
            <button
              onClick={(e) => {
                e.stopPropagation();
                onOpenDetails();
              }}
              style={{
                background: 'rgba(0, 0, 0, 0.65)',
                backdropFilter: 'blur(6px)',
                WebkitBackdropFilter: 'blur(6px)',
                border: '1px solid rgba(56, 189, 248, 0.3)',
                borderRadius: '5px',
                padding: '3px 5px',
                color: '#38bdf8',
                cursor: 'pointer',
                display: 'flex',
                alignItems: 'center',
                flexShrink: 0,
              }}
              title="View Telemetry & Details"
            >
              <Info size={12} />
            </button>
          )}

          <span
            style={{
              fontSize: '0.78rem',
              fontWeight: 600,
              color: '#f8fafc',
              textShadow: '0 1px 3px rgba(0,0,0,0.9)',
              whiteSpace: 'nowrap',
              overflow: 'hidden',
              textOverflow: 'ellipsis',
            }}
          >
            {cameraName || cameraId}
          </span>

          {codec && (
            <span
              style={{
                fontSize: '0.62rem',
                fontWeight: 700,
                background: 'rgba(255, 255, 255, 0.15)',
                padding: '2px 5px',
                borderRadius: '4px',
                textTransform: 'uppercase',
                color: '#cbd5e1',
                flexShrink: 0,
              }}
            >
              {codec}
            </span>
          )}
        </div>

        {/* Right: Protocol Switcher + Maximize in Grid Toggle */}
        <div style={{ display: 'flex', alignItems: 'center', gap: '6px', flexShrink: 0 }}>
          {isLive && (
            <ProtocolSwitcher
              cameraId={cameraId}
              activeProtocol={activeProtocol}
              cameraCodec={codec}
              onProtocolChange={(p) => setActiveProtocol(p)}
            />
          )}

          {onMaximizeToggle && (
            <button
              onClick={(e) => {
                e.stopPropagation();
                onMaximizeToggle();
              }}
              style={{
                background: 'rgba(0, 0, 0, 0.65)',
                backdropFilter: 'blur(6px)',
                WebkitBackdropFilter: 'blur(6px)',
                border: '1px solid rgba(255, 255, 255, 0.15)',
                borderRadius: '5px',
                padding: '3px 5px',
                color: '#cbd5e1',
                cursor: 'pointer',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
              }}
              title={isMaximized ? 'Restore Grid' : 'Maximize in Grid'}
            >
              <Maximize2 size={12} />
            </button>
          )}
        </div>
      </div>

      {/* Bottom Footer Controls Overlay */}
      <div className="v2-camera-cell-footer">
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
          <button
            onClick={() => setMuted(!muted)}
            style={{
              background: 'transparent',
              border: 'none',
              color: muted ? '#94a3b8' : '#38bdf8',
              cursor: 'pointer',
              padding: '3px',
            }}
            title={muted ? 'Unmute' : 'Mute'}
          >
            {muted ? <VolumeX size={15} /> : <Volume2 size={15} />}
          </button>

          <button
            onClick={() => setShowMetadata(!showMetadata)}
            style={{
              background: 'transparent',
              border: 'none',
              color: showMetadata ? '#10b981' : '#94a3b8',
              cursor: 'pointer',
              padding: '3px',
            }}
            title={showMetadata ? 'Hide AI Bounding Boxes' : 'Show AI Bounding Boxes'}
          >
            {showMetadata ? <Eye size={15} /> : <EyeOff size={15} />}
          </button>
        </div>

        <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
          {isLive && (
            <button
              onClick={() => setShowPtz(!showPtz)}
              style={{
                background: showPtz ? 'rgba(99,102,241,0.4)' : 'transparent',
                border: 'none',
                borderRadius: '4px',
                color: showPtz ? '#a5b4fc' : '#94a3b8',
                cursor: 'pointer',
                padding: '3px',
              }}
              title="Toggle PTZ Controls"
            >
              <Navigation size={15} />
            </button>
          )}

          <button
            onClick={captureSnapshot}
            style={{
              background: 'transparent',
              border: 'none',
              color: '#94a3b8',
              cursor: 'pointer',
              padding: '3px',
            }}
            title="Take Snapshot"
          >
            <CameraIcon size={15} />
          </button>

          <button
            onClick={toggleFullscreen}
            style={{
              background: 'transparent',
              border: 'none',
              color: '#94a3b8',
              cursor: 'pointer',
              padding: '3px',
            }}
            title={isFullscreen ? 'Exit Fullscreen' : 'Fullscreen'}
          >
            {isFullscreen ? <Minimize size={15} /> : <Maximize size={15} />}
          </button>
        </div>
      </div>
    </div>
  );
};
