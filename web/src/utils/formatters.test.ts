import { describe, it, expect } from 'vitest';
import { formatBytes, formatUptime } from './formatters';

describe('formatBytes', () => {
  it('handles zero bytes', () => {
    expect(formatBytes(0)).toBe('0 B');
  });

  it('formats small bytes', () => {
    expect(formatBytes(500)).toBe('500 B');
  });

  it('formats kilobytes correctly', () => {
    expect(formatBytes(1024)).toBe('1 KB');
    expect(formatBytes(1536)).toBe('1.5 KB');
  });

  it('formats megabytes correctly', () => {
    expect(formatBytes(1024 * 1024)).toBe('1 MB');
    expect(formatBytes(1024 * 1024 * 5.25)).toBe('5.25 MB');
  });

  it('formats gigabytes and terabytes correctly', () => {
    expect(formatBytes(1024 * 1024 * 1024)).toBe('1 GB');
    expect(formatBytes(1024 * 1024 * 1024 * 1024 * 2)).toBe('2 TB');
  });
});

describe('formatUptime', () => {
  it('formats 0 seconds', () => {
    expect(formatUptime(0)).toBe('0s');
  });

  it('formats seconds only', () => {
    expect(formatUptime(45)).toBe('45s');
  });

  it('formats minutes and seconds', () => {
    expect(formatUptime(125)).toBe('2m 5s');
  });

  it('formats hours, minutes and seconds', () => {
    expect(formatUptime(3665)).toBe('1h 1m 5s');
  });

  it('formats full days in hours', () => {
    expect(formatUptime(86400)).toBe('24h 0s');
  });
});
