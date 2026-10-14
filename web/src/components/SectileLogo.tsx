import React from 'react'

interface SectileLogoProps {
  size?: number
  className?: string
  withBadge?: boolean
}

/**
 * SectileLogo - Official logo representing Opus Sectile interlocking geometric facets
 * slicing complex work into structured, cohesive tasks.
 * Adapts dynamically to Dark and Light themes via CSS custom properties.
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
        {/* Theme-Adaptive Badge Gradient */}
        <linearGradient id="sectile-badge-grad" x1="0%" y1="0%" x2="100%" y2="100%">
          <stop offset="0%" stopColor="#1e1b4b" style={{ stopColor: 'var(--logo-badge-start, #1e1b4b)' }} />
          <stop offset="50%" stopColor="#0f172a" style={{ stopColor: 'var(--logo-badge-mid, #0f172a)' }} />
          <stop offset="100%" stopColor="#020617" style={{ stopColor: 'var(--logo-badge-end, #020617)' }} />
        </linearGradient>

        {/* Central Keystone Facet Gradient (Violet -> Indigo -> Cyan) */}
        <linearGradient id="sectile-stream-primary" x1="0%" y1="0%" x2="100%" y2="100%">
          <stop offset="0%" stopColor="#a855f7" style={{ stopColor: 'var(--logo-keystone-start, #a855f7)' }} />
          <stop offset="50%" stopColor="#6366f1" style={{ stopColor: 'var(--logo-keystone-mid, #6366f1)' }} />
          <stop offset="100%" stopColor="#06b6d4" style={{ stopColor: 'var(--logo-keystone-end, #06b6d4)' }} />
        </linearGradient>

        {/* Upper Arch Facet Gradient (Sky -> Indigo) */}
        <linearGradient id="sectile-stream-secondary" x1="0%" y1="0%" x2="100%" y2="100%">
          <stop offset="0%" stopColor="#38bdf8" style={{ stopColor: 'var(--logo-arch-top-start, #38bdf8)' }} />
          <stop offset="100%" stopColor="#6366f1" style={{ stopColor: 'var(--logo-arch-top-end, #6366f1)' }} />
        </linearGradient>

        {/* Lower Arch Facet Gradient (Indigo -> Violet) */}
        <linearGradient id="sectile-facet-bot" x1="0%" y1="0%" x2="100%" y2="100%">
          <stop offset="0%" stopColor="#6366f1" style={{ stopColor: 'var(--logo-arch-bot-start, #6366f1)' }} />
          <stop offset="100%" stopColor="#a855f7" style={{ stopColor: 'var(--logo-arch-bot-end, #a855f7)' }} />
        </linearGradient>

        {/* Terminal Inlay Facet Glow Gradient */}
        <linearGradient id="sectile-glow" x1="0%" y1="0%" x2="100%" y2="100%">
          <stop offset="0%" stopColor="#7dd3fc" style={{ stopColor: 'var(--logo-inlay-start, #7dd3fc)' }} />
          <stop offset="100%" stopColor="#38bdf8" style={{ stopColor: 'var(--logo-inlay-end, #38bdf8)' }} />
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
            style={{
              stroke: 'var(--logo-badge-stroke, #6366f1)',
              strokeOpacity: 'var(--logo-badge-stroke-opacity, 0.4)',
            }}
          />
        </>
      )}

      {/* Opus Sectile Geometric Facets (5 interlocking cut tiles forming a hexagonal S mosaic) */}
      {/* Top-Right Terminal Inlay Facet */}
      <path
        d="M 22.85 8.45 Q 23.41 8.13 23.97 8.45 L 27.84 10.68 Q 28.40 11.00 28.40 11.65 L 28.40 13.85 Q 28.40 14.50 27.81 14.23 L 20.04 10.68 Q 19.45 10.41 20.01 10.09 Z"
        fill="url(#sectile-glow)"
      />

      {/* Upper-Left Arch Facet */}
      <path
        d="M 7.60 11.65 Q 7.60 11.00 8.16 10.68 L 17.44 5.32 Q 18.00 5.00 18.56 5.33 L 21.59 7.07 Q 22.15 7.40 21.59 7.72 L 8.16 15.48 Q 7.60 15.80 7.60 15.15 Z"
        fill="url(#sectile-stream-secondary)"
      />

      {/* Central Diagonal Keystone Facet */}
      <path
        d="M 7.60 17.70 Q 7.60 17.25 7.99 17.03 L 13.50 13.85 Q 14.00 13.56 14.56 13.71 L 27.97 17.24 Q 28.40 17.35 28.40 17.80 L 28.40 18.30 Q 28.40 18.75 28.01 18.97 L 22.50 22.15 Q 22.00 22.44 21.44 22.29 L 8.03 18.76 Q 7.60 18.65 7.60 18.20 Z"
        fill="url(#sectile-stream-primary)"
      />

      {/* Lower-Right Arch Facet */}
      <path
        d="M 28.40 24.35 Q 28.40 25.00 27.84 25.32 L 18.56 30.68 Q 18.00 31.00 17.44 30.67 L 14.41 28.93 Q 13.85 28.60 14.41 28.28 L 27.84 20.52 Q 28.40 20.20 28.40 20.85 Z"
        fill="url(#sectile-facet-bot)"
      />

      {/* Bottom-Left Terminal Inlay Facet */}
      <path
        d="M 13.15 27.55 Q 12.59 27.87 12.03 27.55 L 8.16 25.32 Q 7.60 25.00 7.60 24.35 L 7.60 22.15 Q 7.60 21.50 8.19 21.77 L 15.96 25.32 Q 16.55 25.59 15.99 25.91 Z"
        fill="url(#sectile-glow)"
      />
    </svg>
  )
}
