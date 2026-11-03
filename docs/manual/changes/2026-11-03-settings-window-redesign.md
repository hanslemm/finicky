---
title: A settings window that runs Finicky for you
summary: You can now set up and run Finicky from its window, see where each link went, make a rule from a link in one click, and test any link.
type: feat
pr: https://github.com/hanslemm/finicky/pull/1
---
## For everyone

Before, the window told a new user "No Valid Configuration Found", and a rule edit could erase
the settings. Now the window starts with a short status: is Finicky your default browser, are
your rules loaded, are there errors. It lists the last links that Finicky opened, with the rule
that decided each one, and a button that makes a rule from a link. On the Rules page you type a
link and see at once which rule wins. The window works with the keyboard and with screen
readers, and it follows light and dark mode.

## In 30 seconds

- Five pages: Overview, Rules, Settings, Logs and About, routed by `location.hash`; `svelte-routing` is gone.
- Go sends `recentLinks` (last 30, memory only) and `defaultBrowserStatus`; the page can send `makeDefaultBrowser`, `openConfig` and `revealConfig`.
- `openUrl` reports the matched handler, and JSON rules carry `ruleIndex`, so a test or a recent link names its rule.
- Profiles are now found also for Chromium- and Firefox-based browsers that `browsers.json` does not list.
- Every text color meets 4.5:1 in light and dark mode, and every control has a name and a focus ring.

## The numbers

Measured with `npm run build` and a WCAG contrast calculation on 2026-11-03.

```chart
{ "type": "bar", "unit": "KB", "rows": [
    { "label": "UI files in the app, before", "value": 772 },
    { "label": "UI files in the app, after", "value": 132, "highlight": true } ] }
```

| Measure | Before | After |
| --- | --- | --- |
| UI files embedded in the binary | 772 KB (571 KB of unused images) | 132 KB |
| `index.js` | 103.5 KB | 100.1 KB, with three more pages |
| Lowest text contrast, light mode | about 2.1:1 (warning text) | 4.7:1 |
| UI build warnings (`npm run build`) | 1 (`state_referenced_locally`) | 0 |

## How it fits

The window and Go talk only through messages. The new messages are marked.

```diagram
{ "nodes": [
    { "id": "loop", "label": "Event loop", "sub": "main.go", "x": 0, "y": 0, "kind": "changed" },
    { "id": "win", "label": "window.go", "sub": "message buffers", "x": 1, "y": 0, "kind": "changed" },
    { "id": "ov", "label": "Overview", "sub": "status, recent links", "x": 2, "y": 0, "kind": "new" },
    { "id": "rules", "label": "Rules", "sub": "test, edit, reorder", "x": 2, "y": 1, "kind": "changed" },
    { "id": "set", "label": "Settings", "sub": "options, config file", "x": 2, "y": 2, "kind": "new" },
    { "id": "ls", "label": "LaunchServices", "sub": "default browser", "x": 0, "y": 1, "kind": "same" } ],
  "edges": [
    { "from": "loop", "to": "win", "label": "recentLinks" },
    { "from": "win", "to": "ov", "label": "state" },
    { "from": "ov", "to": "rules", "label": "Make a rule" },
    { "from": "rules", "to": "win", "label": "testUrl, saveRules" },
    { "from": "set", "to": "win", "label": "saveRules, openConfig" },
    { "from": "loop", "to": "ls", "label": "makeDefault" } ],
  "steps": [
    { "title": "A link opens", "text": "The event loop records the link, the app that sent it and the rule that decided it. The list stays in memory.", "on": ["loop"] },
    { "title": "The window gets the state", "text": "window.go keeps the latest recentLinks and defaultBrowserStatus, and sends them when the page is ready.", "on": ["loop>win", "win", "win>ov", "ov"] },
    { "title": "Make a rule", "text": "On Overview, Make a rule opens the Rules page with a new rule for that site, for example *github.com/*.", "on": ["ov>rules", "rules"] },
    { "title": "Test and save", "text": "The Rules page tests the link and marks the winning rule. Each edit saves after 500 ms.", "on": ["rules>win", "win"] },
    { "title": "Settings", "text": "Options save at once. Open and Show in Finder use the path that Go knows, never one from the page.", "on": ["set>win", "win"] } ] }
```

