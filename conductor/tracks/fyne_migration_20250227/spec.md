# Specification: UI Migration to Fyne.io & Logic Decoupling

## Overview
This track involves migrating the SabaLauncher user interface from `gioui.org` to `fyne.io` and, crucially, refactoring the codebase to separate UI concerns from core launcher logic. The current intertwined "spaghetti" logic will be decoupled into clean, testable interfaces, allowing the UI to act as a pure presentation layer.

## Architectural Requirements
- **UI/Core Separation:** Implement a clear boundary between the UI (Fyne) and the core launcher logic (Modpack management, authentication, process execution).
- **Interface-Driven Design:** Define interfaces for core services (e.g., `Authenticator`, `ProfileManager`, `GameRunner`) that the UI consumes, facilitating easier testing and future-proofing.
- **Event/Observer Pattern:** Use reactive patterns or dedicated channels to communicate background process updates (like download progress) to the UI without tight coupling.

## Functional Requirements
- **Core UI Re-implementation:** Rebuild all views using Fyne widgets.
    - **Main Dashboard:** Profile list, server status, and "Play" button.
    - **Profile Management:** Dialogs/Views for adding profile URLs and managing existing ones.
    - **Authentication Flow:** Microsoft Login integrated via the new service interfaces.
    - **Feedback Systems:** Real-time progress bars and log viewers driven by decoupled event streams.
- **Dependency Replacement:** Complete removal of `gioui.org` and its event loop logic.

## Non-Functional Requirements
- **Feature Parity:** 100% functional parity with the current Gio implementation.
- **Maintainability:** A clear directory structure where `internal/ui` is distinct from `internal/core`.
- **Testability:** Core logic must be testable without initializing a GUI.

## Acceptance Criteria
- [ ] The application launches successfully using Fyne.
- [ ] UI code and Core logic code reside in distinct, decoupled packages.
- [ ] Core services (Auth, Profile, Launch) are defined by interfaces and have unit tests.
- [ ] All `gioui.org` dependencies are removed from `go.mod`.
- [ ] 100% of existing features (Auth, Profile management, Download, Launch, Discord) function correctly.

## Out of Scope
- Introducing new user-facing features.
- Support for multiple UI backends (the focus is on separation, not abstraction for the sake of it).
