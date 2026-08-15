import React, { useEffect, useRef } from 'react';
import type { Detection } from './inference';
import { getAiClassColor, getLocalizedClassLabel } from './ai-labels';

interface AiOverlayProps {
  detections: Detection[];
  visible?: boolean;
}

export const AiOverlay: React.FC<AiOverlayProps> = ({ detections, visible = true }) => {
  const canvasRef = useRef<HTMLCanvasElement>(null);

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const ctx = canvas.getContext('2d');
    if (!ctx) return;

    if (!visible || detections.length === 0) {
      ctx.clearRect(0, 0, canvas.width, canvas.height);
      return;
    }

    const displayW = canvas.clientWidth || canvas.parentElement?.clientWidth || 640;
    const displayH = canvas.clientHeight || canvas.parentElement?.clientHeight || 480;

    if (canvas.width !== displayW || canvas.height !== displayH) {
      canvas.width = displayW;
      canvas.height = displayH;
    }

    ctx.clearRect(0, 0, canvas.width, canvas.height);

    const w = canvas.width;
    const h = canvas.height;

    for (const det of detections) {
      const [x1Norm, y1Norm, x2Norm, y2Norm] = det.bbox;
      const x = x1Norm * w;
      const y = y1Norm * h;
      const boxW = (x2Norm - x1Norm) * w;
      const boxH = (y2Norm - y1Norm) * h;

      if (boxW <= 0 || boxH <= 0) continue;

      const color = getAiClassColor(det.classId);
      const label = `${getLocalizedClassLabel(det.label)} ${Math.round(det.confidence * 100)}%`;

      // 1. Semi-transparent bounding box background
      ctx.fillStyle = `${color}18`;
      ctx.fillRect(x, y, boxW, boxH);

      // 2. Glowing Bounding Box outline
      ctx.lineWidth = 2;
      ctx.strokeStyle = color;
      ctx.strokeRect(x, y, boxW, boxH);

      // 3. Futuristic corner brackets
      const cornerLen = Math.min(14, Math.min(boxW, boxH) / 3);
      ctx.lineWidth = 3;
      ctx.strokeStyle = '#ffffff';

      // Top-Left corner
      ctx.beginPath();
      ctx.moveTo(x, y + cornerLen);
      ctx.lineTo(x, y);
      ctx.lineTo(x + cornerLen, y);
      ctx.stroke();

      // Top-Right corner
      ctx.beginPath();
      ctx.moveTo(x + boxW - cornerLen, y);
      ctx.lineTo(x + boxW, y);
      ctx.lineTo(x + boxW, y + cornerLen);
      ctx.stroke();

      // Bottom-Left corner
      ctx.beginPath();
      ctx.moveTo(x, y + boxH - cornerLen);
      ctx.lineTo(x, y + boxH);
      ctx.lineTo(x + cornerLen, y + boxH);
      ctx.stroke();

      // Bottom-Right corner
      ctx.beginPath();
      ctx.moveTo(x + boxW - cornerLen, y + boxH);
      ctx.lineTo(x + boxW, y + boxH);
      ctx.lineTo(x + boxW, y + boxH - cornerLen);
      ctx.stroke();

      // 4. Label Tag Badge
      ctx.font = '600 11px system-ui, -apple-system, sans-serif';
      const textMetrics = ctx.measureText(label);
      const tagH = 18;
      const tagW = textMetrics.width + 10;
      const tagY = Math.max(0, y - tagH - 2);

      ctx.fillStyle = 'rgba(10, 14, 23, 0.85)';
      ctx.beginPath();
      ctx.roundRect(x, tagY, tagW, tagH, 4);
      ctx.fill();

      ctx.strokeStyle = color;
      ctx.lineWidth = 1;
      ctx.stroke();

      // Indicator dot
      ctx.fillStyle = color;
      ctx.beginPath();
      ctx.arc(x + 7, tagY + tagH / 2, 3, 0, Math.PI * 2);
      ctx.fill();

      // Label text
      ctx.fillStyle = '#f8fafc';
      ctx.fillText(label, x + 14, tagY + 13);
    }
  }, [detections, visible]);

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
        zIndex: 20,
      }}
    />
  );
};
