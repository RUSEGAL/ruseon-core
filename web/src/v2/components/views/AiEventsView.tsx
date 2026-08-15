import React, { useState } from 'react';
import { Cpu, Search } from 'lucide-react';
import type { CameraInfo } from '../../../types';

interface AiEventsViewProps {
  cameras: CameraInfo[];
}

interface MockAiEvent {
  id: string;
  cameraId: string;
  timestamp: string;
  label: string;
  confidence: number;
  box: [number, number, number, number];
}

export const AiEventsView: React.FC<AiEventsViewProps> = ({ cameras }) => {
  const [selectedCam, setSelectedCam] = useState<string>('all');
  const [searchLabel, setSearchLabel] = useState<string>('');

  const mockEvents: MockAiEvent[] = [
    { id: '1', cameraId: cameras[0]?.id || 'cam-01', timestamp: new Date(Date.now() - 10000).toLocaleTimeString(), label: 'person', confidence: 0.94, box: [120, 80, 200, 450] },
    { id: '2', cameraId: cameras[0]?.id || 'cam-01', timestamp: new Date(Date.now() - 25000).toLocaleTimeString(), label: 'car', confidence: 0.88, box: [300, 150, 600, 400] },
    { id: '3', cameraId: cameras[1]?.id || 'cam-02', timestamp: new Date(Date.now() - 60000).toLocaleTimeString(), label: 'license_plate', confidence: 0.96, box: [450, 320, 560, 370] },
    { id: '4', cameraId: cameras[0]?.id || 'cam-01', timestamp: new Date(Date.now() - 95000).toLocaleTimeString(), label: 'person', confidence: 0.91, box: [180, 90, 250, 420] },
  ];

  const filteredEvents = mockEvents.filter((ev) => {
    if (selectedCam !== 'all' && ev.cameraId !== selectedCam) return false;
    if (searchLabel && !ev.label.toLowerCase().includes(searchLabel.toLowerCase())) return false;
    return true;
  });

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '1.25rem' }}>
      <div className="v2-grid-toolbar">
        <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
          <Cpu size={18} color="#6366f1" />
          <h2 style={{ fontSize: '1.1rem', fontWeight: 600, color: '#f8fafc' }}>
            AI Metadata Stream (gRPC / WebVTT)
          </h2>
        </div>

        <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
          <select
            value={selectedCam}
            onChange={(e) => setSelectedCam(e.target.value)}
            style={{
              background: 'rgba(0,0,0,0.5)',
              border: '1px solid rgba(255,255,255,0.12)',
              borderRadius: '8px',
              color: '#f8fafc',
              padding: '5px 10px',
              fontSize: '0.8rem',
            }}
          >
            <option value="all">All Cameras</option>
            {cameras.map((c) => (
              <option key={c.id} value={c.id}>
                {c.id}
              </option>
            ))}
          </select>

          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              background: 'rgba(0,0,0,0.4)',
              border: '1px solid rgba(255,255,255,0.1)',
              borderRadius: '8px',
              padding: '4px 8px',
              gap: '6px',
            }}
          >
            <Search size={13} color="#94a3b8" />
            <input
              type="text"
              placeholder="Filter class (person, car)..."
              value={searchLabel}
              onChange={(e) => setSearchLabel(e.target.value)}
              style={{
                background: 'transparent',
                border: 'none',
                color: '#f8fafc',
                fontSize: '0.78rem',
                outline: 'none',
                width: '160px',
              }}
            />
          </div>
        </div>
      </div>

      <div className="glass" style={{ padding: '1rem', borderRadius: '12px' }}>
        <table style={{ width: '100%', borderCollapse: 'collapse', textAlign: 'left', fontSize: '0.82rem' }}>
          <thead>
            <tr style={{ borderBottom: '1px solid var(--v2-card-border)', color: '#94a3b8' }}>
              <th style={{ padding: '10px' }}>Time</th>
              <th style={{ padding: '10px' }}>Camera</th>
              <th style={{ padding: '10px' }}>Detected Class</th>
              <th style={{ padding: '10px' }}>Confidence</th>
              <th style={{ padding: '10px' }}>Bounding Box [X, Y, W, H]</th>
            </tr>
          </thead>
          <tbody>
            {filteredEvents.map((ev) => (
              <tr
                key={ev.id}
                style={{
                  borderBottom: '1px solid rgba(255,255,255,0.04)',
                }}
              >
                <td style={{ padding: '10px', color: '#64748b' }}>{ev.timestamp}</td>
                <td style={{ padding: '10px', fontWeight: 600, color: '#f1f5f9' }}>{ev.cameraId}</td>
                <td style={{ padding: '10px' }}>
                  <span
                    style={{
                      background: 'rgba(99,102,241,0.2)',
                      color: '#a5b4fc',
                      padding: '3px 8px',
                      borderRadius: '4px',
                      fontWeight: 600,
                      fontSize: '0.75rem',
                    }}
                  >
                    {ev.label}
                  </span>
                </td>
                <td style={{ padding: '10px', color: '#10b981', fontWeight: 600 }}>
                  {(ev.confidence * 100).toFixed(0)}%
                </td>
                <td style={{ padding: '10px', color: '#94a3b8', fontFamily: 'monospace' }}>
                  [{ev.box.join(', ')}]
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
};
