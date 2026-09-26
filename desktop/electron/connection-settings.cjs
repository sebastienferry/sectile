// The desktop shares ~/.config/sectile/settings.json with the local agent. It
// writes the connection keys only: the execution sections (workstation
// defaults, project sections, model lists) belong to the agent, which is their
// only writer since #305, so a renderer asking to save one of them is ignored.
const CONNECTION_KEYS=['server','deviceId','secret','apiKey','binary','repo']

// connectionUpdates keeps the keys the desktop may write and drops the rest.
function connectionUpdates(updates){
 const kept={}
 if(!updates||typeof updates!=='object')return kept
 for(const key of CONNECTION_KEYS)if(Object.prototype.hasOwnProperty.call(updates,key))kept[key]=updates[key]
 return kept
}

// connectionView is what the renderer reads back: the connection facts, never
// the execution sections the agent owns, and never the stored key itself.
function connectionView(saved,token){
 const view={}
 for(const key of ['server','deviceId','binary','repo'])if(saved&&saved[key]!==undefined)view[key]=saved[key]
 view.token=token
 return view
}

module.exports={CONNECTION_KEYS,connectionUpdates,connectionView}
