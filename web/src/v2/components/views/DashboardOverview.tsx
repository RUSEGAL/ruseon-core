import React from 'react';
import { useTranslation } from 'react-i18next';
import type { CameraInfo, ServerStats } from '../../../types';
import { UniversalCameraPlayer } from '../player/UniversalCameraPlayer';
import { Activity, Radio, HardDrive, Cpu, ArrowUpRight } from 'lucide-react';

interface DashboardOverviewProps {
  cameras: CameraInfo[];
  stats: ServerStats | null;
  onNavigateToSurveillance: () => void;
}

export const DashboardOverview: React.FC<DashboardOverviewProps> = ({
  cameras,
  stats,
  onNavigateToSurveillance,
}) => {
  const { t } = useTranslation();
  const onlineCameras = cameras.filter((c) => c.state === 'online');
  const recordingCameras = cameras.filter((c) => c.record);

  const formatTraffic = (bytes: number) => {
    if (bytes > 1024 * 1024 * 1024) {
      return `${(bytes / (1024 * 1024 * 1024)).toFixed(2)} GB`;
    }
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  };

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '1.5rem' }}>
      {/* Top Metric Cards */}
      <div
        style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(auto-fit, minmax(220px, 1fr))',
          gap: '1rem',
        }}
      >
        {/* Metric 1 */}
        <div
          className="glass"
          style={{
            padding: '1.2rem',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
          }}
        >
          <div>
            <div style={{ fontSize: '0.75rem', color: '#94a3b8', fontWeight: 600 }}>
              {t('v2.dashboard.onlineCameras', 'ONLINE CAMERAS')}
            </div>
            <div style={{ fontSize: '1.6rem', fontWeight: 700, color: '#10b981', marginTop: '4px' }}>
              {onlineCameras.length}{' '}
              <span style={{ fontSize: '0.9rem', color: '#64748b' }}>/ {cameras.length}</span>
            </div>
          </div>
          <div
            style={{
              width: '42px',
              height: '42px',
              borderRadius: '10px',
              background: 'rgba(16, 185, 129, 0.15)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
            }}
          >
            <Radio size={22} color="#10b981" />
          </div>
        </div>

        {/* Metric 2 */}
        <div
          className="glass"
          style={{
            padding: '1.2rem',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
          }}
        >
          <div>
            <div style={{ fontSize: '0.75rem', color: '#94a3b8', fontWeight: 600 }}>
              {t('v2.dashboard.continuousRecording', 'CONTINUOUS RECORDING')}
            </div>
            <div style={{ fontSize: '1.6rem', fontWeight: 700, color: '#ef4444', marginTop: '4px' }}>
              {recordingCameras.length}{' '}
              <span style={{ fontSize: '0.9rem', color: '#64748b' }}>{t('v2.dashboard.activeStreams', 'active streams')}</span>
            </div>
          </div>
          <div
            style={{
              width: '42px',
              height: '42px',
              borderRadius: '10px',
              background: 'rgba(239, 68, 68, 0.15)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
            }}
          >
            <HardDrive size={22} color="#ef4444" />
          </div>
        </div>

        {/* Metric 3 */}
        <div
          className="glass"
          style={{
            padding: '1.2rem',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
          }}
        >
          <div>
            <div style={{ fontSize: '0.75rem', color: '#94a3b8', fontWeight: 600 }}>
              {t('v2.dashboard.totalIngestTraffic', 'TOTAL INGEST TRAFFIC')}
            </div>
            <div style={{ fontSize: '1.6rem', fontWeight: 700, color: '#6366f1', marginTop: '4px' }}>
              {stats?.totalBytes ? formatTraffic(stats.totalBytes) : '0 MB'}
            </div>
          </div>
          <div
            style={{
              width: '42px',
              height: '42px',
              borderRadius: '10px',
              background: 'rgba(99, 102, 241, 0.15)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
            }}
          >
            <Activity size={22} color="#6366f1" />
          </div>
        </div>

        {/* Metric 4 */}
        <div
          className="glass"
          style={{
            padding: '1.2rem',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
          }}
        >
          <div>
            <div style={{ fontSize: '0.75rem', color: '#94a3b8', fontWeight: 600 }}>
              {t('v2.dashboard.totalFramesProcessed', 'TOTAL FRAMES PROCESSED')}
            </div>
            <div style={{ fontSize: '1.6rem', fontWeight: 700, color: '#38bdf8', marginTop: '4px' }}>
              {stats?.totalFrames ? stats.totalFrames.toLocaleString() : '0'}
            </div>
          </div>
          <div
            style={{
              width: '42px',
              height: '42px',
              borderRadius: '10px',
              background: 'rgba(56, 189, 248, 0.15)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
            }}
          >
            <Cpu size={22} color="#38bdf8" />
          </div>
        </div>
      </div>

      {/* Featured Live Cameras Section */}
      <div style={{ display: 'flex', flexDirection: 'column', gap: '0.75rem' }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <h2 style={{ fontSize: '1.1rem', fontWeight: 700, color: '#f8fafc' }}>
            {t('v2.dashboard.livePreview', 'Live Camera Feeds Preview')}
          </h2>
          <button
            onClick={onNavigateToSurveillance}
            style={{
              background: 'transparent',
              border: 'none',
              color: '#a5b4fc',
              fontSize: '0.82rem',
              fontWeight: 600,
              cursor: 'pointer',
              display: 'flex',
              alignItems: 'center',
              gap: '4px',
            }}
          >
            <span>{t('v2.dashboard.openGrid', 'Open Full Surveillance Grid')}</span>
            <ArrowUpRight size={14} />
          </button>
        </div>

        <div
          style={{
            display: 'grid',
            gridTemplateColumns: 'repeat(auto-fill, minmax(360px, 1fr))',
            gap: '1rem',
          }}
        >
          {cameras.slice(0, 4).map((cam) => (
            <div
              key={cam.id}
              className="glass"
              style={{
                height: '240px',
                borderRadius: '12px',
                overflow: 'hidden',
                position: 'relative',
              }}
            >
              <UniversalCameraPlayer
                cameraId={cam.id}
                cameraName={cam.id}
                codec={cam.codec}
                isLive={true}
              />
            </div>
          ))}
        </div>
      </div>
    </div>
  );
};
