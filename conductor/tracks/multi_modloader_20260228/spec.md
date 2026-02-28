# Specification: Multi-ModLoader Support and Launch Flow Refactoring

## Overview
Currently, SabaLauncher only supports the Forge mod loader and the launch logic is tightly coupled, making it difficult to maintain and extend. This feature adds support for Fabric, NeoForge, and Quilt mod loaders while refactoring the launch flow into a more modular, interface-driven architecture.

## Functional Requirements
- **Loader Support:** Implement full support for Fabric, NeoForge, and Quilt mod loaders (in addition to existing Forge support).
- **Explicit Loader Selection:** The profile manifest MUST include a `mod_loader` field specifying one of `forge`, `fabric`, `neoforge`, or `quilt`.
- **Validation:** Profiles missing the `mod_loader` field MUST trigger a validation error in the UI, requiring an update to the manifest.
- **Unified Launch Interface:** Refactor the launch flow to use a pluggable architecture where loader-specific logic is abstracted away.

## Technical Architecture (Refactoring)
- **`ModLoader` Interface:** A common interface for all loaders.
    - `Install(ctx, profile)`: Handles downloading and setting up loader-specific files.
    - `GenerateLaunchConfig(profile)`: Produces a `LaunchConfig`.
- **`DependencyResolver` Interface:** Abstracts the resolution of libraries, assets, and native dependencies.
- **`LaunchConfig` Object:** A data structure containing everything needed to start the process:
    - Main Class
    - JVM Arguments
    - Game Arguments
    - Classpath (resolved list of JAR paths)
- **Engine/Runner:** The core `GameRunner` will now interact with these interfaces rather than concrete Forge-specific logic.

## Acceptance Criteria
- [ ] Successfully launch a Minecraft instance using Fabric.
- [ ] Successfully launch a Minecraft instance using NeoForge.
- [ ] Successfully launch a Minecraft instance using Quilt.
- [ ] Forge launch remains functional after refactoring.
- [ ] A profile manifest without `"mod_loader"` displays a clear error message to the user.
- [ ] Unit tests cover the `ModLoader` implementations and the new `DependencyResolver`.

## Out of Scope
- Support for other loaders (like LiteLoader or LegacyFabric) not explicitly mentioned.
- UI changes for manual loader selection by the user (selection is driven by the profile manifest).
