import { useState } from 'react';
import { X, Folder as FolderIcon, Plus, Trash2, Edit2 } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import type { FolderConfig } from '../../types';

interface FolderManagerModalProps {
  folders: FolderConfig[];
  token: string | null;
  onClose: () => void;
  onFoldersChange: () => void;
}

export function FolderManagerModal({ folders, token, onClose, onFoldersChange }: FolderManagerModalProps) {
  const { t } = useTranslation();
  const [newFolderName, setNewFolderName] = useState('');
  
  const [editingId, setEditingId] = useState<string | null>(null);
  const [editName, setEditName] = useState('');

  const generateId = () => Math.random().toString(36).substr(2, 9);

  const handleAdd = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newFolderName.trim()) return;
    
    try {
      const res = await fetch('/api/folders', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${token}`
        },
        body: JSON.stringify({
          id: generateId(),
          name: newFolderName.trim()
        })
      });
      if (res.ok) {
        setNewFolderName('');
        onFoldersChange();
      }
    } catch (err) {
      console.error(err);
    }
  };

  const handleDelete = async (id: string) => {
    if (!confirm(t('folders.deleteConfirm', 'Are you sure you want to delete this folder? It will be removed from all cameras.'))) return;
    try {
      await fetch(`/api/folders/${id}`, {
        method: 'DELETE',
        headers: { 'Authorization': `Bearer ${token}` }
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
          'Authorization': `Bearer ${token}`
        },
        body: JSON.stringify({
          id: editingId,
          name: editName.trim()
        })
      });
      setEditingId(null);
      onFoldersChange();
    } catch (err) {
      console.error(err);
    }
  };

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal-content glass" onClick={e => e.stopPropagation()} style={{ maxWidth: '500px' }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '24px' }}>
          <h3 style={{ margin: 0, display: 'flex', alignItems: 'center', gap: '8px' }}>
            <FolderIcon size={20} color="var(--primary)" />
            {t('folders.title', 'Folders')}
          </h3>
          <button className="btn-icon" onClick={onClose}>
            <X size={20} />
          </button>
        </div>

        <form onSubmit={handleAdd} style={{ display: 'flex', gap: '12px', marginBottom: '24px', alignItems: 'flex-end' }}>
          <div style={{ flex: 1 }}>
            <label style={{ fontSize: '0.8rem', color: 'var(--text-muted)', marginBottom: '4px', display: 'block' }}>{t('folders.name', 'Folder Name')}</label>
            <input 
              type="text" 
              className="input-field" 
              value={newFolderName}
              onChange={e => setNewFolderName(e.target.value)}
              placeholder="e.g. Office"
              required
            />
          </div>
          <button type="submit" className="btn btn-primary" style={{ padding: '0 16px', height: '40px' }}>
            <Plus size={18} /> {t('folders.add', 'Add')}
          </button>
        </form>

        <div style={{ display: 'flex', flexDirection: 'column', gap: '8px', maxHeight: '300px', overflowY: 'auto' }}>
          {folders.length === 0 ? (
            <div style={{ textAlign: 'center', color: 'var(--text-muted)', padding: '16px' }}>{t('folders.noFolders', 'No folders yet')}</div>
          ) : folders.map(folder => (
            <div key={folder.id} style={{ display: 'flex', alignItems: 'center', gap: '12px', padding: '8px 12px', background: 'rgba(255,255,255,0.05)', borderRadius: '6px' }}>
              
              {editingId === folder.id ? (
                <>
                  <input 
                    type="text" 
                    className="input-field" 
                    style={{ flex: 1, padding: '4px 8px' }}
                    value={editName}
                    onChange={e => setEditName(e.target.value)}
                  />
                  <button className="btn btn-primary" style={{ padding: '4px 8px', fontSize: '0.8rem' }} onClick={saveEdit}>{t('folders.save', 'Save')}</button>
                  <button className="btn btn-secondary" style={{ padding: '4px 8px', fontSize: '0.8rem' }} onClick={() => setEditingId(null)}>{t('folders.cancel', 'Cancel')}</button>
                </>
              ) : (
                <>
                  <FolderIcon size={16} color="var(--primary)" />
                  <span style={{ flex: 1, fontWeight: 500 }}>{folder.name}</span>
                  
                  <div style={{ display: 'flex', gap: '4px' }}>
                    <button className="btn-icon" onClick={() => startEdit(folder)}>
                      <Edit2 size={16} />
                    </button>
                    <button className="btn-icon btn-danger" onClick={() => handleDelete(folder.id)}>
                      <Trash2 size={16} />
                    </button>
                  </div>
                </>
              )}
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
