---
title: URL pipeline
summary: How a URL goes from the macOS Apple event, through the rules, to an open browser, and how fast each step is.
sources: ["apps/finicky/src/main.go", "apps/finicky/src/main.m", "apps/finicky/src/main.h", "apps/finicky/src/resolver/**", "apps/finicky/src/shorturl/**", "apps/finicky/src/browser/**", "apps/finicky/src/browser.go", "apps/finicky/src/browser.h", "apps/finicky/src/browser.m", "apps/finicky/src/util/**", "packages/config-api/src/index.ts", "packages/config-api/src/wildcard.ts", "packages/config-api/src/FinickyURL.ts", "packages/config-api/src/legacyURLObject.ts", "packages/config-api/src/utilities.ts", "packages/config-api/src/*.test.ts"]
---
## Purpose

This part gets each URL that macOS sends to Finicky, finds the browser for it, and opens it. A
person waits for this on each click. It must be fast, and it must never stop on a slow app or
network.

## How it works

1. **Get the URL.** `handleGetURLEvent` (`apps/finicky/src/main.m:265`) reads the URL and the
   process that sent it. It asks the Accessibility API for the title of that app's focused window,
   with a 100 ms limit (`AXUIElementSetMessagingTimeout`, `main.m:300`). Then it calls `HandleURL`
   (`main.go:259`).
2. **Queue it.** `HandleURL` decodes `finicky://open/<base64>` links and sends a `URLInfo` on
   `urlListener` (`main.go:55`). The channel holds 16 items, so the Cocoa main thread does not
   wait while an earlier URL resolves.
3. **Resolve it on the event loop.** The goroutine in `main()` receives the URL
   (`main.go:178`) and calls `resolver.ResolveURL` (`resolver/resolver.go:94`).
   - When `options.resolveShortUrls` is true (the default), `shorturl.ResolveURL`
     (`shorturl/resolver.go:63`) follows the redirects of a known shortener. One 750 ms limit
     (`totalTimeout`) covers the HEAD request, the GET fallback and all redirects.
   - `evaluateURL` (`resolver.go:113`) runs a precompiled `openUrl` call (`openURLProgram`,
     `resolver.go:37`) in the VM. It passes `prevalidated = true`, because the VM checked the
     config when it loaded it (see [Configuration](configuration.md)).
   - With a JS config, the JSON rules are added after the JS handlers. `ensureMergedConfig`
     (`resolver.go:69`) builds and checks this merged config once for each rules change, not for
     each URL.
4. **Match.** `openUrl` (`packages/config-api/src/index.ts:89`) runs the rewrites in order, then
   the first matching handler gives the browser, else `defaultBrowser`. `matchWildcard`
   (`wildcard.ts:7`) keeps each compiled pattern in a map, with a cap of 1000 patterns.
5. **Open.** `browser.LaunchBrowser` (`browser/launcher.go:45`) calls `openNative`
   (`browser/open_native.go:107`). It asks LaunchServices through `NSWorkspace`:
   - no profile: `openURLs:withApplicationAtURL:configuration:`;
   - a profile: `openApplicationAtURL:configuration:` with a new instance and the arguments
     `<profile flag> <url>`, the same as `open -n -a App --args …`.
   It waits up to 10 s for LaunchServices to finish.
6. **Record.** `recordRecentLink` (`main.go`) adds the link, the opener and the result to the
   recent-links list of the settings window (see [Settings window and logs](settings-window.md)).

`openUrl` returns `matched: {handlerIndex, ruleIndex}` when a handler matched. `ruleIndex` exists
only for handlers made from the JSON rules file, so the window can point at the rule.
`browser.MatchInfo` carries it to Go; it is `nil` when the default browser was used.

### Profiles

`findBrowser` (`browser/discover.go`) gives the profile store of a browser: from
`browsers.json` first, else by discovery. Discovery looks in `~/Library/Application Support`
for a folder named after the app, the app without " Browser", or the last part of its bundle id,
one or two levels deep (`Vendor/App`). A folder with `Local State` is Chromium-based; one with
`profiles.ini` is Firefox-based. Results, also "not found", are cached for the process. Safari
profiles are not supported: Safari has no command-line switch to pick one.

