import { X, Terminal, Trash2, Play, Square } from 'lucide-react';
import { useState, useEffect, useRef } from 'react';

interface LogsModalProps {
  onClose: () => void;
}

interface LogEntry {
  level: string;
  time: number;
  message: string;
  error?: string;
  [key: string]: any; // for other zerolog fields
}

export function LogsModal({ onClose }: LogsModalProps) {
  const [logs, setLogs] = useState<LogEntry[]>([]);
  const [levelFilter, setLevelFilter] = useState<string>('all');
  const [isPaused, setIsPaused] = useState(false);
  const logsEndRef = useRef<HTMLDivElement>(null);
  const containerRef = useRef<HTMLDivElement>(null);
  
  // Ref to hold the latest state for the event listener without recreating the listener
  const isPausedRef = useRef(isPaused);
  useEffect(() => {
    isPausedRef.current = isPaused;
  }, [isPaused]);

  useEffect(() => {
    const token = localStorage.getItem('token');
    const url = `/api/logs/stream`;
    
    // We can't easily send headers with EventSource natively.
    // Given this is an admin dashboard and EventSource doesn't support headers,
    // we might need to append the token as a query parameter if backend enforces it for SSE,
    // or use a custom polyfill. Let's assume the backend needs the token.
    // Actually, in router.go, the SSE is under `/api` which checks the "Authorization" header.
    // Browsers' native EventSource cannot send custom headers! 
    // We'll have to use the Fetch API to read the stream.

    const abortController = new AbortController();

    const fetchLogs = async () => {
      try {
        const response = await fetch(url, {
          headers: {
            'Authorization': `Bearer ${token}`
          },
          signal: abortController.signal
        });

        if (!response.body) return;

        const reader = response.body.getReader();
        const decoder = new TextDecoder();
        let buffer = '';

        while (true) {
          const { done, value } = await reader.read();
          if (done) break;

          buffer += decoder.decode(value, { stream: true });
          const lines = buffer.split('\n\n');
          
          buffer = lines.pop() || '';

          for (const line of lines) {
            if (line.startsWith('data: ')) {
              const data = line.substring(6);
              try {
                const parsed = JSON.parse(data) as LogEntry;
                if (!isPausedRef.current) {
                  setLogs(prev => {
                    const newLogs = [...prev, parsed];
                    // Keep max 1000 logs in memory to prevent UI lag
                    return newLogs.length > 1000 ? newLogs.slice(newLogs.length - 1000) : newLogs;
                  });
                }
              } catch (e) {
                // Ignore parse errors
              }
            }
          }
        }
      } catch (e) {
        if (e instanceof Error && e.name !== 'AbortError') {
          console.error("Log stream error:", e);
        }
      }
    };

    fetchLogs();

    return () => {
      abortController.abort();
    };
  }, []);

  useEffect(() => {
    if (!isPaused && containerRef.current) {
      const scrollHeight = containerRef.current.scrollHeight;
      const height = containerRef.current.clientHeight;
      const maxScrollTop = scrollHeight - height;
      containerRef.current.scrollTop = maxScrollTop > 0 ? maxScrollTop : 0;
    }
  }, [logs, isPaused]);

  const filteredLogs = logs.filter(log => {
    if (levelFilter === 'all') return true;
    return log.level === levelFilter;
  });

  const getLevelColor = (level: string) => {
    switch (level) {
      case 'info': return 'var(--primary)';
      case 'warn': return 'var(--warning)';
      case 'error': case 'fatal': return 'var(--danger)';
      case 'debug': return '#a8b2c1';
      default: return '#fff';
    }
  };

  return (
    <div className="modal-overlay" onClick={onClose} style={{ zIndex: 1000 }}>
      <div className="modal-content" onClick={e => e.stopPropagation()} style={{ width: '90vw', maxWidth: '1200px', height: '80vh', display: 'flex', flexDirection: 'column' }}>
        
        {/* HEADER */}
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '1.5rem', borderBottom: '1px solid rgba(255,255,255,0.05)', paddingBottom: '1rem' }}>
          <h2 style={{ margin: 0, display: 'flex', alignItems: 'center', gap: '12px' }}>
            <Terminal size={24} color="var(--primary)" />
            Server Logs
          </h2>
          <div style={{ display: 'flex', alignItems: 'center', gap: '1rem' }}>
            <div style={{ display: 'flex', gap: '0.5rem', background: 'rgba(0,0,0,0.2)', padding: '4px', borderRadius: '8px' }}>
              {(['all', 'info', 'warn', 'error'] as const).map(level => (
                <button
                  key={level}
                  onClick={() => setLevelFilter(level)}
                  style={{
                    padding: '4px 12px',
                    borderRadius: '6px',
                    border: 'none',
                    background: levelFilter === level ? 'rgba(255,255,255,0.1)' : 'transparent',
                    color: levelFilter === level ? '#fff' : 'var(--text-muted)',
                    cursor: 'pointer',
                    fontSize: '0.8rem',
                    textTransform: 'uppercase',
                    fontWeight: 600
                  }}
                >
                  {level}
                </button>
              ))}
            </div>

            <button 
              onClick={() => setIsPaused(!isPaused)}
              style={{ display: 'flex', alignItems: 'center', gap: '6px', background: 'rgba(255,255,255,0.05)', border: 'none', color: '#fff', padding: '6px 12px', borderRadius: '6px', cursor: 'pointer' }}
            >
              {isPaused ? <Play size={16} color="var(--success)"/> : <Square size={16} color="var(--warning)"/>}
              {isPaused ? 'Resume' : 'Pause'}
            </button>

            <button 
              onClick={() => setLogs([])}
              style={{ display: 'flex', alignItems: 'center', gap: '6px', background: 'rgba(255, 107, 107, 0.1)', border: 'none', color: 'var(--danger)', padding: '6px 12px', borderRadius: '6px', cursor: 'pointer' }}
            >
              <Trash2 size={16} /> Clear
            </button>
            <button onClick={onClose} style={{ background: 'transparent', border: 'none', color: 'var(--text-muted)', cursor: 'pointer', display: 'flex' }}>
              <X size={24} />
            </button>
          </div>
        </div>

        {/* LOGS CONTAINER */}
        <div 
          ref={containerRef}
          style={{ 
            flex: 1, 
            background: '#0d1117', 
            borderRadius: '8px', 
            border: '1px solid rgba(255,255,255,0.05)',
            padding: '1rem',
            overflowY: 'auto',
            fontFamily: 'monospace',
            fontSize: '0.85rem'
          }}
        >
          {filteredLogs.length === 0 ? (
            <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: '100%', color: 'var(--text-muted)' }}>
              No logs to display...
            </div>
          ) : (
            filteredLogs.map((log, i) => {
              const d = new Date(log.time * 1000);
              const timeStr = `${d.getHours().toString().padStart(2, '0')}:${d.getMinutes().toString().padStart(2, '0')}:${d.getSeconds().toString().padStart(2, '0')}`;
              
              // Extract additional fields to display as JSON
              const { level, time, message, error, ...rest } = log;
              const hasRest = Object.keys(rest).length > 0;

              return (
                <div key={i} style={{ display: 'flex', gap: '1rem', padding: '4px 0', borderBottom: '1px solid rgba(255,255,255,0.02)', whiteSpace: 'pre-wrap', wordBreak: 'break-all' }}>
                  <span style={{ color: 'var(--text-muted)', minWidth: '70px' }}>{timeStr}</span>
                  <span style={{ color: getLevelColor(log.level), minWidth: '50px', textTransform: 'uppercase', fontWeight: 600 }}>{log.level}</span>
                  <span style={{ color: '#e6edf3', flex: 1 }}>
                    {log.message}
                    {log.error && <span style={{ color: 'var(--danger)', marginLeft: '8px' }}>error="{log.error}"</span>}
                    {hasRest && (
                      <span style={{ color: '#8b949e', marginLeft: '8px' }}>
                        {Object.entries(rest).map(([k, v]) => `${k}=${JSON.stringify(v)}`).join(' ')}
                      </span>
                    )}
                  </span>
                </div>
              );
            })
          )}
          <div ref={logsEndRef} />
        </div>
      </div>
    </div>
  );
}
