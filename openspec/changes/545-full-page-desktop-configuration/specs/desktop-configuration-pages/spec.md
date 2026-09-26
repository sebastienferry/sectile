# desktop-configuration-pages Specification

## Purpose

Configuration is one full-page desktop surface that presents workstation
settings first and the selected project's settings second, while preserving
their existing behavior and providing an explicit way back to the desktop.

## ADDED Requirements

### Requirement: Configuration is one full-page surface

The desktop SHALL open workstation and project configuration in one full-page
Configuration surface, not in modal dialogs. The surface SHALL provide one
visible, keyboard-accessible Back action that returns to the normal desktop
page.

#### Scenario: Enter Configuration from Settings

- **GIVEN** the normal desktop page is visible
- **WHEN** the user activates Settings
- **THEN** Configuration replaces the normal desktop content without opening
  the shared modal dialog
- **AND** a visible Back button is available to keyboard users
- **AND** a General workstation category is selected.

#### Scenario: Return from Configuration

- **GIVEN** Configuration is visible
- **WHEN** the user activates Back with a pointer or keyboard
- **THEN** the normal desktop page is restored
- **AND** the desktop retains its selected project and selected execution or
  task state.

### Requirement: The Configuration sidebar groups general and project settings

The Configuration sidebar SHALL list workstation-wide categories under a
General group first. When a project configuration is opened, the sidebar SHALL
then list that selected project's categories under a group named for the
project. The page SHALL NOT show categories for other projects.

#### Scenario: Enter Configuration from Project settings

- **GIVEN** a project is available in the desktop sidebar
- **WHEN** the user opens Project settings for that project
- **THEN** the same Configuration page opens without the shared modal dialog
- **AND** General appears before a group named for that project
- **AND** the requested project category is selected.

#### Scenario: Navigate from a general category to a project category

- **GIVEN** Configuration is open for a selected project
- **WHEN** the user chooses a category in the project group after viewing a
  General category
- **THEN** the page displays the selected project's existing configuration
  panel without leaving Configuration
- **AND** the General group remains available before the project group.

### Requirement: Existing configuration behavior is preserved

The unified Configuration page SHALL preserve existing categories, controls,
selected defaults, validation, persistence, and responsive behavior. Project
configuration SHALL preserve the existing placement of local save actions and
the read-only/action behavior of server and deployment panels.

#### Scenario: Save workstation configuration from Configuration

- **GIVEN** Configuration is visible with a General category selected
- **WHEN** the user changes a valid workstation setting and saves or applies it
- **THEN** the existing persistence operation runs
- **AND** the page reports the existing success or error outcome.

#### Scenario: Save project configuration from Configuration

- **GIVEN** Configuration is visible for a project on a category with local
  settings
- **WHEN** the user changes valid local configuration and saves it
- **THEN** the existing project persistence operation runs
- **AND** the page reports the existing success or error outcome
- **AND** read-only server information remains non-editable.

#### Scenario: Narrow Configuration layout

- **GIVEN** Configuration is displayed in a narrow desktop window
- **WHEN** the available width reaches the existing responsive breakpoint
- **THEN** its sidebar groups, categories, and content remain usable in the
  existing responsive layout.

### Requirement: Leaving Configuration prevents stale updates

The desktop SHALL ignore asynchronous configuration results that settle after
the user leaves Configuration.

#### Scenario: Late Configuration result

- **GIVEN** Configuration has a workstation or project request in progress
- **WHEN** the user activates Back before the request completes
- **THEN** the normal desktop page remains visible
- **AND** the late result does not render configuration content or status into
  that page.

### Requirement: Other dialogs remain modal

The desktop SHALL retain the existing shared modal behavior and close control
for dialogs other than Configuration.

#### Scenario: Open an unrelated dialog

- **GIVEN** the normal desktop page is visible
- **WHEN** the user opens an unrelated dialog flow
- **THEN** that flow opens in the shared modal dialog
- **AND** its existing close action remains available.
