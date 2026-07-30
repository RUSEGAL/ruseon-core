import { X, ShieldAlert, Activity, HardDrive, Wifi, Tag, Phone, Key, Clock, FileText, Copy, Check } from 'lucide-react';
import { useState } from 'react';
import type { CameraInfo, TagConfig } from '../../types';
import { formatBytes, formatUptime } from '../../utils/formatters';
import { VideoPlayer } from '../VideoPlayer';

interface CameraDetailsModalProps {
  detailsCam: CameraInfo;
  bitrates: Record<string, number>;
  fpsMap: Record<string, number>;
  onClose: () => void;
  globalTags: TagConfig[];
}

export function CameraDetailsModal({ detailsCam, bitrates, fpsMap, onClose, globalTags }: CameraDetailsModalProps) {
  const [copied, setCopied] = useState(false);
  const isOnline = detailsCam.connected && !detailsCam.disabled;
  const trafficPercent = Math.min(100, ((detailsCam.trafficUsed || 0) / (detailsCam.trafficLimit || 200 * 1024 * 1024 * 1024)) * 100);

  return (
    <div className="modal-overlay" onClick={onClose} style={{ padding: '1rem' }}>
      <div 
        className="modal-content glass details-modal" 
        onClick={e => e.stopPropagation()}
        style={{ maxWidth: '900px', width: '100%', padding: '0', overflow: 'hidden' }}
      >
        {/* HEADER */}
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', padding: '1.5rem', borderBottom: '1px solid var(--card-border)', background: 'rgba(255,255,255,0.02)' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '1rem' }}>
            <h3 style={{ margin: 0, fontSize: '1.5rem', fontWeight: 600 }}>{detailsCam.id.toUpperCase()}</h3>
            <span style={{
              padding: '4px 10px', borderRadius: '12px', fontSize: '0.8rem', fontWeight: 600,
              background: detailsCam.disabled ? 'rgba(239, 68, 68, 0.1)' : (detailsCam.connected ? 'rgba(16, 185, 129, 0.1)' : 'rgba(100, 116, 139, 0.1)'),
              color: detailsCam.disabled ? 'var(--danger)' : (detailsCam.connected ? 'var(--success)' : 'var(--text-muted)'),
              border: `1px solid ${detailsCam.disabled ? 'var(--danger)' : (detailsCam.connected ? 'var(--success)' : 'var(--text-muted)')}`
            }}>
              {detailsCam.disabled ? (detailsCam.disableReason ? `DISABLED: ${detailsCam.disableReason.toUpperCase()}` : 'DISABLED') : (detailsCam.connected ? 'ONLINE' : 'OFFLINE')}
            </span>
            {detailsCam.record && <span style={{ padding: '4px 10px', borderRadius: '12px', fontSize: '0.8rem', background: 'rgba(59, 130, 246, 0.1)', color: 'var(--primary)', border: '1px solid var(--primary)' }}>REC</span>}
          </div>
          <button className="btn-icon" onClick={onClose}>
            <X size={24} />
          </button>
        </div>

        {/* BODY */}
        <div style={{ display: 'flex', flexWrap: 'wrap', gap: '2rem', padding: '1.5rem' }}>
          
          {/* LEFT COLUMN: Player & Traffic */}
          <div style={{ flex: '1 1 350px', display: 'flex', flexDirection: 'column', gap: '1.5rem' }}>
            <div style={{ borderRadius: '12px', overflow: 'hidden', border: '1px solid var(--card-border)', background: '#000', aspectRatio: '16/9', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
              {isOnline ? (
                <VideoPlayer streamId={detailsCam.id} />
              ) : (
                <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', color: 'var(--text-muted)' }}>
                  <ShieldAlert size={48} style={{ color: 'var(--danger)', marginBottom: '1rem', opacity: 0.8 }} />
                  <span style={{ fontSize: '1.1rem' }}>{detailsCam.disabled ? 'Stream is Disabled' : 'Camera Offline'}</span>
                </div>
              )}
            </div>

            <div className="glass" style={{ padding: '1.2rem', borderRadius: '12px' }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: '8px', fontSize: '0.9rem' }}>
                <span style={{ color: 'var(--text-muted)', display: 'flex', alignItems: 'center', gap: '6px' }}><Activity size={16} /> Traffic Usage</span>
                <span style={{ fontWeight: 600 }}>{trafficPercent.toFixed(1)}%</span>
              </div>
              <div style={{ width: '100%', height: '8px', background: 'rgba(255,255,255,0.1)', borderRadius: '4px', overflow: 'hidden' }}>
                <div style={{
                  height: '100%',
                  width: `${trafficPercent}%`,
                  background: trafficPercent > 90 ? 'var(--danger)' : 'var(--primary)',
                  transition: 'width 0.3s ease'
                }}></div>
              </div>
              <div style={{ display: 'flex', justifyContent: 'space-between', marginTop: '8px', fontSize: '0.8rem', color: 'var(--text-muted)' }}>
                <span>{formatBytes(detailsCam.trafficUsed || 0)}</span>
                <span>{formatBytes(detailsCam.trafficLimit || 200 * 1024 * 1024 * 1024)}</span>
              </div>
            </div>

            {detailsCam.tags && detailsCam.tags.length > 0 && (
              <div style={{ display: 'flex', flexWrap: 'wrap', gap: '8px' }}>
                {detailsCam.tags.map(tId => {
                  const tag = globalTags.find(gt => gt.id === tId);
                  if (!tag) return null;
                  return (
                    <span key={tag.id} style={{ display: 'flex', alignItems: 'center', gap: '4px', background: `${tag.color}22`, border: `1px solid ${tag.color}`, color: '#fff', padding: '4px 10px', borderRadius: '16px', fontSize: '0.85rem' }}>
                      <Tag size={12} color={tag.color} /> {tag.name}
                    </span>
                  );
                })}
              </div>
            )}

            <div className="glass" style={{ padding: '1.2rem', borderRadius: '12px' }}>
              <h4 style={{ margin: '0 0 8px 0', display: 'flex', alignItems: 'center', gap: '8px', fontSize: '0.9rem' }}><HardDrive size={16} color="var(--primary)"/> Processing</h4>
              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '8px' }}>
                <div>
                  <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>Data In</div>
                  <div style={{ fontSize: '0.85rem', color: '#fff' }}>{formatBytes(detailsCam.bytesReceived)}</div>
                </div>
                <div>
                  <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>Data Out (HLS)</div>
                  <div style={{ fontSize: '0.85rem', color: '#fff' }}>{formatBytes(detailsCam.bytesSent || 0)}</div>
                </div>
                <div>
                  <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>Frames</div>
                  <div style={{ fontSize: '0.85rem', color: '#fff' }}>{detailsCam.frames}</div>
                </div>
              </div>
            </div>
          </div>

          {/* RIGHT COLUMN: Stats & Info */}
          <div style={{ flex: '2 1 400px', display: 'flex', flexDirection: 'column', gap: '1.5rem' }}>
            
            {/* Realtime Stats */}
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(140px, 1fr))', gap: '1rem' }}>
              <div className="glass" style={{ padding: '1rem', borderRadius: '12px' }}>
                <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', textTransform: 'uppercase', marginBottom: '4px' }}>Bitrate</div>
                <div style={{ fontSize: '1.25rem', fontWeight: 600, color: 'var(--primary)' }}>{isOnline ? `${bitrates[detailsCam.id]?.toFixed(1) || 0} kbps` : '-'}</div>
              </div>
              <div className="glass" style={{ padding: '1rem', borderRadius: '12px' }}>
                <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', textTransform: 'uppercase', marginBottom: '4px' }}>FPS</div>
                <div style={{ fontSize: '1.25rem', fontWeight: 600, color: 'var(--primary)' }}>{isOnline ? `${fpsMap[detailsCam.id]?.toFixed(1) || 0}` : '-'}</div>
              </div>
              <div className="glass" style={{ padding: '1rem', borderRadius: '12px' }}>
                <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', textTransform: 'uppercase', marginBottom: '4px' }}>Codec</div>
                <div style={{ fontSize: '1.25rem', fontWeight: 600 }}>{detailsCam.codec || '-'}</div>
              </div>
              <div className="glass" style={{ padding: '1rem', borderRadius: '12px' }}>
                <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', textTransform: 'uppercase', marginBottom: '4px' }}>Uptime</div>
                <div style={{ fontSize: '1.25rem', fontWeight: 600 }}>{isOnline ? formatUptime(detailsCam.uptime) : '-'}</div>
              </div>
            </div>

            {/* Network & Device Info */}
            <div className="glass" style={{ padding: '1.2rem', borderRadius: '12px', display: 'flex', flexDirection: 'column', gap: '1rem' }}>
              <h4 style={{ margin: 0, display: 'flex', alignItems: 'center', gap: '8px', fontSize: '1rem' }}><Wifi size={16} color="var(--primary)"/> Network Info</h4>
              
              <div style={{ display: 'flex', flexDirection: 'column', gap: '12px' }}>
                <div>
                  <div style={{ fontSize: '0.8rem', color: 'var(--text-muted)', marginBottom: '2px' }}>Source RTSP URL</div>
                  <div style={{ fontSize: '0.9rem', wordBreak: 'break-all', fontFamily: 'monospace', background: 'rgba(0,0,0,0.2)', padding: '8px', borderRadius: '6px' }}>{detailsCam.url}</div>
                </div>
                
                <div>
                  <div style={{ fontSize: '0.8rem', color: 'var(--text-muted)', marginBottom: '2px' }}>Output HLS URL</div>
                  <div 
                    style={{ fontSize: '0.9rem', wordBreak: 'break-all', fontFamily: 'monospace', background: 'rgba(0,0,0,0.2)', padding: '8px', borderRadius: '6px', cursor: 'pointer', display: 'flex', justifyContent: 'space-between', alignItems: 'center', transition: '0.2s' }}
                    onClick={() => {
                      const hlsUrl = `${window.location.protocol}//${window.location.hostname}:8080/stream/hls/${detailsCam.id}/index.m3u8`;
                      navigator.clipboard.writeText(hlsUrl);
                      setCopied(true);
                      setTimeout(() => setCopied(false), 2000);
                    }}
                    onMouseOver={(e) => e.currentTarget.style.background = 'rgba(255,255,255,0.05)'}
                    onMouseOut={(e) => e.currentTarget.style.background = 'rgba(0,0,0,0.2)'}
                    title="Click to copy URL"
                  >
                    <span>{`${window.location.protocol}//${window.location.hostname}:8080/stream/hls/${detailsCam.id}/index.m3u8`}</span>
                    {copied ? <Check size={16} color="var(--success)" /> : <Copy size={16} style={{ color: 'var(--text-muted)' }} />}
                  </div>
                </div>
              </div>
              
              {(detailsCam.simPhone || detailsCam.simICCID) && (
                <div style={{ display: 'flex', gap: '2rem' }}>
                  {detailsCam.simPhone && (
                    <div>
                      <div style={{ fontSize: '0.8rem', color: 'var(--text-muted)', marginBottom: '2px', display: 'flex', alignItems: 'center', gap: '4px' }}><Phone size={12}/> SIM Phone</div>
                      <div style={{ fontSize: '1rem' }}>{detailsCam.simPhone}</div>
                    </div>
                  )}
                  {detailsCam.simICCID && (
                    <div>
                      <div style={{ fontSize: '0.8rem', color: 'var(--text-muted)', marginBottom: '2px', display: 'flex', alignItems: 'center', gap: '4px' }}><Key size={12}/> ICCID</div>
                      <div style={{ fontSize: '1rem', fontFamily: 'monospace' }}>{detailsCam.simICCID}</div>
                    </div>
                  )}
                </div>
              )}
            </div>

            {/* Comment */}
            {detailsCam.comment && (
              <div className="glass" style={{ padding: '1.2rem', borderRadius: '12px' }}>
                <h4 style={{ margin: '0 0 8px 0', display: 'flex', alignItems: 'center', gap: '8px', fontSize: '0.9rem' }}><FileText size={16} color="var(--primary)"/> Comment</h4>
                <div style={{ fontSize: '0.9rem', color: '#fff', whiteSpace: 'pre-wrap', lineHeight: '1.4' }}>{detailsCam.comment}</div>
              </div>
            )}

            {/* History */}
            {(detailsCam.disableHistory && detailsCam.disableHistory.length > 0) || (detailsCam.recordHistory && detailsCam.recordHistory.length > 0) ? (
              <div style={{ display: 'flex', gap: '1rem', flex: 1, flexWrap: 'wrap' }}>
                {detailsCam.disableHistory && detailsCam.disableHistory.length > 0 && (
                  <div className="glass" style={{ padding: '1.2rem', borderRadius: '12px', flex: '1 1 200px', display: 'flex', flexDirection: 'column' }}>
                    <h4 style={{ margin: '0 0 12px 0', display: 'flex', alignItems: 'center', gap: '8px', fontSize: '0.9rem' }}><Clock size={16} color="var(--primary)"/> Stream History</h4>
                    <div style={{ maxHeight: '250px', overflowY: 'auto', paddingRight: '8px' }}>
                      {detailsCam.disableHistory.slice().reverse().map((h, i) => (
                        <div key={i} style={{ display: 'flex', alignItems: 'center', gap: '12px', fontSize: '0.85rem', padding: '8px 0', borderBottom: '1px solid rgba(255,255,255,0.05)' }}>
                          <span style={{ color: 'var(--text-muted)', minWidth: '130px' }}>{new Date(h.timestamp).toLocaleString()}</span>
                          <span style={{ color: h.action === 'enable' ? 'var(--success)' : 'var(--danger)', fontWeight: 600, width: '60px' }}>{h.action.toUpperCase()}</span>
                          <span style={{ color: '#fff' }}>{h.reason || '-'}</span>
                        </div>
                      ))}
                    </div>
                  </div>
                )}

                {detailsCam.recordHistory && detailsCam.recordHistory.length > 0 && (
                  <div className="glass" style={{ padding: '1.2rem', borderRadius: '12px', flex: '1 1 200px', display: 'flex', flexDirection: 'column' }}>
                    <h4 style={{ margin: '0 0 12px 0', display: 'flex', alignItems: 'center', gap: '8px', fontSize: '0.9rem' }}><HardDrive size={16} color="var(--primary)"/> Record History</h4>
                    <div style={{ maxHeight: '250px', overflowY: 'auto', paddingRight: '8px' }}>
                      {detailsCam.recordHistory.slice().reverse().map((h, i) => (
                        <div key={i} style={{ display: 'flex', alignItems: 'center', gap: '12px', fontSize: '0.85rem', padding: '8px 0', borderBottom: '1px solid rgba(255,255,255,0.05)' }}>
                          <span style={{ color: 'var(--text-muted)', minWidth: '130px' }}>{new Date(h.timestamp).toLocaleString()}</span>
                          <span style={{ color: h.action === 'enable' ? 'var(--success)' : 'var(--danger)', fontWeight: 600 }}>
                            {h.action === 'enable' ? 'START REC' : 'STOP REC'}
                          </span>
                        </div>
                      ))}
                    </div>
                  </div>
                )}
              </div>
            ) : null}

          </div>
        </div>
      </div>
    </div>
  );
}
