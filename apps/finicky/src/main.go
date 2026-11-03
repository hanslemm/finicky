package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa -framework CoreServices
#include <stdlib.h>
#include "main.h"
*/
import "C"

import (
	_ "embed"
	"encoding/base64"
	"finicky/browser"
	"finicky/config"
	"finicky/logger"
	"finicky/resolver"
	"finicky/rules"
	"finicky/util"
	"finicky/version"
	"finicky/window"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync/atomic"
	"time"
)

//go:embed assets/finickyConfigAPI.js
var finickyConfigAPIJS []byte

type UpdateInfo struct {
	ReleaseInfo        *version.ReleaseInfo
	UpdateCheckEnabled bool
}

type URLInfo struct {
	URL              string
	Opener           *resolver.OpenerInfo
	OpenInBackground bool
}

type ConfigInfo struct {
	Handlers       int16
	Rewrites       int16
	DefaultBrowser string
	ConfigPath     string
}

// urlListener is buffered so the Cocoa main thread, which sends on it from
// the Apple-event handler, does not wait while a previous URL is resolved.
var urlListener chan URLInfo = make(chan URLInfo, 16)
var windowClosed chan struct{} = make(chan struct{})

// jobs runs work on the event-loop goroutine. The goja runtime inside vm is
// not safe for concurrent use, so every VM access (URL resolution, test URLs,
// VM rebuilds) goes through the event loop.
var jobs chan func() = make(chan func(), 16)

// vm is read and written only on the event-loop goroutine (and in main
// before the loop starts).
var vm *config.VM

var forceWindowOpen bool = false
var queueWindowOpen chan bool = make(chan bool)
var lastError error
var dryRun bool = false
var skipJSConfig bool = false

// configInfo is written on the event loop and read on the Cocoa main thread
// (menu "Edit config"), so it is an atomic pointer.
var configInfo atomic.Pointer[ConfigInfo]
var shouldKeepRunning bool = true

// recentLinks holds the last links Finicky opened, newest first, for the
// settings window. Memory only: never written to disk. Event loop only.
const maxRecentLinks = 30

type recentLink struct {
	ID        uint64 `json:"id"` // unique in this process: two links can share a URL and second
	Time      string `json:"time"`
	URL       string `json:"url"`
	Opener    string `json:"opener,omitempty"`
	Browser   string `json:"browser"`
	Profile   string `json:"profile,omitempty"`
	Source    string `json:"source"` // "rule", "js", "default" or "error"
	RuleIndex *int   `json:"ruleIndex,omitempty"`
}

var recentLinks []recentLink
var recentLinkSeq uint64

