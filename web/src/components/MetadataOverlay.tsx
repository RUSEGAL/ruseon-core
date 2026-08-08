import React, { useEffect, useRef } from 'react';

export interface BoundingBox {
  classId: number;
  className: string;
  confidence: number;
  x: number;
  y: number;
  w: number;
  h: number;
}

export interface MetadataPayload {
  pts: number;
  objects: BoundingBox[];
}

interface Props {
  metadata: MetadataPayload | null;
  videoRef: React.RefObject<HTMLVideoElement | null> | React.RefObject<HTMLVideoElement>;
}

export function MetadataOverlay({ metadata, videoRef }: Props) {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const animationRef = useRef<number>(0);

  useEffect(() => {
    const canvas = canvasRef.current;
    const video = videoRef.current;
    if (!canvas || !video) return;

    const render = () => {
      const ctx = canvas.getContext('2d');
      if (!ctx) return;

      if (canvas.width !== canvas.offsetWidth || canvas.height !== canvas.offsetHeight) {
        canvas.width = canvas.offsetWidth;
        canvas.height = canvas.offsetHeight;
      }

      ctx.clearRect(0, 0, canvas.width, canvas.height);

      if (metadata && metadata.objects && video.videoWidth > 0 && video.videoHeight > 0) {
        const videoRatio = video.videoWidth / video.videoHeight;
        const canvasRatio = canvas.width / canvas.height;
        
        let drawWidth = canvas.width;
        let drawHeight = canvas.height;
        let offsetX = 0;
        let offsetY = 0;

        if (videoRatio > canvasRatio) {
          drawHeight = canvas.width / videoRatio;
          offsetY = (canvas.height - drawHeight) / 2;
        } else {
          drawWidth = canvas.height * videoRatio;
          offsetX = (canvas.width - drawWidth) / 2;
        }

        ctx.lineWidth = 2;
        ctx.font = '14px sans-serif';

        metadata.objects.forEach(obj => {
          const x = offsetX + obj.x * drawWidth;
          const y = offsetY + obj.y * drawHeight;
          const w = obj.w * drawWidth;
          const h = obj.h * drawHeight;

          const hue = (obj.classId * 137.5) % 360;
          ctx.strokeStyle = `hsl(${hue}, 100%, 50%)`;
          ctx.strokeRect(x, y, w, h);

          ctx.fillStyle = `hsla(${hue}, 100%, 50%, 0.7)`;
          const text = `${obj.className} ${(obj.confidence * 100).toFixed(0)}%`;
          const textMetrics = ctx.measureText(text);
          ctx.fillRect(x, y - 20, textMetrics.width + 10, 20);

          ctx.fillStyle = '#000000';
          ctx.fillText(text, x + 5, y - 5);
        });
      }

      animationRef.current = requestAnimationFrame(render);
    };

    animationRef.current = requestAnimationFrame(render);

    return () => {
      if (animationRef.current) cancelAnimationFrame(animationRef.current);
    };
  }, [metadata, videoRef]);

  return (
    <canvas 
      ref={canvasRef} 
      style={{
        position: 'absolute',
        top: 0,
        left: 0,
        width: '100%',
        height: '100%',
        pointerEvents: 'none',
        zIndex: 10
      }} 
    />
  );
}
