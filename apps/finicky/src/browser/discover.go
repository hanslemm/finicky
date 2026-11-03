package browser

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"finicky/util"
)

// Profile discovery for browsers that browsers.json does not list.
//
// Chromium-based browsers keep their profiles in a "Local State" file, and
// Firefox-based browsers in "profiles.ini", inside a folder in
// ~/Library/Application Support. That folder is usually named after the app
// ("Thorium") or after its vendor and app ("Vendor/App"), or after the bundle
// id. Discovery looks for those names, one and two levels deep.

var discovered sync.Map // identifier -> *browserInfo (nil when nothing was found)

// findBrowser returns the profile information for a browser, by bundle id or
// app name: from browsers.json first, else by discovery. It returns nil when
// the browser has no known profile store.
func findBrowser(identifier string) *browserInfo {
	list := knownBrowsers()
	for i := range list {
		if list[i].ID == identifier || list[i].AppName == identifier {
			return &list[i]
		}
	}
	if identifier == "" {
		return nil
	}
	if v, ok := discovered.Load(identifier); ok {
		return v.(*browserInfo)
	}
	var info *browserInfo
	if home, err := util.UserHomeDir(); err == nil {
		info = discoverIn(filepath.Join(home, "Library", "Application Support"), identifier)
	}
	if info != nil {
		slog.Debug("Discovered browser profiles", "identifier", identifier, "type", info.Type, "dir", info.ConfigDirRelative)
	}
	discovered.Store(identifier, info)
	return info
}

// discoverIn looks in appSupport for the profile store of identifier.
func discoverIn(appSupport, identifier string) *browserInfo {
	names := candidateNames(identifier)
	if len(names) == 0 {
		return nil
	}

	entries, err := os.ReadDir(appSupport)
	if err != nil {
		return nil
	}

	// Level 1: ~/Library/Application Support/<name>
	for _, e := range entries {
		if e.IsDir() && names[strings.ToLower(e.Name())] {
			if info := profileStore(appSupport, e.Name(), identifier); info != nil {
				return info
			}
		}
	}

	// Level 2: ~/Library/Application Support/<vendor>/<name>
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		subs, err := os.ReadDir(filepath.Join(appSupport, e.Name()))
		if err != nil {
			continue
		}
		for _, sub := range subs {
			if sub.IsDir() && names[strings.ToLower(sub.Name())] {
				if info := profileStore(appSupport, filepath.Join(e.Name(), sub.Name()), identifier); info != nil {
					return info
				}
			}
		}
	}
	return nil
}

// candidateNames gives the lowercase folder names that can hold the profiles
// of identifier: the name itself, the name without " Browser", and for a
// bundle id ("com.vendor.App") also its last part.
func candidateNames(identifier string) map[string]bool {
	id := strings.TrimSpace(strings.TrimSuffix(identifier, ".app"))
	if id == "" || strings.ContainsAny(id, `/\`) {
		return nil
	}
	names := map[string]bool{strings.ToLower(id): true}
	if base := strings.TrimSuffix(id, " Browser"); base != id && base != "" {
		names[strings.ToLower(base)] = true
	}
	if !strings.Contains(id, " ") && strings.Count(id, ".") >= 2 {
		parts := strings.Split(id, ".")
		names[strings.ToLower(parts[len(parts)-1])] = true
	}
	return names
}

func profileStore(appSupport, rel, identifier string) *browserInfo {
	dir := filepath.Join(appSupport, rel)
	if fileExists(filepath.Join(dir, "Local State")) {
		return &browserInfo{ConfigDirRelative: rel, ID: identifier, AppName: identifier, Type: "Chromium"}
	}
	if fileExists(filepath.Join(dir, "profiles.ini")) {
		return &browserInfo{ConfigDirRelative: rel, ID: identifier, AppName: identifier, Type: "Firefox"}
	}
	return nil
}

func fileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.Mode().IsRegular()
}
