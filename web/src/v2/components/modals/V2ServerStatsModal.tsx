import React from 'react';
import { X, Activity, Cpu, Server, Database, Download, Upload } from 'lucide-react';
import type { ServerStats } from '../../../types';
import { formatBytes } from '../../../utils/formatters';

interface V2ServerStatsModalProps {
  serverStats: ServerStats;
  onClose: () => void;
}

export const V2ServerStatsModal: React.FC<V2ServerStatsModalProps> = ({
  serverStats,
  onClose,
}) => {
  const handleImportBackup = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;

    const formData = new FormData();
    formData.append('backup', file);

    try {
      const token = localStorage.getItem('token');
      const res = await fetch('/api/system/backup/import', {
        method: 'POST',
        headers: {
          Authorization: token ? `Bearer ${token}` : '',
        },
        body: formData,
      });

      if (res.ok) {
        alert('Configuration backup imported successfully! Reloading...');
        window.location.reload();
      } else {
        const err = await res.json();
        alert('Failed to import backup: ' + (err.error || 'Unknown error'));
      }
    } catch (err) {
      console.error(err);
    }
  };

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
              <Activity size={18} color="#818cf8" />
            </div>
            <h3 style={{ margin: 0, fontSize: '1.1rem', fontWeight: 700, color: '#f8fafc' }}>
              Advanced System & Go Diagnostics
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
        <div className="v2-modal-body" style={{ gap: '1.25rem' }}>
          {/* Runtime Metrics Grid */}
          <div
            style={{
              display: 'grid',
              gridTemplateColumns: 'repeat(auto-fit, minmax(190px, 1fr))',
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
              <div style={{ fontSize: '0.72rem', color: '#94a3b8', display: 'flex', alignItems: 'center', gap: '6px' }}>
                <Cpu size={13} color="#38bdf8" />
                <span>ACTIVE GOROUTINES</span>
              </div>
              <div style={{ fontSize: '1.3rem', fontWeight: 700, color: '#38bdf8', marginTop: '4px' }}>
                {serverStats.goroutines}
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
              <div style={{ fontSize: '0.72rem', color: '#94a3b8', display: 'flex', alignItems: 'center', gap: '6px' }}>
                <Server size={13} color="#a5b4fc" />
                <span>LOGICAL CPU CORES</span>
              </div>
              <div style={{ fontSize: '1.3rem', fontWeight: 700, color: '#a5b4fc', marginTop: '4px' }}>
                {serverStats.numCPU || 'N/A'} Cores
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
              <div style={{ fontSize: '0.72rem', color: '#94a3b8' }}>TOTAL HEAP ALLOC</div>
              <div style={{ fontSize: '1.3rem', fontWeight: 700, color: '#10b981', marginTop: '4px' }}>
                {formatBytes(serverStats.memoryUsed)}
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
              <div style={{ fontSize: '0.72rem', color: '#94a3b8' }}>SYSTEM MEMORY (SYS)</div>
              <div style={{ fontSize: '1.3rem', fontWeight: 700, color: '#f8fafc', marginTop: '4px' }}>
                {formatBytes(serverStats.sysMemory || 0)}
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
              <div style={{ fontSize: '0.72rem', color: '#94a3b8' }}>HEAP OBJECTS COUNT</div>
              <div style={{ fontSize: '1.3rem', fontWeight: 700, color: '#f8fafc', marginTop: '4px' }}>
                {(serverStats.heapObjects || 0).toLocaleString()}
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
              <div style={{ fontSize: '0.72rem', color: '#94a3b8' }}>GARBAGE COLLECTIONS</div>
              <div style={{ fontSize: '1.3rem', fontWeight: 700, color: '#f8fafc', marginTop: '4px' }}>
                {(serverStats.numGC || 0).toLocaleString()} runs
              </div>
            </div>
          </div>

          {/* Backup & Restore Management */}
          <div
            style={{
              background: 'rgba(0, 0, 0, 0.3)',
              padding: '14px',
              borderRadius: '10px',
              border: '1px solid rgba(255, 255, 255, 0.08)',
              display: 'flex',
              flexDirection: 'column',
              gap: '10px',
            }}
          >
            <div style={{ fontSize: '0.82rem', fontWeight: 700, color: '#f8fafc', display: 'flex', alignItems: 'center', gap: '6px' }}>
              <Database size={15} color="#6366f1" />
              <span>Full System Configuration Backup</span>
            </div>
            <div style={{ fontSize: '0.75rem', color: '#94a3b8' }}>
              Export or import the entire database of camera streams, folders, tags, and users in JSON format.
            </div>

            <div style={{ display: 'flex', gap: '10px', marginTop: '4px' }}>
              <a
                href={`/api/system/backup/export?token=${localStorage.getItem('token')}`}
                className="v2-btn-primary"
                style={{ textDecoration: 'none' }}
              >
                <Download size={14} />
                <span>Download JSON Backup</span>
              </a>

              <label className="v2-btn-secondary" style={{ cursor: 'pointer' }}>
                <Upload size={14} />
                <span>Upload JSON Backup</span>
                <input
                  type="file"
                  accept=".json"
                  style={{ display: 'none' }}
                  onChange={handleImportBackup}
                />
              </label>
            </div>
          </div>
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
