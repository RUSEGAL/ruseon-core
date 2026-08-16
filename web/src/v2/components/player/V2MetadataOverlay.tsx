import React, { useEffect, useRef } from 'react';
import type { MetadataPayload } from '../../../types';

interface V2MetadataOverlayProps {
  metadata: MetadataPayload;
  videoRef: React.RefObject<HTMLVideoElement | null>;
}

export const V2MetadataOverlay: React.FC<V2MetadataOverlayProps> = ({ metadata, videoRef }) => {
  const canvasRef = useRef<HTMLCanvasElement>(null);

  useEffect(() => {
    const canvas = canvasRef.current;
    const video = videoRef.current;
    if (!canvas || !video) return;

    const ctx = canvas.getContext('2d');
    if (!ctx) return;

    const rect = video.getBoundingClientRect();
    if (canvas.width !== rect.width || canvas.height !== rect.height) {
      canvas.width = rect.width;
      canvas.height = rect.height;
    }

    ctx.clearRect(0, 0, canvas.width, canvas.height);

    if (!metadata || !metadata.objects || metadata.objects.length === 0) {
      return;
    }

    const { width, height } = canvas;

    metadata.objects.forEach((obj) => {
      let x = 0, y = 0, w = 0, h = 0;
      if (obj.box && obj.box.length === 4) {
        const [ymin, xmin, ymax, xmax] = obj.box;
        x = xmin * width;
        y = ymin * height;
        w = (xmax - xmin) * width;
        h = (ymax - ymin) * height;
      } else if (obj.x !== undefined && obj.y !== undefined && obj.w !== undefined && obj.h !== undefined) {
        x = obj.x * width;
        y = obj.y * height;
        w = obj.w * width;
        h = obj.h * height;
      } else {
        return;
      }

      // Glow & Box
      ctx.strokeStyle = '#10b981';
      ctx.lineWidth = 2;
      ctx.shadowColor = '#10b981';
      ctx.shadowBlur = 6;
      ctx.strokeRect(x, y, w, h);

      // Label background
      ctx.shadowBlur = 0;
      ctx.fillStyle = 'rgba(16, 185, 129, 0.85)';
      const labelText = `${obj.className || obj.class || 'Object'} ${Math.round(obj.confidence * 100)}%`;
      ctx.font = '11px sans-serif';
      const textMetrics = ctx.measureText(labelText);
      ctx.fillRect(x, Math.max(0, y - 18), textMetrics.width + 8, 18);

      // Label text
      ctx.fillStyle = '#ffffff';
      ctx.fillText(labelText, x + 4, Math.max(12, y - 5));
    });
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
        zIndex: 10,
      }}
    />
  );
};