func main() {
	startTime := time.Now()
	logger.Setup()
	runtime.LockOSThread()

	// Define command line flags
	configPathPtr := flag.String("config", "", "Path to custom JS configuration file")
	rulesPathPtr := flag.String("rules", "", "Path to custom rules JSON file")
	noConfigPtr := flag.Bool("no-config", false, "Skip JS configuration file entirely")
	windowPtr := flag.Bool("window", false, "Force window to open")
	dryRunPtr := flag.Bool("dry-run", false, "Simulate without actually opening browsers")
	flag.Parse()

	// Use the parsed values
	customConfigPath := *configPathPtr
	if customConfigPath != "" {
		slog.Debug("Using custom config path", "path", customConfigPath)
	}

	if *rulesPathPtr != "" {
		slog.Debug("Using custom rules path", "path", *rulesPathPtr)
		rules.SetCustomPath(*rulesPathPtr)
	}

	if *noConfigPtr {
		slog.Debug("Skipping JS config")
		skipJSConfig = true
	}

	if *windowPtr {
		forceWindowOpen = true
	}

	dryRun = *dryRunPtr

	currentVersion := version.GetCurrentVersion()
	commitHash, buildDate := version.GetBuildInfo()
	slog.Info("Starting Finicky", "version", currentVersion)
	slog.Debug("Build info", "buildDate", buildDate, "commitHash", commitHash)

	go func() {
		defer reportDefaultBrowser()
		is_default_browser, err := setDefaultBrowser()
		if err != nil {
			slog.Debug("Failed checking if we are the default browser", "error", err)
		} else if !is_default_browser {
			slog.Debug("Finicky is not the default browser")
		} else {
			slog.Debug("Finicky is the default browser")
		}
	}()

	namespace := "finickyConfig"
	configChange := make(chan struct{}, 1)
	cfw, err := config.NewConfigFileWatcher(customConfigPath, namespace, configChange)

	if err != nil {
		handleFatalError(fmt.Sprintf("Failed to setup config file watcher: %v", err))
	}

	vm, err = setupVM(cfw, namespace)
	if err != nil {
		handleFatalError(err.Error())
	}

	slog.Debug("VM setup complete", "duration", fmt.Sprintf("%.2fms", float64(time.Since(startTime).Microseconds())/1000))

	go checkForUpdates(vm.GetAllConfigOptions().CheckForUpdates)

	// UI handlers run on the Cocoa main thread; hand the work to the event loop.
	window.TestUrlHandler = func(url string) {
		jobs <- func() { TestURLInternal(url) }
	}

	// When there is no JS config, rebuild the VM from the updated rules.
	// When there is a JS config, the resolver merges the new JSON rules on the
	// next URL — nothing else to do.
	window.SaveRulesHandler = func(rf rules.RulesFile) {
		jobs <- func() { applySavedRules(rf, namespace) }
	}

	window.MakeDefaultHandler = func() { go makeDefaultBrowser() }
	window.OpenConfigHandler = func(reveal bool) { go openConfigFile(reveal) }
	window.WindowShownHandler = func() {
		go reportDefaultBrowser()
		jobs <- func() { sendRecentLinks() }
	}

	const oneDay = 24 * time.Hour

	var showingWindow bool = false
	timeoutChan := time.After(1 * time.Second)
	updateChan := time.After(oneDay)

	if vm != nil {
		shouldKeepRunning = vm.GetAllConfigOptions().KeepRunning
	}
	if shouldKeepRunning {
		timeoutChan = nil
	}

	go func() {
		slog.Info("Listening for events...")
		for {
			select {
			case job := <-jobs:
				job()

			case urlInfo := <-urlListener:
				startTime := time.Now()

				url := urlInfo.URL

				slog.Info("URL received", "url", url)

				config, err := resolver.ResolveURL(vm, url, urlInfo.Opener, urlInfo.OpenInBackground)
				if err != nil {
					handleRuntimeError(err)
				} else {
					lastError = nil
				}
				recordRecentLink(url, urlInfo.Opener, config, err)
				if launchErr := browser.LaunchBrowser(*config, dryRun, urlInfo.OpenInBackground); launchErr != nil {
					slog.Error("Failed to start browser", "error", launchErr)
				}

				slog.Debug("Time taken evaluating URL and opening browser", "duration", fmt.Sprintf("%.2fms", float64(time.Since(startTime).Microseconds())/1000))

				if !showingWindow && !shouldKeepRunning {
					timeoutChan = time.After(2 * time.Second)
				} else {
					timeoutChan = nil
				}

			case <-configChange:
				startTime := time.Now()
				var setupErr error
				slog.Debug("Config has changed")
				vm, setupErr = setupVM(cfw, namespace)
				if setupErr != nil {
					handleRuntimeError(setupErr)
				} else {
					lastError = nil
					C.SetStatusItemError(false)
				}
				slog.Debug("VM refresh complete", "duration", fmt.Sprintf("%.2fms", float64(time.Since(startTime).Microseconds())/1000))
				opts := vm.GetAllConfigOptions()
				shouldKeepRunning = opts.KeepRunning
				go checkForUpdates(opts.CheckForUpdates)

			case shouldShowWindow := <-queueWindowOpen:
				if !showingWindow && shouldShowWindow {
					go ShowConfigWindow()
					showingWindow = true
					timeoutChan = nil
				}

			case <-updateChan:
				go checkForUpdates(vm.GetAllConfigOptions().CheckForUpdates)
				updateChan = time.After(oneDay)

			case <-windowClosed:
				if !shouldKeepRunning {
					slog.Info("Exiting due to window closed")
					tearDown()
				} else {
					slog.Debug("Window closed")
				}

			case <-timeoutChan:
				slog.Info("Exiting due to timeout")
				tearDown()
			}
		}
	}()

	shouldHideIcon := false
	if vm != nil {
		shouldHideIcon = vm.GetAllConfigOptions().HideIcon
	}
	C.RunApp(C.bool(forceWindowOpen), C.bool(!shouldHideIcon), C.bool(shouldKeepRunning))
}

