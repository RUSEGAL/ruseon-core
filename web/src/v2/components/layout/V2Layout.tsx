import React, { useState } from 'react';
import type { CameraInfo, ServerStats, TagConfig, FolderConfig } from '../../../types';
import { V2Sidebar } from './V2Sidebar';
import type { V2NavRoute } from './V2Sidebar';
import { V2Header } from './V2Header';
import { DashboardOverview } from '../views/DashboardOverview';
import { SurveillanceView } from '../surveillance/SurveillanceView';
import { ArchiveView } from '../archive/ArchiveView';
import { AiEventsView } from '../views/AiEventsView';
import { TopologyView } from '../views/TopologyView';
import { SettingsView } from '../views/SettingsView';

interface V2LayoutProps {
  cameras: CameraInfo[];
  serverStats: ServerStats | null;
  tags: TagConfig[];
  folders: FolderConfig[];
  userRole?: string;
  onLogout: () => void;
  onAddCamera: () => void;
  onEditCamera: (cam: CameraInfo) => void;
  onDeleteCamera: (id: string) => void;
  onOpenDetails: (cam: CameraInfo) => void;
  onOpenStats?: () => void;
  onOpenTags?: () => void;
  onOpenFolders?: () => void;
  onOpenLogs?: () => void;
  onOpenUsers?: () => void;
}

export const V2Layout: React.FC<V2LayoutProps> = ({
  cameras,
  serverStats,
  tags,
  folders,
  userRole,
  onLogout,
  onAddCamera,
  onEditCamera,
  onDeleteCamera,
  onOpenDetails,
  onOpenStats,
  onOpenTags,
  onOpenFolders,
  onOpenLogs,
  onOpenUsers,
}) => {
  const [currentRoute, setCurrentRoute] = useState<V2NavRoute>('surveillance');
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false);
  const [archiveTargetCam] = useState<string | undefined>();

  return (
    <div className="v2-app-container">
      {/* Sidebar Navigation */}
      <V2Sidebar
        currentRoute={currentRoute}
        onRouteChange={(route) => setCurrentRoute(route)}
        collapsed={sidebarCollapsed}
        onToggleCollapsed={() => setSidebarCollapsed(!sidebarCollapsed)}
      />

      {/* Main Content Area */}
      <div className="v2-main-area">
        <V2Header
          stats={serverStats}
          userRole={userRole}
          onLogout={onLogout}
          onOpenStats={onOpenStats}
          onOpenLogs={onOpenLogs}
          onOpenUsers={onOpenUsers}
        />

        <main className="v2-content">
          {currentRoute === 'dashboard' && (
            <DashboardOverview
              cameras={cameras}
              stats={serverStats}
              onNavigateToSurveillance={() => setCurrentRoute('surveillance')}
            />
          )}

          {currentRoute === 'surveillance' && (
            <SurveillanceView
              cameras={cameras}
              folders={folders}
              tags={tags}
              onOpenDetails={onOpenDetails}
            />
          )}

          {currentRoute === 'archive' && (
            <ArchiveView cameras={cameras} initialCameraId={archiveTargetCam} />
          )}

          {currentRoute === 'ai_events' && <AiEventsView cameras={cameras} />}

          {currentRoute === 'topology' && (
            <TopologyView stats={serverStats} cameras={cameras} />
          )}

          {currentRoute === 'settings' && (
            <SettingsView
              cameras={cameras}
              tags={tags}
              folders={folders}
              userRole={userRole}
              onAddCamera={onAddCamera}
              onEditCamera={onEditCamera}
              onDeleteCamera={onDeleteCamera}
              onOpenDetails={onOpenDetails}
              onOpenFolders={onOpenFolders}
              onOpenTags={onOpenTags}
              onOpenUsers={onOpenUsers}
              onOpenLogs={onOpenLogs}
            />
          )}
        </main>
      </div>
    </div>
  );
};
