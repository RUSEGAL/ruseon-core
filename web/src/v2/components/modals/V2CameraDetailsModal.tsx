import React, { useState } from 'react';
import {
  X,
  Radio,
  Phone,
  Activity,
  HardDrive,
  Copy,
  Check,
  Zap,
  Tag as TagIcon,
  Clock,
  Power,
  Server,
} from 'lucide-react';
import type { CameraInfo, TagConfig } from '../../../types';
import { formatBytes, formatUptime } from '../../../utils/formatters';

interface V2CameraDetailsModalProps {
  detailsCam: CameraInfo;
  bitrates: Record<string, number>;
  fpsMap: Record<string, number>;
  globalTags: TagConfig[];
  onClose: () => void;
  onCameraUpdated?: () => void;
}

export const V2CameraDetailsModal: React.FC<V2CameraDetailsModalProps> = ({
  detailsCam,
  bitrates,
  fpsMap,
  globalTags,
  onClose,
  onCameraUpdated,
}) => {
  const [copiedKey, setCopiedKey] = useState<string | null>(null);
  const [tokenExpires, setTokenExpires] = useState<number>(3600);
  const [generatedToken, setGeneratedToken] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<'telemetry' | 'cellular' | 'history'>('telemetry');
  const [isUpdatingState, setIsUpdatingState] = useState(false);

  const isOnline = detailsCam.state === 'online' && !detailsCam.disabled;
  const trafficLimit = detailsCam.trafficLimit || 200 * 1024 * 1024 * 1024;
  const trafficUsed = detailsCam.trafficUsed || 0;
  const trafficPercent = Math.min(100, (trafficUsed / trafficLimit) * 100);

  const copyToClipboard = (text: string, key: string) => {
    navigator.clipboard.writeText(text);
    setCopiedKey(key);
    setTimeout(() => setCopiedKey(null), 2000);
  };

  const handleGenerateToken = async () => {
    try {
      const token = localStorage.getItem('token');
      const res = await fetch(`/api/cameras/${detailsCam.id}/token`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: token ? `Bearer ${token}` : '',
        },
        body: JSON.stringify({ expires_in: tokenExpires }),
      });
      if (res.ok) {
        const data = await res.json();
        setGeneratedToken(data.token);
      }
    } catch (e) {
      console.error('Failed to generate token:', e);
    }
  };

  // State Management: Toggle Enabled / Disable with reason
  const handleSetCameraState = async (disabled: boolean, reason: string = 'technical') => {
    try {
      setIsUpdatingState(true);
      const token = localStorage.getItem('token');
      const payload = {
        ...detailsCam,
        disabled,
        disableReason: reason,
      };

      const res = await fetch(`/api/cameras/${detailsCam.id}`, {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
          Authorization: token ? `Bearer ${token}` : '',
        },
        body: JSON.stringify(payload),
      });

      if (res.ok) {
        if (onCameraUpdated) onCameraUpdated();
      }
    } catch (err) {
      console.error('Failed to update camera state:', err);
    } finally {
      setIsUpdatingState(false);
    }
  };

  const formatBitrate = (kbpsVal: number | undefined) => {
    if (!kbpsVal || kbpsVal <= 0) return '0 kbps';
    if (kbpsVal >= 1000) {
      return `${(kbpsVal / 1000).toFixed(2)} Mbps`;
    }
    return `${kbpsVal.toFixed(1)} kbps`;
  };

  const getDisableReasonLabel = (reason: string | undefined) => {
    switch (reason) {
      case 'payment':
        return 'Отключено за неуплату';
      case 'requested':
        return 'Отключено по требованию';
      case 'technical':
      default:
        return 'Отключено по тех. причинам';
    }
  };

  const currentFps = fpsMap[detailsCam.id] || 0;
  const currentBitrate = bitrates[detailsCam.id] || 0;

  return (
    <div className="v2-modal-overlay" onClick={onClose}>
      <div
        className="v2-modal-container"
        onClick={(e) => e.stopPropagation()}
        style={{ width: '740px', maxWidth: '95vw', maxHeight: '88vh' }}
      >
        {/* Header */}
        <div className="v2-modal-header">
          <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
            <div
              style={{
                width: '36px',
                height: '36px',
                borderRadius: '10px',
                background: 'rgba(99, 102, 241, 0.15)',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
              }}
            >
              <Radio size={18} color="#818cf8" />
            </div>
            <div>
              <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                <h3 style={{ margin: 0, fontSize: '1.15rem', fontWeight: 700, color: '#f8fafc' }}>
                  {detailsCam.id.toUpperCase()}
                </h3>
                <span
                  style={{
                    background: detailsCam.disabled
                      ? 'rgba(239, 68, 68, 0.2)'
                      : isOnline
                      ? 'rgba(16, 185, 129, 0.2)'
                      : 'rgba(100, 116, 139, 0.2)',
                    color: detailsCam.disabled
                      ? '#ef4444'
                      : isOnline
                      ? '#10b981'
                      : '#94a3b8',
                    padding: '2px 8px',
                    borderRadius: '6px',
                    fontSize: '0.72rem',
                    fontWeight: 700,
                  }}
                >
                  {detailsCam.disabled
                    ? getDisableReasonLabel(detailsCam.disableReason)
                    : isOnline
                    ? 'ONLINE'
                    : 'OFFLINE'}
                </span>
                {detailsCam.record && (
                  <span
                    style={{
                      background: 'rgba(239, 68, 68, 0.2)',
                      color: '#fca5a5',
                      padding: '2px 8px',
                      borderRadius: '6px',
                      fontSize: '0.72rem',
                      fontWeight: 700,
                    }}
                  >
                    REC (fMP4)
                  </span>
                )}
              </div>
              <div style={{ fontSize: '0.74rem', color: '#94a3b8', marginTop: '2px' }}>
                Codec: {detailsCam.codec || 'Auto'} | Transport: {detailsCam.transport?.toUpperCase() || 'TCP'}
              </div>
            </div>
          </div>

          <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
            <div className="v2-modal-tabs">
              <button
                className={`v2-modal-tab ${activeTab === 'telemetry' ? 'active' : ''}`}
                onClick={() => setActiveTab('telemetry')}
              >
                Telemetry & URLs
              </button>
              <button
                className={`v2-modal-tab ${activeTab === 'cellular' ? 'active' : ''}`}
                onClick={() => setActiveTab('cellular')}
              >
                Cellular & Tags
              </button>
              <button
                className={`v2-modal-tab ${activeTab === 'history' ? 'active' : ''}`}
                onClick={() => setActiveTab('history')}
              >
                History
              </button>
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
        </div>

        {/* Quick State Management Bar */}
        <div
          style={{
            padding: '8px 1.5rem',
            background: 'rgba(0, 0, 0, 0.4)',
            borderBottom: '1px solid var(--v2-card-border)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            gap: '10px',
            flexWrap: 'wrap',
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', gap: '6px', fontSize: '0.76rem', color: '#94a3b8' }}>
            <Power size={14} color="#6366f1" />
            <span>Состояние камеры:</span>
          </div>

          <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
            <button
              disabled={isUpdatingState || !detailsCam.disabled}
              onClick={() => handleSetCameraState(false)}
              style={{
                background: !detailsCam.disabled ? 'rgba(16, 185, 129, 0.3)' : 'rgba(255, 255, 255, 0.05)',
                border: '1px solid',
                borderColor: !detailsCam.disabled ? '#10b981' : 'rgba(255, 255, 255, 0.1)',
                color: !detailsCam.disabled ? '#6ee7b7' : '#94a3b8',
                borderRadius: '6px',
                padding: '4px 10px',
                fontSize: '0.74rem',
                fontWeight: 600,
                cursor: detailsCam.disabled ? 'pointer' : 'default',
              }}
            >
              ✓ Включено (Active)
            </button>

            <button
              disabled={isUpdatingState || (detailsCam.disabled && detailsCam.disableReason === 'technical')}
              onClick={() => handleSetCameraState(true, 'technical')}
              style={{
                background: detailsCam.disabled && detailsCam.disableReason === 'technical' ? 'rgba(239, 68, 68, 0.3)' : 'rgba(255, 255, 255, 0.05)',
                border: '1px solid',
                borderColor: detailsCam.disabled && detailsCam.disableReason === 'technical' ? '#ef4444' : 'rgba(255, 255, 255, 0.1)',
                color: detailsCam.disabled && detailsCam.disableReason === 'technical' ? '#fca5a5' : '#94a3b8',
                borderRadius: '6px',
                padding: '4px 10px',
                fontSize: '0.74rem',
                fontWeight: 600,
                cursor: 'pointer',
              }}
            >
              Тех. причины
            </button>

            <button
              disabled={isUpdatingState || (detailsCam.disabled && detailsCam.disableReason === 'payment')}
              onClick={() => handleSetCameraState(true, 'payment')}
              style={{
                background: detailsCam.disabled && detailsCam.disableReason === 'payment' ? 'rgba(239, 68, 68, 0.3)' : 'rgba(255, 255, 255, 0.05)',
                border: '1px solid',
                borderColor: detailsCam.disabled && detailsCam.disableReason === 'payment' ? '#ef4444' : 'rgba(255, 255, 255, 0.1)',
                color: detailsCam.disabled && detailsCam.disableReason === 'payment' ? '#fca5a5' : '#94a3b8',
                borderRadius: '6px',
                padding: '4px 10px',
                fontSize: '0.74rem',
                fontWeight: 600,
                cursor: 'pointer',
              }}
            >
              За неуплату
            </button>

            <button
              disabled={isUpdatingState || (detailsCam.disabled && detailsCam.disableReason === 'requested')}
              onClick={() => handleSetCameraState(true, 'requested')}
              style={{
                background: detailsCam.disabled && detailsCam.disableReason === 'requested' ? 'rgba(168, 85, 247, 0.3)' : 'rgba(255, 255, 255, 0.05)',
                border: '1px solid',
                borderColor: detailsCam.disabled && detailsCam.disableReason === 'requested' ? '#a855f7' : 'rgba(255, 255, 255, 0.1)',
                color: detailsCam.disabled && detailsCam.disableReason === 'requested' ? '#d8b4fe' : '#94a3b8',
                borderRadius: '6px',
                padding: '4px 10px',
                fontSize: '0.74rem',
                fontWeight: 600,
                cursor: 'pointer',
              }}
            >
              По требованию
            </button>
          </div>
        </div>

        {/* Body */}
        <div className="v2-modal-body">
          {activeTab === 'telemetry' && (
            <div style={{ display: 'flex', flexDirection: 'column', gap: '1.25rem' }}>
              {/* Telemetry Metric Cards */}
              <div
                style={{
                  display: 'grid',
                  gridTemplateColumns: 'repeat(auto-fit, minmax(140px, 1fr))',
                  gap: '10px',
                }}
              >
                <div
                  style={{
                    background: 'rgba(0,0,0,0.3)',
                    padding: '12px',
                    borderRadius: '10px',
                    border: '1px solid rgba(255,255,255,0.06)',
                  }}
                >
                  <div style={{ fontSize: '0.7rem', color: '#94a3b8' }}>LIVE BITRATE</div>
                  <div style={{ fontSize: '1.25rem', fontWeight: 700, color: '#38bdf8', marginTop: '3px' }}>
                    {isOnline ? formatBitrate(currentBitrate) : '0 kbps'}
                  </div>
                </div>

                <div
                  style={{
                    background: 'rgba(0,0,0,0.3)',
                    padding: '12px',
                    borderRadius: '10px',
                    border: '1px solid rgba(255,255,255,0.06)',
                  }}
                >
                  <div style={{ fontSize: '0.7rem', color: '#94a3b8' }}>FRAME RATE</div>
                  <div style={{ fontSize: '1.25rem', fontWeight: 700, color: '#10b981', marginTop: '3px' }}>
                    {isOnline && currentFps ? `${currentFps.toFixed(1)} fps` : '0 fps'}
                  </div>
                </div>

                <div
                  style={{
                    background: 'rgba(0,0,0,0.3)',
                    padding: '12px',
                    borderRadius: '10px',
                    border: '1px solid rgba(255,255,255,0.06)',
                  }}
                >
                  <div style={{ fontSize: '0.7rem', color: '#94a3b8' }}>UPTIME</div>
                  <div style={{ fontSize: '1.25rem', fontWeight: 700, color: '#a5b4fc', marginTop: '3px' }}>
                    {detailsCam.uptime ? formatUptime(detailsCam.uptime) : '0s'}
                  </div>
                </div>

                <div
                  style={{
                    background: 'rgba(0,0,0,0.3)',
                    padding: '12px',
                    borderRadius: '10px',
                    border: '1px solid rgba(255,255,255,0.06)',
                  }}
                >
                  <div style={{ fontSize: '0.7rem', color: '#94a3b8' }}>PROCESSED FRAMES</div>
                  <div style={{ fontSize: '1.25rem', fontWeight: 700, color: '#f8fafc', marginTop: '3px' }}>
                    {(detailsCam.frames || 0).toLocaleString()}
                  </div>
                </div>

                <div
                  style={{
                    background: 'rgba(0,0,0,0.3)',
                    padding: '12px',
                    borderRadius: '10px',
                    border: '1px solid rgba(255,255,255,0.06)',
                  }}
                >
                  <div style={{ fontSize: '0.7rem', color: '#94a3b8' }}>DATA INGEST (IN)</div>
                  <div style={{ fontSize: '1.25rem', fontWeight: 700, color: '#f8fafc', marginTop: '3px' }}>
                    {formatBytes(detailsCam.bytesReceived || 0)}
                  </div>
                </div>
              </div>

              {/* Direct Playback URLs */}
              <div
                style={{
                  background: 'rgba(0,0,0,0.3)',
                  padding: '14px',
                  borderRadius: '12px',
                  border: '1px solid rgba(255,255,255,0.08)',
                  display: 'flex',
                  flexDirection: 'column',
                  gap: '12px',
                }}
              >
                <div style={{ fontSize: '0.84rem', fontWeight: 700, color: '#f8fafc', display: 'flex', alignItems: 'center', gap: '6px' }}>
                  <Server size={15} color="#6366f1" />
                  <span>Direct Streaming & Ingest Endpoints</span>
                </div>

                {/* RTSP Direct */}
                <div className="v2-form-group">
                  <label className="v2-form-label">RTSP Ingest Source URL</label>
                  <div style={{ display: 'flex', gap: '6px' }}>
                    <input
                      readOnly
                      value={detailsCam.url}
                      className="v2-input"
                      style={{ fontFamily: 'monospace', fontSize: '0.78rem' }}
                    />
                    <button
                      className="v2-btn-secondary"
                      onClick={() => copyToClipboard(detailsCam.url, 'rtsp')}
                    >
                      {copiedKey === 'rtsp' ? <Check size={14} color="#10b981" /> : <Copy size={14} />}
                    </button>
                  </div>
                </div>

                {/* HLS Direct */}
                <div className="v2-form-group">
                  <label className="v2-form-label">HLS Stream Playlist (.m3u8)</label>
                  <div style={{ display: 'flex', gap: '6px' }}>
                    <input
                      readOnly
                      value={`${window.location.origin}/stream/hls/${detailsCam.id}/index.m3u8`}
                      className="v2-input"
                      style={{ fontFamily: 'monospace', fontSize: '0.78rem' }}
                    />
                    <button
                      className="v2-btn-secondary"
                      onClick={() =>
                        copyToClipboard(
                          `${window.location.origin}/stream/hls/${detailsCam.id}/index.m3u8`,
                          'hls'
                        )
                      }
                    >
                      {copiedKey === 'hls' ? <Check size={14} color="#10b981" /> : <Copy size={14} />}
                    </button>
                  </div>
                </div>

                {/* WebRTC WHEP */}
                <div className="v2-form-group">
                  <label className="v2-form-label">WebRTC WHEP Ultra-Low Latency Endpoint</label>
                  <div style={{ display: 'flex', gap: '6px' }}>
                    <input
                      readOnly
                      value={`${window.location.origin}/stream/webrtc/whep/${detailsCam.id}`}
                      className="v2-input"
                      style={{ fontFamily: 'monospace', fontSize: '0.78rem' }}
                    />
                    <button
                      className="v2-btn-secondary"
                      onClick={() =>
                        copyToClipboard(
                          `${window.location.origin}/stream/webrtc/whep/${detailsCam.id}`,
                          'whep'
                        )
                      }
                    >
                      {copiedKey === 'whep' ? <Check size={14} color="#10b981" /> : <Copy size={14} />}
                    </button>
                  </div>
                </div>
              </div>

              {/* Secure Token Generator */}
              <div
                style={{
                  background: 'rgba(0,0,0,0.3)',
                  padding: '14px',
                  borderRadius: '12px',
                  border: '1px solid rgba(255,255,255,0.08)',
                  display: 'flex',
                  flexDirection: 'column',
                  gap: '10px',
                }}
              >
                <div style={{ fontSize: '0.84rem', fontWeight: 700, color: '#f8fafc', display: 'flex', alignItems: 'center', gap: '6px' }}>
                  <Zap size={15} color="#f59e0b" />
                  <span>Generate Shareable Token URL</span>
                </div>

                <div style={{ display: 'flex', gap: '8px' }}>
                  <select
                    value={tokenExpires}
                    onChange={(e) => setTokenExpires(Number(e.target.value))}
                    className="v2-input"
                    style={{ flex: 1, cursor: 'pointer' }}
                  >
                    <option value={3600}>Valid for 1 Hour</option>
                    <option value={86400}>Valid for 24 Hours</option>
                    <option value={604800}>Valid for 7 Days</option>
                    <option value={2592000}>Valid for 30 Days</option>
                  </select>

                  <button className="v2-btn-primary" onClick={handleGenerateToken}>
                    Generate Token
                  </button>
                </div>

                {generatedToken && (
                  <div style={{ display: 'flex', gap: '6px', marginTop: '4px' }}>
                    <input
                      readOnly
                      value={`${window.location.origin}/stream/hls/${detailsCam.id}/index.m3u8?token=${generatedToken}`}
                      className="v2-input"
                      style={{ fontFamily: 'monospace', fontSize: '0.76rem', color: '#10b981' }}
                    />
                    <button
                      className="v2-btn-secondary"
                      onClick={() =>
                        copyToClipboard(
                          `${window.location.origin}/stream/hls/${detailsCam.id}/index.m3u8?token=${generatedToken}`,
                          'genToken'
                        )
                      }
                    >
                      {copiedKey === 'genToken' ? <Check size={14} color="#10b981" /> : <Copy size={14} />}
                    </button>
                  </div>
                )}
              </div>
            </div>
          )}

          {activeTab === 'cellular' && (
            <div style={{ display: 'flex', flexDirection: 'column', gap: '1.25rem' }}>
              <div
                style={{
                  display: 'grid',
                  gridTemplateColumns: 'repeat(auto-fit, minmax(280px, 1fr))',
                  gap: '1rem',
                }}
              >
                {/* Phone & ICCID */}
                <div
                  style={{
                    background: 'rgba(0,0,0,0.3)',
                    padding: '1.25rem',
                    borderRadius: '12px',
                    border: '1px solid rgba(255,255,255,0.08)',
                    display: 'flex',
                    flexDirection: 'column',
                    gap: '10px',
                  }}
                >
                  <div style={{ display: 'flex', alignItems: 'center', gap: '8px', color: '#38bdf8' }}>
                    <Phone size={18} />
                    <span style={{ fontWeight: 600 }}>SIM Card Info</span>
                  </div>
                  <div>
                    <div style={{ fontSize: '0.74rem', color: '#94a3b8' }}>Phone Number</div>
                    <div style={{ fontSize: '1.1rem', fontWeight: 700, color: '#f8fafc', marginTop: '2px' }}>
                      {detailsCam.simPhone || 'Not Configured'}
                    </div>
                  </div>
                  <div>
                    <div style={{ fontSize: '0.74rem', color: '#94a3b8' }}>ICCID Serial</div>
                    <div style={{ fontSize: '0.9rem', fontFamily: 'monospace', color: '#a5b4fc', marginTop: '2px' }}>
                      {detailsCam.simICCID || 'Not Configured'}
                    </div>
                  </div>
                </div>

                {/* Quota Progress */}
                <div
                  style={{
                    background: 'rgba(0,0,0,0.3)',
                    padding: '1.25rem',
                    borderRadius: '12px',
                    border: '1px solid rgba(255,255,255,0.08)',
                    display: 'flex',
                    flexDirection: 'column',
                    gap: '10px',
                  }}
                >
                  <div style={{ display: 'flex', alignItems: 'center', gap: '8px', color: '#10b981' }}>
                    <Activity size={18} />
                    <span style={{ fontWeight: 600 }}>Monthly Cellular Quota</span>
                  </div>
                  <div>
                    <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '0.8rem', color: '#94a3b8' }}>
                      <span>Used: {formatBytes(trafficUsed)}</span>
                      <span>Limit: {formatBytes(trafficLimit)}</span>
                    </div>
                    <div
                      style={{
                        height: '8px',
                        background: 'rgba(255,255,255,0.1)',
                        borderRadius: '4px',
                        overflow: 'hidden',
                        marginTop: '8px',
                      }}
                    >
                      <div
                        style={{
                          height: '100%',
                          width: `${trafficPercent}%`,
                          background: trafficPercent > 90 ? '#ef4444' : '#10b981',
                        }}
                      />
                    </div>
                  </div>
                </div>
              </div>

              {/* Tags & Comment */}
              <div
                style={{
                  background: 'rgba(0,0,0,0.3)',
                  padding: '1.25rem',
                  borderRadius: '12px',
                  border: '1px solid rgba(255,255,255,0.08)',
                  display: 'flex',
                  flexDirection: 'column',
                  gap: '10px',
                }}
              >
                <div style={{ display: 'flex', alignItems: 'center', gap: '8px', color: '#a5b4fc' }}>
                  <TagIcon size={18} />
                  <span style={{ fontWeight: 600 }}>Assigned Tags & Notes</span>
                </div>
                <div style={{ display: 'flex', gap: '6px', flexWrap: 'wrap' }}>
                  {detailsCam.tags && detailsCam.tags.length > 0 ? (
                    detailsCam.tags.map((tId) => {
                      const tag = globalTags.find((t) => t.id === tId);
                      if (!tag) return null;
                      return (
                        <span
                          key={tId}
                          style={{
                            background: tag.color ? `${tag.color}25` : 'rgba(99,102,241,0.2)',
                            color: tag.color || '#a5b4fc',
                            border: `1px solid ${tag.color || 'rgba(99,102,241,0.4)'}`,
                            padding: '3px 8px',
                            borderRadius: '6px',
                            fontSize: '0.76rem',
                            fontWeight: 600,
                          }}
                        >
                          {tag.name}
                        </span>
                      );
                    })
                  ) : (
                    <span style={{ color: '#64748b', fontSize: '0.8rem' }}>No tags assigned</span>
                  )}
                </div>
                {detailsCam.comment && (
                  <div style={{ marginTop: '8px', fontSize: '0.82rem', color: '#94a3b8' }}>
                    <strong>Note:</strong> {detailsCam.comment}
                  </div>
                )}
              </div>
            </div>
          )}

          {activeTab === 'history' && (
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '1rem' }}>
              {/* Disable History */}
              <div
                style={{
                  background: 'rgba(0,0,0,0.3)',
                  padding: '1.25rem',
                  borderRadius: '12px',
                  border: '1px solid rgba(255,255,255,0.08)',
                  display: 'flex',
                  flexDirection: 'column',
                  gap: '8px',
                }}
              >
                <div style={{ fontSize: '0.85rem', fontWeight: 700, color: '#ef4444', display: 'flex', alignItems: 'center', gap: '6px' }}>
                  <Clock size={16} />
                  <span>Disable Events History</span>
                </div>
                {detailsCam.disableHistory && detailsCam.disableHistory.length > 0 ? (
                  <div style={{ display: 'flex', flexDirection: 'column', gap: '6px', maxHeight: '300px', overflowY: 'auto' }}>
                    {detailsCam.disableHistory.map((h, i) => (
                      <div
                        key={i}
                        style={{
                          padding: '6px 8px',
                          borderRadius: '6px',
                          background: 'rgba(255,255,255,0.03)',
                          fontSize: '0.76rem',
                          display: 'flex',
                          justifyContent: 'space-between',
                        }}
                      >
                        <span style={{ color: '#f8fafc' }}>
                          {getDisableReasonLabel(h.reason)}
                        </span>
                        <span style={{ color: '#64748b' }}>{new Date(h.timestamp).toLocaleString()}</span>
                      </div>
                    ))}
                  </div>
                ) : (
                  <span style={{ color: '#64748b', fontSize: '0.8rem' }}>No disable events recorded</span>
                )}
              </div>

              {/* Record History */}
              <div
                style={{
                  background: 'rgba(0,0,0,0.3)',
                  padding: '1.25rem',
                  borderRadius: '12px',
                  border: '1px solid rgba(255,255,255,0.08)',
                  display: 'flex',
                  flexDirection: 'column',
                  gap: '8px',
                }}
              >
                <div style={{ fontSize: '0.85rem', fontWeight: 700, color: '#10b981', display: 'flex', alignItems: 'center', gap: '6px' }}>
                  <HardDrive size={16} />
                  <span>Continuous Recording History</span>
                </div>
                {detailsCam.recordHistory && detailsCam.recordHistory.length > 0 ? (
                  <div style={{ display: 'flex', flexDirection: 'column', gap: '6px', maxHeight: '300px', overflowY: 'auto' }}>
                    {detailsCam.recordHistory.map((h, i) => (
                      <div
                        key={i}
                        style={{
                          padding: '6px 8px',
                          borderRadius: '6px',
                          background: 'rgba(255,255,255,0.03)',
                          fontSize: '0.76rem',
                          display: 'flex',
                          justifyContent: 'space-between',
                        }}
                      >
                        <span
                          style={{
                            color:
                              h.action === 'start' || h.action === 'enable' || h.action === 'started'
                                ? '#10b981'
                                : '#ef4444',
                            fontWeight: 600,
                            textTransform: 'capitalize',
                          }}
                        >
                          {h.action || (h.reason ? h.reason : 'Recorded')}
                        </span>
                        <span style={{ color: '#64748b' }}>{new Date(h.timestamp).toLocaleString()}</span>
                      </div>
                    ))}
                  </div>
                ) : (
                  <span style={{ color: '#64748b', fontSize: '0.8rem' }}>No recording status changes</span>
                )}
              </div>
            </div>
          )}
        </div>

        {/* Footer */}
        <div className="v2-modal-footer">
          <button className="v2-btn-secondary" onClick={onClose}>
            Close
          </button>
        </div>
      </div>
    </div>
  );
};
