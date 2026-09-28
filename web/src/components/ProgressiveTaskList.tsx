import { useEffect, useRef, useState, type ReactNode } from 'react'
import type { Task } from '../types'

const PAGE_SIZE = 20

interface Props {
  tasks: readonly Task[]
  renderTask: (task: Task) => ReactNode
  emptyLabel: string
  moreLabel: string
}

/** Keep loaded cards mounted so their menus, focus and drag state survive scrolling. */
export function ProgressiveTaskList(props: Props) {
  // A changed order or membership starts from the top; report and activity
  // updates with the same task IDs preserve both mounted cards and scrolling.
  return <TaskPages key={JSON.stringify(props.tasks.map(task => task.id))} {...props} />
}

function TaskPages({ tasks, renderTask, emptyLabel, moreLabel }: Props) {
  const [count, setCount] = useState(PAGE_SIZE)
  const scrollRef = useRef<HTMLDivElement>(null)
  const moreRef = useRef<HTMLButtonElement>(null)
  const hasMore = count < tasks.length

  useEffect(() => {
    const target = moreRef.current
    if (!hasMore || !target || typeof IntersectionObserver === 'undefined') return
    const observer = new IntersectionObserver(entries => {
      if (!entries.some(entry => entry.isIntersecting)) return
      observer.disconnect()
      setCount(previous => previous + PAGE_SIZE)
    }, { root: scrollRef.current, rootMargin: '300px 0px' })
    observer.observe(target)
    return () => observer.disconnect()
  }, [count, hasMore])

  return (
    <div ref={scrollRef} className="flex-1 min-h-0 overflow-y-auto p-2.5 space-y-2.5">
      {tasks.slice(0, count).map(renderTask)}
      {hasMore && (
        <button
          ref={moreRef}
          type="button"
          onClick={() => setCount(previous => previous + PAGE_SIZE)}
          className="w-full rounded-lg p-2 text-xs text-[var(--text-secondary)] hover:bg-[var(--bg-tertiary)] focus-visible:outline-2 focus-visible:outline-[var(--accent-color)] cursor-pointer"
        >
          {moreLabel}
        </button>
      )}
      {tasks.length === 0 && (
        <div className="h-32 flex flex-col items-center justify-center text-center p-4 border border-dashed border-[var(--border-color)]/60 rounded-xl">
          <p className="text-xs text-[var(--text-muted)]">{emptyLabel}</p>
        </div>
      )}
    </div>
  )
}
