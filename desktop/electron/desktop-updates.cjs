const DAY=24*60*60*1000
const API='https://api.github.com/repos/sebastienferry/sectile/releases'
function version(value){
 if(typeof value!=='string')return null
 const match=/^v?(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$/.exec(value)
 return match?match.slice(1,4).map(BigInt):null
}
function compare(a,b){for(let i=0;i<3;i++){if(a[i]>b[i])return 1;if(a[i]<b[i])return -1}return 0}
function safeLink(value,download=false){
 try{
  const u=new URL(value)
  const prefix='/sebastienferry/sectile/releases/'
  return u.protocol==='https:'&&u.hostname==='github.com'&&!u.port&&!u.username&&!u.password&&!u.search&&!u.hash&&
   (download?new RegExp('^'+prefix+'download/v?[^/]+/sectile-desktop-[a-z0-9.-]+$'):new RegExp('^'+prefix+'tag/v?[^/]+$')).test(u.pathname)
 }catch{return false}
}
function archive(platform,arch){return `sectile-desktop-${platform==='win32'?'windows':platform}-${arch==='x64'?'amd64':arch}.${platform==='linux'?'tar.gz':'zip'}`}
function select(releases,installed,platform,arch){
 const own=version(installed)
 if(!own)return {status:'uncomparable',available:null,unavailableVersion:null}
 let available=null,unavailableVersion=null
 const ordered=releases.filter(r=>r&& !r.draft&&!r.prerelease&&version(r.tag_name)).sort((a,b)=>compare(version(b.tag_name),version(a.tag_name)))
 for(const r of ordered){
  if(compare(version(r.tag_name),own)<=0)continue
  const normalized=version(r.tag_name).join('.')
  const releasePath=`/sebastienferry/sectile/releases/`
  const filename=archive(platform,arch)
  const asset=Array.isArray(r.assets)&&r.assets.find(a=>a?.name===filename&&Number.isFinite(a.size)&&a.size>0&&safeLink(a.browser_download_url,true)&&new URL(a.browser_download_url).pathname===`${releasePath}download/${r.tag_name}/${filename}`)
  const compatible=asset&&safeLink(r.html_url)&&new URL(r.html_url).pathname===`${releasePath}tag/${r.tag_name}`
  if(compatible&&!available)available={version:normalized,releaseUrl:r.html_url,downloadUrl:asset.browser_download_url}
  else if(!compatible&&!unavailableVersion)unavailableVersion=normalized
 }
 if(available&&unavailableVersion&&compare(version(unavailableVersion),version(available.version))<=0)unavailableVersion=null
 return {status:available?'available':unavailableVersion?'unavailable':'up-to-date',available,unavailableVersion}
}
function createDesktopUpdates({installedVersion,platform=process.platform,arch=process.arch,fetch:request=globalThis.fetch,storage,notify=()=>{},publish=()=>{},now=Date.now,setTimer=setTimeout,clearTimer=clearTimeout,timeout=15000}){
 let state={status:'idle',installedVersion,checkedAt:null,lastAttemptAt:null,available:null,unavailableVersion:null,error:null}
 let notified=new Set(),lastSuccess=null,inflight=null,timer=null,controller=null,stopped=false,nextDue=0,cooldown=0
 try{
  const saved=storage?.read()
  if(Array.isArray(saved?.notified))notified=new Set(saved.notified.filter(v=>version(v)).map(v=>version(v).join('.')))
  const s=saved?.lastSuccess
  if(s&&Number.isFinite(s.checkedAt)&&Array.isArray(s.releases)){
   lastSuccess=s;state={...state,...select(s.releases,installedVersion,platform,arch),checkedAt:s.checkedAt}
  }
 }catch{}
 function snapshot(){return structuredClone(state)}
 function emit(){publish(snapshot())}
 function save(){storage?.write({lastSuccess,notified:[...notified]})}
 function schedule(){
  clearTimer(timer)
  if(!stopped){
   timer=setTimer(()=>{if(now()<Math.max(nextDue,cooldown))schedule();else check()},Math.min(2147483647,Math.max(1,Math.max(nextDue,cooldown)-now())))
   timer?.unref?.()
  }
 }
 async function retrieve(signal){
  const releases=[]
  for(let page=1;page<=10;page++){
   const response=await request(`${API}?per_page=100&page=${page}`,{signal,redirect:'error',headers:{Accept:'application/vnd.github+json','User-Agent':'Sectile-Desktop'}})
   if(response.status===403||response.status===429){
    const retry=response.headers.get('retry-after'),reset=Number(response.headers.get('x-ratelimit-reset'))*1000
    cooldown=Math.max(now()+60000,Number.isFinite(Number(retry))&&retry!==null?now()+Number(retry)*1000:Date.parse(retry)||0,reset||0)
    throw Error('throttled')
   }
   if(!response.ok)throw Error('network')
   if(Number(response.headers.get('content-length'))>2*1024*1024)throw Error('invalid-response')
   const reader=response.body.getReader();let size=0,chunks=[]
   try{while(true){const {done,value}=await reader.read();if(done)break;size+=value.byteLength;if(size>2*1024*1024)throw Error('invalid-response');chunks.push(Buffer.from(value))}}finally{await reader.cancel().catch(()=>{})}
   let data;try{data=JSON.parse(Buffer.concat(chunks).toString('utf8'))}catch{throw Error('invalid-response')}
   if(!Array.isArray(data)||data.length>100)throw Error('invalid-response')
   releases.push(...data)
   if(data.length<100&&!response.headers.get('link')?.includes('rel="next"'))return releases
  }
  throw Error('invalid-response')
 }
 function check(){
  if(inflight)return inflight
  if(now()<cooldown){state={...state,status:'error',error:'throttled'};emit();return Promise.resolve(snapshot())}
  state={...state,status:'checking',lastAttemptAt:now(),error:null};emit()
  controller=new AbortController()
  let timeoutTimer
  const expired=new Promise((_,reject)=>{timeoutTimer=setTimer(()=>{controller.abort();reject(Error('timeout'))},timeout)})
  inflight=(async()=>{
   try{
    const releases=await Promise.race([retrieve(controller.signal),expired])
    // Cache only the selected release facts, never arbitrary remote response fields.
    const result=select(releases,installedVersion,platform,arch)
    const candidates=releases.filter(r=>version(r?.tag_name)&&!r.draft&&!r.prerelease).map(r=>({tag_name:r.tag_name,html_url:r.html_url,assets:Array.isArray(r.assets)?r.assets.filter(a=>a?.name===archive(platform,arch)).map(a=>({name:a.name,size:a.size,browser_download_url:a.browser_download_url})):[]}))
    lastSuccess={checkedAt:now(),releases:candidates}
    state={...state,...result,checkedAt:lastSuccess.checkedAt,error:null}
    try{
     save()
     if(result.available&&!notified.has(result.available.version)){
      notified.add(result.available.version);save()
      try{notify(result.available)}catch{}
     }
    }catch{state.error='storage'}
   }catch(error){state={...state,status:'error',error:['timeout','throttled','invalid-response'].includes(error.message)?error.message:'network'}}
   finally{clearTimer(timeoutTimer);controller=null;inflight=null;nextDue=now()+DAY;emit();schedule()}
   return snapshot()
  })()
  return inflight
 }
 return {snapshot,check,start(){stopped=false;return check()},resume(){if(now()>=Math.max(nextDue,cooldown))return check()},stop(){stopped=true;clearTimer(timer);controller?.abort()}}
}
module.exports={version,compare,archive,safeLink,select,createDesktopUpdates,DAY}
