import React from 'react';
import { useTranslation } from 'react-i18next';
import { Logo } from '../../../components/Logo';
import {
  Grid,
  Film,
  Cpu,
  Activity,
  Settings,
  ChevronLeft,
  ChevronRight,
  Layers,
} from 'lucide-react';

export type V2NavRoute = 'dashboard' | 'surveillance' | 'archive' | 'ai_events' | 'topology' | 'settings';

interface V2SidebarProps {
  currentRoute: V2NavRoute;
  onRouteChange: (route: V2NavRoute) => void;
  collapsed: boolean;
  onToggleCollapsed: () => void;
}

export const V2Sidebar: React.FC<V2SidebarProps> = ({
  currentRoute,
  onRouteChange,
  collapsed,
  onToggleCollapsed,
}) => {
  const { t } = useTranslation();

  const navItems = [
    { id: 'dashboard' as V2NavRoute, label: t('v2.nav.overview', 'Overview'), icon: <Layers size={18} /> },
    { id: 'surveillance' as V2NavRoute, label: t('v2.nav.surveillance', 'Surveillance Grid'), icon: <Grid size={18} /> },
    { id: 'archive' as V2NavRoute, label: t('v2.nav.archive', 'Archive & Timeline'), icon: <Film size={18} /> },
    { id: 'ai_events' as V2NavRoute, label: t('v2.nav.ai_events', 'AI Metadata Stream'), icon: <Cpu size={18} /> },
    { id: 'topology' as V2NavRoute, label: t('v2.nav.topology', 'Topology & Stats'), icon: <Activity size={18} /> },
    { id: 'settings' as V2NavRoute, label: t('v2.nav.settings', 'Camera Settings'), icon: <Settings size={18} /> },
  ];

  return (
    <aside className={`v2-sidebar ${collapsed ? 'collapsed' : ''}`}>
      {/* Brand Header */}
      <div className="v2-sidebar-brand">
        <div
          style={{
            width: '32px',
            height: '32px',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            flexShrink: 0,
          }}
        >
          <Logo size={30} />
        </div>
        {!collapsed && (
          <div style={{ display: 'flex', flexDirection: 'column' }}>
            <span style={{ fontWeight: 700, fontSize: '0.95rem', letterSpacing: '-0.3px', color: '#f8fafc' }}>
              RUSEON Core
            </span>
            <span style={{ fontSize: '0.65rem', color: '#818cf8', fontWeight: 700, letterSpacing: '0.5px' }}>
              NEXT-GEN
            </span>
          </div>
        )}
      </div>

      {/* Nav List */}
      <nav className="v2-sidebar-nav">
        {navItems.map((item) => {
          const isActive = currentRoute === item.id;
          return (
            <button
              key={item.id}
              className={`v2-nav-item ${isActive ? 'active' : ''}`}
              onClick={() => onRouteChange(item.id)}
              style={{
                justifyContent: collapsed ? 'center' : 'flex-start',
                border: 'none',
                background: 'transparent',
                textAlign: 'left',
                width: '100%',
              }}
              title={collapsed ? item.label : undefined}
            >
              <div style={{ color: isActive ? '#a5b4fc' : '#94a3b8', flexShrink: 0 }}>
                {item.icon}
              </div>
              {!collapsed && <span>{item.label}</span>}
            </button>
          );
        })}
      </nav>

      {/* Collapse Toggle */}
      <div
        style={{
          padding: '0.75rem',
          borderTop: '1px solid var(--v2-card-border)',
          display: 'flex',
          justifyContent: collapsed ? 'center' : 'flex-end',
        }}
      >
        <button
          onClick={onToggleCollapsed}
          style={{
            background: 'transparent',
            border: 'none',
            color: '#64748b',
            cursor: 'pointer',
            padding: '6px',
            borderRadius: '6px',
            display: 'flex',
            alignItems: 'center',
          }}
          title={collapsed ? 'Expand Sidebar' : 'Collapse Sidebar'}
        >
          {collapsed ? <ChevronRight size={18} /> : <ChevronLeft size={18} />}
        </button>
      </div>
    </aside>
  );
};
