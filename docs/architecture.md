# Architecture

## Goals

The project is organized around a few long-term constraints:

- fast CLI startup
- predictable ownership of code
- testability without heavy mocking
- minimal runtime dependencies
- enough structure for growth without premature abstraction

## Layer boundaries

### `cmd/firety`

Owns process startup only:

- reads build metadata
- constructs the application
- executes the CLI

### `internal/cli`

Owns user-facing command concerns:

- command tree definition
- help text
- argument validation
- output formatting
- mapping command execution to application services

Business logic should not live here.

### `internal/app`

Owns dependency wiring:

- assembles concrete services
- carries build metadata
- provides one place to evolve application construction over time

This package should stay small and boring.

### `internal/domain`

Owns concepts that should remain stable even if the CLI, config format, or adapters change.

At the current stage, only capability kinds are defined. More domain types should be added only when they represent real shared concepts.

### `internal/service`

Owns use cases and application behavior.

Today this layer only exposes placeholder behavior so commands do not embed implementation details. As Firety grows, command handlers should call services here.

### `internal/platform`

Owns concrete adapters and infrastructure details:

- build metadata
- future config loaders
- future filesystem, process, network, or API integrations
- future GitHub and cloud adapters

Concrete dependencies should be introduced here, then wired through `internal/app`.

## Testing model

The preferred test pyramid is:

- service/domain unit tests for fast feedback
- command-level tests for user-facing behavior
- a small number of integration tests when real adapters are introduced

Tests should favor real values and simple seams over large mock hierarchies.
