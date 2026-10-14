const {test}=require('node:test')
const assert=require('node:assert/strict')
const {version,compare,archive,safeLink,select,createDesktopUpdates,DAY}=require('../electron/desktop-updates.cjs')
const base='https://github.com/sebastienferry/sectile/releases/'
function release(tag='v0.10.0',platform='darwin',arch='arm64'){
 const name=archive(platform,arch)
 return {tag_name:tag,html_url:base+'tag/'+tag,assets:[{name,size:12,browser_download_url:base+'download/'+tag+'/'+name}]}
}
const response=(data,status=200,headers={})=>new Response(JSON.stringify(data),{status,headers})
function fixture(options={}){
 let saved={},notifications=[],events=[],calls=[]
 const service=createDesktopUpdates({installedVersion:'0.9.0',platform:'darwin',arch:'arm64',storage:{read:()=>saved,write:value=>{saved=structuredClone(value)}},notify:r=>notifications.push(r),publish:s=>events.push(s),fetch:async(url,opts)=>{calls.push({url,opts});return response([release()])},...options})
 return {service,notifications,events,calls,saved:()=>saved}
}
test('strict stable numeric versions and supported archive selection',()=>{
 assert.equal(compare(version('0.10.0'),version('0.9.0')),1)
 assert.deepEqual(version('v1.2.3+build.4'),version('1.2.3'))
 for(const invalid of ['01.2.3','1.2','main','1.2.3-beta','1.2.3+'])assert.equal(version(invalid),null)
 for(const [p,a] of [['darwin','arm64'],['darwin','x64'],['linux','x64'],['win32','x64']])assert.equal(select([release('v0.10.0',p,a)],'0.9.0',p,a).available.version,'0.10.0')
 assert.equal(select([release()],'0.9.0','linux','arm64').status,'unavailable')
 assert.equal(select([release()],'unknown','darwin','arm64').status,'uncomparable')
 for(const installed of ['0.10.0','0.11.0'])assert.equal(select([release()],installed,'darwin','arm64').status,'up-to-date')
 const incomplete=release('v0.12.0');incomplete.assets[0].size=0
 const result=select([release(),{...release('v0.13.0'),draft:true},{...release('v0.14.0'),prerelease:true},release('main'),incomplete],'0.9.0','darwin','arm64')
 assert.equal(result.available.version,'0.10.0');assert.equal(result.unavailableVersion,'0.12.0')
})
test('only repository HTTPS release links are allowed',()=>{
 assert.ok(safeLink(release().html_url));assert.ok(safeLink(release().assets[0].browser_download_url,true))
 for(const value of ['http://github.com/sebastienferry/sectile/releases/tag/v1','https://evil.test/','https://user@github.com/sebastienferry/sectile/releases/tag/v1',base+'tag/v1?token=secret',base+'../issues'])assert.equal(safeLink(value),false)
})
test('persistence, deduplication, stale errors and successful clearing',async()=>{
 const f=fixture();await f.service.check();await f.service.check();assert.equal(f.notifications.length,1)
 const stored=f.saved();f.service.stop()
 const next=fixture({storage:{read:()=>stored,write:()=>{}}})
 assert.equal(next.service.snapshot().available.version,'0.10.0');await next.service.check();assert.equal(next.notifications.length,0);next.service.stop()
 const offline=fixture({storage:{read:()=>stored,write:()=>{}},fetch:async()=>{throw Error('offline')}})
 await offline.service.check();assert.equal(offline.service.snapshot().status,'error');assert.ok(offline.service.snapshot().available);offline.service.stop()
 const current=fixture({installedVersion:'0.10.0',storage:{read:()=>stored,write:()=>{}},fetch:async()=>response([])})
 assert.equal(current.service.snapshot().available,null);await current.service.check();assert.equal(current.service.snapshot().status,'up-to-date');current.service.stop()
})
test('failed persistence suppresses notifications; corrupt storage does not block checks',async()=>{
 const f=fixture({storage:{read:()=>{throw Error('corrupt')},write:()=>{throw Error('disk')}}})
 await f.service.check();assert.equal(f.service.snapshot().error,'storage');assert.equal(f.notifications.length,0);assert.ok(f.service.snapshot().available);f.service.stop()
})
test('requests coalesce and contain no credentials',async()=>{
 let resolve;const pending=new Promise(r=>{resolve=r});const f=fixture({fetch:()=>pending})
 const first=f.service.check();assert.equal(first,f.service.check());resolve(response([release()]));await first;f.service.stop()
 const g=fixture();await g.service.check();assert.equal(g.calls[0].opts.redirect,'error');assert.equal(g.calls[0].opts.headers.Authorization,undefined);g.service.stop()
})
test('malformed, oversized, offline, timeout and throttle outcomes remain errors',async()=>{
 for(const fetch of [async()=>new Response('no'),async()=>response({}),async()=>new Response('[]',{headers:{'content-length':3000000}}),async()=>new Response('x'.repeat(2100000)),async()=>{throw Error('offline')},()=>new Promise(()=>{})]){
  const f=fixture({fetch,timeout:5});await f.service.check();assert.equal(f.service.snapshot().status,'error');f.service.stop()
 }
 for(const status of [403,429]){
  let calls=0;const f=fixture({fetch:async()=>{calls++;return response([],status,{'retry-after':'120'})}})
  await f.service.check();await f.service.check();assert.equal(calls,1);assert.equal(f.service.snapshot().error,'throttled');f.service.stop()
 }
})
test('bounded pagination selects highest numeric version and refuses incomplete traversal',async()=>{
 let calls=0
 const f=fixture({fetch:async()=>response(++calls===1?Array.from({length:100},()=>release()):[release('v0.11.0')])})
 await f.service.check();assert.equal(calls,2);assert.equal(f.service.snapshot().available.version,'0.11.0');f.service.stop()
 calls=0;const g=fixture({fetch:async()=>{calls++;return response(Array.from({length:100},()=>release()))}})
 await g.service.check();assert.equal(calls,10);assert.equal(g.service.snapshot().error,'invalid-response');g.service.stop()
})
test('startup, daily scheduling, resume and stop use one deadline',async()=>{
 let time=1000,timers=[],calls=0
 const f=fixture({now:()=>time,setTimer:(fn,delay)=>{const t={fn,delay};timers.push(t);return t},clearTimer:()=>{},fetch:async()=>{calls++;return response([])}})
 await f.service.start();assert.equal(calls,1);assert.equal(timers.at(-1).delay,DAY)
 time+=DAY-1;await f.service.resume();assert.equal(calls,1)
 time+=2;await f.service.resume();assert.equal(calls,2)
 time+=DAY*3;await f.service.resume();assert.equal(calls,3);f.service.stop()
})
test('new versions notify once even when native notification delivery throws',async()=>{
 let latest='v0.10.0',attempts=0
 const f=fixture({fetch:async()=>response([release(latest)]),notify:()=>{attempts++;throw Error('denied')}})
 await f.service.check();await f.service.check();assert.equal(attempts,1)
 latest='v0.11.0';await f.service.check();assert.equal(attempts,2);assert.equal(f.service.snapshot().available.version,'0.11.0');f.service.stop()
})
