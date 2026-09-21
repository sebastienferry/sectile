# #272 — Project list and bookmarks

## Context

Sectile projects are shared across all users on a deployment. In growing teams or multi-project environments, the project switcher in the sidebar currently displays every single project on the server. This causes visual clutter and makes switching between relevant workspaces cumbersome.

This specification defines user-specific project bookmarks. Only projects bookmarked by a user appear in their primary project switcher dropdown and in their "All projects" view. Users can search across all shared projects directly within the switcher to discover and bookmark projects created by teammates.

This specification describes behavior and acceptance criteria only. Technical architecture, data schemas, and API design are detailed in `plan.md`; the ordered implementation checklist is in `tasks.md`.

## Decisions Being Specified

1. **User-Scoped Bookmarks**: Projects remain globally shared and accessible to all users (no private projects or ACL changes), but each user curates their personal list of bookmarked projects.
2. **Initial Bookmarking (Zero-State Prevention)**: A newly created user or an unmigrated account automatically has the deployment default project (`is_default = true`) bookmarked, ensuring the switcher is never empty on first load.
3. **Creator Bookmarking**: Creating a new project automatically bookmarks that project for its creator.
4. **Sidebar Project Switcher Filtering**: The project switcher dropdown displays bookmarked projects by default. A search field inside the dropdown allows searching across all shared projects.
5. **Interactive Bookmark Toggle**: Each project row in the dropdown (and search results) provides a star/bookmark toggle icon to add or remove the project from the user's bookmarks.
6. **"All Projects" View Scoping**: When "Tous les projets" ("All projects") is selected, the board, list, and task counters strictly aggregate tasks from the user's bookmarked projects.
7. **Modal Ordering**: In task creation, detail, and clone modals (QuickAdd, TaskDetail, Clone), projects are presented with bookmarked projects first, followed by all remaining shared projects.

---

## User Stories

### US1 — Automatic default bookmarking for new and unmigrated users (P1)

As a new or existing user opening Sectile,
I want the default project to be bookmarked automatically,
So that my project switcher is never empty and I can start working immediately.

- **Given** a user account with no recorded project bookmarks
- **When** the user loads the application or requests the project list
- **Then** the deployment default project (`is_default = true`) is automatically bookmarked for that user
- **And** the project switcher displays the default project as bookmarked.

---

### US2 — Project switcher displays bookmarked projects by default (P1)

As a user with several projects on the server,
I want the sidebar switcher dropdown to show only the projects I have bookmarked,
So that I can quickly switch between my active projects without navigating through unrelated projects.

- **Given** three projects exist (`Project Alpha`, `Project Beta`, `Project Gamma`)
- **And** I have bookmarked only `Project Alpha` and `Project Beta`
- **When** I click the project switcher in the sidebar without typing in the search box
- **Then** the dropdown displays `Project Alpha`, `Project Beta`, and the "Tous les projets" ("All projects") option
- **And** `Project Gamma` is not listed in the default dropdown view.

---

### US3 — Search and discover shared projects in the switcher dropdown (P1)

As a user looking for a colleague's project,
I want to search across all projects directly within the switcher dropdown,
So that I can discover and bookmark any existing project in the organization.

- **Given** `Project Gamma` exists on the server but is not in my bookmarks
- **When** I open the project switcher dropdown and type `Gamma` into the search box
- **Then** `Project Gamma` appears in the filtered dropdown results
- **And** an unbookmarked indicator (e.g. outline star icon) is visible on its row.

---

### US4 — Toggling project bookmarks (P1)

As a user,
I want to bookmark or unbookmark any project with a single click,
So that I can keep my active project list up to date as my assignments change.

- **Given** `Project Gamma` is displayed in the search results and is not bookmarked
- **When** I click its bookmark toggle icon
- **Then** the icon changes to active (filled star) immediately
- **And** the project is persisted as bookmarked on the server for my account
- **And** when I clear the search box, `Project Gamma` remains listed among my bookmarked projects.
- **Given** `Project Alpha` is currently bookmarked
- **When** I click its bookmark toggle icon to unbookmark it
- **Then** the icon changes to inactive
- **And** the project is removed from my bookmarks without affecting any other user's bookmarks or project settings.

---

### US5 — Automatic bookmarking on project creation (P1)

As a user creating a new project,
I want it to be bookmarked for me automatically upon creation,
So that I don't have to manually search for and bookmark the project I just created.

- **Given** I am a signed-in user
- **When** I submit the project creation form for `Project Delta`
- **Then** `Project Delta` is created and automatically added to my project bookmarks
- **And** `Project Delta` immediately appears in my project switcher list as bookmarked.

---

### US6 — "All projects" view filtered by bookmarked projects (P1)

As a user viewing the aggregated "All projects" board or list,
I want to see tasks belonging exclusively to my bookmarked projects,
So that other teams' backlogs do not clutter my unified overview.

- **Given** I have bookmarked `Project Alpha` (which has 5 tasks) and `Project Beta` (which has 3 tasks)
- **And** unbookmarked `Project Gamma` has 20 tasks
- **When** I select "Tous les projets" ("All projects") in the project switcher
- **Then** the board, list views, and sidebar status facet counts reflect only the 8 tasks from `Project Alpha` and `Project Beta`
- **And** tasks from `Project Gamma` are excluded from the aggregated view and counts.

---

### US7 — Bookmarked projects prioritised in task modals (P2)

As a user creating, moving, or cloning a task,
I want to see my bookmarked projects at the top of the project selector,
So that I can quickly select my frequent projects while retaining access to all shared projects.

- **Given** I open the QuickAdd modal, Task Detail modal (project reassignment), or Clone Task modal
- **When** I open the project selection dropdown
- **Then** my bookmarked projects appear first (grouped under "Favoris / Bookmarks")
- **And** all other shared projects appear below them (grouped under "Autres projets / Other projects")
- **And** I can select any project from either group.

---

### US8 — Persistence across devices and sessions (P1)

As a user working across multiple workstations,
I want my project bookmarks stored on the server,
So that my customized switcher is identical whether I work in a browser, Electron desktop, or across machines.

- **Given** I have bookmarked specific projects in one session
- **When** I sign in from another browser or machine
- **Then** my bookmarked projects and switcher state are restored exactly as saved.

---

### US9 — Project deletion cascades to bookmarks (P2)

As an administrator deleting an obsolete project,
I want bookmark entries associated with the deleted project to be removed cleanly,
So that no orphaned bookmarks linger in any user's profile.

- **Given** a project `Project Obsolete` is bookmarked by several users
- **When** an administrator deletes `Project Obsolete`
- **Then** all entries for `Project Obsolete` in the bookmarks store are deleted
- **And** users who had it selected are redirected to the default project or "Tous les projets".
