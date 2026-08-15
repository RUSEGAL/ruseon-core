import React, { useState, useRef, useEffect } from 'react';
import { ChevronDown, Check, Zap, Cpu, Radio, Image } from 'lucide-react';
import {
  PROTOCOL_OPTIONS,
  globalPlayerOrchestrator,
} from '../../core/orchestrator';
import type { StreamingProtocol } from '../../core/orchestrator';

interface ProtocolSwitcherProps {
  cameraId: string;
  activeProtocol: StreamingProtocol;
  cameraCodec?: string;
  onProtocolChange?: (protocol: StreamingProtocol) => void;
}

export const ProtocolSwitcher: React.FC<ProtocolSwitcherProps> = ({
  cameraId,
  activeProtocol,
  onProtocolChange,
}) => {
  const [isOpen, setIsOpen] = useState(false);
  const dropdownRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const handleOutsideClick = (e: MouseEvent) => {
      if (dropdownRef.current && !dropdownRef.current.contains(e.target as Node)) {
        setIsOpen(false);
      }
    };
    document.addEventListener('mousedown', handleOutsideClick);
    return () => document.removeEventListener('mousedown', handleOutsideClick);
  }, []);

  const handleSelect = (p: StreamingProtocol) => {
    globalPlayerOrchestrator.setOverride(cameraId, p === 'auto' ? null : p);
    if (onProtocolChange) {
      onProtocolChange(p);
    }
    setIsOpen(false);
  };

  const getIcon = (id: StreamingProtocol) => {
    switch (id) {
      case 'webrtc':
        return <Zap size={12} color="#fbbf24" />;
      case 'webcodecs':
        return <Cpu size={12} color="#38bdf8" />;
      case 'hls':
        return <Radio size={12} color="#10b981" />;
      case 'snapshot':
        return <Image size={12} color="#94a3b8" />;
      default:
        return <Zap size={12} color="#818cf8" />;
    }
  };

  const getBadgeStyle = (id: StreamingProtocol) => {
    switch (id) {
      case 'webrtc':
        return { bg: 'rgba(245, 158, 11, 0.15)', border: 'rgba(245, 158, 11, 0.3)', color: '#fcd34d' };
      case 'hls':
        return { bg: 'rgba(16, 185, 129, 0.15)', border: 'rgba(16, 185, 129, 0.3)', color: '#6ee7b7' };
      case 'webcodecs':
        return { bg: 'rgba(56, 189, 248, 0.15)', border: 'rgba(56, 189, 248, 0.3)', color: '#7dd3fc' };
      case 'snapshot':
        return { bg: 'rgba(100, 116, 139, 0.2)', border: 'rgba(100, 116, 139, 0.3)', color: '#cbd5e1' };
      default:
        return { bg: 'rgba(99, 102, 241, 0.2)', border: 'rgba(99, 102, 241, 0.35)', color: '#a5b4fc' };
    }
  };

  const badge = getBadgeStyle(activeProtocol);

  return (
    <div ref={dropdownRef} style={{ position: 'relative', display: 'inline-block' }}>
      <button
        onClick={(e) => {
          e.stopPropagation();
          setIsOpen(!isOpen);
        }}
        style={{
          background: badge.bg,
          backdropFilter: 'blur(8px)',
          WebkitBackdropFilter: 'blur(8px)',
          border: `1px solid ${badge.border}`,
          borderRadius: '6px',
          color: badge.color,
          padding: '2px 7px',
          fontSize: '0.68rem',
          fontWeight: 700,
          display: 'flex',
          alignItems: 'center',
          gap: '4px',
          cursor: 'pointer',
          transition: 'all 0.15s ease',
        }}
        title="Click to switch live stream protocol (WebRTC / HLS / Auto)"
      >
        {getIcon(activeProtocol)}
        <span style={{ textTransform: 'uppercase', letterSpacing: '0.3px' }}>{activeProtocol}</span>
        <ChevronDown size={11} style={{ opacity: 0.8 }} />
      </button>

      {/* Dropdown Menu opens DOWNWARDS from the top header */}
      {isOpen && (
        <div
          style={{
            position: 'absolute',
            top: '100%',
            right: 0,
            marginTop: '6px',
            background: 'rgba(10, 14, 23, 0.95)',
            backdropFilter: 'blur(16px)',
            WebkitBackdropFilter: 'blur(16px)',
            border: '1px solid rgba(255, 255, 255, 0.15)',
            borderRadius: '10px',
            padding: '6px',
            width: '220px',
            boxShadow: '0 12px 32px rgba(0, 0, 0, 0.85), 0 0 0 1px rgba(99, 102, 241, 0.2)',
            zIndex: 150,
            display: 'flex',
            flexDirection: 'column',
            gap: '3px',
          }}
          onClick={(e) => e.stopPropagation()}
        >
          <div
            style={{
              padding: '4px 8px',
              fontSize: '0.66rem',
              fontWeight: 700,
              color: '#94a3b8',
              textTransform: 'uppercase',
              letterSpacing: '0.5px',
            }}
          >
            Streaming Protocol
          </div>
          {PROTOCOL_OPTIONS.map((opt) => {
            const isSelected = activeProtocol === opt.id;
            return (
              <button
                key={opt.id}
                onClick={() => handleSelect(opt.id)}
                style={{
                  background: isSelected ? 'rgba(99, 102, 241, 0.25)' : 'transparent',
                  border: isSelected ? '1px solid rgba(99, 102, 241, 0.4)' : '1px solid transparent',
                  borderRadius: '6px',
                  padding: '6px 8px',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'space-between',
                  cursor: 'pointer',
                  color: isSelected ? '#f8fafc' : '#cbd5e1',
                  textAlign: 'left',
                  fontSize: '0.76rem',
                  transition: 'all 0.15s ease',
                }}
              >
                <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                  {getIcon(opt.id)}
                  <div>
                    <div style={{ fontWeight: 600 }}>{opt.label}</div>
                    <div style={{ fontSize: '0.64rem', color: '#94a3b8' }}>
                      Latency: {opt.latency}
                    </div>
                  </div>
                </div>
                {isSelected && <Check size={13} color="#818cf8" />}
              </button>
            );
          })}
        </div>
      )}
    </div>
  );
};
