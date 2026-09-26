import { useRef, useState } from 'react'
import { Check, Minus, Plus, ZoomIn } from 'lucide-react'
import { useApp } from '../context/AppContext'
import { useClickOutside } from '../hooks/useClickOutside'
import {
  UI_SCALE_OPTIONS,
  canStepUIScale,
  normalizeUIScale,
  stepUIScale,
  uiScaleLabel,
} from '../lib/uiScale'
import type { Density } from '../types'

/**
 * Le zoom et la densité, dans la barre d'état.
 *
 * Ils vivaient dans l'onglet Apparence du profil, à quatre clics de l'écran
 * qu'on trouve trop petit. « C'est trop petit » est une demande qu'on formule en
 * regardant l'écran, donc le réglage se pose là où le regard est déjà, et le cran
 * courant s'affiche sans rien ouvrir.
 *
 * Les deux réglages sont dans le même menu parce qu'on ne sait pas toujours
 * lequel des deux on veut : agrandir tout, ou resserrer les espaces. Les avoir
 * côte à côte permet d'essayer plutôt que de choisir à l'avance.
 */

const DENSITIES: { id: Density; label: string; hint: string }[] = [
  { id: 'compact', label: 'Compacte', hint: 'Resserre les espaces, sans changer la taille du texte' },
  { id: 'standard', label: 'Standard', hint: 'Le réglage par défaut' },
  { id: 'comfortable', label: 'Confortable', hint: 'Aère les espaces' },
]

export function DisplayScaleMenu() {
  const { settings, updateSettings } = useApp()
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)
  useClickOutside(ref, () => setOpen(false), open)

  const scale = normalizeUIScale(settings.uiScale)
  const density = settings.density || 'standard'

  const apply = (next: number) => {
    if (next !== scale) updateSettings({ uiScale: next })
  }

  return (
    <div ref={ref} className="relative">
      <button
        type="button"
        onClick={() => setOpen(prev => !prev)}
        aria-expanded={open}
        aria-haspopup="menu"
        title="Zoom et densité de l'interface"
        className="flex items-center gap-1 font-mono hover:text-[var(--text-primary)] transition-colors cursor-pointer"
      >
        <ZoomIn size={12} />
        {uiScaleLabel(scale)}
      </button>

      {open && (
        <div
          role="menu"
          /* Ancré en bas à droite : la barre d'état est la dernière ligne de la
             fenêtre, un menu qui descendrait sortirait de l'écran. */
          className="absolute bottom-full right-0 mb-2 w-60 p-2.5 rounded-xl border border-[var(--border-color)] bg-[var(--bg-secondary)] shadow-lg z-50 flex flex-col gap-2.5"
        >
          <div>
            <div className="text-[10px] font-bold uppercase tracking-[.08em] text-[var(--text-muted)] mb-1.5">
              Zoom
            </div>
            <div className="flex items-center gap-1.5">
              <button
                type="button"
                onClick={() => apply(stepUIScale(scale, 'down'))}
                disabled={!canStepUIScale(scale, 'down')}
                aria-label="Réduire"
                title="Réduire"
                className="p-1 rounded-lg border border-[var(--border-color)] bg-[var(--bg-tertiary)] text-[var(--text-secondary)] hover:text-[var(--text-primary)] disabled:opacity-40 disabled:cursor-default cursor-pointer"
              >
                <Minus size={12} />
              </button>
              <span className="flex-1 text-center text-xs font-mono font-bold text-[var(--text-primary)]">
                {uiScaleLabel(scale)}
              </span>
              <button
                type="button"
                onClick={() => apply(stepUIScale(scale, 'up'))}
                disabled={!canStepUIScale(scale, 'up')}
                aria-label="Agrandir"
                title="Agrandir"
                className="p-1 rounded-lg border border-[var(--border-color)] bg-[var(--bg-tertiary)] text-[var(--text-secondary)] hover:text-[var(--text-primary)] disabled:opacity-40 disabled:cursor-default cursor-pointer"
              >
                <Plus size={12} />
              </button>
            </div>
            {/* Tous les crans restent atteignables d'un clic : le pas sert à
                tâtonner, la liste à sauter là où l'on sait déjà aller. */}
            <div className="grid grid-cols-4 gap-1 mt-1.5">
              {UI_SCALE_OPTIONS.map(option => (
                <button
                  key={option}
                  type="button"
                  onClick={() => apply(option)}
                  className={`px-1 py-1 rounded-lg text-[10px] font-mono font-bold border transition-colors cursor-pointer ${
                    option === scale
                      ? 'bg-[var(--accent-light)] border-[var(--accent-color)] accent-text'
                      : 'bg-[var(--bg-tertiary)] border-[var(--border-color)] text-[var(--text-secondary)] hover:text-[var(--text-primary)]'
                  }`}
                >
                  {option}
                </button>
              ))}
            </div>
          </div>

          <div className="pt-2 border-t border-[var(--border-color)]">
            <div className="text-[10px] font-bold uppercase tracking-[.08em] text-[var(--text-muted)] mb-1.5">
              Densité
            </div>
            <div className="flex flex-col gap-0.5">
              {DENSITIES.map(option => (
                <button
                  key={option.id}
                  type="button"
                  onClick={() => {
                    if (option.id !== density) updateSettings({ density: option.id })
                  }}
                  title={option.hint}
                  className={`flex items-center gap-1.5 px-2 py-1 rounded-lg text-[11px] text-left transition-colors cursor-pointer ${
                    option.id === density
                      ? 'bg-[var(--accent-light)] accent-text font-bold'
                      : 'text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-tertiary)]'
                  }`}
                >
                  <Check size={11} className={option.id === density ? '' : 'opacity-0'} />
                  {option.label}
                </button>
              ))}
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
