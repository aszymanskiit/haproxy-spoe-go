# Contributing

## Commit messages (Conventional Commits)

All commits must follow [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/):

```text
<type>[optional scope][optional !]: <description>

[optional body]

[optional footer(s)]
```

Allowed types used by this repository:

| Type | Meaning |
|------|---------|
| `feat` | New feature |
| `fix` | Bug fix |
| `perf` | Performance improvement |
| `refactor` | Code change that is not a fix or feature |
| `docs` | Documentation only |
| `test` | Tests |
| `build` | Build system or dependencies |
| `ci` | CI/CD configuration |
| `chore` | Other maintenance |
| `style` | Formatting / non-functional style |
| `revert` | Revert a previous commit |

Breaking changes: add `!` after the type/scope (for example `feat!: ...`) and/or a `BREAKING CHANGE:` footer.

Valid examples:

```text
feat: add user authentication
fix(api): handle missing authorization header
docs: update installation guide
refactor!: remove deprecated configuration format
feat(payments): support card refunds

BREAKING CHANGE: the old payment configuration is no longer supported
```

Invalid examples:

```text
added login
small fixes
update
Fixed bug
changes
```

GitHub merge commits (`Merge pull request #…`) are ignored by CI. Squash and rebase commits are validated normally.

### Local validation

Install [pommitlint](https://github.com/shuymn/pommitlint) (Go single binary, rules compatible with `@commitlint/config-conventional`):

```bash
go install github.com/shuymn/pommitlint@v2.0.1
# or: brew install shuymn/tap/pommitlint
```

```bash
# lint a message
echo 'feat: add something' | pommitlint lint

# lint the prepared commit message file (commit-msg hook)
pommitlint lint --edit

# optional local hook
pommitlint hook install
```

The GitHub Actions workflow `.github/workflows/validate-commits.yml` runs the same checks on every `push` and `pull_request` (no Node.js).

## Changelog and releases

Publishing a GitHub Release (`release: published`) runs `.github/workflows/release-changelog.yml`, which:

1. Generates notes with [git-cliff](https://github.com/orhun/git-cliff) from commits between the previous `v*` tag and the released tag (or from the start of history for the first tag).
2. Groups entries by Conventional Commit type and highlights breaking changes.
3. Sets the GitHub Release description to that changelog.
4. Prepends the same entry to `CHANGELOG.md` (creates the file if missing).
5. Commits and pushes `CHANGELOG.md` to the repository default branch with message:

   `chore(release): update CHANGELOG.md for <tag> [skip ci]`

`[skip ci]` prevents CI from re-running on that docs-only commit. The changelog workflow itself only listens to `release: published`, so the push cannot re-trigger a release loop.

### Manual re-run

- GitHub UI: **Actions → Release Changelog → Re-run jobs** (for a past `release` workflow run), or publish/edit a release again.
- CLI example:

  ```bash
  gh workflow run release-changelog.yml
  ```

  Note: `workflow_dispatch` is not enabled on that workflow by design; re-run an existing `release` run, or publish a new release.

### Required repository settings

1. **Actions permissions**: allow GitHub Actions to create commits with the default `GITHUB_TOKEN`
   (**Settings → Actions → General → Workflow permissions → Read and write permissions**).
2. **Allow GitHub Actions to create and approve pull requests** is not required for this flow.
3. If the default branch has **branch protection**:
   - either allow `github-actions[bot]` to bypass required reviews / status checks, or
   - temporarily relax rules for changelog commits, or
   - use a fine-scoped machine user PAT stored as a secret and pass it to `actions/checkout` (not configured by default).
4. Do **not** use `pull_request_target` for commit linting; the validate workflow uses `pull_request` with `contents: read` only.

### Limitations

- Historical commits that are not Conventional Commits appear under **Other** (or may be skipped when they match ignore rules such as bare `changelog` messages).
- Pre-release tags are included if you publish them as GitHub Releases; adjust the workflow `if:` condition if you want to exclude them.
- Changing only `CHANGELOG.md` via the bot commit will not publish a new Release by itself.
