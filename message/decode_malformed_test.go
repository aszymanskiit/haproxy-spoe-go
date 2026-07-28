package message

import (
	"testing"
)

func TestDecode_Malformed(t *testing.T) {
	cases := []struct {
		name string
		buf  []byte
	}{
		{"empty ok", nil},
		{"truncated name varint", []byte{0xF0}},
		{"name longer than buffer", []byte{0x05, 'a', 'b'}},
		{"missing args count", []byte{0x03, 'a', 'b', 'c'}},
		{"truncated kv", []byte{0x01, 'a', 0x01, 0x03, 'k'}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := NewMessages()
			err := m.Decode(tc.buf)
			if tc.name == "empty ok" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func FuzzMessagesDecode(f *testing.F) {
	f.Add([]byte{0x03, 'F', 'o', 'o', 0x00})
	f.Add([]byte{0xFF, 0xFF, 0xFF})
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 4096 {
			t.Skip()
		}
		m := NewMessages()
		_ = m.Decode(data)
		m.Reset()
	})
}
