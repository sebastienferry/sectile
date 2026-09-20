# #273 — Implementation checklist

Ordered checklist ensuring clean typing and localization before UI component updates, followed by automated test verification.

## 1. Localization & dictionary updates

- [ ] **T1** In `web/src/locales/translations.ts`, add `roadmap: string` and `timeline: string` to `TranslationSchema['nav']`.
- [ ] **T2** In `web/src/locales/translations.ts`, populate `roadmap: 'Roadmap'` and `timeline: 'Timeline'` in `translations.fr.nav` and `translations.en.nav`.
- [ ] **T3** Create `web/tests/navigationViews.test.mjs` asserting that `nav.roadmap` and `nav.timeline` exist, are non-empty strings, and are properly localized in both French and English dictionaries.

## 2. Primary navigation sidebar

- [ ] **T4** In `web/src/components/Sidebar.tsx`, import `Map` and `Clock` from `lucide-react`.
- [ ] **T5** In the "Vues" section of `Sidebar.tsx`, add the Roadmap navigation button between "Board" and "Activités", with `activeView === 'roadmap'` styling, `text-amber-400` icon, and collapsed sidebar handling.
- [ ] **T6** In the "Vues" section of `Sidebar.tsx`, add the Timeline navigation button immediately following Roadmap and preceding "Activités", with `activeView === 'timeline'` styling, `text-blue-400` icon, and collapsed sidebar handling.

## 3. Command palette

- [ ] **T7** In `web/src/components/CommandPalette.tsx`, import `Map` and `Clock` from `lucide-react`.
- [ ] **T8** Add the `switch_roadmap` command entry with shortcut `R`, relevant keywords (`['roadmap', 'macros', 'macro', 'horizon', 'now', 'next', 'future', 'vue', 'plan']`), and action switching `activeView` to `'roadmap'`.
- [ ] **T9** Add the `switch_timeline` command entry with shortcut `TL`, relevant keywords (`['timeline', 'sprint', 'sprints', 'planning', 'vue', 'horizons', 'duree', 'chronologie']`), and action switching `activeView` to `'timeline'`.

## 4. Verification and validation

- [ ] **T10** Run test suite with `npm test` in `web/` and verify all tests pass, including the new navigation localization tests.
- [ ] **T11** Run `npm run lint` (`oxlint`) and `npm run build` (`tsc -b && vite build`) to prove type soundness and build integrity.
- [ ] **T12** Verify manually/visually that clicking the sidebar buttons and executing the command palette actions transitions seamlessly to Roadmap and Timeline views.
