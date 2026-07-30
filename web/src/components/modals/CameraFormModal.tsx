import { X, Camera } from 'lucide-react';
import type { TagConfig } from '../../types';

export interface CamFormState {
  id: string;
  url: string;
  record: boolean;
  retentionDays: number;
  tags: string[];
  comment: string;
  simPhone: string;
  simICCID: string;
}

interface CameraFormModalProps {
  globalTags: TagConfig[];
  isEditing: boolean;
  camForm: CamFormState;
  setCamForm: (form: CamFormState) => void;
  onSave: (e: React.FormEvent) => void;
  onClose: () => void;
}

export function CameraFormModal({ isEditing, camForm, setCamForm, onSave, onClose, globalTags }: CameraFormModalProps) {
  const toggleTag = (tagId: string) => {
    const newTags = camForm.tags.includes(tagId) 
      ? camForm.tags.filter(id => id !== tagId)
      : [...camForm.tags, tagId];
    setCamForm({ ...camForm, tags: newTags });
  };
  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal-content glass" onClick={e => e.stopPropagation()}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '24px' }}>
          <h3 style={{ margin: 0, display: 'flex', alignItems: 'center', gap: '8px' }}>
            <Camera size={20} color="var(--primary)" />
            {isEditing ? 'Edit Camera' : 'Add New Camera'}
          </h3>
          <button className="btn-icon" onClick={onClose}>
            <X size={20} />
          </button>
        </div>
        
        <form onSubmit={onSave}>
          <div className="form-group">
            <label>Camera ID (Unique)</label>
            <input 
              type="text" 
              className="input-field"
              value={camForm.id} 
              onChange={e => setCamForm({...camForm, id: e.target.value})} 
              disabled={isEditing}
              placeholder="e.g. cam1"
              required 
            />
          </div>
          <div className="form-group">
            <label>RTSP URL</label>
            <input 
              type="text" 
              className="input-field"
              value={camForm.url} 
              onChange={e => setCamForm({...camForm, url: e.target.value})} 
              placeholder="rtsp://user:pass@ip:port/stream"
              required 
            />
          </div>
          <div className="form-group" style={{ flexDirection: 'row', alignItems: 'center', gap: '12px' }}>
            <input 
              type="checkbox" 
              id="record-check"
              checked={camForm.record}
              onChange={e => setCamForm({...camForm, record: e.target.checked})}
              style={{ width: '18px', height: '18px', cursor: 'pointer' }}
            />
            <label htmlFor="record-check" style={{ marginBottom: 0, cursor: 'pointer' }}>Enable Archive Recording (fMP4)</label>
          </div>
          
          <div className="form-group">
            <label>Tags</label>
            <div style={{ display: 'flex', gap: '8px', flexWrap: 'wrap', marginTop: '4px' }}>
              {globalTags.map(tag => {
                const isSelected = camForm.tags.includes(tag.id);
                return (
                  <div 
                    key={tag.id}
                    onClick={() => toggleTag(tag.id)}
                    style={{
                      display: 'flex', alignItems: 'center', gap: '6px', 
                      padding: '4px 10px', borderRadius: '16px', cursor: 'pointer',
                      border: `1px solid ${isSelected ? tag.color : 'var(--card-border)'}`,
                      background: isSelected ? `${tag.color}33` : 'transparent',
                      color: isSelected ? '#fff' : 'var(--text-muted)'
                    }}
                  >
                    <div style={{ width: '8px', height: '8px', borderRadius: '50%', backgroundColor: tag.color }}></div>
                    {tag.name}
                  </div>
                );
              })}
              {globalTags.length === 0 && (
                <span style={{ fontSize: '0.8rem', color: 'var(--text-muted)' }}>No tags available. Create tags in Tag Manager first.</span>
              )}
            </div>
          </div>
          
          <div className="form-group">
            <label>Comments</label>
            <textarea 
              className="input-field"
              value={camForm.comment} 
              onChange={e => setCamForm({...camForm, comment: e.target.value})} 
              placeholder="Additional notes about this camera"
              style={{ minHeight: '60px' }}
            />
          </div>

          <div style={{ display: 'flex', gap: '12px' }}>
            <div className="form-group" style={{ flex: 1 }}>
              <label>SIM Phone</label>
              <input 
                type="text" 
                className="input-field"
                value={camForm.simPhone} 
                onChange={e => setCamForm({...camForm, simPhone: e.target.value})} 
                placeholder="+1234567890"
              />
            </div>
            <div className="form-group" style={{ flex: 1 }}>
              <label>SIM ICCID</label>
              <input 
                type="text" 
                className="input-field"
                value={camForm.simICCID} 
                onChange={e => setCamForm({...camForm, simICCID: e.target.value})} 
                placeholder="897..."
              />
            </div>
          </div>
          
          {camForm.record && (
            <div className="form-group">
              <label>Retention (Days). 0 = Global Setting</label>
              <input 
                type="number" 
                min="0"
                className="input-field"
                value={camForm.retentionDays} 
                onChange={e => setCamForm({...camForm, retentionDays: parseInt(e.target.value) || 0})} 
              />
            </div>
          )}
          <div style={{ display: 'flex', gap: '12px', marginTop: '24px' }}>
            <button type="submit" className="btn btn-primary" style={{ flex: 1 }}>Save</button>
            <button type="button" className="btn btn-secondary" style={{ flex: 1 }} onClick={onClose}>Cancel</button>
          </div>
        </form>
      </div>
    </div>
  );
}
