// The project a run works for (#741), on the renderer side: the mode a pickup
// is launched with. Every desktop launch names its project, so the server's
// refusal of a ticket of several projects launched without one never reaches
// the desktop.

// A pickup runs unattended: it names the autonomous mode explicitly, since the
// server treats a launch as unattended only on an explicit mode=autonomous or a
// batch, whatever the project's default mode.
export const PICKUP_MODE='autonomous'

// The mode a launch sends: a pickup's is always autonomous, any other launch
// keeps the one chosen, empty for none.
export function launchModeFor(kind,mode){
 return kind==='pickup'?PICKUP_MODE:mode
}
