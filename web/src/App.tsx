import { useEffect, useState, useRef, useMemo } from 'react';
import { Camera, Search, SlidersHorizontal } from 'lucide-react';
import type { CameraInfo, ServerStats } from './types';
import { Login } from './components/Login';
import { Header } from './components/Header';
import { DashboardStats } from './components/DashboardStats';
import { CameraGrid } from './components/CameraGrid';
import { CameraList } from './components/CameraList';
import { CameraFormModal } from './components/modals/CameraFormModal';
import type { CamFormState } from './components/modals/CameraFormModal';
import { CameraDetailsModal } from './components/modals/CameraDetailsModal';
import { ServerStatsModal } from './components/modals/ServerStatsModal';
import { TagManagerModal } from './components/modals/TagManagerModal';
import { LogsModal } from './components/modals/LogsModal';
import type { TagConfig } from './types';
import { useTranslation } from 'react-i18next';

export default function App() {
  const { t } = useTranslation();
  const [token, setToken] = useState<string | null>(localStorage.getItem('token'));
  const [cameras, setCameras] = useState<CameraInfo[]>([]);
  const [loading, setLoading] = useState(true);
  
  const [showModal, setShowModal] = useState(false);
  const [showStatsModal, setShowStatsModal] = useState(false);
  const [showTagModal, setShowTagModal] = useState(false);
  const [showLogsModal, setShowLogsModal] = useState(false);
  const [isEditing, setIsEditing] = useState(false);
  const [camForm, setCamForm] = useState<CamFormState>({ id: '', url: '', record: false, lazyHLS: false, transport: 'tcp', retentionDays: 0, tags: [], comment: '', simPhone: '', simICCID: '', disabled: false, disableReason: 'technical' });
  const [globalTags, setGlobalTags] = useState<TagConfig[]>([]);

  const [searchQuery, setSearchQuery] = useState('');
  const [sortBy, setSortBy] = useState<'id' | 'uptime' | 'traffic' | 'status'>('id');

  const [filterStatus, setFilterStatus] = useState<'all' | 'online' | 'offline'>('all');
  const [filterRecord, setFilterRecord] = useState<'all' | 'recording' | 'not_recording'>('all');
  const [filterTraffic, setFilterTraffic] = useState<'all' | 'high'>('all');

  const [viewMode, setViewMode] = useState<'grid' | 'list'>('list');
  const [detailsCam, setDetailsCam] = useState<CameraInfo | null>(null);

  const [serverStats, setServerStats] = useState<ServerStats | null>(null);
  
  const [fpsMap, setFpsMap] = useState<Record<string, number>>({});
  const [bitrates, setBitrates] = useState<Record<string, number>>({});
  
  const lastFramesRef = useRef<Record<string, number>>({});
  const lastBytesRef = useRef<Record<string, number>>({});
  const lastTimeRef = useRef<number>(Date.now());

  useEffect(() => {
    if (!token) return;
    
    const fetchData = async () => {
      try {
        const [camsRes, statsRes, tagsRes] = await Promise.all([
          fetch('/api/cameras', { headers: { 'Authorization': `Bearer ${token}` } }),
          fetch('/api/stats', { headers: { 'Authorization': `Bearer ${token}` } }),
          fetch('/api/tags', { headers: { 'Authorization': `Bearer ${token}` } })
        ]);
        
        if (camsRes.status === 401 || statsRes.status === 401) {
          handleLogout();
          return;
        }

        if (camsRes.ok) {
          const data = await camsRes.json();
          setCameras(data);
          calculateStats(data);
        }
        if (statsRes.ok) {
          setServerStats(await statsRes.json());
        }
        if (tagsRes.ok) {
          setGlobalTags(await tagsRes.json());
        }
      } catch (e) {
        console.error(e);
      } finally {
        setLoading(false);
      }
    };

    fetchData();
    const interval = setInterval(fetchData, 2000);
    return () => clearInterval(interval);
  }, [token]);

  // Sync details modal with fresh data
  useEffect(() => {
    if (detailsCam && cameras.length > 0) {
      const updated = cameras.find(c => c.id === detailsCam.id);
      if (updated) {
        setDetailsCam(updated);
      }
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [cameras]);

  const calculateStats = (cams: CameraInfo[]) => {
    const now = Date.now();
    const timeDiff = (now - lastTimeRef.current) / 1000;
    if (timeDiff <= 0) return;

    const newFps: Record<string, number> = {};
    const newBitrates: Record<string, number> = {};

    cams.forEach(cam => {
      const lastF = lastFramesRef.current[cam.id] || 0;
      const lastB = lastBytesRef.current[cam.id] || 0;
      
      const frameDiff = cam.frames - lastF;
      const byteDiff = cam.bytesReceived - lastB;

      if (lastF > 0 && frameDiff >= 0) {
        newFps[cam.id] = frameDiff / timeDiff;
      }
      if (lastB > 0 && byteDiff >= 0) {
        newBitrates[cam.id] = (byteDiff * 8 / 1000) / timeDiff;
      }

      lastFramesRef.current[cam.id] = cam.frames;
      lastBytesRef.current[cam.id] = cam.bytesReceived;
    });

    setFpsMap(prev => ({...prev, ...newFps}));
    setBitrates(prev => ({...prev, ...newBitrates}));
    lastTimeRef.current = now;
  };

  const handleLogout = () => {
    localStorage.removeItem('token');
    setToken(null);
  };

  const saveCamera = async (e: React.FormEvent) => {
    e.preventDefault();
    const method = isEditing ? 'PUT' : 'POST';
    const url = isEditing ? `/api/cameras/${camForm.id}` : '/api/cameras';
    
    try {
      const res = await fetch(url, {
        method,
        headers: { 
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${token}`
        },
        body: JSON.stringify(camForm)
      });
      if (res.ok) {
        setShowModal(false);
        // data will be updated by polling
      }
    } catch (e) {
      console.error(e);
    }
  };

  const deleteCamera = async (id: string) => {
    if (!confirm('Are you sure you want to delete this camera?')) return;
    try {
      await fetch(`/api/cameras/${id}`, {
        method: 'DELETE',
        headers: { 'Authorization': `Bearer ${token}` }
      });
    } catch (e) {
      console.error(e);
    }
  };

  const openAddModal = () => {
    setIsEditing(false);
    setCamForm({ id: '', url: '', record: false, lazyHLS: false, transport: 'tcp', retentionDays: 0, tags: [], comment: '', simPhone: '', simICCID: '', disabled: false, disableReason: 'technical' });
    setShowModal(true);
  };

  const openEditModal = (cam: CameraInfo) => {
    setIsEditing(true);
    setCamForm({ 
      id: cam.id, 
      url: cam.url, 
      record: cam.record || false, 
      lazyHLS: cam.lazyHLS || false,
      transport: cam.transport || 'tcp',
      retentionDays: cam.retentionDays || 0,
      tags: cam.tags || [],
      comment: cam.comment || '',
      simPhone: cam.simPhone || '',
      simICCID: cam.simICCID || '',
      disabled: cam.disabled || false,
      disableReason: cam.disableReason || 'technical'
    });
    setShowModal(true);
  };

  const filteredCameras = useMemo(() => {
    let result = cameras.filter(c => {
      if (filterStatus === 'online' && !c.connected) return false;
      if (filterStatus === 'offline' && c.connected) return false;

      if (filterRecord === 'recording' && !c.record) return false;
      if (filterRecord === 'not_recording' && c.record) return false;

      if (filterTraffic === 'high') {
        const limit = c.trafficLimit || 200*1024*1024*1024;
        const used = c.trafficUsed || 0;
        if (used / limit <= 0.7) return false;
      }

      if (!searchQuery) return true;
      const q = searchQuery.toLowerCase();
      if (c.id.toLowerCase().includes(q)) return true;
      if (c.simPhone?.toLowerCase().includes(q)) return true;
      if (c.simICCID?.toLowerCase().includes(q)) return true;
      if (c.tags && c.tags.length > 0) {
        for (const tId of c.tags) {
          const t = globalTags.find(gt => gt.id === tId);
          if (t && t.name.toLowerCase().includes(q)) return true;
        }
      }
      return false;
    });

    result.sort((a, b) => {
      if (sortBy === 'id') return a.id.localeCompare(b.id);
      if (sortBy === 'uptime') return b.uptime - a.uptime;
      if (sortBy === 'traffic') return (b.trafficUsed || 0) - (a.trafficUsed || 0);
      if (sortBy === 'status') return (b.connected ? 1 : 0) - (a.connected ? 1 : 0);
      return 0;
    });

    return result;
  }, [cameras, searchQuery, sortBy, globalTags, filterStatus, filterRecord, filterTraffic]);

  if (!token) {
    return <Login onLogin={setToken} />;
  }

  return (
    <>
      <Header 
        viewMode={viewMode} 
        setViewMode={setViewMode} 
        onOpenAdd={openAddModal} 
        onOpenTags={() => setShowTagModal(true)}
        onOpenLogs={() => setShowLogsModal(true)}
        onLogout={handleLogout} 
      />

      <main className="main-content">
        {serverStats && (
          <DashboardStats 
            serverStats={serverStats} 
            onOpenAdvancedStats={() => setShowStatsModal(true)} 
          />
        )}

        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: '16px', marginBottom: '1.5rem' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '20px', flexWrap: 'wrap' }}>
            <h2 className="section-title" style={{ margin: 0 }}>
              <Camera size={24} style={{ color: 'var(--primary)' }} />
              {t('nav.cameras')}
            </h2>
            
            <div style={{ display: 'flex', gap: '8px', flexWrap: 'wrap' }}>
              <div className="view-toggle">
                <button className={`view-btn ${filterStatus === 'all' ? 'active' : ''}`} onClick={() => setFilterStatus('all')}>All</button>
                <button className={`view-btn ${filterStatus === 'online' ? 'active' : ''}`} onClick={() => setFilterStatus('online')}>Online</button>
                <button className={`view-btn ${filterStatus === 'offline' ? 'active' : ''}`} onClick={() => setFilterStatus('offline')}>Offline</button>
              </div>
              
              <div className="view-toggle">
                <button className={`view-btn ${filterRecord === 'all' ? 'active' : ''}`} onClick={() => setFilterRecord('all')}>All</button>
                <button className={`view-btn ${filterRecord === 'recording' ? 'active' : ''}`} onClick={() => setFilterRecord('recording')}>REC</button>
                <button className={`view-btn ${filterRecord === 'not_recording' ? 'active' : ''}`} onClick={() => setFilterRecord('not_recording')}>No REC</button>
              </div>

              <div className="view-toggle">
                <button className={`view-btn ${filterTraffic === 'all' ? 'active' : ''}`} onClick={() => setFilterTraffic('all')}>All Data</button>
                <button className={`view-btn ${filterTraffic === 'high' ? 'active' : ''}`} onClick={() => setFilterTraffic('high')}>&gt;70% Used</button>
              </div>
            </div>
          </div>

          <div style={{ display: 'flex', gap: '12px', alignItems: 'center', flexWrap: 'wrap' }}>
            <div style={{ position: 'relative', width: '250px' }}>
              <Search size={16} style={{ position: 'absolute', left: '12px', top: '50%', transform: 'translateY(-50%)', color: 'var(--text-muted)' }} />
              <input 
                type="text" 
                className="input-field" 
                style={{ width: '100%', paddingLeft: '36px', height: '40px' }} 
                placeholder={t('cameras.search')} 
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
              />
            </div>
            
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px', background: 'rgba(0,0,0,0.3)', padding: '4px 12px', borderRadius: '8px', border: '1px solid var(--card-border)', height: '40px' }}>
              <SlidersHorizontal size={16} color="var(--text-muted)" />
              <span style={{ fontSize: '0.85rem', color: 'var(--text-muted)' }}>Sort by:</span>
              <select 
                value={sortBy} 
                onChange={(e) => setSortBy(e.target.value as any)}
                style={{ background: 'transparent', color: 'white', border: 'none', outline: 'none', fontSize: '0.9rem', cursor: 'pointer' }}
              >
                <option value="id" style={{ background: '#1e1e2d' }}>ID (A-Z)</option>
                <option value="status" style={{ background: '#1e1e2d' }}>Status (Online)</option>
                <option value="uptime" style={{ background: '#1e1e2d' }}>Uptime (Longest)</option>
                <option value="traffic" style={{ background: '#1e1e2d' }}>Traffic (Highest)</option>
              </select>
            </div>
          </div>
        </div>



        {loading ? (
          <div className="loader-container">
            <div className="loader"></div>
          </div>
        ) : viewMode === 'grid' ? (
          <CameraGrid 
            cameras={filteredCameras} 
            onEdit={openEditModal} 
            onDelete={deleteCamera} 
            onOpenDetails={setDetailsCam} 
            globalTags={globalTags}
          />
        ) : (
          <CameraList 
            cameras={filteredCameras} 
            bitrates={bitrates} 
            fpsMap={fpsMap}
            onEdit={openEditModal} 
            onDelete={deleteCamera} 
            onOpenDetails={setDetailsCam} 
            globalTags={globalTags}
          />
        )}

        {showModal && (
          <CameraFormModal 
            isEditing={isEditing}
            camForm={camForm}
            setCamForm={setCamForm}
            onSave={saveCamera}
            onClose={() => setShowModal(false)}
            globalTags={globalTags}
          />
        )}

        {showTagModal && (
          <TagManagerModal 
            tags={globalTags}
            token={token}
            onClose={() => setShowTagModal(false)}
            onTagsChange={() => {
              fetch('/api/tags', { headers: { 'Authorization': `Bearer ${token}` } })
                .then(r => r.json())
                .then(setGlobalTags);
            }}
          />
        )}

        {detailsCam && (
          <CameraDetailsModal 
            detailsCam={detailsCam} 
            bitrates={bitrates} 
            fpsMap={fpsMap} 
            onClose={() => setDetailsCam(null)} 
            globalTags={globalTags}
          />
        )}

        {showStatsModal && serverStats && (
          <ServerStatsModal 
            serverStats={serverStats} 
            onClose={() => setShowStatsModal(false)} 
          />
        )}

        {showLogsModal && (
          <LogsModal onClose={() => setShowLogsModal(false)} />
        )}
      </main>
    </>
  );
}
