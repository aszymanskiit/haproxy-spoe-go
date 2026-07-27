package frame

import (
	"fmt"
	"math"
)

const (
	// MinFrameSize is the minimum max-frame-size allowed by the SPOP
	// specification (see HAProxy SPOE.txt).
	MinFrameSize uint32 = 256

	// DefaultMaxFrameSize is the safe default local limit used only when reading
	// the first HAPROXY-HELLO (before the peer's max-frame-size is known). It
	// matches a common HAProxy default of (tune.bufsize - 4) with tune.bufsize = 16384.
	DefaultMaxFrameSize uint32 = 16380

	// HeaderSize is the size of the on-wire length prefix.
	HeaderSize = 4

	// minFrameContentLen is the smallest valid Len value: type (1) + flags (4)
	// + stream-id varint (1) + frame-id varint (1).
	minFrameContentLen uint32 = 7

	// maxRetainedReadBufCap is the largest read buffer capacity kept when a
	// Frame is returned to the pool. Larger buffers are discarded to limit.
	maxRetainedReadBufCap = 32 * 1024
)

func ValidateMaxFrameSize(v uint32) error {
	if v == 0 {
		return fmt.Errorf("max-frame-size must be >= %d, got 0", MinFrameSize)
	}
	if v < MinFrameSize {
		return fmt.Errorf("max-frame-size must be >= %d, got %d", MinFrameSize, v)
	}
	// Ensure the value can be represented as a positive int on this architecture
	// so later conversions for make/slice never overflow.
	if uint64(v) > uint64(math.MaxInt) {
		return fmt.Errorf("max-frame-size %d exceeds architecture int limit %d", v, math.MaxInt)
	}
	return nil
}
