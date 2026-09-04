---
description: Chart a GitLab user's contributions as a four-axis radar SVG or a JSON summary. Read this before answering questions about the activity command, what each axis counts, why a number differs from the GitLab profile, or how the chart is colored.
---

# activity

`activity` summarizes what a user did on GitLab over a period and writes the result as an SVG radar chart or as JSON.

It reads the user's events through the `glab` CLI, so it sees exactly what the logged-in account is allowed to see, and it never stores a token itself.

## Usage

The command takes an optional username and a handful of flags.

```sh
nyctereutes activity                       # the logged-in user, SVG to stdout
nyctereutes activity alice                 # another user
nyctereutes activity --json                # the JSON summary instead of the chart
nyctereutes activity --output activity.svg # write to a file instead of stdout
nyctereutes activity --since 2025-01-01 --until 2025-12-31
```

- Without a username, the command asks `glab` which account it is logged in as and reports on that account.
- `--since` and `--until` take a date as `YYYY-MM-DD` and are both inclusive, so `--until 2025-12-31` includes the last day of the year.
- `--since` defaults to twelve months before today and `--until` defaults to today, where today is the current date in UTC.
- `--json` writes the JSON summary in place of the chart.
- `--output <path>` writes whichever form was chosen to that file, creating or truncating it, instead of standard output.
- A malformed date, or a `--since` later than `--until`, is reported on standard error and the command exits with status 1 without calling `glab`.

## Axes

The chart has four axes, and each event a user produced is credited to at most one of them.

- Commits counts the commits the user pushed. Each push event contributes its commit count, a bulk push that reports only a ref count counts as one commit, and a push that deleted a branch counts nothing.
- Merge requests counts the merge requests the user opened. Merging or closing a merge request later is not counted again.
- Issues counts the issues the user opened.
- Code review counts the distinct merge requests the user approved or commented on. Several comments and an approval on the same merge request count as one, and comments on issues are not review.

Each axis is labeled with its share of the total as a whole-number percentage, so a chart reads "Code review 30%" rather than showing raw counts.
The JSON summary carries both the count and the percentage for every axis, together with the user, the period and the total.

## Retention

GitLab keeps user events for three years and discards older ones, so a period reaching further back is silently shorter than requested.
The events GitLab reports are also the events the logged-in account may see, so a chart of another user omits their work in projects the viewer cannot access.

## Colors

The SVG names no color of its own: every line and label is drawn with `currentColor`, and the polygon is filled with `currentColor` at reduced opacity.
When the SVG is inlined into a page, it takes the text color of the surrounding element, so it follows the page's theme.
When it is embedded as an image, for example through an `<img>` tag in a GitHub README, `currentColor` resolves to the browser's default text color, which is black, regardless of the page's theme.
