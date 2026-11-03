---
title: Automatic tests on each change
summary: Each change to the fork now builds the app and runs all tests on GitHub, and no step fails because of missing upstream secrets.
type: ci
---
## For everyone

Before, the fork had two automatic checks, but neither one worked. One watched a branch that
does not exist, and the other tried to sign the app with keys that only the original author has.
Now each change builds the app and runs every test on GitHub before it merges. The signing step
runs only in the original project. The owner must turn on the checks once in the GitHub settings
of the fork.

## In 30 seconds

- `macos-build.yml` now runs `go test ./...`, the config API tests and the UI type check after the build.
- `sign-and-notarize` runs only when `github.repository` is `johnste/finicky`.
- The workflow token is read-only, and each action is pinned to a commit SHA.
- `concurrency` stops an older run when a new commit arrives on the same branch.
- `nodejs.yml` is removed: it watched `master` and a `config-api` folder that no longer exist.

## Engineering decisions

| Decision | Reason | Not chosen |
| --- | --- | --- |
| Run the tests in the macOS build job, after the build. | The Go tests need cgo with Cocoa and the embedded build outputs. The build job already has both. | A separate Linux job: Cocoa is not available there. |
| Keep the signing job, guarded by the repository name. | Upstream merges stay easy, and upstream still signs its releases. | Delete the job: every upstream sync would conflict. |
| Pin to the newest major release of each action. | The v4 actions use a Node runtime that GitHub retires. | Keep v4 with a SHA pin. |

## What changed

| Where | What it does |
| --- | --- |
| `.github/workflows/macos-build.yml` | Tests after the build, read-only token, SHA pins, `concurrency`, signing only upstream. |
| `.github/workflows/nodejs.yml` | Removed. |
| `docs/manual/chapters/build-and-version.md` | New part "Continuous integration". |

## Check it yourself

1. Open the Actions tab of the fork. The "macOS Build" run of this PR shows a green check.
2. Open the run. The steps "Test Go code", "Test config API" and "Check UI types" pass.
3. The job `sign-and-notarize` shows as skipped.

## Limits and open items

- macOS runners are free only for a public repository. If the fork becomes private, each run costs minutes.
- The run builds the universal app, which takes some minutes. The tests do not run when the build fails.

## Words used here

- **Workflow**: a file that tells GitHub which steps to run after a change.
- **SHA pin**: a reference to one exact commit of an action, so that nobody can change the code that runs.
- **Notarize**: send the app to Apple for an automatic check, so that macOS opens it without a warning.
