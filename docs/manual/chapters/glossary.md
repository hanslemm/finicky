---
title: Glossary
summary: The words this manual uses.
---
## Terms

- **Apple event**: a message that macOS sends to an app. `kAEGetURL` asks the app to open a URL.
- **ARC**: Automatic Reference Counting in Objective-C. This project does not use it, so the code
  calls `retain` and `release` itself.
- **Babel**: a JavaScript compiler. Finicky uses one plugin of it for named regex groups.
- **cgo**: the Go feature that calls C and Objective-C code.
- **esbuild**: a fast bundler. It joins the config file and its imports into one script.
- **Event loop**: the goroutine in `main()` that handles URLs, config reloads and window events
  one at a time.
- **goja**: a JavaScript engine written in Go. It runs the Config API and the user's config.
- **Handler**: a rule with `match` and `browser`. The first matching handler wins.
- **LaunchServices**: the macOS service that finds apps and opens URLs and files in them.
- **NSWorkspace**: the AppKit class that asks LaunchServices to open apps and URLs.
- **Physical footprint**: the memory that macOS counts for one process. `vmmap -summary` shows it.
- **Shortener**: a service such as `bit.ly` or `t.co` that redirects a short link to a long one.
- **WKWebView**: the macOS web view. It runs web content in a separate WebKit process.
- **zod**: a TypeScript library that checks the shape of the config.
