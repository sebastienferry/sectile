// What the recurring poll does on each tick.
//
// The decision follows the connection itself, never what the window happens to
// show. Reading it from the DOM stranded the app: agentUnavailable() leaves
// #setup hidden while the agent-log pane is open, so a poll keyed on that panel
// kept calling runs() - the one branch that cannot recover once the local
// connection is gone - and never called connect() again. The log pane is
// precisely what the user opens when the agent is in trouble.

// pollAction says which of the two calls the tick makes, or neither.
export function pollAction({restarting,agentConnected,shutdownVisible}){
 if(restarting)return 'idle'
 if(agentConnected)return 'refresh'
 // The stop button is offered only while the app drives the agent's lifecycle;
 // reconnecting under it would race that operation.
 return shutdownVisible?'idle':'connect'
}
