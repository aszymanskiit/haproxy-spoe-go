package worker

import (
	"fmt"

	"github.com/negasus/haproxy-spoe-go/frame"
)

func (w *worker) processHaproxyHello(haproxyHello *frame.Frame) (bool, error) {
	defer frame.ReleaseFrame(haproxyHello)

	if w.ready {
		return false, fmt.Errorf("worker already ready, but got HaproxyHello frame")
	}

	if haproxyHello.MaxFrameSize == 0 {
		msg := "max-frame-size value not found or invalid type"
		if err := w.sendAgentDisconnect(haproxyHello.StreamID, haproxyHello.FrameID, statusMaxFrameSizeNotFound, msg); err != nil {
			return false, err
		}
		return false, fmt.Errorf("%s", msg)
	}

	// After the first HELLO (bounded by the local safety limit), the peer's
	// max-frame-size is authoritative for the rest of the connection.
	if err := frame.ValidateMaxFrameSize(haproxyHello.MaxFrameSize); err != nil {
		msg := "max-frame-size too big or too small"
		if discErr := w.sendAgentDisconnect(haproxyHello.StreamID, haproxyHello.FrameID, statusMaxFrameSizeInvalid, msg); discErr != nil {
			return false, discErr
		}
		return false, err
	}

	if err := w.sendAgentHello(haproxyHello); err != nil {
		return false, fmt.Errorf("error send AgentHello frame: %w", err)
	}

	if haproxyHello.Healthcheck {
		return false, nil
	}

	w.engineID = haproxyHello.EngineID
	w.maxFrameSize = haproxyHello.MaxFrameSize
	w.ready = true

	return true, nil
}

func (w *worker) sendAgentHello(haproxyHello *frame.Frame) error {
	agentHello := frame.AcquireFrame()
	defer frame.ReleaseFrame(agentHello)

	agentHello.Type = frame.TypeAgentHello
	agentHello.FrameID = haproxyHello.FrameID
	agentHello.StreamID = haproxyHello.StreamID

	agentHello.KV.Add("version", "2.0")
	agentHello.KV.Add("max-frame-size", haproxyHello.MaxFrameSize)
	agentHello.KV.Add("capabilities", capabilities)

	if err := w.writeFrame(agentHello); err != nil {
		return err
	}

	return nil
}
