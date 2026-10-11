# Workstation project availability

## Why
The server sees an agent connection but cannot tell which projects have accessible local checkouts. Its user/project slot also replaces one workstation with another, preventing a live inventory across multiple devices.

## What Changes
- Announce a complete, versioned project inventory on connection and refresh it when local project configuration or checkout accessibility changes.
- Retain simultaneous authenticated workstations and maintain connection-scoped availability per user, device and project.
- Expose a user-isolated, project-filtered query of connected workstations with verified inventory.
- Preserve existing launch routing as a compatibility view; inventory does not select a launch target.

## Scope
Available means an accessible Git checkout under existing local mappings/fallback and not locally disconnected. Paths remain local. An empty inventory withdraws all availability. Reconnect and disconnect cannot overwrite another live session.

## Non-goals
Automatic workstation selection, a picker, cloning/provisioning, dependency or credential readiness checks, hardware/provider/CLI inventories, durable offline history.

## Impact
Go agent, shared relay protocol, connection dispatcher/status handler and tests; server-agent contract documentation. A live-session directory migration is needed for existing shared-server deployments. No production implementation occurs during specification.

## Decisions
Owner confirmed all three scope decisions on 2026-10-11 (“oui aux trois”); see docs/clarifications/814.md. No open product questions.
