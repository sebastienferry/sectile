// When a run's skill result is read again (#908): a port of the Electron
// client's `desktop/src/skill-result-refresh.mjs`, to be changed together.
//
// A skill result costs the agent two reads of the server: the task and every
// activity of it. The run list is polled every three seconds, and reading each
// visible row's result at that pace would multiply server requests by the rows
// of the sidebar. A result is read again only when something can have changed it.
import {hasWaitMark} from "./execution.ts";
import type {RunResult} from "./workflow.ts";

// Right after a run changes, its result is read at every poll for a while: a
// skill ends its run, then moves its task, and the second step lands a moment
// after the first. Past that, a live run's result is read again at most every
// LIVE_REFRESH_MS and an ended run's every ENDED_REFRESH_MS, which still
// follows a task moved on elsewhere.
export const SETTLE_MS=30_000;
export const LIVE_REFRESH_MS=30_000;
export const ENDED_REFRESH_MS=120_000;

const LIVE=new Set(["queued","pending","preparing","running","waiting"]);
const ENDED=new Set(["completed","failed","canceled"]);

/** What a surface knows of a run when it decides to read its result. */
export interface RefreshRun {status:string;waitingSince?:string|null}
/** What was recorded when the result was last read. */
export interface SkillResultStamp {status:string;waitingSince:string;at:number;changedAt:number;ended:boolean}

// A conversation run may carry the Go zero time as its wait mark: that is no
// mark, and never a change.
const waitMark=(run:RefreshRun)=>hasWaitMark(run.waitingSince)?run.waitingSince as string:"";

// last is what was recorded when the result was last read: the run's status,
// its wait mark, when it was read and when the run last changed. A change of
// either is what a result follows.
//
// A console still open after its skill ended is the exception: the next skill
// started in it changes nothing on the run itself, so its result is read at
// every poll for the status to drop the ended skill's verdict at once (#586).
export function skillResultDue(run:RefreshRun,last:SkillResultStamp|undefined,now:number){
 if(!last)return true;
 if(last.status!==run.status||last.waitingSince!==waitMark(run))return true;
 if(now-last.changedAt<SETTLE_MS)return true;
 if(last.ended&&LIVE.has(run.status))return true;
 return now-last.at>=(LIVE.has(run.status)?LIVE_REFRESH_MS:ENDED_REFRESH_MS);
}

// The stamp of a read. The run's change keeps its time while the run stays as
// it was, so the settling window starts at the change, not at each read. ended
// says the result read showed the run's own skill ended.
export function skillResultStamp(run:RefreshRun,now:number,last?:SkillResultStamp,ended=false):SkillResultStamp{
 const same=!!last&&last.status===run.status&&last.waitingSince===waitMark(run);
 return {status:run.status,waitingSince:waitMark(run),at:now,changedAt:same&&last?last.changedAt:now,ended};
}

// Whether a result read from the agent shows the run's own skill ended. From
// then on, whatever its console goes on to run, waits included, changes
// nothing on the run itself.
export function skillEnded(result?:RunResult){
 return ENDED.has(result?.activity?.status??"");
}
