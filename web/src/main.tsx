import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import App from './App.tsx'
import { installUnauthorizedRedirect } from './lib/session'

// A session that expired must lead to the sign-in screen rather than to a
// board that silently stops loading. One wrapper carries that for every call.
installUnauthorizedRedirect()

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
