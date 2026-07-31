import { X, Camera } from 'lucide-react';
import { useState } from 'react';
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
  disabled: boolean;
  disableReason: string;
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
  
  const [activeTab, setActiveTab] = useState<'general' | 'metadata'>('general');
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
        
        <form onSubmit={onSave} style={{ display: 'flex', flexDirection: 'column' }}>
          
          <div style={{ display: 'flex', gap: '8px', borderBottom: '1px solid rgba(255,255,255,0.1)', marginBottom: '20px' }}>
            <button 
              type="button" 
              onClick={() => setActiveTab('general')}
              style={{ padding: '10px 16px', background: 'none', border: 'none', borderBottom: activeTab === 'general' ? '2px solid var(--primary)' : '2px solid transparent', color: activeTab === 'general' ? '#fff' : 'var(--text-muted)', cursor: 'pointer', fontWeight: activeTab === 'general' ? 600 : 400, transition: 'all 0.2s' }}
            >
              General Settings
            </button>
            <button 
              type="button" 
              onClick={() => setActiveTab('metadata')}
              style={{ padding: '10px 16px', background: 'none', border: 'none', borderBottom: activeTab === 'metadata' ? '2px solid var(--primary)' : '2px solid transparent', color: activeTab === 'metadata' ? '#fff' : 'var(--text-muted)', cursor: 'pointer', fontWeight: activeTab === 'metadata' ? 600 : 400, transition: 'all 0.2s' }}
            >
              Metadata & SIM
            </button>
          </div>

          <div style={{ display: 'flex', flexDirection: 'column', gap: '20px', minHeight: '320px' }}>
            {activeTab === 'general' && (
              <>
                <div style={{ display: 'flex', gap: '16px' }}>
                  <div className="form-group" style={{ flex: 1 }}>
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

                <div style={{ display: 'flex', gap: '16px', flexDirection: 'column', background: 'rgba(0,0,0,0.2)', padding: '16px', borderRadius: '8px' }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
                    <input 
                      type="checkbox" 
                      id="record-check"
                      checked={camForm.record}
                      onChange={e => setCamForm({...camForm, record: e.target.checked})}
                      style={{ width: '18px', height: '18px', cursor: 'pointer' }}
                    />
                    <label htmlFor="record-check" style={{ marginBottom: 0, cursor: 'pointer', fontWeight: 500 }}>Enable Archive Recording</label>
                  </div>
                  
                  {camForm.record && (
                    <div className="form-group" style={{ paddingLeft: '30px', marginBottom: 0 }}>
                      <label>Retention (Days). 0 = Global Setting</label>
                      <input 
                        type="number" 
                        min="0"
                        className="input-field"
                        value={camForm.retentionDays} 
                        onChange={e => setCamForm({...camForm, retentionDays: parseInt(e.target.value) || 0})} 
                        style={{ maxWidth: '200px' }}
                      />
                    </div>
                  )}
                </div>

                <div style={{ display: 'flex', gap: '16px', flexDirection: 'column', background: 'rgba(0,0,0,0.2)', padding: '16px', borderRadius: '8px', border: camForm.disabled ? '1px solid rgba(239, 68, 68, 0.3)' : 'none' }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
                    <input 
                      type="checkbox" 
                      id="disable-check"
                      checked={camForm.disabled}
                      onChange={e => setCamForm({...camForm, disabled: e.target.checked})}
                      style={{ width: '18px', height: '18px', cursor: 'pointer', accentColor: 'var(--danger)' }}
                    />
                    <label htmlFor="disable-check" style={{ marginBottom: 0, cursor: 'pointer', fontWeight: 500, color: camForm.disabled ? 'var(--danger)' : 'var(--text-main)' }}>
                      Disable Stream Processing
                    </label>
                  </div>

                  {camForm.disabled && (
                    <div className="form-group" style={{ paddingLeft: '30px', marginBottom: 0 }}>
                      <label>Reason for Disabling</label>
                      <div style={{ display: 'flex', gap: '8px', flexWrap: 'wrap', marginTop: '4px' }}>
                        {[
                          { value: 'technical', label: 'Technical Issue' },
                          { value: 'requested', label: 'Requested by User' },
                          { value: 'payment', label: 'Unpaid Bill' }
                        ].map(opt => {
                          const isSelected = camForm.disableReason === opt.value;
                          return (
                            <div 
                              key={opt.value}
                              onClick={() => setCamForm({...camForm, disableReason: opt.value})}
                              style={{
                                padding: '6px 14px', borderRadius: '8px', cursor: 'pointer', fontSize: '0.85rem',
                                border: isSelected ? '1px solid rgba(239, 68, 68, 0.5)' : '1px solid rgba(255,255,255,0.1)',
                                background: isSelected ? 'rgba(239, 68, 68, 0.15)' : 'rgba(0,0,0,0.3)',
                                color: isSelected ? '#fff' : 'var(--text-muted)',
                                fontWeight: isSelected ? 500 : 400,
                                transition: 'all 0.2s',
                                boxShadow: isSelected ? '0 0 8px rgba(239, 68, 68, 0.2)' : 'none'
                              }}
                            >
                              {opt.label}
                            </div>
                          );
                        })}
                      </div>
                    </div>
                  )}
                </div>
              </>
            )}

            {activeTab === 'metadata' && (
              <>
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
                            padding: '6px 12px', borderRadius: '20px', cursor: 'pointer',
                            border: `1px solid ${isSelected ? tag.color : 'rgba(255,255,255,0.1)'}`,
                            background: isSelected ? `${tag.color}22` : 'rgba(0,0,0,0.3)',
                            color: isSelected ? '#fff' : 'var(--text-muted)',
                            fontWeight: isSelected ? 500 : 400,
                            transition: 'all 0.2s',
                            boxShadow: isSelected ? `0 0 8px ${tag.color}33` : 'none'
                          }}
                        >
                          <div style={{ width: '8px', height: '8px', borderRadius: '50%', backgroundColor: tag.color }}></div>
                          {tag.name}
                        </div>
                      );
                    })}
                    {globalTags.length === 0 && (
                      <span style={{ fontSize: '0.8rem', color: 'var(--text-muted)', padding: '8px' }}>No tags available. Create tags in Tag Manager first.</span>
                    )}
                  </div>
                </div>
                
                <div className="form-group">
                  <label>Comments / Notes</label>
                  <textarea 
                    className="input-field"
                    value={camForm.comment} 
                    onChange={e => setCamForm({...camForm, comment: e.target.value})} 
                    placeholder="Additional notes about this camera"
                    style={{ minHeight: '60px', resize: 'vertical' }}
                  />
                </div>

                <div style={{ display: 'flex', gap: '16px', marginBottom: 0 }}>
                  <div className="form-group" style={{ flex: 1, marginBottom: 0 }}>
                    <label>SIM Phone Number</label>
                    <input 
                      type="text" 
                      className="input-field"
                      value={camForm.simPhone} 
                      onChange={e => setCamForm({...camForm, simPhone: e.target.value})} 
                      placeholder="+1234567890"
                    />
                  </div>
                  <div className="form-group" style={{ flex: 1, marginBottom: 0 }}>
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
              </>
            )}
          </div>
          
          <div style={{ display: 'flex', gap: '16px', marginTop: '20px', paddingTop: '20px', borderTop: '1px solid rgba(255,255,255,0.05)' }}>
            <button type="submit" className="btn btn-primary" style={{ flex: 2, padding: '12px' }}>{isEditing ? 'Save Changes' : 'Create Camera'}</button>
            <button type="button" className="btn btn-secondary" style={{ flex: 1, padding: '12px' }} onClick={onClose}>Cancel</button>
          </div>
        </form>
      </div>
    </div>
  );
}
