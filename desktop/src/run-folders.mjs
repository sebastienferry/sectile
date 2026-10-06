// What the "Add folder…" action of a run says once the agent answered (#676).
// answer is the agent's {mappedAs, typed, appliesAt}; run says whether it is a
// free console and names the native terminal it was detached to, if any (#689).
export function runFolderOutcome(path,answer,run){
 const later=run?.console?'a new Project prompt or a relaunch sees it':'the discussion sees it at its next launch'
 const typed='typed /add-dir into the session'+(run?.terminal?' in '+run.terminal:'')
 const when=answer?.appliesAt==='now'?typed:answer?.appliesAt==='next-turn'?'Claude sees it from your next message':later
 if(answer?.mappedAs)return path+' is a checkout of '+answer.mappedAs+': it is now that repository\'s folder; '+when
 return answer?.appliesAt==='now'?'Attached '+path+' and '+when:'Attached '+path+': '+when
}

// Whether a run offers the action: a conversation that is not read-only, or a
// ticket discussion running in a Sectile terminal, on an agent that serves it.
// A free console and a run detached to the native terminal need an agent that
// serves them too (terminals, #689).
export function offersRunFolder(run,available,terminals){
 if(!available||!run)return false
 if(run.conversation)return run.status==='running'
 if(run.status!=='running'||!run.sessionId||run.headless)return false
 if(run.kind==='console')return !!terminals
 return run.skill==='discuss'&&(!run.externalTerminal||!!terminals)
}
