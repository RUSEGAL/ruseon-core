// Styles
import './styles/v2.css';

// Core
export { globalPlayerOrchestrator, PlayerOrchestrator } from './core/orchestrator';
export type { StreamingProtocol } from './core/orchestrator';
export { probeBrowserCapabilities, getCachedCapabilities } from './core/capabilities';
export { globalReconnectCoordinator } from './core/reconnect-coordinator';
export * from './core/timeline-math';

// Hooks
export { usePageVisibility } from './hooks/usePageVisibility';
export { useSurveillanceLayout } from './hooks/useSurveillanceLayout';

// Components
export { V2Layout } from './components/layout/V2Layout';
export { UniversalCameraPlayer } from './components/player/UniversalCameraPlayer';
export { WebCodecsPlayerV2 } from './components/player/WebCodecsPlayerV2';
export { WebRTCPlayerV2 } from './components/player/WebRTCPlayerV2';
export { HlsPlayerV2 } from './components/player/HlsPlayerV2';
export { SurveillanceView } from './components/surveillance/SurveillanceView';
export { ArchiveView } from './components/archive/ArchiveView';
export { ArchiveTimelineBar } from './components/archive/ArchiveTimelineBar';

// Modals
export { V2CameraDetailsModal } from './components/modals/V2CameraDetailsModal';
export { V2CameraFormModal } from './components/modals/V2CameraFormModal';
export { V2TagManagerModal } from './components/modals/V2TagManagerModal';
export { V2FolderManagerModal } from './components/modals/V2FolderManagerModal';
export { V2LogsModal } from './components/modals/V2LogsModal';
export { V2ServerStatsModal } from './components/modals/V2ServerStatsModal';
export { V2UserManagerModal } from './components/modals/V2UserManagerModal';

// AI Detection (WebGPU + Server Hybrid)
export { AiOverlay } from './ai/AiOverlay';
export { globalInferenceClient } from './ai/inference-client';
export { useCameraAiDetection } from './hooks/useCameraAiDetection';
