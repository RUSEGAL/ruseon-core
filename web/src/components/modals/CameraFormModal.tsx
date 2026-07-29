import { X, Camera } from 'lucide-react';

export interface CamFormState {
  id: string;
  url: string;
  record: boolean;
  retentionDays: number;
}

interface CameraFormModalProps {
  isEditing: boolean;
  camForm: CamFormState;
  setCamForm: (form: CamFormState) => void;
  onSave: (e: React.FormEvent) => void;
  onClose: () => void;
}

export function CameraFormModal({ isEditing, camForm, setCamForm, onSave, onClose }: CameraFormModalProps) {
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
