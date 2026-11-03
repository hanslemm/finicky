---
title: Faster links, less memory
summary: Links now open faster, and Finicky uses less memory while it waits.
type: perf
pr: https://github.com/hanslemm/finicky/pull/1
---
## For everyone

Each link that you click now opens faster, because Finicky no longer starts a helper program for
each link. When you close the settings window, Finicky now frees the memory
that the window used. Memory also stays the same over days of use, because old messages are no
longer kept. A rule edit no longer erases your Preferences settings. You do not have to do
anything.

## In 30 seconds

- `LaunchBrowser` asks LaunchServices through `NSWorkspace`; `/usr/bin/open` is only a fallback.
- The config is checked once per load, not once per URL; wildcard regexes are compiled once.
- Only the event-loop goroutine uses the goja VM; UI work goes through a `jobs` channel.
- The WKWebView is destroyed on window close, and every log and message buffer has a size limit.
- Babel runs only for configs with named regex groups; the binary is built with `-s -w`.

## The numbers

Measured on an Apple Silicon Mac, arm64 build, on 2026-11-03. The open-step rows are the median
of 20 runs that sent the same URL to the same app, which was already open. The JS-step rows come
from a Go benchmark with the same config before and after.

```chart
{ "type": "bar", "unit": "ms", "rows": [
    { "label": "Open step, /usr/bin/open", "value": 41.8 },
    { "label": "Open step, NSWorkspace by name", "value": 10.8, "highlight": true },
    { "label": "Open step, NSWorkspace by bundle id", "value": 4.8, "highlight": true },
    { "label": "JS step, before", "value": 6.17 },
    { "label": "JS step, after", "value": 0.47, "highlight": true } ] }
```

| Measure | Before | After |
| --- | --- | --- |
| Open step, same app, median of 20 | 41.8 ms | 4.8 ms (bundle id), 10.8 ms (app name) |
| JS evaluation, 20 handlers and 1 JSON rule | 6.17 ms | 0.47 ms |
| Config reload with Babel | 548 ms | Babel skipped for configs with no named groups |
| arm64 binary | 29.9 MB | 21.8 MB |
| WebKit process after the window closes | stays (about 25 MB) | ends |

The new build logged 8–13 ms for a whole URL (rules and open) with a small test config. The
installed old build logged 76–109 ms, but with a different config and a real browser, so the two
ranges do not compare directly.

## How it fits

Most of the changes are on the path of one URL.

```diagram
{ "nodes": [
    { "id": "ev", "label": "Apple event", "sub": "main.m, 100 ms AX limit", "x": 0, "y": 0, "kind": "changed" },
    { "id": "loop", "label": "Event loop", "sub": "main.go, owns the VM", "x": 1, "y": 0, "kind": "changed" },
    { "id": "short", "label": "Short links", "sub": "shorturl, 750 ms total", "x": 1, "y": 1, "kind": "changed" },
    { "id": "js", "label": "openUrl", "sub": "precompiled, prevalidated", "x": 2, "y": 0, "kind": "changed" },
    { "id": "nat", "label": "openNative", "sub": "NSWorkspace", "x": 3, "y": 0, "kind": "new" },
    { "id": "ui", "label": "Settings window", "sub": "web view freed on close", "x": 0, "y": 1, "kind": "changed" } ],
  "edges": [
    { "from": "ev", "to": "loop", "label": "sends" },
    { "from": "loop", "to": "short", "label": "asks" },
    { "from": "loop", "to": "js", "label": "runs" },
    { "from": "loop", "to": "nat", "label": "opens" },
    { "from": "ui", "to": "loop", "label": "queues jobs" } ],
  "steps": [
    { "title": "A link arrives", "text": "main.m reads the link and the app that sent it. The window title lookup now stops after 100 ms, so a frozen app cannot delay the link.", "on": ["ev"] },
    { "title": "The event loop takes it", "text": "The link goes on a channel with room for 16. The event loop is now the only code that uses the JavaScript VM.", "on": ["ev>loop", "loop"] },
    { "title": "Short links, with one time limit", "text": "Only for a known shortener. One 750 ms limit covers the whole lookup. The option resolveShortUrls can turn it off.", "on": ["loop>short", "short"] },
    { "title": "The rules run", "text": "A compiled openUrl call runs. It does not check the whole config again, and it reuses compiled patterns.", "on": ["loop>js", "js"] },
    { "title": "The browser opens", "text": "NSWorkspace sends the link to the browser from inside Finicky. No helper program starts.", "on": ["loop>nat", "nat"] },
    { "title": "The window sends jobs", "text": "Test URL and Save rules go to the event loop as jobs. Closing the window frees its web view.", "on": ["ui", "ui>loop"] } ] }
```

