import React, { useState } from 'react';
import { useTranslation } from 'react-i18next';
import type { CameraInfo, FolderConfig, TagConfig } from '../../../types';
import { SurveillanceGrid } from './SurveillanceGrid';
import { useSurveillanceLayout } from '../../hooks/useSurveillanceLayout';
import type { GridLayoutPreset } from '../../hooks/useSurveillanceLayout';
import {
  Grid2X2,
  Grid3X3,
  Square,
  LayoutGrid,
  Search,
  Maximize2,
  SlidersHorizontal,
  Folder,
} from 'lucide-react';

interface SurveillanceViewProps {
  cameras: CameraInfo[];
  folders?: FolderConfig[];
  tags?: TagConfig[];
  onOpenDetails?: (camera: CameraInfo) => void;
}

export const SurveillanceView: React.FC<SurveillanceViewProps> = ({
  cameras,
  folders = [],
  onOpenDetails,
}) => {
  const { t } = useTranslation();
  const [search, setSearch] = useState('');
  const [filterStatus, setFilterStatus] = useState<'all' | 'online' | 'recording' | 'high_traffic'>('all');
  const [selectedFolder, setSelectedFolder] = useState<string>('all');
  const [sortBy, setSortBy] = useState<'id' | 'status' | 'uptime' | 'traffic'>('id');

  // Filter cameras
  let filteredCameras = cameras.filter((c) => {
    const matchesSearch = c.id.toLowerCase().includes(search.toLowerCase());
    if (!matchesSearch) return false;

    if (selectedFolder !== 'all') {
      const fId = c.folderId || 'unassigned';
      if (fId !== selectedFolder) return false;
    }

    if (filterStatus === 'online') return c.state === 'online' && !c.disabled;
    if (filterStatus === 'recording') return !!c.record;
    if (filterStatus === 'high_traffic') {
      return (c.trafficUsed || 0) > 1024 * 1024 * 1024; // > 1GB
    }
    return true;
  });

  // Sort cameras
  filteredCameras = [...filteredCameras].sort((a, b) => {
    if (sortBy === 'id') return a.id.localeCompare(b.id);
    if (sortBy === 'status') {
      const aOn = a.state === 'online' && !a.disabled ? 1 : 0;
      const bOn = b.state === 'online' && !b.disabled ? 1 : 0;
      return bOn - aOn;
    }
    if (sortBy === 'uptime') return (b.uptime || 0) - (a.uptime || 0);
    if (sortBy === 'traffic') return (b.trafficUsed || 0) - (a.trafficUsed || 0);
    return 0;
  });

  const availableCameraIds = filteredCameras.map((c) => c.id);
  const { layout, setLayout, orderedCameraIds, reorderCameras } = useSurveillanceLayout(availableCameraIds);

  const onlineCount = cameras.filter((c) => c.state === 'online' && !c.disabled).length;
  const recordingCount = cameras.filter((c) => c.record).length;

  const toggleContainerFullscreen = () => {
    const el = document.getElementById('v2-surveillance-container');
    if (!el) return;
    if (!document.fullscreenElement) {
      el.requestFullscreen();
    } else {
      document.exitFullscreen();
    }
  };

  return (
    <div
      id="v2-surveillance-container"
      style={{ display: 'flex', flexDirection: 'column', height: '100%', gap: '10px' }}
    >
      {/* Top Surveillance Toolbar */}
      <div className="v2-grid-toolbar">
        {/* Left: Search, Folder Picker & Filter Pills */}
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px', flexWrap: 'wrap' }}>
          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              background: 'rgba(0,0,0,0.4)',
              border: '1px solid rgba(255,255,255,0.1)',
              borderRadius: '8px',
              padding: '4px 10px',
              gap: '6px',
            }}
          >
            <Search size={14} color="#94a3b8" />
            <input
              type="text"
              placeholder={t('cameras.search', 'Search cameras...')}
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              style={{
                background: 'transparent',
                border: 'none',
                color: '#f8fafc',
                fontSize: '0.8rem',
                outline: 'none',
                width: '130px',
              }}
            />
          </div>

          {/* Folder Filter */}
          {folders.length > 0 && (
            <div
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: '4px',
                background: 'rgba(0,0,0,0.4)',
                border: '1px solid rgba(255,255,255,0.1)',
                borderRadius: '8px',
                padding: '3px 8px',
              }}
            >
              <Folder size={13} color="#a5b4fc" />
              <select
                value={selectedFolder}
                onChange={(e) => setSelectedFolder(e.target.value)}
                style={{
                  background: 'transparent',
                  border: 'none',
                  color: '#f8fafc',
                  fontSize: '0.76rem',
                  outline: 'none',
                  cursor: 'pointer',
                }}
              >
                <option value="all" style={{ background: '#0d111a' }}>{t('folders.title', 'Folders')}: {t('filters.all', 'All')}</option>
                <option value="unassigned" style={{ background: '#0d111a' }}>{t('folders.unassigned', 'Unassigned')}</option>
                {folders.map((f) => (
                  <option key={f.id} value={f.id} style={{ background: '#0d111a' }}>
                    {f.name}
                  </option>
                ))}
              </select>
            </div>
          )}

          {/* Status Pills */}
          <div style={{ display: 'flex', gap: '4px' }}>
            <button
              onClick={() => setFilterStatus('all')}
              style={{
                background: filterStatus === 'all' ? 'rgba(99,102,241,0.2)' : 'transparent',
                border: '1px solid',
                borderColor: filterStatus === 'all' ? '#6366f1' : 'rgba(255,255,255,0.1)',
                color: filterStatus === 'all' ? '#a5b4fc' : '#94a3b8',
                borderRadius: '6px',
                padding: '4px 8px',
                fontSize: '0.74rem',
                cursor: 'pointer',
                fontWeight: 600,
              }}
            >
              {t('filters.all', 'All')} ({cameras.length})
            </button>
            <button
              onClick={() => setFilterStatus('online')}
              style={{
                background: filterStatus === 'online' ? 'rgba(16,185,129,0.2)' : 'transparent',
                border: '1px solid',
                borderColor: filterStatus === 'online' ? '#10b981' : 'rgba(255,255,255,0.1)',
                color: filterStatus === 'online' ? '#6ee7b7' : '#94a3b8',
                borderRadius: '6px',
                padding: '4px 8px',
                fontSize: '0.74rem',
                cursor: 'pointer',
                fontWeight: 600,
              }}
            >
              {t('filters.online', 'Live')} ({onlineCount})
            </button>
            <button
              onClick={() => setFilterStatus('recording')}
              style={{
                background: filterStatus === 'recording' ? 'rgba(239,68,68,0.2)' : 'transparent',
                border: '1px solid',
                borderColor: filterStatus === 'recording' ? '#ef4444' : 'rgba(255,255,255,0.1)',
                color: filterStatus === 'recording' ? '#fca5a5' : '#94a3b8',
                borderRadius: '6px',
                padding: '4px 8px',
                fontSize: '0.74rem',
                cursor: 'pointer',
                fontWeight: 600,
              }}
            >
              {t('filters.recording', 'REC')} ({recordingCount})
            </button>
            <button
              onClick={() => setFilterStatus('high_traffic')}
              style={{
                background: filterStatus === 'high_traffic' ? 'rgba(56,189,248,0.2)' : 'transparent',
                border: '1px solid',
                borderColor: filterStatus === 'high_traffic' ? '#38bdf8' : 'rgba(255,255,255,0.1)',
                color: filterStatus === 'high_traffic' ? '#7dd3fc' : '#94a3b8',
                borderRadius: '6px',
                padding: '4px 8px',
                fontSize: '0.74rem',
                cursor: 'pointer',
                fontWeight: 600,
              }}
            >
              {t('filters.highUsage', 'High Traffic')}
            </button>
          </div>
        </div>

        {/* Right: Sort By, Layout Presets, Fullscreen */}
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px', flexWrap: 'wrap' }}>
          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: '4px',
              background: 'rgba(0,0,0,0.4)',
              border: '1px solid rgba(255,255,255,0.1)',
              borderRadius: '8px',
              padding: '3px 8px',
            }}
          >
            <SlidersHorizontal size={13} color="#94a3b8" />
            <select
              value={sortBy}
              onChange={(e) => setSortBy(e.target.value as any)}
              style={{
                background: 'transparent',
                border: 'none',
                color: '#f8fafc',
                fontSize: '0.76rem',
                outline: 'none',
                cursor: 'pointer',
              }}
            >
              <option value="id" style={{ background: '#0d111a' }}>{t('filters.sortId', 'Sort by ID')}</option>
              <option value="status" style={{ background: '#0d111a' }}>{t('filters.sortStatus', 'Sort by Status')}</option>
              <option value="uptime" style={{ background: '#0d111a' }}>{t('filters.sortUptime', 'Sort by Uptime')}</option>
              <option value="traffic" style={{ background: '#0d111a' }}>{t('filters.sortTraffic', 'Sort by Traffic')}</option>
            </select>
          </div>

          <div
            style={{
              display: 'flex',
              background: 'rgba(0,0,0,0.4)',
              borderRadius: '8px',
              padding: '2px',
              border: '1px solid rgba(255,255,255,0.1)',
            }}
          >
            {(['1x1', '2x2', '3x3', '1+5', 'auto'] as GridLayoutPreset[]).map((preset) => (
              <button
                key={preset}
                onClick={() => setLayout(preset)}
                style={{
                  background: layout === preset ? 'rgba(99,102,241,0.4)' : 'transparent',
                  border: 'none',
                  borderRadius: '6px',
                  padding: '5px 8px',
                  color: layout === preset ? '#a5b4fc' : '#94a3b8',
                  fontSize: '0.72rem',
                  fontWeight: 600,
                  cursor: 'pointer',
                  display: 'flex',
                  alignItems: 'center',
                  gap: '4px',
                }}
                title={`Layout ${preset}`}
              >
                {preset === '1x1' && <Square size={13} />}
                {preset === '2x2' && <Grid2X2 size={13} />}
                {preset === '3x3' && <Grid3X3 size={13} />}
                {preset === '1+5' && <LayoutGrid size={13} />}
                <span>{preset === 'auto' ? t('v2.surveillance.gridAuto', 'Auto') : preset}</span>
              </button>
            ))}
          </div>

          <button
            onClick={toggleContainerFullscreen}
            style={{
              background: 'rgba(0,0,0,0.4)',
              border: '1px solid rgba(255,255,255,0.1)',
              borderRadius: '8px',
              padding: '6px 8px',
              color: '#94a3b8',
              cursor: 'pointer',
              display: 'flex',
              alignItems: 'center',
            }}
            title={t('v2.surveillance.fullscreen', 'Fullscreen')}
          >
            <Maximize2 size={14} />
          </button>
        </div>
      </div>

      {/* Grid Content */}
      <div style={{ flex: 1, minHeight: 0 }}>
        {filteredCameras.length === 0 ? (
          <div
            style={{
              height: '100%',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              color: '#64748b',
              fontSize: '0.9rem',
            }}
          >
            No cameras match current filter.
          </div>
        ) : (
          <SurveillanceGrid
            cameras={filteredCameras}
            orderedCameraIds={orderedCameraIds}
            layout={layout}
            onOpenDetails={onOpenDetails}
            onReorder={reorderCameras}
          />
        )}
      </div>
    </div>
  );
};
