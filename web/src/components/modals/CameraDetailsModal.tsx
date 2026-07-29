import { X, Info, ShieldAlert } from 'lucide-react';
import type { CameraInfo } from '../../types';
import { formatBytes, formatUptime } from '../../utils/formatters';
import { VideoPlayer } from '../VideoPlayer';

interface CameraDetailsModalProps {
  detailsCam: CameraInfo;
  bitrates: Record<string, number>;
  fpsMap: Record<string, number>;
  onClose: () => void;
}

export function CameraDetailsModal({ detailsCam, bitrates, fpsMap, onClose }: CameraDetailsModalProps) {
  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal-content glass details-modal" onClick={e => e.stopPropagation()}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '1rem' }}>
          <h3 style={{ margin: 0, display: 'flex', alignItems: 'center', gap: '8px' }}>
            <Info size={20} color="var(--primary)" />
            {detailsCam.id.toUpperCase()} Details
          </h3>
          <button className="btn-icon" onClick={onClose}>
            <X size={20} />
          </button>
        </div>
        
        <div className="details-grid">
          <div className="details-stat">
            <div className="details-stat-label">Source URL (RTSP)</div>
            <div className="details-stat-val" style={{ wordBreak: 'break-all' }}>{detailsCam.url}</div>
          </div>
          <div className="details-stat">
            <div className="details-stat-label">Output URL (HLS)</div>
            <div className="details-stat-val">http://localhost:8080/stream/hls/{detailsCam.id}/index.m3u8</div>
          </div>
          <div className="details-stat">
            <div className="details-stat-label">Uptime</div>
            <div className="details-stat-val">{detailsCam.connected ? formatUptime(detailsCam.uptime) : 'Offline'}</div>
          </div>
          <div className="details-stat">
            <div className="details-stat-label">Current Bitrate</div>
            <div className="details-stat-val highlight">{detailsCam.connected ? `${bitrates[detailsCam.id]?.toFixed(2) || 0} kbps` : '0 kbps'}</div>
          </div>
          <div className="details-stat">
            <div className="details-stat-label">Current FPS</div>
            <div className="details-stat-val highlight">{detailsCam.connected ? `${fpsMap[detailsCam.id]?.toFixed(1) || 0} fps` : '0 fps'}</div>
          </div>
          <div className="details-stat">
            <div className="details-stat-label">Codec</div>
            <div className="details-stat-val">{detailsCam.codec}</div>
          </div>
          <div className="details-stat">
            <div className="details-stat-label">Total Frames</div>
            <div className="details-stat-val">{detailsCam.frames} <span style={{ color: '#64748b' }}>(Key: {detailsCam.keyFrames})</span></div>
          </div>
          <div className="details-stat">
            <div className="details-stat-label">Total Data Received</div>
            <div className="details-stat-val">{formatBytes(detailsCam.bytesReceived)}</div>
          </div>
          <div className="details-stat">
            <div className="details-stat-label">Total Data Sent (HLS)</div>
            <div className="details-stat-val">{formatBytes(detailsCam.bytesSent || 0)}</div>
          </div>
          <div className="details-stat">
            <div className="details-stat-label">Recording to fMP4</div>
            <div className="details-stat-val">{detailsCam.record ? 'Enabled' : 'Disabled'}</div>
          </div>
        </div>

        {detailsCam.connected ? (
          <div style={{ borderRadius: '8px', overflow: 'hidden', border: '1px solid var(--card-border)' }}>
            <VideoPlayer streamId={detailsCam.id} />
          </div>
        ) : (
          <div className="video-container" style={{ borderRadius: '8px', background: 'rgba(0,0,0,0.8)' }}>
            <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: '12px', color: 'var(--text-muted)' }}>
              <ShieldAlert size={36} style={{ color: 'var(--danger)', opacity: 0.8 }} />
              <span>Stream Disconnected</span>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
