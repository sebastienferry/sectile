package agent

import "time"

// The server returns the accepted run ID independently of its websocket
// dispatch. Correlate by that ID, never by a ticket's next arbitrary dispatch.
// queue.mu protects both this map and the run it may already describe.
type desktopLaunchConnection struct {
	projectID, taskID, connectionID string
	at                              time.Time
}

func (d *agentDaemon) bindDesktopLaunchConnection(id, projectID, taskID, connectionID string) {
	d.queue.mu.Lock()
	for key, binding := range d.desktopLaunchConnections {
		if time.Since(binding.at) >= conversationDiscussionTTL {
			delete(d.desktopLaunchConnections, key)
		}
	}
	if d.desktopLaunchConnections == nil {
		d.desktopLaunchConnections = make(map[string]desktopLaunchConnection)
	}
	d.desktopLaunchConnections[id] = desktopLaunchConnection{projectID, taskID, connectionID, time.Now()}
	run := d.queue.runs[id]
	bound := false
	if run != nil && run.desktop.Conversation && run.desktop.ConnectionID == "" {
		if value := d.takeDesktopLaunchConnectionLocked(run.desktop); value != "" {
			run.desktop.ConnectionID = value
			bound = true
		}
	}
	d.queue.mu.Unlock()
	if bound {
		d.persistRun(id, run)
	}
}

func (d *agentDaemon) takeDesktopLaunchConnectionLocked(entry desktopRun) string {
	binding, ok := d.desktopLaunchConnections[entry.ID]
	if !ok {
		return ""
	}
	delete(d.desktopLaunchConnections, entry.ID)
	if time.Since(binding.at) >= conversationDiscussionTTL || binding.projectID != entry.ProjectID || binding.taskID != entry.TaskID {
		return ""
	}
	return binding.connectionID
}
