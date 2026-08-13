import { Trash2, Edit2, ShieldAlert } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import type { CameraInfo, TagConfig } from '../types';
import { VideoPlayer } from './VideoPlayer';
import { formatBytes } from '../utils/formatters';
import { useState } from 'react';
import { ChevronDown, ChevronRight, Folder as FolderIcon } from 'lucide-react';

interface CameraGridProps {
  cameras: CameraInfo[];
  onEdit: (cam: CameraInfo) => void;
  onDelete: (id: string) => void;
  onOpenDetails: (cam: CameraInfo) => void;
  globalTags: TagConfig[];
  folders: import('../types').FolderConfig[];
}

export function CameraGrid({ cameras, onEdit, onDelete, onOpenDetails, globalTags, folders }: CameraGridProps) {
  const { t } = useTranslation();
  const [collapsedFolders, setCollapsedFolders] = useState<Record<string, boolean>>({});

  const toggleFolder = (folderId: string) => {
    setCollapsedFolders(prev => ({ ...prev, [folderId]: !prev[folderId] }));
  };

  // Group cameras
  const groupedCameras = cameras.reduce((acc, cam) => {
    const fId = cam.folderId || 'unassigned';
    if (!acc[fId]) acc[fId] = [];
    acc[fId].push(cam);
    return acc;
  }, {} as Record<string, CameraInfo[]>);

  // Sort folder groups (unassigned goes last)
  const folderGroups = Object.keys(groupedCameras).sort((a, b) => {
    if (a === 'unassigned') return 1;
    if (b === 'unassigned') return -1;
    const folderA = folders.find(f => f.id === a)?.name || a;
    const folderB = folders.find(f => f.id === b)?.name || b;
    return folderA.localeCompare(folderB);
  });

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '24px' }}>
      {folderGroups.map(folderId => {
        const groupCams = groupedCameras[folderId];
        const isCollapsed = collapsedFolders[folderId];
        const folderName = folderId === 'unassigned' 
          ? t('folders.unassigned', 'Unassigned') 
          : folders.find(f => f.id === folderId)?.name || 'Unknown Folder';

        return (
          <div key={folderId} className="folder-group">
            <div 
              onClick={() => toggleFolder(folderId)}
              style={{ 
                display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer', 
                padding: '8px 12px', background: 'rgba(255,255,255,0.05)', 
                borderRadius: '8px', marginBottom: '12px' 
              }}
            >
              {isCollapsed ? <ChevronRight size={18} /> : <ChevronDown size={18} />}
              <FolderIcon size={18} color="var(--primary)" />
              <h3 style={{ margin: 0, fontSize: '1.1rem' }}>{folderName} <span style={{ color: 'var(--text-muted)', fontSize: '0.9rem' }}>({groupCams.length})</span></h3>
            </div>
            
            {!isCollapsed && (
              <div className="camera-grid">
                {groupCams.map(cam => (
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
                    <div className={`status-indicator ${cam.state === 'online' ? 'online' : 'offline'}`}></div>
                    <span style={{ fontWeight: 600 }}>{cam.state === 'online' ? t('cameras.status.online') : t('cameras.status.offline')}</span>
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
                <span style={{ color: 'var(--text-muted)' }}>{t('cameras.trafficLeft')}</span>
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
              {cam.state === 'online' && !cam.disabled ? (
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
            )}
          </div>
        );
      })}
    </div>
  );
}
