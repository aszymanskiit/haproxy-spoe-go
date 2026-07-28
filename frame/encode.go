package frame

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/aszymanskiit/haproxy-spoe-go/varint"
)

func (f *Frame) Encode(dest io.Writer) (n int, err error) {
	return f.EncodeWithLimit(dest, 0)
}

// EncodeWithLimit marshals the frame to dest. If maxFrameSize is non-zero and
// the encoded content length (excluding the 4-byte length prefix) would exceed
// it, encoding fails with ErrFrameTooLarge before writing.
func (f *Frame) EncodeWithLimit(dest io.Writer, maxFrameSize uint32) (n int, err error) {
	buf := bytes.Buffer{}

	buf.WriteByte(byte(f.Type))

	binary.BigEndian.PutUint32(f.tmp[:], f.Flags)

	buf.Write(f.tmp[0:4])

	n = varint.PutUvarint(f.varintBuf[:], f.StreamID)
	if n <= 0 {
		return 0, fmt.Errorf("encode stream id: %w", ErrMalformedFrame)
	}
	buf.Write(f.varintBuf[:n])

	n = varint.PutUvarint(f.varintBuf[:], f.FrameID)
	if n <= 0 {
		return 0, fmt.Errorf("encode frame id: %w", ErrMalformedFrame)
	}
	buf.Write(f.varintBuf[:n])

	var payload []byte

	switch f.Type {
	case TypeAgentHello, TypeAgentDisconnect, TypeHaproxyHello, TypeHaproxyDisconnect:
		payload, err = f.KV.Bytes()
		if err != nil {
			return
		}

	case TypeAgentAck:
		if f.Actions != nil {
			for _, act := range f.Actions {
				payload, err = act.Marshal(payload)
				if err != nil {
					return
				}
			}
		}
	case TypeNotify:
		if len(*f.Messages) > 0 {
			err = fmt.Errorf("encoding Notify frame with Message isn't handled yet")
			return

		}
	default:
		err = fmt.Errorf("%w %d", ErrUnexpectedFrameType, f.Type)
		return
	}

	buf.Write(payload)

	contentLen := buf.Len()
	if contentLen < 0 || uint64(contentLen) > uint64(^uint32(0)) {
		return 0, fmt.Errorf("encoded frame too large: %w", ErrFrameTooLarge)
	}
	contentLenU32 := uint32(contentLen)
	if maxFrameSize > 0 && contentLenU32 > maxFrameSize {
		return 0, &SizeError{
			Op:    "encode frame",
			Len:   contentLenU32,
			Limit: maxFrameSize,
			Type:  f.Type,
			Err:   ErrFrameTooLarge,
		}
	}

	binary.BigEndian.PutUint32(f.tmp[:], contentLenU32)

	if err = writeFull(dest, f.tmp[0:4]); err != nil {
		return 0, fmt.Errorf("error write frameSize: %w", err)
	}

	if err = writeFull(dest, buf.Bytes()); err != nil {
		return 0, fmt.Errorf("error write frame: %w", err)
	}

	return HeaderSize + contentLen, nil
}

func writeFull(w io.Writer, p []byte) error {
	for len(p) > 0 {
		n, err := w.Write(p)
		if err != nil {
			return err
		}
		if n <= 0 {
			return io.ErrShortWrite
		}
		p = p[n:]
	}
	return nil
}
