import React, { useState } from 'react';
import { useTranslation } from 'react-i18next';
import type { CameraInfo } from '../../../types';
import { UniversalCameraPlayer } from '../player/UniversalCameraPlayer';
import { GripVertical, AlertCircle } from 'lucide-react';

interface CameraCellProps {
  camera: CameraInfo;
  index: number;
  isMaximized?: boolean;
  onMaximizeToggle?: (cameraId: string) => void;
  onOpenDetails?: (camera: CameraInfo) => void;
  onDragStart?: (e: React.DragEvent, index: number) => void;
  onDragOver?: (e: React.DragEvent, index: number) => void;
  onDrop?: (e: React.DragEvent, index: number) => void;
}

export const CameraCell: React.FC<CameraCellProps> = ({
  camera,
  index,
  isMaximized,
  onMaximizeToggle,
  onOpenDetails,
  onDragStart,
  onDragOver,
  onDrop,
}) => {
  const { t } = useTranslation();
  const [isHovered, setIsHovered] = useState(false);

  return (
    <div
      className="v2-camera-cell"
      draggable
      onDragStart={(e) => onDragStart && onDragStart(e, index)}
      onDragOver={(e) => {
        e.preventDefault();
        if (onDragOver) onDragOver(e, index);
      }}
      onDrop={(e) => onDrop && onDrop(e, index)}
      onMouseEnter={() => setIsHovered(true)}
      onMouseLeave={() => setIsHovered(false)}
      onDoubleClick={() => onMaximizeToggle && onMaximizeToggle(camera.id)}
      style={{
        width: '100%',
        height: '100%',
        position: 'relative',
        cursor: 'default',
      }}
    >
      {/* Drag handle pill on top-center when hovered */}
      {isHovered && !isMaximized && (
        <div
          style={{
            position: 'absolute',
            top: '6px',
            left: '50%',
            transform: 'translateX(-50%)',
            zIndex: 35,
            background: 'rgba(0, 0, 0, 0.75)',
            backdropFilter: 'blur(6px)',
            WebkitBackdropFilter: 'blur(6px)',
            border: '1px solid rgba(255, 255, 255, 0.15)',
            padding: '2px 8px',
            borderRadius: '12px',
            cursor: 'grab',
            color: '#94a3b8',
            display: 'flex',
            alignItems: 'center',
            gap: '2px',
            fontSize: '0.65rem',
          }}
          title="Drag to reorder in grid"
        >
          <GripVertical size={12} />
          <span>{t('cameras.reorder', 'Move')}</span>
        </div>
      )}

      {/* Video Stream or Disabled State */}
      {camera.disabled ? (
        <div
          style={{
            width: '100%',
            height: '100%',
            display: 'flex',
            flexDirection: 'column',
            alignItems: 'center',
            justifyContent: 'center',
            gap: '8px',
            background: '#090d16',
            color: '#94a3b8',
            padding: '1rem',
            textAlign: 'center',
          }}
        >
          <AlertCircle
            size={24}
            color={
              camera.disableReason === 'payment'
                ? '#ef4444'
                : camera.disableReason === 'requested'
                ? '#a855f7'
                : '#f59e0b'
            }
          />
          <span
            style={{
              fontSize: '0.84rem',
              fontWeight: 700,
              color:
                camera.disableReason === 'payment'
                  ? '#fca5a5'
                  : camera.disableReason === 'requested'
                  ? '#d8b4fe'
                  : '#fcd34d',
            }}
          >
            {camera.disableReason === 'payment'
              ? t('cameras.details.reasons.payment', 'Payment')
              : camera.disableReason === 'requested'
              ? t('cameras.details.reasons.requested', 'Requested')
              : t('cameras.details.reasons.technical', 'Technical')}
          </span>
          <span style={{ fontSize: '0.72rem', color: '#64748b' }}>
            {camera.id} ({t('cameras.details.streamDisabledMsg', 'Stream Disabled')})
          </span>

          {onOpenDetails && (
            <button
              onClick={() => onOpenDetails(camera)}
              style={{
                marginTop: '6px',
                background: 'rgba(255, 255, 255, 0.06)',
                border: '1px solid rgba(255, 255, 255, 0.12)',
                borderRadius: '6px',
                padding: '4px 10px',
                color: '#38bdf8',
                fontSize: '0.74rem',
                cursor: 'pointer',
              }}
            >
              {t('cameras.edit', 'Edit')}
            </button>
          )}
        </div>
      ) : (
        <UniversalCameraPlayer
          cameraId={camera.id}
          cameraName={camera.id}
          codec={camera.codec}
          isLive={true}
          isMaximized={isMaximized}
          onOpenDetails={onOpenDetails ? () => onOpenDetails(camera) : undefined}
          onMaximizeToggle={onMaximizeToggle ? () => onMaximizeToggle(camera.id) : undefined}
        />
      )}
    </div>
  );
};
