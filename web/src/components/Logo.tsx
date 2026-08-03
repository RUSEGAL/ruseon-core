import React from 'react';

export const Logo: React.FC<{ size?: number; className?: string; style?: React.CSSProperties }> = ({ 
  size = 28, 
  className = '',
  style
}) => (
  <svg 
    xmlns="http://www.w3.org/2000/svg" 
    viewBox="0 0 100 100" 
    width={size} 
    height={size} 
    className={className}
    style={{ display: 'inline-block', flexShrink: 0, ...style }}
  >
    <defs>
      <linearGradient id="reaGrad" x1="0%" y1="0%" x2="100%" y2="100%">
        <stop offset="0%" style={{ stopColor: 'var(--primary)', stopOpacity: 1 }} />
        <stop offset="100%" style={{ stopColor: '#4338ca', stopOpacity: 1 }} />
      </linearGradient>
    </defs>
    {/* Outer ring / Engine */}
    <path 
      d="M50 10 a40 40 0 1 0 0 80 a40 40 0 1 0 0 -80" 
      fill="none" 
      stroke="url(#reaGrad)" 
      strokeWidth="6" 
      strokeDasharray="60 15" 
      strokeLinecap="round" 
    />
    {/* Inner Play */}
    <polygon points="35,35 60,50 35,65" fill="url(#reaGrad)" />
    
    {/* Stream Waves */}
    <path d="M68 35 Q80 50 68 65" fill="none" stroke="var(--primary)" strokeWidth="5" strokeLinecap="round" />
    <path d="M78 28 Q95 50 78 72" fill="none" stroke="#4338ca" strokeWidth="5" strokeLinecap="round" />
    <path d="M22 40 L22 60" fill="none" stroke="var(--primary)" strokeWidth="5" strokeLinecap="round" opacity="0.6" />
  </svg>
);
