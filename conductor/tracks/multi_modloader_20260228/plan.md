# Implementation Plan: Multi-ModLoader Support and Launch Refactoring

This plan outlines the steps to refactor the launch flow and add support for Fabric, NeoForge, and Quilt.

## Phase 1: Core Abstractions and Interfaces [checkpoint: c75d87d]
This phase focuses on defining the new architecture.

- [x] Task: Define Core Interfaces [eeb8c9b]
    - [x] Create `ModLoader` interface in `pkg/resource/mod_loader.go`
    - [x] Create `DependencyResolver` interface
    - [x] Define `LaunchConfig` struct
- [x] Task: Update Profile Manifest Schema [5047b9f]
    - [x] Add `ModLoader` field to `ProfileManifest` struct
    - [x] Update JSON parsing and validation logic
- [x] Task: Conductor - User Manual Verification 'Core Abstractions' (Protocol in workflow.md)

## Phase 2: Refactor Forge Implementation
Move existing Forge logic into the new modular structure.

- [x] Task: Implement `ForgeLoader` [7e85267]
    - [x] Write failing tests for `ForgeLoader` (Red)
    - [x] Port existing Forge installation logic to `ForgeLoader.Install` (Green)
    - [x] Port existing Forge argument generation to `ForgeLoader.GenerateLaunchConfig`
- [x] Task: Refactor GameRunner Integration
    - [x] Update `GameRunner` to use `ModLoader` and `LaunchConfig`
    - [x] Verify Forge still launches correctly after refactoring
- [ ] Task: Conductor - User Manual Verification 'Forge Refactoring' (Protocol in workflow.md)

## Phase 3: Add Support for New Loaders
Implement the new mod loaders following the same pattern.

- [ ] Task: Implement `FabricLoader`
    - [ ] Write tests for Fabric installation and config (Red)
    - [ ] Implement Fabric installation logic (Green)
- [ ] Task: Implement `NeoForgeLoader`
    - [ ] Write tests for NeoForge (Red)
    - [ ] Implement NeoForge installation logic (Green)
- [ ] Task: Implement `QuiltLoader`
    - [ ] Write tests for Quilt (Red)
    - [ ] Implement Quilt installation logic (Green)
- [ ] Task: Conductor - User Manual Verification 'New Loaders' (Protocol in workflow.md)

## Phase 4: Validation and UX Improvements
Ensure errors are handled gracefully.

- [ ] Task: Implement Manifest Validation
    - [ ] Add checks for mandatory `mod_loader` field
    - [ ] Create user-friendly error messages for missing/invalid loaders
- [ ] Task: Final Integration Test
    - [ ] Verify all 4 loaders work in a production-like environment
- [ ] Task: Conductor - User Manual Verification 'Validation and UX' (Protocol in workflow.md)
