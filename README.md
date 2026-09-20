# fopost-cli

[![CI](https://github.com/fopost/fopost-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/fopost/fopost-cli/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/fopost/fopost-cli.svg)](https://pkg.go.dev/github.com/fopost/fopost-cli)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

The official command-line interface for [FoPost](https://fopost.com). Schedule,
publish, and analyze social media content across +30 platforms without leaving
your terminal.

A single static binary. Every command speaks to the FoPost API through the
[official Go SDK](https://github.com/fopost/fopost-go) — the CLI adds the flags,
tables, and exit codes, and nothing else.

> **0.x release.** The command surface is still settling and minor versions may
> change it.

## Install

**Homebrew**

```bash
brew install fopost/tap/fopost
```

**Go**

```bash
go install github.com/fopost/fopost-cli@latest
```

That installs a binary named `fopost-cli`, because Go names it after the module.
Rename it once if you want the short name:

```bash
mv "$(go env GOPATH)/bin/fopost-cli" "$(go env GOPATH)/bin/fopost"
```

**Direct download**

Grab the archive for your platform from the
[releases page](https://github.com/fopost/fopost-cli/releases), verify it against
`checksums.txt`, and put `fopost` on your `PATH`:

```bash
tar -xzf fopost_0.1.1_darwin_arm64.tar.gz
sudo mv fopost /usr/local/bin/
fopost version
```

Requires Go 1.22 or newer to build from source. Prebuilt binaries cover macOS,
Linux, and Windows on both amd64 and arm64.

## Authenticate

Create a key in the FoPost dashboard under **Settings → API Keys**, then:

```bash
fopost auth login
# API key: (input is hidden)
# ✓ Signed in as fp_••••••••3d5g
```

The key is stored at `~/.config/fopost/config.json` with mode `0600`
(`$XDG_CONFIG_HOME` is honoured). It is never printed back — `auth status` shows
only a masked prefix, and verbose output never contains it.

```bash
fopost auth status     # which key is in effect, and where it came from
fopost auth logout     # remove it
```

Three sources, in order of precedence:

| Source | Example |
| :-- | :-- |
| `--api-key` flag | `fopost posts list --api-key fp_...` |
| `FOPOST_API_KEY` | `FOPOST_API_KEY=fp_... fopost posts list` |
| config file | written by `fopost auth login` |

In CI, set `FOPOST_API_KEY` as a secret and skip `auth login` entirely.

`fopost auth login --workspace ws_123` saves a default workspace, so
workspace-scoped commands need no `--workspace` flag. With exactly one reachable
workspace, login picks it for you.

## Schedule a post

```bash
# Which accounts can I post to?
fopost accounts list

# Attach a local image, schedule for next Tuesday morning, and check it first.
fopost posts create \
  --account acc_9f2c4a7b \
  --account acc_1ee3d5a0 \
  --text "We shipped dark mode. Every surface, every chart, no flash on load." \
  --media ./screenshots/dark-mode.png \
  --schedule-at "2026-09-01 09:00"
# ✓ Scheduled post_7d2a for 2026-09-01 07:00

fopost posts preflight post_7d2a
fopost posts deliveries post_7d2a
```

`--schedule-at` reads RFC 3339 (`2026-09-01T09:00:00Z`) or a plain
`"2026-09-01 09:00"` in your local timezone. Files passed with `--media` are
uploaded to the workspace's media library first, then attached.
`fopost media upload --direct <file>` sends each file straight to storage
through a presigned URL instead of through the API.

To send something out now, create and publish in one step:

```bash
fopost posts create --account acc_9f2c4a7b --text "Live now." --publish
```

Publishing returns once the deliveries are **queued**, not once they are live.
Poll `fopost posts deliveries <id>` for the outcome.

Long copy comes from a file, or a pipe:

```bash
fopost posts create --account acc_9f2c4a7b --text-file announcement.md --draft

git log -1 --pretty=%s | \
  fopost posts create --account acc_9f2c4a7b --text-file - --draft
```

Nothing reaches a platform without `--publish`, an explicit `posts publish`, or
a schedule you set.

## Scripting with `--json`

Every command takes `--json` and prints the resource instead of a table, so it
composes with `jq`:

```bash
# Every account whose credentials have gone stale.
fopost accounts health --json \
  | jq -r '.accounts[] | select(.healthStatus != "healthy") | .id'

# Re-validate each of them, stopping at the first hard failure.
fopost accounts health --json \
  | jq -r '.accounts[] | select(.healthStatus != "healthy") | .id' \
  | xargs -I{} fopost accounts validate {} --quiet

# The id of the post you just created, for the next step in a pipeline.
POST_ID=$(fopost posts create --account acc_1 --text "…" --draft --json | jq -r '.post.id')
fopost posts publish "$POST_ID" --json | jq '.deliveries[] | {accountId, status}'
```

`--quiet` silences the human-facing output while keeping the exit code, and
`--json` still prints under `--quiet` — the machine output is not chatter.
Colour is dropped when `NO_COLOR` is set, when `--no-color` is passed, or when
output is not a terminal.

### Exit codes

| Code | Meaning |
| :-- | :-- |
| `0` | success |
| `1` | an error that has no more specific code |
| `2` | bad invocation — a missing flag, a contradictory pair, a cancelled prompt |
| `3` | `401` — no key, or the key is invalid |
| `4` | `402` — the plan does not cover this; the message carries the upgrade URL |
| `5` | `403` — the key lacks the scope or workspace access |
| `6` | `404` — no such resource |
| `7` | `429` — rate limited; the message says when to retry |
| `8` | `400`/`422` — the request was rejected as invalid |
| `9` | `5xx` — the API failed after the SDK's retries |
| `10` | the API could not be reached |

Errors print one line to stderr, never to stdout, so a failed run never
contaminates a pipe.

## Shell completion

```bash
# bash — add to ~/.bashrc
source <(fopost completion bash)

# zsh — add to ~/.zshrc, and make sure compinit runs
source <(fopost completion zsh)

# fish
fopost completion fish | source

# powershell
fopost completion powershell | Out-String | Invoke-Expression
```

To install it permanently instead of per shell:

```bash
fopost completion zsh > "${fpath[1]}/_fopost"
fopost completion bash > /etc/bash_completion.d/fopost
```

Homebrew installs completions for you.

## Commands

```
fopost auth          login · status · logout
fopost workspaces    list · get · create
fopost accounts      list · get · rename · move · health · validate · refresh
                     telegram connect-code · connect-status
                     telegram commands get · set · clear
                     slack channels · members · identity · set-identity
                     messaging ice-breakers · persistent-menu · greeting (get · set · clear)
                     webhook-subscription · webhook-subscription resubscribe
fopost account-groups list · get · create · rename · set-members · delete
fopost posts         list · get · create · publish · cancel · delete
                     duplicate · preflight · deliveries
fopost media         list · upload · delete
fopost labels        list · create · delete
fopost analytics     overview · top-posts · time-series
fopost automations   list · get · toggle · trigger · runs
fopost webhooks      list · create · test · delete
fopost ads           tree · pause · resume · insights · leads
fopost completion    bash · zsh · fish · powershell
fopost version
```

Run `fopost <command> --help` for the flags on any of them.

Global flags, accepted everywhere: `--api-key`, `--base-url`, `--workspace`,
`--timeout`, `--json`, `--quiet`, `--no-color`.

## Examples

[`examples/daily-digest.sh`](examples/daily-digest.sh) is a runnable script that
picks a workspace, checks account health, composes a post from a file, previews
it with `preflight`, and publishes it — the whole loop in `--json` mode.

## Retries and rate limits

Handled by the SDK, not the CLI: three attempts per request, exponential backoff
from 500 ms capped at 60 s, retrying only `429`, `5xx`, and network failures, and
honouring `Retry-After`. When the retries are exhausted the CLI prints the reason
and exits `7` or `9`, so a script can back off on its own terms.

## Support

- Documentation: <https://fopost.com/docs>
- Contact: <https://fopost.com/contact>
- Issues: <https://github.com/fopost/fopost-cli/issues>

## License

MIT © Porter Bridge, LLC. See [LICENSE](LICENSE).
