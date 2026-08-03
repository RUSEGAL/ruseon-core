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
      d="M50 5 a45 45 0 1 0 0 90 a45 45 0 1 0 0 -90" 
      fill="none" 
      stroke="url(#reaGrad)" 
      strokeWidth="8" 
      strokeDasharray="70 20" 
      strokeLinecap="round" 
    />
    {/* Inner Play / Stream */}
    <polygon points="40,30 70,50 40,70" fill="url(#reaGrad)" />
    <path d="M30 35 L30 65" stroke="var(--primary)" strokeWidth="8" strokeLinecap="round" />
  </svg>
);
