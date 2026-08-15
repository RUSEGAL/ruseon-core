/**
 * YOLOv11-nano Inference Pipeline — ObjectDetector
 *
 * Preprocesses VideoFrame / ImageBitmap -> 640x640 Float32 tensor -> runs ONNX ->
 * parses YOLO output -> applies Sigmoid + Greedy NMS -> returns Detection[] with EMA smoothing.
 */
import * as ort from 'onnxruntime-web';
import { AiRuntime } from './runtime';
import { COCO_CLASSES } from './ai-labels';

export interface Detection {
  /** Bounding box [x1, y1, x2, y2] in normalized coordinates (0..1) */
  bbox: [number, number, number, number];
  confidence: number;
  classId: number;
  label: string;
}

interface RawDetection {
  x1: number;
  y1: number;
  x2: number;
  y2: number;
  score: number;
  classId: number;
}

interface SmoothedDetection {
  bbox: [number, number, number, number];
  score: number;
  classId: number;
  age: number;
}

export interface ObjectDetectorOptions {
  confidenceThreshold?: number;
  nmsThreshold?: number;
  emaAlpha?: number;
  maxAge?: number;
  enabledClasses?: string[] | null;
}

const INPUT_SIZE = 640;
const NUM_CLASSES = 80;
const CONFIDENCE_THRESHOLD = 0.65; // High confidence threshold (65%) for crisp, accurate detections
const NMS_THRESHOLD = 0.40; // Tighter IoU threshold
const EMA_ALPHA = 0.35;
const MAX_AGE = 10;
const MAX_VALID_LOGIT = 15; // Ignore corrupted decode artifacts
const MAX_DETECTIONS = 15; // Clean top detections limit

function sigmoid(x: number): number {
  return 1 / (1 + Math.exp(-x));
}

export class ObjectDetector {
  private runtime: AiRuntime;
  private options: Required<ObjectDetectorOptions>;
  private smoothedDetections: SmoothedDetection[] = [];
  private offscreenCanvas: OffscreenCanvas | null = null;
  private offscreenCtx: OffscreenCanvasRenderingContext2D | null = null;
  private frameCounter = 0;

  constructor(runtime: AiRuntime, options?: ObjectDetectorOptions) {
    this.runtime = runtime;
    this.options = {
      confidenceThreshold: options?.confidenceThreshold ?? CONFIDENCE_THRESHOLD,
      nmsThreshold: options?.nmsThreshold ?? NMS_THRESHOLD,
      emaAlpha: options?.emaAlpha ?? EMA_ALPHA,
      maxAge: options?.maxAge ?? MAX_AGE,
      enabledClasses: options?.enabledClasses ?? null,
    };
  }

  public updateOptions(options: ObjectDetectorOptions): void {
    if (options.confidenceThreshold !== undefined) this.options.confidenceThreshold = options.confidenceThreshold;
    if (options.nmsThreshold !== undefined) this.options.nmsThreshold = options.nmsThreshold;
    if (options.emaAlpha !== undefined) this.options.emaAlpha = options.emaAlpha;
    if (options.maxAge !== undefined) this.options.maxAge = options.maxAge;
    if (options.enabledClasses !== undefined) this.options.enabledClasses = options.enabledClasses;
  }

  public async detect(imageSource: ImageBitmap | HTMLVideoElement | HTMLCanvasElement): Promise<Detection[]> {
    if (!this.runtime.isReady()) {
      return [];
    }

    const { tensor, scale, padX, padY, originalWidth, originalHeight } = this.preprocess(imageSource);

    try {
      const t0 = performance.now();
      const results = await this.runtime.run({ images: tensor });
      const inferDurationMs = (performance.now() - t0).toFixed(1);

      const output = results['output0'] || Object.values(results)[0];
      if (!output) {
        return [];
      }

      const rawDetections = this.parseYoloOutput(output.data, output.dims, scale, padX, padY, originalWidth, originalHeight);
      const nmsDetections = this.applyNms(rawDetections, inferDurationMs);
      const smoothed = this.applyEmaSmoothing(nmsDetections);

      return smoothed.map((d) => ({
        bbox: d.bbox,
        confidence: d.score,
        classId: d.classId,
        label: COCO_CLASSES[d.classId] || 'object',
      }));
    } finally {
      tensor.dispose();
    }
  }

  public clearSmoothing(): void {
    this.smoothedDetections = [];
  }

