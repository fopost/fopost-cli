# Changelog

All notable changes to the CLI are documented here. Releases are `v<version>` tags on `main`.

## Unreleased

### Added

- `fopost blog` over the Blog service: `settings get · set · rotate-secret · test-website`, `keywords list · plan · add · update · delete` and `articles list · request · get · edit · regenerate · publish · reject · delete`. `settings set` prints the website secret once when the API issues it; `articles get --body` prints the markdown; `articles publish`, `rotate-secret` and the deletes confirm unless `--yes` is passed.
