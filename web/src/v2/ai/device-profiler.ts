/**
 * Device & Hardware Capability Profiler for Browser-side AI
 * Detects GPU architecture, device type, RAM, and CPU threads to select the optimal YOLO model tier.
 */

export type AiModelTier = 'nano' | 'small' | 'medium';
export type AiModelPreference = 'auto' | 'nano' | 'small' | 'medium';

export interface HardwareProfile {
  deviceType: 'desktop' | 'laptop' | 'tablet' | 'mobile';
  gpuVendor: string;
  gpuRenderer: string;
  isDiscreteGpu: boolean;
  hasWebGpu: boolean;
  cpuCores: number;
  memoryGb?: number;
  recommendedTier: AiModelTier;
  recommendedModelUrl: string;
  summary: string;
}

export const AI_MODELS_INFO: Record<
  AiModelTier,
  {
    name: string;
    description: string;
    sizeMb: number;
    parameters: string;
    modelUrl: string;
    confidenceThreshold: number;
    recommendedFor: string;
  }
> = {
  nano: {
    name: 'YOLOv11-Nano (Эконом)',
    description: 'Минимальное потребление батареи и трафика. Подходит для мобильных и слабых ПК.',
    sizeMb: 5.8,
    parameters: '2.6M',
    modelUrl: '/models/yolo11n.onnx',
    confidenceThreshold: 0.55,
    recommendedFor: 'Смартфоны, планшеты, CPU (WASM)',
  },
  small: {
    name: 'YOLOv11-Small (Баланс)',
    description: 'Оптимальный баланс высокой точности и скорости. Быстрая детекция на ультрабуках и встроенной графике.',
    sizeMb: 20.1,
    parameters: '9.4M',
    modelUrl: '/models/yolo11s.onnx',
    confidenceThreshold: 0.58,
    recommendedFor: 'Ноутбуки, ПК с Intel Iris / Apple M1, мощные планшеты',
  },
  medium: {
    name: 'YOLOv11-Medium (Максимальная точность)',
    description: 'Профессиональная детекция. Различает мелкие объекты вдалеке, людей под любым углом и в сложной одежде.',
    sizeMb: 41.2,
    parameters: '20.1M',
    modelUrl: '/models/yolo11m.onnx',
    confidenceThreshold: 0.62,
    recommendedFor: 'Десктопы с дискретными видеокартами (NVIDIA / AMD / Apple Pro/Max)',
  },
};

const PREFERENCE_STORAGE_KEY = 'ruseon_ai_model_preference';

let cachedProfile: HardwareProfile | null = null;

/**
 * Inspects browser hardware capabilities and determines the optimal AI model.
 */
