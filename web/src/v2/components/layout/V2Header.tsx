import React from 'react';
import type { ServerStats } from '../../../types';
import { UiVariantToggle } from '../common/UiVariantToggle';
import { LanguageSwitcher } from '../../../components/LanguageSwitcher';
import { Activity, LogOut, Cpu, Radio, Terminal, Users } from 'lucide-react';

interface V2HeaderProps {
  stats: ServerStats | null;
  userRole?: string;
  onLogout: () => void;
  onOpenStats?: () => void;
  onOpenLogs?: () => void;
  onOpenUsers?: () => void;
}

export const V2Header: React.FC<V2HeaderProps> = ({
  stats,
  userRole,
  onLogout,
  onOpenStats,
  onOpenLogs,
  onOpenUsers,
}) => {
  const isAdmin = userRole === 'admin';
  const formatMem = (bytes: number) => {
    return `${(bytes / (1024 * 1024)).toFixed(0)} MB`;
  };

  return (
    <header className="v2-header">
      {/* Left: Server Telemetry Badges */}
      <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
        {stats && (
          <>
            <button
              onClick={onOpenStats}
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: '6px',
                background: 'rgba(16, 185, 129, 0.12)',
                border: '1px solid rgba(16, 185, 129, 0.25)',
                padding: '4px 10px',
                borderRadius: '6px',
                fontSize: '0.74rem',
                color: '#6ee7b7',
                fontWeight: 600,
                cursor: onOpenStats ? 'pointer' : 'default',
              }}
              title="Click to view full server metrics & connected clients"
            >
              <Radio size={12} className="animate-pulse" />
              <span>{stats.onlineCameras} / {stats.totalCameras} Live</span>
            </button>

            <button
              onClick={onOpenStats}
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: '6px',
                background: 'rgba(255, 255, 255, 0.04)',
                border: '1px solid rgba(255, 255, 255, 0.08)',
                padding: '4px 10px',
                borderRadius: '6px',
                fontSize: '0.74rem',
                color: '#94a3b8',
                cursor: onOpenStats ? 'pointer' : 'default',
              }}
              title="Click to view detailed memory breakdown"
            >
              <Cpu size={12} color="#6366f1" />
              <span>RAM: {stats.memoryUsed ? formatMem(stats.memoryUsed) : 'N/A'}</span>
            </button>

            <div
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: '6px',
                background: 'rgba(255, 255, 255, 0.04)',
                border: '1px solid rgba(255, 255, 255, 0.08)',
                padding: '4px 10px',
                borderRadius: '6px',
                fontSize: '0.74rem',
                color: '#94a3b8',
              }}
            >
              <Activity size={12} color="#38bdf8" />
              <span>Goroutines: {stats.goroutines || 0}</span>
            </div>
          </>
        )}
      </div>

      {/* Right: Actions, Language Switcher, UI Mode Switcher & Logout */}
      <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
        {isAdmin && onOpenUsers && (
          <button
            onClick={onOpenUsers}
            style={{
              background: 'rgba(255, 255, 255, 0.05)',
              border: '1px solid rgba(255, 255, 255, 0.1)',
              borderRadius: '8px',
              color: '#f8fafc',
              padding: '6px 12px',
              fontSize: '0.78rem',
              cursor: 'pointer',
              display: 'flex',
              alignItems: 'center',
              gap: '6px',
            }}
            title="User Management (RBAC)"
          >
            <Users size={14} color="#a5b4fc" />
            <span>Users</span>
          </button>
        )}

        {isAdmin && onOpenLogs && (
          <button
            onClick={onOpenLogs}
            style={{
              background: 'rgba(255, 255, 255, 0.05)',
              border: '1px solid rgba(255, 255, 255, 0.1)',
              borderRadius: '8px',
              color: '#f8fafc',
              padding: '6px 12px',
              fontSize: '0.78rem',
              cursor: 'pointer',
              display: 'flex',
              alignItems: 'center',
              gap: '6px',
            }}
            title="Live Server Logs Stream"
          >
            <Terminal size={14} color="#38bdf8" />
            <span>Logs</span>
          </button>
        )}

        <LanguageSwitcher />

        <UiVariantToggle />

        <button
          onClick={onLogout}
          style={{
            background: 'transparent',
            border: '1px solid rgba(255, 255, 255, 0.1)',
            borderRadius: '8px',
            color: '#94a3b8',
            padding: '6px 12px',
            fontSize: '0.78rem',
            cursor: 'pointer',
            display: 'flex',
            alignItems: 'center',
            gap: '6px',
            transition: 'all 0.15s ease',
          }}
          title="Log out"
        >
          <LogOut size={14} />
          <span>Exit</span>
        </button>
      </div>
    </header>
  );
};
