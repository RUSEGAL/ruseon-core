import React, { useState } from 'react';
import type { CameraInfo } from '../../../types';
import { CameraCell } from './CameraCell';
import type { GridLayoutPreset } from '../../hooks/useSurveillanceLayout';

interface SurveillanceGridProps {
  cameras: CameraInfo[];
  orderedCameraIds: string[];
  layout: GridLayoutPreset;
  onOpenDetails?: (camera: CameraInfo) => void;
  onReorder?: (fromIndex: number, toIndex: number) => void;
}

export const SurveillanceGrid: React.FC<SurveillanceGridProps> = ({
  cameras,
  orderedCameraIds,
  layout,
  onOpenDetails,
  onReorder,
}) => {
  const [maximizedCameraId, setMaximizedCameraId] = useState<string | null>(null);
  const [draggedIndex, setDraggedIndex] = useState<number | null>(null);
  const [dragOverIndex, setDragOverIndex] = useState<number | null>(null);

  const cameraMap = new Map(cameras.map((c) => [c.id, c]));

  const orderedCameras: CameraInfo[] = orderedCameraIds
    .map((id) => cameraMap.get(id))
    .filter((c): c is CameraInfo => !!c);

  if (maximizedCameraId) {
    const singleCam = cameraMap.get(maximizedCameraId);
    if (singleCam) {
      return (
        <div style={{ width: '100%', height: 'calc(100vh - 165px)' }}>
          <CameraCell
            camera={singleCam}
            index={0}
            isMaximized={true}
            onMaximizeToggle={() => setMaximizedCameraId(null)}
            onOpenDetails={onOpenDetails}
          />
        </div>
      );
    }
  }

  const getLayoutClass = () => {
    switch (layout) {
      case '1x1':
        return 'v2-grid-1x1';
      case '2x2':
        return 'v2-grid-2x2';
      case '3x3':
        return 'v2-grid-3x3';
      case '4x4':
        return 'v2-grid-4x4';
      case '1+5':
        return 'v2-grid-1-5';
      default:
        return 'v2-grid-auto';
    }
  };

  const handleDragStart = (_e: React.DragEvent, index: number) => {
    setDraggedIndex(index);
  };

  const handleDragOver = (_e: React.DragEvent, index: number) => {
    setDragOverIndex(index);
  };

  const handleDrop = (_e: React.DragEvent, dropIndex: number) => {
    if (draggedIndex !== null && draggedIndex !== dropIndex && onReorder) {
      onReorder(draggedIndex, dropIndex);
    }
    setDraggedIndex(null);
    setDragOverIndex(null);
  };

  return (
    <div className={`v2-surveillance-grid ${getLayoutClass()}`}>
      {orderedCameras.map((cam, idx) => (
        <div
          key={cam.id}
          className={`v2-camera-cell-wrapper ${dragOverIndex === idx ? 'drag-over' : ''}`}
          style={{ width: '100%', height: '100%' }}
        >
          <CameraCell
            camera={cam}
            index={idx}
            isMaximized={false}
            onMaximizeToggle={(id) => setMaximizedCameraId(id)}
            onOpenDetails={onOpenDetails}
            onDragStart={handleDragStart}
            onDragOver={handleDragOver}
            onDrop={handleDrop}
          />
        </div>
      ))}
    </div>
  );
};
