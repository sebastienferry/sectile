# #358 — Handle creator of ticket from tracker

Ticket: https://github.com/sebastienferry/sectile/issues/358  
Task Key: #358 (`gh-ef5a2777-920f-4744-a7e8-a58a4c257a23-358`)  
Type: Feature — "Handle creator of ticker from tracker" (ticket)  
Branch: `feat/358`  
Clarification: [`docs/clarifications/358.md`](../../docs/clarifications/358.md)  

## Context

Sectile synchronizes issues from external trackers (GitHub, Jira) and enables creating tasks locally. Today, task attribution captures who is assigned to a ticket (`assignee` and `assigneeAvatar`), but completely discards who authored or created the ticket (`creator` and `creatorAvatar`).

In GitHub, issues provide the original author via the `user` object (`login` and `avatar_url`).
In Jira Cloud, issues provide the creator in `fields.creator` (with fallback to `fields.reporter`), including their display name and avatar URL.

Without creator tracking:
- Users inspecting a ticket cannot see who reported or created it on GitHub/Jira without navigating out to the remote tracker.
- Local tasks lack author attribution.

This feature adds first-class support for `creator` and `creatorAvatar` across data models, database storage, tracker synchronization adapters, API responses, and the user interface.

Out of scope:
- Mutating remote ticket creators (authorship on external trackers is immutable).
- Filtering/faceting the board or list views by creator.
- Modifying project ownership semantics (`projects.owner_user_id`).

---

## Decisions being specified

1. **Authorship is Immutable & Read-Only**:
   The `creator` and `creatorAvatar` represent original ticket authorship and cannot be edited by users in the UI.
2. **GitHub Mapping**:
   GitHub issues map `issue.user.login` to `creator` and `issue.user.avatar_url` to `creatorAvatar`.
3. **Jira Mapping**:
   Jira issues map `fields.creator.displayName` (fallback `fields.creator.accountId`) to `creator` and `avatarUrls["48x48"]` to `creatorAvatar`. If `creator` is empty, fall back to `fields.reporter`.
4. **Local Tasks Creation**:
   When creating a local task in Sectile, populate `creator` and `creatorAvatar` with the acting authenticated user's name and avatar (or empty if anonymous).
5. **Database Migration**:
   Under ADR 0021, add numbered migration 2 (`version: 2, name: "tasks.creator"`) with `ALTER TABLE tasks ADD COLUMN creator TEXT NOT NULL DEFAULT ''` and `ALTER TABLE tasks ADD COLUMN creator_avatar TEXT NOT NULL DEFAULT ''`.
6. **UI Presentation**:
   - `TaskDetailModal`: Display Creator alongside created/updated dates and assignee in the story header / metadata section, with their avatar.
   - `ListView`: Display Creator alongside or adjacent to Assignee/metadata.
   - Kanban cards retain concise display without creator to prevent clutter.

---

## User Stories & Acceptance Criteria

### User Story 1: GitHub Issue Creator Attribution
As a developer viewing a ticket imported from GitHub,  
I want to see the GitHub username and avatar of who created the issue,  
So that I know who reported it without having to open GitHub.

#### Acceptance Criteria
- **Given** an issue on GitHub created by user `octocat` with avatar `https://avatars.githubusercontent.com/u/1`,
- **When** the project is synchronized via GitHub tracker adapter,
- **Then** the resulting `Task` has `creator = "octocat"` and `creatorAvatar = "https://avatars.githubusercontent.com/u/1"`.
- **When** viewing the task in `TaskDetailModal`,
- **Then** `octocat` and their avatar are clearly displayed as the creator.

---

### User Story 2: Jira Issue Creator Attribution
As a team member viewing a Jira story in Sectile,  
I want to see who originally created the Jira ticket,  
So that I know who the author/reporter is.

#### Acceptance Criteria
- **Given** a Jira issue with `fields.creator` having `displayName: "Ada Lovelace"` and avatar `https://jira.example.com/avatar/ada.png`,
- **When** the project is synchronized via Jira tracker adapter,
- **Then** the resulting `Task` has `creator = "Ada Lovelace"` and `creatorAvatar = "https://jira.example.com/avatar/ada.png"`.
- **Given** a Jira issue where `fields.creator` is null/empty but `fields.reporter` has `displayName: "Charles Babbage"`,
- **When** the project is synchronized,
- **Then** the resulting `Task` falls back to `creator = "Charles Babbage"`.

---

### User Story 3: Local Task Creator Attribution
As a user creating a new task directly in Sectile,  
I want my identity to be recorded as the task's creator,  
So that my team knows who created the task.

#### Acceptance Criteria
- **Given** an authenticated user "Alice" with avatar "https://example.com/alice.png",
- **When** Alice creates a new task in Sectile,
- **Then** the created task has `creator = "Alice"` and `creatorAvatar = "https://example.com/alice.png"`.
- **Given** a task created anonymously or without an authenticated user,
- **Then** `creator` and `creatorAvatar` default to empty strings without error.

---

### User Story 4: UI Display in TaskDetailModal and ListView
As a user browsing tasks,  
I want to see the creator in the task detail modal and in the list view,  
So that authorship is visible when inspecting details or reviewing lists.

#### Acceptance Criteria
- **Given** a task with non-empty `creator` and `creatorAvatar`,
- **When** opened in `TaskDetailModal`,
- **Then** the creator's avatar and name are rendered in the details section (e.g. "Créé par" / "Created by").
- **When** viewed in `ListView`,
- **Then** the creator is visible in the row metadata.
