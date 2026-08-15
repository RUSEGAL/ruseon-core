/**
 * Inference Client — main-thread singleton facade over the shared AI Web Worker.
 *
 * - Auto-initialization
 * - Busy-guard per camera (drops ticks if worker is still processing previous frame)
 * - Zero-copy transferable ImageBitmap transfer
 * - Pub/Sub subscription for camera bounding boxes
 */
import type { Detection, ObjectDetectorOptions } from './inference';
import type { InferenceWorkerRequest, InferenceWorkerResponse } from './inference-worker';
import InferenceWorker from './inference-worker?worker';

type DetectionsCallback = (detections: Detection[]) => void;

class InferenceClient {
  private worker: Worker | null = null;
  private isReady = false;
  private backend = 'initializing...';
  private subscribers = new Map<string, Set<DetectionsCallback>>();
  private busyCameras = new Set<string>();
  private registeredCameras = new Set<string>();

  constructor() {
    // Eagerly initialize worker in browser environment
    if (typeof window !== 'undefined') {
      this.init();
    }
  }

  public init(modelUrl?: string): void {
    if (this.worker) return;

    try {
      console.log('[AI Client] Spawning Inference Web Worker via Vite...');
      this.worker = new InferenceWorker();

      this.worker.onmessage = (event: MessageEvent<InferenceWorkerResponse>) => {
        this.handleMessage(event.data);
      };

      this.worker.onerror = (e: ErrorEvent) => {
        console.error('[AI Client] Inference Worker runtime error:', e.message || e);
      };

      this.worker.postMessage({ type: 'init', modelUrl } as InferenceWorkerRequest);
    } catch (err) {
      console.error('[AI Client] Failed to create Web Worker:', err);
    }
  }

  public registerCamera(cameraId: string, options?: ObjectDetectorOptions): void {
    if (!this.worker) {
      this.init();
    }
    this.registeredCameras.add(cameraId);
    this.worker?.postMessage({
      type: 'register',
      cameraId,
      options,
    } as InferenceWorkerRequest);
  }

  public updateOptions(cameraId: string, options: ObjectDetectorOptions): void {
    this.worker?.postMessage({
      type: 'update-options',
      cameraId,
      options,
    } as InferenceWorkerRequest);
  }

  public detect(cameraId: string, frame: ImageBitmap): void {
    if (!this.worker) {
      this.init();
    }

    if (!this.worker || !this.isReady) {
      frame.close();
      return;
    }

    if (!this.registeredCameras.has(cameraId)) {
      this.registerCamera(cameraId);
    }

    // Busy-guard: drop frame if previous is still inferring on GPU
    if (this.busyCameras.has(cameraId)) {
      frame.close();
      return;
    }

    this.busyCameras.add(cameraId);

    // Transfer ImageBitmap to worker without memory copy
    this.worker.postMessage(
      {
        type: 'detect',
        cameraId,
        frame,
      } as InferenceWorkerRequest,
      [frame]
    );
  }

  public subscribe(cameraId: string, callback: DetectionsCallback): () => void {
    if (!this.worker) {
      this.init();
    }
    if (!this.registeredCameras.has(cameraId)) {
      this.registerCamera(cameraId);
    }

    if (!this.subscribers.has(cameraId)) {
      this.subscribers.set(cameraId, new Set());
    }
    this.subscribers.get(cameraId)!.add(callback);

    return () => {
      const set = this.subscribers.get(cameraId);
      if (set) {
        set.delete(callback);
        if (set.size === 0) {
          this.subscribers.delete(cameraId);
          this.worker?.postMessage({ type: 'clear', cameraId });
        }
      }
    };
  }

  public getBackend(): string {
    return this.backend;
  }

  public getIsReady(): boolean {
    return this.isReady;
  }

  private handleMessage(msg: InferenceWorkerResponse): void {
    switch (msg.type) {
      case 'ready': {
        this.isReady = true;
        this.backend = msg.backend;
        console.log(`%c[AI Client] AI Inference Worker ready, backend: ${this.backend}`, 'color: #10b981; font-weight: bold;');
        break;
      }

      case 'detections': {
        this.busyCameras.delete(msg.cameraId);
        const set = this.subscribers.get(msg.cameraId);
        if (set && set.size > 0) {
          for (const cb of set) {
            cb(msg.detections);
          }
        }
        break;
      }

      case 'error': {
        if (msg.cameraId) {
          this.busyCameras.delete(msg.cameraId);
        }
        console.warn('[AI Client] AI Worker warning:', msg.error);
        break;
      }

      case 'init-error': {
        this.isReady = false;
        console.error('[AI Client] AI Runtime initialization failed:', msg.error);
        break;
      }
    }
  }
}

export const globalInferenceClient = new InferenceClient();
