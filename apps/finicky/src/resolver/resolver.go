package resolver

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	"finicky/browser"
	"finicky/config"
	"finicky/rules"
	"finicky/shorturl"

	"github.com/dop251/goja"
)

// OpenerInfo describes the process that triggered the URL open.
type OpenerInfo struct {
	Name        string `json:"name"`
	BundleID    string `json:"bundleId"`
	Path        string `json:"path"`
	WindowTitle string `json:"windowTitle,omitempty"`
}

var (
	cachedRulesMu   sync.Mutex
	cachedRulesFile rules.RulesFile
	// cachedRulesGen changes each time the rules change, so a VM can tell
	// whether its merged config is stale. It starts at 1 so that 0 never
	// matches a fresh VM.
	cachedRulesGen uint64 = 1
)

// Scripts run for each URL. Compiled once; a goja.Program can run on any
// runtime.
var (
	openURLProgram = goja.MustCompile("openUrl",
		"finickyConfigAPI.openUrl(url, opener, originalUrl, finalConfig, true)", false)
	openMergedURLProgram = goja.MustCompile("openMergedUrl",
		"finickyConfigAPI.openUrl(url, opener, originalUrl, _mergedConfig, true)", false)
	buildMergedProgram = goja.MustCompile("buildMerged", `(function () {
		var merged = Object.assign({}, finalConfig, {
			handlers: (finalConfig.handlers || []).concat(_jsonHandlers)
		});
		if (finickyConfigAPI.validateConfig(merged)) { _mergedConfig = merged; return true; }
		_mergedConfig = finalConfig;
		return false;
	})()`, false)
)

// SetCachedRules stores a snapshot of the JSON rules so evaluateURL can use
// them without hitting disk on every URL open.
func SetCachedRules(rf rules.RulesFile) {
	cachedRulesMu.Lock()
	cachedRulesFile = rf
	cachedRulesGen++
	cachedRulesMu.Unlock()
}

func getCachedRules() (rules.RulesFile, uint64) {
	cachedRulesMu.Lock()
	defer cachedRulesMu.Unlock()
	return cachedRulesFile, cachedRulesGen
}

// ensureMergedConfig makes the _mergedConfig JS global hold the JS config
// plus the current JSON rules (as lower-priority handlers). It rebuilds and
// validates only when the rules or the VM changed, not on every URL.
func ensureMergedConfig(vm *config.VM) error {
	rf, gen := getCachedRules()
	if built, ok := vm.MergedRulesGen(); ok && built == gen {
		return nil
	}
	runtime := vm.Runtime()
	runtime.Set("_jsonHandlers", rules.ToJSHandlers(rf.Rules))
	valid, err := runtime.RunProgram(buildMergedProgram)
	if err != nil {
		return fmt.Errorf("failed to merge JSON rules into config: %v", err)
	}
	if !valid.ToBoolean() {
		slog.Warn("JSON rules are invalid, using the JS config without them")
	}
	vm.SetMergedRulesGen(gen)
	return nil
}

// ResolveURL determines which browser to use for the given URL.
//
// vm may be nil (no configuration at all). Whether to merge JSON rules is
// derived from vm.IsJSConfig().
//
// Always returns a non-nil config. Returns a non-nil error only when JS
// evaluation failed.
func ResolveURL(vm *config.VM, urlStr string, opener *OpenerInfo, openInBackground bool) (*browser.BrowserConfig, error) {
	if vm != nil {
		cfg, err := evaluateURL(vm, urlStr, opener)
		if err != nil {
			return defaultBrowserConfig(urlStr, openInBackground), err
		}
		cfg.OpenInBackground = mergeBackground(cfg.OpenInBackground, openInBackground)
		return cfg, nil
	}
	return defaultBrowserConfig(urlStr, openInBackground), nil
}

func mergeBackground(fromConfig *bool, requested bool) *bool {
	if fromConfig != nil {
		return fromConfig
	}
	return &requested
}

func evaluateURL(vm *config.VM, url string, opener *OpenerInfo) (*browser.BrowserConfig, error) {
	runtime := vm.Runtime()

	runtime.Set("originalUrl", url)
	if vm.GetAllConfigOptions().ResolveShortURLs {
		resolvedURL, err := shorturl.ResolveURL(url)
		if err != nil {
			slog.Info("Failed to resolve short URL", "error", err, "url", url, "using", resolvedURL)
		}
		url = resolvedURL
	}
	runtime.Set("url", url)

	if opener != nil {
		openerMap := map[string]interface{}{
			"name":     opener.Name,
			"bundleId": opener.BundleID,
			"path":     opener.Path,
		}
		if opener.WindowTitle != "" {
			openerMap["windowTitle"] = opener.WindowTitle
		}
		runtime.Set("opener", openerMap)
		slog.Debug("Setting opener", "name", opener.Name, "bundleId", opener.BundleID, "path", opener.Path, "windowTitle", opener.WindowTitle)
	} else {
		runtime.Set("opener", nil)
		slog.Debug("No opener detected")
	}

	// The config was validated when the VM was built, so openUrl skips the
	// per-URL validation. With a JS config, JSON rules are appended as
	// lower-priority handlers (merged and validated once per rules change).
	program := openURLProgram
	if vm.IsJSConfig() {
		if err := ensureMergedConfig(vm); err != nil {
			return nil, err
		}
		program = openMergedURLProgram
	}

	openResult, err := runtime.RunProgram(program)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate URL in config: %v", err)
	}

	resultJSON := openResult.ToObject(runtime).Export()
	resultBytes, err := json.Marshal(resultJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to process browser configuration: %v", err)
	}

	var browserResult browser.BrowserResult
	if err := json.Unmarshal(resultBytes, &browserResult); err != nil {
		return nil, fmt.Errorf("failed to parse browser configuration: %v", err)
	}

	slog.Debug("Final browser options",
		"name", browserResult.Browser.Name,
		"openInBackground", browserResult.Browser.OpenInBackground,
		"profile", browserResult.Browser.Profile,
		"args", browserResult.Browser.Args,
		"appType", browserResult.Browser.AppType,
	)

	var resultErr error
	if browserResult.Error != "" {
		resultErr = fmt.Errorf("%s", browserResult.Error)
	}
	browserResult.Browser.Matched = browserResult.Matched
	return &browserResult.Browser, resultErr
}

func defaultBrowserConfig(urlStr string, openInBackground bool) *browser.BrowserConfig {
	bg := openInBackground
	return &browser.BrowserConfig{
		Name:             "com.apple.Safari",
		AppType:          "bundleId",
		OpenInBackground: &bg,
		Args:             []string{},
		URL:              urlStr,
	}
}
