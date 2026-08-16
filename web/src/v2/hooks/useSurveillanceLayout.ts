import { useState, useEffect, useCallback } from 'react';

export type GridLayoutPreset = '1x1' | '2x2' | '3x3' | '4x4' | '1+5' | '1+7' | 'auto';

const STORAGE_GRID_PRESET = 'ruseon_surveillance_grid_preset';
const STORAGE_CAMERA_ORDER = 'ruseon_surveillance_camera_order';

export function useSurveillanceLayout(availableCameraIds: string[]) {
  const [layout, setLayoutState] = useState<GridLayoutPreset>(() => {
    return (localStorage.getItem(STORAGE_GRID_PRESET) as GridLayoutPreset) || '2x2';
  });

  const [orderedCameraIds, setOrderedCameraIds] = useState<string[]>(() => {
    try {
      const saved = localStorage.getItem(STORAGE_CAMERA_ORDER);
      if (saved) {
        const parsed = JSON.parse(saved);
        if (Array.isArray(parsed)) return parsed;
      }
    } catch {
      // Fallback
    }
    return availableCameraIds;
  });

  // Sync available cameras when new ones appear or removed
  useEffect(() => {
    setOrderedCameraIds((prev) => {
      const existing = new Set(prev);
      const newIds = availableCameraIds.filter((id) => !existing.has(id));
      const validOldIds = prev.filter((id) => availableCameraIds.includes(id));
      const merged = [...validOldIds, ...newIds];
      localStorage.setItem(STORAGE_CAMERA_ORDER, JSON.stringify(merged));
      return merged;
    });
  }, [availableCameraIds]);

  const setLayout = useCallback((newLayout: GridLayoutPreset) => {
    setLayoutState(newLayout);
    localStorage.setItem(STORAGE_GRID_PRESET, newLayout);
  }, []);

  const reorderCameras = useCallback((fromIndex: number, toIndex: number) => {
    setOrderedCameraIds((prev) => {
      if (fromIndex < 0 || fromIndex >= prev.length || toIndex < 0 || toIndex >= prev.length) {
        return prev;
      }
      const updated = [...prev];
      const [moved] = updated.splice(fromIndex, 1);
      updated.splice(toIndex, 0, moved);
      localStorage.setItem(STORAGE_CAMERA_ORDER, JSON.stringify(updated));
      return updated;
    });
  }, []);

  const moveCameraToFront = useCallback((cameraId: string) => {
    setOrderedCameraIds((prev) => {
      const filtered = prev.filter((id) => id !== cameraId);
      const updated = [cameraId, ...filtered];
      localStorage.setItem(STORAGE_CAMERA_ORDER, JSON.stringify(updated));
      return updated;
    });
  }, []);

  return {
    layout,
    setLayout,
    orderedCameraIds,
    reorderCameras,
    moveCameraToFront,
  };
}
