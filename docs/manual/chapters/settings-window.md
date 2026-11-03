---
title: Settings window and logs
summary: How the settings window loads, talks to Go, and frees its memory, and how logs are kept within fixed limits.
sources: ["apps/finicky/src/window/**", "apps/finicky/src/logger/**", "apps/finicky/src/assets/**", "apps/finicky/assets/**", "packages/finicky-ui/**"]
---
## Purpose

The settings window lets a person run Finicky without a config file: see its status, write and
test rules, change options and read logs. It is closed most of the time. When closed, it must
cost almost nothing.

### Pages

| Page (hash) | File | What a person does there |
| --- | --- | --- |
| Overview (`#overview`) | `packages/finicky-ui/src/pages/Overview.svelte` | Sees if Finicky is the default browser (and makes it so), the config state and errors, the first-run steps, and the recent links with "Make a rule". |
| Rules (`#rules`) | `packages/finicky-ui/src/pages/Rules.svelte` | Tests a URL (the winning rule is marked), edits rules ("When the URL matches … Open in …"), reorders them by drag or keyboard, sets "Otherwise open in". |
| Settings (`#settings`) | `packages/finicky-ui/src/pages/Settings.svelte` | Changes the options and opens or reveals the config file. |
| Logs (`#logs`) | `packages/finicky-ui/src/pages/LogViewer.svelte` | Reads, filters and copies the log. |
| About (`#about`) | `packages/finicky-ui/src/pages/About.svelte` | Version and credits. |

`lib/router.svelte.ts` keeps the page in `location.hash`; there is no routing library.

## How it works

1. **First open.** `window.ShowWindow` (`apps/finicky/src/window/window.go:200`) loads the
   embedded UI files into the Objective-C side one time (`loadAssets`, `window.go:147`). Before
   this change, every start loaded them, also when Finicky only opened a link.
2. **Web view.** `createWebView` (`window/window.m:95`) makes a WKWebView, which starts a WebKit
   content process. The page loads from the `finicky-assets://` scheme.
3. **Ready.** When the page finishes loading, `WindowIsReady` (`window.go:66`) sends what the
   page needs: the latest message of each state type, the log history, then the pending
   messages.
4. **Close.** `windowWillClose:` (`window.m:296`) calls `destroyWebView` (`window.m:139`). This
   ends the WebKit content process (about 25 MB, measured). `WindowDidClose` (`main.go:373`) calls
   `window.MarkClosed`, so new messages are buffered again.

### Messages from Go to the page

`SendMessageToWebView` (`window.go:105`) sends `{type, message}` JSON. It keeps:

| Kind | Types | Kept |
| --- | --- | --- |
| State | `config`, `version`, `updateInfo`, `buildInfo`, `recentLinks`, `defaultBrowserStatus` | The latest of each type, always. |
| Log | `log` | The last 1000 lines (`maxLogHistory`), always. |
| Other | `testUrlResult`, `rules`, … | Only while the page loads, at most 100 (`maxPending`). |

Messages from the page to Go arrive in `HandleWebViewMessage` (`window.go:217`). Work that uses
the VM goes to the event loop (see [URL pipeline](url-pipeline.md)).

### Logs

`logger.Setup` (`logger/logger.go:56`) writes JSON `slog` records to stdout, to the window and
to `memLog`. `memLog` (`logger.go:19`) keeps at most 256 KB, cut at line ends
(`logger/membuf`). When `logRequests` turns on, `SetupFile` (`logger.go:62`) opens one log file
for the process and copies `memLog` into it. When the option turns off, it closes the file.

## Rules and invariants

- Text tokens in `packages/finicky-ui/src/app.css` (`--text-*`, `--accent-color`) meet 4.5:1 on
  every background in light and dark mode. Do not lower text contrast with `opacity`.
- Every control has an accessible name and a visible `:focus-visible` ring. Rules move with the
  arrow keys on their grip, and a live region announces the new position.
- A save from a page sends the whole file it knows (`{...rulesFile, …}`); Go also keeps stored
  `options` when a save has none (`rules.MergeOptions`).
- Rules save 500 ms after the last edit, and a pending save runs when the page closes.
- `prefers-reduced-motion` stops transitions and toast motion.

- No buffer grows without a limit while the process runs: state messages (4), log history (1000
  lines), pending messages (100), `memLog` (256 KB).
- The Objective-C code uses manual retain and release (no ARC). Each `alloc` needs a `release`.
- When you change the Go↔UI messages, change `window/window.go` and
  `packages/finicky-ui/src/types.ts` together.

## Data and interfaces

- Go to page: `{"type": "<type>", "message": <value>}`, sent with `window.SendMessageToWebView`.
  The page receives it in `window.finicky.receiveMessage`.
- Page to Go: `{"type": "testUrl" | "getRules" | "saveRules" | "getInstalledBrowsers" |
  "getBrowserProfiles" | "makeDefaultBrowser" | "openConfig" | "revealConfig", …}`, handled in
  `HandleWebViewMessage`. `openConfig` and `revealConfig` take no path: Go opens its own config
  path (`openConfigFile`, `apps/finicky/src/main.go`).
- `recentLinks`: the last 30 links, newest first: `id`, `time`, `url`, `opener`, `browser`,
  `profile`, `source` (`rule`, `js`, `default`, `error`) and `ruleIndex`. The event loop keeps
  them in memory only (`recordRecentLink`, `main.go`); they are never written to disk.
- `defaultBrowserStatus`: `{isDefault}`. Sent at start, each time the window opens, and after
  `makeDefaultBrowser` (Go polls for 30 s while macOS shows its dialog).
- `testUrlResult` also has `source`, `ruleIndex` and `handlerIndex`.
- `browserProfiles`: `browser`, `profiles`, `supported` (the browser has profiles Finicky can
  select) and `blocked` (macOS refused the read). With `supported` and no list, the profile field
  is a text field for the folder name, with a hint when `blocked`.
- Log file: `~/Library/Logs/Finicky/Finicky_<time>.log`, mode `0600`, one file per process.

## Traps

> [!WARNING]
> The web view's message handler keeps a reference to the controller. `destroyWebView` must
> call `removeScriptMessageHandlerForName:` first, or the controller and the web view are never
> freed.

- A keyed `{#each}` needs a key that is unique. Recent links carry a Go `id`: two links can
  have the same URL in the same second, and a duplicate key stops the list from rendering.
- The native window and web view take the system window color (`window.m`), so a light-mode
  window does not flash dark before the page paints.
- `window` imports nothing from `main`, but its Objective-C code calls `WindowDidClose`, which
  only `main` defines. So a test binary that links `window` (for example a `logger` test) fails
  to link. Put testable code in a package without cgo, such as `logger/membuf`.

## How to check it

1. `cd apps/finicky/src && go test ./logger/membuf`
2. `cd packages/finicky-ui && npx svelte-check --tsconfig ./tsconfig.app.json`: 0 errors.
3. Open the window, close it, and count the processes named `com.apple.WebKit.WebContent`. One
   fewer runs after the close.
4. Open the window again: the log page shows the earlier lines.

## Sources

- `apps/finicky/src/window/window.go`, `apps/finicky/src/window/window.m`
- `apps/finicky/src/logger/logger.go`, `apps/finicky/src/logger/membuf/membuf.go`
