/**
 * Inference Client — main-thread singleton facade over the shared AI Web Worker.
 *
 * - Hardware capability aware & dynamic model resolution
 * - Hot model tier switching (Nano / Small / Medium)
 * - Busy-guard per camera (drops ticks if worker is still processing previous frame)
 * - Zero-copy transferable ImageBitmap transfer
 * - Pub/Sub subscription for camera bounding boxes
 */
import type { Detection, ObjectDetectorOptions } from './inference';
import type { InferenceWorkerRequest, InferenceWorkerResponse } from './inference-worker';
import { getEffectiveModelUrl, setAiModelPreference, type AiModelPreference, type AiModelTier } from './device-profiler';
import InferenceWorker from './inference-worker?worker';

type DetectionsCallback = (detections: Detection[]) => void;
type ModelChangedCallback = (tier: AiModelTier, modelUrl: string, isAuto: boolean) => void;

class InferenceClient {
  private worker: Worker | null = null;
  private isReady = false;
  private backend = 'initializing...';
  private currentTier: AiModelTier = 'medium';
  private currentModelUrl: string = '/models/yolo11m.onnx';
  private isAuto = true;
  private subscribers = new Map<string, Set<DetectionsCallback>>();
  private modelSubscribers = new Set<ModelChangedCallback>();
  private busyCameras = new Set<string>();
  private registeredCameras = new Set<string>();

  constructor() {
    // Eagerly initialize worker in browser environment
    if (typeof window !== 'undefined') {
      this.init();
    }
  }

  public async init(explicitModelUrl?: string): Promise<void> {
    if (this.worker) return;

    try {
      let targetModelUrl = explicitModelUrl;
      if (!targetModelUrl) {
        const eff = await getEffectiveModelUrl();
        targetModelUrl = eff.modelUrl;
        this.currentTier = eff.tier;
        this.currentModelUrl = eff.modelUrl;
        this.isAuto = eff.isAuto;
      }

      console.log(`[AI Client] Spawning Inference Web Worker with model: ${targetModelUrl}...`);
      this.worker = new InferenceWorker();

      this.worker.onmessage = (event: MessageEvent<InferenceWorkerResponse>) => {
        this.handleMessage(event.data);
      };

      this.worker.onerror = (e: ErrorEvent) => {
        console.error('[AI Client] Inference Worker runtime error:', e.message || e);
      };

      this.worker.postMessage({ type: 'init', modelUrl: targetModelUrl } as InferenceWorkerRequest);
    } catch (err) {
      console.error('[AI Client] Failed to create Web Worker:', err);
    }
  }

  public async switchModelPreference(pref: AiModelPreference): Promise<void> {
    setAiModelPreference(pref);
    const eff = await getEffectiveModelUrl();
    this.currentTier = eff.tier;
    this.currentModelUrl = eff.modelUrl;
    this.isAuto = eff.isAuto;

    console.log(`[AI Client] Switching AI Model Tier -> ${this.currentTier.toUpperCase()} (${this.currentModelUrl}) [Auto: ${this.isAuto}]`);

    if (!this.worker) {
      await this.init(this.currentModelUrl);
    } else {
      this.isReady = false;
      this.worker.postMessage({
        type: 'init',
        modelUrl: this.currentModelUrl,
        forceReload: true,
      } as InferenceWorkerRequest);
    }

    this.notifyModelSubscribers();
  }

  public onModelChanged(cb: ModelChangedCallback): () => void {
    this.modelSubscribers.add(cb);
    cb(this.currentTier, this.currentModelUrl, this.isAuto);
    return () => {
      this.modelSubscribers.delete(cb);
    };
  }

  private notifyModelSubscribers() {
    for (const cb of this.modelSubscribers) {
      cb(this.currentTier, this.currentModelUrl, this.isAuto);
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

  public getCurrentTier(): AiModelTier {
    return this.currentTier;
  }

  public getIsAuto(): boolean {
    return this.isAuto;
  }

  private handleMessage(msg: InferenceWorkerResponse): void {
    switch (msg.type) {
      case 'ready': {
        this.isReady = true;
        this.backend = msg.backend;
        console.log(`%c[AI Client] AI Inference Worker ready, backend: ${this.backend} (Model: ${this.currentModelUrl})`, 'color: #10b981; font-weight: bold;');
        this.notifyModelSubscribers();
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
