// The offer to make a project folder a Git repository (#481): which folder
// states it applies to, and what it says. Kept apart from main.js so the texts
// are tested without Electron.

const TITLES={
 folder:'This folder is not a Git repository.',
 unborn:'This Git repository has no commit yet.'
}

export const GIT_INIT_DETAIL='Sectile can initialize it with an empty first commit so it can create worktrees. The repository stays on this workstation: it is never pushed and needs no remote. Nothing in the folder is committed, so worktrees start without its existing files.'

// offerFor answers the offer shown for a folder state reported by the agent,
// or null when there is nothing to offer: a ready checkout, a folder that
// does not exist, or an agent too old to tell.
export function offerFor(state){
 const title=TITLES[state]
 return title?{title,detail:GIT_INIT_DETAIL,action:'Initialize a Git repository',dismiss:'Not now'}:null
}

export function initializedNotice(path){
 return 'Git repository initialized in '+path+'. Save the local configuration to use it.'
}
