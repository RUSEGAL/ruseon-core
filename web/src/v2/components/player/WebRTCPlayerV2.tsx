import React, { useEffect, useRef, useState } from 'react';
import { Loader2, WifiOff } from 'lucide-react';
import { V2MetadataOverlay } from './V2MetadataOverlay';
import type { MetadataPayload } from '../../../types';

interface WebRTCPlayerV2Props {
  streamId: string;
  autoPlay?: boolean;
  muted?: boolean;
  showMetadata?: boolean;
  onError?: (err: string) => void;
  onConnected?: () => void;
}

export const WebRTCPlayerV2: React.FC<WebRTCPlayerV2Props> = ({
  streamId,
  autoPlay = true,
  muted = true,
  showMetadata = false,
  onError,
  onConnected,
}) => {
  const videoRef = useRef<HTMLVideoElement>(null);
  const pcRef = useRef<RTCPeerConnection | null>(null);
  const dataChannelRef = useRef<RTCDataChannel | null>(null);
  const clearMetadataTimeout = useRef<number | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [localMetadata, setLocalMetadata] = useState<MetadataPayload | null>(null);

  useEffect(() => {
    let isCancelled = false;
    setLoading(true);
    setError(null);
    setLocalMetadata(null);

    const startWhep = async () => {
      try {
        const pc = new RTCPeerConnection({
          iceServers: [{ urls: 'stun:stun.l.google.com:19302' }],
        });
        pcRef.current = pc;

        pc.addTransceiver('video', { direction: 'recvonly' });
        pc.addTransceiver('audio', { direction: 'recvonly' });

        // CRITICAL: The client creates the DataChannel BEFORE creating the SDP offer
        const dc = pc.createDataChannel('metadata', { ordered: false, maxRetransmits: 0 });
        dataChannelRef.current = dc;

        dc.onopen = () => {
          console.log(`[WebRTC] Metadata DataChannel opened for stream ${streamId}`);
        };

        dc.onmessage = (msgEvent) => {
          try {
            const parsed = JSON.parse(msgEvent.data);
            setLocalMetadata(parsed);

            if (clearMetadataTimeout.current) {
              window.clearTimeout(clearMetadataTimeout.current);
            }
            // Auto-clear boxes if no new frame arrives in 1s
            clearMetadataTimeout.current = window.setTimeout(() => {
              setLocalMetadata(null);
            }, 1000);
          } catch {
            // Non-JSON packet
          }
        };

        pc.ontrack = (event) => {
          if (videoRef.current && event.streams[0]) {
            videoRef.current.srcObject = event.streams[0];
            setLoading(false);
            if (onConnected) onConnected();
          }
        };

        pc.oniceconnectionstatechange = () => {
          if (pc.iceConnectionState === 'failed' || pc.iceConnectionState === 'disconnected') {
            if (!isCancelled) {
              const errMsg = `WebRTC ICE ${pc.iceConnectionState}`;
              setError(errMsg);
              if (onError) onError(errMsg);
            }
          }
        };

        const offer = await pc.createOffer();
        await pc.setLocalDescription(offer);

        const token = localStorage.getItem('token');
        const whepUrl = `/stream/webrtc/whep/${streamId}`;
        const headers: Record<string, string> = {
          'Content-Type': 'application/sdp',
        };
        if (token) {
          headers['Authorization'] = `Bearer ${token}`;
        }

        const res = await fetch(whepUrl, {
          method: 'POST',
          headers,
          body: offer.sdp,
        });

        if (!res.ok) {
          throw new Error(`WHEP endpoint returned ${res.status}`);
        }

        const answerSdp = await res.text();
        if (isCancelled) return;

        await pc.setRemoteDescription(
          new RTCSessionDescription({
            type: 'answer',
            sdp: answerSdp,
          })
        );
      } catch (err: unknown) {
        if (!isCancelled) {
          const msg = err instanceof Error ? err.message : 'WebRTC connection failed';
          setError(msg);
          setLoading(false);
          if (onError) onError(msg);
        }
      }
    };

    startWhep();

    return () => {
      isCancelled = true;
      if (clearMetadataTimeout.current) {
        window.clearTimeout(clearMetadataTimeout.current);
      }
      if (dataChannelRef.current) {
        dataChannelRef.current.close();
        dataChannelRef.current = null;
      }
      if (pcRef.current) {
        pcRef.current.close();
        pcRef.current = null;
      }
    };
  }, [streamId]);

  return (
    <div style={{ width: '100%', height: '100%', position: 'relative', background: '#000' }}>
      <video
        ref={videoRef}
        data-media-target="true"
        autoPlay={autoPlay}
        muted={muted}
        playsInline
        style={{
          width: '100%',
          height: '100%',
          objectFit: 'contain',
          display: error ? 'none' : 'block',
        }}
      />

      {/* Render AI Bounding Boxes overlay synchronized with video element */}
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
          <span>Connecting WebRTC...</span>
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
