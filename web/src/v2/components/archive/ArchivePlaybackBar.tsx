import React from 'react';
import { useTranslation } from 'react-i18next';
import {
  Play,
  Pause,
  RotateCcw,
  RotateCw,
  Scissors,
  Download,
} from 'lucide-react';

interface ArchivePlaybackBarProps {
  isPlaying: boolean;
  onPlayToggle: () => void;
  onStep: (deltaSeconds: number) => void;
  playbackSpeed: number;
  onSpeedChange: (speed: number) => void;
  onRangeSelectToggle: () => void;
  isRangeSelecting: boolean;
  onOpenExport: () => void;
}

const SPEEDS = [0.5, 1, 2, 4, 8, 16];

export const ArchivePlaybackBar: React.FC<ArchivePlaybackBarProps> = ({
  isPlaying,
  onPlayToggle,
  onStep,
  playbackSpeed,
  onSpeedChange,
  onRangeSelectToggle,
  isRangeSelecting,
  onOpenExport,
}) => {
  const { t } = useTranslation();

  return (
    <div
      style={{
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        background: 'var(--v2-card-bg)',
        border: '1px solid var(--v2-card-border)',
        borderRadius: '12px',
        padding: '0.6rem 1.25rem',
      }}
    >
      {/* Left: Step Controls & Play/Pause */}
      <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
        <button
          onClick={() => onStep(-10)}
          style={{
            background: 'rgba(255, 255, 255, 0.05)',
            border: '1px solid rgba(255, 255, 255, 0.1)',
            borderRadius: '8px',
            padding: '6px 10px',
            color: '#94a3b8',
            cursor: 'pointer',
            display: 'flex',
            alignItems: 'center',
            gap: '4px',
            fontSize: '0.75rem',
          }}
          title={t('v2.archive.stepBack', '-10s')}
        >
          <RotateCcw size={14} />
          <span>-10s</span>
        </button>

        <button
          onClick={onPlayToggle}
          style={{
            background: isPlaying ? 'rgba(239, 68, 68, 0.2)' : 'var(--v2-primary)',
            border: 'none',
            borderRadius: '50%',
            width: '38px',
            height: '38px',
            color: '#fff',
            cursor: 'pointer',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            boxShadow: '0 0 12px rgba(99, 102, 241, 0.4)',
          }}
          title={isPlaying ? t('v2.archive.pause', 'Pause') : t('v2.archive.play', 'Play')}
        >
          {isPlaying ? <Pause size={18} /> : <Play size={18} style={{ marginLeft: '2px' }} />}
        </button>

        <button
          onClick={() => onStep(10)}
          style={{
            background: 'rgba(255, 255, 255, 0.05)',
            border: '1px solid rgba(255, 255, 255, 0.1)',
            borderRadius: '8px',
            padding: '6px 10px',
            color: '#94a3b8',
            cursor: 'pointer',
            display: 'flex',
            alignItems: 'center',
            gap: '4px',
            fontSize: '0.75rem',
          }}
          title={t('v2.archive.stepForward', '+10s')}
        >
          <RotateCw size={14} />
          <span>+10s</span>
        </button>
      </div>

      {/* Center: Playback Speed Buttons */}
      <div
        style={{
          display: 'flex',
          background: 'rgba(0, 0, 0, 0.4)',
          borderRadius: '8px',
          padding: '2px',
          border: '1px solid rgba(255, 255, 255, 0.08)',
        }}
      >
        {SPEEDS.map((spd) => {
          const isSelected = playbackSpeed === spd;
          return (
            <button
              key={spd}
              onClick={() => onSpeedChange(spd)}
              style={{
                background: isSelected ? 'rgba(99, 102, 241, 0.4)' : 'transparent',
                border: 'none',
                borderRadius: '6px',
                padding: '4px 8px',
                color: isSelected ? '#a5b4fc' : '#64748b',
                fontSize: '0.75rem',
                fontWeight: 600,
                cursor: 'pointer',
              }}
            >
              {spd}x
            </button>
          );
        })}
      </div>

      {/* Right: Range Selector & Export Action */}
      <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
        <button
          onClick={onRangeSelectToggle}
          style={{
            background: isRangeSelecting ? 'rgba(99, 102, 241, 0.25)' : 'rgba(255, 255, 255, 0.05)',
            border: '1px solid',
            borderColor: isRangeSelecting ? '#6366f1' : 'rgba(255, 255, 255, 0.1)',
            borderRadius: '8px',
            padding: '6px 12px',
            color: isRangeSelecting ? '#a5b4fc' : '#94a3b8',
            fontSize: '0.78rem',
            fontWeight: 600,
            cursor: 'pointer',
            display: 'flex',
            alignItems: 'center',
            gap: '6px',
          }}
          title={t('v2.archive.selectRange', 'Select Range')}
        >
          <Scissors size={14} />
          <span>{isRangeSelecting ? t('v2.archive.cancelSelection', 'Cancel Selection') : t('v2.archive.selectRange', 'Select Range')}</span>
        </button>

        <button
          onClick={onOpenExport}
          style={{
            background: 'var(--v2-primary)',
            border: 'none',
            borderRadius: '8px',
            padding: '6px 14px',
            color: '#fff',
            fontSize: '0.78rem',
            fontWeight: 600,
            cursor: 'pointer',
            display: 'flex',
            alignItems: 'center',
            gap: '6px',
          }}
          title={t('v2.archive.export', 'Export')}
        >
          <Download size={14} />
          <span>{t('v2.archive.export', 'Export')}</span>
        </button>
      </div>
    </div>
  );
};
