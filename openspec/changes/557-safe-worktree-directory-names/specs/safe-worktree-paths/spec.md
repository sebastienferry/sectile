## ADDED Requirements

### Requirement: Safe deterministic generated names
Sectile SHALL generate bounded filesystem-safe directory basenames independently of display keys for new primary task, secondary task, and macro specification worktrees. Generated names SHALL contain only ASCII letters, digits, and hyphens, avoid platform-reserved names and dot components, and remain inside the repository's worktree container. A canonical numeric GitHub key `#289` SHALL generate `issue-289` when the destination is free. Tracker identity, displayed key, and assigned branch SHALL remain unchanged.

#### Scenario: Numeric GitHub task in primary and secondary repositories
- **Given** task `#289`, its assigned branch, and repositories with safe ancestor paths and free destinations
- **When** Sectile prepares the task in the primary repository and an attached Git repository
- **Then** each new checkout is under `.tasks/worktrees/issue-289`
- **And** the tracker key stays `#289` and the assigned branch is preserved in both repositories

#### Scenario: Macro preparation
- **Given** a macro with a valid key and no checkout on its selected specification branch
- **When** Sectile prepares its specification worktree
- **Then** the generated basename follows the same safe naming convention as task worktrees
- **And** existing macro branch selection and base-branch behavior remain intact

#### Scenario: Normalization and platform collisions
- **Given** distinct valid keys differing by punctuation, case, Unicode normalization, or matching the reserved `issue-<number>` namespace
- **When** their directory names are generated
- **Then** distinct keys cannot silently alias the same destination on a case-insensitive filesystem
- **And** repeated naming of the same key produces the same result independently of preparation order

#### Scenario: Unsafe input rejection
- **Given** an empty key, a dot component, or a key containing a path separator
- **When** Sectile prepares or predicts a task checkout
- **Then** it rejects the input before filesystem mutation or legacy path probing

### Requirement: Existing branch checkouts take precedence
Sectile SHALL resolve the checkout on the assigned branch through local Git metadata before deriving a directory from a key. Preparation, discovery, workspace information, launches, editor and diff operations, and explicit removal SHALL agree on the actual checkout. No creation or lookup SHALL relocate, reset, or delete an existing checkout.

#### Scenario: Repeated preparation
- **Given** a newly prepared safe worktree on the assigned branch
- **When** the same task is prepared again
- **Then** the same checkout is reused without creating another worktree

#### Scenario: Legacy arbitrary and shared batch checkout
- **Given** the assigned branch is checked out at a legacy `#289` path, an arbitrary directory, or a shared batch path with uncommitted changes
- **When** Sectile prepares, discovers, inspects, or launches the task
- **Then** every operation uses that actual checkout and preserves its changes and location
- **And** a conflicting predicted directory is not used

#### Scenario: Assigned branch at the main checkout
- **Given** the main checkout carries the assigned branch
- **When** Sectile prepares the task or resolves its workspace
- **Then** it reuses the main checkout
- **And** explicit task-worktree removal preserves that checkout

#### Scenario: Unrelated or detached legacy candidate
- **Given** no checkout on the assigned branch and a legacy directory belonging to another branch or a detached HEAD
- **When** Sectile resolves or prepares the task
- **Then** that directory is not accepted as the task checkout merely because it exists
- **And** its contents and Git state remain unchanged

### Requirement: Occupied destinations remain protected
Task creation SHALL select a safe non-overwriting sibling when its generated destination is occupied by unrelated content. Macro creation SHALL retain its existing occupied-path safeguards, including refusal to delete nonempty content. Files, directories, and symlinks SHALL never be overwritten to claim a destination. Exhausted candidates or creation races SHALL fail safely without destructive recovery.

#### Scenario: Task destination and sibling occupied
- **Given** the generated task destination and initial sibling candidate are occupied by unrelated entries
- **When** Sectile prepares the task
- **Then** it either creates at another safe free destination or reports a safe failure
- **And** every occupied entry is preserved
- **And** a successful subsequent preparation reuses the branch checkout

#### Scenario: Macro destination occupied
- **Given** a nonempty directory or file occupies the generated macro destination
- **When** Sectile prepares the macro
- **Then** it refuses with the existing diagnostic behavior and preserves the entry

### Requirement: Explicit cleanup uses the actual checkout
Explicit worktree cleanup SHALL target the locally resolved branch checkout in each repository and retain existing non-forced Git removal safeguards. Lookup and creation SHALL not initiate cleanup of legacy worktrees.

#### Scenario: Dirty legacy worktree removal
- **Given** a dirty legacy checkout on the assigned branch
- **When** explicit removal is requested
- **Then** Sectile targets that checkout and Git's non-forced removal protection preserves uncommitted work

### Requirement: Vite source modules load in newly generated paths
A Vite application SHALL successfully serve and load its source modules from a newly generated safe worktree under a safe repository root, with dependencies installed in that worktree.

#### Scenario: Browser loads the source module
- **Given** a task with a numeric GitHub key, a newly generated safe worktree, and Vite dependencies installed there
- **When** Vite starts from that checkout and a browser opens the application
- **Then** the source module request succeeds and executes, rendering the expected application content without source-loading errors
- **And** the verification does not relocate the application or reuse dependencies from an unsafe checkout

#### Scenario: Legacy Vite paths stay unchanged
- **Given** an existing worktree whose path contains `#`
- **When** Sectile resolves it
- **Then** it remains available at its original path without promising to repair Vite there
