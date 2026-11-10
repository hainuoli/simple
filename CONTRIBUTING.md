# Contributing to Simple

Thank you for your interest in improving Simple. Small, focused changes with
clear motivation and regression tests are especially welcome.

## Before you start

- Search existing issues and pull requests before proposing a change.
- For API changes or new packages, open an issue first to discuss the design.
- For security vulnerabilities, use the private reporting process in
  [SECURITY.md](SECURITY.md). Do not create a public issue.

## Workflow

1. Fork the repository, or create a topic branch if you have write access.
2. Make one logical change per pull request.
3. Include tests when modifying behavior, and update documentation when needed.
4. Run the checks below when the Go module and packages are present.
5. Open a pull request using the template, explaining the problem and solution.

## Go conventions

- Follow [Effective Go](https://go.dev/doc/effective_go) and Go's code review comments.
- Format Go files with `gofmt`.
- Prefer small, explicit APIs and composition over unnecessary abstractions.
- Keep package dependencies minimal; do not couple independent subsystems.
- Add table-driven regression tests for bug fixes where appropriate.
- Preserve standard library behavior unless the change is deliberate and documented.

When the repository contains a Go module, run from its root:

```sh
go mod verify
go vet ./...
go test ./...
go test -race ./...
```

## Pull requests

- Use a concise, descriptive title, for example `fix(rest): preserve encoded paths`.
- Explain the bug or motivation, the approach, and how you verified the change.
- Avoid unrelated refactoring and large dependency additions in bug-fix PRs.
- If a behavior is not covered by tests, explain why.

No DCO sign-off is required at this time.
