import React from 'react';
import type { CameraInfo, TagConfig, FolderConfig } from '../../../types';
import {
  Plus,
  Edit2,
  Trash2,
  Settings,
  Folder,
  Tag as TagIcon,
  Users,
  Terminal,
  Info,
  Phone,
} from 'lucide-react';

interface SettingsViewProps {
  cameras: CameraInfo[];
  tags: TagConfig[];
  folders: FolderConfig[];
  userRole?: string;
  onAddCamera: () => void;
  onEditCamera: (cam: CameraInfo) => void;
  onDeleteCamera: (id: string) => void;
  onOpenDetails: (cam: CameraInfo) => void;
  onOpenFolders?: () => void;
  onOpenTags?: () => void;
  onOpenUsers?: () => void;
  onOpenLogs?: () => void;
}

export const SettingsView: React.FC<SettingsViewProps> = ({
  cameras,
  tags,
  folders,
  userRole,
  onAddCamera,
  onEditCamera,
  onDeleteCamera,
  onOpenDetails,
  onOpenFolders,
  onOpenTags,
  onOpenUsers,
  onOpenLogs,
}) => {
  const isAdmin = userRole === 'admin';
  const isOperator = userRole === 'operator';
  const canEdit = isAdmin || isOperator;

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '1.5rem' }}>
      {/* Top Toolbar with Management Modals */}
      <div className="v2-grid-toolbar">
        <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
          <Settings size={18} color="#6366f1" />
          <h2 style={{ fontSize: '1.1rem', fontWeight: 600, color: '#f8fafc' }}>
            Camera & Resource Management
          </h2>
        </div>

        {/* Action Buttons */}
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px', flexWrap: 'wrap' }}>
          {isAdmin && onOpenFolders && (
            <button
              onClick={onOpenFolders}
              style={{
                background: 'rgba(255, 255, 255, 0.05)',
                border: '1px solid rgba(255, 255, 255, 0.1)',
                borderRadius: '8px',
                padding: '6px 12px',
                color: '#f8fafc',
                fontSize: '0.78rem',
                cursor: 'pointer',
                display: 'flex',
                alignItems: 'center',
                gap: '6px',
              }}
            >
              <Folder size={14} color="#6366f1" />
              <span>Folders ({folders.length})</span>
            </button>
          )}

          {isAdmin && onOpenTags && (
            <button
              onClick={onOpenTags}
              style={{
                background: 'rgba(255, 255, 255, 0.05)',
                border: '1px solid rgba(255, 255, 255, 0.1)',
                borderRadius: '8px',
                padding: '6px 12px',
                color: '#f8fafc',
                fontSize: '0.78rem',
                cursor: 'pointer',
                display: 'flex',
                alignItems: 'center',
                gap: '6px',
              }}
            >
              <TagIcon size={14} color="#10b981" />
              <span>Tags ({tags.length})</span>
            </button>
          )}

          {isAdmin && onOpenUsers && (
            <button
              onClick={onOpenUsers}
              style={{
                background: 'rgba(255, 255, 255, 0.05)',
                border: '1px solid rgba(255, 255, 255, 0.1)',
                borderRadius: '8px',
                padding: '6px 12px',
                color: '#f8fafc',
                fontSize: '0.78rem',
                cursor: 'pointer',
                display: 'flex',
                alignItems: 'center',
                gap: '6px',
              }}
            >
              <Users size={14} color="#a5b4fc" />
              <span>Users</span>
            </button>
          )}

          {isAdmin && onOpenLogs && (
            <button
              onClick={onOpenLogs}
              style={{
                background: 'rgba(255, 255, 255, 0.05)',
                border: '1px solid rgba(255, 255, 255, 0.1)',
                borderRadius: '8px',
                padding: '6px 12px',
                color: '#f8fafc',
                fontSize: '0.78rem',
                cursor: 'pointer',
                display: 'flex',
                alignItems: 'center',
                gap: '6px',
              }}
            >
              <Terminal size={14} color="#38bdf8" />
              <span>Logs</span>
            </button>
          )}

          {canEdit && (
            <button
              onClick={onAddCamera}
              style={{
                background: 'var(--v2-primary)',
                border: 'none',
                borderRadius: '8px',
                padding: '6px 14px',
                color: '#fff',
                fontSize: '0.8rem',
                fontWeight: 600,
                cursor: 'pointer',
                display: 'flex',
                alignItems: 'center',
                gap: '6px',
              }}
            >
              <Plus size={15} />
              <span>Add Camera</span>
            </button>
          )}
        </div>
      </div>

      {/* Main Cameras Table */}
      <div className="glass" style={{ padding: '1rem', borderRadius: '12px' }}>
        <table style={{ width: '100%', borderCollapse: 'collapse', textAlign: 'left', fontSize: '0.82rem' }}>
          <thead>
            <tr style={{ borderBottom: '1px solid var(--v2-card-border)', color: '#94a3b8' }}>
              <th style={{ padding: '10px' }}>ID / Name</th>
              <th style={{ padding: '10px' }}>Folder / Location</th>
              <th style={{ padding: '10px' }}>Tags</th>
              <th style={{ padding: '10px' }}>Stream URL</th>
              <th style={{ padding: '10px' }}>Recording</th>
              <th style={{ padding: '10px' }}>Cellular / SIM</th>
              <th style={{ padding: '10px', textAlign: 'right' }}>Actions</th>
            </tr>
          </thead>
          <tbody>
            {cameras.map((cam) => {
              const folderName = folders.find((f) => f.id === cam.folderId)?.name;
              return (
                <tr
                  key={cam.id}
                  style={{
                    borderBottom: '1px solid rgba(255,255,255,0.04)',
                  }}
                >
                  <td style={{ padding: '10px' }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                      <div
                        style={{
                          width: '8px',
                          height: '8px',
                          borderRadius: '50%',
                          background: cam.disabled
                            ? '#ef4444'
                            : cam.state === 'online'
                            ? '#10b981'
                            : '#64748b',
                        }}
                      />
                      <span style={{ fontWeight: 600, color: '#f1f5f9' }}>{cam.id}</span>
                      {cam.disabled && (
                        <span
                          style={{
                            background:
                              cam.disableReason === 'payment'
                                ? 'rgba(239, 68, 68, 0.25)'
                                : cam.disableReason === 'requested'
                                ? 'rgba(168, 85, 247, 0.25)'
                                : 'rgba(245, 158, 11, 0.25)',
                            color:
                              cam.disableReason === 'payment'
                                ? '#fca5a5'
                                : cam.disableReason === 'requested'
                                ? '#d8b4fe'
                                : '#fcd34d',
                            padding: '2px 6px',
                            borderRadius: '4px',
                            fontSize: '0.68rem',
                            fontWeight: 600,
                          }}
                        >
                          {cam.disableReason === 'payment'
                            ? 'За неуплату'
                            : cam.disableReason === 'requested'
                            ? 'По требованию'
                            : 'Тех. причины'}
                        </span>
                      )}
                    </div>
                  </td>

                  {/* Folder */}
                  <td style={{ padding: '10px', color: folderName ? '#a5b4fc' : '#64748b' }}>
                    {folderName ? (
                      <span style={{ display: 'flex', alignItems: 'center', gap: '4px' }}>
                        <Folder size={13} />
                        {folderName}
                      </span>
                    ) : (
                      '—'
                    )}
                  </td>

                  {/* Tags */}
                  <td style={{ padding: '10px' }}>
                    <div style={{ display: 'flex', gap: '4px', flexWrap: 'wrap' }}>
                      {cam.tags && cam.tags.length > 0 ? (
                        cam.tags.map((tId) => {
                          const tag = tags.find((t) => t.id === tId);
                          if (!tag) return null;
                          return (
                            <span
                              key={tId}
                              style={{
                                background: tag.color ? `${tag.color}25` : 'rgba(99,102,241,0.2)',
                                color: tag.color || '#a5b4fc',
                                border: `1px solid ${tag.color || 'rgba(99,102,241,0.4)'}`,
                                padding: '1px 6px',
                                borderRadius: '4px',
                                fontSize: '0.7rem',
                                fontWeight: 500,
                              }}
                            >
                              {tag.name}
                            </span>
                          );
                        })
                      ) : (
                        <span style={{ color: '#64748b' }}>—</span>
                      )}
                    </div>
                  </td>

                  {/* URL */}
                  <td
                    style={{
                      padding: '10px',
                      color: '#94a3b8',
                      fontFamily: 'monospace',
                      maxWidth: '220px',
                      overflow: 'hidden',
                      textOverflow: 'ellipsis',
                      whiteSpace: 'nowrap',
                    }}
                  >
                    {cam.url}
                  </td>

                  {/* Recording */}
                  <td style={{ padding: '10px' }}>
                    {cam.record ? (
                      <span style={{ color: '#10b981', fontWeight: 600 }}>
                        Rec ({cam.retentionDays || 'Default'}d)
                      </span>
                    ) : (
                      <span style={{ color: '#64748b' }}>Off</span>
                    )}
                  </td>

                  {/* Cellular / SIM */}
                  <td style={{ padding: '10px' }}>
                    {cam.simPhone ? (
                      <span
                        style={{
                          display: 'flex',
                          alignItems: 'center',
                          gap: '4px',
                          color: '#38bdf8',
                          fontSize: '0.74rem',
                        }}
                      >
                        <Phone size={12} />
                        {cam.simPhone}
                      </span>
                    ) : (
                      <span style={{ color: '#64748b' }}>—</span>
                    )}
                  </td>

                  {/* Actions */}
                  <td style={{ padding: '10px', textAlign: 'right' }}>
                    <div style={{ display: 'inline-flex', gap: '6px' }}>
                      <button
                        onClick={() => onOpenDetails(cam)}
                        style={{
                          background: 'rgba(56, 189, 248, 0.15)',
                          border: 'none',
                          borderRadius: '6px',
                          padding: '6px',
                          color: '#38bdf8',
                          cursor: 'pointer',
                        }}
                        title="View Stream Details & Direct URLs"
                      >
                        <Info size={14} />
                      </button>

                      {canEdit && (
                        <button
                          onClick={() => onEditCamera(cam)}
                          style={{
                            background: 'rgba(255,255,255,0.06)',
                            border: 'none',
                            borderRadius: '6px',
                            padding: '6px',
                            color: '#a5b4fc',
                            cursor: 'pointer',
                          }}
                          title="Edit Configuration"
                        >
                          <Edit2 size={14} />
                        </button>
                      )}

                      {isAdmin && (
                        <button
                          onClick={() => onDeleteCamera(cam.id)}
                          style={{
                            background: 'rgba(239,68,68,0.15)',
                            border: 'none',
                            borderRadius: '6px',
                            padding: '6px',
                            color: '#ef4444',
                            cursor: 'pointer',
                          }}
                          title="Delete Camera"
                        >
                          <Trash2 size={14} />
                        </button>
                      )}
                    </div>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </div>
  );
};
