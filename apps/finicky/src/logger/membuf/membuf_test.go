package membuf

import (
	"bytes"
	"strings"
	"testing"
)

func TestBufferKeepsNewestLines(t *testing.T) {
	b := New(64)
	for i := 0; i < 100; i++ {
		b.Write([]byte(strings.Repeat("x", 9) + string(rune('a'+i%26)) + "\n"))
	}
	got := b.Bytes()
	if len(got) > 64 {
		t.Fatalf("buffer holds %d bytes, max 64", len(got))
	}
	if !bytes.HasSuffix(got, []byte("xxxxxxxxxv\n")) { // line 99: 'a'+99%26 = 'v'
		t.Fatalf("newest line missing: %q", got)
	}
	if got[0] != 'x' {
		t.Fatalf("buffer does not start at a line boundary: %q", got)
	}
}
