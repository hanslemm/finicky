// Package membuf provides a byte buffer with a size limit.
package membuf

import (
	"bytes"
	"sync"
)

// Buffer keeps at most max bytes of written data, dropping the oldest lines
// first. It is safe for concurrent use.
type Buffer struct {
	mu  sync.Mutex
	buf []byte
	max int
}

// New returns a Buffer that keeps at most max bytes.
func New(max int) *Buffer {
	return &Buffer{max: max}
}

func (b *Buffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf = append(b.buf, p...)
	if over := len(b.buf) - b.max; over > 0 {
		// Cut at the first line boundary after the excess, then compact.
		cut := over
		if i := bytes.IndexByte(b.buf[over:], '\n'); i >= 0 {
			cut = over + i + 1
		}
		b.buf = append(b.buf[:0], b.buf[cut:]...)
	}
	return len(p), nil
}

// Bytes returns a copy of the buffered data.
func (b *Buffer) Bytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]byte(nil), b.buf...)
}
