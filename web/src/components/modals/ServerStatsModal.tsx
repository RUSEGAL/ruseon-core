import { X, Activity } from 'lucide-react';
import type { ServerStats } from '../../types';
import { formatBytes } from '../../utils/formatters';

interface ServerStatsModalProps {
  serverStats: ServerStats;
  onClose: () => void;
}

export function ServerStatsModal({ serverStats, onClose }: ServerStatsModalProps) {
  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal-content glass" style={{ maxWidth: '600px' }} onClick={e => e.stopPropagation()}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '24px' }}>
          <h3 style={{ margin: 0, display: 'flex', alignItems: 'center', gap: '12px' }}>
            <Activity size={24} style={{ color: 'var(--primary)' }} />
            Advanced Server Metrics
          </h3>
          <button className="btn-icon" onClick={onClose}>
            <X size={20} />
          </button>
        </div>

        <div className="details-grid">
          <div className="details-stat">
            <div className="details-stat-label">Goroutines (Threads)</div>
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
      </div>
    </div>
  );
}
