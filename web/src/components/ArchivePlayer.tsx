import { useState, useEffect } from 'react';
import { VideoPlayer } from './VideoPlayer';
import { Film } from 'lucide-react';

interface ArchiveInterval {
  start: string;
  end: string;
  filename: string;
}

export function ArchivePlayer({ streamId }: { streamId: string }) {
  const [intervals, setIntervals] = useState<ArchiveInterval[]>([]);
  const [selectedFile, setSelectedFile] = useState<string>('');
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const token = localStorage.getItem('token');
    fetch(`/api/cameras/${streamId}/archive`, {
      headers: { Authorization: `Bearer ${token}` }
    })
    .then(r => r.json())
    .then(data => {
      if (Array.isArray(data)) {
        setIntervals(data);
        if (data.length > 0) {
          setSelectedFile(data[0].filename);
        }
      }
    })
    .catch(console.error)
    .finally(() => setLoading(false));
  }, [streamId]);

  if (loading) {
    return <div style={{ padding: '2rem', textAlign: 'center', color: 'var(--text-muted)' }}>Loading archive...</div>;
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '1rem', width: '100%' }}>
      {intervals.length > 0 ? (
        <>
          <div style={{ display: 'flex', gap: '1rem', alignItems: 'center' }}>
            <Film size={20} color="var(--primary)" />
            <select 
              value={selectedFile} 
              onChange={e => setSelectedFile(e.target.value)}
              style={{ flex: 1, padding: '8px 12px', borderRadius: '8px', background: 'rgba(255,255,255,0.05)', color: '#fff', border: '1px solid var(--card-border)', outline: 'none' }}
            >
              {intervals.map(i => (
                <option key={i.filename} value={i.filename} style={{ background: '#1e293b' }}>
                  {i.filename} ({new Date(i.start).toLocaleString()} - {new Date(i.end).toLocaleString()})
                </option>
              ))}
            </select>
          </div>
          
          <div style={{ borderRadius: '12px', overflow: 'hidden', border: '1px solid var(--card-border)', background: '#000', aspectRatio: '16/9', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
            <VideoPlayer key={selectedFile} streamId={streamId} sourceUrl={`/hls/${streamId}/archive.m3u8?file=${selectedFile}`} autoPlay={true} />
          </div>
        </>
      ) : (
        <div style={{ padding: '2rem', textAlign: 'center', color: 'var(--text-muted)', background: 'rgba(255,255,255,0.02)', borderRadius: '12px', border: '1px solid var(--card-border)' }}>
          No archive recordings found for this camera.
        </div>
      )}
    </div>
  );
}
