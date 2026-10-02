// What the recurring poll does on each tick.
//
// The decision follows the connection itself, never what the window happens to
// show. Reading it from the DOM stranded the app: agentUnavailable() leaves
// #setup hidden while the agent-log pane is open, so a poll keyed on that panel
// kept calling runs() - the one branch that cannot recover once the local
// connection is gone - and never called connect() again. The log pane is
// precisely what the user opens when the agent is in trouble.

// pollAction says which of the two calls the tick makes, or neither.
// A start in progress is left alone too: connecting under it would drop the
// connection the start is setting up (#716).
export function pollAction({restarting,starting,agentConnected,shutdownVisible}){
 if(restarting||starting)return 'idle'
 if(agentConnected)return 'refresh'
 // The stop button is offered only while the app drives the agent's lifecycle;
 // reconnecting under it would race that operation.
 return shutdownVisible?'idle':'connect'
}

// Every start control is clickable whenever no agent runs and nothing is in progress (#716).
export const startEnabled=({agentConnected,restarting,pending,starting})=>!agentConnected&&!restarting&&!pending&&!starting
