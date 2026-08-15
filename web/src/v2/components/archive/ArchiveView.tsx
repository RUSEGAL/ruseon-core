import React, { useState, useEffect, useMemo } from 'react';
import type { CameraInfo } from '../../../types';
import type {
  ArchiveSegment,
  DaySecSegment,
} from '../../core/timeline-math';
import {
  convertSegmentsToDaySec,
  findSegmentAtTime,
} from '../../core/timeline-math';
import { ArchiveTimelineBar } from './ArchiveTimelineBar';
import { ArchivePlaybackBar } from './ArchivePlaybackBar';
import { RangeExportModal } from './RangeExportModal';
import { UniversalCameraPlayer } from '../player/UniversalCameraPlayer';
import { Calendar, Video, Film } from 'lucide-react';

interface ArchiveViewProps {
  cameras: CameraInfo[];
  initialCameraId?: string;
}

export const ArchiveView: React.FC<ArchiveViewProps> = ({
  cameras,
  initialCameraId,
}) => {
  const [selectedCamId, setSelectedCamId] = useState<string>(() => {
    return initialCameraId || (cameras.length > 0 ? cameras[0].id : '');
  });

  const [selectedDate, setSelectedDate] = useState<string>(() => {
    const d = new Date();
    return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
  });

  const [rawSegments, setRawSegments] = useState<ArchiveSegment[]>([]);
  const [currentDaySec, setCurrentDaySec] = useState<number>(0);
  const [isPlaying, setIsPlaying] = useState(true);
  const [playbackSpeed, setPlaybackSpeed] = useState(1);
  const [selectedFile, setSelectedFile] = useState<string>('');

  // Range export state
  const [isRangeSelecting, setIsRangeSelecting] = useState(false);
  const [rangeStartSec, setRangeStartSec] = useState<number | null>(null);
  const [rangeEndSec, setRangeEndSec] = useState<number | null>(null);
  const [showExportModal, setShowExportModal] = useState(false);

  useEffect(() => {
    if (!selectedCamId) return;
    const token = localStorage.getItem('token');

    fetch(`/api/cameras/${selectedCamId}/archive`, {
      headers: token ? { Authorization: `Bearer ${token}` } : {},
    })
      .then((r) => r.json())
      .then((data) => {
        if (Array.isArray(data)) {
          setRawSegments(data);
          if (data.length > 0) {
            setSelectedFile(data[0].filename);
          }
        }
      })
      .catch((e) => console.error('Failed to load archive:', e));
  }, [selectedCamId]);

  const daySegments: DaySecSegment[] = useMemo(() => {
    return convertSegmentsToDaySec(rawSegments, selectedDate);
  }, [rawSegments, selectedDate]);

  useEffect(() => {
    if (daySegments.length > 0) {
      setCurrentDaySec(daySegments[0].startSec);
      setSelectedFile(daySegments[0].filename);
    }
  }, [daySegments]);

  const handleSeek = (targetSec: number) => {
    if (isRangeSelecting) {
      if (rangeStartSec === null) {
        setRangeStartSec(targetSec);
      } else if (rangeEndSec === null) {
        if (targetSec > rangeStartSec) {
          setRangeEndSec(targetSec);
        } else {
          setRangeEndSec(rangeStartSec);
          setRangeStartSec(targetSec);
        }
      } else {
        setRangeStartSec(targetSec);
        setRangeEndSec(null);
      }
    }

    setCurrentDaySec(targetSec);
    const snap = findSegmentAtTime(daySegments, targetSec);
    if (snap.segment) {
      setSelectedFile(snap.segment.filename);
    }
  };

  const handleStep = (deltaSec: number) => {
    const nextSec = Math.max(0, Math.min(86400, currentDaySec + deltaSec));
    handleSeek(nextSec);
  };

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100%', gap: '12px' }}>
      {/* Top Controls: Camera Selector & Calendar */}
      <div className="v2-grid-toolbar">
        <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
          {/* Camera Picker */}
          <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
            <Video size={16} color="#6366f1" />
            <select
              value={selectedCamId}
              onChange={(e) => setSelectedCamId(e.target.value)}
              style={{
                background: 'rgba(0, 0, 0, 0.5)',
                border: '1px solid rgba(255, 255, 255, 0.12)',
                borderRadius: '8px',
                color: '#f8fafc',
                padding: '6px 12px',
                fontSize: '0.85rem',
                outline: 'none',
                cursor: 'pointer',
              }}
            >
              {cameras.map((c) => (
                <option key={c.id} value={c.id} style={{ background: '#0d111a' }}>
                  {c.id} {c.record ? '(Recording)' : ''}
                </option>
              ))}
            </select>
          </div>

          {/* Date Picker */}
          <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
            <Calendar size={16} color="#10b981" />
            <input
              type="date"
              value={selectedDate}
              onChange={(e) => setSelectedDate(e.target.value)}
              style={{
                background: 'rgba(0, 0, 0, 0.5)',
                border: '1px solid rgba(255, 255, 255, 0.12)',
                borderRadius: '8px',
                color: '#f8fafc',
                padding: '5px 10px',
                fontSize: '0.85rem',
                outline: 'none',
                cursor: 'pointer',
              }}
            />
          </div>
        </div>

        {/* Status */}
        <div style={{ fontSize: '0.78rem', color: '#94a3b8', display: 'flex', gap: '8px' }}>
          <span>Available Segments: <strong>{daySegments.length}</strong></span>
        </div>
      </div>

      {/* Main Video Stage */}
      <div
        style={{
          flex: 1,
          minHeight: '340px',
          background: '#000',
          borderRadius: '12px',
          overflow: 'hidden',
          border: '1px solid var(--v2-card-border)',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          position: 'relative',
        }}
      >
        {selectedFile ? (
          <UniversalCameraPlayer
            key={selectedFile}
            cameraId={selectedCamId}
            cameraName={`${selectedCamId} (Archive: ${selectedFile})`}
            sourceUrl={`/hls/${selectedCamId}/archive.m3u8?file=${encodeURIComponent(selectedFile)}`}
            isLive={false}
            autoPlay={isPlaying}
          />
        ) : (
          <div
            style={{
              display: 'flex',
              flexDirection: 'column',
              alignItems: 'center',
              gap: '10px',
              color: '#64748b',
            }}
          >
            <Film size={36} />
            <span>No recordings found for {selectedDate}</span>
          </div>
        )}
      </div>

      {/* Interactive 24h Timeline Bar */}
      <ArchiveTimelineBar
        segments={daySegments}
        currentSec={currentDaySec}
        onSeek={handleSeek}
        rangeStartSec={rangeStartSec}
        rangeEndSec={rangeEndSec}
      />

      {/* Playback Controls & Range Export Trigger */}
      <ArchivePlaybackBar
        isPlaying={isPlaying}
        onPlayToggle={() => setIsPlaying(!isPlaying)}
        onStep={handleStep}
        playbackSpeed={playbackSpeed}
        onSpeedChange={(s) => setPlaybackSpeed(s)}
        onRangeSelectToggle={() => {
          setIsRangeSelecting(!isRangeSelecting);
          if (isRangeSelecting) {
            setRangeStartSec(null);
            setRangeEndSec(null);
          }
        }}
        isRangeSelecting={isRangeSelecting}
        onOpenExport={() => setShowExportModal(true)}
      />

      {/* Export Modal */}
      {showExportModal && (
        <RangeExportModal
          cameraId={selectedCamId}
          selectedDate={selectedDate}
          startSec={rangeStartSec !== null ? rangeStartSec : currentDaySec}
          endSec={rangeEndSec !== null ? rangeEndSec : Math.min(86400, currentDaySec + 300)}
          onClose={() => setShowExportModal(false)}
        />
      )}
    </div>
  );
};
