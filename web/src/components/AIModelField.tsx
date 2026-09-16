import React from 'react'
import { Cpu, Info } from 'lucide-react'
import type { AIProvider } from '../types'
import { AI_MODEL_SUGGESTIONS, isValidModel, providerTakesModel, templateGovernsCommand } from '../lib/aiModels'

interface AIModelFieldProps {
  provider: AIProvider | ''
  commandTemplate: string
  value: string
  onChange: (value: string) => void
  /** Texte affiché quand le champ est vide : « défaut du CLI » ou « réglage global ». */
  placeholder: string
  label: string
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
}) => {
  const listID = `ai-model-suggestions-${provider || 'none'}`
  const suggestions = (provider && AI_MODEL_SUGGESTIONS[provider as AIProvider]) || []
  const templateWins = templateGovernsCommand(provider, commandTemplate)
  const ignored = !providerTakesModel(provider)
  const invalid = !isValidModel(value)

  return (
    <div className="space-y-1.5">
      <label className="block text-[11px] font-bold uppercase tracking-wider text-[var(--text-muted)] flex items-center gap-1.5">
        <Cpu size={13} className="text-indigo-400" />
        <span>{label}</span>
      </label>

      <input
        type="text"
        list={listID}
        value={value}
        onChange={e => onChange(e.target.value)}
        placeholder={placeholder}
        aria-invalid={invalid}
        className={`w-full px-3 py-2 text-xs font-mono rounded-xl bg-[var(--bg-primary)] border text-[var(--text-primary)] focus:outline-none transition-all ${
          invalid ? 'border-red-500 focus:border-red-500' : 'border-[var(--border-color)] focus:border-[var(--accent-color)]'
        }`}
      />
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
            Le modèle de ligne de commande pilote l'exécution : le modèle n'est appliqué que via le
            marqueur <code className="text-indigo-400 font-mono">{'{model}'}</code>.
          </span>
        </p>
      )}
      {!templateWins && ignored && provider !== '' && (
        <p className="flex items-start gap-1 text-[10.5px] text-[var(--text-muted)] leading-relaxed">
          <Info size={12} className="text-indigo-400 shrink-0 mt-0.5" />
          <span>{provider.toUpperCase()} n'accepte pas de sélection de modèle : la valeur est ignorée.</span>
        </p>
      )}
    </div>
  )
}
