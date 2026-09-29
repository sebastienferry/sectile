import React, { useEffect, useMemo, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { Bold, Italic, Code, Link2, List, ListOrdered, Quote, Heading2, Eye, Pencil, CheckSquare, Maximize2, Minimize2 } from 'lucide-react'
import { useApp } from '../context/AppContext'
import type { TaskDetailStrings } from '../locales/taskDetail'

type MarkdownStrings = TaskDetailStrings['markdown']

/**
 * Rendu et édition du Markdown, pour les descriptions et les commentaires.
 *
 * Les descriptions que produisent les skills, celles que le tracker renvoie et
 * celles qu'on écrit à la main sont du Markdown depuis toujours : elles étaient
 * simplement affichées telles quelles, dièses et astérisques compris.
 *
 * Le rendu passe par react-markdown, qui construit des noeuds React au lieu
 * d'injecter du HTML : un ticket peut contenir n'importe quoi, y compris du HTML
 * collé depuis un mail, et rien de tout cela ne doit s'exécuter ici.
 */

/** Rendu seul, pour un commentaire ou une description déjà écrite. */
export const MarkdownView: React.FC<{ children: string; className?: string; compact?: boolean }> = ({
  children,
  className,
  compact,
}) => {
  const text = (children || '').trim()
  if (!text) return null

  return (
    <div className={`markdown-body ${compact ? 'markdown-compact' : ''} ${className || ''}`}>
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        components={{
          // Un lien d'un ticket mène ailleurs : il s'ouvre à côté, jamais en
          // remplaçant l'application.
          a: props => <a {...props} target="_blank" rel="noreferrer" />,
        }}
      >
        {text}
      </ReactMarkdown>
    </div>
  )
}

type SnippetId = keyof MarkdownStrings['snippets']

type Snippet = {
  /** Catalog entry holding the button title and the placeholder text. */
  id: SnippetId
  icon: React.ReactNode
  /** Texte inséré avant la sélection. */
  before: string
  /** Texte inséré après la sélection, vide pour un préfixe de ligne. */
  after?: string
  /** Le préfixe s'applique à chaque ligne sélectionnée (listes, citations). */
  perLine?: boolean
  shortcut?: string
}

const SNIPPETS: Snippet[] = [
  { id: 'bold', icon: <Bold size={12} />, before: '**', after: '**', shortcut: 'b' },
  { id: 'italic', icon: <Italic size={12} />, before: '_', after: '_', shortcut: 'i' },
  { id: 'code', icon: <Code size={12} />, before: '`', after: '`' },
  { id: 'link', icon: <Link2 size={12} />, before: '[', after: '](url)', shortcut: 'k' },
  { id: 'heading', icon: <Heading2 size={12} />, before: '## ', perLine: true },
  { id: 'list', icon: <List size={12} />, before: '- ', perLine: true },
  { id: 'orderedList', icon: <ListOrdered size={12} />, before: '1. ', perLine: true },
  { id: 'checkbox', icon: <CheckSquare size={12} />, before: '- [ ] ', perLine: true },
  { id: 'quote', icon: <Quote size={12} />, before: '> ', perLine: true },
]

/**
 * Éditeur Markdown : une zone de saisie, une barre de mise en forme et un
 * aperçu. L'aperçu est un onglet et non un second panneau, parce que ces champs
 * vivent dans une fiche déjà dense et dans un commentaire de quelques lignes.
 *
 * A field that holds long text can ask for `maximizable`: a button then opens
 * the same text in a full-screen editor, with the input and its rendered
 * preview side by side, which the field itself has no room for. It edits the
 * same value through the same onChange, so whatever saves the field saves what
 * was typed there, and nothing else does.
 */
export const MarkdownEditor: React.FC<{
  value: string
  onChange: (value: string) => void
  placeholder?: string
  rows?: number
  minHeight?: number
  maxHeight?: number
  disabled?: boolean
  /** Rendu au dessus de l'onglet Aperçu, pour un bouton d'envoi par exemple. */
  actions?: React.ReactNode
  onKeyDown?: (e: React.KeyboardEvent<HTMLTextAreaElement>) => void
  /** Offer the full-screen editor. Off by default: a comment has no use for it. */
  maximizable?: boolean
  /** The heading of the full-screen editor, naming the field it edits. */
  maximizeTitle?: string
}> = ({ value, onChange, placeholder, minHeight = 120, maxHeight, disabled, actions, onKeyDown, maximizable, maximizeTitle }) => {
  const [isPreview, setIsPreview] = useState(false)
  const [isMaximized, setIsMaximized] = useState(false)
  const textareaRef = useRef<HTMLTextAreaElement>(null)
  const maximizedRef = useRef<HTMLTextAreaElement>(null)
  const maximizeButtonRef = useRef<HTMLButtonElement>(null)
  const { t } = useApp()
  const strings = t.taskDetail.markdown

  const closeMaximized = () => {
    setIsMaximized(false)
    requestAnimationFrame(() => maximizeButtonRef.current?.focus())
  }

  // Escape closes the full-screen editor and nothing else. The dialogs of the
  // app listen on the document in the capture phase (useEscapeKey), so this one
  // listens on the window, which the event reaches first, and stops it there.
  useEffect(() => {
    if (!isMaximized) return
    maximizedRef.current?.focus()
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key !== 'Escape') return
      e.preventDefault()
      e.stopPropagation()
      setIsMaximized(false)
      requestAnimationFrame(() => maximizeButtonRef.current?.focus())
    }
    window.addEventListener('keydown', onKeyDown, true)
    return () => window.removeEventListener('keydown', onKeyDown, true)
  }, [isMaximized])

  const applySnippet = (snippet: Snippet, textarea: HTMLTextAreaElement | null) => {
    if (!textarea || disabled) return

    const start = textarea.selectionStart
    const end = textarea.selectionEnd
    const selected = value.slice(start, end)
    const placeholder = strings.snippets[snippet.id].placeholder

    let inserted: string
    let nextStart: number
    let nextEnd: number

    if (snippet.perLine) {
      // Le préfixe se pose en tête de chaque ligne, et sur la ligne courante
      // quand rien n'est sélectionné.
      const lineStart = value.lastIndexOf('\n', Math.max(0, start - 1)) + 1
      const target = selected || placeholder
      const body = selected ? value.slice(lineStart, end) : target
      inserted = body
        .split('\n')
        .map(line => (line.startsWith(snippet.before) ? line : snippet.before + line))
        .join('\n')
      const from = selected ? lineStart : start
      const to = selected ? end : start
      onChange(value.slice(0, from) + inserted + value.slice(to))
      nextStart = from
      nextEnd = from + inserted.length
    } else {
      const target = selected || placeholder
      inserted = snippet.before + target + (snippet.after || '')
      onChange(value.slice(0, start) + inserted + value.slice(end))
      // Sans sélection, le curseur se pose sur le mot posé, prêt à être remplacé.
      nextStart = start + snippet.before.length
      nextEnd = nextStart + target.length
    }

    requestAnimationFrame(() => {
      textarea.focus()
      textarea.setSelectionRange(nextStart, nextEnd)
    })
  }

  const shortcuts = useMemo(() => SNIPPETS.filter(s => s.shortcut), [])

  // The field and the full-screen editor are two textareas on one value; the
  // toolbar and the shortcuts act on the one they belong to, resolved when used.
  type Surface = 'inline' | 'maximized'
  const textareaOf = (surface: Surface) => (surface === 'inline' ? textareaRef.current : maximizedRef.current)

  const snippetButtons = (surface: Surface) =>
    SNIPPETS.map(snippet => {
      const title = strings.snippets[snippet.id].title
      return (
        <button
          key={snippet.id}
          type="button"
          disabled={disabled}
          onClick={() => applySnippet(snippet, textareaOf(surface))}
          title={snippet.shortcut ? `${title} (Ctrl/Cmd+${snippet.shortcut.toUpperCase()})` : title}
          className="p-1 rounded text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-tertiary)] cursor-pointer disabled:opacity-40"
        >
          {snippet.icon}
        </button>
      )
    })

  const textareaProps = (surface: Surface) => ({
    value,
    disabled,
    placeholder,
    onChange: (e: React.ChangeEvent<HTMLTextAreaElement>) => onChange(e.target.value),
    onKeyDown: (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
      // Les raccourcis d'usage, ceux qu'on tape sans y penser.
      if ((e.metaKey || e.ctrlKey) && !e.altKey) {
        const snippet = shortcuts.find(s => s.shortcut === e.key.toLowerCase())
        if (snippet) {
          e.preventDefault()
          applySnippet(snippet, textareaOf(surface))
          return
        }
      }
      onKeyDown?.(e)
    },
  })

  const preview = value.trim() ? (
    <MarkdownView>{value}</MarkdownView>
  ) : (
    <span className="text-[11px] text-[var(--text-muted)] italic">{strings.emptyPreview}</span>
  )

  return (
    <div className="rounded-xl border border-[var(--border-color)] bg-[var(--bg-tertiary)] overflow-hidden">
      <div className="flex items-center gap-0.5 px-1.5 py-1 border-b border-[var(--border-color)] bg-[var(--bg-secondary)]/60">
        <button
          type="button"
          onClick={() => setIsPreview(false)}
          className={`flex items-center gap-1 px-2 py-0.5 rounded text-[10.5px] font-bold cursor-pointer ${
            isPreview ? 'text-[var(--text-muted)] hover:text-[var(--text-primary)]' : 'text-[var(--accent-color)] bg-[var(--accent-light)]'
          }`}
        >
          <Pencil size={11} />
          {strings.write}
        </button>
        <button
          type="button"
          onClick={() => setIsPreview(true)}
          className={`flex items-center gap-1 px-2 py-0.5 rounded text-[10.5px] font-bold cursor-pointer ${
            isPreview ? 'text-[var(--accent-color)] bg-[var(--accent-light)]' : 'text-[var(--text-muted)] hover:text-[var(--text-primary)]'
          }`}
        >
          <Eye size={11} />
          {strings.preview}
        </button>

        {!isPreview && (
          <div className="flex items-center gap-0.5 ml-2 pl-2 border-l border-[var(--border-color)]">
            {snippetButtons('inline')}
          </div>
        )}

        {(actions || maximizable) && (
          <div className="ml-auto flex items-center gap-1.5">
            {actions}
            {maximizable && (
              <button
                ref={maximizeButtonRef}
                type="button"
                onClick={() => setIsMaximized(true)}
                title={strings.maximize}
                aria-label={strings.maximize}
                className="p-1 rounded text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-tertiary)] cursor-pointer"
              >
                <Maximize2 size={12} />
              </button>
            )}
          </div>
        )}
      </div>

      {isPreview ? (
        <div className="px-3 py-2 overflow-auto" style={{ minHeight, maxHeight }}>
          {preview}
        </div>
      ) : (
        <textarea
          ref={textareaRef}
          {...textareaProps('inline')}
          style={{ minHeight, maxHeight }}
          className="w-full px-3 py-2 text-[13px] bg-transparent text-[var(--text-primary)] focus:outline-none leading-relaxed resize-y font-[inherit]"
        />
      )}

      {isMaximized &&
        createPortal(
          <div
            role="dialog"
            aria-modal="true"
            aria-label={maximizeTitle || strings.maximize}
            className="fixed inset-0 z-[110] flex flex-col bg-[var(--bg-primary)] text-[var(--text-primary)]"
          >
            <div className="flex items-center gap-2 px-4 py-2 border-b border-[var(--border-color)] bg-[var(--bg-secondary)] shrink-0">
              {maximizeTitle && <span className="text-[12px] font-bold uppercase tracking-[.08em]">{maximizeTitle}</span>}
              <div className="flex items-center gap-0.5 ml-2 pl-2 border-l border-[var(--border-color)]">
                {snippetButtons('maximized')}
              </div>
              <button
                type="button"
                onClick={closeMaximized}
                title={strings.restore}
                className="ml-auto inline-flex items-center gap-1 px-2 py-1 rounded-lg text-xs font-semibold text-[var(--text-secondary)] hover:text-[var(--text-primary)] cursor-pointer border border-[var(--border-color)] hover:border-[var(--accent-color)]/50 transition-colors"
              >
                <Minimize2 size={12} />
                <span>{strings.restore}</span>
              </button>
            </div>
            <div className="flex-1 min-h-0 grid grid-cols-2">
              <textarea
                ref={maximizedRef}
                {...textareaProps('maximized')}
                className="h-full w-full px-4 py-3 text-[13px] bg-[var(--bg-tertiary)] text-[var(--text-primary)] focus:outline-none leading-relaxed resize-none font-[inherit] border-r border-[var(--border-color)]"
              />
              <div className="h-full overflow-auto px-4 py-3" aria-label={strings.preview}>
                {preview}
              </div>
            </div>
          </div>,
          document.body
        )}
    </div>
  )
}
