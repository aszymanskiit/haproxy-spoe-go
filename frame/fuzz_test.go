package frame

import (
	"bytes"
	"testing"
)

func FuzzFrameRead(f *testing.F) {
	f.Add(testFrame)
	f.Add([]byte("GET / HTTP/1.1\r\n\r\n"))
	f.Add([]byte{0, 0, 0, 0, 1})
	f.Add([]byte{0xff, 0xff, 0xff, 0xff, 3})
	f.Add([]byte{0, 0, 0, 7, 3, 0, 0, 0, 1, 0, 0})
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, data []byte) {
		frame := NewFrame()
		defer ReleaseFrame(frame)
		_ = frame.ReadWithLimit(bytes.NewReader(data), MinFrameSize)
	})
}
