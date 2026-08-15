import React from 'react';
import {
  ChevronUp,
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  ZoomIn,
  ZoomOut,
  RotateCcw,
  X,
} from 'lucide-react';

interface PtzControlOverlayProps {
  cameraId: string;
  onClose: () => void;
}

export const PtzControlOverlay: React.FC<PtzControlOverlayProps> = ({
  cameraId,
  onClose,
}) => {
  const sendPtzCommand = (action: string, speed = 0.5) => {
    // API trigger for ONVIF PTZ
    const token = localStorage.getItem('token');
    fetch(`/api/cameras/${cameraId}/ptz`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${token}`,
      },
      body: JSON.stringify({ action, speed }),
    }).catch(() => {
      // Mock / fallback if camera doesn't support PTZ
      console.log(`[PTZ] Sent ${action} to ${cameraId}`);
    });
  };

  return (
    <div
      className="v2-ptz-overlay"
      onClick={(e) => e.stopPropagation()}
    >
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          borderBottom: '1px solid rgba(255, 255, 255, 0.1)',
          paddingBottom: '4px',
        }}
      >
        <span style={{ fontSize: '0.72rem', fontWeight: 700, color: '#a5b4fc' }}>
          PTZ Control
        </span>
        <button
          onClick={onClose}
          style={{
            background: 'transparent',
            border: 'none',
            color: '#94a3b8',
            cursor: 'pointer',
            padding: '2px',
          }}
        >
          <X size={14} />
        </button>
      </div>

      {/* Directional Keypad */}
      <div className="v2-ptz-keypad">
        <div />
        <button
          className="v2-ptz-btn"
          onMouseDown={() => sendPtzCommand('up')}
          onMouseUp={() => sendPtzCommand('stop')}
          title="Tilt Up"
        >
          <ChevronUp size={16} />
        </button>
        <div />

        <button
          className="v2-ptz-btn"
          onMouseDown={() => sendPtzCommand('left')}
          onMouseUp={() => sendPtzCommand('stop')}
          title="Pan Left"
        >
          <ChevronLeft size={16} />
        </button>
        <button
          className="v2-ptz-btn"
          onClick={() => sendPtzCommand('home')}
          title="Home / Center"
        >
          <RotateCcw size={14} />
        </button>
        <button
          className="v2-ptz-btn"
          onMouseDown={() => sendPtzCommand('right')}
          onMouseUp={() => sendPtzCommand('stop')}
          title="Pan Right"
        >
          <ChevronRight size={16} />
        </button>

        <div />
        <button
          className="v2-ptz-btn"
          onMouseDown={() => sendPtzCommand('down')}
          onMouseUp={() => sendPtzCommand('stop')}
          title="Tilt Down"
        >
          <ChevronDown size={16} />
        </button>
        <div />
      </div>

      {/* Zoom Controls */}
      <div style={{ display: 'flex', gap: '4px', marginTop: '2px' }}>
        <button
          className="v2-ptz-btn"
          style={{ flex: 1, padding: '4px 0' }}
          onMouseDown={() => sendPtzCommand('zoom_in')}
          onMouseUp={() => sendPtzCommand('stop')}
          title="Zoom In"
        >
          <ZoomIn size={14} />
        </button>
        <button
          className="v2-ptz-btn"
          style={{ flex: 1, padding: '4px 0' }}
          onMouseDown={() => sendPtzCommand('zoom_out')}
          onMouseUp={() => sendPtzCommand('stop')}
          title="Zoom Out"
        >
          <ZoomOut size={14} />
        </button>
      </div>
    </div>
  );
};
