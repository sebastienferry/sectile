import React, { useEffect, useState } from 'react'
import { Cpu, Plus, X } from 'lucide-react'
import type { AIProvider } from '../types'
import { DEFAULT_PROVIDER_MODELS, isValidModel, providerTakesModel } from '../lib/aiModels'
import { useApp } from '../context/AppContext'

interface ProviderModelsFieldProps {
  /** Moteur du profil : celui que l'éditeur propose en premier. */
  provider: AIProvider | ''
  /** Moteurs éditables, dans l'ordre d'affichage. */
  providers: AIProvider[]
  /** Listes configurées, tous moteurs confondus. */
  value: Record<string, string[]>
  onChange: (value: Record<string, string[]>) => void
  label?: string
}

/**
 * Éditeur de la liste de modèles d'un moteur. C'est cette liste que les
 * surfaces de lancement proposent : un modèle absent d'ici ne peut pas être
 * choisi au lancement, alors que les champs de configuration acceptent, eux,
 * n'importe quel identifiant.
 *
 * Un moteur sans liste propre affiche celle livrée avec Sectile et ne la
 * persiste pas : la retoucher est ce qui crée la liste du moteur.
 */
export const ProviderModelsField: React.FC<ProviderModelsFieldProps> = ({ provider, providers, value, onChange, label }) => {
  const { t } = useApp()
  const [draft, setDraft] = useState('')
  // Le moteur édité suit celui du profil par défaut, mais s'en détache : un
  // projet peut tourner sur un autre moteur, et sa liste doit être atteignable
  // sans changer le moteur global.
  const [edited, setEdited] = useState<AIProvider | ''>(provider)
  useEffect(() => setEdited(provider), [provider])

  const target = (edited || provider) as AIProvider | ''
  if (!target) return null

  const shipped = DEFAULT_PROVIDER_MODELS[target] || []
  // Une liste vide est conservée telle quelle : c'est la façon de dire « aucun
  // modèle pour ce moteur », et elle doit survivre à l'enregistrement.
  const configured = value[target]
  const models = configured ?? shipped
  const draftIsValid = isValidModel(draft)
  const canAdd = draft.trim() !== '' && draftIsValid && !models.includes(draft.trim())

  const write = (next: string[]) => onChange({ ...value, [target]: next })

  const add = () => {
    if (!canAdd) return
    write([...models, draft.trim()])
    setDraft('')
  }

  return (
    <div className="space-y-2.5 p-4 rounded-xl bg-[var(--bg-tertiary)]/70 border border-[var(--border-color)]">
      <label className="block text-[11px] font-bold uppercase tracking-wider text-[var(--text-primary)] flex items-center gap-1.5">
        <Cpu size={13} className="text-indigo-400" />
        <span>{label || t?.profileModal?.ai?.proposedModelsFor || 'Modèles proposés pour'}</span>
        <select
          value={target}
          onChange={e => {
            setEdited(e.target.value as AIProvider)
            setDraft('')
          }}
          aria-label="Moteur dont la liste est éditée"
          className="px-1.5 py-0.5 rounded-lg text-[11px] font-bold uppercase tracking-wider bg-[var(--bg-primary)] text-[var(--text-primary)] border border-[var(--border-color)] focus:outline-none focus:border-[var(--accent-color)] cursor-pointer">
          {providers.map(id => (
            <option key={id} value={id}>
              {id.toUpperCase()}
            </option>
          ))}
        </select>
      </label>

      {!providerTakesModel(target) && (
        <p className="text-[10.5px] text-[var(--text-muted)] leading-relaxed">
          {t?.profileModal?.ai?.engineIgnoresModelNotice ? (
            t.profileModal.ai.engineIgnoresModelNotice.split('{model}').map((part, i, arr) => (
              <React.Fragment key={i}>
                {part}
                {i < arr.length - 1 && <code className="text-indigo-400 font-mono">{'{model}'}</code>}
              </React.Fragment>
            ))
          ) : (
            <>
              Ce moteur ignore le modèle sauf si sa commande porte le marqueur <code className="text-indigo-400 font-mono">{'{model}'}</code>.
            </>
          )}
        </p>
      )}

      <div className="flex flex-wrap gap-1.5">
        {models.map(model => {
          const removeLabel = t?.profileModal?.ai?.removeModelAria
            ? t.profileModal.ai.removeModelAria.replace('{model}', model).replace('{target}', target.toUpperCase())
            : `Retirer ${model} de ${target}`
          return (
            <span
              key={model}
              className="inline-flex items-center gap-1 pl-2.5 pr-1 py-1 rounded-lg bg-[var(--bg-primary)] border border-[var(--border-color)] text-[11px] font-mono text-[var(--text-primary)]">
              {model}
              <button
                type="button"
                onClick={() => write(models.filter(kept => kept !== model))}
                className="p-0.5 rounded text-[var(--text-muted)] hover:text-red-400 cursor-pointer"
                title={removeLabel}
                aria-label={removeLabel}>
                <X size={11} />
              </button>
            </span>
          )
        })}
        {models.length === 0 && (
          <span className="text-[10.5px] text-[var(--text-muted)]">
            {t?.profileModal?.ai?.noModelsConfigured || 'Aucun modèle : rien ne sera proposé au lancement.'}
          </span>
        )}
      </div>

      <div className="flex items-center gap-1.5">
        <input
          type="text"
          value={draft}
          onChange={e => setDraft(e.target.value)}
          onKeyDown={e => {
            if (e.key === 'Enter') {
              e.preventDefault()
              add()
            }
          }}
          placeholder={t?.profileModal?.ai?.addModelPlaceholder || 'Ajouter un modèle...'}
          aria-label={
            t?.profileModal?.ai?.addModelAria
              ? t.profileModal.ai.addModelAria.replace('{target}', target.toUpperCase())
              : `Ajouter un modèle pour ${target}`
          }
          aria-invalid={!draftIsValid}
          className={`flex-1 px-3 py-2 text-xs font-mono rounded-xl bg-[var(--bg-primary)] border text-[var(--text-primary)] focus:outline-none transition-all ${
            draftIsValid ? 'border-[var(--border-color)] focus:border-[var(--accent-color)]' : 'border-red-500 focus:border-red-500'
          }`}
        />
        <button
          type="button"
          onClick={add}
          disabled={!canAdd}
          className="p-2 rounded-xl bg-[var(--bg-primary)] border border-[var(--border-color)] text-[var(--text-primary)] disabled:opacity-40 disabled:cursor-not-allowed cursor-pointer"
          title={t?.profileModal?.ai?.addModelButton || 'Ajouter ce modèle'}
          aria-label={t?.profileModal?.ai?.addModelButton || 'Ajouter ce modèle'}>
          <Plus size={13} />
        </button>
      </div>

      {!draftIsValid && (
        <p className="text-[10.5px] text-red-400 leading-relaxed">
          {t?.profileModal?.ai?.invalidModelIdentifier || 'Identifiant invalide : lettres, chiffres et . _ - : @ / uniquement, sans espace.'}
        </p>
      )}
    </div>
  )
}
