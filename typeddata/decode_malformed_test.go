package typeddata

import (
	"errors"
	"testing"
)

func TestDecode_Malformed(t *testing.T) {
	cases := []struct {
		name string
		buf  []byte
	}{
		{"empty", nil},
		{"truncated int", []byte{TypeInt32, 0xF0}},
		{"short ipv4", []byte{TypeIPv4, 1, 2}},
		{"short ipv6", []byte{TypeIPv6, 1, 2, 3}},
		{"truncated string len", []byte{TypeString, 0xF0}},
		{"string longer than buf", []byte{TypeString, 0x10, 'a'}},
		{"huge binary len", []byte{TypeBinary, 0xFF, 0xFF, 0xFF, 0xFF, 0x0F}},
		{"unknown type", []byte{0x0F}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := Decode(tc.buf)
			if err == nil {
				t.Fatal("expected error")
			}
		})
	}

	_, _, err := Decode(nil)
	if !errors.Is(err, ErrEmptyBuffer) {
		t.Fatalf("expected ErrEmptyBuffer, got %v", err)
	}
}

func FuzzTypedDataDecode(f *testing.F) {
	f.Add([]byte{TypeBoolean | 0x10})
	f.Add([]byte{TypeString, 0x03, 'a', 'b', 'c'})
	f.Add([]byte{TypeBinary, 0x02, 0x01, 0x02})
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 4096 {
			t.Skip()
		}
		_, _, _ = Decode(data)
	})
}
