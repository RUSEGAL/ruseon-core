import { useState } from 'react';
import { X, Tag, Plus, Trash2, Edit2 } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import type { TagConfig } from '../../types';

interface TagManagerModalProps {
  tags: TagConfig[];
  token: string | null;
  onClose: () => void;
  onTagsChange: () => void;
}

export function TagManagerModal({ tags, token, onClose, onTagsChange }: TagManagerModalProps) {
  const { t } = useTranslation();
  const [newTagName, setNewTagName] = useState('');
  const [newTagColor, setNewTagColor] = useState('#3b82f6'); // default blue
  
  const [editingId, setEditingId] = useState<string | null>(null);
  const [editName, setEditName] = useState('');
  const [editColor, setEditColor] = useState('');

  const generateId = () => Math.random().toString(36).substr(2, 9);

  const handleAdd = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newTagName.trim()) return;
    
    try {
      const res = await fetch('/api/tags', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${token}`
        },
        body: JSON.stringify({
          id: generateId(),
          name: newTagName.trim(),
          color: newTagColor
        })
      });
      if (res.ok) {
        setNewTagName('');
        onTagsChange();
      }
    } catch (err) {
      console.error(err);
    }
  };

  const handleDelete = async (id: string) => {
    if (!confirm('Are you sure you want to delete this tag? It will be removed from all cameras.')) return;
    try {
      await fetch(`/api/tags/${id}`, {
        method: 'DELETE',
        headers: { 'Authorization': `Bearer ${token}` }
      });
      onTagsChange();
    } catch (err) {
      console.error(err);
    }
  };

  const startEdit = (tag: TagConfig) => {
    setEditingId(tag.id);
    setEditName(tag.name);
    setEditColor(tag.color);
  };

  const saveEdit = async () => {
    if (!editName.trim() || !editingId) return;
    try {
      await fetch(`/api/tags/${editingId}`, {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${token}`
        },
        body: JSON.stringify({
          id: editingId,
          name: editName.trim(),
          color: editColor
        })
      });
      setEditingId(null);
      onTagsChange();
    } catch (err) {
      console.error(err);
    }
  };

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal-content glass" onClick={e => e.stopPropagation()} style={{ maxWidth: '500px' }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '24px' }}>
          <h3 style={{ margin: 0, display: 'flex', alignItems: 'center', gap: '8px' }}>
            <Tag size={20} color="var(--primary)" />
            {t('tags.title')}
          </h3>
          <button className="btn-icon" onClick={onClose}>
            <X size={20} />
          </button>
        </div>

        <form onSubmit={handleAdd} style={{ display: 'flex', gap: '12px', marginBottom: '24px', alignItems: 'flex-end' }}>
          <div style={{ flex: 1 }}>
            <label style={{ fontSize: '0.8rem', color: 'var(--text-muted)', marginBottom: '4px', display: 'block' }}>{t('tags.name')}</label>
            <input 
              type="text" 
              className="input-field" 
              value={newTagName}
              onChange={e => setNewTagName(e.target.value)}
              placeholder="e.g. Office"
              required
            />
          </div>
          <div>
            <label style={{ fontSize: '0.8rem', color: 'var(--text-muted)', marginBottom: '4px', display: 'block' }}>{t('tags.color')}</label>
            <input 
              type="color" 
              value={newTagColor}
              onChange={e => setNewTagColor(e.target.value)}
              style={{ width: '40px', height: '40px', padding: 0, border: 'none', borderRadius: '4px', cursor: 'pointer', background: 'transparent' }}
            />
          </div>
          <button type="submit" className="btn btn-primary" style={{ padding: '0 16px', height: '40px' }}>
            <Plus size={18} /> {t('tags.add')}
          </button>
        </form>

        <div style={{ display: 'flex', flexDirection: 'column', gap: '8px', maxHeight: '300px', overflowY: 'auto' }}>
          {tags.length === 0 ? (
            <div style={{ textAlign: 'center', color: 'var(--text-muted)', padding: '16px' }}>{t('tags.noTags')}</div>
          ) : tags.map(tag => (
            <div key={tag.id} style={{ display: 'flex', alignItems: 'center', gap: '12px', padding: '8px 12px', background: 'rgba(255,255,255,0.05)', borderRadius: '6px' }}>
              
              {editingId === tag.id ? (
                <>
                  <input 
                    type="color" 
                    value={editColor}
                    onChange={e => setEditColor(e.target.value)}
                    style={{ width: '24px', height: '24px', padding: 0, border: 'none' }}
                  />
                  <input 
                    type="text" 
                    className="input-field" 
                    style={{ flex: 1, padding: '4px 8px' }}
                    value={editName}
                    onChange={e => setEditName(e.target.value)}
                  />
                  <button className="btn btn-primary" style={{ padding: '4px 8px', fontSize: '0.8rem' }} onClick={saveEdit}>{t('tags.save')}</button>
                  <button className="btn btn-secondary" style={{ padding: '4px 8px', fontSize: '0.8rem' }} onClick={() => setEditingId(null)}>{t('tags.cancel')}</button>
                </>
              ) : (
                <>
                  <div style={{ width: '12px', height: '12px', borderRadius: '50%', backgroundColor: tag.color }}></div>
                  <span style={{ flex: 1, fontWeight: 500 }}>{tag.name}</span>
                  
                  <div style={{ display: 'flex', gap: '4px' }}>
                    <button className="btn-icon" onClick={() => startEdit(tag)}>
                      <Edit2 size={16} />
                    </button>
                    <button className="btn-icon btn-danger" onClick={() => handleDelete(tag.id)}>
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
