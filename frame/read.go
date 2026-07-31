package frame

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"

	"github.com/aszymanskiit/haproxy-spoe-go/varint"
)

func (f *Frame) Read(src io.Reader) error {
	return f.ReadWithLimit(src, DefaultMaxFrameSize)
}

// ReadWithLimit reads a single SPOP frame from src, rejecting any frame whose
// declared length exceeds maxFrameSize before allocating or reading the payload.
//
// maxFrameSize is the negotiated/local SPOP max-frame-size (content length
// excluding the 4-byte length prefix). A value of 0 is treated as
// DefaultMaxFrameSize so callers cannot accidentally disable limits.
func (f *Frame) ReadWithLimit(src io.Reader, maxFrameSize uint32) error {
	if maxFrameSize == 0 {
		maxFrameSize = DefaultMaxFrameSize
	}
	if err := ValidateMaxFrameSize(maxFrameSize); err != nil {
		return err
	}

	n, err := io.ReadFull(src, f.tmp[:])
	if err != nil {
		if err == io.EOF && n == 0 {
			return io.EOF
		}
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return err
		}
		return fmt.Errorf("error read frame size: %w", err)
	}

	f.Len = binary.BigEndian.Uint32(f.tmp[0:4])
	f.Type = Type(f.tmp[4])

	if !isKnownFrameType(f.Type) {
		return &DecodeError{
			Op:   "read frame",
			Type: f.Type,
			Err:  fmt.Errorf("%w %d", ErrUnexpectedFrameType, f.Type),
		}
	}

	if f.Len == 0 {
		return &SizeError{
			Op:    "read frame",
			Len:   f.Len,
			Limit: maxFrameSize,
			Type:  f.Type,
			Err:   ErrInvalidFrameLength,
		}
	}

	if f.Len < minFrameContentLen {
		return &SizeError{
			Op:    "read frame",
			Len:   f.Len,
			Limit: maxFrameSize,
			Type:  f.Type,
			Err:   ErrInvalidFrameLength,
		}
	}

	if f.Len > maxFrameSize {
		return &SizeError{
			Op:    "read frame",
			Len:   f.Len,
			Limit: maxFrameSize,
			Type:  f.Type,
			Err:   ErrFrameTooLarge,
		}
	}

	// Len includes the type byte already consumed from the 5-byte header.
	payloadLenU32 := f.Len - 1
	if uint64(payloadLenU32) > uint64(math.MaxInt) {
		return &SizeError{
			Op:    "read frame",
			Len:   f.Len,
			Limit: maxFrameSize,
			Type:  f.Type,
			Err:   ErrFrameTooLarge,
		}
	}
	payloadLen := int(payloadLenU32)

	if cap(f.readBuf) < payloadLen {
		f.readBuf = make([]byte, payloadLen)
	} else {
		f.readBuf = f.readBuf[:payloadLen]
	}

	n, err = io.ReadFull(src, f.readBuf)
	if err != nil {
		return fmt.Errorf("error read frame payload: %w", err)
	}
	if n != payloadLen {
		return fmt.Errorf("unexpected frame payload length %d, expect %d", n, payloadLen)
	}

	buf := f.readBuf

	if len(buf) < 4 {
		return &DecodeError{Op: "decode flags", Type: f.Type, Err: ErrMalformedFrame}
	}
	f.Flags = binary.BigEndian.Uint32(buf[0:4])
	buf = buf[4:]

	var vn int
	f.StreamID, vn = varint.Uvarint(buf)
	if vn <= 0 {
		return &DecodeError{Op: "decode stream id", Type: f.Type, Err: ErrMalformedFrame}
	}
	buf = buf[vn:]

	f.FrameID, vn = varint.Uvarint(buf)
	if vn <= 0 {
		return &DecodeError{Op: "decode frame id", Type: f.Type, Err: ErrMalformedFrame}
	}
	buf = buf[vn:]

	switch f.Type {
	case TypeHaproxyHello, TypeHaproxyDisconnect, TypeAgentHello, TypeAgentDisconnect:
		if err = f.KV.Unmarshal(buf); err != nil {
			return &DecodeError{Op: "decode kv", Type: f.Type, Err: err}
		}
		f.applyHelloKV()

	case TypeNotify:
		if err = f.Messages.Decode(buf); err != nil {
			return &DecodeError{Op: "decode messages", Type: f.Type, Err: err}
		}

	case TypeAgentAck:
		// ACK payload is a list of actions; generic readers leave it unparsed.

	default:
		return &DecodeError{
			Op:   "read frame",
			Type: f.Type,
			Err:  fmt.Errorf("%w %d", ErrUnexpectedFrameType, f.Type),
		}
	}

	return nil
}

func isKnownFrameType(t Type) bool {
	switch t {
	case TypeHaproxyHello, TypeHaproxyDisconnect, TypeNotify,
		TypeAgentHello, TypeAgentDisconnect, TypeAgentAck:
		return true
	default:
		return false
	}
}

func (f *Frame) applyHelloKV() {
	if v, ok := f.KV.Get("healthcheck"); ok {
		if b, ok := v.(bool); ok {
			f.Healthcheck = b
		}
	}
	if v, ok := f.KV.Get("max-frame-size"); ok {
		if size, ok := asUint32(v); ok {
			f.MaxFrameSize = size
		}
	}
	if v, ok := f.KV.Get("engine-id"); ok {
		if s, ok := v.(string); ok {
			f.EngineID = s
		}
	}
}

func asUint32(v interface{}) (uint32, bool) {
	switch n := v.(type) {
	case uint32:
		return n, true
	case uint64:
		if n > math.MaxUint32 {
			return 0, false
		}
		return uint32(n), true
	case int64:
		if n < 0 || n > math.MaxUint32 {
			return 0, false
		}
		return uint32(n), true
	case int32:
		if n < 0 {
			return 0, false
		}
		return uint32(n), true
	case int:
		if n < 0 || uint64(n) > math.MaxUint32 {
			return 0, false
		}
		return uint32(n), true
	case uint:
		if uint64(n) > math.MaxUint32 {
			return 0, false
		}
		return uint32(n), true
	default:
		return 0, false
	}
}
