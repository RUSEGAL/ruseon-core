import { useState, useEffect, useMemo } from 'react';
import { VideoPlayer } from './VideoPlayer';
import { Calendar } from 'lucide-react';

interface ArchiveInterval {
  start: string;
  end: string;
  filename: string;
}

export function ArchivePlayer({ streamId }: { streamId: string }) {
  const [intervals, setIntervals] = useState<ArchiveInterval[]>([]);
  const [selectedFile, setSelectedFile] = useState<string>('');
  const [loading, setLoading] = useState(true);
  const [activeDate, setActiveDate] = useState<string>('');

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
          setActiveDate(getDateKey(new Date(data[0].start)));
        }
      }
    })
    .catch(console.error)
    .finally(() => setLoading(false));
  }, [streamId]);

  const getDateKey = (d: Date) => {
    return `${d.getFullYear()}-${String(d.getMonth()+1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
  };

  const groupedIntervals = useMemo(() => {
    const groups: Record<string, ArchiveInterval[]> = {};
    intervals.forEach(i => {
      const dateKey = getDateKey(new Date(i.start));
      if (!groups[dateKey]) groups[dateKey] = [];
      groups[dateKey].push(i);
    });
    // Сортируем дни по убыванию (новые сверху)
    const sortedKeys = Object.keys(groups).sort().reverse();
    const sortedGroups: Record<string, ArchiveInterval[]> = {};
    sortedKeys.forEach(k => sortedGroups[k] = groups[k]);
    return sortedGroups;
  }, [intervals]);

  const availableDates = Object.keys(groupedIntervals);
  const activeIntervals = groupedIntervals[activeDate] || [];

  const MS_IN_DAY = 24 * 60 * 60 * 1000;
  const getDayStart = (dateStr: string) => {
    return new Date(`${dateStr}T00:00:00`).getTime();
  };

  if (loading) {
    return <div style={{ padding: '2rem', textAlign: 'center', color: 'var(--text-muted)' }}>Loading archive...</div>;
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '1rem', width: '100%' }}>
      {intervals.length > 0 ? (
        <>
          <div style={{ borderRadius: '12px', overflow: 'hidden', border: '1px solid var(--card-border)', background: '#000', aspectRatio: '16/9', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
            <VideoPlayer key={selectedFile} streamId={streamId} sourceUrl={`/hls/${streamId}/archive.m3u8?file=${selectedFile}`} autoPlay={true} />
          </div>
          
          <div className="glass" style={{ padding: '1rem', borderRadius: '12px', display: 'flex', flexDirection: 'column', gap: '1rem' }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
              <div style={{ display: 'flex', gap: '1rem', alignItems: 'center' }}>
                <Calendar size={18} color="var(--primary)" />
                <select 
                  value={activeDate} 
                  onChange={e => setActiveDate(e.target.value)}
                  style={{ padding: '6px 10px', borderRadius: '6px', background: 'rgba(255,255,255,0.05)', color: '#fff', border: '1px solid var(--card-border)', outline: 'none' }}
                >
                  {availableDates.map(d => (
                    <option key={d} value={d} style={{ background: '#1e293b' }}>
                      {new Date(d).toLocaleDateString()}
                    </option>
                  ))}
                </select>
              </div>
              <div style={{ fontSize: '0.85rem', color: 'var(--text-muted)' }}>
                {activeIntervals.length} recordings this day
              </div>
            </div>

            {/* Visual Timeline */}
            <div style={{ position: 'relative', height: '60px', background: 'rgba(0,0,0,0.3)', borderRadius: '8px', border: '1px solid var(--card-border)', overflow: 'hidden' }}>
              {/* Hour markers */}
              {[0, 6, 12, 18, 24].map(hour => (
                <div key={hour} style={{ position: 'absolute', left: `${(hour/24)*100}%`, top: 0, bottom: 0, borderLeft: '1px dashed rgba(255,255,255,0.1)' }}>
                  <span style={{ position: 'absolute', top: '4px', left: '4px', fontSize: '0.7rem', color: 'var(--text-muted)' }}>{hour}:00</span>
                </div>
              ))}

              {/* Blocks */}
              {activeIntervals.map(interval => {
                const dayStart = getDayStart(activeDate);
                const startMs = new Date(interval.start).getTime();
                const endMs = new Date(interval.end).getTime();
                
                let relativeStart = startMs - dayStart;
                let relativeEnd = endMs - dayStart;
                
                if (relativeStart < 0) relativeStart = 0;
                if (relativeEnd > MS_IN_DAY) relativeEnd = MS_IN_DAY;
                
                const leftPercent = (relativeStart / MS_IN_DAY) * 100;
                const widthPercent = ((relativeEnd - relativeStart) / MS_IN_DAY) * 100;

                const isSelected = selectedFile === interval.filename;

                return (
                  <div
                    key={interval.filename}
                    onClick={() => setSelectedFile(interval.filename)}
                    title={`${new Date(interval.start).toLocaleTimeString()} - ${new Date(interval.end).toLocaleTimeString()}`}
                    style={{
                      position: 'absolute',
                      left: `${leftPercent}%`,
                      width: `${Math.max(widthPercent, 0.2)}%`,
                      top: '20px',
                      bottom: '4px',
                      background: isSelected ? 'var(--primary)' : (interval.filename.includes('ongoing') ? 'var(--danger)' : 'rgba(59, 130, 246, 0.6)'),
                      borderRadius: '4px',
                      cursor: 'pointer',
                      border: isSelected ? '2px solid #fff' : 'none',
                      transition: 'all 0.2s',
                      opacity: isSelected ? 1 : 0.8
                    }}
                    onMouseEnter={(e) => e.currentTarget.style.opacity = '1'}
                    onMouseLeave={(e) => !isSelected && (e.currentTarget.style.opacity = '0.8')}
                  />
                );
              })}
            </div>
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
