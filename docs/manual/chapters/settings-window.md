---
title: Settings window and logs
summary: How the settings window loads, talks to Go, and frees its memory, and how logs are kept within fixed limits.
sources: ["apps/finicky/src/window/**", "apps/finicky/src/logger/**", "apps/finicky/src/assets/**", "apps/finicky/assets/**", "packages/finicky-ui/**"]
---
## Purpose

The settings window shows the start page, the rules editor, the URL tester and the logs. It is
closed most of the time. When closed, it must cost almost nothing.

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
| State | `config`, `version`, `updateInfo`, `buildInfo` | The latest of each type, always. |
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

- No buffer grows without a limit while the process runs: state messages (4), log history (1000
  lines), pending messages (100), `memLog` (256 KB).
- The Objective-C code uses manual retain and release (no ARC). Each `alloc` needs a `release`.
- When you change the Go↔UI messages, change `window/window.go` and
  `packages/finicky-ui/src/types.ts` together.

## Data and interfaces

- Go to page: `{"type": "<type>", "message": <value>}`, sent with `window.SendMessageToWebView`.
  The page receives it in `window.finicky.receiveMessage`.
- Page to Go: `{"type": "testUrl" | "getRules" | "saveRules" | "getInstalledBrowsers" |
  "getBrowserProfiles", …}`, handled in `HandleWebViewMessage`.
- Log file: `~/Library/Logs/Finicky/Finicky_<time>.log`, mode `0600`, one file per process.

## Traps

> [!WARNING]
> The web view's message handler keeps a reference to the controller. `destroyWebView` must
> call `removeScriptMessageHandlerForName:` first, or the controller and the web view are never
> freed.

- `window` imports nothing from `main`, but its Objective-C code calls `WindowDidClose`, which
  only `main` defines. So a test binary that links `window` (for example a `logger` test) fails
  to link. Put testable code in a package without cgo, such as `logger/membuf`.

## How to check it

1. `cd apps/finicky/src && go test ./logger/membuf`
2. Open the window, close it, and count the processes named `com.apple.WebKit.WebContent`. One
   fewer runs after the close.
3. Open the window again: the log page shows the earlier lines.

## Sources

- `apps/finicky/src/window/window.go`, `apps/finicky/src/window/window.m`
- `apps/finicky/src/logger/logger.go`, `apps/finicky/src/logger/membuf/membuf.go`
