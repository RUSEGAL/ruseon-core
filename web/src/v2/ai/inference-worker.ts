/// <reference lib="webworker" />
import { AiRuntime } from './runtime';
import { ObjectDetector, type Detection, type ObjectDetectorOptions } from './inference';

export type InferenceWorkerRequest =
  | { type: 'init'; modelUrl?: string }
  | { type: 'register'; cameraId: string; options?: ObjectDetectorOptions }
  | { type: 'update-options'; cameraId: string; options: ObjectDetectorOptions }
  | { type: 'detect'; cameraId: string; frame: ImageBitmap }
  | { type: 'clear'; cameraId: string }
  | { type: 'dispose-all' };

export type InferenceWorkerResponse =
  | { type: 'ready'; backend: string }
  | { type: 'init-error'; error: string }
  | { type: 'detections'; cameraId: string; detections: Detection[] }
  | { type: 'error'; cameraId?: string; error: string };

let runtime: AiRuntime | null = null;
let initPromise: Promise<void> | null = null;
const detectors = new Map<string, ObjectDetector>();

// Strict sequential queue to serialize WebGPU session execution across all cameras
let isProcessing = false;
const detectQueue: Array<{ cameraId: string; frame: ImageBitmap }> = [];

function post(msg: InferenceWorkerResponse, transfer?: Transferable[]) {
  (self as unknown as Worker).postMessage(msg, transfer ? { transfer } : undefined);
}

async function ensureRuntime(modelUrl?: string): Promise<void> {
  if (runtime && runtime.isReady()) return;
  if (initPromise) return initPromise;

  initPromise = (async () => {
    const rt = new AiRuntime();
    await rt.init(modelUrl);
    runtime = rt;
  })();

  try {
    await initPromise;
    const backend = runtime?.getBackend() || 'wasm';
    console.log(`%c[AI Inference Worker] Ready! Backend: ${backend}`, 'color: #10b981; font-weight: bold;');
    post({ type: 'ready', backend });
  } catch (err: any) {
    console.error('[AI Inference Worker] Failed to initialize:', err);
    post({ type: 'init-error', error: err?.message || 'Failed to initialize AI runtime' });
    throw err;
  } finally {
    initPromise = null;
  }
}

async function processQueue() {
  if (isProcessing || detectQueue.length === 0) return;
  isProcessing = true;

  const item = detectQueue.shift()!;
  const { cameraId, frame } = item;

  try {
    if (!runtime || !runtime.isReady()) {
      await ensureRuntime();
    }

    let detector = detectors.get(cameraId);
    if (!detector && runtime) {
      detector = new ObjectDetector(runtime);
      detectors.set(cameraId, detector);
    }

    if (!detector) {
      try { frame.close(); } catch {}
      return;
    }

    const detections = await detector.detect(frame);
    post({
      type: 'detections',
      cameraId,
      detections,
    });
  } catch (err: any) {
    console.warn(`[AI Worker] Detection error on ${cameraId}:`, err?.message || err);
    post({
      type: 'error',
      cameraId,
      error: err?.message || 'Detection inference failed',
    });
  } finally {
    try { frame.close(); } catch {}
    isProcessing = false;

    // Process next queued camera frame if any
    if (detectQueue.length > 0) {
      processQueue();
    }
  }
}

self.onmessage = async (event: MessageEvent<InferenceWorkerRequest>) => {
  const req = event.data;
  if (!req) return;

  switch (req.type) {
    case 'init': {
      try {
        await ensureRuntime(req.modelUrl);
      } catch {
        // Handled
      }
      break;
    }

    case 'register': {
      try {
        await ensureRuntime();
        if (!detectors.has(req.cameraId) && runtime) {
          detectors.set(req.cameraId, new ObjectDetector(runtime, req.options));
          console.log(`[AI Worker] Registered detector for camera: ${req.cameraId}`);
        }
      } catch (err: any) {
        post({ type: 'error', cameraId: req.cameraId, error: err.message });
      }
      break;
    }

    case 'update-options': {
      const detector = detectors.get(req.cameraId);
      if (detector) {
        detector.updateOptions(req.options);
      }
      break;
    }

    case 'detect': {
      // If queue already contains an un-processed frame for this camera, replace with freshest frame
      const existingIdx = detectQueue.findIndex((q) => q.cameraId === req.cameraId);
      if (existingIdx !== -1) {
        const old = detectQueue.splice(existingIdx, 1)[0];
        try { old.frame.close(); } catch {}
      }

      detectQueue.push({ cameraId: req.cameraId, frame: req.frame });
      processQueue();
      break;
    }

    case 'clear': {
      const detector = detectors.get(req.cameraId);
      if (detector) {
        detector.clearSmoothing();
      }
      break;
    }

    case 'dispose-all': {
      for (const item of detectQueue) {
        try { item.frame.close(); } catch {}
      }
      detectQueue.length = 0;
      detectors.clear();
      if (runtime) {
        runtime.dispose();
        runtime = null;
      }
      break;
    }
  }
};
