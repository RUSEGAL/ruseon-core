import { Activity, Server, Users, Network, Camera, Clock } from 'lucide-react';
import type { ServerStats } from '../types';
import { formatBytes, formatUptime } from '../utils/formatters';

interface DashboardStatsProps {
  serverStats: ServerStats;
  onOpenAdvancedStats: () => void;
}

export function DashboardStats({ serverStats, onOpenAdvancedStats }: DashboardStatsProps) {
  return (
    <div className="glass" style={{ 
      display: 'flex', 
      flexWrap: 'wrap', 
      alignItems: 'center',
      gap: '2rem', 
      padding: '1rem 1.5rem', 
      marginBottom: '1.5rem', 
      borderRadius: '12px' 
    }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
        <div style={{ padding: '8px', background: 'rgba(99, 102, 241, 0.1)', borderRadius: '8px', color: 'var(--primary)' }}>
          <Activity size={20} />
        </div>
        <span style={{ fontWeight: 600, fontSize: '1.1rem' }}>Dashboard</span>
      </div>

      <div style={{ height: '32px', width: '1px', background: 'var(--card-border)' }}></div>

      <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
        <Camera size={18} style={{ color: 'var(--text-muted)' }} />
        <div style={{ display: 'flex', flexDirection: 'column' }}>
          <span style={{ fontSize: '0.7rem', color: 'var(--text-muted)', textTransform: 'uppercase', letterSpacing: '0.5px' }}>Cameras</span>
          <span style={{ fontSize: '1rem', fontWeight: 600 }}>
            <span style={{ color: serverStats.onlineCameras === serverStats.totalCameras ? 'var(--success)' : 'var(--danger)' }}>{serverStats.onlineCameras}</span> / {serverStats.totalCameras}
          </span>
        </div>
      </div>

      <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
        <Network size={18} style={{ color: 'var(--text-muted)' }} />
        <div style={{ display: 'flex', flexDirection: 'column' }}>
          <span style={{ fontSize: '0.7rem', color: 'var(--text-muted)', textTransform: 'uppercase', letterSpacing: '0.5px' }}>Traffic</span>
          <span style={{ fontSize: '0.95rem', fontWeight: 500, fontFamily: 'monospace' }}>
            <span style={{ color: 'var(--success)' }}>↓{formatBytes(serverStats.totalBytes)}</span> <span style={{ color: 'var(--primary)', marginLeft: '6px' }}>↑{formatBytes(serverStats.totalBytesSent)}</span>
          </span>
        </div>
      </div>

      <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
        <Users size={18} style={{ color: 'var(--text-muted)' }} />
        <div style={{ display: 'flex', flexDirection: 'column' }}>
          <span style={{ fontSize: '0.7rem', color: 'var(--text-muted)', textTransform: 'uppercase', letterSpacing: '0.5px' }}>Viewers</span>
          <span style={{ fontSize: '1rem', fontWeight: 600, color: '#f59e0b' }}>
            {serverStats.activeClients || 0}
          </span>
        </div>
      </div>

      <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
        <Clock size={18} style={{ color: 'var(--text-muted)' }} />
        <div style={{ display: 'flex', flexDirection: 'column' }}>
          <span style={{ fontSize: '0.7rem', color: 'var(--text-muted)', textTransform: 'uppercase', letterSpacing: '0.5px' }}>Uptime</span>
          <span style={{ fontSize: '0.95rem', fontWeight: 500 }}>
            {formatUptime(serverStats.uptime)}
          </span>
        </div>
      </div>

      <div style={{ flex: 1, minWidth: '20px' }}></div>

      <div 
        style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer', padding: '8px 12px', background: 'rgba(255,255,255,0.05)', borderRadius: '8px', transition: '0.2s', border: '1px solid rgba(255,255,255,0.05)' }} 
        onClick={onOpenAdvancedStats}
        onMouseOver={(e) => e.currentTarget.style.background = 'rgba(255,255,255,0.1)'}
        onMouseOut={(e) => e.currentTarget.style.background = 'rgba(255,255,255,0.05)'}
        title="View detailed stats"
      >
        <Server size={16} style={{ color: 'var(--primary)' }} />
        <span style={{ fontSize: '0.85rem', fontWeight: 500 }}>
          {formatBytes(serverStats.memoryUsed)} / {serverStats.goroutines} Thr
        </span>
      </div>
    </div>
  );
}
