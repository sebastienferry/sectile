import React, { useEffect } from 'react'
import { Shield, X } from 'lucide-react'
import { useApp } from '../context/AppContext'
import { useCurrentUser } from '../hooks/useCurrentUser'
import { UsersPanel } from './UsersPanel'

/**
 * Dedicated administration modal window, accessible strictly to users holding
 * the 'admin' role. Renders user management, role assignment, and permissions.
 */
export const AdminModal: React.FC = () => {
  const { isAdminOpen, setIsAdminOpen } = useApp()
  const { user: currentUser } = useCurrentUser()

  useEffect(() => {
    if (!isAdminOpen) return
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        setIsAdminOpen(false)
      }
    }
    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [isAdminOpen, setIsAdminOpen])

  // Strictly refuse display if the modal is not requested or if the user is not an admin
  if (!isAdminOpen || currentUser?.role !== 'admin') {
    return null
  }

  return (
    <div
      className="fixed top-0 left-0 h-[var(--app-h)] w-[var(--app-w)] z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-xs animate-in fade-in duration-200"
      onClick={e => {
        if (e.target === e.currentTarget) setIsAdminOpen(false)
      }}
    >
      <div
        className="relative w-full max-w-2xl rounded-2xl bg-[var(--bg-secondary)] border border-[var(--border-color)] shadow-2xl overflow-hidden flex flex-col max-h-[calc(var(--app-h)*0.92)] animate-in zoom-in-95 duration-150"
        role="dialog"
        aria-modal="true"
        aria-labelledby="admin-modal-title"
      >
        {/* Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-[var(--border-color)] bg-[var(--bg-tertiary)]/30 shrink-0">
          <div className="flex items-center gap-2.5">
            <div className="w-8 h-8 rounded-xl bg-amber-500/20 text-amber-500 flex items-center justify-center shadow-xs">
              <Shield size={16} />
            </div>
            <div>
              <h3 id="admin-modal-title" className="text-sm font-bold text-[var(--text-primary)]">
                Administration
              </h3>
              <p className="text-[11px] text-[var(--text-muted)]">
                Gestion des utilisateurs, des rôles et des autorisations d'accès
              </p>
            </div>
          </div>
          <button
            type="button"
            onClick={() => setIsAdminOpen(false)}
            aria-label="Fermer"
            className="p-1.5 rounded-lg text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-tertiary)] transition-colors cursor-pointer"
          >
            <X size={17} />
          </button>
        </div>

        {/* Body */}
        <div className="p-6 overflow-y-auto space-y-6 flex-1 text-xs">
          <UsersPanel currentUserId={currentUser.userId} embedded />
        </div>
      </div>
    </div>
  )
}
