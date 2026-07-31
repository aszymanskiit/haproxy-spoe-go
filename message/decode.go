package message

import (
	"fmt"

	"github.com/aszymanskiit/haproxy-spoe-go/varint"
)

func (m *Messages) Decode(buf []byte) error {
	for {
		if len(buf) == 0 {
			break
		}

		message := AcquireMessage()

		messageNameLen, n := varint.Uvarint(buf)
		if n <= 0 {
			ReleaseMessage(message)
			return fmt.Errorf("message name length: truncated varint")
		}
		buf = buf[n:]

		if messageNameLen > uint64(len(buf)) {
			ReleaseMessage(message)
			return fmt.Errorf("message name length %d exceeds remaining buffer %d", messageNameLen, len(buf))
		}
		nameLen := int(messageNameLen)
		message.Name = string(buf[:nameLen])
		buf = buf[nameLen:]

		if len(buf) < 1 {
			ReleaseMessage(message)
			return fmt.Errorf("missing message arguments count")
		}
		nbArgs := int(buf[0])
		buf = buf[1:]

		consumed, err := message.KV.UnmarshalNB(buf, nbArgs)
		if err != nil {
			ReleaseMessage(message)
			return err
		}
		if consumed < 0 || consumed > len(buf) {
			ReleaseMessage(message)
			return fmt.Errorf("invalid kv bytes consumed: %d", consumed)
		}
		buf = buf[consumed:]

		*m = append(*m, message)
	}

	return nil
}
