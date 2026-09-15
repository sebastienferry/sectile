// Diagnostics are plain text snapshots, not a terminal screen or command stream.
export function logText(text){
 let output='',state='text',osc=false
 for(let i=0;i<text.length;i++){
  const char=text[i],code=text.charCodeAt(i)
  if(state==='string'){
   if(code===0x9c||(osc&&code===7))state='text'
   else if(code===0x1b&&text[i+1]==='\\'){state='text';i++}
   continue
  }
  if(state==='csi'){
   if(code>=0x40&&code<=0x7e)state='text'
   else if(code===0x1b)state='escape'
   continue
  }
  if(state==='escape'){
   if(char==='['){state='csi';continue}
   if(']PX^_'.includes(char)){state='string';osc=char===']';continue}
   if(code>=0x20&&code<=0x2f){state='intermediate';continue}
   state='text';continue
  }
  if(state==='intermediate'){
   if(code>=0x30&&code<=0x7e)state='text'
   continue
  }
  if(code===0x1b){state='escape';continue}
  if(code===0x9b){state='csi';continue}
  if([0x90,0x98,0x9d,0x9e,0x9f].includes(code)){state='string';osc=code===0x9d;continue}
  if(char==='\r'){output+='\n';if(text[i+1]==='\n')i++;continue}
  if(char==='\n'||char==='\t'||(code>=0x20&&(code<0x7f||code>0x9f)))output+=char
 }
 return output
}
