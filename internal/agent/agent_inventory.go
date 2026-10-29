package agent

import (
	"context"
	"encoding/json"
	"log"
	"slices"
	"sync"
	"time"

	"tasks/internal/agentconfig"
	"tasks/internal/agentprotocol"

	"github.com/gorilla/websocket"
)

type inventoryNotifier struct {
	mu     sync.Mutex
	events chan struct{}
}

func (n *inventoryNotifier) channel() chan struct{} {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.events == nil {
		n.events = make(chan struct{}, 1)
	}
	return n.events
}
func (n *inventoryNotifier) notify() {
	select {
	case n.channel() <- struct{}{}:
	default:
	}
}

func (d *agentDaemon) projectInventory(ctx context.Context) agentprotocol.ProjectInventory {
	p := agentprotocol.ProjectInventory{SchemaVersion: agentprotocol.InventoryVersion, ProjectIDs: []string{}}
	if _, err := d.readSettings(); err != nil {
		log.Printf("[Agent] Lecture de l'inventaire local impossible : %v", err)
		return p
	}
	projects, err := d.discoverProjects(ctx)
	if err != nil {
		log.Printf("[Agent] Découverte des projets impossible : %v", err)
		return p
	}
	scope := d.link.projectID
	for _, project := range projects.Projects {
		if scope != "" && scope != "all" && scope != "default" && scope != project.ID {
			continue
		}
		if _, _, err := d.localProjectRoot(ctx, agentconfig.Config{ProjectID: project.ID, GitRemoteURL: project.GitRemoteURL}); err == nil {
			p.ProjectIDs = append(p.ProjectIDs, project.ID)
		}
		if ctx.Err() != nil {
			p.ProjectIDs = []string{}
			return p
		}
	}
	if _, err := d.readSettings(); err != nil {
		p.ProjectIDs = []string{}
		return p
	}
	slices.Sort(p.ProjectIDs)
	p.ProjectIDs = slices.Compact(p.ProjectIDs)
	if len(p.ProjectIDs) > agentprotocol.InventoryMaxProjects {
		p.ProjectIDs = []string{}
	}
	return p
}

func (d *agentDaemon) sendProjectInventory(ctx context.Context, conn *websocket.Conn) {
	computation, cancel := context.WithTimeout(ctx, 10*time.Second)
	p := d.projectInventory(computation)
	cancel()
	if ctx.Err() != nil {
		return
	}
	raw, err := json.Marshal(p)
	if err != nil {
		return
	}
	envelope, _ := json.Marshal(agentprotocol.Message{Type: agentprotocol.ProjectInventoryType, Payload: raw})
	if len(envelope) > agentprotocol.InventoryMaxBytes {
		raw = []byte(`{"schemaVersion":1,"projectIds":[]}`)
	}
	if err := d.link.write(conn, agentprotocol.Message{Type: agentprotocol.ProjectInventoryType, Payload: raw}); err != nil {
		_ = conn.Close()
	}
}

func (d *agentDaemon) inventoryLoop(ctx context.Context, conn *websocket.Conn) {
	d.inventoryLoopEvery(ctx, conn, 30*time.Second)
}

func (d *agentDaemon) inventoryLoopEvery(ctx context.Context, conn *websocket.Conn, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.sendProjectInventory(ctx, conn)
		case <-d.inventory.channel():
			d.sendProjectInventory(ctx, conn)
		}
	}
}
