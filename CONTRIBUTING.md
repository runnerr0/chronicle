# Contributing to Chronicle

## Development Setup

1. Install Go 1.22+
2. Install golangci-lint: `go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest`
3. Clone: `git clone https://github.com/runnerr0/chronicle.git`
4. Build: `make build`
5. Test: `make test`

## Branch Strategy

- `main` — stable, releases cut from here
- Feature branches: `feature/<task-id>-<short-description>` (e.g., `feature/P0-4-storage-layer`)
- Bug fixes: `fix/<short-description>`

## PR Guidelines

- One focused concern per PR (maps to one task from TASK_BREAKDOWN.md)
- Include tests for new functionality
- Run `make lint` and `make test` before submitting
- Reference the task ID in your PR title (e.g., "P0-4: Implement storage layer CRUD")
- Keep PRs small — under 500 lines of changes when possible

## Code Style

- Follow standard Go conventions (`gofmt`, `go vet`)
- Use `go-flags` for CLI flags (not Cobra) — matches fabric's approach
- Error messages: lowercase, no trailing punctuation
- Test files: `*_test.go` alongside the code they test

## Architecture

See `ARCHITECTURE.md` in the spec bundle for the full component architecture.
The project follows fabric's patterns where possible:
- Sequential handler chain for CLI commands
- Gin for HTTP server
- SQLite for persistence