  private preprocess(imageSource: ImageBitmap | HTMLVideoElement | HTMLCanvasElement) {
    const srcWidth = 'videoWidth' in imageSource ? imageSource.videoWidth : imageSource.width;
    const srcHeight = 'videoHeight' in imageSource ? imageSource.videoHeight : imageSource.height;

    if (!this.offscreenCanvas) {
      this.offscreenCanvas = new OffscreenCanvas(INPUT_SIZE, INPUT_SIZE);
      this.offscreenCtx = this.offscreenCanvas.getContext('2d', { willReadFrequently: true });
    }

    const ctx = this.offscreenCtx!;
    ctx.fillStyle = '#727272'; // Gray letterbox padding
    ctx.fillRect(0, 0, INPUT_SIZE, INPUT_SIZE);

    // Letterbox scale
    const scale = Math.min(INPUT_SIZE / (srcWidth || 640), INPUT_SIZE / (srcHeight || 480));
    const scaledWidth = Math.round((srcWidth || 640) * scale);
    const scaledHeight = Math.round((srcHeight || 480) * scale);
    const padX = Math.round((INPUT_SIZE - scaledWidth) / 2);
    const padY = Math.round((INPUT_SIZE - scaledHeight) / 2);

    ctx.drawImage(imageSource, padX, padY, scaledWidth, scaledHeight);
    const imgData = ctx.getImageData(0, 0, INPUT_SIZE, INPUT_SIZE);
    const pixels = imgData.data;

    // Convert RGBA -> Float32 planar RGB [1, 3, 640, 640] normalized to [0, 1]
    const floatData = new Float32Array(3 * INPUT_SIZE * INPUT_SIZE);
    const channelSize = INPUT_SIZE * INPUT_SIZE;

    for (let i = 0; i < channelSize; i++) {
      const srcOffset = i * 4;
      floatData[i] = pixels[srcOffset] / 255.0; // R
      floatData[channelSize + i] = pixels[srcOffset + 1] / 255.0; // G
      floatData[2 * channelSize + i] = pixels[srcOffset + 2] / 255.0; // B
    }

    const tensor = new ort.Tensor('float32', floatData, [1, 3, INPUT_SIZE, INPUT_SIZE]);

    return {
      tensor,
      scale,
      padX,
      padY,
      originalWidth: srcWidth || 640,
      originalHeight: srcHeight || 480,
    };
  }

  private parseYoloOutput(
    output: Float32Array,
    dims: number[],
    scale: number,
    padX: number,
    padY: number,
    origW: number,
    origH: number
  ): RawDetection[] {
    const detections: RawDetection[] = [];
    this.frameCounter++;

    // YOLOv11 tensor layout: [1, 84, 8400]
    let numBoxes = 8400;
    let isChannelsFirst = true;

    if (dims && dims.length >= 3) {
      if (dims[1] === 84 || dims[1] === (4 + NUM_CLASSES)) {
        isChannelsFirst = true;
        numBoxes = dims[2];
      } else if (dims[2] === 84 || dims[2] === (4 + NUM_CLASSES)) {
        isChannelsFirst = false;
        numBoxes = dims[1];
      }
    }

    for (let i = 0; i < numBoxes; i++) {
      let cx: number, cy: number, w: number, h: number;
      let maxRawScore = -Infinity;
      let maxClassId = 0;

      if (isChannelsFirst) {
        // [1, 84, numBoxes]
        cx = output[0 * numBoxes + i];
        cy = output[1 * numBoxes + i];
        w = output[2 * numBoxes + i];
        h = output[3 * numBoxes + i];

        for (let c = 0; c < NUM_CLASSES; c++) {
          const raw = output[(4 + c) * numBoxes + i];
          if (raw > maxRawScore) {
            maxRawScore = raw;
            maxClassId = c;
          }
        }
      } else {
        // [1, numBoxes, 84]
        const stride = 4 + NUM_CLASSES;
        const offset = i * stride;
        cx = output[offset];
        cy = output[offset + 1];
        w = output[offset + 2];
        h = output[offset + 3];

        for (let c = 0; c < NUM_CLASSES; c++) {
          const raw = output[offset + 4 + c];
          if (raw > maxRawScore) {
            maxRawScore = raw;
            maxClassId = c;
          }
        }
      }

      // Skip invalid dimensions or exploding decode artifacts
      if (w <= 0 || h <= 0 || maxRawScore > MAX_VALID_LOGIT) continue;

      // Apply Sigmoid to class logit to get confidence probability [0..1]
      const score = sigmoid(maxRawScore);

      // Filter by confidence threshold (52%)
      if (score >= this.options.confidenceThreshold) {
        if (this.options.enabledClasses && this.options.enabledClasses.length > 0) {
          const className = COCO_CLASSES[maxClassId];
          if (!this.options.enabledClasses.includes(className)) {
            continue;
          }
        }

        // Un-pad and scale back to original resolution, then normalize (0..1)
        const x1Pix = (cx - w / 2 - padX) / scale;
        const y1Pix = (cy - h / 2 - padY) / scale;
        const x2Pix = (cx + w / 2 - padX) / scale;
        const y2Pix = (cy + h / 2 - padY) / scale;

        const wPix = x2Pix - x1Pix;
        const hPix = y2Pix - y1Pix;

        // Reject tiny noise artifacts (< 24px)
        if (wPix < 24 || hPix < 24) continue;

        const x1 = Math.max(0, Math.min(1, x1Pix / origW));
        const y1 = Math.max(0, Math.min(1, y1Pix / origH));
        const x2 = Math.max(0, Math.min(1, x2Pix / origW));
        const y2 = Math.max(0, Math.min(1, y2Pix / origH));

        const normW = x2 - x1;
        const normH = y2 - y1;

        // Reject full-screen phantom overlay artifacts (> 90% of screen)
        if (normW > 0.90 && normH > 0.90) continue;

        if (normW > 0.03 && normH > 0.03) {
          detections.push({ x1, y1, x2, y2, score, classId: maxClassId });
        }
      }
    }

    return detections;
  }

