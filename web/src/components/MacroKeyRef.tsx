import { useEffect, useRef, useState } from 'react'
import { Check, Copy, ExternalLink } from 'lucide-react'
import { useApp } from '../context/AppContext'
import { copyText } from '../lib/clipboard'
import { format } from '../lib/i18n'

interface Props {
  macroKey: string
  /** The macro's tracker address; empty or missing renders plain text. */
  url?: string
  /** Classes of the key text, so each surface keeps its size and colour. */
  className?: string
  /** Size of the external-link and copy icons. */
  iconSize?: number
}

/**
 * A macro key the way the ticket detail shows a ticket key (#809): a link to
 * the macro on its tracker when the server gives an address, then a button
 * that copies the key alone.
 *
 * It sits inside clickable, draggable rows and cards, so no click, press or
 * drag begun on it reaches them: the span stops click and mousedown, and is
 * itself draggable only to cancel a drag before the row's handler sees it.
 */
export function MacroKeyRef({ macroKey, url, className = '', iconSize = 11 }: Props) {
  const { addToast, t } = useApp()
  const td = t.taskDetail
  const [copied, setCopied] = useState(false)
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null)

  useEffect(() => () => {
    if (timer.current) clearTimeout(timer.current)
  }, [])

  const copy = async () => {
    if (!(await copyText(macroKey))) {
      addToast({ type: 'error', title: td.toasts.copyFailedTitle, description: td.toasts.copyFailedDescription })
      return
    }
    setCopied(true)
    if (timer.current) clearTimeout(timer.current)
    timer.current = setTimeout(() => {
      setCopied(false)
      timer.current = null
    }, 2000)
    addToast({
      type: 'success',
      title: td.toasts.keyCopiedTitle,
      description: format(td.toasts.keyCopiedDescription, { key: macroKey }),
    })
  }

  const copyLabel = format(td.header.copyKey, { key: macroKey })
  return (
    <span
      data-macro-key={macroKey}
      className="inline-flex items-center shrink-0"
      draggable
      onDragStart={e => {
        e.preventDefault()
        e.stopPropagation()
      }}
      onClick={e => e.stopPropagation()}
      onMouseDown={e => e.stopPropagation()}
    >
      {url ? (
        <a
          href={url}
          target="_blank"
          rel="noreferrer"
          className={`inline-flex items-center gap-1 hover:underline ${className}`}
          title={format(t.planning.roadmap.panel.openRemoteTitle, { key: macroKey })}
        >
          <span>{macroKey}</span>
          <ExternalLink size={iconSize} className="opacity-70" />
        </a>
      ) : (
        <span className={className}>{macroKey}</span>
      )}
      <button
        type="button"
        onClick={copy}
        title={copyLabel}
        aria-label={copyLabel}
        className="ml-0.5 inline-flex items-center rounded p-0.5 opacity-60 hover:opacity-100 hover:bg-[var(--bg-tertiary)] focus-visible:opacity-100 cursor-pointer"
      >
        {copied ? <Check size={iconSize} className="text-emerald-400" /> : <Copy size={iconSize} />}
      </button>
    </span>
  )
}
