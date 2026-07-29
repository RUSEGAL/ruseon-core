import { useEffect, useState, useRef } from 'react';
import { Camera } from 'lucide-react';
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

export default function App() {
  const [token, setToken] = useState<string | null>(localStorage.getItem('token'));
  const [cameras, setCameras] = useState<CameraInfo[]>([]);
  const [loading, setLoading] = useState(true);
  
  const [showModal, setShowModal] = useState(false);
  const [showStatsModal, setShowStatsModal] = useState(false);
  const [isEditing, setIsEditing] = useState(false);
  const [camForm, setCamForm] = useState<CamFormState>({ id: '', url: '', record: false, retentionDays: 0 });

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
        const [camsRes, statsRes] = await Promise.all([
          fetch('/api/cameras', { headers: { 'Authorization': `Bearer ${token}` } }),
          fetch('/api/stats', { headers: { 'Authorization': `Bearer ${token}` } })
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
    setCamForm({ id: '', url: '', record: false, retentionDays: 0 });
    setShowModal(true);
  };

  const openEditModal = (cam: CameraInfo) => {
    setIsEditing(true);
    setCamForm({ id: cam.id, url: cam.url, record: cam.record || false, retentionDays: cam.retentionDays || 0 });
    setShowModal(true);
  };

  if (!token) {
    return <Login onLogin={setToken} />;
  }

  return (
    <>
      <Header 
        viewMode={viewMode} 
        setViewMode={setViewMode} 
        onOpenAdd={openAddModal} 
        onLogout={handleLogout} 
      />

      <main className="main-content">
        {serverStats && (
          <DashboardStats 
            serverStats={serverStats} 
            onOpenAdvancedStats={() => setShowStatsModal(true)} 
          />
        )}

        <h2 className="section-title">
          <Camera size={24} style={{ color: 'var(--primary)' }} />
          Live Cameras
        </h2>

        {loading ? (
          <div className="loader-container">
            <div className="loader"></div>
          </div>
        ) : viewMode === 'grid' ? (
          <CameraGrid 
            cameras={cameras} 
            onEdit={openEditModal} 
            onDelete={deleteCamera} 
            onOpenDetails={setDetailsCam} 
          />
        ) : (
          <CameraList 
            cameras={cameras} 
            bitrates={bitrates} 
            fpsMap={fpsMap}
            onEdit={openEditModal} 
            onDelete={deleteCamera} 
            onOpenDetails={setDetailsCam} 
          />
        )}

        {showModal && (
          <CameraFormModal 
            isEditing={isEditing}
            camForm={camForm}
            setCamForm={setCamForm}
            onSave={saveCamera}
            onClose={() => setShowModal(false)}
          />
        )}

        {detailsCam && (
          <CameraDetailsModal 
            detailsCam={detailsCam} 
            bitrates={bitrates} 
            fpsMap={fpsMap} 
            onClose={() => setDetailsCam(null)} 
          />
        )}

        {showStatsModal && serverStats && (
          <ServerStatsModal 
            serverStats={serverStats} 
            onClose={() => setShowStatsModal(false)} 
          />
        )}
      </main>
    </>
  );
}
