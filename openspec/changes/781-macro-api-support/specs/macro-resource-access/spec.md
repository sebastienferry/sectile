# Macro Resource Access

Status: scope confirmed by the owner on 2026-10-07; no open product questions.

## ADDED Requirements

### Requirement: Project-scoped macro discovery

An MCP client SHALL be able to list the macros of an explicitly identified project, including closed macros, using the same resource information available to existing macro readers.

#### Scenario: List project macros
- **GIVEN** a project containing open and closed macros
- **WHEN** a client lists that project's macros
- **THEN** the response contains only that project's macros, including their keys, metadata, ordered todos, computed write flags, and tracker copy status

#### Scenario: Empty or unknown project
- **GIVEN** a project identifier
- **WHEN** a client lists its macros
- **THEN** an existing project without macros returns an empty collection, while a missing or unknown identifier returns an error

### Requirement: Single-macro access

HTTP and MCP clients SHALL be able to read one macro by project and macro key. The existing HTTP collection response and MCP read response SHALL remain compatible.

#### Scenario: Read the requested macro
- **GIVEN** two projects containing the same macro key
- **WHEN** a client reads that key within one project
- **THEN** it receives that project's macro, including its todos and copy status, rather than a collection or the other project's macro

#### Scenario: Unknown resource
- **GIVEN** an unknown project or macro key
- **WHEN** a client requests that resource
- **THEN** the response reports that it was not found and creates no macro

### Requirement: Identified macro creation

An identified MCP caller SHALL be able to create a macro with a title and optional horizon using existing project and tracker rules. Caller identity SHALL come from the authenticated connection.

#### Scenario: Create a macro
- **GIVEN** an identified caller and an existing project
- **WHEN** the caller submits a nonblank title and an optional supported horizon
- **THEN** the response identifies the created macro and reflects the existing default horizon when none was supplied

#### Scenario: Refuse an invalid or anonymous creation
- **GIVEN** an anonymous caller, an unknown project, a blank title, or an invalid horizon
- **WHEN** creation is requested
- **THEN** the request fails before local or remote mutation

### Requirement: Partial metadata edits

An identified MCP caller SHALL be able to edit title, description, framing comment, horizon, closed state, priority, quarter, and readiness on an existing macro. Omitted fields SHALL retain their values, while explicit empty values and false SHALL retain the field's existing clearing and reopening semantics. Validation failures SHALL occur before any mutation.

#### Scenario: Edit selected fields
- **GIVEN** an existing macro with todos linked to stories
- **WHEN** its description is cleared and its closed state is explicitly set to false
- **THEN** the description becomes empty and the macro is reopened, while omitted metadata, ordered todos, and story links remain unchanged

#### Scenario: Refuse invalid metadata or an unknown macro
- **GIVEN** an invalid metadata value or a macro that does not exist in the named project
- **WHEN** an edit is requested
- **THEN** no submitted metadata is saved and no tracker operation is started

#### Scenario: Preserve dedicated todo editing
- **GIVEN** a client needs to change a macro's ordered todos
- **WHEN** it inspects the metadata editing interface
- **THEN** todo replacement is not offered there and remains subject to the existing dedicated operation's linked-story protection

### Requirement: Preserve tracker outcomes and identity

Writes SHALL preserve current local-versus-tracker rules, computed write restrictions, and acting-user identity. Responses SHALL distinguish local persistence from failed or queued tracker work whenever that outcome is reported by the underlying operation. A queued copy SHALL NOT be presented as completed.

#### Scenario: Local save with tracker refusal
- **GIVEN** an operation saves a macro locally and reports a tracker refusal
- **WHEN** the client receives the result
- **THEN** it can identify the saved macro and the tracker failure without being told that the entire operation succeeded or that nothing was saved

#### Scenario: Restricted roadmap macro
- **GIVEN** a macro whose tracker writes are restricted by the current roadmap rules
- **WHEN** an allowed local metadata edit is requested
- **THEN** local changes and any permitted queued writes follow those rules using the caller's identity, without enabling additional tracker writes

### Requirement: Document the resource contract

The delivered change SHALL document the added tools and HTTP read, input fields, response shapes, and partial-success semantics, and include one user-facing Added entry under the changelog's Unreleased section.

#### Scenario: Integration documentation
- **WHEN** an engineer integrates a client with the added interfaces
- **THEN** the documentation explains how to identify a project and macro, distinguish an empty collection from a missing resource, and recognize local persistence with incomplete tracker work
