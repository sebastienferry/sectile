// Signing in through the browser: the server's web sign-in (Auth0 or local) hands a single-use pairing code back to a
// one-shot listener on 127.0.0.1. The key itself never travels in a URL (ADR 0049).
const http=require('node:http'),crypto=require('node:crypto')
const SIGN_IN_TIMEOUT=5*60*1000
const SIGNED_IN_PAGE='<!doctype html><html><head><meta charset="utf-8"><title>Sectile</title></head><body><p>You can close this tab and return to Sectile Desktop.</p></body></html>'
// browserSignIn opens {server}/auth/workstation in the browser through `open` and resolves with the pairing code the
// server redirects to this machine's loopback listener.
function browserSignIn(server,{open,timeout=SIGN_IN_TIMEOUT,signal}={}){
 const url=new URL(server)
 if(!['http:','https:'].includes(url.protocol)||url.username||url.password)throw Error('Use an HTTP or HTTPS server URL')
 if(signal?.aborted)return Promise.reject(Error('Sign-in cancelled'))
 const state=crypto.randomBytes(32).toString('base64url'),expected=Buffer.from(state)
 return new Promise((resolve,reject)=>{
  let timer,done=false
  const listener=http.createServer((req,res)=>{
   const callback=new URL(req.url,'http://127.0.0.1')
   if(req.method!=='GET'||callback.pathname!=='/callback'){res.writeHead(404).end();return}
   const given=Buffer.from(callback.searchParams.get('state')||''),code=callback.searchParams.get('code')||''
   // A stray or forged hit is refused without ending the wait: only the browser carrying this state finishes it.
   if(done||!code||given.length!==expected.length||!crypto.timingSafeEqual(given,expected)){res.writeHead(400,{'Content-Type':'text/plain; charset=utf-8'}).end('Sign-in state mismatch');return}
   res.writeHead(200,{'Content-Type':'text/html; charset=utf-8','Cache-Control':'no-store',Connection:'close'})
   res.end(SIGNED_IN_PAGE,()=>finish(null,code))
  })
  const onAbort=()=>finish(Error('Sign-in cancelled'))
  function finish(error,code){
   if(done)return
   done=true;clearTimeout(timer);signal?.removeEventListener('abort',onAbort)
   listener.close();listener.closeAllConnections?.()
   error?reject(error):resolve(code)
  }
  signal?.addEventListener('abort',onAbort,{once:true})
  timer=setTimeout(()=>finish(Error('Sign-in timed out. Try again, or use a pairing code.')),timeout)
  listener.on('error',error=>finish(error))
  listener.listen(0,'127.0.0.1',()=>{
   if(done)return
   const target=new URL('/auth/workstation?'+new URLSearchParams({port:String(listener.address().port),state}),server)
   // `open` may throw or reject: either way the listener is closed and the reason is said.
   Promise.resolve().then(()=>open(target.href)).catch(error=>finish(Error('Could not open the browser: '+(error?.message||error))))
  })
 })
}
module.exports={browserSignIn,SIGN_IN_TIMEOUT}
