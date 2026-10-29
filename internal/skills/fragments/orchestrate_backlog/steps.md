1. Read the orchestration state. With no free slot, finish the run as completed with a note saying so: there is nothing to launch.
2. List the candidates: tickets before `implemented`, not already being worked on, without a blocker label. Skip a ticket deferred earlier and unchanged since, unless the policy says otherwise; it keeps its earlier reason.
3. Judge each remaining candidate against the policy and against each other, with the whole list in view:
   - **Ready**: a clear, actionable goal, enough technical context to start, a bounded scope, and no unresolved product or architecture decision.
   - **Dependencies**: a ticket that needs another one first waits for it; prefer the one that unblocks others.
   - **Order**: the policy's priorities first, then the board's priority and order. Say why when you depart from the board order.
4. For as many tickets as there are free slots, best first, call `launch_task_chain` with a rationale of one or two sentences that every step of the chain will read. A refusal is final for this run: record it, then try the next candidate.
5. Call `defer_task` on every other candidate you judged, with a reason written for the people who read the backlog: what is missing, or what it waits for.
6. Finish the run with `finish_run`, its note listing the launched tickets and the deferred ones, each with its reason.
