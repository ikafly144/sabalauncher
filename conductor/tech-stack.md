# Technology Stack

## Core Language & Runtime
- **Go (1.26.0):** The primary programming language for the launcher, chosen for its performance, ease of deployment, and strong standard library.

## User Interface
- **Gio (gioui.org):** A portable Go library for building immediate-mode graphical user interfaces. It enables high-performance, hardware-accelerated UIs.

## Authentication & Security
- **Microsoft Authentication Library (MSAL) for Go:** Used to handle secure OAuth2 authentication with Microsoft accounts, required for Minecraft licensing.

## Platform Integration & Features
- **Discord Rich Presence (rich-go):** Integrates the launcher with Discord to display the user's current game status.
- **GitHub API (go-github):** Used for version checking and potentially downloading updates or Modpacks hosted on GitHub.
- **gosigar:** For system resource monitoring (memory, CPU).

## Packaging & Distribution
- **WiX Toolset (MSI):** The standard format for Windows installers, ensuring a professional and reliable installation experience for users.

## Architectural Patterns
- **Modular Monolith:** The project is organized into clear packages (e.g., `pages` for UI, `pkg` for core logic) to maintain separation of concerns while keeping the deployment simple.
