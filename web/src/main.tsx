import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import App from './App.tsx'
import { installUnauthorizedRedirect } from './lib/session'
import { applyDocumentLocale, resolveInitialLocale } from './lib/i18n'
import { translations } from './locales/translations'

// A session that expired must lead to the sign-in screen rather than to a
// board that silently stops loading. One wrapper carries that for every call.
installUnauthorizedRedirect()

// The page speaks the remembered language from the first paint, the loading
// and sign-in screens included; the settings take over once they arrive.
const startupLocale = resolveInitialLocale()
applyDocumentLocale(startupLocale, translations[startupLocale].app.documentTitle)

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
