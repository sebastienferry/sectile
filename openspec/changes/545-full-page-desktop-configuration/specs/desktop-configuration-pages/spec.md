# desktop-configuration-pages Specification

## Purpose

Workstation Settings and project configuration are full-page desktop surfaces
that preserve their existing configuration behavior and provide an explicit
way back to the normal desktop page.

## ADDED Requirements

### Requirement: Workstation Settings is a full-page surface

The desktop SHALL open workstation Settings as a full-page application surface,
not as a modal dialog. The surface SHALL provide a visible, keyboard-accessible
Back action that returns to the normal desktop page.

#### Scenario: Open and leave workstation Settings

- **GIVEN** the normal desktop page is visible
- **WHEN** the user activates Settings
- **THEN** workstation Settings replaces the normal desktop content without
  opening the shared modal dialog
- **AND** its settings categories are available
- **AND** a visible Back button is available to keyboard users.

#### Scenario: Return from workstation Settings

- **GIVEN** workstation Settings is visible
- **WHEN** the user activates Back with a pointer or keyboard
- **THEN** the normal desktop page is restored
- **AND** the desktop retains its selected project and selected execution or
  task state.

### Requirement: Project configuration is a full-page surface

The desktop SHALL open a project's configuration as a full-page application
surface, not as a modal dialog. The surface SHALL provide a visible,
keyboard-accessible Back action that returns to the normal desktop page.

#### Scenario: Open and leave project configuration

- **GIVEN** a project is available in the desktop sidebar
- **WHEN** the user opens Project settings for that project
- **THEN** that project's configuration replaces the normal desktop content
  without opening the shared modal dialog
- **AND** its configuration categories are available
- **AND** a visible Back button is available to keyboard users.

#### Scenario: Return from project configuration

- **GIVEN** a project's configuration is visible
- **WHEN** the user activates Back with a pointer or keyboard
- **THEN** the normal desktop page is restored
- **AND** the project remains available and its existing desktop state is
  retained.

### Requirement: Existing configuration behavior is preserved

The full-page configuration surfaces SHALL preserve their existing categories,
controls, selected defaults, validation, persistence, and responsive behavior.
Project configuration SHALL preserve the existing placement of local save
actions and the read-only/action behavior of server and deployment panels.

#### Scenario: Save workstation configuration from the full page

- **GIVEN** workstation Settings is visible
- **WHEN** the user changes a valid workstation setting and saves or applies it
- **THEN** the existing persistence operation runs
- **AND** the page reports the existing success or error outcome.

#### Scenario: Save project configuration from the full page

- **GIVEN** project configuration is visible on a category with local settings
- **WHEN** the user changes valid local configuration and saves it
- **THEN** the existing project persistence operation runs
- **AND** the page reports the existing success or error outcome
- **AND** read-only server information remains non-editable.

#### Scenario: Narrow full-page layout

- **GIVEN** either configuration page is displayed in a narrow desktop window
- **WHEN** the available width reaches the existing responsive breakpoint
- **THEN** its category navigation and content remain usable in the existing
  responsive layout.

### Requirement: Leaving a configuration page prevents stale updates

The desktop SHALL ignore asynchronous configuration results that settle after
the user leaves the corresponding full-page surface.

#### Scenario: Late workstation Settings result

- **GIVEN** workstation Settings has an asynchronous request in progress
- **WHEN** the user activates Back before the request completes
- **THEN** the normal desktop page remains visible
- **AND** the late result does not render Settings content or status into that
  page.

#### Scenario: Late project configuration result

- **GIVEN** project configuration has an asynchronous request in progress
- **WHEN** the user activates Back before the request completes
- **THEN** the normal desktop page remains visible
- **AND** the late result does not render project configuration content or
  status into that page.

### Requirement: Other dialogs remain modal

The desktop SHALL retain the existing shared modal behavior and close control
for dialogs other than workstation Settings and project configuration.

#### Scenario: Open an unrelated dialog

- **GIVEN** the normal desktop page is visible
- **WHEN** the user opens an unrelated dialog flow
- **THEN** that flow opens in the shared modal dialog
- **AND** its existing close action remains available.