## Engineering decisions

| Decision | Reason | Not chosen |
| --- | --- | --- |
| Open through `NSWorkspace`, and keep `/usr/bin/open` as a fallback. | 4 to 9 times faster per URL. The fallback keeps old behaviour for custom `args` (they can hold `open` flags) and for apps that the native lookup cannot find. | Remove `open` fully: it would break configs with `open` flags in `args`. |
| Keep Babel, but run it only when the config has `(?<name>`. | A test showed that goja parses named groups but leaves `match.groups` empty. Most configs have none, so most reloads skip about 0.5 s. | Remove goja-babel: configs with named groups would silently break. |
| `resolveShortUrls` defaults to `true`. | Existing configs that match the long URL of a short link keep working. | Default `false`: faster and more private, but it changes what existing rules match. |
| One event loop owns the VM; the UI sends closures on a `jobs` channel. | goja is not safe for concurrent use. Before, the URL tester, rule saves and the update check used the VM from other goroutines. | A mutex around the VM: it still lets a slow job block the main thread. |
| Keep the last message of each state type, plus 1000 log lines, for the window. | A new web view must show the current state and recent logs, with a fixed memory cost. | Keep everything (the old way): memory grew for the life of the process. |
| Merge the missing `options` from disk when the rules editor saves. | The editor sends rules only, so each rule edit erased the Preferences toggles. | Fix only the UI payload: an older or other client could still erase them. |

## What changed

| Where | What it does |
| --- | --- |
| `apps/finicky/src/browser/open_native.go` | New. Opens URLs and apps through `NSWorkspace` and waits for the result. |
| `apps/finicky/src/browser/launcher.go` | Uses the native path first. Parses `browsers.json` once. |
| `apps/finicky/src/main.go` | Event loop owns the VM; `jobs` channel; buffered URL channel; no update check at exit. |
| `apps/finicky/src/main.m` | 100 ms limit on the Accessibility window-title lookup. |
| `apps/finicky/src/resolver/resolver.go` | Precompiled scripts; merged JS and JSON config built once per rules change. |
| `apps/finicky/src/shorturl/resolver.go` | Exact host match, one 750 ms limit, shared connections, bodies closed. |
| `apps/finicky/src/config/vm.go`, `configfiles.go` | Options read once; `resolveShortUrls`; Babel only for named groups, cache checked first. |
| `apps/finicky/src/window/window.go`, `window.m` | UI files loaded on first open; web view freed on close; bounded message buffers. |
| `apps/finicky/src/logger/`, `logger/membuf/` | 256 KB start-up buffer; one log file per process, mode `0600`; option off closes it. |
| `apps/finicky/src/version/version.go` | Version read once from Info.plist with no child process; update check takes a flag, not the VM. |
| `apps/finicky/src/rules/rules.go`, `window/window.go` | `MergeOptions` keeps stored options on a rules save. |
| `packages/config-api/src/` | `openUrl(…, prevalidated)`, wildcard regex cache, `resolveShortUrls` in the schema. |
| `scripts/build.sh` | `-trimpath -ldflags "-s -w …"`. |
| `VERSION`, `docs/manual/`, `CLAUDE.md`, `AGENTS.md` | Version file for the manual, the new manual, and agent guides. |

## Check it yourself

1. Run `cd apps/finicky/src && go test ./...`. All packages pass.
2. Run `cd packages/config-api && npx vitest run`. 80 tests pass.
3. Run `BUILD_TARGET_ARCH=arm64 ./scripts/build.sh`. The binary is about 22 MB.
4. Install the build and click a link. The log shows `Opened natively` and a time near 10 ms.
5. Open the settings window, then close it. One `com.apple.WebKit.WebContent` process ends.

## Limits and open items

- A browser profile (Chrome or Firefox) through the native path was not tested by hand. It uses
  the same new-instance and arguments as `open -n --args`. The fallback still covers apps that
  the native lookup cannot find.
- The universal (arm64 and amd64) build was not run; only arm64 was built and measured.
- The settings window switch for `resolveShortUrls` and the design-review fixes come in a
  separate delivery: "A settings window that runs Finicky for you".

## Words used here

- **LaunchServices**: the macOS service that finds apps and opens links in them.
- **NSWorkspace**: the macOS programming interface that asks LaunchServices to open things.
- **goja**: the JavaScript engine, written in Go, that runs the user's rules.
- **Event loop**: the one Go routine that handles links, config reloads and window events in order.
- **WKWebView**: the macOS web view that shows the settings window. It runs a separate process.
- **Babel**: a JavaScript compiler. Finicky uses it only for named regex groups.
