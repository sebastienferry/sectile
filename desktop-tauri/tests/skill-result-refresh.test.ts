import test from "node:test";
import assert from "node:assert/strict";
import {ENDED_REFRESH_MS,LIVE_REFRESH_MS,SETTLE_MS,skillEnded,skillResultDue,skillResultStamp} from "../src/domain/skill-result-refresh.ts";

const T=1_000_000;

test("a result never read is due",()=>{
 assert.equal(skillResultDue({status:"running"},undefined,T),true);
});

test("a change of status or of wait mark is due at once",()=>{
 const last=skillResultStamp({status:"running"},T);
 const later=T+SETTLE_MS+1;
 assert.equal(skillResultDue({status:"completed"},{...last,changedAt:0},later),true);
 assert.equal(skillResultDue({status:"running",waitingSince:"2026-11-15T10:00:00Z"},{...last,changedAt:0},later),true);
});

test("a zero-date wait mark is no change",()=>{
 const last={...skillResultStamp({status:"running"},T),changedAt:0};
 assert.equal(skillResultDue({status:"running",waitingSince:"0001-01-01T00:00:00Z"},last,T+1),false);
 assert.equal(skillResultStamp({status:"running",waitingSince:"0001-01-01T00:00:00Z"},T).waitingSince,"");
});

test("every poll is due during the settling window after a change",()=>{
 const last=skillResultStamp({status:"completed"},T);
 assert.equal(skillResultDue({status:"completed"},{...last,at:T+SETTLE_MS-2},T+SETTLE_MS-1),true);
 assert.equal(skillResultDue({status:"completed"},{...last,at:T+SETTLE_MS-1},T+SETTLE_MS),false);
});

test("a live run whose own skill ended is read at every poll to notice a successor",()=>{
 const last={...skillResultStamp({status:"running"},T,undefined,true),changedAt:0};
 assert.equal(skillResultDue({status:"running"},last,T+1),true);
 assert.equal(skillResultDue({status:"running"},{...last,ended:false},T+1),false);
});

test("live and ended runs are read again at their own intervals",()=>{
 const live={...skillResultStamp({status:"running"},T),changedAt:0};
 assert.equal(skillResultDue({status:"running"},live,T+LIVE_REFRESH_MS-1),false);
 assert.equal(skillResultDue({status:"running"},live,T+LIVE_REFRESH_MS),true);
 assert.equal(skillResultDue({status:"waiting"},{...skillResultStamp({status:"waiting"},T),changedAt:0},T+LIVE_REFRESH_MS),true);
 const ended={...skillResultStamp({status:"completed"},T),changedAt:0};
 assert.equal(skillResultDue({status:"completed"},ended,T+LIVE_REFRESH_MS),false);
 assert.equal(skillResultDue({status:"completed"},ended,T+ENDED_REFRESH_MS),true);
});

test("a stamp keeps the time of the change while the run stays as it was",()=>{
 const first=skillResultStamp({status:"running"},T);
 const second=skillResultStamp({status:"running"},T+5000,first,true);
 assert.deepEqual(second,{status:"running",waitingSince:"",at:T+5000,changedAt:T,ended:true});
 assert.equal(skillResultStamp({status:"completed"},T+6000,second).changedAt,T+6000);
});

test("a result shows the run's own skill ended once its record is terminal",()=>{
 assert.equal(skillEnded(undefined),false);
 assert.equal(skillEnded({activity:{id:"r",taskId:"t",skillId:"clarify",status:"running"}}),false);
 for(const status of ["completed","failed","canceled"])assert.equal(skillEnded({activity:{id:"r",taskId:"t",skillId:"clarify",status}}),true);
});
