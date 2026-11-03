package config

import "testing"

func TestNamedGroupDetection(t *testing.T) {
	tests := map[string]bool{
		`match: /(?<host>[a-z]+)\.com/`: true,
		`match: /(?<$x>a)/`:             true,
		`match: /(?<=foo)bar/`:          false,
		`match: /(?<!foo)bar/`:          false,
		`match: "example.com/*"`:        false,
		`match: /(?:a|b)/`:              false,
	}
	for src, want := range tests {
		if got := namedGroupRe.MatchString(src); got != want {
			t.Errorf("namedGroupRe.MatchString(%q) = %v, want %v", src, got, want)
		}
	}
}
