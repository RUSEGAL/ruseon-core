import React, { useState, useEffect, useRef } from 'react';
import { X, Terminal, Trash2, Play, Pause, Copy, Check, Filter } from 'lucide-react';

interface V2LogsModalProps {
  onClose: () => void;
}

interface LogEntry {
  level: string;
  time: number;
  message: string;
  error?: string;
  [key: string]: any;
}

export const V2LogsModal: React.FC<V2LogsModalProps> = ({ onClose }) => {
  const [logs, setLogs] = useState<LogEntry[]>([]);
  const [levelFilter, setLevelFilter] = useState<string>('all');
  const [isPaused, setIsPaused] = useState(false);
  const [autoScroll, setAutoScroll] = useState(true);
  const [copied, setCopied] = useState(false);
  const [search, setSearch] = useState('');
  const logsEndRef = useRef<HTMLDivElement>(null);

  const isPausedRef = useRef(isPaused);
  useEffect(() => {
    isPausedRef.current = isPaused;
  }, [isPaused]);

  useEffect(() => {
    const token = localStorage.getItem('token');
    const url = `/api/logs/stream`;
    const abortController = new AbortController();

    const fetchLogs = async () => {
      try {
        const response = await fetch(url, {
          headers: {
            Authorization: token ? `Bearer ${token}` : '',
            Accept: 'text/event-stream',
          },
          signal: abortController.signal,
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
            const trimmedLine = line.trim();
            if (trimmedLine.startsWith('data: ')) {
              const data = trimmedLine.substring(6);
              try {
                const parsed = JSON.parse(data) as LogEntry;
                if (!isPausedRef.current) {
                  setLogs((prev) => {
                    const next = [...prev, parsed];
                    return next.length > 800 ? next.slice(next.length - 800) : next;
                  });
                }
              } catch {
                // Ignore parse errors
              }
            }
          }
        }
      } catch (e) {
        if (e instanceof Error && e.name !== 'AbortError') {
          console.error('Log stream error:', e);
        }
      }
    };

    fetchLogs();

    return () => {
      abortController.abort();
    };
  }, []);

  useEffect(() => {
    if (autoScroll && !isPaused) {
      logsEndRef.current?.scrollIntoView({ behavior: 'smooth' });
    }
  }, [logs, autoScroll, isPaused]);

  const filteredLogs = logs.filter((log) => {
    if (levelFilter !== 'all' && log.level?.toLowerCase() !== levelFilter.toLowerCase()) {
      return false;
    }
    if (search) {
      const matchMsg = log.message?.toLowerCase().includes(search.toLowerCase());
      const matchErr = log.error?.toLowerCase().includes(search.toLowerCase());
      if (!matchMsg && !matchErr) return false;
    }
    return true;
  });

  const getLevelColor = (lvl: string) => {
    switch (lvl?.toLowerCase()) {
      case 'error':
        return '#ef4444';
      case 'warn':
        return '#f59e0b';
      case 'info':
        return '#10b981';
      case 'debug':
        return '#38bdf8';
      default:
        return '#94a3b8';
    }
  };

  const copyLogs = () => {
    const raw = filteredLogs.map((l) => JSON.stringify(l)).join('\n');
    navigator.clipboard.writeText(raw);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div className="v2-modal-overlay" onClick={onClose}>
      <div
        className="v2-modal-container"
        onClick={(e) => e.stopPropagation()}
        style={{ width: '1100px', maxWidth: '95vw', height: '86vh' }}
      >
        {/* Header */}
        <div className="v2-modal-header">
          <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
            <div
              style={{
                width: '34px',
                height: '34px',
                borderRadius: '8px',
                background: 'rgba(56, 189, 248, 0.15)',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
              }}
            >
              <Terminal size={18} color="#38bdf8" />
            </div>
            <div>
              <h3 style={{ margin: 0, fontSize: '1.1rem', fontWeight: 700, color: '#f8fafc' }}>
                Live Server Logs Stream (SSE)
              </h3>
              <div style={{ fontSize: '0.72rem', color: '#94a3b8' }}>
                Showing {filteredLogs.length} events (Buffer: {logs.length}/800)
              </div>
            </div>
          </div>

          <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
            <button
              onClick={() => setIsPaused(!isPaused)}
              className="v2-btn-secondary"
              style={{ padding: '6px 12px', fontSize: '0.75rem' }}
            >
              {isPaused ? <Play size={13} color="#10b981" /> : <Pause size={13} color="#f59e0b" />}
              <span>{isPaused ? 'Resume' : 'Pause'}</span>
            </button>

            <button
              onClick={copyLogs}
              className="v2-btn-secondary"
              style={{ padding: '6px 12px', fontSize: '0.75rem' }}
            >
              {copied ? <Check size={13} color="#10b981" /> : <Copy size={13} />}
              <span>{copied ? 'Copied' : 'Copy'}</span>
            </button>

            <button
              onClick={() => setLogs([])}
              className="v2-btn-danger"
              style={{ padding: '6px 10px', fontSize: '0.75rem' }}
            >
              <Trash2 size={13} />
            </button>

            <button
              onClick={onClose}
              style={{
                background: 'rgba(255,255,255,0.06)',
                border: 'none',
                borderRadius: '8px',
                padding: '6px',
                color: '#94a3b8',
                cursor: 'pointer',
              }}
            >
              <X size={18} />
            </button>
          </div>
        </div>

        {/* Toolbar */}
        <div
          style={{
            padding: '8px 1.5rem',
            background: 'rgba(0, 0, 0, 0.4)',
            borderBottom: '1px solid var(--v2-card-border)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            gap: '10px',
            flexWrap: 'wrap',
          }}
        >
          {/* Level Filter Pills */}
          <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
            <Filter size={13} color="#94a3b8" />
            {(['all', 'debug', 'info', 'warn', 'error'] as const).map((lvl) => (
              <button
                key={lvl}
                onClick={() => setLevelFilter(lvl)}
                style={{
                  background: levelFilter === lvl ? 'rgba(99, 102, 241, 0.3)' : 'transparent',
                  border: '1px solid',
                  borderColor: levelFilter === lvl ? '#6366f1' : 'rgba(255, 255, 255, 0.08)',
                  borderRadius: '6px',
                  padding: '3px 8px',
                  color: levelFilter === lvl ? '#a5b4fc' : '#94a3b8',
                  fontSize: '0.72rem',
                  fontWeight: 600,
                  cursor: 'pointer',
                  textTransform: 'uppercase',
                }}
              >
                {lvl}
              </button>
            ))}
          </div>

          {/* Search & AutoScroll */}
          <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
            <input
              type="text"
              placeholder="Search in logs..."
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              style={{
                background: 'rgba(255,255,255,0.04)',
                border: '1px solid rgba(255,255,255,0.1)',
                borderRadius: '6px',
                color: '#f8fafc',
                padding: '3px 8px',
                fontSize: '0.75rem',
                outline: 'none',
                width: '180px',
              }}
            />

            <label style={{ display: 'flex', alignItems: 'center', gap: '6px', cursor: 'pointer', fontSize: '0.75rem', color: '#94a3b8' }}>
              <input
                type="checkbox"
                checked={autoScroll}
                onChange={(e) => setAutoScroll(e.target.checked)}
                style={{ accentColor: '#6366f1' }}
              />
              <span>Auto-scroll</span>
            </label>
          </div>
        </div>

        {/* Terminal Body */}
        <div className="v2-modal-body" style={{ padding: '0.75rem 1.5rem 1.5rem', minHeight: 0 }}>
          <div className="v2-terminal">
            {filteredLogs.length === 0 ? (
              <div style={{ color: '#64748b', textAlign: 'center', padding: '2rem' }}>
                Waiting for server logs or no logs match current filter...
              </div>
            ) : (
              filteredLogs.map((log, index) => {
                const lvl = log.level || 'info';
                const timeStr = log.time ? new Date(log.time * 1000).toLocaleTimeString() : '';
                return (
                  <div
                    key={index}
                    style={{
                      display: 'flex',
                      gap: '8px',
                      padding: '2px 0',
                      borderBottom: '1px solid rgba(255,255,255,0.02)',
                    }}
                  >
                    <span style={{ color: '#475569', userSelect: 'none' }}>{timeStr}</span>
                    <span
                      style={{
                        color: getLevelColor(lvl),
                        fontWeight: 700,
                        width: '46px',
                        textTransform: 'uppercase',
                      }}
                    >
                      {lvl}
                    </span>
                    <span style={{ color: '#cbd5e1', flex: 1, wordBreak: 'break-all' }}>
                      {log.message}
                      {log.error && <span style={{ color: '#ef4444', marginLeft: '6px' }}>({log.error})</span>}
                    </span>
                  </div>
                );
              })
            )}
            <div ref={logsEndRef} />
          </div>
        </div>
      </div>
    </div>
  );
};
