import React, { useState } from 'react';
import { X, Download, Scissors, Loader2 } from 'lucide-react';
import {
  formatDaySecondsToTime,
  formatDuration,
  formatBytes,
} from '../../core/timeline-math';

interface RangeExportModalProps {
  cameraId: string;
  selectedDate: string;
  startSec: number;
  endSec: number;
  onClose: () => void;
}

export const RangeExportModal: React.FC<RangeExportModalProps> = ({
  cameraId,
  selectedDate,
  startSec,
  endSec,
  onClose,
}) => {
  const [rangeStart] = useState(startSec);
  const [rangeEnd] = useState(endSec);
  const [exporting, setExporting] = useState(false);
  const [exportFormat, setExportFormat] = useState<'mp4' | 'dataset'>('mp4');

  const durationSec = Math.max(1, rangeEnd - rangeStart);
  const estimatedBytes = durationSec * 375 * 1024;

  const handleExport = async () => {
    setExporting(true);
    try {
      const token = localStorage.getItem('token');
      const query = new URLSearchParams({
        date: selectedDate,
        startSec: rangeStart.toString(),
        endSec: rangeEnd.toString(),
        format: exportFormat,
      });

      const exportUrl = `/api/cameras/${cameraId}/export?${query.toString()}`;
      const res = await fetch(exportUrl, {
        headers: token ? { Authorization: `Bearer ${token}` } : {},
      });

      if (!res.ok) {
        throw new Error(`Export failed with HTTP ${res.status}`);
      }

      const blob = await res.blob();
      const url = window.URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `${cameraId}_${selectedDate}_${formatDaySecondsToTime(rangeStart).replace(/:/g, '')}-${formatDaySecondsToTime(rangeEnd).replace(/:/g, '')}.${exportFormat === 'mp4' ? 'mp4' : 'tar.gz'}`;
      document.body.appendChild(a);
      a.click();
      window.URL.revokeObjectURL(url);
      onClose();
    } catch (err) {
      console.error('Export failed:', err);
      alert('Export failed. Please check server logs.');
    } finally {
      setExporting(false);
    }
  };

  return (
    <div
      style={{
        position: 'fixed',
        inset: 0,
        backgroundColor: 'rgba(0, 0, 0, 0.75)',
        backdropFilter: 'blur(6px)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        zIndex: 200,
      }}
      onClick={onClose}
    >
      <div
        className="glass"
        style={{
          width: '460px',
          padding: '1.5rem',
          borderRadius: '16px',
          background: '#0d111a',
          border: '1px solid var(--v2-card-border)',
          display: 'flex',
          flexDirection: 'column',
          gap: '1.2rem',
        }}
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
            <Scissors size={18} color="#6366f1" />
            <h3 style={{ fontSize: '1.1rem', fontWeight: 600, color: '#f8fafc' }}>
              Export Archive Range
            </h3>
          </div>
          <button
            onClick={onClose}
            style={{ background: 'transparent', border: 'none', color: '#94a3b8', cursor: 'pointer' }}
          >
            <X size={18} />
          </button>
        </div>

        {/* Info Grid */}
        <div
          style={{
            background: 'rgba(255, 255, 255, 0.03)',
            borderRadius: '10px',
            padding: '1rem',
            display: 'flex',
            flexDirection: 'column',
            gap: '0.75rem',
            border: '1px solid rgba(255, 255, 255, 0.05)',
          }}
        >
          <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '0.85rem' }}>
            <span style={{ color: '#94a3b8' }}>Camera ID:</span>
            <span style={{ fontWeight: 600, color: '#f1f5f9' }}>{cameraId}</span>
          </div>
          <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '0.85rem' }}>
            <span style={{ color: '#94a3b8' }}>Date:</span>
            <span style={{ fontWeight: 600, color: '#f1f5f9' }}>{selectedDate}</span>
          </div>
          <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '0.85rem' }}>
            <span style={{ color: '#94a3b8' }}>Time Interval:</span>
            <span style={{ fontWeight: 600, color: '#a5b4fc' }}>
              {formatDaySecondsToTime(rangeStart)} → {formatDaySecondsToTime(rangeEnd)}
            </span>
          </div>
          <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '0.85rem' }}>
            <span style={{ color: '#94a3b8' }}>Duration:</span>
            <span style={{ fontWeight: 600, color: '#10b981' }}>{formatDuration(durationSec)}</span>
          </div>
          <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '0.85rem' }}>
            <span style={{ color: '#94a3b8' }}>Estimated Size:</span>
            <span style={{ fontWeight: 600, color: '#f1f5f9' }}>{formatBytes(estimatedBytes)}</span>
          </div>
        </div>

        {/* Format Selector */}
        <div style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
          <label style={{ fontSize: '0.8rem', color: '#94a3b8' }}>Export Target:</label>
          <div style={{ display: 'flex', gap: '8px' }}>
            <button
              onClick={() => setExportFormat('mp4')}
              style={{
                flex: 1,
                padding: '8px',
                borderRadius: '8px',
                border: '1px solid',
                borderColor: exportFormat === 'mp4' ? '#6366f1' : 'rgba(255,255,255,0.1)',
                background: exportFormat === 'mp4' ? 'rgba(99,102,241,0.2)' : 'transparent',
                color: exportFormat === 'mp4' ? '#a5b4fc' : '#94a3b8',
                fontSize: '0.82rem',
                fontWeight: 600,
                cursor: 'pointer',
              }}
            >
              Video MP4 (H.264/H.265)
            </button>
            <button
              onClick={() => setExportFormat('dataset')}
              style={{
                flex: 1,
                padding: '8px',
                borderRadius: '8px',
                border: '1px solid',
                borderColor: exportFormat === 'dataset' ? '#10b981' : 'rgba(255,255,255,0.1)',
                background: exportFormat === 'dataset' ? 'rgba(16,185,129,0.2)' : 'transparent',
                color: exportFormat === 'dataset' ? '#6ee7b7' : '#94a3b8',
                fontSize: '0.82rem',
                fontWeight: 600,
                cursor: 'pointer',
              }}
            >
              AI Dataset (Frames + Meta)
            </button>
          </div>
        </div>

        {/* Actions */}
        <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '10px', marginTop: '6px' }}>
          <button
            onClick={onClose}
            style={{
              padding: '8px 16px',
              borderRadius: '8px',
              border: '1px solid rgba(255, 255, 255, 0.1)',
              background: 'transparent',
              color: '#94a3b8',
              fontSize: '0.85rem',
              cursor: 'pointer',
            }}
          >
            Cancel
          </button>
          <button
            onClick={handleExport}
            disabled={exporting}
            style={{
              padding: '8px 18px',
              borderRadius: '8px',
              border: 'none',
              background: '#6366f1',
              color: '#fff',
              fontSize: '0.85rem',
              fontWeight: 600,
              cursor: exporting ? 'not-allowed' : 'pointer',
              display: 'flex',
              alignItems: 'center',
              gap: '6px',
            }}
          >
            {exporting ? (
              <>
                <Loader2 size={16} className="animate-spin" />
                <span>Exporting...</span>
              </>
            ) : (
              <>
                <Download size={16} />
                <span>Start Export</span>
              </>
            )}
          </button>
        </div>
      </div>
    </div>
  );
};