export async function getHardwareProfile(): Promise<HardwareProfile> {
  if (cachedProfile) return cachedProfile;

  const isMobile =
    (navigator as any).userAgentData?.mobile ||
    /Android|iPhone|iPad|iPod|BlackBerry|IEMobile|Opera Mini/i.test(navigator.userAgent);

  const isTablet =
    /iPad|Tablet/i.test(navigator.userAgent) ||
    (isMobile && Math.min(window.screen.width, window.screen.height) > 600);

  const cpuCores = navigator.hardwareConcurrency || 4;
  const memoryGb = (navigator as any).deviceMemory || 8;

  let hasWebGpu = false;
  let gpuVendor = 'Unknown';
  let gpuRenderer = 'Default Graphics';
  let isDiscreteGpu = false;

  // 1. Check WebGPU capabilities
  if (typeof navigator !== 'undefined' && 'gpu' in navigator && (navigator as any).gpu) {
    try {
      const adapter = await (navigator as any).gpu.requestAdapter();
      if (adapter) {
        hasWebGpu = true;
        const info = (await adapter.requestAdapterInfo?.()) || adapter.info || {};
        gpuVendor = info.vendor || '';
        gpuRenderer = info.architecture || info.device || info.description || '';

        const lowGpu = (gpuVendor + ' ' + gpuRenderer).toLowerCase();
        if (
          lowGpu.includes('nvidia') ||
          lowGpu.includes('geforce') ||
          lowGpu.includes('rtx') ||
          lowGpu.includes('gtx') ||
          lowGpu.includes('radeon rx') ||
          lowGpu.includes('apple m1 pro') ||
          lowGpu.includes('apple m2 pro') ||
          lowGpu.includes('apple m3 pro') ||
          lowGpu.includes('apple m1 max') ||
          lowGpu.includes('apple m2 max') ||
          lowGpu.includes('apple m3 max') ||
          lowGpu.includes('apple m4')
        ) {
          isDiscreteGpu = true;
        }
      }
    } catch {
      // Ignore WebGPU adapter error
    }
  }

  // 2. Fallback to WebGL Renderer unmasked string for GPU identification
  if (!gpuRenderer || gpuRenderer === 'Default Graphics') {
    try {
      const canvas = document.createElement('canvas');
      const gl = canvas.getContext('webgl') || canvas.getContext('experimental-webgl');
      if (gl) {
        const debugInfo = (gl as WebGLRenderingContext).getExtension('WEBGL_debug_renderer_info');
        if (debugInfo) {
          gpuRenderer = (gl as WebGLRenderingContext).getParameter(debugInfo.UNMASKED_RENDERER_WEBGL) || '';
          gpuVendor = (gl as WebGLRenderingContext).getParameter(debugInfo.UNMASKED_VENDOR_WEBGL) || '';

          const low = (gpuVendor + ' ' + gpuRenderer).toLowerCase();
          if (
            low.includes('nvidia') ||
            low.includes('geforce') ||
            low.includes('rtx') ||
            low.includes('gtx') ||
            low.includes('radeon rx') ||
            low.includes('apple m1 pro') ||
            low.includes('apple m2 pro') ||
            low.includes('apple m3 pro') ||
            low.includes('apple m1 max') ||
            low.includes('apple m2 max') ||
            low.includes('apple m3 max')
          ) {
            isDiscreteGpu = true;
          }
        }
      }
    } catch {
      // Ignore WebGL error
    }
  }

  // 3. Determine device type
  let deviceType: HardwareProfile['deviceType'] = 'desktop';
  if (isMobile && !isTablet) {
    deviceType = 'mobile';
  } else if (isTablet) {
    deviceType = 'tablet';
  } else if (/Macintosh|Windows|Linux/i.test(navigator.userAgent) && !isDiscreteGpu && cpuCores <= 8) {
    deviceType = 'laptop';
  }

  // 4. Determine Recommended Tier
  let recommendedTier: AiModelTier = 'small';

  if (!hasWebGpu || deviceType === 'mobile' || memoryGb <= 4 || cpuCores <= 4) {
    recommendedTier = 'nano';
  } else if (isDiscreteGpu && hasWebGpu && memoryGb >= 8) {
    recommendedTier = 'medium';
  } else {
    recommendedTier = 'small';
  }

  const cleanGpuName = gpuRenderer && gpuRenderer !== 'Default Graphics'
    ? gpuRenderer.replace(/\(R\)|\(TM\)/gi, '').trim()
    : hasWebGpu ? 'WebGPU-совместимый акселератор' : 'CPU Software Rasterizer';

  const summary = `${deviceType.toUpperCase()} | ${cleanGpuName} | ${memoryGb}GB RAM (${cpuCores} cores)`;

  cachedProfile = {
    deviceType,
    gpuVendor,
    gpuRenderer: cleanGpuName,
    isDiscreteGpu,
    hasWebGpu,
    cpuCores,
    memoryGb,
    recommendedTier,
    recommendedModelUrl: AI_MODELS_INFO[recommendedTier].modelUrl,
    summary,
  };

  console.log(`%c[Hardware Profiler] Detected: ${summary} -> Recommended AI Model: ${recommendedTier.toUpperCase()}`, 'color: #3b82f6; font-weight: bold;');

  return cachedProfile;
}

/**
 * Gets the current user preference for AI model ('auto', 'nano', 'small', 'medium').
 */
export function getAiModelPreference(): AiModelPreference {
  try {
    const saved = localStorage.getItem(PREFERENCE_STORAGE_KEY) as AiModelPreference;
    if (saved && (saved === 'auto' || saved === 'nano' || saved === 'small' || saved === 'medium')) {
      return saved;
    }
  } catch {}
  return 'auto';
}

/**
 * Sets user preference for AI model.
 */
export function setAiModelPreference(pref: AiModelPreference): void {
  try {
    localStorage.setItem(PREFERENCE_STORAGE_KEY, pref);
  } catch {}
}

/**
 * Resolves the active model URL based on user preference and hardware profile.
 */
export async function getEffectiveModelUrl(): Promise<{ modelUrl: string; tier: AiModelTier; isAuto: boolean }> {
  const pref = getAiModelPreference();
  const profile = await getHardwareProfile();

  if (pref !== 'auto') {
    return {
      modelUrl: AI_MODELS_INFO[pref].modelUrl,
      tier: pref,
      isAuto: false,
    };
  }

  return {
    modelUrl: profile.recommendedModelUrl,
    tier: profile.recommendedTier,
    isAuto: true,
  };
}
