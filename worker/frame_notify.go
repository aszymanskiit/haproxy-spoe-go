package worker

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/negasus/haproxy-spoe-go/frame"
	"github.com/negasus/haproxy-spoe-go/request"
)

func (w *worker) processNotifyFrame(f *frame.Frame) {
	defer frame.ReleaseFrame(f)
	defer w.wg.Done()

	req := request.AcquireRequest()
	defer request.ReleaseRequest(req)

	req.StreamID = f.StreamID
	req.FrameID = f.FrameID
	req.EngineID = w.engineID
	req.Messages = f.Messages

	w.handler(req)

	ackFrame := frame.AcquireFrame()
	defer frame.ReleaseFrame(ackFrame)

	ackFrame.Type = frame.TypeAgentAck
	ackFrame.StreamID = f.StreamID
	ackFrame.FrameID = f.FrameID
	ackFrame.Actions = req.Actions

	err := w.writeFrame(ackFrame)
	if err != nil {
		if errors.Is(err, frame.ErrFrameTooLarge) {
			// ACK exceeded negotiated max-frame-size; notify peer then close path via log.
			_ = w.sendAgentDisconnect(0, 0, statusFrameTooBig, "ack frame too big")
		}
		w.logger.Errorf("ack frame write failed: %v", err)
	}
}

func (w *worker) writeFrame(f *frame.Frame) error {
	buf := bytes.NewBuffer(make([]byte, 0))
	n, err := f.EncodeWithLimit(buf, w.maxFrameSize)
	if err != nil {
		return fmt.Errorf("cannot marshal frame: %w", err)
	}
	if n != buf.Len() {
		return fmt.Errorf("encoded size mismatch %d, expect %d", n, buf.Len())
	}

	// Encode off the writer goroutine; only the socket write is serialized.
	errc := make(chan error, 1)
	w.writeCh <- writeRequest{data: buf.Bytes(), errc: errc}
	if err := <-errc; err != nil {
		return fmt.Errorf("cannot write frame to connection: %w", err)
	}

	return nil
}

func writeFull(w interface{ Write([]byte) (int, error) }, p []byte) error {
	for len(p) > 0 {
		n, err := w.Write(p)
		if err != nil {
			return err
		}
		if n <= 0 {
			return fmt.Errorf("short write")
		}
		p = p[n:]
	}
	return nil
}
