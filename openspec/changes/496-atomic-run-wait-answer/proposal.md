# Atomic answers to run waits

## Problem

On PostgreSQL with multiple server instances, an answer can read one wait and then clear a newer question written by another instance. The new question disappears from the board and desktop before its owner answers it.

## Value

An answer clears only the question whose mark was shown to the agent, so a newer question remains visible.

## Scope

- Make the answer's database clear conditional on the answered wait mark in a single statement.
- Cover the cross-instance race on PostgreSQL and preserve SQLite behavior.

## Out of scope

Wait declaration, MCP resume behavior, UI rendering, protocol changes, and schema changes.
