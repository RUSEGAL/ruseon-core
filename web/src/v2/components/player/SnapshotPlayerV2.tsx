import React, { useEffect, useState } from 'react';
import { ImageOff, Loader2 } from 'lucide-react';

interface SnapshotPlayerV2Props {
  streamId: string;
  intervalMs?: number;
  onError?: (err: string) => void;
}

export const SnapshotPlayerV2: React.FC<SnapshotPlayerV2Props> = ({
  streamId,
  intervalMs = 3000,
  onError,
}) => {
  const [imgUrl, setImgUrl] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let isCancelled = false;

    const fetchSnapshot = async () => {
      try {
        const token = localStorage.getItem('token');
        // Snapshot endpoint or fallback
        const url = `/api/cameras/${streamId}/snapshot?t=${Date.now()}`;
        const res = await fetch(url, {
          headers: token ? { Authorization: `Bearer ${token}` } : {},
        });

        if (!res.ok) {
          throw new Error(`Snapshot error HTTP ${res.status}`);
        }

        const blob = await res.blob();
        if (!isCancelled) {
          const blobUrl = URL.createObjectURL(blob);
          setImgUrl((prev) => {
            if (prev) URL.revokeObjectURL(prev);
            return blobUrl;
          });
          setLoading(false);
          setError(null);
        }
      } catch (err: unknown) {
        if (!isCancelled) {
          const msg = err instanceof Error ? err.message : 'Snapshot fetch failed';
          setError(msg);
          setLoading(false);
          if (onError) onError(msg);
        }
      }
    };

    fetchSnapshot();
    const interval = setInterval(fetchSnapshot, intervalMs);

    return () => {
      isCancelled = true;
      clearInterval(interval);
      if (imgUrl) {
        URL.revokeObjectURL(imgUrl);
      }
    };
  }, [streamId, intervalMs]);

  return (
    <div style={{ width: '100%', height: '100%', position: 'relative', background: '#000', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
      {imgUrl && !error && (
        <img
          src={imgUrl}
          alt={`Camera ${streamId}`}
          style={{ width: '100%', height: '100%', objectFit: 'contain' }}
        />
      )}

      {loading && !imgUrl && (
        <div style={{ display: 'flex', gap: '8px', color: '#94a3b8', fontSize: '0.82rem' }}>
          <Loader2 className="animate-spin" size={18} />
          <span>Loading snapshot...</span>
        </div>
      )}

      {error && (
        <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: '6px', color: '#64748b', fontSize: '0.8rem' }}>
          <ImageOff size={22} />
          <span>Snapshot mode standby</span>
        </div>
      )}
    </div>
  );
};
