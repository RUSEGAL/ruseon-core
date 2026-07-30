export interface CameraInfo {
  id: string;
  url: string;
  connected: boolean;
  record?: boolean;
  retentionDays?: number;
  tags?: string[];
  comment?: string;
  simPhone?: string;
  simICCID?: string;
  uptime: number;
  bytesReceived: number;
  bytesSent: number;
  frames: number;
  keyFrames: number;
  codec: string;
}

export interface TagConfig {
  id: string;
  name: string;
  color: string;
}

export interface ServerStats {
  uptime: number;
  memoryUsed: number;
  goroutines: number;
  totalCameras: number;
  onlineCameras: number;
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
