# GH Notification manager for merged and closed PRs

## Why

This comes out desperation for a lack of filter in the [GitHub notifications UI](https://github.com/notifications).
In cases where a user is involved in several active repositories, managing notifications after a period of 
inactivity (e.g. coming back from vacation) is extremely painful (see [GitHub discussion](https://github.com/orgs/community/discussions/15591),
since the UI is currently lacking the ability to filter out merged or closed PRs.

## How does this tool workaround the issue?

This little Go script will go thought the list of unread notifications and mark as read PRs that are either merged or
closed. Then you can use the [Unread view](https://github.com/notifications?query=is%3Aunread) to help focus only on
actionable notifications.

Alternatively, you can even mark notifications as done via `--mark-done` rather than mark them as`read`.

To also mark notifications from specific authors (e.g. bots) regardless of PR state, use
`--mark-read-authors`, e.g. `./ghn --mark-read-authors renovate,dependabot`. The `[bot]` suffix is optional.

Note that this script doesn't do anything related to the unread status of **issues** and solely focuses on PRs.

## Prerequisites

1. A working Go environment


2. A valid `GITHUB_TOKEN` that has the following permissions:

    | Scope	         | Purpose                                      |
    |----------------|----------------------------------------------|
    | notifications	 | To read and mark notifications as read       |
    | repo           | To access PRs from private repos (if needed) |

## How to run

Build it (needed only once) with:
```
make build
```

and then, with a `GITHUB_TOKEN` env var present, run it using:

`./ghn`

Example output will look like:

```
Fetching all GitHub notifications
📦 Page 2 — Total fetched: 73 notifications
✅ Done! Fetched 73 notifications in total.
PR: https://github.com/testorg/testrepo/pull/1234, Title: "PR description", is unmerged and waiting for your review!
PR: https://github.com/testorg/anotherrepo/pull/203, Title: "Another PR description", is unmerged and waiting for your review!
🟡 PR: https://github.com/testorg/anotherrepo/pull/204, Title: "Some other PR description" is merged or closed and notification will be marked as read
  🟡 About to mark GH Notification with threadID: "..." as *READ*
    🟢 Successfully marked thread as read
...
```

For each notification change you'll be prompted for y/n, but you can skip by `./ghn --no-prompt` (assumes yes).

## Installation

You can also using `make install` which will install the tool (by default) under `/usr/local/bin/ghn`.
The destination path can be modified via the env var `DESTINATION_PATH`.

### gh extension

```shell
gh extension install dliappis/gh-ghn
```

### Upgrade

```shell
gh extension upgrade ghn
```

## Release

Please follow the process in [RELEASE.md](RELEASE.md) to create a new release of the extension.