import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { useClickOutside } from '../hooks/useClickOutside'
import { Loader2, Search, X } from 'lucide-react'
import { Avatar } from './Avatar'
import { useApp } from '../context/AppContext'

export interface LookupOption {
  id: string
  label: string
  /** Seconde ligne : l'e-mail d'une personne, le nombre de tickets d'une équipe. */
  sublabel?: string
  avatarUrl?: string
  /** Grisé sans être exclu : un compte désactivé reste choisissable, pour ne pas
   *  effacer en silence ce que le ticket porte déjà. */
  muted?: boolean
}

/**
 * Champ de recherche à choix unique, pour les valeurs qui vivent dans le tracker
 * et qu'une liste fermée ne peut pas contenir : les personnes de l'instance, ses
 * équipes. Sans frappe, la recherche renvoie le contexte le plus probable (les
 * membres de l'équipe du ticket, les équipes déjà sur le board) ; taper élargit à
 * l'instance entière, ce qui est la seule façon d'assigner quelqu'un hors équipe.
 */
export const LookupField: React.FC<{
  value: string
  onSearch: (query: string) => Promise<LookupOption[]>
  onPick: (option: LookupOption | null) => void
  placeholder?: string
  icon?: React.ReactNode
  allowClear?: boolean
  clearLabel?: string
  disabled?: boolean
  emptyHint?: string
}> = ({ value, onSearch, onPick, placeholder, icon, allowClear = true, clearLabel: clearLabelProp, disabled, emptyHint }) => {
  const { t } = useApp()
  const strings = t.taskDetail.lookups
  const clearLabel = clearLabelProp ?? strings.none
  const [isOpen, setIsOpen] = useState(false)
  const [query, setQuery] = useState('')
  const [options, setOptions] = useState<LookupOption[]>([])
  const [isSearching, setIsSearching] = useState(false)
  const [highlight, setHighlight] = useState(0)
  const wrapRef = useRef<HTMLDivElement>(null)
  const panelRef = useRef<HTMLDivElement>(null)
  const [panelBox, setPanelBox] = useState<{ top: number; left: number; width: number } | null>(null)

  /**
   * Le panneau est rendu dans un portail et positionné à la main. Dans un tableau
   * défilant, un panneau en position absolue est coupé par le conteneur : c'est
   * exactement le cas de la vue de triage, où chaque ligne porte deux champs de
   * recherche.
   *
   * Le zoom de l'interface doit être défait ici : getBoundingClientRect renvoie
   * des pixels déjà zoomés, alors que le panneau, placé dans le même document,
   * subira le zoom à son tour.
   */
  const measure = useCallback(() => {
    const anchor = wrapRef.current
    if (!anchor) return
    const rect = anchor.getBoundingClientRect()
    const zoomRaw = getComputedStyle(document.documentElement).getPropertyValue('--ui-zoom')
    const zoom = parseFloat(zoomRaw) || 1
    setPanelBox({
      top: (rect.bottom + 4) / zoom,
      left: rect.left / zoom,
      width: rect.width / zoom,
    })
  }, [])

  useEffect(() => {
    if (!isOpen) {
      setPanelBox(null)
      return
    }
    measure()
    // Le panneau suit son champ : une liste qui défile sous lui le laisserait
    // flotter à l'ancienne position.
    window.addEventListener('scroll', measure, true)
    window.addEventListener('resize', measure)
    return () => {
      window.removeEventListener('scroll', measure, true)
      window.removeEventListener('resize', measure)
    }
  }, [isOpen, measure])

  // Fermeture au clic extérieur : le champ vit dans une grille dense, un panneau
  // resté ouvert masque les champs voisins.
  // Le panneau est rendu dans un portail : il n'est pas dans l'ancre, et les
  // deux comptent comme « dedans ».
  const closePanel = useCallback(() => setIsOpen(false), [])
  const outsideTargets = useMemo(() => [wrapRef, panelRef], [])
  useClickOutside(outsideTargets, closePanel, isOpen)

  // La frappe est amortie : chaque caractère est sinon une requête au tracker.
  useEffect(() => {
    if (!isOpen) return
    let alive = true
    setIsSearching(true)
    const timer = setTimeout(() => {
      onSearch(query).then(list => {
        if (!alive) return
        setOptions(list)
        setHighlight(0)
        setIsSearching(false)
      })
    }, query ? 250 : 0)
    return () => {
      alive = false
      clearTimeout(timer)
    }
  }, [isOpen, query, onSearch])

  const rows = useMemo<(LookupOption | null)[]>(() => {
    const base: (LookupOption | null)[] = allowClear ? [null] : []
    return base.concat(options)
  }, [options, allowClear])

  const pick = (option: LookupOption | null) => {
    onPick(option)
    setIsOpen(false)
    setQuery('')
  }

  return (
    <div className="relative" ref={wrapRef}>
      <div className="relative">
        <input
          type="text"
          value={isOpen ? query : value}
          disabled={disabled}
          placeholder={placeholder}
          onFocus={() => {
            if (disabled) return
            setQuery('')
            setIsOpen(true)
          }}
          onChange={e => setQuery(e.target.value)}
          onKeyDown={e => {
            if (!isOpen) return
            if (e.key === 'ArrowDown') {
              e.preventDefault()
              setHighlight(h => Math.min(rows.length - 1, h + 1))
            } else if (e.key === 'ArrowUp') {
              e.preventDefault()
              setHighlight(h => Math.max(0, h - 1))
            } else if (e.key === 'Enter') {
              e.preventDefault()
              if (rows.length > 0) pick(rows[Math.min(highlight, rows.length - 1)])
            } else if (e.key === 'Escape') {
              e.preventDefault()
              setIsOpen(false)
            }
          }}
          className="w-full pl-7 pr-6 py-1.5 text-xs rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)] disabled:opacity-50"
        />
        <span className="absolute left-2.5 top-2.5 text-[var(--text-muted)] pointer-events-none">
          {isOpen ? <Search size={12} /> : icon}
        </span>
        {value && !isOpen && allowClear && !disabled && (
          <button
            type="button"
            onClick={() => pick(null)}
            title={clearLabel}
            className="absolute right-1.5 top-1.5 p-0.5 rounded text-[var(--text-muted)] hover:text-rose-400 cursor-pointer"
          >
            <X size={12} />
          </button>
        )}
      </div>

      {isOpen && panelBox && createPortal(
        <div
          ref={panelRef}
          style={{ position: 'fixed', top: panelBox.top, left: panelBox.left, width: panelBox.width, zIndex: 90 }}
          className="max-h-[240px] overflow-auto rounded-xl bg-[var(--bg-secondary)] border border-[var(--border-color)] shadow-lg">
          {isSearching && options.length === 0 ? (
            <div className="flex items-center gap-2 px-2.5 py-2 text-[11px] text-[var(--text-muted)]">
              <Loader2 size={12} className="animate-spin" />
              {strings.searching}
            </div>
          ) : rows.length === 0 ? (
            <div className="px-2.5 py-2 text-[11px] text-[var(--text-muted)]">{emptyHint || strings.noResults}</div>
          ) : (
            rows.map((option, index) => (
              <button
                key={option?.id || '__clear__'}
                type="button"
                onMouseEnter={() => setHighlight(index)}
                onClick={() => pick(option)}
                className={`w-full flex items-center gap-2 px-2.5 py-1.5 text-left cursor-pointer ${
                  index === highlight ? 'bg-[var(--bg-tertiary)]' : ''
                }`}
              >
                {option?.avatarUrl ? (
                  <Avatar name={option.label} url={option.avatarUrl} size={16} title={option.label} />
                ) : (
                  <span className="w-4 h-4 shrink-0" />
                )}
                <span className="min-w-0 flex-1">
                  <span
                    className={`block text-[11px] truncate ${
                      option ? 'text-[var(--text-primary)]' : 'text-[var(--text-muted)] italic'
                    } ${option?.muted ? 'opacity-60' : ''}`}
                  >
                    {option ? option.label : clearLabel}
                  </span>
                  {option?.sublabel && (
                    <span className="block text-[9px] text-[var(--text-muted)] truncate">{option.sublabel}</span>
                  )}
                </span>
              </button>
            ))
          )}
        </div>,
        document.body
      )}
    </div>
  )
}
