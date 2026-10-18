# Changelog

All notable changes to the CLI are documented here. Releases are `v<version>` tags on `main`.

## 0.3.0 - 2026-10-18

### Added

- `fopost queue slots list · create · update · delete`, `queue next` and `queue preview` manage the posting queue, and `posts create --schedule queue` drops a post into the next free slot.
- `fopost posts unpublish` takes a live post down on every network; it confirms unless `--yes` is passed.
- `fopost connect-sessions create · get` mints a hosted connect link for one of your users and reads its status.
- `fopost webhooks deliveries list · get · redeliver` reads and replays a subscription's delivery log.
- `fopost automations versions list · get · restore` walks an automation's saved graphs and restores one as a new version.
- `fopost analytics inbox` prints the inbox roll-up.
- Requires `fopost-go` 0.4.0 (`Queue`, `ConnectSessions`, `Posts.Unpublish`, webhook deliveries, automation versions, `Analytics.Inbox`); the pin in `go.mod` must be re-pointed at the tag once it is released.

## 0.2.0 - 2026-10-18

### Removed

- **Breaking:** `fopost blog settings set --auto-publish` is gone, and `blog settings get` no longer shows auto publish; the API dropped the setting. `--review-hold-days` now sets how many days before its planned date a picked keyword starts being written.

### Added

- `fopost accounts reddit subreddits · rules · flairs · set-default` lists the subreddits an account can post to, a subreddit's rules and flairs, and sets the default subreddit; `r/webdev` and `webdev` both work, and `set-default --clear` falls back to the account's own profile page.
- `fopost blogs list · articles (list · get · create · update · delete) · products (list · update)` reads and edits the articles and products a connected site already owns, addressed by the site's own ids; `articles update` changes the live article in place, and `articles delete` confirms unless `--yes` is passed.
- `fopost blog keywords suggest` suggests search terms from the blog goal and topics (`--seed`, repeatable, and `--prompt`).
- `fopost blog articles submit` sends a draft, failed or rejected article for approval. `blog articles publish` and `edit --publish-at` need the article in review.
- `fopost blog settings set --goal --focus-topics --avoid-topics`; `settings get` shows the goal, the topics and the last plan run, and the keyword tables show the fit reason.
- `fopost blog` over the Blog service: `settings get · set · rotate-secret · test-website`, `keywords list · plan · add · update · delete` and `articles list · request · get · edit · regenerate · publish · reject · delete`. `settings set` prints the website secret once when the API issues it; `articles get --body` prints the markdown; `articles publish`, `rotate-secret` and the deletes confirm unless `--yes` is passed.
- `fopost media limits` prints the account's upload ceiling, the multipart per-file ceiling, the next tier and the allowed types.
- `fopost media upload` reads that ceiling and sends any file over it through direct upload, streamed from disk; `--direct` still forces the route for every file.
- `fopost posts create --file <path>` (repeatable, up to five) sends files with the post through the SDK, which picks multipart or direct upload by size.
- Requires `fopost-go` with `Media.Limits` and `PostFile` support (the pin in `go.mod` must be bumped once that SDK version is released).