  /**
   * Greedy Non-Maximum Suppression (NMS)
   * Suppresses overlapping boxes across all classes to eliminate duplicate/overlapping boxes.
   */
  private applyNms(detections: RawDetection[], inferDurationMs?: string): RawDetection[] {
    if (detections.length === 0) return [];

    // Sort by confidence descending
    const sorted = [...detections].sort((a, b) => b.score - a.score);
    const kept: RawDetection[] = [];

    while (sorted.length > 0 && kept.length < MAX_DETECTIONS) {
      const best = sorted.shift()!;
      kept.push(best);

      // Remove boxes that overlap significantly with best box
      const remaining: RawDetection[] = [];
      for (const det of sorted) {
        if (this.calculateIou(best, det) <= this.options.nmsThreshold) {
          remaining.push(det);
        }
      }
      sorted.length = 0;
      sorted.push(...remaining);
    }

    if (this.frameCounter % 10 === 0 || kept.length > 0) {
      console.log(
        `%c[AI Inference] ${inferDurationMs ? `${inferDurationMs}ms | ` : ''}Found ${kept.length} objects (top: ${kept[0] ? COCO_CLASSES[kept[0].classId] : 'none'}, score: ${((kept[0]?.score || 0) * 100).toFixed(1)}%)`,
        kept.length > 0 ? 'color: #10b981; font-weight: bold;' : 'color: #94a3b8;'
      );
    }

    return kept;
  }

  private calculateIou(a: RawDetection, b: RawDetection): number {
    const interX1 = Math.max(a.x1, b.x1);
    const interY1 = Math.max(a.y1, b.y1);
    const interX2 = Math.min(a.x2, b.x2);
    const interY2 = Math.min(a.y2, b.y2);

    const interW = Math.max(0, interX2 - interX1);
    const interH = Math.max(0, interY2 - interY1);
    const interArea = interW * interH;

    const areaA = (a.x2 - a.x1) * (a.y2 - a.y1);
    const areaB = (b.x2 - b.x1) * (b.y2 - b.y1);

    if (areaA === 0 || areaB === 0) return 0;

    return interArea / (areaA + areaB - interArea);
  }

  private applyEmaSmoothing(detections: RawDetection[]): SmoothedDetection[] {
    const alpha = this.options.emaAlpha;
    const newSmoothed: SmoothedDetection[] = [];
    const matchedOld = new Set<number>();

    for (const det of detections) {
      let bestMatchIdx = -1;
      let bestIou = 0.3; // Match threshold

      for (let i = 0; i < this.smoothedDetections.length; i++) {
        if (matchedOld.has(i)) continue;
        const old = this.smoothedDetections[i];
        if (old.classId !== det.classId) continue;

        const iou = this.calculateIou(
          det,
          { x1: old.bbox[0], y1: old.bbox[1], x2: old.bbox[2], y2: old.bbox[3], score: old.score, classId: old.classId }
        );

        if (iou > bestIou) {
          bestIou = iou;
          bestMatchIdx = i;
        }
      }

      if (bestMatchIdx !== -1) {
        matchedOld.add(bestMatchIdx);
        const old = this.smoothedDetections[bestMatchIdx];
        newSmoothed.push({
          bbox: [
            old.bbox[0] * (1 - alpha) + det.x1 * alpha,
            old.bbox[1] * (1 - alpha) + det.y1 * alpha,
            old.bbox[2] * (1 - alpha) + det.x2 * alpha,
            old.bbox[3] * (1 - alpha) + det.y2 * alpha,
          ],
          score: old.score * (1 - alpha) + det.score * alpha,
          classId: det.classId,
          age: 0,
        });
      } else {
        newSmoothed.push({
          bbox: [det.x1, det.y1, det.x2, det.y2],
          score: det.score,
          classId: det.classId,
          age: 0,
        });
      }
    }

    // Keep unmatched old detections up to maxAge
    for (let i = 0; i < this.smoothedDetections.length; i++) {
      if (!matchedOld.has(i)) {
        const old = this.smoothedDetections[i];
        if (old.age + 1 <= this.options.maxAge) {
          newSmoothed.push({
            ...old,
            age: old.age + 1,
          });
        }
      }
    }

    this.smoothedDetections = newSmoothed;
    return newSmoothed;
  }
}
