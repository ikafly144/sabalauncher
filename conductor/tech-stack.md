# Technology Stack

## Core Language & Runtime
- **Go (1.26.0):** The primary programming language for the launcher, chosen for its performance, ease of deployment, and strong standard library.

## User Interface
- **Fyne (fyne.io):** A modern UI toolkit for Go that enables building cross-platform, natively-styled applications with ease. Chosen for its rich widget set and ease of development compared to immediate-mode frameworks.

## Authentication & Security
- **Microsoft Authentication Library (MSAL) for Go:** Used to handle secure OAuth2 authentication with Microsoft accounts, required for Minecraft licensing.
- **DPAPI (Data Protection API):** Used on Windows to securely encrypt and store user credentials and session tokens locally.

## Platform Integration & Features
- **Discord Rich Presence (rich-go):** Integrates the launcher with Discord to display the user's current game status.
- **GitHub API (go-github):** Used for version checking and potentially downloading updates or Modpacks hosted on GitHub.
- **gosigar:** For system resource monitoring (memory, CPU).

## Packaging & Distribution
- **WiX Toolset (MSI):** The standard format for Windows installers, ensuring a professional and reliable installation experience for users.

## Architectural Patterns
- **UI/Core Decoupling:** Strict separation between the presentation layer (Fyne) and core launcher logic via Go interfaces.
- **Interface-Driven Design:** Core services (Authenticator, ProfileManager, GameRunner) are abstracted behind interfaces to facilitate testing and maintainability.
