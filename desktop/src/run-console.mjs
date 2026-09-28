// What the console pane says for a run it cannot attach to.
//
// A run with no PTY is not automatically a broken run. An autonomous run has no
// terminal on purpose: nobody answers it, and what the CLI printed is recorded
// on the task activity. Reporting it as "no console available" would send the
// user looking for a launch error that never happened.
//
// Such a run can still have something to watch. When its engine reports what it
// is doing, the agent serves that trace on the same route a console is attached
// to, and the pane shows the run working instead of a sentence saying it cannot
// be answered. An agent that does not send a trace — an older one, or an engine
// whose stream is not read — keeps the notice.

// tracedRun says whether an autonomous run has a trace to attach to.
function tracedRun(run){
 return run.headless===true&&run.trace===true
}

// needsConsoleNotice says whether the pane shows a message instead of attaching.
export function needsConsoleNotice(run){
 if(run.status==='queued'||run.status==='preparing')return true
 if(run.headless===true)return !tracedRun(run)
 return !run.sessionId
}

// readOnlyConsole says whether what the pane attached to only shows the run.
// Nobody is answering an autonomous run, so its pane is never given the focus:
// a cursor waiting in it is an invitation to type at a process with no ear.
export function readOnlyConsole(run){
 return tracedRun(run)
}

export function consoleNotice(run){
 if(run.status==='queued')return 'Execution queued. Waiting for a console.'
 if(run.status==='preparing')return 'Preparing execution. Waiting for a console.'
 if(run.headless===true)return 'Autonomous execution: no terminal to answer. Its output is recorded on the task activity.'
 if(run.status==='canceled')return 'Execution canceled before a console was created.'
 return 'No console is available for this execution. Check the task activity and local agent.log for launch errors.'
}
