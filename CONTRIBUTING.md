# Contributing

Thanks for contributing to this maintained fork of `haproxy-spoe-go`.

## Development setup

```bash
git clone https://github.com/aszymanskiit/haproxy-spoe-go.git
cd haproxy-spoe-go
go mod download
```

## Checks before opening a pull request

```bash
go mod tidy
git diff --exit-code -- go.mod go.sum
go vet ./...
go test ./...
go test -race ./...
go build ./...
go build ./examples/...
```

If `golangci-lint` is installed:

```bash
golangci-lint run ./...
```

Or use the Makefile:

```bash
make test
make build
make lint   # requires golangci-lint
```

## Guidelines

- Keep changes focused; avoid unrelated refactors.
- Preserve public API compatibility unless the PR explicitly documents a breaking change.
- Add or update tests for behaviour changes.
- Update documentation (`README.md`, `docs/`, `CHANGELOG.md`) when user-visible behaviour changes.
- Do not commit secrets, credentials, or large unrelated binaries.
- For security issues, follow [SECURITY.md](SECURITY.md) instead of opening a public issue.

## Pull requests

1. Fork the repository and create a topic branch.
2. Make your changes with clear commits.
3. Ensure the checks above pass.
4. Open a pull request against the default branch and fill in the PR template.

## Code of conduct expectations

Be respectful and constructive in issues and pull requests. This project builds on the original work of the upstream maintainers and contributors.
