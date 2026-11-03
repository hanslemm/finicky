package browser

import (
	"errors"
	"testing"
)

func TestOpenNativeRejectsNUL(t *testing.T) {
	cases := []struct {
		app, url string
		args     []string
	}{
		{"Safari", "https://example.com/\x00x", nil},
		{"Saf\x00ari", "https://example.com/", nil},
		{"Safari", "https://example.com/", []string{"--a\x00b"}},
	}
	for _, c := range cases {
		if err := openNative(c.app, false, c.url, true, false, c.args, c.args == nil); !errors.Is(err, errNativeUnresolved) {
			t.Errorf("openNative(%q, %q, %q) = %v, want errNativeUnresolved", c.app, c.url, c.args, err)
		}
	}
}
