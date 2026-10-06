// A stand-in for the agent binary in the UI tests: the desktop spawns it as it
// would the real agent, it answers the few desktop routes a connected workspace
// reads, and it records the key it was started with. It exits on its own, since
// the desktop starts it detached and closing the app leaves it running.
const http=require('node:http'),fs=require('node:fs')
const argument=name=>{const index=process.argv.indexOf(name);return index>=0?process.argv[index+1]:''}
const info=argument('--desktop-info'),server=argument('--url')
const desktopToken=process.env.SECTILE_DESKTOP_TOKEN
const routes={
 '/desktop/runs':[],
 '/desktop/projects':[],
 '/desktop/status':{connected:true,server,capabilities:[],disconnectedProjects:[]}
}
const listener=http.createServer((req,res)=>{
 if(req.headers.authorization!=='Bearer '+desktopToken){res.writeHead(401).end();return}
 const route=routes[new URL(req.url,'http://127.0.0.1').pathname]
 if(!route){res.writeHead(404,{'Content-Type':'application/json'}).end('{}');return}
 res.writeHead(200,{'Content-Type':'application/json'}).end(JSON.stringify(route))
})
listener.listen(0,'127.0.0.1',()=>{
 if(process.env.SECTILE_FAKE_AGENT_RECORD)fs.writeFileSync(process.env.SECTILE_FAKE_AGENT_RECORD,JSON.stringify({pid:process.pid,token:process.env.TOKEN||''}))
 fs.writeFileSync(info,JSON.stringify({url:'http://127.0.0.1:'+listener.address().port,token:desktopToken}))
})
setTimeout(()=>process.exit(0),60000)
