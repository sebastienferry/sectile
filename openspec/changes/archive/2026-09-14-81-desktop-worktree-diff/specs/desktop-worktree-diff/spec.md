## ADDED Requirements

### Requirement: Inspect the selected local execution
TaskFlow Desktop SHALL provide a read-only Changes view for a selected execution whose assigned checkout is available. It SHALL identify the task, actual checkout directory, actual branch, default-branch reference, and common-ancestor commit. Inspection SHALL remain available after the execution stops while its local execution record and checkout remain available.

#### Scenario: Running or stopped execution
- **GIVEN** a selected running or stopped execution with an available assigned checkout
- **WHEN** the user opens Changes
- **THEN** Desktop shows that checkout's comparison and identifying context
- **AND** the execution continues in its current state.

#### Scenario: Assigned branch uses the main checkout
- **GIVEN** the selected execution legitimately uses the project's main checkout instead of a separate worktree
- **WHEN** the user opens Changes
- **THEN** the comparison uses the recorded execution directory.

#### Scenario: Unavailable or changed checkout
- **GIVEN** an execution without a prepared directory, a deleted checkout, an unknown execution, or a checkout now on another branch
- **WHEN** inspection is requested
- **THEN** Desktop explains why changes are unavailable
- **AND** it does not substitute another checkout or present a clean result.

### Requirement: Compare with the default-branch common ancestor
The viewer SHALL use the common ancestor of the assigned task branch and the resolved repository default branch as its baseline. It SHALL use available local references without fetching and SHALL display the resolved reference and ancestor commit. It SHALL report unavailable or ambiguous baseline history explicitly.

#### Scenario: Default branch advances independently
- **GIVEN** a task branch and default branch that diverged at a common ancestor
- **AND** the default branch contains subsequent changes absent from the task
- **WHEN** the comparison is shown
- **THEN** changes made only on the default branch are not shown as task deletions or reversions.

#### Scenario: Nonstandard default branch
- **GIVEN** locally recorded remote metadata identifies an available default branch named `trunk`
- **WHEN** the baseline is resolved
- **THEN** the comparison uses the common ancestor with that branch rather than assuming `main`.

#### Scenario: Missing or ambiguous history
- **GIVEN** the default branch cannot be resolved, a common ancestor is missing, or more than one best common ancestor exists
- **WHEN** the comparison is requested
- **THEN** Desktop shows an unavailable-baseline explanation
- **AND** it does not fall back to the last commit or a different comparison meaning.

### Requirement: Show the current net changes
The comparison SHALL include committed, staged, unstaged, and non-ignored untracked files as one net view of current worktree contents against the baseline. A path SHALL occur once in the result; counters SHALL describe its net textual changes rather than sum intermediate patches. Ignored untracked files SHALL be excluded, and tracked files SHALL remain eligible even if they match ignore rules.

#### Scenario: Overlapping committed and local changes
- **GIVEN** the same file has committed, staged, and unstaged edits
- **WHEN** the comparison is shown
- **THEN** the file appears once with a patch from its baseline contents to its current contents
- **AND** its counts and the aggregate counts agree with that patch.

#### Scenario: Local edit reverses an earlier change
- **GIVEN** a committed or staged edit has been reversed in the current file contents
- **WHEN** the comparison is shown
- **THEN** the canceled change contributes no net addition or deletion
- **AND** a file identical to the baseline is absent from the changed-files list.

#### Scenario: New files and ignore rules
- **GIVEN** a non-ignored untracked text file and an ignored untracked file
- **WHEN** the comparison is shown
- **THEN** the first file appears as an addition with its text
- **AND** the ignored file does not appear.

#### Scenario: Staged deletion followed by recreation
- **GIVEN** a baseline file was removed from staging and then recreated as a non-ignored untracked file at the same path
- **WHEN** the comparison is shown
- **THEN** that path is compared once against its baseline contents
- **AND** it is not represented as unrelated deletion and addition patches.

