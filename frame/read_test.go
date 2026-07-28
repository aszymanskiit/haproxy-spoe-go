package frame

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"testing"
)

type failOnRead struct {
	header []byte
	t      *testing.T
}

func (r *failOnRead) Read(p []byte) (int, error) {
	if len(r.header) > 0 {
		n := copy(p, r.header)
		r.header = r.header[n:]
		return n, nil
	}
	r.t.Fatal("attempted to read frame payload after header rejection")
	return 0, io.EOF
}

func frameHeader(length uint32, typ Type) []byte {
	b := make([]byte, 5)
	binary.BigEndian.PutUint32(b[0:4], length)
	b[4] = byte(typ)
	return b
}

func TestReadWithLimit_MaxUint32KnownType(t *testing.T) {
	src := &failOnRead{
		header: frameHeader(math.MaxUint32, TypeNotify),
		t:      t,
	}
	f := NewFrame()
	err := f.ReadWithLimit(src, DefaultMaxFrameSize)
	if !errors.Is(err, ErrFrameTooLarge) {
		t.Fatalf("expected ErrFrameTooLarge, got %v", err)
	}
	var se *SizeError
	if !errors.As(err, &se) {
		t.Fatalf("expected SizeError, got %T", err)
	}
	if se.Len != math.MaxUint32 || se.Limit != DefaultMaxFrameSize || se.Type != TypeNotify {
		t.Fatalf("unexpected SizeError: %+v", se)
	}
}

func TestReadWithLimit_ZeroLength(t *testing.T) {
	src := &failOnRead{
		header: frameHeader(0, TypeHaproxyHello),
		t:      t,
	}
	f := NewFrame()
	err := f.ReadWithLimit(src, DefaultMaxFrameSize)
	if !errors.Is(err, ErrInvalidFrameLength) {
		t.Fatalf("expected ErrInvalidFrameLength, got %v", err)
	}
}

func TestReadWithLimit_TooShortMetadata(t *testing.T) {
	src := &failOnRead{
		header: frameHeader(minFrameContentLen-1, TypeNotify),
		t:      t,
	}
	f := NewFrame()
	err := f.ReadWithLimit(src, DefaultMaxFrameSize)
	if !errors.Is(err, ErrInvalidFrameLength) {
		t.Fatalf("expected ErrInvalidFrameLength, got %v", err)
	}
}

func TestReadWithLimit_ExactLimitAccepted(t *testing.T) {
	// Build a Notify whose content length equals MinFrameSize exactly, with a
	// valid empty message list (even number of padding bytes).
	const limit = MinFrameSize
	payload := make([]byte, limit-1) // bytes after type
	binary.BigEndian.PutUint32(payload[0:4], 0x01)
	// 2-byte stream id (256) + 1-byte frame id => metadata length 7, remaining 248.
	payload[4] = 0xF0
	payload[5] = 0x01
	payload[6] = 0x00
	for i := 7; i < len(payload); i += 2 {
		payload[i] = 0x00   // message name length 0
		payload[i+1] = 0x00 // nb args 0
	}

	var buf bytes.Buffer
	_ = binary.Write(&buf, binary.BigEndian, uint32(limit))
	buf.WriteByte(byte(TypeNotify))
	buf.Write(payload)

	f := NewFrame()
	if err := f.ReadWithLimit(&buf, limit); err != nil {
		t.Fatalf("expected success at exact limit, got %v", err)
	}
	if f.StreamID != 256 || f.FrameID != 0 {
		t.Fatalf("unexpected ids stream=%d frame=%d", f.StreamID, f.FrameID)
	}
}

func TestReadWithLimit_LimitPlusOneRejected(t *testing.T) {
	src := &failOnRead{
		header: frameHeader(MinFrameSize+1, TypeNotify),
		t:      t,
	}
	f := NewFrame()
	err := f.ReadWithLimit(src, MinFrameSize)
	if !errors.Is(err, ErrFrameTooLarge) {
		t.Fatalf("expected ErrFrameTooLarge, got %v", err)
	}
}

func TestReadWithLimit_UnknownTypeHugeLength(t *testing.T) {
	src := &failOnRead{
		header: frameHeader(math.MaxUint32, Type(47)), // 'G' from "GET /"
		t:      t,
	}
	f := NewFrame()
	err := f.ReadWithLimit(src, DefaultMaxFrameSize)
	if !errors.Is(err, ErrUnexpectedFrameType) {
		t.Fatalf("expected ErrUnexpectedFrameType, got %v", err)
	}
}

func TestReadWithLimit_EarlyRejectDoesNotReadPayload(t *testing.T) {
	src := &failOnRead{
		header: frameHeader(DefaultMaxFrameSize+1, TypeHaproxyHello),
		t:      t,
	}
	f := NewFrame()
	err := f.ReadWithLimit(src, DefaultMaxFrameSize)
	if !errors.Is(err, ErrFrameTooLarge) {
		t.Fatalf("expected ErrFrameTooLarge, got %v", err)
	}
}

