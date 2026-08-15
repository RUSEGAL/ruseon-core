import React, { useState } from 'react';
import { X, Folder, Plus, Trash2, Edit2, Check } from 'lucide-react';
import type { FolderConfig } from '../../../types';

interface V2FolderManagerModalProps {
  folders: FolderConfig[];
  token: string | null;
  onClose: () => void;
  onFoldersChange: () => void;
}

export const V2FolderManagerModal: React.FC<V2FolderManagerModalProps> = ({
  folders,
  token,
  onClose,
  onFoldersChange,
}) => {
  const [newFolderName, setNewFolderName] = useState('');
  const [editingId, setEditingId] = useState<string | null>(null);
  const [editName, setEditName] = useState('');

  const generateId = () => Math.random().toString(36).substring(2, 9);

  const handleAdd = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newFolderName.trim()) return;

    try {
      const res = await fetch('/api/folders', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: token ? `Bearer ${token}` : '',
        },
        body: JSON.stringify({
          id: generateId(),
          name: newFolderName.trim(),
        }),
      });
      if (res.ok) {
        setNewFolderName('');
        onFoldersChange();
      }
    } catch (err) {
      console.error('Failed to add folder:', err);
    }
  };

  const handleDelete = async (id: string) => {
    if (!confirm('Delete this folder? It will be removed from all cameras.')) return;
    try {
      await fetch(`/api/folders/${id}`, {
        method: 'DELETE',
        headers: { Authorization: token ? `Bearer ${token}` : '' },
      });
      onFoldersChange();
    } catch (err) {
      console.error(err);
    }
  };

  const startEdit = (folder: FolderConfig) => {
    setEditingId(folder.id);
    setEditName(folder.name);
  };

  const saveEdit = async () => {
    if (!editName.trim() || !editingId) return;
    try {
      await fetch(`/api/folders/${editingId}`, {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
          Authorization: token ? `Bearer ${token}` : '',
        },
        body: JSON.stringify({
          id: editingId,
          name: editName.trim(),
        }),
      });
      setEditingId(null);
      onFoldersChange();
    } catch (err) {
      console.error(err);
    }
  };

  return (
    <div className="v2-modal-overlay" onClick={onClose}>
      <div
        className="v2-modal-container"
        onClick={(e) => e.stopPropagation()}
        style={{ width: '520px', maxWidth: '95vw' }}
      >
        {/* Header */}
        <div className="v2-modal-header">
          <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
            <div
              style={{
                width: '34px',
                height: '34px',
                borderRadius: '8px',
                background: 'rgba(99, 102, 241, 0.15)',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
              }}
            >
              <Folder size={18} color="#818cf8" />
            </div>
            <h3 style={{ margin: 0, fontSize: '1.1rem', fontWeight: 700, color: '#f8fafc' }}>
              Folder & Location Manager ({folders.length})
            </h3>
          </div>

          <button
            onClick={onClose}
            style={{
              background: 'rgba(255,255,255,0.06)',
              border: 'none',
              borderRadius: '8px',
              padding: '6px',
              color: '#94a3b8',
              cursor: 'pointer',
            }}
          >
            <X size={18} />
          </button>
        </div>

        {/* Body */}
        <div className="v2-modal-body" style={{ gap: '1rem' }}>
          {/* Create Form */}
          <form
            onSubmit={handleAdd}
            style={{
              background: 'rgba(0, 0, 0, 0.3)',
              padding: '12px',
              borderRadius: '10px',
              border: '1px solid rgba(255, 255, 255, 0.08)',
              display: 'flex',
              flexDirection: 'column',
              gap: '10px',
            }}
          >
            <div style={{ fontSize: '0.78rem', fontWeight: 600, color: '#94a3b8' }}>
              CREATE NEW FOLDER
            </div>
            <div style={{ display: 'flex', gap: '8px' }}>
              <input
                type="text"
                className="v2-input"
                placeholder="Folder name (e.g. Main Building, Warehouse 3, Parking)"
                value={newFolderName}
                onChange={(e) => setNewFolderName(e.target.value)}
                style={{ flex: 1 }}
                required
              />
              <button type="submit" className="v2-btn-primary">
                <Plus size={15} />
                <span>Create</span>
              </button>
            </div>
          </form>

          {/* Folder List */}
          <div style={{ display: 'flex', flexDirection: 'column', gap: '6px', maxHeight: '280px', overflowY: 'auto' }}>
            {folders.length === 0 ? (
              <div style={{ textAlign: 'center', color: '#64748b', fontSize: '0.8rem', padding: '1rem' }}>
                No folders created yet.
              </div>
            ) : (
              folders.map((folder) => {
                const isEditing = editingId === folder.id;
                return (
                  <div
                    key={folder.id}
                    style={{
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'space-between',
                      padding: '8px 12px',
                      borderRadius: '8px',
                      background: 'rgba(255, 255, 255, 0.03)',
                      border: '1px solid rgba(255, 255, 255, 0.05)',
                    }}
                  >
                    {isEditing ? (
                      <div style={{ display: 'flex', alignItems: 'center', gap: '8px', flex: 1 }}>
                        <input
                          type="text"
                          className="v2-input"
                          value={editName}
                          onChange={(e) => setEditName(e.target.value)}
                          style={{ flex: 1, padding: '4px 8px', fontSize: '0.8rem' }}
                        />
                        <button
                          onClick={saveEdit}
                          className="v2-btn-primary"
                          style={{ padding: '4px 8px', fontSize: '0.75rem' }}
                        >
                          <Check size={13} />
                        </button>
                      </div>
                    ) : (
                      <>
                        <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                          <Folder size={15} color="#818cf8" />
                          <span style={{ fontWeight: 600, color: '#f1f5f9', fontSize: '0.82rem' }}>
                            {folder.name}
                          </span>
                        </div>

                        <div style={{ display: 'flex', gap: '4px' }}>
                          <button
                            onClick={() => startEdit(folder)}
                            style={{
                              background: 'rgba(255, 255, 255, 0.05)',
                              border: 'none',
                              borderRadius: '6px',
                              padding: '4px 6px',
                              color: '#a5b4fc',
                              cursor: 'pointer',
                            }}
                            title="Rename"
                          >
                            <Edit2 size={13} />
                          </button>
                          <button
                            onClick={() => handleDelete(folder.id)}
                            style={{
                              background: 'rgba(239, 68, 68, 0.15)',
                              border: 'none',
                              borderRadius: '6px',
                              padding: '4px 6px',
                              color: '#ef4444',
                              cursor: 'pointer',
                            }}
                            title="Delete"
                          >
                            <Trash2 size={13} />
                          </button>
                        </div>
                      </>
                    )}
                  </div>
                );
              })
            )}
          </div>
        </div>

        {/* Footer */}
        <div className="v2-modal-footer">
          <button className="v2-btn-secondary" onClick={onClose}>
            Done
          </button>
        </div>
      </div>
    </div>
  );
};
