import { LayoutGrid, List, Plus, LogOut, Tag, Terminal, Folder } from 'lucide-react';
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
  onLogout: () => void;
}

export function Header({ viewMode, setViewMode, onOpenAdd, onOpenTags, onOpenFolders, onOpenLogs, onLogout }: HeaderProps) {
  const { t } = useTranslation();

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
        
        <button onClick={onOpenFolders} className="btn btn-secondary" style={{ display: 'flex', gap: '8px', alignItems: 'center' }}>
          <Folder size={16} /> {t('folders.title', 'Folders')}
        </button>
        <button onClick={onOpenTags} className="btn btn-secondary" style={{ display: 'flex', gap: '8px', alignItems: 'center' }}>
          <Tag size={16} /> {t('nav.tags')}
        </button>
        <button onClick={onOpenLogs} className="btn btn-secondary" style={{ display: 'flex', gap: '8px', alignItems: 'center' }}>
          <Terminal size={16} /> {t('nav.logs')}
        </button>
        <button onClick={onOpenAdd} className="btn btn-primary" style={{ display: 'flex', gap: '8px', alignItems: 'center' }}>
          <Plus size={16} /> {t('cameras.add')}
        </button>
        <button onClick={onLogout} className="btn btn-secondary" style={{ display: 'flex', gap: '8px', alignItems: 'center' }} title={t('nav.logout')}>
          <LogOut size={16} /> {t('nav.logout')}
        </button>
      </div>
    </header>
  );
}
