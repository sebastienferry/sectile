import React, { useEffect, useRef, useState, useCallback } from 'react'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import { WebLinksAddon } from '@xterm/addon-web-links'
import '@xterm/xterm/css/xterm.css'
import {
  Terminal as TerminalIcon,
  RotateCcw,
  Sparkles,
  Zap,
  AlertCircle,
  GitBranch,
  Code2,
  Maximize2,
  Minimize2,
  X,
  Check,
} from 'lucide-react'
import type { Task } from '../types'
import { terminalSkillCommand } from '../lib/terminalSkillCommand'
import { useApp } from '../context/AppContext'

interface InteractiveTerminalProps {
  /** Task-scoped session: runs in the task worktree, unlocks the skill shortcuts. */
  task?: Task
  /** Workspace session: an explicit PTY session id, kept alive server-side. */
  sessionId?: string
  /** Working directory for a workspace session. */
  cwd?: string
  /** Project the workspace session belongs to, for the agent command template. */
  projectId?: string
  /** Label shown in the session bar; defaults to "ZSH Session". */
  label?: string
  isExpanded?: boolean
  /** Un agent tourne déjà dans cette session : le bouton propose de le relancer. */
  agentRunning?: boolean
  initialCommand?: string
  onToggleExpand?: () => void
  onClose?: () => void
}

// Un seul style pour tous les boutons de la barre d'actions : la couleur ne
// portait aucune information, chaque bouton avait la sienne et l'oeil s'y perdait.
const BAR_BUTTON =
  'flex items-center gap-1 px-2 py-1 bg-slate-900 hover:bg-slate-800 border border-slate-800 text-slate-300 hover:text-white rounded-md font-mono text-[10.5px] transition-colors shrink-0 cursor-pointer'

