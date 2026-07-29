import { Activity } from 'lucide-react';
import type { ServerStats } from '../types';
import { formatBytes, formatUptime } from '../utils/formatters';

interface DashboardStatsProps {
  serverStats: ServerStats;
  onOpenAdvancedStats: () => void;
}

export function DashboardStats({ serverStats, onOpenAdvancedStats }: DashboardStatsProps) {
  return (
    <>
      <h2 className="section-title" style={{ marginTop: 0 }}>
        <Activity size={24} style={{ color: 'var(--primary)' }} />
        System Dashboard
      </h2>
      <div style={{ marginBottom: '2.5rem', display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(220px, 1fr))', gap: '1rem' }}>
        <div 
          className="glass" 
          style={{ padding: '1.2rem', borderRadius: '8px', borderLeft: '4px solid var(--primary)', cursor: 'pointer' }}
          onClick={onOpenAdvancedStats}
          title="Click for detailed server stats"
        >
          <div style={{ color: 'var(--text-muted)', fontSize: '0.8rem', textTransform: 'uppercase', marginBottom: '8px', display: 'flex', justifyContent: 'space-between' }}>
            <span>Server Load</span>
            <Activity size={14} style={{ color: 'var(--primary)' }} />
          </div>
          <div style={{ fontSize: '1.8rem', fontWeight: 600 }}>
            {serverStats.goroutines} <span style={{ fontSize: '1rem', fontWeight: 'normal', color: 'var(--text-muted)' }}>threads</span>
          </div>
          <div style={{ fontSize: '0.9rem', color: 'var(--primary)', marginTop: '4px' }}>
            {formatBytes(serverStats.memoryUsed)} RAM Used
          </div>
        </div>
        <div className="glass" style={{ padding: '1.2rem', borderRadius: '8px', borderLeft: '4px solid #8b5cf6' }}>
          <div style={{ color: 'var(--text-muted)', fontSize: '0.8rem', textTransform: 'uppercase', marginBottom: '8px' }}>System Uptime</div>
          <div style={{ fontSize: '1.8rem', fontWeight: 600 }}>{formatUptime(serverStats.uptime)}</div>
          <div style={{ fontSize: '0.9rem', color: '#8b5cf6', marginTop: '4px' }}>Continuous Operation</div>
        </div>
        <div className="glass" style={{ padding: '1.2rem', borderRadius: '8px', borderLeft: '4px solid #10b981' }}>
          <div style={{ color: 'var(--text-muted)', fontSize: '0.8rem', textTransform: 'uppercase', marginBottom: '8px' }}>Network Traffic</div>
          <div style={{ fontSize: '1.4rem', fontWeight: 600 }}>In: {formatBytes(serverStats.totalBytes)}</div>
          <div style={{ fontSize: '1.4rem', fontWeight: 600 }}>Out: {formatBytes(serverStats.totalBytesSent)}</div>
          <div style={{ fontSize: '0.9rem', color: '#10b981', marginTop: '4px' }}>{serverStats.totalFrames.toLocaleString()} frames parsed</div>
        </div>
        <div className="glass" style={{ padding: '1.2rem', borderRadius: '8px', borderLeft: `4px solid ${serverStats.onlineCameras === serverStats.totalCameras ? '#3b82f6' : '#ef4444'}` }}>
          <div style={{ color: 'var(--text-muted)', fontSize: '0.8rem', textTransform: 'uppercase', marginBottom: '8px' }}>Cameras Status</div>
          <div style={{ fontSize: '1.8rem', fontWeight: 600 }}>{serverStats.onlineCameras} / {serverStats.totalCameras}</div>
          <div style={{ fontSize: '0.9rem', color: serverStats.onlineCameras === serverStats.totalCameras ? '#3b82f6' : '#ef4444', marginTop: '4px' }}>Online & Streaming</div>
        </div>
        <div className="glass" style={{ padding: '1.2rem', borderRadius: '8px', borderLeft: '4px solid #f59e0b', display: 'flex', flexDirection: 'column' }}>
          <div style={{ color: 'var(--text-muted)', fontSize: '0.8rem', textTransform: 'uppercase', marginBottom: '8px' }}>Active Clients (HLS)</div>
          <div style={{ fontSize: '1.8rem', fontWeight: 600, marginBottom: '8px' }}>{serverStats.activeClients || 0}</div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: '4px', overflowY: 'auto', maxHeight: '60px' }}>
            {serverStats.clients && serverStats.clients.length > 0 ? (
              serverStats.clients.map((client, i) => (
                <div key={i} style={{ display: 'flex', justifyContent: 'space-between', fontSize: '0.8rem', background: 'rgba(255,255,255,0.05)', padding: '2px 6px', borderRadius: '4px' }}>
                  <span style={{ color: '#f8fafc' }}>{client.ip}</span>
                  <span style={{ color: '#f59e0b', fontWeight: 'bold' }}>{client.streamId.toUpperCase()}</span>
                </div>
              ))
            ) : (
              <div style={{ fontSize: '0.8rem', color: 'var(--text-muted)' }}>No active viewers</div>
            )}
          </div>
        </div>
      </div>
    </>
  );
}