#### Scenario: No net changes
- **GIVEN** no included content or file metadata differs from the baseline
- **WHEN** Changes opens or refreshes
- **THEN** Desktop displays “No changes compared with baseline” with zero changed files and zero text counts
- **AND** it displays no unrelated last-commit changes.

### Requirement: Present files and readable patches accurately
The viewer SHALL provide a selectable changed-files list, change status, per-file and aggregate text counts, and a unified textual patch for eligible text files. It SHALL preserve filenames containing spaces, tabs, Unicode, or newlines without splitting them into separate files. Binary, symbolic-link, file-mode, submodule, and oversized changes SHALL be represented explicitly rather than silently omitted or presented as empty text files.

#### Scenario: Select a text change
- **GIVEN** multiple changed text files
- **WHEN** the user selects one
- **THEN** its path, status, context lines, additions, and deletions are readable
- **AND** diff text and filenames are displayed as inert text.

#### Scenario: Rename and metadata changes
- **GIVEN** a rename, deletion, executable-bit change, or symbolic-link change
- **WHEN** the result is displayed
- **THEN** the change is identifiable, with both paths when classified as a rename
- **AND** symbolic links are described without displaying the contents of their targets.

#### Scenario: Binary or submodule change
- **GIVEN** an included binary file or submodule change
- **WHEN** the user selects it
- **THEN** Desktop identifies the change kind without a misleading textual patch or numeric text count
- **AND** it does not recursively inspect a submodule's files.

#### Scenario: Response limits
- **GIVEN** a file or comparison exceeds the supported inspection limit
- **WHEN** inspection completes or reaches that limit
- **THEN** Desktop explains that content or the result is incomplete
- **AND** partial totals are labeled as partial and the result is not called clean.

### Requirement: Refresh without disrupting the console
The viewer SHALL load on opening and offer an explicit Refresh action. The user SHALL be able to return to the console without restarting, stopping, or detaching its execution. Requests and displayed results SHALL remain associated with the selected execution.

#### Scenario: Refresh after another edit
- **GIVEN** Changes is open and local files have changed since the displayed result
- **WHEN** the user selects Refresh
- **THEN** the comparison is recomputed with a visible loading state
- **AND** the selected file remains selected if it is still present.

#### Scenario: Selection changes during a request
- **GIVEN** an inspection request for execution A is pending
- **WHEN** the user selects execution B and the response for A arrives later
- **THEN** A's response does not replace B's context or result.

#### Scenario: Failed refresh or unavailable agent
- **GIVEN** a refresh fails, the local agent is unavailable, or its version does not support inspection
- **WHEN** Desktop receives the failure
- **THEN** it shows an actionable error and allows retry
- **AND** any retained prior result is clearly marked stale.

#### Scenario: Keyboard navigation
- **GIVEN** a user operating Desktop by keyboard
- **WHEN** they open Changes, choose a file, refresh, and return to Console
- **THEN** all controls have accessible names and visible focus
- **AND** terminal input is sent only when the console has focus.

### Requirement: Preserve local state and local access boundaries
Inspection SHALL require the existing authenticated local connection and a known execution. It SHALL not accept an arbitrary filesystem directory from the caller, upload source contents to the server or tracker, modify repository state, or invoke user-configured external diff commands.

#### Scenario: Read-only inspection
- **GIVEN** a checkout with staged, unstaged, and untracked changes
- **WHEN** the user opens and refreshes Changes
- **THEN** file contents, index, branches, refs, and execution state remain unchanged
- **AND** no fetch, checkout, staging, commit, or worktree creation occurs.

#### Scenario: Unauthorized inspection
- **GIVEN** a request without valid local credentials or from an unsupported browser-origin context
- **WHEN** it requests a diff
- **THEN** it receives no local repository contents.

#### Scenario: Concurrent modification
- **GIVEN** the branch or comparison inputs change while an inspection is being assembled
- **WHEN** the service detects the change
- **THEN** it retries within its bounded request time or reports that the checkout changed and requests a refresh
- **AND** it does not label a detected inconsistent result as current.
