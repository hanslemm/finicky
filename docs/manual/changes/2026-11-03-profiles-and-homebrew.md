---
title: Fast profile links, and Homebrew from git
summary: Links that open in a browser profile are now fast too, profiles work again when macOS hides the profile list, and Homebrew installs and updates this version from its source.
type: feat
pr: https://github.com/hanslemm/finicky/pull/2
---
## For everyone

Links that open in a browser profile, for example a work profile in Chrome, now get the same
speed-up as other links. A newer macOS stops apps from reading other apps' files, so Finicky
could not see Chrome's profile list; you can now type the profile's folder name, and the window
tells you where to find it. Homebrew can now install this version and update it from its source
code with one command. You do not have to change your config.

## In 30 seconds

- `nativePlan` sends custom args of the form `[-n] [-g] --args …` and plain app args through `NSWorkspace`; only other `open` flags still use `/usr/bin/open`.
- When macOS blocks `Local State`, a Chromium profile like `Profile 15` goes to `--profile-directory` as it is; display names do not, so Chrome never makes an empty profile.
- The window gets `supported` and `blocked` for each browser and shows a typed profile field with a hint.
- `Formula/finicky.rb` makes the repository a Homebrew tap that builds from `main`.
- `--config <file>` no longer also opens the file as a link (old upstream bug).

## The numbers

Measured on an Apple Silicon Mac on 2026-11-03. The open step is the median of 20 runs that
sent a URL to an app that was already open; a profile link now uses the same API, but its own
time was not measured. Build sizes come from `ls`.

```chart
{ "type": "bar", "unit": "ms", "rows": [
    { "label": "Open step, /usr/bin/open (custom args before)", "value": 41.8 },
    { "label": "Open step, NSWorkspace (custom args now)", "value": 4.8, "highlight": true } ] }
```

| Measure | Before | After |
| --- | --- | --- |
| Universal binary (arm64 + amd64) | 55 MB (4.2.2) | 43.9 MB |
| Stray "links" when started with `--config` and `--rules` | 2 | 0 |

## Engineering decisions

| Decision | Reason | Not chosen |
| --- | --- | --- |
| Map only `-n` and `-g` before `--args` to native options. | They have exact `NSWorkspace` equals. Other `open` flags (`-F`, `-W`, …) keep their exact `open` behaviour. | Map every `open` flag: some have no native equal. |
| Pass a profile through only when it looks like a folder name. | Chrome makes a new, empty profile for an unknown `--profile-directory`. A folder name is what the user sees in `chrome://version`. | Pass any name: wrong names make stray profiles. |
| Formula in the fork repository, `head` only. | One repository, and Homebrew follows `main` with `brew upgrade --fetch-HEAD`. | A separate `homebrew-tap` repository with release builds: needs signed releases and CI. |
| `/Applications/Finicky.app` is a link to `opt/finicky`. | The link stays valid across upgrades. | Copy the app: Homebrew would not manage the copy. |

## What changed

| Where | What it does |
| --- | --- |
| `apps/finicky/src/browser/launcher.go` | `nativePlan`; blocked-list fallback; `GetProfileInfo` with `supported` and `blocked`. |
| `apps/finicky/src/window/window.go` | `browserProfiles` sends `supported` and `blocked`. |
| `apps/finicky/src/main.m` | `isLaunchArgument` skips flag values in `openFile`. |
| `packages/finicky-ui/src/components/BrowserProfileSelector.svelte` | Typed profile field and hint when the list is blocked. |
| `Formula/finicky.rb` | Homebrew formula that builds from `main`. |
| `scripts/build.sh` | Works without `.git`; takes `COMMIT_HASH`. |

## Check it yourself

1. Run `cd apps/finicky/src && go test ./browser`. The plan, folder-name and block tests pass.
2. Run `brew install --HEAD hanslemm/finicky/finicky`. Homebrew builds the app from `main`.
3. Click a link that your config sends to a Chrome profile. The log shows `Opened natively`.

## Limits and open items

- A display name (for example "Work") still cannot select a Chrome profile while macOS blocks the list. Use the folder name, or allow Finicky in System Settings > Privacy & Security.
- Each Homebrew build has a new ad hoc signature, so macOS can ask again for a permission that you gave to an earlier build.

## Words used here

- **Profile folder**: the folder of one browser profile, for example `Profile 15`; `chrome://version` shows it as "Profile Path".
- **Tap**: a git repository that Homebrew reads formulas from.
- **Head-only formula**: a formula that builds from the newest commit of a branch, not from a release.
- **Ad hoc signature**: a code signature with no developer certificate; it changes with each build.