## Engineering decisions

| Decision | Reason | Not chosen |
| --- | --- | --- |
| Put the test field on the Rules page, and mark the winning rule. | A person tests a link to fix a rule; both belong on one screen. | A separate Test page (the old way): it could not show which rule won. |
| Keep recent links in memory only, at most 30. | They hold every URL that you open. Memory only means nothing stays after Finicky quits. | Store them on disk: more history, but a privacy cost. |
| Show "Otherwise open in" as the last line of the rules. | The default browser is the rule that runs when no other rule matches. | A separate "Default browser" card on the start page (the old way). |
| Reorder by keyboard on the grip (arrow keys, Home, End), plus drag. | Order decides which rule wins; it must work without a mouse. | Move-up and move-down buttons on each row: more noise on every row. |
| Undo after delete, no confirm dialog. | Delete stays fast, and a mistake costs one click. | A confirm dialog on each delete. |
| Profile discovery by folder name in Application Support. | New Chromium and Firefox forks get profiles with no list update. | Only `browsers.json`: each new browser needs a code change. |
| Hash routing with a `$state` value. | Five fixed pages need no library. History routing has nothing to resolve on `finicky-assets://`. | Keep `svelte-routing`. |

## What changed

| Where | What it does |
| --- | --- |
| `packages/finicky-ui/src/pages/Overview.svelte` | New. Status, first-run steps, recent links with Test and Make a rule. |
| `packages/finicky-ui/src/pages/Rules.svelte` | Test field, "When … Open in …" rows, keyboard reorder, undo, wildcard fix, "Otherwise open in", save state. |
| `packages/finicky-ui/src/pages/Settings.svelte` | New. Options (also "Expand short links") and the config file. |
| `packages/finicky-ui/src/App.svelte`, `components/TabBar.svelte`, `lib/router.svelte.ts` | Runes, hash routing, log list bounded to 1000, safe start without the native host. |
| `packages/finicky-ui/src/components/` | Accessible option rows, selectors and tooltips; toasts with Undo and Dismiss; reduced motion. |
| `packages/finicky-ui/src/app.css` | Contrast-checked tokens for light and dark mode, focus ring, reduced motion. |
| `packages/finicky-ui/public/`, `StartPage.svelte`, `TestUrl.svelte`, `Toggle.svelte` | Removed: unused images and replaced pages. |
| `apps/finicky/src/main.go`, `window/window.go` | `recentLinks`, `defaultBrowserStatus`, `makeDefaultBrowser`, `openConfig`, match info in test results. |
| `apps/finicky/src/browser/discover.go` | New. Finds profiles of browsers that `browsers.json` does not list. |
| `apps/finicky/src/window/window.m` | The window follows the system appearance. |
| `packages/config-api/src/index.ts`, `apps/finicky/src/rules/rules.go` | `openUrl` returns `matched`; JSON rules carry `ruleIndex`. |

## Check it yourself

1. Run `cd packages/finicky-ui && npx svelte-check --tsconfig ./tsconfig.app.json`. It reports 0 errors.
2. Run `cd apps/finicky/src && go test ./...`. All packages pass.
3. Build and open the window. Overview shows the default-browser status.
4. Open a link from another app. It shows under Recent links with the rule that decided it.
5. On Rules, type a URL in "Test a link". The winning rule gets a blue mark.
6. Press Tab to a rule number, then the up arrow. The rule moves up and "Saved" shows.

## Limits and open items

- The test field cannot point at a handler inside a JS config; it names it as "Handler N of your JS config".
- Discovery finds profiles only in folders named after the app or its bundle id.
- Safari profiles are not supported.
- Only the macOS dark appearance was checked in the real app; light mode was checked in a browser.

## Words used here

- **Recent links**: the last 30 links that Finicky opened, kept in memory until it quits.
- **Profile store**: the file in which a browser lists its profiles (`Local State` or `profiles.ini`).
- **Focus ring**: the outline that shows which control the keyboard acts on.
- **Contrast ratio**: how much lighter one color is than another; 4.5:1 is the minimum for text.
- **Hash routing**: choosing the page from the part of the address after `#`.
