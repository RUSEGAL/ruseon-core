import { Trash2, Edit2 } from 'lucide-react';
import type { CameraInfo, TagConfig } from '../types';
import { formatBytes, formatUptime } from '../utils/formatters';

interface CameraListProps {
  cameras: CameraInfo[];
  bitrates: Record<string, number>;
  fpsMap: Record<string, number>;
  onEdit: (cam: CameraInfo) => void;
  onDelete: (id: string) => void;
  onOpenDetails: (cam: CameraInfo) => void;
  globalTags: TagConfig[];
}

export function CameraList({ cameras, bitrates, fpsMap, onEdit, onDelete, onOpenDetails, globalTags }: CameraListProps) {
  return (
    <div className="camera-list">
      {cameras.map(cam => (
        <div key={cam.id} className="camera-list-item glass" onClick={() => onOpenDetails(cam)}>
          <div className="camera-list-info" style={{ flex: 1, display: 'flex', flexDirection: 'column', gap: '6px', justifyContent: 'center' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
              <div className={`status-indicator ${cam.connected && !cam.disabled ? 'online' : 'offline'}`} style={{ backgroundColor: cam.disabled ? 'var(--danger)' : undefined }}></div>
              <div style={{ fontWeight: 600, fontSize: '1.1rem', textTransform: 'uppercase' }}>{cam.id}</div>
              
              <div style={{ display: 'flex', gap: '8px', alignItems: 'center', marginLeft: '4px' }}>
                {cam.disabled && (
                  <span className="status-badge error">
                    Disabled: {cam.disableReason === 'technical' ? 'Tech. Issue' : cam.disableReason === 'requested' ? 'By User' : cam.disableReason === 'payment' ? 'Unpaid Bill' : cam.disableReason}
                  </span>
                )}
                {!cam.disabled && !cam.connected && (
                  <span className="status-badge error">Offline</span>
                )}
                {cam.record && (
                  <span className="status-badge warning" style={{ display: 'flex', alignItems: 'center', gap: '4px' }}>
                    <div className="status-indicator" style={{ backgroundColor: 'var(--danger)', width: '6px', height: '6px', margin: 0 }}></div>
                    REC
                  </span>
                )}
              </div>
            </div>

            {cam.tags && cam.tags.length > 0 && (
              <div style={{ display: 'flex', gap: '6px', flexWrap: 'wrap', flex: 1 }}>
                {cam.tags.map(tId => {
                  const tag = globalTags.find(gt => gt.id === tId);
                  if (!tag) return null;
                  return (
                    <span key={tag.id} style={{ 
                      background: `${tag.color}33`, 
                      border: `1px solid ${tag.color}`,
                      color: '#fff',
                      padding: '2px 8px', 
                      borderRadius: '12px', 
                      fontSize: '0.75rem',
                      whiteSpace: 'nowrap'
                    }}>
                      {tag.name}
                    </span>
                  );
                })}
              </div>
            )}
          </div>
          
          <div style={{ display: 'flex', alignItems: 'center', gap: '2rem', flex: 2, justifyContent: 'flex-end' }}>
            <div style={{ textAlign: 'center' }}>
              <div style={{ fontSize: '0.8rem', color: 'var(--text-muted)' }}>Uptime</div>
              <div style={{ fontWeight: 600 }}>{cam.connected ? formatUptime(cam.uptime) : '-'}</div>
            </div>
            <div style={{ minWidth: '120px' }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '0.75rem', marginBottom: '4px' }}>
                <span style={{ color: 'var(--text-muted)' }}>Traffic</span>
                <span style={{ fontWeight: 600, color: (cam.trafficUsed || 0) > (cam.trafficLimit || 200*1024*1024*1024) * 0.9 ? 'var(--danger)' : 'var(--primary)' }}>
                  {formatBytes((cam.trafficLimit || 200*1024*1024*1024) - (cam.trafficUsed || 0))} left
                </span>
              </div>
              <div style={{ width: '100%', height: '6px', background: 'var(--card-bg)', borderRadius: '3px', overflow: 'hidden', border: '1px solid var(--card-border)' }}>
                <div style={{ 
                  height: '100%', 
                  width: `${Math.min(100, ((cam.trafficUsed || 0) / (cam.trafficLimit || 200*1024*1024*1024)) * 100)}%`,
                  background: ((cam.trafficUsed || 0) / (cam.trafficLimit || 200*1024*1024*1024)) > 0.9 ? 'var(--danger)' : 'var(--primary)'
                }}></div>
              </div>
            </div>
            <div style={{ textAlign: 'center' }}>
              <div style={{ fontSize: '0.8rem', color: 'var(--text-muted)' }}>Codec / FPS</div>
              <div style={{ fontWeight: 600 }}>{cam.codec || '-'} / {cam.connected ? `${fpsMap[cam.id]?.toFixed(1) || 0}` : '-'}</div>
            </div>
            <div style={{ textAlign: 'right', minWidth: '100px' }}>
              <div style={{ fontWeight: 600 }}>{cam.connected ? `${bitrates[cam.id]?.toFixed(2) || 0} kbps` : '-'}</div>
              <div style={{ fontSize: '0.8rem', color: 'var(--text-muted)' }}>{formatBytes(cam.bytesReceived)} Total</div>
            </div>
            <div style={{ display: 'flex', gap: '8px' }}>
              <button className="btn-icon" onClick={(e) => { e.stopPropagation(); onEdit(cam); }} title="Edit">
                <Edit2 size={16} />
              </button>
              <button className="btn-icon btn-danger" onClick={(e) => { e.stopPropagation(); onDelete(cam.id); }} title="Delete">
                <Trash2 size={16} />
              </button>
            </div>
          </div>
        </div>
      ))}
      {cameras.length === 0 && (
        <div style={{ textAlign: 'center', padding: '2rem', color: 'var(--text-muted)' }}>
          No cameras configured yet. Click "Add Camera" to start.
        </div>
      )}
    </div>
  );
}
