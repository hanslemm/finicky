package browser

import (
	"os"
	"path/filepath"
	"testing"
)

func touch(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDiscoverIn(t *testing.T) {
	root := t.TempDir()
	touch(t, filepath.Join(root, "Thorium", "Local State"))
	touch(t, filepath.Join(root, "Acme", "Nova Browser", "Local State"))
	touch(t, filepath.Join(root, "Floorp", "profiles.ini"))
	touch(t, filepath.Join(root, "Empty", "notes.txt"))

	tests := []struct {
		id, wantType, wantDir string
	}{
		{"Thorium", "Chromium", "Thorium"},
		{"thorium", "Chromium", "Thorium"},
		{"Nova Browser", "Chromium", filepath.Join("Acme", "Nova Browser")},
		{"Nova", "", ""}, // "Nova" is not a candidate for "Nova Browser" folder
		{"one.ablaze.floorp", "Firefox", "Floorp"},
		{"Empty", "", ""},
		{"../etc", "", ""},
		{"", "", ""},
	}
	for _, tt := range tests {
		got := discoverIn(root, tt.id)
		if tt.wantType == "" {
			if got != nil {
				t.Errorf("discoverIn(%q) = %+v, want nil", tt.id, got)
			}
			continue
		}
		if got == nil || got.Type != tt.wantType || got.ConfigDirRelative != tt.wantDir {
			t.Errorf("discoverIn(%q) = %+v, want %s in %s", tt.id, got, tt.wantType, tt.wantDir)
		}
	}
}
