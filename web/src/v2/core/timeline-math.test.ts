import { describe, it, expect } from 'vitest';
import {
  formatDaySecondsToTime,
  formatDuration,
  formatBytes,
  convertSegmentsToDaySec,
  findSegmentAtTime,
  ZOOM_SCALE_SECONDS,
} from './timeline-math';

describe('formatDaySecondsToTime', () => {
  it('formats midnight 00:00:00', () => {
    expect(formatDaySecondsToTime(0)).toBe('00:00:00');
  });

  it('formats midday 12:30:45', () => {
    expect(formatDaySecondsToTime(12 * 3600 + 30 * 60 + 45)).toBe('12:30:45');
  });

  it('formats end of day 23:59:59 and clamps above 86400', () => {
    expect(formatDaySecondsToTime(86399)).toBe('23:59:59');
    expect(formatDaySecondsToTime(90000)).toBe('24:00:00');
  });
});

describe('formatDuration', () => {
  it('formats seconds', () => {
    expect(formatDuration(45)).toBe('45s');
  });

  it('formats minutes and seconds', () => {
    expect(formatDuration(125)).toBe('2m 5s');
    expect(formatDuration(120)).toBe('2m');
  });

  it('formats hours and minutes', () => {
    expect(formatDuration(3660)).toBe('1h 1m');
    expect(formatDuration(7200)).toBe('2h');
  });
});

describe('timeline zoom scales', () => {
  it('defines correct scale durations', () => {
    expect(ZOOM_SCALE_SECONDS['15m']).toBe(900);
    expect(ZOOM_SCALE_SECONDS['1h']).toBe(3600);
    expect(ZOOM_SCALE_SECONDS['4h']).toBe(14400);
    expect(ZOOM_SCALE_SECONDS['12h']).toBe(43200);
    expect(ZOOM_SCALE_SECONDS['24h']).toBe(86400);
  });
});

describe('formatBytes', () => {
  it('formats zero and small bytes', () => {
    expect(formatBytes(0)).toBe('0 B');
    expect(formatBytes(-10)).toBe('0 B');
    expect(formatBytes(500)).toBe('500.0 B');
  });

  it('formats megabytes and gigabytes', () => {
    expect(formatBytes(1024 * 1024 * 15)).toBe('15.0 MB');
    expect(formatBytes(1024 * 1024 * 1024 * 2.5)).toBe('2.5 GB');
  });
});

describe('convertSegmentsToDaySec & findSegmentAtTime', () => {
  const dayStr = '2026-08-18';
  const rawSegments = [
    {
      filename: 'seg_1.mp4',
      start: '2026-08-18T10:00:00.000Z',
      end: '2026-08-18T10:10:00.000Z',
    },
    {
      filename: 'seg_2.mp4',
      start: '2026-08-18T12:00:00.000Z',
      end: '2026-08-18T12:30:00.000Z',
    },
  ];

  it('converts segments within day bounds', () => {
    const daySec = convertSegmentsToDaySec(rawSegments, dayStr);
    expect(daySec.length).toBeGreaterThan(0);
    expect(daySec[0].durationSec).toBe(600); // 10 minutes
  });

  it('finds exact segment at time', () => {
    const daySec = convertSegmentsToDaySec(rawSegments, dayStr);
    if (daySec.length >= 1) {
      const target = daySec[0].startSec + 30;
      const res = findSegmentAtTime(daySec, target);
      expect(res.snapped).toBe(false);
      expect(res.segment?.filename).toBe(daySec[0].filename);
      expect(res.offsetSec).toBe(30);
    }
  });

  it('snaps to next segment when in gap', () => {
    const daySec = convertSegmentsToDaySec(rawSegments, dayStr);
    if (daySec.length >= 2) {
      const gapTarget = daySec[0].endSec + 10;
      const res = findSegmentAtTime(daySec, gapTarget);
      expect(res.snapped).toBe(true);
      expect(res.segment?.filename).toBe(daySec[1].filename);
      expect(res.offsetSec).toBe(0);
    }
  });

  it('returns null on empty segments', () => {
    const res = findSegmentAtTime([], 5000);
    expect(res.segment).toBeNull();
    expect(res.snapped).toBe(false);
  });
});
