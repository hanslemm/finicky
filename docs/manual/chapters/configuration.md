---
title: Configuration
summary: How Finicky finds, bundles, checks and reloads the user's config file and JSON rules, and which options it reads.
sources: ["apps/finicky/src/config/**", "apps/finicky/src/rules/**", "packages/config-api/src/configSchema.ts", "packages/config-api/scripts/**", "packages/config-api/package.json", "packages/config-api/tsconfig.json", "example-config/**", "testdata/**"]
---
## Purpose

The user writes rules in a JS or TS file, or in the rules editor of the settings window (a JSON
file). This part turns them into a checked config inside the goja VM, and loads it again when the
file changes.

## How it works

1. **Find the file.** `GetConfigPath` (`apps/finicky/src/config/configfiles.go:94`) uses the
   first file that exists: `~/.finicky.js`, `~/.finicky.ts`, `~/.config/finicky.js`,
   `~/.config/finicky.ts`, `~/.config/finicky/finicky.js`, `~/.config/finicky/finicky.ts`.
   `--config <path>` replaces the list.
2. **Use the cache.** `BundleConfig` (`configfiles.go:122`) returns the cached bundle in
   `~/Library/Caches/Finicky/` when the file did not change and the app version is the same
   (`config/cache.go`).
3. **Babel, only when needed.** `babelTransform` (`configfiles.go:186`) runs Babel only when the
   file has a named regex group, `(?<name>` (`namedGroupRe`, `configfiles.go:182`). goja parses
   named groups but does not fill in `match.groups`, so Babel's plugin is still needed for them.
   The transform cache is checked before Babel runs.
4. **Bundle.** esbuild bundles the file to one IIFE with the global name `finickyConfig`.
5. **Load and check.** `config.New` (`config/vm.go:56`) makes a new `goja.Runtime`, runs the
   embedded Config API, adds the `finicky.*` helpers and runs the bundle. `setup`
   (`vm.go:89`) checks the result with the zod schema (`configSchema.ts`). Then it reads the
   options one time (`readConfigOptions`, `vm.go:183`).
6. **Reload.** fsnotify watches the file. Events are debounced for 500 ms
   (`handleConfigFileEvent`, `configfiles.go:349`). A change builds a new VM; the old VM is not
   changed.

### JSON rules

The rules editor writes `~/Library/Application Support/Finicky/rules.json` (`rules.GetPath`,
`rules/rules.go:81`).

- No JS config: `rules.ToJSConfigScript` (`rules.go:191`) makes a JS config from the file, and
  the VM runs it.
- A JS config exists: the JSON rules come after the JS handlers (see
  [URL pipeline](url-pipeline.md), `ensureMergedConfig`).
- A save from the rules editor has no `options`. `rules.MergeOptions` (`rules.go:124`) keeps the
  stored options, so a rule edit does not erase the Preferences toggles.

### Options

| Option | Default | Effect |
| --- | --- | --- |
| `keepRunning` | `true` | Stay open after a URL. When false, the app exits 2 s after the last URL. |
| `hideIcon` | `false` | No menu bar icon. |
| `logRequests` | `false` | Also write logs to `~/Library/Logs/Finicky/`. |
| `checkForUpdates` | `true` | Ask the update server, at most once a day. |
| `resolveShortUrls` | `true` | Follow redirects of known shorteners before matching. Each short link then costs up to 750 ms and tells the shortener which link you opened. |

The defaults are in `defaultOptions` (`vm.go:38`). The JSON rules file has the same keys in
`options` (`rules.Options`).

## Rules and invariants

- A VM never changes its config. A new config means a new VM. So `GetAllConfigOptions`
  (`vm.go:176`) can return the copy that `setup` read.
- The Config API runs in goja with an ES2015 target and `core-js` URL polyfills. Do not use
  Node or browser APIs in `packages/config-api/src`.
- The Config API is a public contract: user configs call `finicky.*` and follow
  `configSchema.ts`. Keep changes backwards compatible.

## Data and interfaces

- `config.ConfigOptions` (`vm.go`): `KeepRunning`, `HideIcon`, `LogRequests`,
  `CheckForUpdates`, `ResolveShortURLs`.
- `rules.RulesFile` (`rules.go`): `defaultBrowser`, `defaultProfile`, `options`, `rules[]`
  (`match`, `browser`, `profile`).
- `npm run generate-types` makes `finicky.d.ts` from the schema. The build copies it into the app
  bundle.

## Traps

> [!WARNING]
> The named-group check also matches text in comments. That only costs one Babel run (about
> 0.5 s). A missed match would break `match.groups` in the user's config, so the check errs to
> the safe side.

- The option `urlShorteners` is in the schema, but nothing reads it.
- `setupVM` (`main.go:429`) turns file logging on when no config loads, to help troubleshooting.

## How to check it

1. `cd apps/finicky/src && go test ./config ./rules`
2. Edit `~/.finicky.js`: the log shows `Configuration file changed`, then `VM refresh complete`.
3. Without a named group, the log shows `Skipping babel transform: no named capturing groups`.

## Sources

- `apps/finicky/src/config/configfiles.go`, `apps/finicky/src/config/vm.go`,
  `apps/finicky/src/config/cache.go`
- `apps/finicky/src/rules/rules.go`
- `packages/config-api/src/configSchema.ts`
