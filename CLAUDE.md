# CLAUDE.md

Guidance for Claude Code (claude.ai/code) when working in this repository.

## What This Is

`github.com/fopost/fopost-cli` — the official FoPost command-line interface. It
builds a single static binary named `fopost` (version 0.1.1) that wraps the
official Go SDK, `github.com/fopost/fopost-go`. It is distributed through a
Homebrew tap, GitHub Releases, and `go install`; it is not a library and exports
no public API.

**This repository is a consumer of the SDK, not a second implementation of it.**
HTTP, retries, backoff, the `{"data": …}` envelope, and error typing all live in
`fopost-go`. Nothing here may re-implement them. If an endpoint is missing,
the fix is a change in `fopost-go` (or `client.Do`, its escape hatch) — never a
hand-rolled request in a command.

## Brand Rules

- The product is **FoPost** (`fopost.com`). Never write "OwlStack" — retired Aug 2026.
- Never write an email address. Support is https://fopost.com/contact and GitHub issues.
  This includes config files: `.goreleaser.yaml` deliberately omits `commit_author`
  so GoReleaser supplies its own default rather than an address written here.
- Never name AI providers/models, infrastructure vendors, or any person.
- Legal entity for LICENSE and package metadata: Porter Bridge, LLC,
  131 Continental Dr, Suite 305, Newark, DE 19713.

## Architecture

```
main.go                      thin entry point: os.Exit(cmd.Main())
internal/buildinfo/          Version, Commit, Date — stamped by -ldflags -X
internal/config/             the 0600 config file, and flag > env > file precedence
internal/output/             tables, JSON, NO_COLOR, --quiet
internal/cmd/
  root.go                    State, the SDK client factory, persistent flags
  registry.go                the command registry (see below)
  errors.go                  error → exit code, error → one-line message
  helpers.go                 schedule parsing, --text-file/stdin, confirmation
  <resource>.go              one file per top-level command
```

**A command is a file.** `registry.go` holds a slice of builders, and each
command file appends its own in an `init`. Nothing in `root.go` names a
command, so adding one never edits another file — the same modularity rule the
API's agent tools follow.

How a call flows: cobra parses → `State.Client()` resolves the key and builds
one `*fopost.Client` → the command calls one SDK method → the result goes to
`Printer.Value(v, human)`, which prints indented JSON under `--json` and
otherwise runs the table closure. A command's `RunE` returns the SDK's error
untouched; `Execute` turns it into one stderr line via `Explain` and an exit
code via `ExitCode`.

### Rules that are load-bearing

- **Never print the API key.** `auth status` shows `config.Mask()` only, login
  reads it with echo off, and no command logs it. `internal/config` and
  `internal/cmd` both have tests that fail if a key reaches stdout or stderr.
- **Every command supports `--json`.** A command that renders only a table is
  incomplete — a script has to be able to consume it.
- **Errors go to stderr, results to stdout.** A failed run must never write to
  stdout, or it poisons a pipe.
- `--quiet` silences the human view but **not** `--json`, which is the machine's
  output rather than chatter.
- **`--workspace` is a persistent root flag**, resolved through
  `State.WorkspaceID("")`. Do not add a per-command workspace flag.
- **Nothing publishes without an explicit action.** `posts create` makes a draft
  unless `--schedule-at` or `--publish` is passed. Keep it that way.
- Destructive commands prompt unless `-y/--yes` is passed, and their copy ends
  with `This cannot be undone.`

### Why a hand-rolled config instead of viper

Viper would pull in a dozen transitive modules to solve a three-source
precedence chain (`--api-key` > `FOPOST_API_KEY` > the config file) over a
single JSON file with four fields. It also cannot enforce mode `0600` on write,
which is the one property that actually matters here. `internal/config` is
~150 lines, fully tested, and keeps the dependency list at cobra plus
`golang.org/x/term`. Do not add viper.

## API Contract

Owned by `fopost-go`; repeated here only so a change is recognisable:

