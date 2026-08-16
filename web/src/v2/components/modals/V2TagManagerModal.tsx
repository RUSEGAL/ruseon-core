import React, { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { X, Tag, Plus, Trash2, Edit2, Check } from 'lucide-react';
import type { TagConfig } from '../../../types';

interface V2TagManagerModalProps {
  tags: TagConfig[];
  token: string | null;
  onClose: () => void;
  onTagsChange: () => void;
}

const PRESET_COLORS = [
  '#6366f1',
  '#10b981',
  '#38bdf8',
  '#f59e0b',
  '#ef4444',
  '#ec4899',
  '#8b5cf6',
  '#14b8a6',
];

export const V2TagManagerModal: React.FC<V2TagManagerModalProps> = ({
  tags,
  token,
  onClose,
  onTagsChange,
}) => {
  const { t } = useTranslation();
  const [newTagName, setNewTagName] = useState('');
  const [newTagColor, setNewTagColor] = useState('#6366f1');
  const [editingId, setEditingId] = useState<string | null>(null);
  const [editName, setEditName] = useState('');
  const [editColor, setEditColor] = useState('');

  const generateId = () => Math.random().toString(36).substring(2, 9);

  const handleAdd = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newTagName.trim()) return;

    try {
      const res = await fetch('/api/tags', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: token ? `Bearer ${token}` : '',
        },
        body: JSON.stringify({
          id: generateId(),
          name: newTagName.trim(),
          color: newTagColor,
        }),
      });
      if (res.ok) {
        setNewTagName('');
        onTagsChange();
      }
    } catch (err) {
      console.error('Failed to add tag:', err);
    }
  };

  const handleDelete = async (id: string) => {
    if (!confirm('Delete this tag? It will be unassigned from all cameras.')) return;
    try {
      await fetch(`/api/tags/${id}`, {
        method: 'DELETE',
        headers: { Authorization: token ? `Bearer ${token}` : '' },
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
          Authorization: token ? `Bearer ${token}` : '',
        },
        body: JSON.stringify({
          id: editingId,
          name: editName.trim(),
          color: editColor,
        }),
      });
      setEditingId(null);
      onTagsChange();
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
                background: 'rgba(16, 185, 129, 0.15)',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
              }}
            >
              <Tag size={18} color="#10b981" />
            </div>
            <h3 style={{ margin: 0, fontSize: '1.1rem', fontWeight: 700, color: '#f8fafc' }}>
              {t('v2.modals.tags.title', 'Tag Management')} ({tags.length})
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
              {t('tags.add', 'Create Tag')}
            </div>
            <div style={{ display: 'flex', gap: '8px' }}>
              <input
                type="text"
                className="v2-input"
                placeholder={t('v2.modals.tags.placeholder', 'Tag name...')}
                value={newTagName}
                onChange={(e) => setNewTagName(e.target.value)}
                style={{ flex: 1 }}
                required
              />
              <button type="submit" className="v2-btn-primary">
                <Plus size={15} />
                <span>{t('tags.add', 'Add')}</span>
              </button>
            </div>

            {/* Color Palette Chips */}
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
              <span style={{ fontSize: '0.72rem', color: '#64748b' }}>{t('tags.color', 'Color')}:</span>
              <div style={{ display: 'flex', gap: '6px' }}>
                {PRESET_COLORS.map((col) => (
                  <div
                    key={col}
                    onClick={() => setNewTagColor(col)}
                    style={{
                      width: '20px',
                      height: '20px',
                      borderRadius: '50%',
                      background: col,
                      cursor: 'pointer',
                      boxShadow: newTagColor === col ? `0 0 0 2px #fff, 0 0 8px ${col}` : 'none',
                      transform: newTagColor === col ? 'scale(1.15)' : 'scale(1)',
                      transition: 'all 0.15s ease',
                    }}
                  />
                ))}
              </div>
            </div>
          </form>

          {/* Tag List */}
          <div style={{ display: 'flex', flexDirection: 'column', gap: '6px', maxHeight: '280px', overflowY: 'auto' }}>
            {tags.length === 0 ? (
              <div style={{ textAlign: 'center', color: '#64748b', fontSize: '0.8rem', padding: '1rem' }}>
                {t('tags.noTags', 'No tags created yet.')}
              </div>
            ) : (
              tags.map((tag) => {
                const isEditing = editingId === tag.id;
                return (
                  <div
                    key={tag.id}
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
                        <div style={{ display: 'flex', gap: '4px' }}>
                          {PRESET_COLORS.map((col) => (
                            <div
                              key={col}
                              onClick={() => setEditColor(col)}
                              style={{
                                width: '16px',
                                height: '16px',
                                borderRadius: '50%',
                                background: col,
                                cursor: 'pointer',
                                border: editColor === col ? '2px solid #fff' : 'none',
                              }}
                            />
                          ))}
                        </div>
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
                          <div
                            style={{
                              width: '10px',
                              height: '10px',
                              borderRadius: '50%',
                              background: tag.color || '#6366f1',
                            }}
                          />
                          <span
                            style={{
                              background: tag.color ? `${tag.color}25` : 'rgba(99,102,241,0.2)',
                              color: tag.color || '#a5b4fc',
                              border: `1px solid ${tag.color || 'rgba(99,102,241,0.4)'}`,
                              padding: '2px 8px',
                              borderRadius: '6px',
                              fontSize: '0.78rem',
                              fontWeight: 600,
                            }}
                          >
                            {tag.name}
                          </span>
                        </div>

                        <div style={{ display: 'flex', gap: '4px' }}>
                          <button
                            onClick={() => startEdit(tag)}
                            style={{
                              background: 'rgba(255, 255, 255, 0.05)',
                              border: 'none',
                              borderRadius: '6px',
                              padding: '4px 6px',
                              color: '#a5b4fc',
                              cursor: 'pointer',
                            }}
                            title={t('cameras.edit', 'Edit')}
                          >
                            <Edit2 size={13} />
                          </button>
                          <button
                            onClick={() => handleDelete(tag.id)}
                            style={{
                              background: 'rgba(239, 68, 68, 0.15)',
                              border: 'none',
                              borderRadius: '6px',
                              padding: '4px 6px',
                              color: '#ef4444',
                              cursor: 'pointer',
                            }}
                            title={t('cameras.delete', 'Delete')}
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
            {t('cameras.cancel', 'Done')}
          </button>
        </div>
      </div>
    </div>
  );
};
