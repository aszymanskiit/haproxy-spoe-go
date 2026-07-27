package varint

import "testing"

func FuzzUvarint(f *testing.F) {
	f.Add([]byte{0xEF})
	f.Add([]byte{0xF0})
	f.Add([]byte{0xF0, 0x01})
	f.Add([]byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF})
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 32 {
			t.Skip()
		}
		_, n := Uvarint(data)
		if n > len(data) {
			t.Fatalf("n=%d > len=%d", n, len(data))
		}
	})
}
