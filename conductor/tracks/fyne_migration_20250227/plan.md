# Plan: UI Migration to Fyne & Logic Decoupling

## Phase 1: Architecture Definition & Package Scaffolding [checkpoint: ac1f04f]
- [x] Task: Define Core Interfaces
    - [x] Create `pkg/core` package and define `Authenticator`, `ProfileManager`, and `GameRunner` interfaces.
    - [x] Define event/channel structures for background process updates (progress, logs).
- [x] Task: Scaffold New Project Structure
    - [x] Create `pkg/ui/fyne` for the new UI implementation.
    - [x] Ensure existing logic in `pkg` is prepared for decoupling.
- [x] Task: Conductor - User Manual Verification 'Architecture Definition & Package Scaffolding' (Protocol in workflow.md) [ac1f04f]

## Phase 2: Core Logic Refactoring (Decoupling)
- [x] Task: Refactor Authentication Logic
    - [x] Write unit tests for the `Authenticator` implementation.
    - [x] Extract Microsoft Auth logic from UI-coupled code into a standalone service.
- [x] Task: Refactor Profile Management Logic
    - [x] Write unit tests for the `ProfileManager` implementation.
    - [x] Decouple profile loading, adding, and deleting from Gio-specific structures.
- [~] Task: Refactor Game Launch & Update Logic
    - [ ] Write unit tests for the `GameRunner` implementation (using mocks for process execution).
    - [ ] Decouple download progress and log streaming into channel-based events.
- [ ] Task: Conductor - User Manual Verification 'Core Logic Refactoring (Decoupling)' (Protocol in workflow.md)

## Phase 3: Fyne UI Foundation & Component Scaffolding
- [ ] Task: Initialize Fyne Application
    - [ ] Set up the main Fyne app loop and basic window management.
    - [ ] Configure the default Fyne theme (Clean & Modern).
- [ ] Task: Create UI Component Primitives
    - [ ] Implement reusable widgets or layouts for navigation and status headers.
- [ ] Task: Conductor - User Manual Verification 'Fyne UI Foundation & Component Scaffolding' (Protocol in workflow.md)

## Phase 4: Page Re-implementation
- [ ] Task: Implement Authentication View
    - [ ] Build the Microsoft login popup/dialog using Fyne.
    - [ ] Connect the view to the `Authenticator` service.
- [ ] Task: Implement Profile Management Views
    - [ ] Rebuild the profile list and "Add Profile" dialogs.
    - [ ] Connect the view to the `ProfileManager` service.
- [ ] Task: Implement Main Dashboard & Launch Controls
    - [ ] Build the main server list and the primary "Play" button.
    - [ ] Connect the view to the `GameRunner` service.
- [ ] Task: Conductor - User Manual Verification 'Page Re-implementation' (Protocol in workflow.md)

## Phase 5: Feedback Systems & Integration
- [ ] Task: Implement Real-time Progress & Logs
    - [ ] Create a dedicated view/overlay for download progress bars and live logs.
    - [ ] Pipe events from the `GameRunner` service to the Fyne UI components.
- [ ] Task: Implement Discord Rich Presence Integration
    - [ ] Re-hook the Discord service to the new UI state and lifecycle.
- [ ] Task: Conductor - User Manual Verification 'Feedback Systems & Integration' (Protocol in workflow.md)

## Phase 6: Final Integration & Gio Removal
- [ ] Task: End-to-End System Testing
    - [ ] Verify the full flow: Login -> Add Profile -> Download -> Play.
- [ ] Task: Remove Legacy Gio Dependencies
    - [ ] Delete `gioui.org` related code and cleanup `go.mod`.
    - [ ] Remove old UI packages (`pages`, `applayout`).
- [ ] Task: Conductor - User Manual Verification 'Final Integration & Gio Removal' (Protocol in workflow.md)
