import { LayoutGrid, List, Plus, LogOut, Tag, Terminal, Folder, Users } from 'lucide-react';
import { Logo } from './Logo';
import { useTranslation } from 'react-i18next';
import { LanguageSwitcher } from './LanguageSwitcher';

interface HeaderProps {
  viewMode: 'grid' | 'list';
  setViewMode: (mode: 'grid' | 'list') => void;
  onOpenAdd: () => void;
  onOpenTags: () => void;
  onOpenFolders: () => void;
  onOpenLogs: () => void;
  onOpenUsers: () => void;
  onLogout: () => void;
  userRole: string;
}

export function Header({ viewMode, setViewMode, onOpenAdd, onOpenTags, onOpenFolders, onOpenLogs, onOpenUsers, onLogout, userRole }: HeaderProps) {
  const { t } = useTranslation();
  
  const isAdmin = userRole === 'admin';
  const isOperator = userRole === 'operator';
  const canEditCameras = isAdmin || isOperator;

  return (
    <header className="dashboard-header glass">
      <div className="brand">
        <Logo size={28} className="brand-icon" />
        {t('app.title')}
      </div>
      
      <div style={{ display: 'flex', gap: '16px', alignItems: 'center' }}>
        <div className="view-toggle">
          <div 
            className={`view-btn ${viewMode === 'grid' ? 'active' : ''}`}
            onClick={() => setViewMode('grid')}
          >
            <LayoutGrid size={16} /> {t('nav.grid')}
          </div>
          <div 
            className={`view-btn ${viewMode === 'list' ? 'active' : ''}`}
            onClick={() => setViewMode('list')}
          >
            <List size={16} /> {t('nav.list')}
          </div>
        </div>
        <LanguageSwitcher />
        
        {isAdmin && (
          <>
            <button onClick={onOpenFolders} className="btn btn-secondary" style={{ display: 'flex', gap: '8px', alignItems: 'center' }}>
              <Folder size={16} /> {t('folders.title', 'Folders')}
            </button>
            <button onClick={onOpenTags} className="btn btn-secondary" style={{ display: 'flex', gap: '8px', alignItems: 'center' }}>
              <Tag size={16} /> {t('nav.tags')}
            </button>
            <button onClick={onOpenUsers} className="btn btn-secondary" style={{ display: 'flex', gap: '8px', alignItems: 'center' }}>
              <Users size={16} /> {t('nav.users', 'Users')}
            </button>
            <button onClick={onOpenLogs} className="btn btn-secondary" style={{ display: 'flex', gap: '8px', alignItems: 'center' }}>
              <Terminal size={16} /> {t('nav.logs')}
            </button>
          </>
        )}
        
        {canEditCameras && (
          <button onClick={onOpenAdd} className="btn btn-primary" style={{ display: 'flex', gap: '8px', alignItems: 'center' }}>
            <Plus size={16} /> {t('cameras.add')}
          </button>
        )}
        
        <button onClick={onLogout} className="btn btn-secondary" style={{ display: 'flex', gap: '8px', alignItems: 'center' }} title={t('nav.logout')}>
          <LogOut size={16} /> {t('nav.logout')}
        </button>
      </div>
    </header>
  );
}
