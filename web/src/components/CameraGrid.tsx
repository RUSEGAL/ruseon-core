import { Trash2, Edit2, ShieldAlert } from 'lucide-react';
import type { CameraInfo } from '../types';
import { VideoPlayer } from './VideoPlayer';

interface CameraGridProps {
  cameras: CameraInfo[];
  onEdit: (cam: CameraInfo) => void;
  onDelete: (id: string) => void;
  onOpenDetails: (cam: CameraInfo) => void;
}

export function CameraGrid({ cameras, onEdit, onDelete, onOpenDetails }: CameraGridProps) {
  return (
    <div className="camera-grid">
      {cameras.map(cam => (
        <div key={cam.id} className="camera-card glass" onClick={(e) => {
          // If clicked directly on the card background, open details
          if ((e.target as HTMLElement).className.includes('camera-card') || (e.target as HTMLElement).className.includes('camera-info')) {
            onOpenDetails(cam);
          }
        }}>
          <div className="camera-header">
            <h3 style={{ margin: 0, textTransform: 'uppercase', letterSpacing: '1px' }}>{cam.id}</h3>
            <div style={{ display: 'flex', gap: '8px' }}>
              <button className="btn-icon" onClick={(e) => { e.stopPropagation(); onEdit(cam); }} title="Edit">
                <Edit2 size={16} />
              </button>
              <button className="btn-icon btn-danger" onClick={(e) => { e.stopPropagation(); onDelete(cam.id); }} title="Delete">
                <Trash2 size={16} />
              </button>
            </div>
          </div>
          
          <div style={{ padding: '1.25rem', flex: 1, display: 'flex', flexDirection: 'column' }}>
            <div className="camera-info" style={{ marginBottom: '8px', display: 'flex', alignItems: 'center', gap: '8px' }}>
              <span style={{ color: 'var(--text-muted)', fontSize: '0.9rem' }}>Status:</span>
              <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                <div className={`status-indicator ${cam.connected ? 'online' : 'offline'}`}></div>
                <span style={{ fontWeight: 600 }}>{cam.connected ? 'Online' : 'Offline'}</span>
              </div>
            </div>
            
            {cam.record && (
              <div style={{ display: 'flex', alignItems: 'center', gap: '4px', fontSize: '0.8rem', color: 'var(--danger)', fontWeight: 'bold' }}>
                <div className="status-indicator" style={{ backgroundColor: 'var(--danger)', width: '8px', height: '8px' }}></div>
                REC
              </div>
            )}
            
            <div className="video-container" style={{ marginTop: 'auto', cursor: 'pointer', borderRadius: '8px', overflow: 'hidden' }} onClick={() => onOpenDetails(cam)}>
              {cam.connected ? (
                <VideoPlayer streamId={cam.id} />
              ) : (
                <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: '12px', color: 'var(--text-muted)' }}>
                  <ShieldAlert size={36} style={{ color: 'var(--danger)', opacity: 0.8 }} />
                  <span>Stream Disconnected</span>
                </div>
              )}
            </div>
          </div>
        </div>
      ))}
    </div>
  );
}
