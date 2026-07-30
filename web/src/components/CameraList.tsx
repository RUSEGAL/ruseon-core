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
          <div className="camera-list-info" style={{ flex: 1 }}>
            <div className={`status-indicator ${cam.connected ? 'online' : 'offline'}`}></div>
            <div style={{ fontWeight: 600, fontSize: '1.1rem', textTransform: 'uppercase', minWidth: '80px' }}>{cam.id}</div>
            <div style={{ color: 'var(--text-muted)', fontSize: '0.9rem', maxWidth: '250px', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{cam.url}</div>
            
            {cam.tags && cam.tags.length > 0 && (
              <div style={{ display: 'flex', gap: '4px', flexWrap: 'wrap', maxWidth: '200px' }}>
                {cam.tags.map(tId => {
                  const tag = globalTags.find(gt => gt.id === tId);
                  if (!tag) return null;
                  return (
                    <span key={tag.id} style={{ 
                      background: `${tag.color}33`, 
                      border: `1px solid ${tag.color}`,
                      color: '#fff',
                      padding: '2px 6px', 
                      borderRadius: '12px', 
                      fontSize: '0.7rem' 
                    }}>
                      {tag.name}
                    </span>
                  );
                })}
              </div>
            )}

            {cam.record && (
              <div style={{ display: 'flex', alignItems: 'center', gap: '4px', fontSize: '0.8rem', color: 'var(--danger)', fontWeight: 'bold' }}>
                <div className="status-indicator" style={{ backgroundColor: 'var(--danger)', width: '8px', height: '8px' }}></div>
                REC
              </div>
            )}
          </div>
          
          <div style={{ display: 'flex', alignItems: 'center', gap: '2rem', flex: 2, justifyContent: 'flex-end' }}>
            <div style={{ textAlign: 'center' }}>
              <div style={{ fontSize: '0.8rem', color: 'var(--text-muted)' }}>Uptime</div>
              <div style={{ fontWeight: 600 }}>{cam.connected ? formatUptime(cam.uptime) : '-'}</div>
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
