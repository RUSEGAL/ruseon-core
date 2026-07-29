import { MonitorPlay, LayoutGrid, List, Plus, LogOut } from 'lucide-react';

interface HeaderProps {
  viewMode: 'grid' | 'list';
  setViewMode: (mode: 'grid' | 'list') => void;
  onOpenAdd: () => void;
  onLogout: () => void;
}

export function Header({ viewMode, setViewMode, onOpenAdd, onLogout }: HeaderProps) {
  return (
    <header className="dashboard-header glass">
      <div className="brand">
        <MonitorPlay className="brand-icon" size={28} />
        Gritprof Media Server
      </div>
      
      <div style={{ display: 'flex', gap: '16px', alignItems: 'center' }}>
        <div className="view-toggle">
          <div 
            className={`view-btn ${viewMode === 'grid' ? 'active' : ''}`}
            onClick={() => setViewMode('grid')}
          >
            <LayoutGrid size={16} /> Grid
          </div>
          <div 
            className={`view-btn ${viewMode === 'list' ? 'active' : ''}`}
            onClick={() => setViewMode('list')}
          >
            <List size={16} /> List
          </div>
        </div>
        <button onClick={onOpenAdd} className="btn btn-primary" style={{ display: 'flex', gap: '8px', alignItems: 'center' }}>
          <Plus size={16} /> Add Camera
        </button>
        <div className="status-badge" style={{ border: '1px solid var(--card-border)' }}>
          <div className="status-indicator online"></div>
          System Active
        </div>
        <button onClick={onLogout} className="btn btn-secondary" style={{ display: 'flex', gap: '8px', alignItems: 'center' }} title="Logout">
          <LogOut size={16} /> Logout
        </button>
      </div>
    </header>
  );
}
