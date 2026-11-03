---
title: Overview
summary: The parts of Finicky, how a URL moves through them, and the chapter list.
sources: ["README.md", "CLAUDE.md", "AGENTS.md", "VERSION", "docs/manual/manual.json"]
---
## System map

Finicky is a macOS app that is the default browser. For each URL, it runs the user's rules and
opens the URL in the browser (and profile) that the rules choose.

Three parts make one binary:

| Part | Path | Language | Role |
| --- | --- | --- | --- |
| Native app | `apps/finicky/src` | Go and Objective-C (cgo) | Gets URLs from macOS, runs the rules, opens the browser, shows the window. |
| Config API | `packages/config-api` | TypeScript | Matching, rewrites and config checks. It runs inside the goja JavaScript VM. |
| Settings UI | `packages/finicky-ui` | Svelte | The settings window: start page, rules editor, URL tester, logs. |

`apps/browser-addon` is a separate browser extension. It sends links as `finicky://open/<base64>`.

```diagram
{ "nodes": [
    { "id": "mac", "label": "macOS", "sub": "Apple event kAEGetURL", "x": 0, "y": 0, "kind": "same" },
    { "id": "objc", "label": "main.m", "sub": "handleGetURLEvent", "x": 1, "y": 0, "kind": "same" },
    { "id": "loop", "label": "Event loop", "sub": "main.go", "x": 2, "y": 0, "kind": "same" },
    { "id": "js", "label": "Config API", "sub": "goja VM", "x": 3, "y": 0, "kind": "same" },
    { "id": "ls", "label": "LaunchServices", "sub": "NSWorkspace", "x": 4, "y": 0, "kind": "same" },
    { "id": "cfg", "label": "Config file", "sub": "~/.finicky.js", "x": 3, "y": 1, "kind": "same" },
    { "id": "ui", "label": "Settings window", "sub": "WKWebView", "x": 2, "y": 1, "kind": "same" } ],
  "edges": [
    { "from": "mac", "to": "objc", "label": "URL" },
    { "from": "objc", "to": "loop", "label": "HandleURL" },
    { "from": "loop", "to": "js", "label": "openUrl" },
    { "from": "loop", "to": "ls", "label": "open" },
    { "from": "cfg", "to": "js", "label": "bundle" },
    { "from": "ui", "to": "loop", "label": "jobs" } ],
  "steps": [
    { "title": "macOS sends the URL", "text": "Finicky is the default browser, so macOS sends each clicked link to it as an Apple event.", "on": ["mac", "mac>objc", "objc"] },
    { "title": "The event loop gets it", "text": "main.m reads the source app and calls the Go function HandleURL. It puts the URL on a channel.", "on": ["objc>loop", "loop"] },
    { "title": "The rules decide", "text": "The event loop runs the Config API in the JavaScript VM. The first matching handler gives the browser.", "on": ["loop>js", "js", "cfg>js", "cfg"] },
    { "title": "The browser opens", "text": "The event loop asks LaunchServices through NSWorkspace to open the URL in that browser.", "on": ["loop>ls", "ls"] },
    { "title": "The window uses the same loop", "text": "The settings window sends its work (test a URL, save rules) to the event loop. Only the loop uses the VM.", "on": ["ui", "ui>loop", "loop"] } ] }
```

The version of the app is in `VERSION`. It must be equal to `CFBundleVersion` in
`apps/finicky/assets/Info.plist` (test: `TestVersionFileMatchesInfoPlist` in
`apps/finicky/src/version/version_test.go`).

## Chapters

- [URL pipeline](url-pipeline.md): from the Apple event to the open browser.
- [Configuration](configuration.md): config files, the VM, JSON rules and options.
- [Settings window and logs](settings-window.md): the web view, its messages and the log buffers.
- [Build and version](build-and-version.md): build scripts, binary size and the version.
- [Glossary](glossary.md)
