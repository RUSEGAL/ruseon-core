import { useEffect, useRef, useState } from 'react';
import { Eye, EyeOff } from 'lucide-react';
import { MetadataOverlay } from './MetadataOverlay';
import type { MetadataPayload } from './MetadataOverlay';

interface WebRTCPlayerProps {
  streamId: string;
}

export function WebRTCPlayer({ streamId }: WebRTCPlayerProps) {
  const videoRef = useRef<HTMLVideoElement | null>(null);
  const pcRef = useRef<RTCPeerConnection | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [status, setStatus] = useState<string>('Connecting...');
  const [metadata, setMetadata] = useState<MetadataPayload | null>(null);
  const [showMetadata, setShowMetadata] = useState(false);
  const clearMetadataTimeout = useRef<number | null>(null);

  useEffect(() => {
    const startWHEP = async () => {
      try {
        const pc = new RTCPeerConnection({
          iceServers: [{ urls: 'stun:stun.l.google.com:19302' }]
        });
        pcRef.current = pc;

        pc.addTransceiver('video', { direction: 'recvonly' });

        pc.ontrack = (event) => {
          if (videoRef.current && event.streams[0]) {
            videoRef.current.srcObject = event.streams[0];
            setStatus('Playing (Low Latency)');
          }
        };

        const dc = pc.createDataChannel('metadata', { ordered: false, maxRetransmits: 0 });
        dc.onmessage = (event) => {
          try {
            const data = JSON.parse(event.data);
            setMetadata(data);

            if (clearMetadataTimeout.current) {
              window.clearTimeout(clearMetadataTimeout.current);
            }
            clearMetadataTimeout.current = window.setTimeout(() => {
              setMetadata(null);
            }, 1000); // clear if no updates for 1 second
          } catch (e) {
            console.error('Failed to parse metadata', e);
          }
        };

        pc.oniceconnectionstatechange = () => {
          if (pc.iceConnectionState === 'failed' || pc.iceConnectionState === 'disconnected') {
            setError('Connection lost');
            setStatus('Error');
          }
        };

        const offer = await pc.createOffer();
        await pc.setLocalDescription(offer);

        const apiUrl = import.meta.env.VITE_API_URL || '';
        const response = await fetch(`${apiUrl}/stream/webrtc/whep/${streamId}`, {
          method: 'POST',
          headers: {
            'Content-Type': 'application/sdp',
            'Authorization': `Bearer ${localStorage.getItem('token')}`
          },
          body: offer.sdp
        });

        if (!response.ok) {
          throw new Error(await response.text());
        }

        const answerSdp = await response.text();
        await pc.setRemoteDescription({ type: 'answer', sdp: answerSdp });

      } catch (err: any) {
        setError(err.message || 'Failed to start WebRTC');
        setStatus('Error');
      }
    };

    startWHEP();

    return () => {
      if (pcRef.current) {
        pcRef.current.close();
      }
    };
  }, [streamId]);

  return (
    <div style={{ position: 'relative', width: '100%', height: '100%', background: '#000', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
      <style>{`
        @keyframes spin { 100% { transform: rotate(360deg); } }
      `}</style>
      <video
        ref={videoRef}
        autoPlay
        playsInline
        muted
        style={{ width: '100%', height: '100%', objectFit: 'contain' }}
      />
      {status !== 'Playing (Low Latency)' && !error && (
        <div style={{ position: 'absolute', top: '50%', left: '50%', transform: 'translate(-50%, -50%)', background: 'rgba(0,0,0,0.7)', padding: '10px 20px', borderRadius: '8px', color: '#fff', textAlign: 'center' }}>
          <div style={{ margin: '0 auto 10px', width: '24px', height: '24px', border: '3px solid rgba(255,255,255,0.3)', borderTopColor: '#fff', borderRadius: '50%', animation: 'spin 1s linear infinite' }} />
          {status}
        </div>
      )}
      {error && (
        <div style={{ position: 'absolute', top: '50%', left: '50%', transform: 'translate(-50%, -50%)', background: 'rgba(239, 68, 68, 0.9)', padding: '10px 20px', borderRadius: '8px', color: '#fff', textAlign: 'center' }}>
          <div>{error}</div>
        </div>
      )}
      {showMetadata && <MetadataOverlay metadata={metadata} videoRef={videoRef} />}

      <div style={{ position: 'absolute', bottom: '16px', right: '16px', display: 'flex', gap: '8px' }}>
        <button 
          onClick={(e) => { e.stopPropagation(); setShowMetadata(!showMetadata); }}
          style={{ 
            background: 'rgba(0,0,0,0.5)', border: 'none', color: '#fff', 
            padding: '8px', borderRadius: '4px', cursor: 'pointer',
            opacity: showMetadata ? 1 : 0.5 
          }}
          title="Toggle Metadata"
        >
          {showMetadata ? <Eye size={20} /> : <EyeOff size={20} />}
        </button>
      </div>
    </div>
  );
}
