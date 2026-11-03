package version

import (
	"os"
	"strings"
	"testing"
)

func TestIsUpdateAvailable(t *testing.T) {
	tests := []struct {
		name     string
		current  string
		latest   string
		expected bool
		wantErr  bool
	}{
		{"same version", "1.0.0", "1.0.0", false, false},
		{"newer version available", "1.0.0", "1.0.1", true, false},
		{"older version", "1.0.1", "1.0.0", false, false},
		{"v-prefixed versions", "v1.0.0", "v1.0.1", true, false},
		{"mixed prefixes", "1.0.0", "v1.0.1", true, false},
		{"pre-release vs stable", "1.0.0-beta", "1.0.0", true, false},
		{"invalid current version", "invalid", "1.0.0", false, true},
		{"invalid latest version", "1.0.0", "invalid", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := isUpdateAvailable(tt.current, tt.latest)
			if (err != nil) != tt.wantErr {
				t.Errorf("isUpdateAvailable() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.expected {
				t.Errorf("isUpdateAvailable() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestReadCurrentVersionFromXMLPlist(t *testing.T) {
	dir := t.TempDir()
	plist := dir + "/Info.plist"
	content := `<?xml version="1.0" encoding="UTF-8"?>
<plist version="1.0"><dict>
	<key>CFBundleName</key><string>Finicky</string>
	<key>CFBundleVersion</key>
	<string>4.2.2</string>
</dict></plist>`
	if err := os.WriteFile(plist, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BUNDLE_PATH", plist)
	if got := readCurrentVersion(); got != "4.2.2" {
		t.Errorf("readCurrentVersion() = %q, want 4.2.2", got)
	}
}

// The repository VERSION file (read by the docs/manual tooling) must match the
// version the app reports, CFBundleVersion in Info.plist.
func TestVersionFileMatchesInfoPlist(t *testing.T) {
	file, err := os.ReadFile("../../../../VERSION")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("BUNDLE_PATH", "../../assets/Info.plist")
	want := strings.TrimSpace(string(file))
	if got := readCurrentVersion(); got != want {
		t.Fatalf("Info.plist CFBundleVersion = %q, VERSION = %q; update both", got, want)
	}
}
