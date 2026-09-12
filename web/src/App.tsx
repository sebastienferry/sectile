import React from 'react'
import { AppProvider, useApp } from './context/AppContext'
import { Sidebar } from './components/Sidebar'
import { Header } from './components/Header'
import { BoardView } from './components/BoardView'
import { ListView } from './components/ListView'
import { TriageView } from './components/TriageView'
import { RoadmapView } from './components/RoadmapView'
import { SprintTimelineView } from './components/SprintTimelineView'
import { TeamView } from './components/TeamView'
import { TrackerSetup } from './components/TrackerSetup'
import { ActivitiesView } from './components/ActivitiesView'
import { SyncView } from './components/SyncView'
import { DigestView } from './components/DigestView'
import { SkillsView } from './components/SkillsView'
import { PinnedBar } from './components/PinnedBar'
import { QuickAddModal } from './components/QuickAddModal'
import { CloneTaskModal } from './components/CloneTaskModal'
import { TaskDetailModal } from './components/TaskDetailModal'
import { CommandPalette } from './components/CommandPalette'
import { ProfileModal } from './components/ProfileModal'
import { ProjectModal } from './components/ProjectModal'
import { GitDiffModal } from './components/GitDiffModal'
import { BranchSwitcherModal } from './components/BranchSwitcherModal'
import { StatusBar } from './components/StatusBar'
import { WorkspaceTerminalPanel } from './components/WorkspaceTerminalPanel'
import { ToastContainer } from './components/ToastContainer'
import { Loader2 } from 'lucide-react'

const MainContent: React.FC = () => {
  const {
    activeView,
    isLoading,
    error,
    tasks,
    isTrackerSetupOpen,
    setIsTrackerSetupOpen,
    isTerminalPanelOpen,
    terminalDockPosition,
  } = useApp()

  const dockPos = terminalDockPosition || 'right'

  return (
    <>
    <div className="flex flex-col h-[var(--app-h)] w-[var(--app-w)] overflow-hidden bg-[var(--bg-primary)]">
      <div className="flex flex-1 overflow-hidden min-h-0">
        {/* Navigation Sidebar */}
        <Sidebar />

        {/* Docked workspace CLI on LEFT */}
        {isTerminalPanelOpen && dockPos === 'left' && (
          <WorkspaceTerminalPanel position="left" />
        )}

        {/* Main Workspace Area */}
        <div className="flex-1 flex flex-col h-full overflow-hidden min-w-0 min-h-0">
          <Header />

          {/* Bascule à chaud entre les tickets épinglés */}
          <PinnedBar />

          {/* Dynamic View Body */}
          <main className="flex-1 flex flex-col overflow-hidden relative min-h-0">
            {isLoading && tasks.length === 0 ? (
              <div className="flex-1 flex flex-col items-center justify-center gap-3 text-[var(--text-muted)]">
                <Loader2 size={28} className="animate-spin text-[var(--accent-color)]" />
                <span className="text-xs font-medium">Chargement des tâches...</span>
              </div>
            ) : error ? (
              <div className="flex-1 flex flex-col items-center justify-center p-6 text-center text-rose-400">
                <p className="text-sm font-semibold mb-2">Erreur de connexion</p>
                <p className="text-xs text-[var(--text-muted)] max-w-md">{error}</p>
              </div>
            ) : activeView === 'board' ? (
              <BoardView />
            ) : activeView === 'list' ? (
              <ListView />
            ) : activeView === 'triage' ? (
              <TriageView />
            ) : activeView === 'roadmap' ? (
              <RoadmapView />
            ) : activeView === 'timeline' ? (
              <SprintTimelineView />
            ) : activeView === 'team' ? (
              <TeamView />
            ) : activeView === 'sync' ? (
              <SyncView />
            ) : activeView === 'digest' ? (
              <DigestView />
            ) : activeView === 'skills' ? (
              <SkillsView />
            ) : (
              <ActivitiesView />
            )}
          </main>

          {/* Docked workspace CLI on BOTTOM */}
          {isTerminalPanelOpen && dockPos === 'bottom' && (
            <WorkspaceTerminalPanel position="bottom" />
          )}
        </div>

        {/* Docked workspace CLI on RIGHT */}
        {isTerminalPanelOpen && dockPos === 'right' && (
          <WorkspaceTerminalPanel position="right" />
        )}
      </div>

      {/* Global Bottom Status Bar with CWD Git Branch, Project, Engine & Live Jobs */}
      <StatusBar />

      {/* Global Modals & Overlays */}
      <QuickAddModal />
      <CloneTaskModal />
      <TaskDetailModal />
      <ProjectModal />
      <GitDiffModal />
      <BranchSwitcherModal />
      <CommandPalette />
      <ProfileModal />
      <ToastContainer />
    </div>

    {/* Connexion au tracker : jamais au démarrage, seulement quand un projet
        vient d'être posé sur un tracker distant sans accès configurés, ou à la
        demande depuis les réglages. Elle se ferme sans rien remplir. */}
    {isTrackerSetupOpen && <TrackerSetup onClose={() => setIsTrackerSetupOpen(false)} />}
    </>
  )
}

export function App() {
  return (
    <AppProvider>
      <MainContent />
    </AppProvider>
  )
}

export default App