func handleRuntimeError(err error) {
	slog.Error("Failed evaluating url", "error", err)
	lastError = err
	C.SetStatusItemError(true)
}

//export HandleURL
func HandleURL(url *C.char, name *C.char, bundleId *C.char, path *C.char, windowTitle *C.char, openInBackground C.bool) {
	var opener resolver.OpenerInfo

	if name != nil && bundleId != nil && path != nil {
		opener = resolver.OpenerInfo{
			Name:     C.GoString(name),
			BundleID: C.GoString(bundleId),
			Path:     C.GoString(path),
		}
		if windowTitle != nil {
			opener.WindowTitle = C.GoString(windowTitle)
		}
	}

	urlString := C.GoString(url)

	// Handle finicky:// protocol URLs by extracting and decoding the embedded URL
	if strings.HasPrefix(urlString, "finicky://open/") {
		encodedURL := strings.TrimPrefix(urlString, "finicky://open/")
		if decodedBytes, err := base64.StdEncoding.DecodeString(encodedURL); err == nil {
			urlString = string(decodedBytes)
			slog.Debug("Decoded finicky protocol URL", "original", C.GoString(url), "decoded", urlString)
		} else {
			slog.Warn("Failed to decode finicky protocol URL", "error", err, "url", C.GoString(url))
		}
	}

	urlListener <- URLInfo{
		URL:              urlString,
		Opener:           &opener,
		OpenInBackground: bool(openInBackground),
	}
}

//export TestURL
func TestURL(url *C.char) {
	urlString := C.GoString(url)
	jobs <- func() { TestURLInternal(urlString) }
}

// applySavedRules runs on the event loop after the UI saved rules.json.
func applySavedRules(rf rules.RulesFile, namespace string) {
	slog.Debug("Rules updated", "count", len(rf.Rules))
	resolver.SetCachedRules(rf)
	if vm != nil && vm.IsJSConfig() {
		return
	}
	if rf.DefaultBrowser == "" && len(rf.Rules) == 0 && rf.Options == nil {
		vm = nil
		return
	}
	script, err := rules.ToJSConfigScript(rf, namespace)
	if err != nil {
		slog.Error("Failed to generate config from rules", "error", err)
		return
	}
	newVM, err := config.NewFromScript(finickyConfigAPIJS, namespace, script)
	if err != nil {
		slog.Error("Failed to rebuild VM from rules", "error", err)
		return
	}
	vm = newVM
	opts := vm.GetAllConfigOptions()
	shouldKeepRunning = opts.KeepRunning
	if err := logger.SetupFile(opts.LogRequests); err != nil {
		slog.Warn("Failed to setup file logging", "error", err)
	}
	go checkForUpdates(opts.CheckForUpdates)
}

