# AGENTS.md — Coding Standards & AI Development Guidelines

This document defines the coding standards, testing expectations, and conventions for AI-assisted development on Cartographer. Any AI agent contributing to this codebase must follow these guidelines.

## Language & Toolchain

- **Go 1.25+** with modules enabled.
- **Linter**: `golangci-lint` — all code must pass with zero issues before committing.
- **Test framework**: `testing` stdlib + `github.com/stretchr/testify` (assert/require).
- **CLI framework**: Cobra + Viper.
- **CI**: GitHub Actions runs lint, test, and build on every push and PR.

## Code Style

- Follow standard Go conventions: `gofmt`, `goimports`, short variable names in small scopes, clear naming in exported APIs.
- Prefer early returns over deep nesting.
- Keep functions focused — one responsibility per function. If a function exceeds ~50 lines, consider splitting it.
- Use `log.WithField`/`log.WithFields` for structured logging. Always include a `"func"` field.
- Do not add comments that restate what the code does. Only comment on **why** something is done when the reason isn't obvious.
- Do not add docstrings, type annotations, or comments to code you didn't change.

## Testing Expectations

### Coverage
- **Target**: 85%+ overall, no exported function below 80%.
- Run `go test -coverprofile=coverage.out ./...` and `go tool cover -func=coverage.out` to verify.
- CI enforces lint, test, and build on every push.

### Test Structure
- **Unit tests** go in `_test.go` files alongside the code they test, in the same package (for internal functions) or `_test` package (for black-box testing of exported APIs).
- **Integration tests** exercise the full pipeline: YAML parsing → dependency analysis → output generation. Use embedded YAML constants, not external fixture files.
- **CLI integration tests** invoke `cmd.RootCmd.Execute()` with `SetArgs()` and capture output via `bytes.Buffer` with `SetOut()`/`SetErr()`.
- **Real-world tests** model manifests after actual Helm chart patterns (e.g., Bitnami `app.kubernetes.io/*` labels) to catch issues that synthetic tests miss.

### Test Quality
- Every new feature must include tests that verify both **positive** (correct matches) and **negative** (correct non-matches) behavior.
- Use table-driven tests for functions with multiple input/output cases.
- Use `t.Run()` subtests for logical grouping within a test function.
- Use `require` for preconditions that must hold (test aborts on failure). Use `assert` for the actual assertions being tested.
- Use `t.Helper()` in test helper functions so failure messages point to the calling test.
- Use `t.Cleanup()` for teardown instead of `defer` in test helpers.
- **All output formats** (DOT, Mermaid, JSON, PNG, SVG) must have integration tests for any new feature that affects graph output. Do not ship a feature tested in only one format.

### Test Naming
- `TestFunctionName` for basic unit tests.
- `TestFunctionName_SpecificScenario` for scenario-specific tests.
- `TestAnalyzeCommand_FeatureName_Format` for CLI integration tests (e.g., `TestAnalyzeCommand_MatchExpressions_MermaidStdout`).

## Architecture

### Package Layout
- `cmd/` — CLI commands (Cobra). Thin layer that parses flags and delegates to `pkg/`.
- `cmd/analyze/` — The `analyze` subcommand.
- `cmd/version/` — The `version` subcommand. Version injected via `-ldflags` at build time.
- `pkg/dependency/` — Core dependency analysis engine: graph building, handlers, output generation.
- `pkg/helm/` — Helm SDK integration for chart rendering.
- `pkg/parser/` — YAML parsing into Kubernetes unstructured objects.

### Adding New Dependency Handlers
When adding support for a new Kubernetes resource type:
1. Add a handler function in `pkg/dependency/handlers.go` following the existing pattern (accept the resource, label index, and deps map).
2. Add the handler dispatch in `BuildDependencies()` in `pkg/dependency/dependency.go`.
3. Add unit tests in `handlers_test.go` with inline unstructured objects.
4. Add integration tests in `dependency_test.go` with embedded YAML manifests.
5. Add CLI integration tests in `cmd/analyze/analyze_test.go` covering all output formats.
6. Update the "Supported Resource Types" table in `README.md`.

### Label Selector Matching
- Services use flat `map[string]string` selectors — use `LabelIndex.Match()`.
- NetworkPolicy and PDB use full `metav1.LabelSelector` (matchLabels + matchExpressions) — use `LabelIndex.MatchSelector()`.
- Always extract both `matchLabels` and `matchExpressions` from selector maps.

## Commit & Release Standards

- **Commit messages**: Use conventional commits (`feat`, `fix`, `test`, `ci`, `docs`, `refactor`). Include a scope in parentheses (e.g., `feat(dependency): add RBAC handler`).
- **Commit body**: Explain the **why**, not just the what. Keep the first line under 72 characters.
- **Releases**: Versioned via git tags. GoReleaser handles binary builds and Homebrew tap updates automatically on tag push.
- **Tag messages**: Include a one-line summary and a brief list of highlights.
- All tests must pass and lint must be clean before tagging a release.

## What Not To Do

- Do not introduce new dependencies without justification. Prefer stdlib where possible.
- Do not add feature flags or backward-compatibility shims — just change the code.
- Do not create helper utilities for one-time operations.
- Do not add error handling for scenarios that cannot happen.
- Do not modify files unrelated to the current task (no drive-by refactors).
- Do not create documentation files (README, etc.) unless explicitly requested.
- Do not skip pre-commit hooks or linting.
- Do not commit scratch files, temporary manifests, or debug output.
