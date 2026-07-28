package kv

import (
	"testing"
)

func TestUnmarshal_Malformed(t *testing.T) {
	cases := [][]byte{
		{0xF0},            // truncated key length varint
		{0x05, 'a', 'b'},  // key longer than buffer
		{0x01, 'a'},       // missing value
		{0x01, 'a', 0x08}, // truncated string typed-data
		{0x01, 'a', 0x08, 0xFF, 0xFF, 0xFF, 0xFF, 0x0F}, // huge string length
	}

	for i, buf := range cases {
		kv := NewKV()
		if err := kv.Unmarshal(buf); err == nil {
			t.Fatalf("case %d: expected error", i)
		}
	}
}

func TestUnmarshalNB_Malformed(t *testing.T) {
	kv := NewKV()
	if _, err := kv.UnmarshalNB(nil, 1); err == nil {
		t.Fatal("expected error for empty buffer with count=1")
	}
	if _, err := kv.UnmarshalNB([]byte{0x01, 'a'}, -1); err == nil {
		t.Fatal("expected error for negative count")
	}
}

func FuzzKVUnmarshal(f *testing.F) {
	f.Add([]byte{0x03, 'k', 'e', 'y', 0x08, 0x03, 'a', 'b', 'c'})
	f.Add([]byte{0xFF})
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 4096 {
			t.Skip()
		}
		kv := NewKV()
		_ = kv.Unmarshal(data)
	})
}
