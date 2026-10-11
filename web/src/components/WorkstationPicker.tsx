import { useEffect, useRef } from 'react'
import { useApp } from '../context/AppContext'
import { useEscapeKey } from '../hooks/useEscapeKey'
import { useBackdropDismiss } from '../hooks/useBackdropDismiss'
import { workstationName } from '../lib/workstations'

export function WorkstationPicker() {
  const { workstationChoice, chooseWorkstation, t } = useApp()
  const dialog = useRef<HTMLDivElement>(null)
  const cancel = () => chooseWorkstation(null)
  useEscapeKey(Boolean(workstationChoice), cancel)
  const backdrop = useBackdropDismiss(cancel)
  const isOpen = Boolean(workstationChoice)
  useEffect(() => {
    if (!isOpen) return
    const previous = document.activeElement as HTMLElement | null
    dialog.current?.querySelector<HTMLButtonElement>('button')?.focus()
    return () => previous?.focus()
  }, [isOpen])
  if (!workstationChoice) return null
  const strings = t.workstationPicker
  return (
    <div className="fixed inset-0 z-60 flex items-center justify-center bg-black/60 p-4" {...backdrop}>
      <div ref={dialog} role="dialog" aria-modal="true" aria-labelledby="workstation-title" data-workstation-picker
        className="w-full max-w-md rounded-2xl bg-[var(--bg-secondary)] border border-[var(--border-color)] p-5 space-y-3"
        onKeyDown={event => {
          if (event.key !== 'Tab') return
          const buttons = [...(dialog.current?.querySelectorAll<HTMLButtonElement>('button:not(:disabled)') || [])]
          const first = buttons[0], last = buttons[buttons.length - 1]
          if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last?.focus() }
          else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first?.focus() }
        }}>
        <h2 id="workstation-title" className="font-bold">{strings.title}</h2>
        {!workstationChoice.candidates.length && <p role="status">{strings.empty}</p>}
        {workstationChoice.candidates.map(device => (
          <button type="button" key={device.deviceId} data-workstation={device.deviceId}
            className="block w-full rounded-xl border border-[var(--border-color)] px-3 py-2 text-left hover:border-[var(--accent-color)] cursor-pointer focus-visible:outline-2 focus-visible:outline-[var(--accent-color)]"
            onClick={() => chooseWorkstation(device.deviceId)}>{workstationName(device, workstationChoice.candidates)}</button>
        ))}
        <div className="flex justify-end gap-3">
          <button type="button" className="cursor-pointer focus-visible:outline-2 focus-visible:outline-[var(--accent-color)]" onClick={() => void workstationChoice.refresh()}>{strings.refresh}</button>
          <button type="button" className="cursor-pointer focus-visible:outline-2 focus-visible:outline-[var(--accent-color)]" onClick={cancel}>{strings.cancel}</button>
        </div>
      </div>
    </div>
  )
}
