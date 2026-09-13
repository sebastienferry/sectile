const fs=require('node:fs')
const MAX_LOG_BYTES=256*1024

async function readAgentLog(file){
 let handle
 try{
  if(!(await fs.promises.lstat(file)).isFile())throw Error('Agent log is not a regular file.')
  handle=await fs.promises.open(file,fs.constants.O_RDONLY|fs.constants.O_NOFOLLOW|fs.constants.O_NONBLOCK)
 }catch(error){
  if(error.code==='ENOENT')return {path:file,text:'',missing:true,truncated:false}
  throw error
 }
 try{
  const stat=await handle.stat()
  if(!stat.isFile())throw Error('Agent log is not a regular file.')
  const start=Math.max(0,stat.size-MAX_LOG_BYTES)
  const buffer=Buffer.alloc(Math.min(stat.size,MAX_LOG_BYTES))
  let length=0
  while(length<buffer.length){
   const {bytesRead}=await handle.read(buffer,length,buffer.length-length,start+length)
   if(!bytesRead)break
   length+=bytesRead
  }
  let offset=0
  if(start>0)while(offset<length&&(buffer[offset]&0xc0)===0x80)offset++
  return {path:file,text:buffer.subarray(offset,length).toString('utf8'),missing:false,truncated:start>0}
 }finally{await handle.close()}
}

module.exports={readAgentLog,MAX_LOG_BYTES}
