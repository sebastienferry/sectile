# #273 — Implementation plan

## Stack

- **Framework**: React 19 + TypeScript (`web/src`)
- **Icons**: `lucide-react` (specifically `Map` or `MapIcon`, and `Clock`)
- **Styling**: Tailwind CSS + CSS Variables (`accent-text`, `bg-[var(--accent-light)]`, etc.)
- **Testing**: Node test runner (`node --test tests/*.test.mjs`), `oxlint`, and TypeScript compile (`tsc -b`)

## Architecture decisions

### D1 — Navigation placement and ordering in `Sidebar.tsx`

The primary navigation sidebar organizes views logically from operational execution to higher-level planning:
1. "Mes tâches" (`isMyTasksActive`)
2. "Backlog" (`activeView === 'list'`)
3. "Board" (`activeView === 'board'`)
4. **"Roadmap"** (`activeView === 'roadmap'`) — restored here
5. **"Timeline"** (`activeView === 'timeline'`) — restored here
6. "Activités" (`activeView === 'activities'`)
7. "Skills" (`activeView === 'skills'`)
8. "Équipe" (`activeView === 'team'`)

Placing Roadmap and Timeline immediately following Board and before Activités mirrors the historical hierarchy and ergonomic placement prior to commit `0d93a04`.

### D2 — Visual consistency and iconography

To maintain consistency with existing sidebar buttons:
- **Roadmap**:
  - Icon: `Map` from `lucide-react` (size 15, `shrink-0`, amber accent `text-amber-400`).
  - Active styling: `bg-[var(--accent-light)] accent-text font-bold shadow-xs`.
  - Inactive styling: `text-[var(--text-secondary)] hover:bg-[var(--bg-tertiary)] hover:text-[var(--text-primary)]`.
  - Tooltip: `"Roadmap : NOW / NEXT / FUTURE"`.
- **Timeline**:
  - Icon: `Clock` from `lucide-react` (size 15, `shrink-0`, blue accent `text-blue-400`).
  - Active styling: `bg-[var(--accent-light)] accent-text font-bold shadow-xs`.
  - Inactive styling: `text-[var(--text-secondary)] hover:bg-[var(--bg-tertiary)] hover:text-[var(--text-primary)]`.
  - Tooltip: `"Timeline Sprints"`.

Both buttons respect `sidebarCollapsed` by hiding their text label while keeping the icon visible and centered with their respective tooltip.

### D3 — Command palette actions in `CommandPalette.tsx`

Re-add two distinct command items to the `commands` array:
1. `switch_roadmap`:
   - Title: `'🗺️ Vue Roadmap (Macros : NOW / NEXT / FUTURE)'`
   - Icon: `<Map size={16} className="text-emerald-400" />`
   - Shortcut: `'R'`
   - Keywords: `['roadmap', 'macros', 'macro', 'horizon', 'now', 'next', 'future', 'vue', 'plan']`
   - Action: `setActiveView('roadmap')`, `setIsCommandPaletteOpen(false)`
2. `switch_timeline`:
   - Title: `'⏱️ Vue Timeline Sprints'`
   - Icon: `<Clock size={16} className="text-blue-400" />`
   - Shortcut: `'TL'`
   - Keywords: `['timeline', 'sprint', 'sprints', 'planning', 'vue', 'horizons', 'duree', 'chronologie']`
   - Action: `setActiveView('timeline')`, `setIsCommandPaletteOpen(false)`

### D4 — Centralized localization in `translations.ts`

Rather than hardcoding labels in TSX, add dictionary keys to `TranslationSchema['nav']`:
- `roadmap: string`
- `timeline: string`
- `roadmapTooltip?: string`
- `timelineTooltip?: string`

Populate both French (`translations.fr.nav`) and English (`translations.en.nav`) dictionaries.

### D5 — Scope boundaries and out-of-scope protections

- No changes to backend Go code, database schemas, or API endpoints.
- No modifications to the underlying `RoadmapView.tsx` or `SprintTimelineView.tsx` components.
- Triage view and sidebar filters (macro list and tags list) remain excluded.

## Target files

| File | Nature of Change |
|---|---|
| `web/src/locales/translations.ts` | Add `roadmap` and `timeline` entries to `TranslationSchema['nav']`, `fr`, and `en` |
| `web/src/components/Sidebar.tsx` | Import `Map` and `Clock`, insert Roadmap and Timeline navigation buttons in "Vues" |
| `web/src/components/CommandPalette.tsx` | Import `Map` and `Clock`, register `switch_roadmap` and `switch_timeline` commands |
| `web/tests/navigationViews.test.mjs` | New unit test verifying navigation translations and views configuration |
