import React, { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { X, Camera, Settings, Smartphone, AlertTriangle } from 'lucide-react';
import type { TagConfig, FolderConfig } from '../../../types';

export interface CamFormState {
  id: string;
  url: string;
  record: boolean;
  lazyHLS: boolean;
  tokenAuth: boolean;
  transport: string;
  retentionDays: number;
  tags: string[];
  folderId: string;
  comment: string;
  simPhone: string;
  simICCID: string;
  disabled: boolean;
  disableReason: string;
}

interface V2CameraFormModalProps {
  isEditing: boolean;
  camForm: CamFormState;
  setCamForm: (form: CamFormState) => void;
  globalTags: TagConfig[];
  folders: FolderConfig[];
  onSave: (e: React.FormEvent) => void;
  onClose: () => void;
}

export const V2CameraFormModal: React.FC<V2CameraFormModalProps> = ({
  isEditing,
  camForm,
  setCamForm,
  globalTags,
  folders,
  onSave,
  onClose,
}) => {
  const { t } = useTranslation();
  const [activeTab, setActiveTab] = useState<'general' | 'cellular'>('general');

  const toggleTag = (tagId: string) => {
    const newTags = camForm.tags.includes(tagId)
      ? camForm.tags.filter((id: string) => id !== tagId)
      : [...camForm.tags, tagId];
    setCamForm({ ...camForm, tags: newTags });
  };

  const DISABLE_REASONS = [
    { value: 'technical', label: t('cameras.details.reasons.technical', 'Technical') },
    { value: 'payment', label: t('cameras.details.reasons.payment', 'Payment') },
    { value: 'requested', label: t('cameras.details.reasons.requested', 'Requested') },
  ];

  return (
    <div className="v2-modal-overlay" onClick={onClose}>
      <div
        className="v2-modal-container"
        onClick={(e) => e.stopPropagation()}
        style={{ width: '680px', maxWidth: '95vw' }}
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
              <Camera size={18} color="#818cf8" />
            </div>
            <h3 style={{ margin: 0, fontSize: '1.1rem', fontWeight: 700, color: '#f8fafc' }}>
              {isEditing ? `${t('v2.modals.cameraForm.editTitle', 'Edit Camera')}: ${camForm.id}` : t('v2.modals.cameraForm.addTitle', 'Add New Camera')}
            </h3>
          </div>

          <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
            <div className="v2-modal-tabs">
              <button
                type="button"
                className={`v2-modal-tab ${activeTab === 'general' ? 'active' : ''}`}
                onClick={() => setActiveTab('general')}
              >
                <Settings size={14} style={{ flexShrink: 0 }} />
                <span>{t('v2.modals.cameraForm.generalTab', 'General')}</span>
              </button>
              <button
                type="button"
                className={`v2-modal-tab ${activeTab === 'cellular' ? 'active' : ''}`}
                onClick={() => setActiveTab('cellular')}
              >
                <Smartphone size={14} style={{ flexShrink: 0 }} />
                <span>{t('v2.modals.cameraForm.cellularTab', 'Cellular, State & Tags')}</span>
              </button>
            </div>

            <button
              type="button"
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
        </div>

        {/* Form Form Body */}
        <form onSubmit={onSave} style={{ display: 'flex', flexDirection: 'column', flex: 1, minHeight: 0 }}>
          <div className="v2-modal-body" style={{ minHeight: '360px' }}>
            {activeTab === 'general' && (
              <>
                <div style={{ display: 'grid', gridTemplateColumns: '1.2fr 1fr', gap: '12px' }}>
                  <div className="v2-form-group">
                    <label className="v2-form-label">{t('cameras.id', 'Camera Identifier')} *</label>
                    <input
                      type="text"
                      className="v2-input"
                      value={camForm.id}
                      onChange={(e) => setCamForm({ ...camForm, id: e.target.value })}
                      placeholder="e.g. entrance-cam-01"
                      disabled={isEditing}
                      required
                    />
                  </div>

                  <div className="v2-form-group">
                    <label className="v2-form-label">{t('cameras.transport', 'RTSP Transport')}</label>
                    <select
                      className="v2-input"
                      value={camForm.transport || 'tcp'}
                      onChange={(e) => setCamForm({ ...camForm, transport: e.target.value as 'tcp' | 'udp' })}
                    >
                      <option value="tcp">TCP (Reliable, Interleaved)</option>
                      <option value="udp">UDP (Low-latency / Cellular)</option>
                    </select>
                  </div>
                </div>

                <div className="v2-form-group">
                  <label className="v2-form-label">{t('cameras.url', 'Source RTSP Stream URL')} *</label>
                  <input
                    type="text"
                    className="v2-input"
                    value={camForm.url}
                    onChange={(e) => setCamForm({ ...camForm, url: e.target.value })}
                    placeholder="rtsp://admin:pass@192.168.1.100:554/stream1"
                    required
                  />
                </div>

                <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '12px' }}>
                  <div className="v2-form-group">
                    <label className="v2-form-label">{t('cameras.retention', 'Archive Retention (Days)')}</label>
                    <input
                      type="number"
                      className="v2-input"
                      min={0}
                      value={camForm.retentionDays || 7}
                      onChange={(e) => setCamForm({ ...camForm, retentionDays: parseInt(e.target.value) || 0 })}
                    />
                  </div>

                  <div className="v2-form-group">
                    <label className="v2-form-label">{t('v2.settings.folders', 'Assigned Folder')}</label>
                    <select
                      className="v2-input"
                      value={camForm.folderId || ''}
                      onChange={(e) => setCamForm({ ...camForm, folderId: e.target.value })}
                    >
                      <option value="">{t('folders.noFolder', 'No Folder (Unassigned)')}</option>
                      {folders.map((f) => (
                        <option key={f.id} value={f.id}>
                          {f.name}
                        </option>
                      ))}
                    </select>
                  </div>
                </div>

                {/* Toggles */}
                <div
                  style={{
                    background: 'rgba(255, 255, 255, 0.02)',
                    border: '1px solid rgba(255, 255, 255, 0.06)',
                    borderRadius: '10px',
                    padding: '12px',
                    display: 'flex',
                    flexDirection: 'column',
                    gap: '10px',
                  }}
                >
                  <label style={{ display: 'flex', alignItems: 'center', gap: '10px', cursor: 'pointer' }}>
                    <input
                      type="checkbox"
                      checked={camForm.record}
                      onChange={(e) => setCamForm({ ...camForm, record: e.target.checked })}
                      style={{ accentColor: '#6366f1', width: '16px', height: '16px' }}
                    />
                    <div>
                      <div style={{ fontSize: '0.82rem', fontWeight: 600, color: '#f8fafc' }}>
                        {t('cameras.record', 'Continuous fMP4 Recording')}
                      </div>
                      <div style={{ fontSize: '0.72rem', color: '#94a3b8' }}>
                        {t('v2.modals.cameraForm.recordDesc', 'Record incoming H.264/H.265 chunks into indexed timeline archive')}
                      </div>
                    </div>
                  </label>

                  <label style={{ display: 'flex', alignItems: 'center', gap: '10px', cursor: 'pointer' }}>
                    <input
                      type="checkbox"
                      checked={camForm.lazyHLS}
                      onChange={(e) => setCamForm({ ...camForm, lazyHLS: e.target.checked })}
                      style={{ accentColor: '#6366f1', width: '16px', height: '16px' }}
                    />
                    <div>
                      <div style={{ fontSize: '0.82rem', fontWeight: 600, color: '#f8fafc' }}>
                        {t('cameras.lazy', 'On-Demand Lazy HLS')}
                      </div>
                      <div style={{ fontSize: '0.72rem', color: '#94a3b8' }}>
                        {t('v2.modals.cameraForm.lazyDesc', 'Only transcode/package HLS when viewers are actively connected')}
                      </div>
                    </div>
                  </label>

                  <label style={{ display: 'flex', alignItems: 'center', gap: '10px', cursor: 'pointer' }}>
                    <input
                      type="checkbox"
                      checked={camForm.tokenAuth}
                      onChange={(e) => setCamForm({ ...camForm, tokenAuth: e.target.checked })}
                      style={{ accentColor: '#6366f1', width: '16px', height: '16px' }}
                    />
                    <div>
                      <div style={{ fontSize: '0.82rem', fontWeight: 600, color: '#f8fafc' }}>
                        {t('v2.modals.cameraForm.tokenAuth', 'Require Token Authentication for Playback')}
                      </div>
                      <div style={{ fontSize: '0.72rem', color: '#94a3b8' }}>
                        {t('v2.modals.cameraForm.tokenAuthDesc', 'Enforce JWT token validation for HLS and WebRTC endpoints')}
                      </div>
                    </div>
                  </label>
                </div>
              </>
            )}

            {activeTab === 'cellular' && (
              <>
                {/* State Control / Disabled Management */}
                <div
                  style={{
                    background: camForm.disabled ? 'rgba(239, 68, 68, 0.1)' : 'rgba(16, 185, 129, 0.08)',
                    padding: '14px',
                    borderRadius: '10px',
                    border: '1px solid',
                    borderColor: camForm.disabled ? 'rgba(239, 68, 68, 0.3)' : 'rgba(16, 185, 129, 0.2)',
                    display: 'flex',
                    flexDirection: 'column',
                    gap: '10px',
                  }}
                >
                  <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                      <AlertTriangle size={18} color={camForm.disabled ? '#ef4444' : '#10b981'} />
                      <span style={{ fontSize: '0.86rem', fontWeight: 700, color: '#f8fafc' }}>
                        {t('v2.modals.cameraForm.stateControl', 'Camera Activity State')}
                      </span>
                    </div>

                    <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer' }}>
                      <input
                        type="checkbox"
                        checked={camForm.disabled}
                        onChange={(e) => setCamForm({ ...camForm, disabled: e.target.checked })}
                        style={{ accentColor: '#ef4444', width: '16px', height: '16px' }}
                      />
                      <span style={{ fontSize: '0.8rem', fontWeight: 600, color: camForm.disabled ? '#fca5a5' : '#94a3b8' }}>
                        {camForm.disabled ? t('cameras.status.disabled', 'Disabled') : t('cameras.status.online', 'Active')}
                      </span>
                    </label>
                  </div>

                  {camForm.disabled && (
                    <div style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
                      <label className="v2-form-label" style={{ color: '#fca5a5' }}>
                        {t('v2.modals.cameraForm.reasonsLabel', 'Reason:')}
                      </label>
                      <div style={{ display: 'flex', gap: '6px', flexWrap: 'wrap' }}>
                        {DISABLE_REASONS.map((r) => {
                          const isSelected = camForm.disableReason === r.value;
                          return (
                            <button
                              key={r.value}
                              type="button"
                              onClick={() => setCamForm({ ...camForm, disableReason: r.value })}
                              style={{
                                background: isSelected ? 'rgba(239, 68, 68, 0.25)' : 'rgba(0, 0, 0, 0.3)',
                                border: '1px solid',
                                borderColor: isSelected ? '#ef4444' : 'rgba(255, 255, 255, 0.1)',
                                color: isSelected ? '#fff' : '#94a3b8',
                                padding: '6px 12px',
                                borderRadius: '6px',
                                fontSize: '0.76rem',
                                fontWeight: 600,
                                cursor: 'pointer',
                              }}
                            >
                              {r.label}
                            </button>
                          );
                        })}
                      </div>
                    </div>
                  )}
                </div>

                <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '12px' }}>
                  <div className="v2-form-group">
                    <label className="v2-form-label">{t('cameras.simPhone', 'SIM Phone Number')}</label>
                    <input
                      type="text"
                      className="v2-input"
                      value={camForm.simPhone || ''}
                      onChange={(e) => setCamForm({ ...camForm, simPhone: e.target.value })}
                      placeholder="+7 999 123-45-67"
                    />
                  </div>

                  <div className="v2-form-group">
                    <label className="v2-form-label">{t('cameras.simIccid', 'SIM ICCID Serial')}</label>
                    <input
                      type="text"
                      className="v2-input"
                      value={camForm.simICCID || ''}
                      onChange={(e) => setCamForm({ ...camForm, simICCID: e.target.value })}
                      placeholder="897010203040506070"
                    />
                  </div>
                </div>

                <div className="v2-form-group">
                  <label className="v2-form-label">{t('tags.title', 'Tags')}</label>
                  <div style={{ display: 'flex', gap: '6px', flexWrap: 'wrap' }}>
                    {globalTags.map((tag) => {
                      const isSelected = camForm.tags.includes(tag.id);
                      return (
                        <button
                          key={tag.id}
                          type="button"
                          onClick={() => toggleTag(tag.id)}
                          style={{
                            background: isSelected ? (tag.color ? `${tag.color}35` : 'rgba(99,102,241,0.3)') : 'rgba(255,255,255,0.05)',
                            color: isSelected ? (tag.color || '#a5b4fc') : '#94a3b8',
                            border: `1px solid ${isSelected ? (tag.color || '#6366f1') : 'rgba(255,255,255,0.1)'}`,
                            padding: '4px 10px',
                            borderRadius: '6px',
                            fontSize: '0.76rem',
                            fontWeight: 600,
                            cursor: 'pointer',
                          }}
                        >
                          {tag.name}
                        </button>
                      );
                    })}
                  </div>
                </div>

                <div className="v2-form-group">
                  <label className="v2-form-label">{t('cameras.comment', 'Comment')}</label>
                  <textarea
                    className="v2-input"
                    rows={2}
                    value={camForm.comment || ''}
                    onChange={(e) => setCamForm({ ...camForm, comment: e.target.value })}
                    placeholder="Location details, technician notes, etc."
                  />
                </div>
              </>
            )}
          </div>

          {/* Footer */}
          <div className="v2-modal-footer">
            <button type="button" className="v2-btn-secondary" onClick={onClose}>
              {t('cameras.cancel', 'Cancel')}
            </button>
            <button type="submit" className="v2-btn-primary">
              {isEditing ? t('cameras.save', 'Save Changes') : t('cameras.add', 'Create Camera')}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
};
