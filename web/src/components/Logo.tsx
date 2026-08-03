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
    
    {/* Streams bursting inward from the left */}
    <path d="M10 50 L35 50" fill="none" stroke="url(#reaGrad)" strokeWidth="6" strokeLinecap="round" />
    <path d="M18 35 Q 28 35 38 42" fill="none" stroke="#818cf8" strokeWidth="5" strokeLinecap="round" />
    <path d="M18 65 Q 28 65 38 58" fill="none" stroke="#818cf8" strokeWidth="5" strokeLinecap="round" />

    {/* Inner Play */}
    <polygon points="45,30 75,50 45,70" fill="url(#reaGrad)" />
  </svg>
);
