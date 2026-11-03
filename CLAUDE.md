# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

Finicky is a macOS app that registers as the default browser. For each URL it receives, it runs the
user's rules (a JS/TS config file and/or a JSON rules file) and opens the URL in the chosen browser
and profile. This repository is a fork of `johnste/finicky` (remote `upstream`); `origin` is
`hanslemm/finicky`.

## Commands

All scripts run from the repository root. They need Go 1.24+ (cgo, Xcode command-line tools), and
Node 22.

```sh
./scripts/install.sh          # npm ci in both packages, go mod tidy
./scripts/build.sh            # build config-api + UI, embed them, build the app
                              # local build: arm64 only, REPLACES /Applications/Finicky.app
BUILD_TARGET_ARCH=arm64 ./scripts/build.sh   # build into apps/finicky/build/, no install
brew upgrade --fetch-HEAD hanslemm/finicky/finicky   # rebuild the installed app from main (Formula/finicky.rb)
BUILD_UNIVERSAL=1 ./scripts/build.sh   # arm64 + amd64 + lipo (what CI does)
./scripts/test.sh             # go test ./... in apps/finicky/src

# Go tests (package-level / single test)
cd apps/finicky/src && go test ./resolver/
cd apps/finicky/src && go test ./rules/ -run TestToJSHandlers

# config-api (TypeScript, vitest)
cd packages/config-api && npx vitest run              # all
cd packages/config-api && npx vitest run src/wildcard.test.ts
cd packages/config-api && npm run check-types

# UI (Svelte 5 + Vite)
cd packages/finicky-ui && npm run dev     # browser preview without the Go host
cd packages/finicky-ui && npm run check

# Run the built binary against test scenarios (testdata/config.js, testdata/rules.json)
./scripts/dev.sh 2            # 0 normal, 1 none, 2 js, 3 json, 4 both
```

Binary flags: `--config <path>`, `--rules <path>`, `--no-config`, `--window`, `--dry-run` (resolve
but do not launch). `FINICKY_MOCK_UPDATE=1.99.0` fakes an update banner.

`scripts/build.sh` must run before `go build` works: `apps/finicky/src/assets/finickyConfigAPI.js`
and `assets/templates/` are build outputs (git-ignored) that the Go code embeds with `//go:embed`.

## Architecture

Read the manual first. Before you work in an area, read the frontmatter (`summary`, `sources`)
of the files in `docs/manual/chapters/` to find its chapter, and read that chapter.
`docs/manual/manual.json` gives the chapter order. Update the chapter in the same change as the
code.

| Area | Chapter |
|---|---|
| Apple event → rules → browser launch, event loop, short links | `docs/manual/chapters/url-pipeline.md` |
| Config files, Babel/esbuild, the goja VM, JSON rules, options | `docs/manual/chapters/configuration.md` |
| Settings window (WKWebView), Go↔UI messages, logging | `docs/manual/chapters/settings-window.md` |
| Build scripts, binary size, version, update check | `docs/manual/chapters/build-and-version.md` |

Three parts build into one Go binary: `apps/finicky/src` (Go + Objective-C via cgo),
`packages/config-api` (TypeScript, bundled to one IIFE and run inside goja), and
`packages/finicky-ui` (Svelte, shown in a WKWebView). The rule that matters most: only the
event-loop goroutine in `main.go` may use the goja runtime; send other work through the `jobs`
channel.

## Conventions

- The config API is a public contract: user configs call `finicky.*` helpers and use the schema in
  `configSchema.ts`. `npm run generate-types` builds `finicky.d.ts`, which ships in the app bundle.
  Keep changes backwards compatible, and update the tests in `packages/config-api/src/*.test.ts`.
- The config API runs in goja (ES2015 target, with `core-js` URL polyfills), not in Node. Do not use
  Node or browser APIs there.
- The version is in both `VERSION` and `apps/finicky/assets/Info.plist` (`CFBundleVersion`);
  a Go test fails when they differ.
- This fork is public. Keep personal data (home paths, emails, private app lists) out of
  commits, docs and test data.
- When you change the Go↔UI message protocol, change `window/window.go` and
  `packages/finicky-ui/src/types.ts` together.
- Upstream sync: `git fetch upstream && git merge upstream/main`. Keep fork changes in small,
  separate commits so upstream merges stay easy.
