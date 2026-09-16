# Design

## Single ceiling constant
The value 3 is currently duplicated across the models normalizer, the server job
limiter, the project read path, the agent-side `ExecutionLimit`, the desktop
mapping endpoint, and two UI pickers. Each site is changed to read one exported
constant, `models.MaxParallelism`, so a future adjustment is a one-line change.
`internal/agentconfig` is a leaf package used by the agent protocol; if importing
`internal/models` would create a cycle, it declares its own documented constant
with the same value rather than importing.

The UI pickers render the range from 1 to the ceiling instead of a literal list.

Stored values above the new ceiling remain impossible; values previously clamped
to 3 are untouched, so the change is backward compatible with existing rows and
with `~/.taskflow/agent.json` overrides.

## Consoles are not workers
Admission happens in `awaitRunSlot`, which counts other live runs of the same
project into `active` and compares with `run.limit`. A run's kind is already
recorded on `desktopRun.Kind` before the runs mutex is released, so it is visible
to the admission loop.

Two independent rules:
- A console never increments `active` for another run.
- A console is not compared against `run.limit` at all.

The checkout guard stays, but it is stated precisely. A console works in the mapped
checkout; a task execution either works there too (`isolated == false`) or in its own
worktree. `sharesCheckout` therefore makes a console collide only with non-isolated
executions, and never with worktree-isolated ones. Two consoles do not collide: each
is opened deliberately by the user, and the free-console capability already promises
they run side by side. Non-console pairs keep their existing rule unchanged.

Without this precision the exemption would be inert: an execution enqueued on the
project root with `isolated == true` blocked every console, which is what the report
describes.

### Rejected alternative
Giving consoles a dedicated limit of their own. It adds a second setting to
explain for no observed need: consoles are opened one at a time by a human, and
the shared-checkout guard already prevents the harmful case.