func TestReadWithLimit_Uint32ToIntBoundary(t *testing.T) {
	// On all architectures, Len just above DefaultMaxFrameSize must be rejected
	// without converting a huge uint32 into a negative/overflowed int for make().
	cases := []uint32{
		DefaultMaxFrameSize + 1,
		math.MaxUint32,
		math.MaxUint32 - 1,
		uint32(math.MaxInt32) + 1,
	}
	for _, length := range cases {
		if length < minFrameContentLen {
			continue
		}
		src := &failOnRead{
			header: frameHeader(length, TypeNotify),
			t:      t,
		}
		f := NewFrame()
		err := f.ReadWithLimit(src, DefaultMaxFrameSize)
		if err == nil {
			t.Fatalf("length %d: expected error", length)
		}
		if errors.Is(err, ErrFrameTooLarge) || errors.Is(err, ErrInvalidFrameLength) {
			continue
		}
		t.Fatalf("length %d: unexpected error %v", length, err)
	}
}

func TestRead_UsesDefaultLimit(t *testing.T) {
	src := &failOnRead{
		header: frameHeader(math.MaxUint32, TypeNotify),
		t:      t,
	}
	f := NewFrame()
	err := f.Read(src)
	if !errors.Is(err, ErrFrameTooLarge) {
		t.Fatalf("expected ErrFrameTooLarge from Read, got %v", err)
	}
}

func TestValidateMaxFrameSize(t *testing.T) {
	if err := ValidateMaxFrameSize(0); err == nil {
		t.Fatal("expected error for 0")
	}
	if err := ValidateMaxFrameSize(MinFrameSize - 1); err == nil {
		t.Fatal("expected error below minimum")
	}
	if err := ValidateMaxFrameSize(MinFrameSize); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := ValidateMaxFrameSize(DefaultMaxFrameSize); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReset_DropsOversizedReadBuf(t *testing.T) {
	f := NewFrame()
	f.readBuf = make([]byte, maxRetainedReadBufCap+1)
	f.Reset()
	if f.readBuf != nil {
		t.Fatalf("expected oversized buffer to be dropped, cap=%d", cap(f.readBuf))
	}
}

func TestEncodeWithLimit_TooLarge(t *testing.T) {
	f := NewFrame()
	f.Type = TypeAgentAck
	f.Actions = nil
	// Empty ACK is tiny; use a tiny limit below min content.
	_, err := f.EncodeWithLimit(&bytes.Buffer{}, 1)
	if !errors.Is(err, ErrFrameTooLarge) {
		t.Fatalf("expected ErrFrameTooLarge, got %v", err)
	}
}

func TestFrame_Read(t *testing.T) {
	r := bytes.NewBuffer(testFrame)
	f := NewFrame()
	err := f.Read(r)
	if err != nil {
		t.Fatal(err)
	}
	if int(f.FrameID) != 1 {
		t.Fatal("wrong FrameID")
	}
	if int(f.StreamID) != 542 {
		t.Fatal("wrong StreamID")
	}
	if f.Type != TypeNotify {
		t.Fatal("wrong type")
	}
	messages := *f.Messages
	if len(messages) != 1 {
		t.Fatal("wrong messages len")
	}
	host, found := messages[0].KV.Get("host")
	if !found {
		t.Fatal("host not found")
	}
	hostString, ok := host.(string)
	if !ok {
		t.Fatal("error convert host to string")
	}
	if hostString != "domain.example.com" {
		t.Fatal("wrong hostString")
	}
}

func BenchmarkFrame_Read(b *testing.B) {
	readers := make([]io.Reader, b.N)
	for idx := range readers {
		readers[idx] = bytes.NewBuffer(testFrame)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f := NewFrame()
		_ = f.Read(readers[i])
	}
}

var testFrame = []byte(string(
	"\x00\x00\x00\x53" + // Size
		"\x03" + //TypeNotify
		"\x00\x00\x00\x01\xfe\x12\x01\x11\x67\x65\x74" +
		"\x2d\x69\x70\x2d\x72\x65\x70\x75\x74\x61\x74\x69\x6f\x6e\x04\x02" +
		"\x69\x70\x06\xc1\xc8\xe3\xde\x04" +
		"host" + //Host
		"\x08\x12" +
		"domain.example.com" + // authtest.ninjas.pl
		"\x0d\x61\x75\x74\x68\x6f\x72\x69\x7a\x61\x74\x69\x6f\x6e\x00\x06" +
		"\x63\x6f\x6f\x6b\x69\x65\x00",
))
