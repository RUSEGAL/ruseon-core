export interface CameraInfo {
  id: string;
  url: string;
  state: 'connecting' | 'online' | 'degraded' | 'offline' | 'disabled';
  record?: boolean;
  retentionDays?: number;
  tags?: string[];
  folderId?: string;
  comment?: string;
  simPhone?: string;
  simICCID?: string;
  trafficLimit?: number;
  trafficUsed: number;
  uptime: number;
  bytesReceived: number;
  bytesSent: number;
  frames: number;
  keyFrames: number;
  codec: string;
  lastFrameTime?: number;
  lastKeyTime?: number;
  lastError?: string;
  reconnects?: number;
  bitrate?: number;
  lazyHLS?: boolean;
  tokenAuth?: boolean;
  transport?: string;
  disabled: boolean;
  disableReason: string;
  disableHistory: {
    timestamp: string;
    action: string;
    reason: string;
  }[];
  recordHistory: {
    timestamp: string;
    action: string;
    reason: string;
  }[];
}

export interface HlsTelemetry {
  bandwidth: number;
  bufferLength: number;
  droppedFrames: number;
  latency: number;
}

export interface TagConfig {
  id: string;
  name: string;
  color: string;
}

export interface FolderConfig {
  id: string;
  name: string;
}

export interface ServerStats {
  uptime: number;
  memoryUsed: number;
  goroutines: number;
  totalCameras: number;
  onlineCameras: number;
  disabledCameras: number;
  disabledReasons: Record<string, number>;
  totalBytes: number;
  totalBytesSent: number;
  totalFrames: number;
  activeClients: number;
  clients: { ip: string; streamId: string }[];
  sysMemory?: number;
  heapAlloc?: number;
  heapSys?: number;
  heapObjects?: number;
  numGC?: number;
  numCPU?: number;
}

export interface DetectedObject {
  class?: string;
  className?: string;
  confidence: number;
  box?: [number, number, number, number];
  x?: number;
  y?: number;
  w?: number;
  h?: number;
  trackId?: number;
}

export interface MetadataPayload {
  timestamp: number;
  objects: DetectedObject[];
}