- Base URL `https://api.fopost.com/v1`, overridable with `--base-url` or `FOPOST_BASE_URL`
- Auth header `X-API-Key` (not Bearer)
- Success envelope `{"data": …}`; paginated lists add snake_case `meta`
- Error envelope `{"error": "<code>", "message": "<text>"}`; `402` may carry `upgrade_url`
- Retries: 3 attempts, exponential backoff from 500 ms capped at 60 s, only on
  `429`, `5xx`, and network errors, honouring `Retry-After`

Exit codes are this repo's contract and are documented in the README table.
Changing one is a breaking change for anyone's script.

## Parent dependency

`github.com/fopost/fopost-go` is **published** to the Go module proxy, so CI
resolves it normally with no shim. It is currently pinned to a pseudo-version
because the SDK has no `v*` tag yet; once it is tagged, bump `go.mod` to the
released version.

The pin currently points at a commit on the SDK's `feature/contacts` branch,
which is what `fopost contacts` needs. Re-point it at `main` — or at the tag —
once that branch merges, or a `go get -u` will silently walk it backwards.

`go.mod` declares `go 1.22`, which is why `golang.org/x/term` is pinned to
`v0.27.0` — from `v0.34.0` it requires a newer toolchain. CI builds on 1.22 and
on stable, so a dependency bump that raises the floor fails there first.

## Commands

```bash
go build ./...                 # build
go test ./...                  # every test, fully offline
go test -race ./...            # what CI runs
go vet ./...
gofmt -l .                     # must print nothing
go run . --help                # try the CLI without installing it

go build -ldflags "-X github.com/fopost/fopost-cli/internal/buildinfo.Version=0.1.1" -o fopost .
```

Tests never reach the network. They run the real cobra tree in-process through
`Execute(args, stdin, stdout, stderr)` against an `httptest.Server` pointed at
via `--base-url`, and `isolate(t)` redirects `XDG_CONFIG_HOME` to a temp
directory and clears `FOPOST_API_KEY`, so a developer's own key can never leak
into a run. Add a test for anything that fails silently — a leaked key, a wrong
exit code, a command that stops emitting `--json` — and nothing for layout.

## Releasing

Bump `Version` in `internal/buildinfo/buildinfo.go`, then tag `v<version>`.
`.github/workflows/release.yml` checks the tag against that constant, then runs
GoReleaser: six binaries (darwin, linux, windows × amd64, arm64), `checksums.txt`,
the GitHub Release, and the Homebrew formula.

- `GITHUB_TOKEN` is provided by Actions; the workflow grants it `contents: write`.
- **`HOMEBREW_TAP_TOKEN`** must be added as a repository secret — a token with
  write access to the tap. The built-in `GITHUB_TOKEN` cannot push to another
  repository.
- **The tap repository `fopost/homebrew-tap` must exist before the first
  release**, or GoReleaser fails at the formula step after everything else has
  already published. Create it (public, with a `Formula/` directory) first.
- `go install github.com/fopost/fopost-cli@latest` works with no release at all,
  straight from `main` — it just yields a binary named `fopost-cli`, which the
  README tells the user to rename.
- `brews:` is the formula block. GoReleaser has been steering toward
  `homebrew_casks` for prebuilt binaries; if a release fails on a deprecation,
  that is the migration, not a bug in the build.

## Conventions

- Idiomatic Go, `gofmt`-clean, doc comments on exported symbols.
- Comments explain a non-obvious "why" in one line. No narrated docblocks.
- Command help is sentence case; `Short` is a verb phrase with no trailing period.
- User-facing copy follows the product's voice: Title Case labels, `Verb + Noun`
  on an action, and destructive confirmations close with `This cannot be undone.`
- No third dependency without a reason that survives the viper argument above.

## Git

Conventional Commits, atomic — one logical change per commit. Branch
`feature/<description>`, merged to `main` via PR. Never run `gh pr create`; push
the branch and hand over the compare link
`https://github.com/fopost/fopost-cli/compare/main...<branch>`.
