package worker

import (
	"fmt"

	"github.com/negasus/haproxy-spoe-go/frame"
)

func (w *worker) sendAgentHello(haproxyHello *frame.Frame) error {
	if haproxyHello.MaxFrameSize == 0 {
		msg := "max-frame-size value not found or invalid type"
		if err := w.sendAgentDisconnect(haproxyHello.StreamID, haproxyHello.FrameID, statusMaxFrameSizeNotFound, msg); err != nil {
			return err
		}
		return fmt.Errorf("%s", msg)
	}

	// After the first HELLO (bounded by the local safety limit), the peer's
	// max-frame-size is authoritative for the rest of the connection.
	if err := frame.ValidateMaxFrameSize(haproxyHello.MaxFrameSize); err != nil {
		msg := "max-frame-size too big or too small"
		if discErr := w.sendAgentDisconnect(haproxyHello.StreamID, haproxyHello.FrameID, statusMaxFrameSizeInvalid, msg); discErr != nil {
			return discErr
		}
		return err
	}
	peerMax := haproxyHello.MaxFrameSize

	agentHello := frame.AcquireFrame()
	defer frame.ReleaseFrame(agentHello)

	agentHello.Type = frame.TypeAgentHello
	agentHello.FrameID = haproxyHello.FrameID
	agentHello.StreamID = haproxyHello.StreamID

	agentHello.KV.Add("version", "2.0")
	agentHello.KV.Add("max-frame-size", peerMax)
	agentHello.KV.Add("capabilities", capabilities)

	if err := w.writeFrame(agentHello); err != nil {
		return err
	}

	// Switch from the local HELLO-only limit to HAProxy's announced value.
	w.maxFrameSize = peerMax
	return nil
}
