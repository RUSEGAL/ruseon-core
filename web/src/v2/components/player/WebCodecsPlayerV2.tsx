import React, { useEffect, useRef, useState, useCallback } from 'react';
import { Loader2, AlertTriangle, Cpu } from 'lucide-react';
import type { MetadataPayload } from '../../../types';

interface WebCodecsPlayerV2Props {
  streamId: string;
  autoPlay?: boolean;
  muted?: boolean;
  showMetadata?: boolean;
  onError?: (err: string) => void;
  onConnected?: () => void;
}

export const WebCodecsPlayerV2: React.FC<WebCodecsPlayerV2Props> = ({
  streamId,
  showMetadata = false,
  onError,
  onConnected,
}) => {
  const containerRef = useRef<HTMLDivElement>(null);
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [errorMsg, setErrorMsg] = useState<string | null>(null);

  const decoderRef = useRef<VideoDecoder | null>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const currentMetadataRef = useRef<MetadataPayload | null>(null);

  // Keep callback refs stable to prevent unneeded reconnect loops
  const onErrorRef = useRef(onError);
  onErrorRef.current = onError;
  const onConnectedRef = useRef(onConnected);
  onConnectedRef.current = onConnected;
  const showMetadataRef = useRef(showMetadata);
  showMetadataRef.current = showMetadata;

  // Helper to extract H.264 codec parameter string from SPS (e.g., avc1.640028)
  const getH264CodecString = (spsBytes: Uint8Array): string => {
    if (spsBytes.length >= 4) {
      const profile = spsBytes[1].toString(16).padStart(2, '0');
      const compat = spsBytes[2].toString(16).padStart(2, '0');
      const level = spsBytes[3].toString(16).padStart(2, '0');
      return `avc1.${profile}${compat}${level}`.toLowerCase();
    }
    return 'avc1.42e01e';
  };

  // Helper to build standard AVCC description buffer from SPS and PPS
  const createAvccDescription = (sps: Uint8Array, pps: Uint8Array): Uint8Array => {
    const desc = new Uint8Array(5 + 1 + 2 + sps.length + 1 + 2 + pps.length);
    desc[0] = 0x01; // configurationVersion
    desc[1] = sps[1]; // AVCProfileIndication
    desc[2] = sps[2]; // profile_compatibility
    desc[3] = sps[3]; // AVCLevelIndication
    desc[4] = 0xff; // 6 bits reserved (111111b) + 2 bits NALULengthSizeMinusOne (3 => 4 bytes length)
    desc[5] = 0xe1; // 3 bits reserved (111b) + 5 bits numOfSequenceParameterSets (1)
    desc[6] = (sps.length >> 8) & 0xff;
    desc[7] = sps.length & 0xff;
    desc.set(sps, 8);
    let offset = 8 + sps.length;
    desc[offset] = 0x01; // numOfPictureParameterSets
    offset += 1;
    desc[offset] = (pps.length >> 8) & 0xff;
    offset += 1;
    desc[offset] = pps.length & 0xff;
    offset += 1;
    desc.set(pps, offset);
    return desc;
  };

  // Convert Annex-B stream (00 00 00 01 NALU) into AVCC (4-byte length prefix NALU)
  const annexBToAvcc = (annexB: Uint8Array): Uint8Array => {
    const output = new Uint8Array(annexB.length);
    output.set(annexB);

    let start = -1;
    const naluPositions: number[] = [];

    for (let i = 0; i < annexB.length - 3; i++) {
      if (
        annexB[i] === 0 &&
        annexB[i + 1] === 0 &&
        annexB[i + 2] === 0 &&
        annexB[i + 3] === 1
      ) {
        if (start !== -1) {
          naluPositions.push(start, i);
        }
        start = i;
        i += 3;
      }
    }
    if (start !== -1) {
      naluPositions.push(start, annexB.length);
    }

    for (let j = 0; j < naluPositions.length; j += 2) {
      const pos = naluPositions[j];
      const end = naluPositions[j + 1];
      const naluLen = end - (pos + 4);
      output[pos] = (naluLen >> 24) & 0xff;
      output[pos + 1] = (naluLen >> 16) & 0xff;
      output[pos + 2] = (naluLen >> 8) & 0xff;
      output[pos + 3] = naluLen & 0xff;
    }

    return output;
  };

  const connectStream = useCallback(() => {
    if (typeof window === 'undefined' || !('VideoDecoder' in window)) {
      const msg = 'WebCodecs VideoDecoder API is not supported in this browser.';
      setErrorMsg(msg);
      onErrorRef.current?.(msg);
      setIsLoading(false);
      return;
    }

    setIsLoading(true);
    setErrorMsg(null);

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const token = localStorage.getItem('token');
    const wsUrl = token
      ? `${protocol}//${window.location.host}/stream/ws/${streamId}?token=${token}`
      : `${protocol}//${window.location.host}/stream/ws/${streamId}`;

    const ws = new WebSocket(wsUrl);
    ws.binaryType = 'arraybuffer';
    wsRef.current = ws;

    let hasReceivedFirstFrame = false;

    ws.onmessage = (e: MessageEvent) => {
      if (!(e.data instanceof ArrayBuffer)) return;
      const data = new Uint8Array(e.data);
      if (data.length < 2) return;

      const packetType = data[0];

      // 1. Header Packet (0x01): Codec configuration
      if (packetType === 0x01) {
        try {
          const codecType = data[1]; // 1 = H.264, 2 = H.265
          const view = new DataView(e.data);
          let offset = 2;

          const vpsLen = view.getUint16(offset, false);
          offset += 2;
          offset += vpsLen;

          const spsLen = view.getUint16(offset, false);
          offset += 2;
          const spsBytes = data.slice(offset, offset + spsLen);
          offset += spsLen;

          const ppsLen = view.getUint16(offset, false);
          offset += 2;
          const ppsBytes = data.slice(offset, offset + ppsLen);
          offset += ppsLen;

          let codecString = 'avc1.42e01e';
          if (codecType === 2) {
            codecString = 'hvc1.1.6.L93.B0'; // HEVC Main Profile
          } else if (spsBytes.length > 0) {
            codecString = getH264CodecString(spsBytes);
          }

          // Close previous decoder if open
          if (decoderRef.current && decoderRef.current.state !== 'closed') {
            decoderRef.current.close();
          }

          // Instantiate new VideoDecoder
          const decoder = new VideoDecoder({
            output: (frame: VideoFrame) => {
              if (!hasReceivedFirstFrame) {
                hasReceivedFirstFrame = true;
                setIsLoading(false);
                onConnectedRef.current?.();
              }

              const canvas = canvasRef.current;
              if (canvas) {
                if (canvas.width !== frame.displayWidth || canvas.height !== frame.displayHeight) {
                  canvas.width = frame.displayWidth;
                  canvas.height = frame.displayHeight;
                }
                const ctx = canvas.getContext('2d');
                if (ctx) {
                  ctx.drawImage(frame, 0, 0, canvas.width, canvas.height);

                  // AI detection boxes
                  if (showMetadataRef.current && currentMetadataRef.current?.objects) {
                    ctx.save();
                    ctx.lineWidth = 3;
                    ctx.font = 'bold 16px sans-serif';
                    for (const obj of currentMetadataRef.current.objects) {
                      const objX = obj.x !== undefined ? obj.x : (obj.box ? obj.box[1] : 0);
                      const objY = obj.y !== undefined ? obj.y : (obj.box ? obj.box[0] : 0);
                      const objW = obj.w !== undefined ? obj.w : (obj.box ? obj.box[3] - obj.box[1] : 0);
                      const objH = obj.h !== undefined ? obj.h : (obj.box ? obj.box[2] - obj.box[0] : 0);
                      const x = objX * canvas.width;
                      const y = objY * canvas.height;
                      const w = objW * canvas.width;
                      const h = objH * canvas.height;
                      ctx.strokeStyle = '#10b981';
                      ctx.fillStyle = 'rgba(16, 185, 129, 0.2)';
                      ctx.strokeRect(x, y, w, h);
                      ctx.fillRect(x, y, w, h);

                      ctx.fillStyle = '#10b981';
                      const label = `${obj.className || obj.class || 'Object'} (${Math.round(obj.confidence * 100)}%)`;
                      ctx.fillText(label, x + 4, Math.max(20, y - 6));
                    }
                    ctx.restore();
                  }
                }
              }

              // CRUCIAL: Release GPU VideoFrame immediately to guarantee zero memory leak
              frame.close();
            },
            error: (err: DOMException) => {
              console.error('WebCodecs Decoder error:', err);
              setErrorMsg(`Decoder error: ${err.message}`);
              onErrorRef.current?.(err.message);
            },
          });

          // Build decoder configuration
          const config: VideoDecoderConfig = {
            codec: codecString,
            hardwareAcceleration: 'prefer-hardware',
            optimizeForLatency: true,
          };

          if (codecType === 1 && spsBytes.length > 0 && ppsBytes.length > 0) {
            config.description = createAvccDescription(spsBytes, ppsBytes);
          }

          decoder.configure(config);
          decoderRef.current = decoder;
        } catch (err: any) {
          console.error('Failed to configure WebCodecs decoder:', err);
          setErrorMsg(`Config error: ${err.message}`);
          onErrorRef.current?.(err.message);
        }
        return;
      }

      // 2. Video Data Packet (0x02): NAL Units Access Unit
      if (packetType === 0x02) {
        if (!decoderRef.current || decoderRef.current.state !== 'configured') {
          return;
        }

        const isKeyFrame = data[1] === 1 ? 'key' : 'delta';
        const view = new DataView(e.data);
        const timestampMicro = Number(view.getBigUint64(2, false));
        const rawAnnexB = data.slice(10);
        const avccPayload = annexBToAvcc(rawAnnexB);

        try {
          decoderRef.current.decode(
            new EncodedVideoChunk({
              type: isKeyFrame,
              timestamp: timestampMicro,
              data: avccPayload,
            })
          );
        } catch (err: any) {
          console.warn('Decode chunk dropped:', err);
        }
      }
    };

    ws.onerror = (err) => {
      console.error('WebCodecs WebSocket error:', err);
      setErrorMsg('WebSocket connection error');
      onErrorRef.current?.('WebSocket connection error');
      setIsLoading(false);
    };

    ws.onclose = () => {
      // WS closed
    };
  }, [streamId]);

  useEffect(() => {
    connectStream();

    return () => {
      if (wsRef.current) {
        wsRef.current.close();
        wsRef.current = null;
      }
      if (decoderRef.current && decoderRef.current.state !== 'closed') {
        try {
          decoderRef.current.close();
        } catch (e) {
          // Ignore
        }
        decoderRef.current = null;
      }
    };
  }, [connectStream]);

  return (
    <div
      ref={containerRef}
      style={{
        position: 'relative',
        width: '100%',
        height: '100%',
        background: '#000',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        overflow: 'hidden',
      }}
    >
      {/* Canvas Video Target */}
      <canvas
        ref={canvasRef}
        data-media-target="true"
        style={{
          width: '100%',
          height: '100%',
          objectFit: 'contain',
          display: isLoading || errorMsg ? 'none' : 'block',
        }}
      />

      {/* Loading Spinner */}
      {isLoading && !errorMsg && (
        <div
          style={{
            position: 'absolute',
            inset: 0,
            display: 'flex',
            flexDirection: 'column',
            alignItems: 'center',
            justifyContent: 'center',
            gap: '8px',
            color: '#38bdf8',
            background: '#090d16',
          }}
        >
          <Loader2 size={28} className="animate-spin" />
          <div style={{ display: 'flex', alignItems: 'center', gap: '6px', fontSize: '0.76rem', color: '#94a3b8' }}>
            <Cpu size={14} color="#38bdf8" />
            <span>Connecting Hardware WebCodecs...</span>
          </div>
        </div>
      )}

      {/* Error Fallback */}
      {errorMsg && (
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
            background: '#090d16',
            padding: '1rem',
            textAlign: 'center',
          }}
        >
          <AlertTriangle size={24} />
          <span style={{ fontSize: '0.78rem', fontWeight: 600 }}>{errorMsg}</span>
          <button
            onClick={() => {
              connectStream();
            }}
            style={{
              marginTop: '4px',
              background: 'rgba(255,255,255,0.08)',
              border: '1px solid rgba(255,255,255,0.15)',
              borderRadius: '6px',
              padding: '4px 10px',
              color: '#f8fafc',
              fontSize: '0.72rem',
              cursor: 'pointer',
            }}
          >
            Retry Connection
          </button>
        </div>
      )}
    </div>
  );
};
