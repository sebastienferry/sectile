import React from 'react'

interface SectileLogoProps {
  size?: number
  className?: string
  withBadge?: boolean
}

/**
 * SectileLogo - Official logo representing continuous, intelligent task flow streams.
 * Designed for crisp vector scaling and high contrast in both Dark and Light themes.
 */
export const SectileLogo: React.FC<SectileLogoProps> = ({
  size = 32,
  className = '',
  withBadge = true,
}) => {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 36 36"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      className={className}
      aria-label="Sectile Logo"
    >
      <defs>
        {/* Deep Tech Badge Gradient */}
        <linearGradient id="sectile-badge-grad" x1="0%" y1="0%" x2="100%" y2="100%">
          <stop offset="0%" stopColor="#1e1b4b" />
          <stop offset="50%" stopColor="#0f172a" />
          <stop offset="100%" stopColor="#020617" />
        </linearGradient>

        {/* Primary Stream Gradient (Indigo -> Violet -> Cyan) */}
        <linearGradient id="sectile-stream-primary" x1="0%" y1="0%" x2="100%" y2="100%">
          <stop offset="0%" stopColor="#6366f1" />
          <stop offset="50%" stopColor="#8b5cf6" />
          <stop offset="100%" stopColor="#06b6d4" />
        </linearGradient>

        {/* Secondary Stream Gradient */}
        <linearGradient id="sectile-stream-secondary" x1="0%" y1="100%" x2="100%" y2="0%">
          <stop offset="0%" stopColor="#38bdf8" />
          <stop offset="100%" stopColor="#818cf8" />
        </linearGradient>

        {/* Node Glow Gradient */}
        <linearGradient id="sectile-glow" x1="0%" y1="0%" x2="100%" y2="100%">
          <stop offset="0%" stopColor="#a5b4fc" />
          <stop offset="100%" stopColor="#38bdf8" />
        </linearGradient>
      </defs>

      {/* Optional Badge Container */}
      {withBadge && (
        <>
          <rect
            width="36"
            height="36"
            rx="9"
            fill="url(#sectile-badge-grad)"
          />
          <rect
            x="0.5"
            y="0.5"
            width="35"
            height="35"
            rx="8.5"
            stroke="#6366f1"
            strokeOpacity="0.4"
            strokeWidth="1"
          />
        </>
      )}

      {/* Dynamic Flow Streams */}
      {/* Top Stream */}
      <path
        d="M8.5 12C8.5 12 13 10 18 12C23 14 27.5 12 27.5 12"
        stroke="url(#sectile-stream-primary)"
        strokeWidth="2.5"
        strokeLinecap="round"
      />

      {/* Middle Connecting Pulse */}
      <path
        d="M10 18C13 18 16 16.5 18 16.5C21 16.5 24 19.5 27 19.5"
        stroke="url(#sectile-stream-secondary)"
        strokeWidth="2.2"
        strokeLinecap="round"
      />

      {/* Bottom Acceleration Stream */}
      <path
        d="M12 24C15 24 18 25.5 21 25.5C23.5 25.5 26 24 26 24"
        stroke="url(#sectile-stream-primary)"
        strokeWidth="2.5"
        strokeLinecap="round"
      />

      {/* Flow Nodes */}
      <circle cx="8.5" cy="12" r="2" fill="url(#sectile-glow)" />
      <circle cx="18" cy="16.5" r="2.2" fill="#38bdf8" />
      <circle cx="27.5" cy="12" r="2" fill="#a855f7" />
      <circle cx="26" cy="24" r="2.2" fill="url(#sectile-glow)" />
      <circle cx="12" cy="24" r="1.8" fill="#818cf8" />
    </svg>
  )
}
