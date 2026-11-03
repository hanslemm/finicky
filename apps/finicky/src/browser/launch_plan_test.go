package browser

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestNativePlan(t *testing.T) {
	const url = "https://example.com/x"
	profile := []string{"--profile-directory=Profile 3"}

	tests := []struct {
		name       string
		custom     []string
		profile    []string
		hasProfile bool
		wantNative bool
		want       launchPlan
	}{
		{"plain URL", nil, nil, false, true, launchPlan{passURL: true}},
		{"profile", nil, profile, true, true,
			launchPlan{args: []string{"--profile-directory=Profile 3", url}, newInstance: true}},
		{"app args without --args", []string{"--incognito", url}, nil, false, true,
			launchPlan{args: []string{"--incognito", url}}},
		{"open-style -n --args (the common profile workaround)",
			[]string{"-n", "--args", "--profile-directory=Profile 15", url}, nil, false, true,
			launchPlan{args: []string{"--profile-directory=Profile 15", url}, newInstance: true}},
		{"-g before --args", []string{"-g", "--args", url}, nil, false, true,
			launchPlan{args: []string{url}, background: true}},
		{"unknown open flag needs open(1)", []string{"-F", "--args", url}, nil, false, false, launchPlan{}},
		{"profile plus --args needs open(1)", []string{"--args", url}, profile, true, false, launchPlan{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, native := nativePlan(tt.custom, tt.profile, tt.hasProfile, url, false)
			if native != tt.wantNative {
				t.Fatalf("native = %v, want %v", native, tt.wantNative)
			}
			if !native {
				return
			}
			if !slices.Equal(got.args, tt.want.args) || got.passURL != tt.want.passURL ||
				got.newInstance != tt.want.newInstance || got.background != tt.want.background {
				t.Fatalf("plan = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestChromiumProfileDirRe(t *testing.T) {
	for name, want := range map[string]bool{
		"Default": true, "Profile 1": true, "Profile 15": true, "Guest Profile": true,
		"Work": false, "Profile": false, "Profile x": false, "../Default": false,
	} {
		if got := chromiumProfileDirRe.MatchString(name); got != want {
			t.Errorf("%q: got %v, want %v", name, got, want)
		}
	}
}

func TestIsBlocked(t *testing.T) {
	dir := t.TempDir()
	open := filepath.Join(dir, "open")
	locked := filepath.Join(dir, "locked")
	os.WriteFile(open, []byte("{}"), 0o644)
	os.WriteFile(locked, []byte("{}"), 0o000)
	if isBlocked(open) {
		t.Error("readable file reported as blocked")
	}
	if isBlocked(filepath.Join(dir, "missing")) {
		t.Error("missing file reported as blocked")
	}
	if os.Geteuid() != 0 && !isBlocked(locked) {
		t.Error("unreadable file not reported as blocked")
	}
}
