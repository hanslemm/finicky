package shorturl

import "testing"

func TestIsShortenerHost(t *testing.T) {
	saved := shortenerDomains
	t.Cleanup(func() { shortenerDomains = saved })
	shortenerDomains = []string{"bit.ly", "t.co"}

	tests := map[string]bool{
		"bit.ly":         true,
		"BIT.LY":         true,
		"www.bit.ly":     true,
		"bit.ly:443":     true,
		"bit.ly.":        true,
		"notbit.ly":      false,
		"t.co":           true,
		"microsoft.com":  false,
		"bit.ly.evil.io": false,
	}
	for host, want := range tests {
		if got := isShortenerHost(host); got != want {
			t.Errorf("isShortenerHost(%q) = %v, want %v", host, got, want)
		}
	}
}

func TestResolveURLSkipsNonShortener(t *testing.T) {
	got, err := ResolveURL("https://example.com/a")
	if err != nil || got != "https://example.com/a" {
		t.Fatalf("got %q, %v", got, err)
	}
}
