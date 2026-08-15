import { useEffect, useState, useRef } from 'react';
import { globalInferenceClient } from '../ai/inference-client';
import type { Detection } from '../ai/inference';

interface UseCameraAiDetectionOptions {
  cameraId: string;
  enabled: boolean;
  frameIntervalMs?: number;
  getTargetElement: () => HTMLVideoElement | HTMLCanvasElement | null;
}

export function useCameraAiDetection({
  cameraId,
  enabled,
  frameIntervalMs = 120, // ~8 FPS
  getTargetElement,
}: UseCameraAiDetectionOptions) {
  const [detections, setDetections] = useState<Detection[]>([]);
  const isRunningRef = useRef(false);
  const getTargetElementRef = useRef(getTargetElement);
  getTargetElementRef.current = getTargetElement;

  useEffect(() => {
    if (!enabled) {
      setDetections([]);
      return;
    }

    console.log(`%c[AI Hook] Activated AI tracking for camera: ${cameraId}`, 'color: #38bdf8; font-weight: bold;');

    // Subscribe to detections from Web Worker
    const unsubscribe = globalInferenceClient.subscribe(cameraId, (newDetections) => {
      setDetections(newDetections);
    });

    isRunningRef.current = true;

    // Periodic frame capture
    const intervalId = setInterval(async () => {
      if (!isRunningRef.current) return;
      const el = getTargetElementRef.current();
      if (!el) return;

      const isVideo = el instanceof HTMLVideoElement;
      const w = isVideo ? el.videoWidth : (el as HTMLCanvasElement).width;
      const h = isVideo ? el.videoHeight : (el as HTMLCanvasElement).height;

      if (w <= 0 || h <= 0) return;

      try {
        const bitmap = await createImageBitmap(el);
        globalInferenceClient.detect(cameraId, bitmap);
      } catch (err) {
        // Silently catch during video layout changes
      }
    }, frameIntervalMs);

    return () => {
      isRunningRef.current = false;
      clearInterval(intervalId);
      unsubscribe();
      console.log(`[AI Hook] Deactivated AI tracking for camera: ${cameraId}`);
    };
  }, [cameraId, enabled, frameIntervalMs]);

  return {
    detections,
    isReady: globalInferenceClient.getIsReady(),
    backend: globalInferenceClient.getBackend(),
  };
}
