import React, { useState } from 'react'
import { useApp } from '../context/AppContext'
import { format } from '../lib/i18n'

/**
 * Visage d'une personne, avec ses initiales en repli.
 *
 * La photo vient du tracker et son adresse est publique, mais elle peut manquer,
 * expirer, ou être bloquée par le réseau de l'utilisateur. Les initiales prennent
 * alors le relais : c'est ce qui était affiché avant, et une carte ne doit jamais
 * se retrouver avec un carré vide à la place de son assigné.
 */
export const Avatar: React.FC<{
  name: string
  url?: string
  /** Diamètre en pixels. Les cartes en veulent 18, une liste 20, une fiche 24. */
  size?: number
  title?: string
  className?: string
}> = ({ name, url, size = 20, title, className }) => {
  const { t } = useApp()
  const [failed, setFailed] = useState(false)
  const initials = (name || '?').trim().substring(0, 2).toUpperCase()
  const label = title || (name ? format(t.shell.avatar.assignedTo, { name }) : t.shell.avatar.unassigned)

  if (url && !failed) {
    return (
      <img
        src={url}
        alt=""
        title={label}
        width={size}
        height={size}
        loading="lazy"
        onError={() => setFailed(true)}
        style={{ width: size, height: size }}
        className={`rounded-full shrink-0 object-cover bg-[var(--bg-tertiary)] ${className || ''}`}
      />
    )
  }

  return (
    <div
      title={label}
      style={{ width: size, height: size, fontSize: Math.max(8, Math.round(size * 0.42)) }}
      className={`rounded-full accent-bg text-white flex items-center justify-center font-bold shadow-2xs shrink-0 ${className || ''}`}
    >
      {initials}
    </div>
  )
}
