// What the console pane says for a run it cannot attach to.
//
// A run with no PTY is not automatically a broken run. An autonomous run has no
// terminal on purpose: nobody answers it, and what the CLI printed is recorded
// on the task activity. Reporting it as "no console available" would send the
// user looking for a launch error that never happened.
//
// A headless run still gets no PTY, so needsConsoleNotice keeps returning true
// for it, but the pane no longer stops at a notice: select() writes the banner
// below and then polls the agent for what the run has printed.

// needsConsoleNotice says whether the pane shows a message instead of attaching.
export function needsConsoleNotice(run){
 return run.status==='queued'||run.status==='preparing'||run.headless===true||!run.sessionId
}

// showsHeadlessOutput says whether the pane reads the run's transcript instead
// of writing a notice. A run still waiting for its slot has printed nothing and
// is not yet autonomous in any visible way: it keeps the waiting notice, the
// same order consoleNotice reads its branches in.
export function showsHeadlessOutput(run){
 return run.headless===true&&run.status!=='queued'&&run.status!=='preparing'
}

// headlessBanner heads the read-only transcript. It says the pane takes no
// input before any output appears, so a user who types into it knows why
// nothing happens, and names the activity as the durable record.
export const HEADLESS_EMPTY='Nothing printed yet.'
export function headlessBanner(empty=false){
 const head='Autonomous execution: read-only, no terminal to answer. Its output is also recorded on the task activity.'
 return empty?head+'\n'+HEADLESS_EMPTY:head
}

export function consoleNotice(run){
 if(run.status==='queued')return 'Execution queued. Waiting for a console.'
 if(run.status==='preparing')return 'Preparing execution. Waiting for a console.'
 if(run.headless===true)return headlessBanner()
 if(run.status==='canceled')return 'Execution canceled before a console was created.'
 return 'No console is available for this execution. Check the task activity and local agent.log for launch errors.'
}
