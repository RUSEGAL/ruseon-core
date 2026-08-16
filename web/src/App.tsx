import { useEffect, useState, useRef, useMemo } from 'react';
import type { CameraInfo, ServerStats, TagConfig, FolderConfig } from './types';
import { Login } from './components/Login';
import type { CamFormState } from './v2/components/modals/V2CameraFormModal';
import {
  V2Layout,
  V2CameraDetailsModal,
  V2CameraFormModal,
  V2TagManagerModal,
  V2FolderManagerModal,
  V2LogsModal,
  V2ServerStatsModal,
  V2UserManagerModal,
} from './v2';

export default function App() {
  const [token, setToken] = useState<string | null>(localStorage.getItem('token'));
  const [cameras, setCameras] = useState<CameraInfo[]>([]);

  const [showModal, setShowModal] = useState(false);
  const [showStatsModal, setShowStatsModal] = useState(false);
  const [showTagModal, setShowTagModal] = useState(false);
  const [showFolderModal, setShowFolderModal] = useState(false);
  const [showLogsModal, setShowLogsModal] = useState(false);
  const [showUserModal, setShowUserModal] = useState(false);
  const [isEditing, setIsEditing] = useState(false);
  const [camForm, setCamForm] = useState<CamFormState>({
    id: '',
    url: '',
    record: false,
    lazyHLS: false,
    tokenAuth: false,
    transport: 'tcp',
    retentionDays: 0,
    tags: [],
    folderId: '',
    comment: '',
    simPhone: '',
    simICCID: '',
    disabled: false,
    disableReason: 'technical',
  });
  const [globalTags, setGlobalTags] = useState<TagConfig[]>([]);
  const [folders, setFolders] = useState<FolderConfig[]>([]);
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
        const [camsRes, statsRes, tagsRes, foldersRes] = await Promise.all([
          fetch('/api/cameras', { headers: { Authorization: `Bearer ${token}` } }),
          fetch('/api/stats', { headers: { Authorization: `Bearer ${token}` } }),
          fetch('/api/tags', { headers: { Authorization: `Bearer ${token}` } }),
          fetch('/api/folders', { headers: { Authorization: `Bearer ${token}` } }),
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
        if (foldersRes.ok) {
          setFolders(await foldersRes.json());
        }
      } catch (e) {
        console.error('Failed to fetch telemetry data:', e);
      }
    };

    fetchData();
    const interval = setInterval(fetchData, 2000);
    return () => clearInterval(interval);
  }, [token]);

  // Sync details modal with fresh data
  useEffect(() => {
    if (detailsCam && cameras.length > 0) {
      const updated = cameras.find((c) => c.id === detailsCam.id);
      if (updated) {
        setDetailsCam(updated);
      }
    }
  }, [cameras, detailsCam]);

  const calculateStats = (cams: CameraInfo[]) => {
    const now = Date.now();
    const timeDiff = (now - lastTimeRef.current) / 1000;
    if (timeDiff <= 0) return;

    const newFps: Record<string, number> = {};
    const newBitrates: Record<string, number> = {};

    cams.forEach((cam) => {
      const lastF = lastFramesRef.current[cam.id] || 0;
      const lastB = lastBytesRef.current[cam.id] || 0;

      const frameDiff = cam.frames - lastF;
      const byteDiff = cam.bytesReceived - lastB;

      if (lastF > 0 && frameDiff >= 0) {
        newFps[cam.id] = frameDiff / timeDiff;
      }
      if (lastB > 0 && byteDiff >= 0) {
        newBitrates[cam.id] = ((byteDiff * 8) / 1000) / timeDiff;
      }

      lastFramesRef.current[cam.id] = cam.frames;
      lastBytesRef.current[cam.id] = cam.bytesReceived;
    });

    setFpsMap((prev) => ({ ...prev, ...newFps }));
    setBitrates((prev) => ({ ...prev, ...newBitrates }));
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
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify(camForm),
      });
      if (res.ok) {
        setShowModal(false);
      }
    } catch (e) {
      console.error('Failed to save camera:', e);
    }
  };

  const deleteCamera = async (id: string) => {
    if (!confirm('Are you sure you want to delete this camera?')) return;
    try {
      await fetch(`/api/cameras/${id}`, {
        method: 'DELETE',
        headers: { Authorization: `Bearer ${token}` },
      });
    } catch (e) {
      console.error('Failed to delete camera:', e);
    }
  };

  const openAddModal = () => {
    setIsEditing(false);
    setCamForm({
      id: '',
      url: '',
      record: false,
      lazyHLS: false,
      tokenAuth: false,
      transport: 'tcp',
      retentionDays: 0,
      tags: [],
      folderId: '',
      comment: '',
      simPhone: '',
      simICCID: '',
      disabled: false,
      disableReason: 'technical',
    });
    setShowModal(true);
  };

  const openEditModal = (cam: CameraInfo) => {
    setIsEditing(true);
    setCamForm({
      id: cam.id,
      url: cam.url,
      record: cam.record || false,
      lazyHLS: cam.lazyHLS || false,
      tokenAuth: cam.tokenAuth || false,
      transport: cam.transport || 'tcp',
      retentionDays: cam.retentionDays || 0,
      tags: cam.tags || [],
      folderId: cam.folderId || '',
      comment: cam.comment || '',
      simPhone: cam.simPhone || '',
      simICCID: cam.simICCID || '',
      disabled: cam.disabled || false,
      disableReason: cam.disableReason || 'technical',
    });
    setShowModal(true);
  };

  const userRole = useMemo(() => {
    if (!token) return 'viewer';
    try {
      const payload = JSON.parse(atob(token.split('.')[1]));
      return payload.role || 'viewer';
    } catch {
      return 'viewer';
    }
  }, [token]);

  if (!token) {
    return <Login onLogin={setToken} />;
  }

  return (
    <>
      <V2Layout
        cameras={cameras}
        serverStats={serverStats}
        tags={globalTags}
        folders={folders}
        userRole={userRole}
        onLogout={handleLogout}
        onAddCamera={openAddModal}
        onEditCamera={(cam) => {
          openEditModal(cam);
        }}
        onDeleteCamera={deleteCamera}
        onOpenDetails={setDetailsCam}
        onOpenStats={() => setShowStatsModal(true)}
        onOpenTags={() => setShowTagModal(true)}
        onOpenFolders={() => setShowFolderModal(true)}
        onOpenLogs={() => setShowLogsModal(true)}
        onOpenUsers={() => setShowUserModal(true)}
      />

      {/* Next-Gen Modals */}
      {showModal && (
        <V2CameraFormModal
          isEditing={isEditing}
          camForm={camForm}
          setCamForm={setCamForm}
          onClose={() => setShowModal(false)}
          onSave={saveCamera}
          globalTags={globalTags}
          folders={folders}
        />
      )}

      {showTagModal && (
        <V2TagManagerModal
          tags={globalTags}
          token={token}
          onClose={() => setShowTagModal(false)}
          onTagsChange={() => {
            fetch('/api/tags', { headers: { Authorization: `Bearer ${token}` } })
              .then((r) => r.json())
              .then(setGlobalTags);
          }}
        />
      )}

      {showFolderModal && (
        <V2FolderManagerModal
          folders={folders}
          token={token}
          onClose={() => setShowFolderModal(false)}
          onFoldersChange={() => {
            fetch('/api/folders', { headers: { Authorization: `Bearer ${token}` } })
              .then((r) => r.json())
              .then(setFolders);
          }}
        />
      )}

      {detailsCam && (
        <V2CameraDetailsModal
          detailsCam={detailsCam}
          bitrates={bitrates}
          fpsMap={fpsMap}
          onClose={() => setDetailsCam(null)}
          globalTags={globalTags}
        />
      )}

      {showStatsModal && serverStats && (
        <V2ServerStatsModal
          serverStats={serverStats}
          onClose={() => setShowStatsModal(false)}
        />
      )}

      {showLogsModal && (
        <V2LogsModal
          onClose={() => setShowLogsModal(false)}
        />
      )}

      {showUserModal && (
        <V2UserManagerModal
          token={token}
          onClose={() => setShowUserModal(false)}
        />
      )}
    </>
  );
}
