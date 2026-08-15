/**
 * ONNX Runtime Web Integration — AiRuntime
 *
 * - WebGPU execution provider (preferred) with WASM SIMD fallback
 * - Cache API for model files (avoids re-downloading 5.8MB model)
 * - Session execution with named input/output tensors
 */
import * as ort from 'onnxruntime-web';

export const MODEL_CACHE_NAME = 'ruseon-ai-models';
export const MODEL_URLS = [
  '/models/yolo11n.onnx',
];

export interface AiRuntimeInitOptions {
  onProgress?: (loaded: number, total: number) => void;
}

export interface AiRunResult {
  [name: string]: {
    data: Float32Array;
    dims: number[];
    dispose?: () => void;
  };
}

export class AiRuntime {
  private session: ort.InferenceSession | null = null;
  private isInitializing = false;
  private backend: 'webgpu' | 'wasm' = 'wasm';

  public async init(modelUrl?: string, options?: AiRuntimeInitOptions): Promise<void> {
    if (this.session) return;
    if (this.isInitializing) return;

    this.isInitializing = true;
    try {
      console.log('[AI Runtime] Starting initialization...');

      ort.env.wasm.numThreads = 1;
      ort.env.wasm.simd = true;

      const modelBuffer = await this.fetchModelWithFallback(modelUrl, options?.onProgress);
      console.log(`[AI Runtime] Model loaded into buffer: ${(modelBuffer.byteLength / 1024 / 1024).toFixed(2)} MB`);

      // Check WebGPU availability
      const hasWebGPU = typeof navigator !== 'undefined' && 'gpu' in navigator;

      try {
        if (hasWebGPU) {
          console.log('[AI Runtime] Attempting InferenceSession with WebGPU provider...');
          this.session = await ort.InferenceSession.create(modelBuffer, {
            executionProviders: ['webgpu'],
            graphOptimizationLevel: 'all',
          });
          this.backend = 'webgpu';
        } else {
          throw new Error('WebGPU not supported in this browser environment');
        }
      } catch (gpuErr) {
        console.warn('[AI Runtime] WebGPU init failed or unavailable, falling back to WASM:', gpuErr);
        this.session = await ort.InferenceSession.create(modelBuffer, {
          executionProviders: ['wasm'],
          graphOptimizationLevel: 'all',
        });
        this.backend = 'wasm';
      }

      console.log(`[AI Runtime] InferenceSession READY on backend: ${this.backend}`);
    } catch (err: any) {
      console.error('[AI Runtime] Initialization failed:', err);
      throw err;
    } finally {
      this.isInitializing = false;
    }
  }

  public async run(feeds: Record<string, ort.Tensor>): Promise<AiRunResult> {
    if (!this.session) {
      throw new Error('AiRuntime session is not initialized');
    }

    const results = await this.session.run(feeds);
    const outMap: AiRunResult = {};

    for (const [name, tensor] of Object.entries(results)) {
      outMap[name] = {
        data: tensor.data as Float32Array,
        dims: tensor.dims as number[],
      };
    }

    return outMap;
  }

  public getBackend(): 'webgpu' | 'wasm' {
    return this.backend;
  }

  public isReady(): boolean {
    return this.session !== null;
  }

  public dispose(): void {
    if (this.session) {
      this.session.release();
      this.session = null;
    }
  }

  private async fetchModelWithFallback(
    customUrl?: string,
    onProgress?: (loaded: number, total: number) => void
  ): Promise<ArrayBuffer> {
    const urls = customUrl ? [customUrl, ...MODEL_URLS] : MODEL_URLS;

    for (const url of urls) {
      try {
        console.log(`[AI Runtime] Attempting to load model from: ${url}`);
        const buf = await this.fetchModelWithCache(url, onProgress);
        if (buf && buf.byteLength > 1024 * 1024) {
          return buf;
        }
      } catch (err) {
        console.warn(`[AI Runtime] Failed loading from ${url}:`, err);
      }
    }

    throw new Error('Could not download AI model from any available mirror.');
  }

  private async fetchModelWithCache(
    url: string,
    onProgress?: (loaded: number, total: number) => void
  ): Promise<ArrayBuffer> {
    // 1. Try browser Cache API
    if (typeof caches !== 'undefined') {
      try {
        const cache = await caches.open(MODEL_CACHE_NAME);
        const cachedResp = await cache.match(url);
        if (cachedResp && cachedResp.ok) {
          const buf = await cachedResp.arrayBuffer();
          if (buf.byteLength > 1024 * 1024) {
            console.log(`[AI Runtime] Loaded model from Cache API (${(buf.byteLength / 1024 / 1024).toFixed(2)} MB)`);
            onProgress?.(buf.byteLength, buf.byteLength);
            return buf;
          }
        }
      } catch {
        // Fallback to fetch
      }
    }

    // 2. Fetch from network
    const response = await fetch(url, { mode: 'cors' });
    if (!response.ok) {
      throw new Error(`HTTP ${response.status} from ${url}`);
    }

    const contentLength = Number(response.headers.get('content-length')) || 0;
    let arrayBuffer: ArrayBuffer;

    if (response.body && contentLength > 0 && onProgress) {
      const reader = response.body.getReader();
      const chunks: Uint8Array[] = [];
      let loaded = 0;

      while (true) {
        const { done, value } = await reader.read();
        if (done) break;
        if (value) {
          chunks.push(value);
          loaded += value.length;
          onProgress(loaded, contentLength);
        }
      }

      const merged = new Uint8Array(loaded);
      let offset = 0;
      for (const chunk of chunks) {
        merged.set(chunk, offset);
        offset += chunk.length;
      }
      arrayBuffer = merged.buffer;
    } else {
      arrayBuffer = await response.arrayBuffer();
    }

    // 3. Store in Cache API
    if (typeof caches !== 'undefined') {
      try {
        const cache = await caches.open(MODEL_CACHE_NAME);
        await cache.put(url, new Response(arrayBuffer.slice(0), {
          headers: {
            'Content-Type': 'application/octet-stream',
            'Content-Length': String(arrayBuffer.byteLength),
          },
        }));
        console.log('[AI Runtime] Saved model to Cache Storage.');
      } catch {
        // Ignore
      }
    }

    return arrayBuffer;
  }
}
