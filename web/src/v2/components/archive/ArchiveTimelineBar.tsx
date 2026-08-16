import React, { useRef, useState, useEffect, useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import type {
  DaySecSegment,
  TimelineZoomLevel,
} from '../../core/timeline-math';
import {
  formatDaySecondsToTime,
  ZOOM_SCALE_SECONDS,
} from '../../core/timeline-math';

interface ArchiveTimelineBarProps {
  segments: DaySecSegment[];
  currentSec: number;
  onSeek: (targetSec: number) => void;
  rangeStartSec?: number | null;
  rangeEndSec?: number | null;
  zoomLevel?: TimelineZoomLevel;
}

export const ArchiveTimelineBar: React.FC<ArchiveTimelineBarProps> = ({
  segments,
  currentSec,
  onSeek,
  rangeStartSec,
  rangeEndSec,
  zoomLevel = '24h',
}) => {
  const { t } = useTranslation();
  const trackRef = useRef<HTMLDivElement>(null);
  const [isDragging, setIsDragging] = useState(false);
  const [hoverSec, setHoverSec] = useState<number | null>(null);

  const totalSec = ZOOM_SCALE_SECONDS[zoomLevel] || 86400;

  const calculateSecFromX = useCallback(
    (clientX: number): number => {
      if (!trackRef.current) return 0;
      const rect = trackRef.current.getBoundingClientRect();
      const x = Math.max(0, Math.min(rect.width, clientX - rect.left));
      const ratio = x / rect.width;
      return Math.round(ratio * totalSec);
    },
    [totalSec]
  );

  const handleMouseDown = (e: React.MouseEvent) => {
    setIsDragging(true);
    const sec = calculateSecFromX(e.clientX);
    onSeek(sec);
  };

  const handleMouseMove = (e: React.MouseEvent) => {
    const sec = calculateSecFromX(e.clientX);
    setHoverSec(sec);
    if (isDragging) {
      onSeek(sec);
    }
  };

  const handleMouseLeave = () => {
    setHoverSec(null);
  };

  useEffect(() => {
    const handleGlobalMouseUp = () => {
      if (isDragging) {
        setIsDragging(false);
      }
    };
    window.addEventListener('mouseup', handleGlobalMouseUp);
    return () => window.removeEventListener('mouseup', handleGlobalMouseUp);
  }, [isDragging]);

  const getPercent = (sec: number) => {
    return Math.max(0, Math.min(100, (sec / totalSec) * 100));
  };

  const hourTicks: number[] = [];
  const tickStep = zoomLevel === '15m' ? 120 : zoomLevel === '1h' ? 600 : zoomLevel === '4h' ? 1800 : 3600 * 2;
  for (let s = 0; s <= totalSec; s += tickStep) {
    hourTicks.push(s);
  }

  const isRangeValid =
    rangeStartSec !== null &&
    rangeStartSec !== undefined &&
    rangeEndSec !== null &&
    rangeEndSec !== undefined &&
    rangeEndSec > rangeStartSec;

  return (
    <div className="v2-timeline-container" style={{ userSelect: 'none' }}>
      {/* Top Bar: Time Info */}
      <div
        style={{
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
          fontSize: '0.78rem',
          color: '#94a3b8',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
          <span style={{ fontWeight: 600, color: '#f1f5f9' }}>
            {formatDaySecondsToTime(currentSec)}
          </span>
          {hoverSec !== null && (
            <span style={{ color: '#6366f1', fontSize: '0.72rem' }}>
              {t('v2.archive.cursor', 'Cursor:')} {formatDaySecondsToTime(hoverSec)}
            </span>
          )}
        </div>

        <div style={{ display: 'flex', gap: '12px', fontSize: '0.72rem' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '4px' }}>
            <div style={{ width: '8px', height: '8px', background: '#10b981', borderRadius: '2px' }} />
            <span>{t('v2.archive.continuousFmp4', 'Continuous fMP4')}</span>
          </div>
          <div style={{ display: 'flex', alignItems: 'center', gap: '4px' }}>
            <div style={{ width: '8px', height: '8px', background: '#f59e0b', borderRadius: '2px' }} />
            <span>{t('v2.archive.aiEventMotion', 'AI Event / Motion')}</span>
          </div>
        </div>
      </div>

      {/* Main Interactive Timeline Track */}
      <div
        ref={trackRef}
        className="v2-timeline-track"
        onMouseDown={handleMouseDown}
        onMouseMove={handleMouseMove}
        onMouseLeave={handleMouseLeave}
      >
        {/* Hour Axis Grid Lines */}
        {hourTicks.map((tickSec) => {
          const leftPct = getPercent(tickSec);
          return (
            <div
              key={tickSec}
              style={{
                position: 'absolute',
                left: `${leftPct}%`,
                top: 0,
                bottom: 0,
                width: '1px',
                background: 'rgba(255, 255, 255, 0.08)',
                pointerEvents: 'none',
              }}
            />
          );
        })}

        {/* Available Recording Intervals */}
        {segments.map((seg, i) => {
          const startPct = getPercent(seg.startSec);
          const endPct = getPercent(seg.endSec);
          const widthPct = Math.max(0.2, endPct - startPct);

          return (
            <div
              key={`${seg.filename}_${i}`}
              className="v2-timeline-segment"
              style={{
                left: `${startPct}%`,
                width: `${widthPct}%`,
              }}
              title={`${t('v2.archive.segment', 'Segment:')} ${formatDaySecondsToTime(seg.startSec)} - ${formatDaySecondsToTime(seg.endSec)}`}
            />
          );
        })}

        {/* Selected Export Range Highlight */}
        {isRangeValid && (
          <div
            className="v2-timeline-range-highlight"
            style={{
              left: `${getPercent(rangeStartSec!)}%`,
              width: `${getPercent(rangeEndSec!) - getPercent(rangeStartSec!)}%`,
            }}
          />
        )}

        {/* Current Playhead Cursor */}
        <div
          className="v2-timeline-cursor"
          style={{
            left: `${getPercent(currentSec)}%`,
          }}
        >
          <div
            style={{
              position: 'absolute',
              top: '-4px',
              left: '-4px',
              width: '10px',
              height: '10px',
              background: '#ef4444',
              borderRadius: '50%',
            }}
          />
        </div>

        {/* Hover Guide Line */}
        {hoverSec !== null && !isDragging && (
          <div
            style={{
              position: 'absolute',
              left: `${getPercent(hoverSec)}%`,
              top: 0,
              bottom: 0,
              width: '1px',
              background: 'rgba(99, 102, 241, 0.6)',
              pointerEvents: 'none',
            }}
          />
        )}
      </div>

      {/* Axis Hour Labels */}
      <div
        style={{
          display: 'flex',
          justifyContent: 'space-between',
          fontSize: '0.68rem',
          color: '#64748b',
          padding: '0 2px',
        }}
      >
        <span>00:00</span>
        <span>04:00</span>
        <span>08:00</span>
        <span>12:00</span>
        <span>16:00</span>
        <span>20:00</span>
        <span>24:00</span>
      </div>
    </div>
  );
};
