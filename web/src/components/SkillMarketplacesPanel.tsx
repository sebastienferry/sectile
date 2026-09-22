import { useCallback, useEffect, useState } from 'react'
import { Package, Plus, Trash2 } from 'lucide-react'
import type { SkillMarketplace } from '../types'

const KINDS: { value: SkillMarketplace['kind']; label: string; hint: string }[] = [
  { value: 'github', label: 'GitHub', hint: 'owner/repo' },
  { value: 'git', label: 'Git', hint: 'https://… or git@…' },
  { value: 'path', label: 'Répertoire', hint: '/chemin/absolu' },
]

function when(value?: string): string {
  if (!value) return 'jamais'
  const at = Date.parse(value)
  return Number.isNaN(at) ? value : new Date(at).toLocaleString()
}

/**
 * Le registre des marketplaces de skills : une source est enregistrée une fois
 * pour le déploiement, et chaque projet y choisit ensuite le plugin qu'il
 * exécute. Enregistrer résout la source d'abord, donc un locator qui ne répond
 * pas est refusé avec sa raison plutôt qu'enregistré et cassé plus tard.
 */
export function SkillMarketplacesPanel() {
  const [entries, setEntries] = useState<SkillMarketplace[]>([])
  const [status, setStatus] = useState('')
  const [busy, setBusy] = useState(false)
  const [name, setName] = useState('')
  const [kind, setKind] = useState<SkillMarketplace['kind']>('github')
  const [locator, setLocator] = useState('')

  const load = useCallback(async () => {
    try {
      const res = await fetch('/api/skill-marketplaces')
      if (!res.ok) throw new Error(`HTTP ${res.status}`)
      setEntries((await res.json()) || [])
    } catch {
      setStatus('Registre illisible.')
    }
  }, [])

  useEffect(() => { void load() }, [load])

  async function register() {
    setStatus('')
    setBusy(true)
    try {
      const res = await fetch('/api/skill-marketplaces', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name, kind, locator }),
      })
      const body = await res.json().catch(() => ({}))
      if (!res.ok) throw new Error(body.error || `HTTP ${res.status}`)
      setName('')
      setLocator('')
      await load()
      setStatus(`${body.name} enregistrée.`)
    } catch (err) {
      setStatus(err instanceof Error ? err.message : 'Enregistrement refusé.')
    } finally {
      setBusy(false)
    }
  }

  async function remove(entry: SkillMarketplace) {
    setStatus('')
    setBusy(true)
    try {
      const res = await fetch('/api/skill-marketplaces/' + encodeURIComponent(entry.name), { method: 'DELETE' })
      const body = await res.json().catch(() => ({}))
      if (!res.ok) throw new Error(body.error || `HTTP ${res.status}`)
      await load()
      // Les projets qui l'épinglaient gardent les corps déjà appliqués : le
      // dire est ce qui distingue un retrait d'une modification silencieuse.
      const pinned: string[] = body.pinnedBy || []
      setStatus(pinned.length
        ? `${entry.name} retirée. ${pinned.length} projet(s) gardent les corps appliqués, leur épingle est orpheline.`
        : `${entry.name} retirée.`)
    } catch (err) {
      setStatus(err instanceof Error ? err.message : 'Retrait refusé.')
    } finally {
      setBusy(false)
    }
  }

  const hint = KINDS.find(k => k.value === kind)?.hint || ''

  return (
    <div className="space-y-2">
      <label className="block text-[11px] font-bold uppercase tracking-wider text-[var(--text-muted)] flex items-center gap-1.5">
        <Package size={14} className="text-sky-400" />
        <span>Marketplaces de skills</span>
      </label>
      <p className="text-[10.5px] text-[var(--text-muted)] leading-relaxed">
        Sources au format marketplace de plugins Claude (<code className="font-mono">.claude-plugin/marketplace.json</code>).
        Sectile lit le format lui-même : un projet sur codex, agy, gemini, cursor ou vibe en utilise une comme un projet claude.
        Chaque projet choisit ensuite son plugin, voit le diff, et l'applique explicitement.
      </p>

      <div className="space-y-1">
        {entries.length === 0 && <p className="text-[10.5px] text-[var(--text-muted)]">Aucune source enregistrée.</p>}
        {entries.map(entry => (
          <div key={entry.name} className="flex items-center gap-2 px-2.5 py-1.5 rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)]">
            <div className="min-w-0 flex-1">
              <div className="text-[11px] font-bold text-[var(--text-primary)] truncate">
                {entry.name}
                {entry.owner && <span className="ml-1.5 text-[10px] font-normal text-[var(--text-muted)]">{entry.owner}</span>}
              </div>
              <code className="text-[9.5px] font-mono text-[var(--text-muted)]">
                {entry.kind} · {entry.locator} · lue {when(entry.lastFetchedAt)}
                {entry.lastCommit ? ` · ${entry.lastCommit.slice(0, 7)}` : ''}
              </code>
            </div>
            <button
              type="button"
              onClick={() => remove(entry)}
              disabled={busy}
              className="p-1.5 rounded-lg text-[var(--text-muted)] hover:text-rose-400 disabled:opacity-40 cursor-pointer"
              title="Retirer du registre"
            >
              <Trash2 size={13} />
            </button>
          </div>
        ))}
      </div>

      <div className="flex items-center gap-1.5 flex-wrap">
        <input
          value={name}
          onChange={e => setName(e.target.value)}
          placeholder="nom"
          className="w-28 px-2 py-1 text-[11px] rounded-lg bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none"
        />
        <select
          value={kind}
          onChange={e => setKind(e.target.value as SkillMarketplace['kind'])}
          className="px-2 py-1 text-[11px] rounded-lg bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] cursor-pointer"
        >
          {KINDS.map(option => (
            <option key={option.value} value={option.value}>{option.label}</option>
          ))}
        </select>
        <input
          value={locator}
          onChange={e => setLocator(e.target.value)}
          placeholder={hint}
          className="flex-1 min-w-40 px-2 py-1 text-[11px] font-mono rounded-lg bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none"
        />
        <button
          type="button"
          onClick={register}
          disabled={busy || !name.trim() || !locator.trim()}
          className="flex items-center gap-1 px-2.5 py-1 rounded-lg text-[11px] font-bold text-white accent-bg hover:opacity-90 disabled:opacity-40 cursor-pointer"
        >
          <Plus size={12} />
          <span>Enregistrer</span>
        </button>
      </div>

      {status && <p role="status" className="text-[10.5px] text-[var(--text-secondary)]">{status}</p>}
    </div>
  )
}
