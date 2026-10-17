# Changelog

All notable changes to the CLI are documented here. Releases are `v<version>` tags on `main`.

## Unreleased

### Added

- `fopost blog` over the Blog service: `settings get · set · rotate-secret · test-website`, `keywords list · plan · add · update · delete` and `articles list · request · get · edit · regenerate · publish · reject · delete`. `settings set` prints the website secret once when the API issues it; `articles get --body` prints the markdown; `articles publish`, `rotate-secret` and the deletes confirm unless `--yes` is passed.
- `fopost media limits` prints the account's upload ceiling, the multipart per-file ceiling, the next tier and the allowed types.
- `fopost media upload` reads that ceiling and sends any file over it through direct upload, streamed from disk; `--direct` still forces the route for every file.
- `fopost posts create --file <path>` (repeatable, up to five) sends files with the post through the SDK, which picks multipart or direct upload by size.
- Requires `fopost-go` with `Media.Limits` and `PostFile` support (the pin in `go.mod` must be bumped once that SDK version is released).