func TestURLInternal(urlString string) {
	slog.Debug("Testing URL", "url", urlString)

	config, err := resolver.ResolveURL(vm, urlString, nil, false)
	if err != nil {
		slog.Error("Failed to evaluate URL", "error", err)
		window.SendMessageToWebView("testUrlResult", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	source, ruleIndex := matchSource(config, nil)
	window.SendMessageToWebView("testUrlResult", map[string]interface{}{
		"url":              config.URL,
		"browser":          config.Name,
		"openInBackground": config.OpenInBackground,
		"profile":          config.Profile,
		"args":             config.Args,
		"source":           source,
		"ruleIndex":        ruleIndex,
		"handlerIndex":     handlerIndex(config),
	})
}

// matchSource says what decided a URL: a JSON rule ("rule", with its index),
// a JS config handler ("js"), the default browser ("default"), or an error.
func matchSource(config *browser.BrowserConfig, err error) (string, *int) {
	switch {
	case err != nil:
		return "error", nil
	case config.Matched == nil:
		return "default", nil
	case config.Matched.RuleIndex != nil:
		return "rule", config.Matched.RuleIndex
	default:
		return "js", nil
	}
}

func handlerIndex(config *browser.BrowserConfig) *int {
	if config.Matched == nil {
		return nil
	}
	return &config.Matched.HandlerIndex
}

// recordRecentLink runs on the event loop after each URL.
func recordRecentLink(url string, opener *resolver.OpenerInfo, config *browser.BrowserConfig, err error) {
	source, ruleIndex := matchSource(config, err)
	recentLinkSeq++
	link := recentLink{
		ID:        recentLinkSeq,
		Time:      time.Now().Format(time.RFC3339),
		URL:       url,
		Browser:   config.Name,
		Profile:   config.Profile,
		Source:    source,
		RuleIndex: ruleIndex,
	}
	if opener != nil {
		link.Opener = opener.Name
	}
	recentLinks = append([]recentLink{link}, recentLinks...)
	if len(recentLinks) > maxRecentLinks {
		recentLinks = recentLinks[:maxRecentLinks]
	}
	sendRecentLinks()
}

func sendRecentLinks() {
	list := make([]recentLink, len(recentLinks))
	copy(list, recentLinks)
	window.SendMessageToWebView("recentLinks", list)
}

// openConfigFile opens the active config (JS file or rules.json) in its
// default app, or shows it in Finder. The path comes from Finicky itself, never
// from the window.
func openConfigFile(reveal bool) {
	ci := configInfo.Load()
	path := ""
	if ci != nil {
		path = ci.ConfigPath
	}
	if path == "" {
		path, _ = rules.GetPath()
	}
	if _, err := os.Stat(path); err != nil {
		slog.Warn("No config file to open", "path", path)
		return
	}
	args := []string{path}
	if reveal {
		args = []string{"-R", path}
	}
	if err := exec.Command("open", args...).Run(); err != nil {
		slog.Warn("Failed to open config file", "path", path, "error", err)
	}
}

// reportDefaultBrowser tells the window whether Finicky handles http, https
// and finicky links. It only reads Launch Services, so any goroutine may call it.
func reportDefaultBrowser() {
	isDefault, _ := isDefaultBrowser()
	window.SendMessageToWebView("defaultBrowserStatus", map[string]interface{}{"isDefault": isDefault})
}

// makeDefaultBrowser asks macOS to make Finicky the default browser. macOS
// shows a confirmation dialog, so poll the result for up to 30 s.
func makeDefaultBrowser() {
	if _, err := setDefaultBrowser(); err != nil {
		slog.Warn("Failed to set Finicky as default browser", "error", err)
	}
	for i := 0; i < 30; i++ {
		time.Sleep(time.Second)
		if isDefault, _ := isDefaultBrowser(); isDefault {
			break
		}
	}
	reportDefaultBrowser()
}

func handleFatalError(errorMessage string) {
	slog.Error("Fatal error", "msg", errorMessage)
	lastError = fmt.Errorf("%s", errorMessage)
	C.SetStatusItemError(true)
}

//export QueueWindowDisplay
func QueueWindowDisplay(openWindow int32) {
	queueWindowOpen <- openWindow != 0
}

//export ShowConfigWindow
func ShowConfigWindow() {
	slog.Debug("Showing window")
	window.ShowWindow()

	// Send version information
	currentVersion := version.GetCurrentVersion()
	window.SendMessageToWebView("version", currentVersion)

}

//export WindowDidClose
func WindowDidClose() {
	window.MarkClosed()
	windowClosed <- struct{}{}
}

//export GetCurrentConfigPath
func GetCurrentConfigPath() *C.char {
	if ci := configInfo.Load(); ci != nil && ci.ConfigPath != "" {
		return C.CString(ci.ConfigPath)
	}
	return nil
}

// checkForUpdates runs on its own goroutine. It must not touch vm: the caller
// passes the checkForUpdates option in.
func checkForUpdates(enabled bool) {
	releaseInfo, updateCheckEnabled, err := version.CheckForUpdatesIfEnabled(enabled)
	if err != nil {
		slog.Error("Error checking for updates", "error", err)
	}

	updateInfo := UpdateInfo{
		ReleaseInfo:        releaseInfo,
		UpdateCheckEnabled: updateCheckEnabled,
	}

	if updateInfo.ReleaseInfo != nil && updateInfo.ReleaseInfo.HasUpdate {
		slog.Info("New version is available", "version", updateInfo.ReleaseInfo.LatestVersion)
	}

	if updateInfo.ReleaseInfo != nil {
		window.SendMessageToWebView("updateInfo", map[string]interface{}{
			"version":            updateInfo.ReleaseInfo.LatestVersion,
			"hasUpdate":          updateInfo.ReleaseInfo.HasUpdate,
			"updateCheckEnabled": updateInfo.UpdateCheckEnabled,
			"downloadUrl":        updateInfo.ReleaseInfo.DownloadUrl,
			"releaseUrl":         updateInfo.ReleaseInfo.ReleaseUrl,
		})
	} else {
		window.SendMessageToWebView("updateInfo", map[string]interface{}{
			"version":            "",
			"hasUpdate":          false,
			"updateCheckEnabled": updateInfo.UpdateCheckEnabled,
			"downloadUrl":        "",
			"releaseUrl":         "",
		})
	}
}

// tearDown exits at once. The update check already ran at start (at most
// once a day); waiting for the network here only delayed the exit.
func tearDown() {
	slog.Info("Exiting...")
	os.Exit(0)
}

func setupVM(cfw *config.ConfigFileWatcher, namespace string) (*config.VM, error) {
	logRequests := true
	var err error

	defer func() {
		err = logger.SetupFile(logRequests)
		if err != nil {
			slog.Warn("Failed to setup file logging", "error", err)
		}
	}()

	var currentBundlePath, configPath string
	if !skipJSConfig {
		var err2 error
		currentBundlePath, configPath, err2 = cfw.BundleConfig()
		if err2 != nil {
			return nil, fmt.Errorf("failed to read config: %v", err2)
		}
	}

	// Always seed the cached rules from disk so JSON rules are applied
	// immediately on startup, even when a JS config is also present.
	if rf, rulesErr := rules.Load(); rulesErr != nil {
		slog.Warn("Failed to pre-load rules cache", "error", rulesErr)
	} else {
		resolver.SetCachedRules(rf)
	}

	var newVM *config.VM

	if currentBundlePath != "" {
		newVM, err = config.New(finickyConfigAPIJS, namespace, currentBundlePath)
		if err != nil {
			return nil, fmt.Errorf("failed to setup VM: %v", err)
		}
	} else {
		rf, rulesErr := rules.Load()
		if rulesErr != nil {
			slog.Warn("Failed to load rules file", "error", rulesErr)
		} else {
			resolver.SetCachedRules(rf)
			if rf.DefaultBrowser != "" || len(rf.Rules) > 0 {
				script, scriptErr := rules.ToJSConfigScript(rf, namespace)
				if scriptErr != nil {
					return nil, fmt.Errorf("failed to generate config from rules: %v", scriptErr)
				}
				newVM, err = config.NewFromScript(finickyConfigAPIJS, namespace, script)
				if err != nil {
					return nil, fmt.Errorf("failed to setup VM from rules: %v", err)
				}
				configPath, _ = rules.GetPath()
			}
		}
	}

	if newVM == nil {
		return nil, nil
	}

	ci := &ConfigInfo{ConfigPath: configPath}
	if cs := newVM.GetConfigState(); cs != nil {
		ci.Handlers = cs.Handlers
		ci.Rewrites = cs.Rewrites
		ci.DefaultBrowser = cs.DefaultBrowser
	}
	configInfo.Store(ci)

	opts := newVM.GetAllConfigOptions()
	logRequests = opts.LogRequests

	window.SendMessageToWebView("config", map[string]interface{}{
		"handlers":       ci.Handlers,
		"rewrites":       ci.Rewrites,
		"defaultBrowser": ci.DefaultBrowser,
		"configPath":     util.ShortenPath(ci.ConfigPath),
		"isJSConfig":     newVM.IsJSConfig(),
		"options": map[string]interface{}{
			"keepRunning":      opts.KeepRunning,
			"hideIcon":         opts.HideIcon,
			"logRequests":      opts.LogRequests,
			"checkForUpdates":  opts.CheckForUpdates,
			"resolveShortUrls": opts.ResolveShortURLs,
		},
	})

	return newVM, nil
}
