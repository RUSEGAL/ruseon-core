import { useState, useEffect, useMemo } from 'react';
import { VideoPlayer } from './VideoPlayer';
import { Calendar, Download } from 'lucide-react';

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
  
  // Экспорт
  const [exportStart, setExportStart] = useState<number>(0);
  const [exportEnd, setExportEnd] = useState<number>(0);
  const [playerTime, setPlayerTime] = useState<number>(0);
  const [timelineZoom, setTimelineZoom] = useState<number>(1);

  const formatTime = (secs: number) => {
    const m = Math.floor(secs / 60);
    const s = Math.floor(secs % 60);
    return `${m.toString().padStart(2, '0')}:${s.toString().padStart(2, '0')}`;
  };

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
          
          const dur = (new Date(data[0].end).getTime() - new Date(data[0].start).getTime()) / 1000;
          setExportStart(0);
          setExportEnd(Math.max(1, Math.floor(dur)));
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

  const selectedInterval = intervals.find(i => i.filename === selectedFile);
  const selectedDurSec = selectedInterval ? Math.max(1, Math.floor((new Date(selectedInterval.end).getTime() - new Date(selectedInterval.start).getTime()) / 1000)) : 1;

  const handleSelectFile = (interval: ArchiveInterval) => {
    setSelectedFile(interval.filename);
    const dur = Math.max(1, Math.floor((new Date(interval.end).getTime() - new Date(interval.start).getTime()) / 1000));
    setExportStart(0);
    setExportEnd(dur);
  };

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '1rem', width: '100%' }}>
      {intervals.length > 0 ? (
        <>
          <div style={{ borderRadius: '12px', overflow: 'hidden', border: '1px solid var(--card-border)', background: '#000', aspectRatio: '16/9', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
            <VideoPlayer key={selectedFile} streamId={streamId} sourceUrl={`/hls/${streamId}/archive.m3u8?file=${selectedFile}`} autoPlay={true} onTimeUpdate={setPlayerTime} />
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
              <div style={{ display: 'flex', gap: '1rem', alignItems: 'center' }}>
                <div style={{ fontSize: '0.85rem', color: 'var(--text-muted)' }}>
                  {activeIntervals.length} recordings this day
                </div>
                {selectedFile && (
                  <button 
                    onClick={() => {
                      const t = localStorage.getItem('token');
                      window.open(`/api/cameras/${streamId}/export?file=${selectedFile}&token=${t}`, '_blank');
                    }}
                    style={{ display: 'flex', alignItems: 'center', gap: '6px', padding: '6px 12px', borderRadius: '6px', background: 'rgba(255,255,255,0.1)', color: '#fff', border: '1px solid var(--card-border)', cursor: 'pointer', fontSize: '0.85rem' }}
                  >
                    <Download size={16} /> Export Full File
                  </button>
                )}
              </div>
            </div>

            {/* Visual Timeline Controls */}
            <div style={{ display: 'flex', gap: '0.5rem', alignItems: 'center' }}>
              <span style={{ color: 'var(--text-muted)', fontSize: '0.85rem' }}>Timeline Zoom:</span>
              {[1, 2, 4, 12, 24].map(z => (
                <button 
                  key={z} 
                  onClick={() => setTimelineZoom(z)} 
                  style={{ background: timelineZoom === z ? 'var(--primary)' : 'rgba(255,255,255,0.1)', border: 'none', color: '#fff', borderRadius: '4px', padding: '2px 8px', fontSize: '0.75rem', cursor: 'pointer', transition: 'background 0.2s' }}
                >
                  {z}x
                </button>
              ))}
            </div>

            {/* Visual Timeline */}
            <div style={{ width: '100%', overflowX: 'auto', borderRadius: '8px', border: '1px solid var(--card-border)', background: 'rgba(0,0,0,0.3)' }}>
              <div style={{ position: 'relative', height: '60px', width: `${timelineZoom * 100}%` }}>
                {/* Hour markers */}
                {Array.from({length: 25}, (_, i) => i).map(hour => (
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
                    onClick={() => handleSelectFile(interval)}
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
            
            {/* Панель точного экспорта выбранного отрезка */}
            {selectedInterval && (
              <div style={{ marginTop: '0.5rem', background: 'rgba(255,255,255,0.03)', padding: '1rem', borderRadius: '8px', border: '1px solid var(--card-border)' }}>
                <div style={{ marginBottom: '1rem', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                  <h4 style={{ margin: 0, fontSize: '0.95rem' }}>Export Fragment</h4>
                  <button 
                    onClick={() => {
                      const t = localStorage.getItem('token');
                      const sSeq = Math.floor(exportStart / 2);
                      const eSeq = Math.ceil(exportEnd / 2);
                      window.open(`/api/cameras/${streamId}/export?file=${selectedFile}&startSeq=${sSeq}&endSeq=${eSeq}&token=${t}`, '_blank');
                    }}
                    style={{ display: 'flex', alignItems: 'center', gap: '6px', padding: '6px 12px', borderRadius: '6px', background: 'var(--primary)', color: '#fff', border: 'none', cursor: 'pointer', fontSize: '0.85rem' }}
                  >
                    <Download size={16} /> Download Fragment
                  </button>
                </div>
                
                <div style={{ display: 'flex', flexDirection: 'column', gap: '1rem' }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '1rem' }}>
                    <span style={{ width: '80px', fontSize: '0.85rem', color: 'var(--text-muted)' }}>Start: {formatTime(exportStart)}</span>
                    <input 
                      type="range" min={0} max={selectedDurSec} value={exportStart} 
                      onChange={e => setExportStart(Math.min(Number(e.target.value), exportEnd - 1))}
                      style={{ flex: 1, accentColor: 'var(--primary)' }} 
                    />
                    <button 
                      onClick={() => setExportStart(Math.min(Math.floor(playerTime), exportEnd - 1))}
                      style={{ padding: '4px 8px', borderRadius: '4px', background: 'rgba(255,255,255,0.1)', border: 'none', color: '#fff', cursor: 'pointer', fontSize: '0.75rem', whiteSpace: 'nowrap' }}
                    >
                      Set to {formatTime(playerTime)}
                    </button>
                  </div>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '1rem' }}>
                    <span style={{ width: '80px', fontSize: '0.85rem', color: 'var(--text-muted)' }}>End: {formatTime(exportEnd)}</span>
                    <input 
                      type="range" min={0} max={selectedDurSec} value={exportEnd} 
                      onChange={e => setExportEnd(Math.max(Number(e.target.value), exportStart + 1))}
                      style={{ flex: 1, accentColor: 'var(--primary)' }} 
                    />
                    <button 
                      onClick={() => setExportEnd(Math.max(Math.floor(playerTime), exportStart + 1))}
                      style={{ padding: '4px 8px', borderRadius: '4px', background: 'rgba(255,255,255,0.1)', border: 'none', color: '#fff', cursor: 'pointer', fontSize: '0.75rem', whiteSpace: 'nowrap' }}
                    >
                      Set to {formatTime(playerTime)}
                    </button>
                  </div>
                </div>
              </div>
            )}
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
