---
title: Build and version
summary: How the scripts build the app, why the binary has its size, and where the version comes from.
sources: ["scripts/**", ".github/**", "apps/finicky/src/version/**", "apps/finicky/src/go.mod", "apps/finicky/src/go.sum", "apps/finicky/.gitignore", ".gitignore", ".env.example", ".vscode/**", "apps/browser-addon/**"]
---
## Purpose

This part builds one macOS app bundle from the Go code and the two TypeScript packages, and
gives the app its version.

## How it works

`scripts/build.sh` does these steps:

1. Builds `packages/config-api` with esbuild and copies `finickyConfigAPI.js` to
   `apps/finicky/src/assets/`.
2. Builds `packages/finicky-ui` with Vite and copies `dist/*` to
   `apps/finicky/src/assets/templates/`.
3. Runs `go build -trimpath -ldflags "-s -w -X …"`. `-s -w` removes the symbol table and the
   debug data. `-trimpath` removes local folder paths from the binary.
4. Copies `apps/finicky/assets/` (Info.plist, icons) into the bundle.

| Mode | Command | Result |
| --- | --- | --- |
| Local | `./scripts/build.sh` | arm64 only. It **replaces** `/Applications/Finicky.app`. |
| One arch, no install | `BUILD_TARGET_ARCH=arm64 ./scripts/build.sh` | `apps/finicky/build/Finicky-arm64.app` |
| Universal (CI) | `BUILD_UNIVERSAL=1 ./scripts/build.sh` | arm64 and amd64, joined with `lipo`. |

### Binary size

| Build (arm64) | Size |
| --- | --- |
| Before: no `-s -w` | 29.9 MB |
| After: `-s -w -trimpath` | 21.8 MB |

The largest parts are goja, esbuild's Go API and goja-babel (it embeds `babel.js`, 3 MB).

### Version

- `VERSION` at the root and `CFBundleVersion` in `apps/finicky/assets/Info.plist` hold the same
  value. `TestVersionFileMatchesInfoPlist` fails when they differ.
- `version.GetCurrentVersion` (`apps/finicky/src/version/version.go:109`) reads Info.plist one
  time per process. It reads the XML directly and uses `defaults read` only when that fails.
- The update check (`checkForUpdates`, `version.go:150`) asks `API_HOST` at most once a day.
  `API_HOST` comes from `.env` at build time. Without it, there is no update check.

## Rules and invariants

- `apps/finicky/src/assets/finickyConfigAPI.js` and `assets/templates/` are build outputs (git
  ignores them). Run the first two build steps before `go build` or `go test`.
- Change `VERSION` and Info.plist together.

## Data and interfaces

- Build-time values, set with `-ldflags -X`: `finicky/version.commitHash`,
  `finicky/version.buildDate`, `finicky/version.apiHost`.
- `.env` (git-ignored): `API_HOST=<update server>`. See `.env.example`.
- Update check cache: `~/Library/Caches/Finicky/last_update_check.json`.

## Traps

> [!WARNING]
> The local build mode deletes `/Applications/Finicky.app` and copies the new build there. Use
> `BUILD_TARGET_ARCH=arm64` to build without an install.

## How to check it

1. `BUILD_TARGET_ARCH=arm64 ./scripts/build.sh`
2. `ls -l apps/finicky/build/Finicky-arm64.app/Contents/MacOS/Finicky`: about 22 MB.
3. `cd apps/finicky/src && go test ./version`

## Sources

- `scripts/build.sh`, `scripts/install.sh`, `scripts/test.sh`, `scripts/dev.sh`
- `apps/finicky/src/version/version.go`
- `.github/workflows/macos-build.yml`
