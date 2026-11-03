package browser

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"

	"al.essio.dev/pkg/shellescape"
	"finicky/util"
)

//go:embed browsers.json
var browsersJsonData []byte

type BrowserResult struct {
	Browser BrowserConfig `json:"browser"`
	Error   string        `json:"error"`
	Matched *MatchInfo    `json:"matched"`
}

// MatchInfo says which handler decided a URL. RuleIndex is set only when the
// handler came from the JSON rules file; it is the index of that rule.
type MatchInfo struct {
	HandlerIndex int  `json:"handlerIndex"`
	RuleIndex    *int `json:"ruleIndex,omitempty"`
}

type BrowserConfig struct {
	Name             string   `json:"name"`
	AppType          string   `json:"appType"`
	OpenInBackground *bool    `json:"openInBackground"`
	Profile          string   `json:"profile"`
	Args             []string `json:"args"`
	URL              string   `json:"url"`
	// Matched is nil when no handler matched (the default browser was used).
	Matched *MatchInfo `json:"-"`
}

type browserInfo struct {
	ConfigDirRelative string `json:"config_dir_relative"`
	ID                string `json:"id"`
	AppName           string `json:"app_name"`
	Type              string `json:"type"`
}

func LaunchBrowser(config BrowserConfig, dryRun bool, openInBackgroundByDefault bool) error {
	if config.AppType == "none" {
		slog.Info("AppType is 'none', not launching any browser")
		return nil
	}

	slog.Info("Starting browser", "name", config.Name, "url", config.URL)

	var openArgs []string

	if config.AppType == "bundleId" {
		openArgs = []string{"-b", config.Name}
	} else {
		openArgs = []string{"-a", config.Name}
	}

	var openInBackground bool = openInBackgroundByDefault

	if config.OpenInBackground != nil {
		openInBackground = *config.OpenInBackground
	}

	if openInBackground {
		openArgs = append(openArgs, "-g")
	}

	// Handle profile and custom args
	profileArgs, ok := resolveBrowserProfileArgs(config.Name, config.Profile)
	hasCustomArgs := len(config.Args) > 0

	// Fast path: ask LaunchServices directly, when the request has a native
	// equivalent (see nativePlan). Otherwise only `open` can honour it.
	if plan, native := nativePlan(config.Args, profileArgs, ok, config.URL, openInBackground); native {
		if dryRun {
			slog.Debug("Would open natively (dry run)", "app", config.Name, "url", config.URL, "background", plan.background, "newInstance", plan.newInstance, "args", plan.args)
			return nil
		}
		err := openNative(config.Name, config.AppType == "bundleId", config.URL, plan.background, plan.newInstance, plan.args, plan.passURL)
		if err == nil {
			slog.Debug("Opened natively", "app", config.Name, "profileArgs", profileArgs)
			return nil
		}
		if !errors.Is(err, errNativeUnresolved) {
			return err
		}
		slog.Debug("Native open could not resolve app or URL, falling back to open(1)", "app", config.Name)
	}

	// Add -n flag if profile is used (required for profile switching)
	if ok {
		openArgs = append(openArgs, "-n")
	}

	// Add --args if we have profile args or custom args
	if ok || hasCustomArgs {
		if !slices.Contains(config.Args, "--args") {
			openArgs = append(openArgs, "--args")
		}
		// Add profile arguments first if present
		if ok {
			openArgs = append(openArgs, profileArgs...)
		}

		// Add custom args or URL
		if hasCustomArgs {
			openArgs = append(openArgs, config.Args...)
		} else {
			openArgs = append(openArgs, config.URL)
		}
	} else {
		// No special args, just add the URL
		openArgs = append(openArgs, config.URL)
	}

	cmd := exec.Command("open", openArgs...)

	// Pretty print the command with proper escaping
	prettyCmd := formatCommand(cmd.Path, cmd.Args)

	if dryRun {
		slog.Debug("Would run command (dry run)", "command", prettyCmd)
		return nil
	} else {
		slog.Debug("Run command", "command", prettyCmd)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	stderrBytes, err := io.ReadAll(stderr)
	if err != nil {
		return fmt.Errorf("error reading stderr: %v", err)
	}

	stdoutBytes, err := io.ReadAll(stdout)
	if err != nil {
		return fmt.Errorf("error reading stdout: %v", err)
	}

	cmdErr := cmd.Wait()

	if len(stderrBytes) > 0 {
		slog.Error("Command returned error", "error", string(stderrBytes))
	}
	if len(stdoutBytes) > 0 {
		slog.Debug("Command returned output", "output", string(stdoutBytes))
	}

	if cmdErr != nil {
		return fmt.Errorf("command failed: %v", cmdErr)
	}

	return nil
}

// launchPlan is a request that NSWorkspace can do without /usr/bin/open.
type launchPlan struct {
	args        []string // app arguments; empty when passURL is true
	passURL     bool     // open the URL in the app (else launch it with args)
	newInstance bool     // `open -n`
	background  bool     // `open -g`
}

// nativePlan maps a launch request to NSWorkspace. It returns false when only
// `open` can do it. It covers:
//
//   - no custom args: open the URL, or with a profile launch a new instance
//     with "<profile args> <url>" (`open -n -a App --args <profile> <url>`);
//   - custom args with no "--args": they are app arguments (`open` adds
//     "--args" before them), with the profile arguments first;
//   - custom args of the form [-n] [-g] --args <app args>, with no profile:
//     the open flags become options. Any other open flag needs `open`.
//
// As with `open`, custom args replace the URL: the config gives it in args.
func nativePlan(custom, profileArgs []string, hasProfile bool, url string, background bool) (launchPlan, bool) {
	plan := launchPlan{background: background}

	if len(custom) == 0 {
		if hasProfile {
			plan.args = append(append([]string{}, profileArgs...), url)
			plan.newInstance = true
			return plan, true
		}
		plan.passURL = true
		return plan, true
	}

	sep := slices.Index(custom, "--args")
	if sep < 0 {
		plan.args = append(append([]string{}, profileArgs...), custom...)
		plan.newInstance = hasProfile
		return plan, true
	}

	if hasProfile {
		// `open` puts the profile args before the custom "--args"; keep that
		// exact behaviour by leaving it to `open`.
		return launchPlan{}, false
	}
	for _, flag := range custom[:sep] {
		switch flag {
		case "-n":
			plan.newInstance = true
		case "-g":
			plan.background = true
		default:
			return launchPlan{}, false
		}
	}
	plan.args = append([]string{}, custom[sep+1:]...)
	return plan, true
}

// knownBrowsers parses the embedded browsers.json once.
var knownBrowsers = sync.OnceValue(func() []browserInfo {
	var list []browserInfo
	if err := json.Unmarshal(browsersJsonData, &list); err != nil {
		slog.Error("Error parsing browsers.json", "error", err)
	}
	return list
})

func resolveBrowserProfileArgs(identifier string, profile string) ([]string, bool) {
	if profile == "" {
		return nil, false
	}
	matchedBrowser := findBrowser(identifier)
	if matchedBrowser == nil {
		return nil, false
	}

	slog.Debug("Browser profile store found", "identifier", identifier, "type", matchedBrowser.Type)

	if profile != "" {
		switch matchedBrowser.Type {
		case "Chromium":
			homeDir, err := util.UserHomeDir()
			if err != nil {
				slog.Info("Error getting home directory", "error", err)
				return nil, false
			}

			localStatePath := filepath.Join(homeDir, "Library/Application Support", matchedBrowser.ConfigDirRelative, "Local State")
			profilePath, ok := parseProfiles(localStatePath, profile)
			if ok {
				return []string{"--profile-directory=" + profilePath}, true
			}
			// macOS can block the read of another app's data. A profile
			// folder name ("Profile 3") is still safe to pass on; a display
			// name is not: Chrome would make a new, empty profile with it.
			if isBlocked(localStatePath) {
				if chromiumProfileDirRe.MatchString(profile) {
					slog.Debug("Profile list is blocked by macOS; using the profile folder name", "profile", profile)
					return []string{"--profile-directory=" + profile}, true
				}
				slog.Warn("macOS blocks Finicky from reading the browser's profile list. Use the profile folder name (see chrome://version, \"Profile Path\"), for example \"Profile 1\", or allow Finicky in System Settings > Privacy & Security.", "browser", identifier, "profile", profile)
			}
		case "Firefox":
			homeDir, err := util.UserHomeDir()
			if err != nil {
				slog.Info("Error getting home directory", "error", err)
				return nil, false
			}

			profilesIniPath := filepath.Join(homeDir, "Library/Application Support", matchedBrowser.ConfigDirRelative, "profiles.ini")
			profileName, ok := parseFirefoxProfiles(profilesIniPath, profile)
			if ok {
				return []string{"-P", profileName}, true
			}
			// Firefox takes a profile by name; when macOS blocks the list,
			// pass the name on. An unknown name opens Firefox's profile manager.
			if isBlocked(profilesIniPath) {
				return []string{"-P", profile}, true
			}
		default:
			slog.Info("Browser is not a supported browser type, skipping profile detection", "identifier", identifier)
		}
	}

	return nil, false
}

func readFirefoxProfileNames(profilesIniPath string) []string {
	data, err := os.ReadFile(profilesIniPath)
	if err != nil {
		slog.Info("Error reading profiles.ini", "path", profilesIniPath, "error", err)
		return []string{}
	}

	names := []string{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if name, ok := strings.CutPrefix(line, "Name="); ok {
			names = append(names, name)
		}
	}
	return names
}

func parseFirefoxProfiles(profilesIniPath string, profile string) (string, bool) {
	names := readFirefoxProfileNames(profilesIniPath)
	for _, name := range names {
		if name == profile {
			return name, true
		}
	}
	slog.Warn("Could not find profile in Firefox profiles.", "Expected profile", profile, "Available profiles", strings.Join(names, ", "))
	return "", false
}

func chromiumInfoCache(localStatePath string) (map[string]interface{}, bool) {
	data, err := os.ReadFile(localStatePath)
	if err != nil {
		slog.Info("Error reading Local State file", "path", localStatePath, "error", err)
		return nil, false
	}

	var localState map[string]interface{}
	if err := json.Unmarshal(data, &localState); err != nil {
		slog.Info("Error parsing Local State JSON", "error", err)
		return nil, false
	}

	profiles, ok := localState["profile"].(map[string]interface{})
	if !ok {
		slog.Info("Could not find profile section in Local State")
		return nil, false
	}

	infoCache, ok := profiles["info_cache"].(map[string]interface{})
	if !ok {
		slog.Info("Could not find info_cache in profile section")
		return nil, false
	}

	return infoCache, true
}

func getAllChromiumProfiles(localStatePath string) []string {
	cache, ok := chromiumInfoCache(localStatePath)
	if !ok {
		return []string{}
	}

	var names []string
	for _, info := range cache {
		profileInfo, ok := info.(map[string]interface{})
		if !ok {
			continue
		}
		name, ok := profileInfo["name"].(string)
		if !ok {
			continue
		}
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

func parseProfiles(localStatePath string, profile string) (string, bool) {
	infoCache, ok := chromiumInfoCache(localStatePath)
	if !ok {
		return "", false
	}

	// Look for the specified profile
	for profilePath, info := range infoCache {
		profileInfo, ok := info.(map[string]interface{})
		if !ok {
			continue
		}

		name, ok := profileInfo["name"].(string)
		if !ok {
			continue
		}

		if name == profile {
			slog.Info("Found profile by name", "name", name, "path", profilePath)
			return profilePath, true
		}
	}

	// If we didn't find the profile, try to find it by profile folder name
	slog.Debug("Could not find profile in browser profiles, trying to find by profile path", "profile", profile)
	for profilePath, info := range infoCache {
		if profilePath == profile {
			// Try to get the profile name of the profile we want the user to use instead
			if profileInfo, ok := info.(map[string]interface{}); ok {
				if name, ok := profileInfo["name"].(string); ok {
					slog.Warn("Found profile using profile path", "path", profilePath, "name", name, "suggestion", "Please use the profile name instead")
				}
			}
			return profilePath, true
		}
	}

	var profileNames []string
	for _, info := range infoCache {
		profileInfo, ok := info.(map[string]interface{})
		if !ok {
			continue
		}

		name, ok := profileInfo["name"].(string)
		if !ok {
			continue
		}

		profileNames = append(profileNames, name)
	}
	slog.Warn("Could not find profile in browser profiles.", "Expected profile", profile, "Available profiles", strings.Join(profileNames, ", "))

	return "", false
}

// ProfileInfo describes the profiles of one browser for the settings window.
type ProfileInfo struct {
	Profiles []string `json:"profiles"`
	// Supported is true for browsers whose profiles Finicky can select.
	Supported bool `json:"supported"`
	// Blocked is true when macOS refused the read of the profile list.
	Blocked bool `json:"blocked"`
}

// chromiumProfileDirRe matches Chromium profile folder names.
var chromiumProfileDirRe = regexp.MustCompile(`^(Default|Profile [0-9]+|Guest Profile|System Profile)$`)

// isBlocked reports whether reading path fails for lack of permission, as it
// does on recent macOS for another app's data.
func isBlocked(path string) bool {
	f, err := os.Open(path)
	if err == nil {
		f.Close()
		return false
	}
	return errors.Is(err, fs.ErrPermission)
}

// GetProfileInfo returns the profiles of a browser (app name or bundle ID). It
// knows the browsers in browsers.json and discovers other Chromium- and
// Firefox-based browsers (see discover.go).
func GetProfileInfo(identifier string) ProfileInfo {
	info := ProfileInfo{Profiles: []string{}}
	matchedBrowser := findBrowser(identifier)
	if matchedBrowser == nil {
		return info
	}

	homeDir, err := util.UserHomeDir()
	if err != nil {
		slog.Info("Error getting home directory", "error", err)
		return info
	}

	var path string
	switch matchedBrowser.Type {
	case "Chromium":
		path = filepath.Join(homeDir, "Library/Application Support", matchedBrowser.ConfigDirRelative, "Local State")
		info.Profiles = getAllChromiumProfiles(path)
	case "Firefox":
		path = filepath.Join(homeDir, "Library/Application Support", matchedBrowser.ConfigDirRelative, "profiles.ini")
		info.Profiles = readFirefoxProfileNames(path)
	default:
		return info
	}
	info.Supported = true
	info.Blocked = len(info.Profiles) == 0 && isBlocked(path)
	return info
}

// GetProfilesForBrowser returns the profile names of a browser, or an empty
// slice when there are none or they cannot be read.
func GetProfilesForBrowser(identifier string) []string {
	return GetProfileInfo(identifier).Profiles
}

// formatCommand returns a properly shell-escaped string representation of the command
func formatCommand(path string, args []string) string {
	if len(args) == 0 {
		return shellescape.Quote(path)
	}

	quotedArgs := make([]string, len(args))
	for i, arg := range args {
		quotedArgs[i] = shellescape.Quote(arg)
	}

	return strings.Join(quotedArgs, " ")
}
