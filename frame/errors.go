package frame

import (
	"errors"
	"fmt"
)

var (
	// ErrFrameTooLarge is returned when a frame length exceeds the configured
	// or negotiated maximum frame size.
	ErrFrameTooLarge = errors.New("frame exceeds maximum size")

	// ErrInvalidFrameLength is returned when the declared frame length is zero
	// or otherwise invalid for the SPOP framing format.
	ErrInvalidFrameLength = errors.New("invalid frame length")

	// ErrMalformedFrame is returned when frame metadata or payload cannot be
	// decoded safely (truncated fields, bad varints, etc.).
	ErrMalformedFrame = errors.New("malformed frame")

	// ErrUnexpectedFrameType is returned when the frame type byte is not a
	// known SPOP frame type.
	ErrUnexpectedFrameType = errors.New("unexpected frame type")
)

// SizeError describes a rejected frame length. Use errors.Is(err, ErrFrameTooLarge)
// or errors.Is(err, ErrInvalidFrameLength) to classify the failure.
type SizeError struct {
	Op    string
	Len   uint32
	Limit uint32
	Type  Type
	Err   error
}

func (e *SizeError) Error() string {
	if e.Type != 0 {
		return fmt.Sprintf("%s: declared length %d, limit %d, type %d: %v", e.Op, e.Len, e.Limit, e.Type, e.Err)
	}
	return fmt.Sprintf("%s: declared length %d, limit %d: %v", e.Op, e.Len, e.Limit, e.Err)
}

func (e *SizeError) Unwrap() error {
	return e.Err
}

// DecodeError describes a malformed frame payload without embedding untrusted data.
type DecodeError struct {
	Op   string
	Type Type
	Err  error
}

func (e *DecodeError) Error() string {
	if e.Type != 0 {
		return fmt.Sprintf("%s: type %d: %v", e.Op, e.Type, e.Err)
	}
	return fmt.Sprintf("%s: %v", e.Op, e.Err)
}

func (e *DecodeError) Unwrap() error {
	return e.Err
}
