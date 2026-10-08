package server

import (
	"bytes"
	"sync"
)

// maxPooledBufferBytes keeps an unusually large response from holding its
// memory after it was sent.
const maxPooledBufferBytes = 1 << 20

// responseBuffers holds the buffers responses render into before they are
// sent, so a render error can still become an error page.
var responseBuffers = sync.Pool{New: func() any { return new(bytes.Buffer) }}

func getBuffer() *bytes.Buffer { return responseBuffers.Get().(*bytes.Buffer) }

func putBuffer(buf *bytes.Buffer) {
	if buf.Cap() > maxPooledBufferBytes {
		return
	}
	buf.Reset()
	responseBuffers.Put(buf)
}
