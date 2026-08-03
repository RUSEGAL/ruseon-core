import { Trash2, Edit2, ShieldAlert } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import type { CameraInfo, TagConfig } from '../types';
import { VideoPlayer } from './VideoPlayer';
import { formatBytes } from '../utils/formatters';

interface CameraGridProps {
  cameras: CameraInfo[];
  onEdit: (cam: CameraInfo) => void;
  onDelete: (id: string) => void;
  onOpenDetails: (cam: CameraInfo) => void;
  globalTags: TagConfig[];
}

export function CameraGrid({ cameras, onEdit, onDelete, onOpenDetails, globalTags }: CameraGridProps) {
  const { t } = useTranslation();
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
              <button className="btn-icon" onClick={(e) => { e.stopPropagation(); onEdit(cam); }} title={t('cameras.edit')}>
                <Edit2 size={16} />
              </button>
              <button className="btn-icon btn-danger" onClick={(e) => { e.stopPropagation(); onDelete(cam.id); }} title={t('cameras.delete')}>
                <Trash2 size={16} />
              </button>
            </div>
          </div>
          
          <div style={{ padding: '1.25rem', flex: 1, display: 'flex', flexDirection: 'column' }}>
            <div className="camera-info" style={{ marginBottom: '8px', display: 'flex', alignItems: 'center', gap: '8px' }}>
              <span style={{ color: 'var(--text-muted)', fontSize: '0.9rem' }}>Status:</span>
              <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                {cam.disabled ? (
                  <>
                    <div className="status-indicator error"></div>
                    <span style={{ fontWeight: 600, color: 'var(--danger)' }}>{t('cameras.status.disabled')}</span>
                  </>
                ) : (
                  <>
                    <div className={`status-indicator ${cam.connected ? 'online' : 'offline'}`}></div>
                    <span style={{ fontWeight: 600 }}>{cam.connected ? t('cameras.status.online') : t('cameras.status.offline')}</span>
                  </>
                )}
              </div>
            </div>
            
            {cam.record && (
              <div style={{ display: 'flex', alignItems: 'center', gap: '4px', fontSize: '0.8rem', color: 'var(--danger)', fontWeight: 'bold' }}>
                <div className="status-indicator" style={{ backgroundColor: 'var(--danger)', width: '8px', height: '8px' }}></div>
                REC
              </div>
            )}
            
            <div style={{ width: '100%', marginTop: '8px', marginBottom: '8px' }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '0.75rem', marginBottom: '4px' }}>
                <span style={{ color: 'var(--text-muted)' }}>Traffic Left</span>
                <span style={{ fontWeight: 600, color: (cam.trafficUsed || 0) > (cam.trafficLimit || 200*1024*1024*1024) * 0.9 ? 'var(--danger)' : 'var(--primary)' }}>
                  {formatBytes((cam.trafficLimit || 200*1024*1024*1024) - (cam.trafficUsed || 0))}
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

            {cam.tags && cam.tags.length > 0 && (
              <div style={{ display: 'flex', gap: '4px', flexWrap: 'wrap', marginTop: '8px', marginBottom: '8px' }}>
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
            
            <div className="video-container" style={{ marginTop: 'auto', cursor: 'pointer', borderRadius: '8px', overflow: 'hidden' }} onClick={() => onOpenDetails(cam)}>
              {cam.connected && !cam.disabled ? (
                <VideoPlayer streamId={cam.id} />
              ) : (
                <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: '12px', color: 'var(--text-muted)' }}>
                  <ShieldAlert size={36} style={{ color: 'var(--danger)', opacity: 0.8 }} />
                  <span>{cam.disabled ? t('cameras.status.disabled') : t('cameras.status.offline')}</span>
                </div>
              )}
            </div>
          </div>
        </div>
      ))}
    </div>
  );
}
