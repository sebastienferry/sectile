import React, { useState } from 'react'
import { Cpu, Info, X } from 'lucide-react'
import type { AIProvider } from '../types'
import { isValidModel, providerModels, providerTakesModel, templateGovernsCommand } from '../lib/aiModels'
import { useApp } from '../context/AppContext'

interface AIModelFieldProps {
  provider: AIProvider | ''
  commandTemplate: string
  value: string
  onChange: (value: string) => void
  /** Texte affiché quand le champ est vide : « défaut du CLI » ou « réglage global ». */
  placeholder: string
  label: string
  /** Modèles proposés pour ce moteur (si omis, utilise la configuration globale). */
  availableModels?: string[]
}

/**
 * Model picker: suggestions for the selected engine, free text for everything
 * else. The notices state the two cases where the value does not reach the
 * command line, rather than hiding the field and leaving the user guessing.
 */
export const AIModelField: React.FC<AIModelFieldProps> = ({
  provider,
  commandTemplate,
  value,
  onChange,
  placeholder,
  label,
  availableModels,
}) => {
  const { settings, t } = useApp()
  const listID = `ai-model-suggestions-${provider || 'none'}`
  // The suggestions are the models configured for this provider: the same list
  // the launch surfaces offer, so adding one there makes it available here too.
  const suggestions = availableModels ?? providerModels(settings, provider)
  const templateWins = templateGovernsCommand(provider, commandTemplate)
  const ignored = !providerTakesModel(provider)
  const invalid = !isValidModel(value)

  const isCustomValue = value !== '' && !suggestions.includes(value)
  const [isCustomMode, setIsCustomMode] = useState(false)
  const showCustomInput = suggestions.length === 0 || isCustomMode || isCustomValue

  const customLabel = t?.profileModal?.ai?.customModelOption || 'Autre modèle (saisie libre)...'
  const quickSelectLabel = t?.profileModal?.ai?.quickSelect || 'Sélection rapide :'

  return (
    <div className="space-y-1.5">
      <label className="block text-[11px] font-bold uppercase tracking-wider text-[var(--text-muted)] flex items-center gap-1.5">
        <Cpu size={13} className="text-indigo-400" />
        <span>{label}</span>
      </label>

      {suggestions.length > 0 && (
        <div className="space-y-2">
          <select
            value={suggestions.includes(value) ? value : value === '' ? '' : '__custom__'}
            onChange={e => {
              const val = e.target.value
              if (val === '__custom__') {
                setIsCustomMode(true)
              } else {
                setIsCustomMode(false)
                onChange(val)
              }
            }}
            aria-label={label}
            className="w-full px-3 py-2 text-xs font-mono rounded-xl bg-[var(--bg-primary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)] cursor-pointer"
          >
            <option value="">{placeholder}</option>
            <optgroup label={t?.profileModal?.ai?.proposedModelsGroup || 'Modèles proposés'}>
              {suggestions.map(model => (
                <option key={model} value={model}>
                  {model}
                </option>
              ))}
            </optgroup>
            <option value="__custom__">
              {isCustomValue ? `Personnalisé : ${value}` : customLabel}
            </option>
          </select>

          <div className="flex flex-wrap items-center gap-1.5">
            <span className="text-[10.5px] text-[var(--text-muted)]">{quickSelectLabel}</span>
            {suggestions.map(model => {
              const isSelected = value === model
              return (
                <button
                  key={model}
                  type="button"
                  onClick={() => {
                    setIsCustomMode(false)
                    onChange(isSelected ? '' : model)
                  }}
                  className={`px-2 py-0.5 rounded-lg text-[11px] font-mono cursor-pointer transition-all ${
                    isSelected
                      ? 'bg-indigo-500/20 text-indigo-300 border border-indigo-500/50 font-semibold shadow-xs'
                      : 'bg-[var(--bg-primary)] text-[var(--text-secondary)] border border-[var(--border-color)] hover:border-[var(--text-muted)] hover:text-[var(--text-primary)]'
                  }`}
                  title={isSelected ? 'Modèle actif (cliquer pour désélectionner)' : `Définir ${model} par défaut`}
                >
                  {model}
                </button>
              )
            })}
          </div>
        </div>
      )}

      {showCustomInput && (
        <div className="flex items-center gap-1.5">
          <input
            type="text"
            list={listID}
            value={value}
            onChange={e => onChange(e.target.value)}
            placeholder={placeholder}
            aria-invalid={invalid}
            aria-label={label}
            className={`flex-1 px-3 py-2 text-xs font-mono rounded-xl bg-[var(--bg-primary)] border text-[var(--text-primary)] focus:outline-none transition-all ${
              invalid ? 'border-red-500 focus:border-red-500' : 'border-[var(--border-color)] focus:border-[var(--accent-color)]'
            }`}
          />
          {suggestions.length > 0 && (
            <button
              type="button"
              onClick={() => {
                setIsCustomMode(false)
                onChange('')
              }}
              className="p-2 rounded-xl bg-[var(--bg-primary)] border border-[var(--border-color)] text-[var(--text-muted)] hover:text-[var(--text-primary)] cursor-pointer"
              title="Réinitialiser"
              aria-label="Réinitialiser"
            >
              <X size={13} />
            </button>
          )}
        </div>
      )}

      <datalist id={listID}>
        {suggestions.map(model => (
          <option key={model} value={model} />
        ))}
      </datalist>

      {invalid && (
        <p className="text-[10.5px] text-red-400 leading-relaxed">
          Identifiant invalide : lettres, chiffres et . _ - : @ / uniquement, sans espace.
        </p>
      )}
      {templateWins && (
        <p className="flex items-start gap-1 text-[10.5px] text-[var(--text-muted)] leading-relaxed">
          <Info size={12} className="text-indigo-400 shrink-0 mt-0.5" />
          <span>
            {(t?.profileModal?.ai?.modelTemplatePlaceholderNotice || "Le modèle est appliqué via le marqueur {model} dans la commande.")
              .split('{model}')
              .map((part, i, arr) => (
                <React.Fragment key={i}>
                  {part}
                  {i < arr.length - 1 && <code className="text-indigo-400 font-mono">{'{model}'}</code>}
                </React.Fragment>
              ))}
          </span>
        </p>
      )}
      {!templateWins && ignored && provider !== '' && (
        <p className="flex items-start gap-1 text-[10.5px] text-[var(--text-muted)] leading-relaxed">
          <Info size={12} className="text-indigo-400 shrink-0 mt-0.5" />
          <span>
            {t?.profileModal?.ai?.providerIgnoresModel
              ? t.profileModal.ai.providerIgnoresModel.replace('{provider}', provider.toUpperCase())
              : `${provider.toUpperCase()} n'accepte pas de sélection de modèle : la valeur est ignorée.`}
          </span>
        </p>
      )}
    </div>
  )
}
