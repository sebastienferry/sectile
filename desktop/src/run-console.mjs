// What the console pane says for a run it cannot attach to.
//
// A run with no PTY is not automatically a broken run. An autonomous run has no
// terminal on purpose: nobody answers it, and what the CLI printed is recorded
// on the task activity. Reporting it as "no console available" would send the
// user looking for a launch error that never happened.

// needsConsoleNotice says whether the pane shows a message instead of attaching.
export function needsConsoleNotice(run){
 return run.status==='queued'||run.status==='preparing'||run.headless===true||!run.sessionId
}

export function consoleNotice(run){
 if(run.status==='queued')return 'Execution queued. Waiting for a console.'
 if(run.status==='preparing')return 'Preparing execution. Waiting for a console.'
 if(run.headless===true)return 'Autonomous execution: no terminal to answer. Its output is recorded on the task activity.'
 if(run.status==='canceled')return 'Execution canceled before a console was created.'
 return 'No console is available for this execution. Check the task activity and local agent.log for launch errors.'
}
