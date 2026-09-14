# Project New task choice

## Why
The desktop project's New task button only offers searching existing tickets. Quick add is available through the command palette, so creating a ticket from its project is unnecessarily hidden.

## What Changes
- Offer Run an existing ticket and Quick add task when clicking a project's New task button.
- Preserve the clicked project across both paths.
- Reuse quick add's explicit Launch task action after successful creation.

## Scope
Desktop entry UI, regression tests, and usage documentation. No backend, tracker, web UI, implicit execution, or workflow policy changes.

## Clarification
The desktop project row matches the ticket's existing-ticket-only behavior. Creation and launch remain separate explicit actions, as established in the current quick add flow. No unresolved requirements or external dependencies.
