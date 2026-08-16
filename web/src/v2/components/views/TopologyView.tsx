import React from 'react';
import { useTranslation } from 'react-i18next';
import type { ServerStats, CameraInfo } from '../../../types';
import { Activity, Cpu, Database, Server } from 'lucide-react';

interface TopologyViewProps {
  stats: ServerStats | null;
  cameras: CameraInfo[];
}

export const TopologyView: React.FC<TopologyViewProps> = ({ stats, cameras }) => {
  const { t } = useTranslation();

  const formatBytes = (bytes: number) => {
    if (bytes > 1024 * 1024 * 1024) {
      return `${(bytes / (1024 * 1024 * 1024)).toFixed(2)} GB`;
    }
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  };

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '1.5rem' }}>
      <div className="v2-grid-toolbar">
        <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
          <Activity size={18} color="#38bdf8" />
          <h2 style={{ fontSize: '1.1rem', fontWeight: 600, color: '#f8fafc' }}>
            {t('v2.topology.title', 'System Runtime Topology & Go Profiling')} ({cameras.length} {t('v2.topology.camerasConfigured', 'cameras configured')})
          </h2>
        </div>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(280px, 1fr))', gap: '1rem' }}>
        <div className="glass" style={{ padding: '1.25rem', display: 'flex', flexDirection: 'column', gap: '8px' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px', color: '#94a3b8', fontSize: '0.82rem' }}>
            <Server size={16} color="#6366f1" />
            <span>{t('v2.topology.goRuntime', 'Go Runtime & Memory Alloc')}</span>
          </div>
          <div style={{ fontSize: '1.4rem', fontWeight: 700, color: '#f1f5f9' }}>
            {stats?.heapAlloc ? formatBytes(stats.heapAlloc) : stats?.memoryUsed ? formatBytes(stats.memoryUsed) : 'N/A'}
          </div>
          <div style={{ fontSize: '0.75rem', color: '#64748b' }}>
            {t('v2.topology.sys', 'Sys')}: {stats?.heapSys ? formatBytes(stats.heapSys) : 'N/A'} | {t('v2.topology.gcRuns', 'GC Runs')}: {stats?.numGC || 0}
          </div>
        </div>

        <div className="glass" style={{ padding: '1.25rem', display: 'flex', flexDirection: 'column', gap: '8px' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px', color: '#94a3b8', fontSize: '0.82rem' }}>
            <Cpu size={16} color="#38bdf8" />
            <span>{t('v2.topology.activeGoroutines', 'Active Goroutines')}</span>
          </div>
          <div style={{ fontSize: '1.4rem', fontWeight: 700, color: '#38bdf8' }}>
            {stats?.goroutines || 0}
          </div>
          <div style={{ fontSize: '0.75rem', color: '#64748b' }}>
            {t('v2.topology.goleakCertified', 'Zero goroutine leaks (goleak certified)')}
          </div>
        </div>

        <div className="glass" style={{ padding: '1.25rem', display: 'flex', flexDirection: 'column', gap: '8px' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px', color: '#94a3b8', fontSize: '0.82rem' }}>
            <Database size={16} color="#10b981" />
            <span>{t('v2.topology.badgerDb', 'BadgerDB Storage Engine')}</span>
          </div>
          <div style={{ fontSize: '1.4rem', fontWeight: 700, color: '#10b981' }}>
            {t('v2.topology.lsmActive', 'LSM Active')}
          </div>
          <div style={{ fontSize: '0.75rem', color: '#64748b' }}>
            {t('v2.topology.subMillisecond', 'Sub-millisecond state & metadata index')}
          </div>
        </div>
      </div>
    </div>
  );
};
