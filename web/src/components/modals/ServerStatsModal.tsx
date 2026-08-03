import { X, Activity } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import type { ServerStats } from '../../types';
import { formatBytes } from '../../utils/formatters';

interface ServerStatsModalProps {
  serverStats: ServerStats;
  onClose: () => void;
}

export function ServerStatsModal({ serverStats, onClose }: ServerStatsModalProps) {
  const { t } = useTranslation();
  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal-content glass" style={{ maxWidth: '600px' }} onClick={e => e.stopPropagation()}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '24px' }}>
          <h3 style={{ margin: 0, display: 'flex', alignItems: 'center', gap: '12px' }}>
            <Activity size={24} style={{ color: 'var(--primary)' }} />
            {t('dashboard.advanced')}
          </h3>
          <button className="btn-icon" onClick={onClose}>
            <X size={20} />
          </button>
        </div>

        <div className="details-grid">
          <div className="details-stat">
            <div className="details-stat-label">{t('dashboard.goroutines')}</div>
            <div className="details-stat-val highlight">{serverStats.goroutines}</div>
          </div>
          <div className="details-stat">
            <div className="details-stat-label">Logical CPUs</div>
            <div className="details-stat-val">{serverStats.numCPU || 'N/A'}</div>
          </div>
          <div className="details-stat">
            <div className="details-stat-label">Total Allocated (Heap)</div>
            <div className="details-stat-val">{formatBytes(serverStats.memoryUsed)}</div>
          </div>
          <div className="details-stat">
            <div className="details-stat-label">System Memory (Sys)</div>
            <div className="details-stat-val">{formatBytes(serverStats.sysMemory || 0)}</div>
          </div>
          <div className="details-stat">
            <div className="details-stat-label">Heap In Use</div>
            <div className="details-stat-val">{formatBytes(serverStats.heapAlloc || 0)}</div>
          </div>
          <div className="details-stat">
            <div className="details-stat-label">Heap Reserved (Sys)</div>
            <div className="details-stat-val">{formatBytes(serverStats.heapSys || 0)}</div>
          </div>
          <div className="details-stat">
            <div className="details-stat-label">Heap Objects</div>
            <div className="details-stat-val">{(serverStats.heapObjects || 0).toLocaleString()}</div>
          </div>
          <div className="details-stat">
            <div className="details-stat-label">Garbage Collections</div>
            <div className="details-stat-val">{(serverStats.numGC || 0).toLocaleString()}</div>
          </div>
        </div>
        <div style={{ marginTop: '24px', paddingTop: '20px', borderTop: '1px solid rgba(255,255,255,0.1)' }}>
          <h4 style={{ margin: '0 0 16px 0', fontSize: '1.1rem', color: 'var(--text-main)' }}>{t('nav.backup')}</h4>
          <div style={{ display: 'flex', gap: '12px' }}>
            <a 
              href={`/api/system/backup/export?token=${localStorage.getItem('token')}`}
              className="btn btn-primary" 
              style={{ textDecoration: 'none', display: 'inline-flex', alignItems: 'center', justifyContent: 'center' }}
            >
              Download JSON Backup
            </a>
            
            <label className="btn btn-secondary" style={{ cursor: 'pointer', margin: 0, display: 'inline-flex', alignItems: 'center', justifyContent: 'center' }}>
              Upload JSON Backup
              <input 
                type="file" 
                accept=".json"
                style={{ display: 'none' }}
                onChange={async (e) => {
                  const file = e.target.files?.[0];
                  if (!file) return;
                  
                  const formData = new FormData();
                  formData.append('backup', file);
                  
                  try {
                    const res = await fetch('/api/system/backup/import', {
                      method: 'POST',
                      headers: {
                        'Authorization': `Bearer ${localStorage.getItem('token')}`
                      },
                      body: formData,
                    });
                    
                    if (res.ok) {
                      alert('Backup imported successfully! Streams have been restarted automatically.');
                      window.location.reload();
                    } else {
                      const err = await res.json();
                      alert('Failed to import backup: ' + (err.error || 'Unknown error'));
                    }
                  } catch (err) {
                    alert('Error uploading backup');
                    console.error(err);
                  }
                  e.target.value = '';
                }}
              />
            </label>
          </div>
          <p style={{ marginTop: '12px', fontSize: '0.85rem', color: 'var(--text-muted)' }}>
            System automatically creates binary snapshots in the <code>data/backups/</code> directory every 24 hours.
          </p>
        </div>
      </div>
    </div>
  );
}
