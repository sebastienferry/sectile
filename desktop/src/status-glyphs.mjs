import {runStateSvg} from '../../shared/runStates.ts'

// The glyph of each kind of primary status (#878). Every kind but one reuses a
// shared run-state glyph, so the row, the notification and the web client keep
// drawing the same picture for the same situation. Needs verification has no
// run state: its glyph lives here, never in shared/, so it cannot leak into the
// web client or the notifications.
const SHARED={working:'running',waiting:'waiting',done:'completed',failed:'failed',stopped:'canceled',stopping:'queued'}
const VERIFY_COLOR='#c2a878'

// The id drawn for a status: the shared run state, `silent` for a working
// session that went quiet, `queued` for one not started yet, `verify` for the
// Desktop-only glyph. It goes in data-run-state, so the running pulse keeps
// applying to Working.
export function statusGlyphId(status){
 if(status.kind==='working'&&status.detail==='No output for a while')return 'silent'
 if(status.kind==='working'&&['Queued','Preparing'].includes(status.detail))return 'queued'
 return SHARED[status.kind]||'verify'
}

export function statusGlyphSvg(status,size=14){
 const id=statusGlyphId(status)
 if(id!=='verify')return runStateSvg(id,size)
 // lucide circle-help, drawn as the shared glyphs are.
 return `<svg xmlns="http://www.w3.org/2000/svg" width="${size}" height="${size}" viewBox="0 0 24 24" color="${VERIFY_COLOR}" fill="none" stroke="${VERIFY_COLOR}" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">`
  +'<circle cx="12" cy="12" r="10"/><path d="M9.09 9a3 3 0 0 1 5.83 1c0 2-3 3-3 3"/><path d="M12 17h.01"/></svg>'
}

// The text a tooltip and an accessible label carry for a status: what the user
// reads first, then the signals it was drawn from.
export function statusText(status){
 return [status.label,status.detail,status.next].filter(Boolean).join(' · ')+'\n'+status.diagnostics
}
