/**
 * Pure math and utility functions for interactive fMP4 archive timeline.
 * Handles 24h day projections, interval snapping, zoom scales, and range calculations.
 */

export interface ArchiveSegment {
  filename: string;
  start: string; // ISO 8601
  end: string;   // ISO 8601
  sizeBytes?: number;
}

export interface DaySecSegment {
  filename: string;
  startSec: number; // Seconds from day start 00:00 (0 - 86400)
  endSec: number;
  durationSec: number;
  rawStart: string;
  rawEnd: string;
}

export interface SeekSnapResult {
  segment: DaySecSegment | null;
  offsetSec: number;
  snapped: boolean;
}

export interface TimeRangeSelection {
  startSec: number;
  endSec: number;
  durationSec: number;
  estimatedBytes: number;
}

export type TimelineZoomLevel = '15m' | '1h' | '4h' | '12h' | '24h';

export const ZOOM_SCALE_SECONDS: Record<TimelineZoomLevel, number> = {
  '15m': 15 * 60,
  '1h': 60 * 60,
  '4h': 4 * 60 * 60,
  '12h': 12 * 60 * 60,
  '24h': 24 * 60 * 60,
};

/**
 * Parses ISO date string to local day start timestamp (local 00:00:00).
 */
export function getLocalDayStartMs(isoOrDateStr: string): number {
  const d = new Date(isoOrDateStr);
  const localMidnight = new Date(d.getFullYear(), d.getMonth(), d.getDate(), 0, 0, 0, 0);
  return localMidnight.getTime();
}

/**
 * Formats seconds-from-midnight (0..86400) into HH:MM:SS format.
 */
export function formatDaySecondsToTime(daySec: number): string {
  const total = Math.max(0, Math.min(86400, Math.floor(daySec)));
  const h = Math.floor(total / 3600);
  const m = Math.floor((total % 3600) / 60);
  const s = total % 60;
  return `${String(h).padStart(2, '0')}:${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`;
}

/**
 * Formats duration in seconds to human readable string (e.g. "1h 24m", "45s").
 */
export function formatDuration(sec: number): string {
  const s = Math.floor(sec);
  if (s < 60) return `${s}s`;
  const m = Math.floor(s / 60);
  const remSec = s % 60;
  if (m < 60) return remSec > 0 ? `${m}m ${remSec}s` : `${m}m`;
  const h = Math.floor(m / 60);
  const remMin = m % 60;
  return remMin > 0 ? `${h}h ${remMin}m` : `${h}h`;
}

/**
 * Formats bytes to human readable string (e.g. "45.2 MB", "1.2 GB").
 */
export function formatBytes(bytes: number): string {
  if (bytes <= 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(1024));
  return `${(bytes / Math.pow(1024, i)).toFixed(1)} ${units[i]}`;
}

/**
 * Converts ISO ArchiveSegments for a given day into DaySecSegments (0..86400).
 */
export function convertSegmentsToDaySec(
  segments: ArchiveSegment[],
  dayDateStr: string
): DaySecSegment[] {
  const dayStartMs = getLocalDayStartMs(dayDateStr);
  const dayEndMs = dayStartMs + 24 * 60 * 60 * 1000;

  const result: DaySecSegment[] = [];

  for (const seg of segments) {
    const startMs = new Date(seg.start).getTime();
    const endMs = new Date(seg.end).getTime();

    // Check overlap with current local day
    if (endMs < dayStartMs || startMs > dayEndMs) {
      continue;
    }

    const clampedStartMs = Math.max(startMs, dayStartMs);
    const clampedEndMs = Math.min(endMs, dayEndMs);

    const startSec = Math.max(0, (clampedStartMs - dayStartMs) / 1000);
    const endSec = Math.min(86400, (clampedEndMs - dayStartMs) / 1000);
    const durationSec = Math.max(0, endSec - startSec);

    if (durationSec > 0) {
      result.push({
        filename: seg.filename,
        startSec,
        endSec,
        durationSec,
        rawStart: seg.start,
        rawEnd: seg.end,
      });
    }
  }

  return result.sort((a, b) => a.startSec - b.startSec);
}

/**
 * Finds segment containing targetSec or snaps to the nearest available recording.
 */
export function findSegmentAtTime(
  segments: DaySecSegment[],
  targetSec: number
): SeekSnapResult {
  if (segments.length === 0) {
    return { segment: null, offsetSec: 0, snapped: false };
  }

  // Exact hit
  for (const seg of segments) {
    if (targetSec >= seg.startSec && targetSec <= seg.endSec) {
      return { segment: seg, offsetSec: targetSec - seg.startSec, snapped: false };
    }
  }

  // Snap to next segment if in gap
  const next = segments.find((s) => s.startSec > targetSec);
  if (next) {
    return { segment: next, offsetSec: 0, snapped: true };
  }

  // Snap to end of previous
  const prev = segments[segments.length - 1];
  return { segment: prev, offsetSec: prev.durationSec, snapped: true };
}
