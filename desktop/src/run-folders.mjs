// What the "Add folder…" action of a run says once the agent answered (#676).
// answer is the agent's {mappedAs, typed, appliesAt}.
export function runFolderOutcome(path,answer){
 const when=answer?.appliesAt==='now'?'typed /add-dir into the session':answer?.appliesAt==='next-turn'?'Claude sees it from your next message':'the discussion sees it at its next launch'
 if(answer?.mappedAs)return path+' is a checkout of '+answer.mappedAs+': it is now that repository\'s folder; '+when
 return answer?.appliesAt==='now'?'Attached '+path+' and '+when:'Attached '+path+': '+when
}

// Whether a run offers the action: a conversation that is not read-only, or a
// ticket discussion running in a Sectile terminal, on an agent that serves it.
export function offersRunFolder(run,available){
 if(!available||!run)return false
 if(run.conversation)return run.status==='running'
 return run.skill==='discuss'&&run.status==='running'&&!!run.sessionId&&!run.externalTerminal&&!run.headless
}