```diagram
{ "nodes": [
    { "id": "ev", "label": "handleGetURLEvent", "sub": "main.m", "x": 0, "y": 0, "kind": "changed" },
    { "id": "ch", "label": "urlListener", "sub": "buffered, 16", "x": 1, "y": 0, "kind": "changed" },
    { "id": "res", "label": "ResolveURL", "sub": "resolver.go", "x": 2, "y": 0, "kind": "changed" },
    { "id": "short", "label": "shorturl", "sub": "750 ms total", "x": 2, "y": 1, "kind": "changed" },
    { "id": "js", "label": "openUrl", "sub": "precompiled, prevalidated", "x": 3, "y": 0, "kind": "changed" },
    { "id": "nat", "label": "openNative", "sub": "NSWorkspace", "x": 4, "y": 0, "kind": "new" },
    { "id": "open", "label": "/usr/bin/open", "sub": "fallback only", "x": 4, "y": 1, "kind": "same" } ],
  "edges": [
    { "from": "ev", "to": "ch", "label": "HandleURL" },
    { "from": "ch", "to": "res", "label": "event loop" },
    { "from": "res", "to": "short", "label": "if shortener" },
    { "from": "res", "to": "js", "label": "RunProgram" },
    { "from": "res", "to": "nat", "label": "LaunchBrowser" },
    { "from": "nat", "to": "open", "label": "unresolved / custom args" } ],
  "steps": [
    { "title": "Apple event", "text": "main.m reads the URL and the source app. The window title lookup stops after 100 ms.", "on": ["ev"] },
    { "title": "Buffered hand-off", "text": "HandleURL puts the URL on a channel with room for 16. The main thread continues at once.", "on": ["ev>ch", "ch"] },
    { "title": "Short links", "text": "Only for a known shortener host, and only when resolveShortUrls is on. One 750 ms limit for the whole lookup.", "on": ["ch>res", "res", "res>short", "short"] },
    { "title": "Rules", "text": "A compiled openUrl call runs in the VM. It does not check the config again.", "on": ["res>js", "js"] },
    { "title": "Open", "text": "NSWorkspace opens the URL in the process. /usr/bin/open is used only when the native path cannot resolve the app or the URL, or when the handler has custom args.", "on": ["res>nat", "nat", "nat>open", "open"] } ] }
```

### Measured cost

Measured on an Apple Silicon Mac, arm64 build, 2026-11-03. The test app was Calculator, which
was already open and ignores URLs, so the numbers show Finicky's cost and the LaunchServices
hand-off.

Median time to hand a URL to an open app, 20 runs each:

```chart
{ "type": "bar", "unit": "ms", "rows": [
    { "label": "/usr/bin/open", "value": 41.8 },
    { "label": "NSWorkspace, by name", "value": 10.8 },
    { "label": "NSWorkspace, by bundle id", "value": 4.8 } ] }
```

| Step | Before | After |
| --- | --- | --- |
| JS evaluation of one URL, 20 handlers and 1 JSON rule (Go benchmark) | 6.17 ms | 0.47 ms |

The app logs the whole time of one URL as "Time taken evaluating URL and opening browser". With a
small test config and an app that was already open, it was 8–13 ms.

## Rules and invariants

> [!RULE]
> Only the event-loop goroutine uses the goja runtime. goja is not safe for concurrent use. UI
> work goes through the `jobs` channel (`main.go:61`): `TestUrlHandler`, `SaveRulesHandler` and
> the exported `TestURL`.

- `checkForUpdates` (`main.go:388`) runs on its own goroutine. It gets the `checkForUpdates`
  option as an argument and never reads `vm`.
- `openUrl` may skip the config check only when the host passes `prevalidated = true` for a
  config object that it checked before.
- A host is a shortener only when it equals a listed domain or ends with `.` plus that domain
  (`isShortenerHost`, `shorturl/resolver.go:48`). `notbit.ly` is not `bit.ly`.
- With no VM (no config), URLs open in Safari (`defaultBrowserConfig`, `resolver.go:184`).

## Data and interfaces

- `browser.BrowserConfig` (`browser/launcher.go`): `name`, `appType` (`appName`, `bundleId`,
  `path`, `none`), `openInBackground`, `profile`, `args`, `url`.
- `browser/browsers.json`: known browsers with their type (`Chromium`, `Firefox`) and profile
  folder. It is parsed once (`knownBrowsers`, `launcher.go:179`).
- `shorturl/shortener_domains.json`: the shortener hosts.

## Traps

> [!WARNING]
> Handler `args` can hold `open` flags (for example `--args`). Only `/usr/bin/open` understands
> them, so a handler with custom args always uses the `open` fallback.

- `NSWorkspace fullPathForApplication:` is deprecated but is the only API that finds an app by
  its display name, the same as `open -a`. When it finds nothing, the code falls back to `open`.
- The completion block of `openNative` can run after the 10 s wait ends. The code then leaks the
  semaphore on purpose, so the late block cannot use a freed object (no ARC in this project).

## How to check it

1. `cd apps/finicky/src && go test ./resolver ./shorturl ./browser`
2. `cd packages/config-api && npx vitest run`
3. Run a build with `--dry-run` and open a URL: the log shows `Would open natively (dry run)`.
4. Without `--dry-run`, the log shows `Opened natively` and a time of about 10 ms.

## Sources

- `apps/finicky/src/main.go`, `apps/finicky/src/main.m`
- `apps/finicky/src/resolver/resolver.go`, `apps/finicky/src/shorturl/resolver.go`
- `apps/finicky/src/browser/launcher.go`, `apps/finicky/src/browser/open_native.go`
- `packages/config-api/src/index.ts`, `packages/config-api/src/wildcard.ts`
