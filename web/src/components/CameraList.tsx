import { Trash2, Edit2 } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import type { CameraInfo, TagConfig } from '../types';
import { formatBytes, formatUptime } from '../utils/formatters';
import { useState } from 'react';
import { ChevronDown, ChevronRight, Folder as FolderIcon } from 'lucide-react';

interface CameraListProps {
  cameras: CameraInfo[];
  bitrates: Record<string, number>;
  fpsMap: Record<string, number>;
  onEdit: (cam: CameraInfo) => void;
  onDelete: (id: string) => void;
  onOpenDetails: (cam: CameraInfo) => void;
  globalTags: TagConfig[];
  folders: import('../types').FolderConfig[];
}

export function CameraList({ cameras, bitrates, fpsMap, onEdit, onDelete, onOpenDetails, globalTags, folders }: CameraListProps) {
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
              <div className="camera-list">
                {groupCams.map(cam => (
        <div key={cam.id} className="camera-list-item glass" onClick={() => onOpenDetails(cam)}>
          <div className="camera-list-info" style={{ flex: 1, display: 'flex', flexDirection: 'column', gap: '8px', justifyContent: 'center', alignItems: 'flex-start' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
              <div className={`status-indicator ${cam.connected && !cam.disabled ? 'online' : 'offline'}`} style={{ backgroundColor: cam.disabled ? 'var(--danger)' : undefined }}></div>
              <div style={{ fontWeight: 600, fontSize: '1.1rem', textTransform: 'uppercase' }}>{cam.id}</div>
              
              <div style={{ display: 'flex', gap: '8px', alignItems: 'center', marginLeft: '4px' }}>
                {cam.disabled && (
                  <span className="status-badge error">
                    {t('cameras.status.disabled')}: {cam.disableReason}
                  </span>
                )}
                {!cam.disabled && !cam.connected && (
                  <span className="status-badge error">{t('cameras.status.offline')}</span>
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
              <div style={{ fontSize: '0.8rem', color: 'var(--text-muted)' }}>{t('dashboard.uptime')}</div>
              <div style={{ fontWeight: 600 }}>{cam.connected ? formatUptime(cam.uptime) : '-'}</div>
            </div>
            <div style={{ minWidth: '120px' }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '0.75rem', marginBottom: '4px' }}>
                <span style={{ color: 'var(--text-muted)' }}>{t('cameras.traffic')}</span>
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
            <div style={{ textAlign: 'center' }}>
              <div style={{ fontSize: '0.8rem', color: 'var(--text-muted)' }}>{t('cameras.codec')}</div>
              <div style={{ fontWeight: 600 }}>{cam.codec || '-'} / {cam.connected ? `${fpsMap[cam.id]?.toFixed(1) || 0}` : '-'}</div>
            </div>
            <div style={{ textAlign: 'right', minWidth: '100px' }}>
              <div style={{ fontWeight: 600 }}>{cam.connected ? `${bitrates[cam.id]?.toFixed(2) || 0} kbps` : '-'}</div>
              <div style={{ fontSize: '0.8rem', color: 'var(--text-muted)' }}>{formatBytes(cam.bytesReceived)} {t('cameras.trafficTotal')}</div>
            </div>
            <div style={{ display: 'flex', gap: '8px' }}>
              <button className="btn-icon" onClick={(e) => { e.stopPropagation(); onEdit(cam); }} title={t('cameras.edit')}>
                <Edit2 size={16} />
              </button>
              <button className="btn-icon btn-danger" onClick={(e) => { e.stopPropagation(); onDelete(cam.id); }} title={t('cameras.delete')}>
                <Trash2 size={16} />
              </button>
            </div>
            </div>
          </div>
        ))}
        {groupCams.length === 0 && (
                <div style={{ textAlign: 'center', padding: '2rem', color: 'var(--text-muted)' }}>
                  {t('cameras.empty')}
                </div>
              )}
            </div>
            )}
          </div>
        );
      })}
      
      {cameras.length === 0 && (
        <div style={{ textAlign: 'center', padding: '2rem', color: 'var(--text-muted)' }}>
          {t('cameras.empty')}
        </div>
      )}
    </div>
  );
}