export const InteractiveTerminal: React.FC<InteractiveTerminalProps> = ({
  task,
  sessionId,
  cwd,
  projectId,
  label,
  isExpanded,
  agentRunning,
  initialCommand,
  onToggleExpand,
  onClose,
}) => {
  const { settings, openInEditor, projects, skills, startTaskAgent, injectTaskSkill, setSelectedTask } = useApp()
  const terminalContainerRef = useRef<HTMLDivElement>(null)
  const termRef = useRef<Terminal | null>(null)
  const fitAddonRef = useRef<FitAddon | null>(null)
  const wsRef = useRef<WebSocket | null>(null)

  const [status, setStatus] = useState<'connecting' | 'connected' | 'disconnected' | 'error'>('connecting')

  // Connect WebSocket to Go backend PTY
  const connectWs = useCallback(() => {
    if (wsRef.current) {
      wsRef.current.close()
    }

    setStatus('connecting')

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    // In dev mode (Vite at :5173), connect to backend port :8090, or fallback to current host
    const host = window.location.port === '5173' ? `${window.location.hostname}:8090` : window.location.host
    const params = new URLSearchParams()
    if (task) {
      params.set('taskId', task.id)
    } else {
      params.set('sessionId', sessionId || 'global-workspace')
      if (cwd) params.set('cwd', cwd)
    }
    const wsUrl = `${protocol}//${host}/ws/terminal?${params.toString()}`

    try {
      const ws = new WebSocket(wsUrl)
      ws.binaryType = 'arraybuffer'
      wsRef.current = ws

      ws.onopen = () => {
        setStatus('connected')
        // Send initial resize
        if (fitAddonRef.current && termRef.current) {
          fitAddonRef.current.fit()
          const { cols, rows } = termRef.current
          ws.send(JSON.stringify({ type: 'resize', cols, rows }))
        }
        termRef.current?.focus()
        if (initialCommand) {
          setTimeout(() => {
            if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
              wsRef.current.send(JSON.stringify({ type: 'input', data: `${initialCommand}\n` }))
              termRef.current?.focus()
            }
          }, 400)
        }
      }

      ws.onmessage = (event) => {
        if (termRef.current) {
          if (event.data instanceof ArrayBuffer) {
            const bytes = new Uint8Array(event.data)
            termRef.current.write(bytes)
          } else if (typeof event.data === 'string') {
            try {
              const msg = JSON.parse(event.data)
              if (msg.type === 'data') {
                termRef.current.write(msg.data)
              }
            } catch {
              termRef.current.write(event.data)
            }
          }
        }
      }

      ws.onclose = () => {
        setStatus('disconnected')
      }

      ws.onerror = (err) => {
        console.error('Terminal WebSocket error:', err)
        setStatus('error')
      }
    } catch (err: any) {
      console.error('WebSocket connection error:', err)
      setStatus('error')
    }
  }, [task?.id, sessionId, cwd])

  // Initialize Xterm.js
  useEffect(() => {
    if (!terminalContainerRef.current) return

    terminalContainerRef.current.innerHTML = ''

    const term = new Terminal({
      cursorBlink: true,
      cursorStyle: 'block',
      cursorInactiveStyle: 'outline',
      fontSize: 12,
      lineHeight: 1.25,
      fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace',
      theme: {
        background: '#0a0f1d',
        foreground: '#e2e8f0',
        cursor: '#818cf8',
        cursorAccent: '#0a0f1d',
        selectionBackground: '#4338ca55',
        selectionInactiveBackground: '#4338ca33',
        black: '#0f172a',
        red: '#f87171',
        green: '#4ade80',
        yellow: '#facc15',
        blue: '#60a5fa',
        magenta: '#c084fc',
        cyan: '#38bdf8',
        white: '#f1f5f9',
        brightBlack: '#475569',
        brightRed: '#ef4444',
        brightGreen: '#22c55e',
        brightYellow: '#eab308',
        brightBlue: '#3b82f6',
        brightMagenta: '#a855f7',
        brightCyan: '#06b6d4',
        brightWhite: '#ffffff',
      },
      convertEol: true,
      allowProposedApi: true,
    })

    const fitAddon = new FitAddon()
    const webLinksAddon = new WebLinksAddon()

    term.loadAddon(fitAddon)
    term.loadAddon(webLinksAddon)

    term.open(terminalContainerRef.current)
    try {
      fitAddon.fit()
    } catch {
      // ignore initial fit error if not yet rendered
    }

    termRef.current = term
    fitAddonRef.current = fitAddon

    // Immédiat puis différé pour garantir le focus dès l'affichage
    term.focus()
    const focusTimer = setTimeout(() => {
      term.focus()
    }, 60)

    // Handle user keystrokes in xterm
    term.onData((data) => {
      if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
        wsRef.current.send(JSON.stringify({ type: 'input', data }))
      }
    })

    // Resize observer to auto fit when container dimensions change
    const resizeObserver = new ResizeObserver(() => {
      if (fitAddonRef.current && termRef.current && wsRef.current?.readyState === WebSocket.OPEN) {
        try {
          fitAddonRef.current.fit()
          const { cols, rows } = termRef.current
          wsRef.current.send(JSON.stringify({ type: 'resize', cols, rows }))
        } catch {
          // ignore transient resize errors
        }
      }
    })

    resizeObserver.observe(terminalContainerRef.current)

    // Connect WebSocket
    connectWs()

    return () => {
      clearTimeout(focusTimer)
      resizeObserver.disconnect()
      if (wsRef.current) {
        wsRef.current.close()
      }
      term.dispose()
    }
  }, [connectWs])

  // Handle expand resize
  useEffect(() => {
    const timer = setTimeout(() => {
      if (fitAddonRef.current && termRef.current && wsRef.current?.readyState === WebSocket.OPEN) {
        fitAddonRef.current.fit()
        const { cols, rows } = termRef.current
        wsRef.current.send(JSON.stringify({ type: 'resize', cols, rows }))
      }
      termRef.current?.focus()
    }, 200)
    return () => clearTimeout(timer)
  }, [isExpanded])

  // Helper to send text / command into terminal
  const sendCommand = (cmd: string) => {
    if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify({ type: 'input', data: cmd }))
      termRef.current?.focus()
    }
  }

  // Quick Action commands
  const proj = projects.find(p => p.id === (task?.projectId || projectId))
  const provider = proj?.aiProvider || settings.aiProvider || 'agy'

  // Le lancement passe par le serveur : il résout le binaire du moteur du projet,
  // ouvre la session dans le bon répertoire et retient qu'un agent y tourne.
  const handleLaunchAgent = async () => {
    if (task) {
      // Un agent déjà présent est relancé explicitement : c'est le cas où on l'a
      // quitté à la main et où TaskFlow le croit encore vivant.
      await startTaskAgent(task.id, Boolean(agentRunning))
      return
    }
    sendCommand(`${provider}\n`)
  }

  // Le serveur adapte le nom de la skill au moteur de l'agent qui tourne.
  // Avant, ce bouton composait « claude -p "/clarify-issue ..." » et l'envoyait
  // au shell : si l'agent était ouvert, cette ligne devenait un simple message.
  const handleRunSkill = async (skillId: string) => {
    if (!task) return
    await injectTaskSkill(task.id, skillId)
  }

  const handleShellCommand = (cmd: string) => {
    sendCommand(`${cmd}\n`)
  }

  const handleInterrupt = () => {
    sendCommand('\x03') // Ctrl+C
  }

  const handleClear = () => {
    termRef.current?.clear()
    sendCommand('clear\n')
  }

  const handleResetSession = async () => {
    try {
      await fetch('/api/terminal/reset', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ taskId: task?.id, sessionId: task ? undefined : (sessionId || 'global-workspace') }),
      })
      termRef.current?.clear()
      connectWs()
    } catch (e) {
      console.error('Reset session error:', e)
    }
  }

  return (
    <div className="flex flex-col h-full bg-[#070b14] border border-slate-800/80 rounded-xl overflow-hidden shadow-2xl">
      {/* Top Bar: Session Info & Status */}
      <div className="flex items-center justify-between px-3 py-2 bg-slate-900/90 border-b border-slate-800/80 text-xs select-none">
        <div className="flex items-center gap-2 overflow-hidden">
          <div className="flex items-center gap-1.5 font-mono text-[11px] text-slate-300 font-semibold">
            <TerminalIcon size={14} className="text-indigo-400" />
            {task ? (
              <>
                {/* La clé ouvre la fiche du ticket : c'est le geste attendu quand
                    on lit un compte-rendu dans la console et qu'on veut le
                    contexte. */}
                <button
                  type="button"
                  onClick={() => setSelectedTask(task)}
                  className="text-indigo-300 hover:text-indigo-200 hover:underline cursor-pointer"
                  title={`Ouvrir la fiche de ${task.key}`}
                >
                  {task.key}
                </button>
                <span className="text-slate-500 font-normal truncate max-w-[180px]">{task.title}</span>
              </>
            ) : (
              <span>{label || 'ZSH Session'}</span>
            )}
          </div>

          <span className="text-slate-600">•</span>

          {/* Status Indicator */}
          {status === 'connected' && (
            <div className="flex items-center gap-1 text-[10px] text-emerald-400 bg-emerald-500/10 border border-emerald-500/20 px-2 py-0.5 rounded-full font-medium">
              <span className="w-1.5 h-1.5 rounded-full bg-emerald-400 animate-pulse" />
              <span>Connecté (Live PTY)</span>
            </div>
          )}
          {status === 'connecting' && (
            <div className="flex items-center gap-1 text-[10px] text-amber-400 bg-amber-500/10 border border-amber-500/20 px-2 py-0.5 rounded-full font-medium">
              <span className="w-1.5 h-1.5 rounded-full bg-amber-400 animate-ping" />
              <span>Connexion...</span>
            </div>
          )}
          {(status === 'disconnected' || status === 'error') && (
            <div className="flex items-center gap-1.5">
              <span className="flex items-center gap-1 text-[10px] text-red-400 bg-red-500/10 border border-red-500/20 px-2 py-0.5 rounded-full font-medium">
                <AlertCircle size={10} />
                <span>Déconnecté</span>
              </span>
              <button
                type="button"
                onClick={connectWs}
                className="text-[10px] text-indigo-400 hover:text-indigo-300 underline cursor-pointer"
              >
                Reconnecter
              </button>
            </div>
          )}
        </div>

        {/* Task Worktree & Branch badges */}
        <div className="flex items-center gap-2">
          {task?.branchName && (
            <div className="hidden sm:flex items-center gap-1 text-[10px] font-mono text-purple-300 bg-purple-500/10 border border-purple-500/20 px-2 py-0.5 rounded-md">
              <GitBranch size={10} />
              <span className="truncate max-w-[140px]">{task.branchName}</span>
            </div>
          )}

          <div className="flex items-center gap-1">
            <button
              type="button"
              onClick={handleInterrupt}
              title="Envoyer Ctrl+C (Interrompre le processus en cours)"
              className="px-2 py-1 bg-red-500/10 hover:bg-red-500/20 border border-red-500/30 text-red-300 text-[10px] rounded font-mono font-medium transition-colors cursor-pointer"
            >
              Ctrl+C
            </button>
            <button
              type="button"
              onClick={handleClear}
              title="Effacer le texte à l'écran"
              className="px-2 py-1 bg-slate-800 hover:bg-slate-700 border border-slate-700 text-slate-300 text-[10px] rounded font-mono transition-colors cursor-pointer"
            >
              Clear
            </button>
            <button
              type="button"
              onClick={handleResetSession}
              title="Redémarrer une nouvelle session shell vierge"
              className="flex items-center gap-1 px-2 py-1 bg-slate-800 hover:bg-slate-700 border border-slate-700 text-slate-300 text-[10px] rounded font-mono transition-colors cursor-pointer"
            >
              <RotateCcw size={10} />
              <span>Relancer le shell</span>
            </button>

            {onToggleExpand && (
              <button
                type="button"
                onClick={onToggleExpand}
                title={isExpanded ? 'Réduire la console' : 'Agrandir en plein écran'}
                className="p-1 bg-slate-800 hover:bg-slate-700 border border-slate-700 text-slate-300 rounded transition-colors cursor-pointer"
              >
                {isExpanded ? <Minimize2 size={12} /> : <Maximize2 size={12} />}
              </button>
            )}

            {onClose && (
              <button
                type="button"
                onClick={onClose}
                title="Masquer le panneau"
                className="p-1 bg-slate-800 hover:bg-slate-700 border border-slate-700 text-slate-300 hover:text-red-400 rounded transition-colors cursor-pointer"
              >
                <X size={12} />
              </button>
            )}
          </div>
        </div>
      </div>

      {/* Quick Agent Actions Bar */}
      <div className="flex items-center gap-1.5 px-3 py-1.5 bg-slate-950/80 border-b border-slate-800/60 overflow-x-auto text-[11px] scrollbar-none">
        <span className="text-slate-400 font-mono text-[10px] flex items-center gap-1 shrink-0 mr-1">
          <Zap size={11} className="text-amber-400" />
          <span>Actions rapides :</span>
        </span>

        <button
          type="button"
          onClick={handleLaunchAgent}
          className={BAR_BUTTON}
          title={
            task
              ? agentRunning
                ? `Redémarrer ${provider} dans cette session`
                : `Démarrer ${provider} dans cette session`
              : `Lancer ${provider}`
          }
        >
          {task && agentRunning ? <Check size={11} /> : <Sparkles size={11} />}
          <span className="font-semibold">
            {task && agentRunning ? `${provider} en cours` : `Lancer ${provider}`}
          </span>
        </button>

        {task &&
          skills.map(sk => {
            const command = terminalSkillCommand(provider, sk.command, proj?.skillOverrides?.[sk.id])
            return (
              <button
                key={sk.id}
                type="button"
                onClick={() => handleRunSkill(sk.id)}
                className={BAR_BUTTON}
                title={`Taper ${command} dans l'agent de cette session`}
              >
                <span>{command}</span>
              </button>
            )
          })}

        <button
          type="button"
          onClick={() => handleShellCommand('git status -s')}
          className={BAR_BUTTON}
        >
          <span>git status</span>
        </button>

        <button
          type="button"
          onClick={() => openInEditor(task ? { taskId: task.id } : { projectId, path: cwd })}
          className={BAR_BUTTON}
          title={`Ouvrir le dossier dans ${settings.editorCommand || 'VS Code'}`}
        >
          <Code2 size={11} />
          <span>{settings.editorCommand || 'code'}</span>
        </button>
      </div>

      {/* Terminal Viewport */}
      <div
        ref={terminalContainerRef}
        onMouseDown={() => {
          setTimeout(() => termRef.current?.focus(), 0)
        }}
        onClick={() => {
          termRef.current?.focus()
        }}
        className="flex-1 w-full h-full overflow-hidden bg-[#0a0f1d] cursor-text p-1"
        style={{ minHeight: '320px' }}
      />
    </div>
  )
}
